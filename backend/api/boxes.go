package api

// Boxes come from milestones (see review.go) or from admins. Opening one rolls
// a special stamp from its box type's drops, skipping stamp types whose supply
// has run out.

import (
	"crypto/rand"
	"errors"
	"log"
	"math/big"
	"net/http"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/scherepi/crossroads-backend/auth"
	"github.com/scherepi/crossroads-backend/db"
	"github.com/scherepi/crossroads-backend/respond"
)

type boxJSON struct {
	ID             int64      `json:"id"`
	BoxTypeID      int64      `json:"box_type_id"`
	MilestoneID    *int64     `json:"milestone_id"` // null if an admin gave it
	GrantedAt      time.Time  `json:"granted_at"`
	OpenedAt       *time.Time `json:"opened_at"`
	SpecialStampID *int64     `json:"special_stamp_id"` // what it dropped, once opened
}

func toBoxJSON(b db.Box, specialStampID *int64) boxJSON {
	return boxJSON{
		ID:             b.ID,
		BoxTypeID:      b.BoxTypeID,
		MilestoneID:    b.MilestoneID,
		GrantedAt:      b.GrantedAt,
		OpenedAt:       b.OpenedAt,
		SpecialStampID: specialStampID,
	}
}

type specialStampTypeJSON struct {
	ID          int64                 `json:"id"`
	Name        string                `json:"name"`
	Description string                `json:"description"`
	ImageURL    *string               `json:"image_url"`
	Rarity      db.SpecialStampRarity `json:"rarity"`
	Effect      db.SpecialStampEffect `json:"effect"`
}

func toSpecialStampTypeJSON(t db.SpecialStampType) specialStampTypeJSON {
	return specialStampTypeJSON{
		ID:          t.ID,
		Name:        t.Name,
		Description: t.Description,
		ImageURL:    t.ImageUrl,
		Rarity:      t.Rarity,
		Effect:      t.Effect,
	}
}

type specialStampJSON struct {
	ID           int64                `json:"id"`
	Type         specialStampTypeJSON `json:"type"`
	BoxID        *int64               `json:"box_id"`        // null if an admin gave it
	SubmissionID *int64               `json:"submission_id"` // set once it's been used
	AcquiredAt   time.Time            `json:"acquired_at"`
	// Set once it's stuck on a project; it's in the stamp box until then.
	ProjectID *int64 `json:"project_id"`
}

func toSpecialStampJSON(s db.SpecialStamp, t db.SpecialStampType) specialStampJSON {
	return specialStampJSON{
		ID:           s.ID,
		Type:         toSpecialStampTypeJSON(t),
		BoxID:        s.BoxID,
		SubmissionID: s.SubmissionID,
		AcquiredAt:   s.AcquiredAt,
		ProjectID:    s.ProjectID,
	}
}

func (a *API) listBoxes(w http.ResponseWriter, r *http.Request) {
	user := auth.User(r.Context())
	rows, err := a.Queries.ListBoxes(r.Context(), user.ID)
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	out := []boxJSON{}
	for _, row := range rows {
		out = append(out, toBoxJSON(db.Box{
			ID:          row.ID,
			BoxTypeID:   row.BoxTypeID,
			MilestoneID: row.MilestoneID,
			GrantedAt:   row.GrantedAt,
			OpenedAt:    row.OpenedAt,
		}, row.SpecialStampID))
	}
	respond.JSON(w, http.StatusOK, out)
}

type boxTypeJSON struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	ImageURL    *string `json:"image_url"`
	// What can drop right now and the chance of each, from 0 to 1. Stamp
	// types that have run out aren't listed.
	Odds []oddsJSON `json:"odds"`
}

type oddsJSON struct {
	SpecialStampType specialStampTypeJSON `json:"special_stamp_type"`
	Chance           float64              `json:"chance"`
}

// listBoxTypes lists every box type with its current odds, so people can see
// exactly what they might get.
func (a *API) listBoxTypes(w http.ResponseWriter, r *http.Request) {
	boxTypes, err := a.Queries.ListBoxTypes(r.Context())
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	drops, err := a.Queries.ListAvailableDrops(r.Context())
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	out := []boxTypeJSON{}
	for _, boxType := range boxTypes {
		available := dropsFor(drops, boxType.ID)
		total := 0
		for _, d := range available {
			total += int(d.Weight)
		}
		odds := []oddsJSON{}
		for _, d := range available {
			odds = append(odds, oddsJSON{
				SpecialStampType: toSpecialStampTypeJSON(d.SpecialStampType),
				Chance:           float64(d.Weight) / float64(total),
			})
		}
		out = append(out, boxTypeJSON{
			ID:          boxType.ID,
			Name:        boxType.Name,
			Description: boxType.Description,
			ImageURL:    boxType.ImageUrl,
			Odds:        odds,
		})
	}
	respond.JSON(w, http.StatusOK, out)
}

func dropsFor(drops []db.ListAvailableDropsRow, boxTypeID int64) []db.ListAvailableDropsRow {
	var out []db.ListAvailableDropsRow
	for _, d := range drops {
		if d.BoxTypeID == boxTypeID {
			out = append(out, d)
		}
	}
	return out
}

func (a *API) openBox(w http.ResponseWriter, r *http.Request) {
	user := auth.User(r.Context())
	id, ok := idParam(w, r, "boxID")
	if !ok {
		return
	}
	tx, err := a.Pool.Begin(r.Context())
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())
	q := a.Queries.WithTx(tx)

	box, err := q.LockBox(r.Context(), db.LockBoxParams{ID: id, UserID: user.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		respond.Error(w, http.StatusNotFound, "That box doesn't exist.")
		return
	}
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	if box.OpenedAt != nil {
		respond.Error(w, http.StatusConflict, "You've already opened this box.")
		return
	}
	all, err := q.ListAvailableDrops(r.Context())
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	drops := dropsFor(all, box.BoxTypeID)

	for len(drops) > 0 {
		i, err := roll(drops)
		if err != nil {
			respond.ServerError(w, r, err)
			return
		}
		stampType, err := q.ClaimSpecialStampSupply(r.Context(), drops[i].SpecialStampType.ID)
		if errors.Is(err, pgx.ErrNoRows) {
			// Someone else just got the last one. Roll again without it.
			drops = slices.Delete(drops, i, i+1)
			continue
		}
		if err != nil {
			respond.ServerError(w, r, err)
			return
		}
		stamp, err := q.CreateSpecialStamp(r.Context(), db.CreateSpecialStampParams{
			UserID:             user.ID,
			SpecialStampTypeID: stampType.ID,
			BoxID:              &box.ID,
		})
		if err != nil {
			respond.ServerError(w, r, err)
			return
		}
		if err := q.MarkBoxOpened(r.Context(), box.ID); err != nil {
			respond.ServerError(w, r, err)
			return
		}
		if err := tx.Commit(r.Context()); err != nil {
			respond.ServerError(w, r, err)
			return
		}
		respond.JSON(w, http.StatusOK, toSpecialStampJSON(stamp, stampType))
		return
	}
	// Admins can't save a box type without an uncapped drop, so this
	// shouldn't happen.
	log.Printf("box type %d has nothing left to drop", box.BoxTypeID)
	respond.Error(w, http.StatusConflict, "This box has nothing left to drop right now. Let the organizers know!")
}

// roll picks one of drops at random, weighted by their weights.
func roll(drops []db.ListAvailableDropsRow) (int, error) {
	total := int64(0)
	for _, d := range drops {
		total += int64(d.Weight)
	}
	n, err := rand.Int(rand.Reader, big.NewInt(total))
	if err != nil {
		return 0, err
	}
	pick := n.Int64()
	for i, d := range drops {
		if pick < int64(d.Weight) {
			return i, nil
		}
		pick -= int64(d.Weight)
	}
	panic("unreachable")
}

func (a *API) listSpecialStamps(w http.ResponseWriter, r *http.Request) {
	user := auth.User(r.Context())
	rows, err := a.Queries.ListSpecialStamps(r.Context(), user.ID)
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	out := []specialStampJSON{}
	for _, row := range rows {
		out = append(out, toSpecialStampJSON(row.SpecialStamp, row.SpecialStampType))
	}
	respond.JSON(w, http.StatusOK, out)
}
