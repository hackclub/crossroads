package api

// Stamp placement: special stamps go from the owner's stamp box onto one of
// their projects, where anyone viewing the project sees them. A placed stamp
// stays on that project for good; the owner can only move it around on it.
//
// Effects happen when the stamp goes on:
//   - Express (priority_review): the project's ship in review gets priority
//     review. If nothing is in review, the next ship does (see submit).
//   - Golden Spike (stamp_multiplier): approved ships of the project pay 10%
//     more stamps (see reviewSubmission). One spike or several, it's 10%.
//   - Collectibles (none): just there to look at.

import (
	"errors"
	"math"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/scherepi/crossroads-backend/auth"
	"github.com/scherepi/crossroads-backend/db"
	"github.com/scherepi/crossroads-backend/respond"
)

// placedStampJSON is a stamp as it sits on a project. It carries nothing
// private, so it's safe to show to other people viewing the project.
type placedStampJSON struct {
	ID       int64                `json:"id"`
	Type     specialStampTypeJSON `json:"type"`
	X        float32              `json:"x"`        // 0-1 across the stamp board
	Y        float32              `json:"y"`        // 0-1 down the stamp board
	Rotation float32              `json:"rotation"` // degrees
	// The ship an Express stamp gave priority review to, if it has yet.
	SubmissionID *int64    `json:"submission_id"`
	PlacedAt     time.Time `json:"placed_at"`
}

func toPlacedStampJSON(s db.SpecialStamp, t db.SpecialStampType) placedStampJSON {
	out := placedStampJSON{ID: s.ID, Type: toSpecialStampTypeJSON(t), Rotation: s.Rotation, SubmissionID: s.SubmissionID}
	if s.PosX != nil {
		out.X = *s.PosX
	}
	if s.PosY != nil {
		out.Y = *s.PosY
	}
	if s.PlacedAt != nil {
		out.PlacedAt = *s.PlacedAt
	}
	return out
}

func (a *API) projectStamps(r *http.Request, q *db.Queries, projectID int64) ([]placedStampJSON, error) {
	rows, err := q.ListProjectStamps(r.Context(), &projectID)
	if err != nil {
		return nil, err
	}
	out := []placedStampJSON{}
	for _, row := range rows {
		out = append(out, toPlacedStampJSON(row.SpecialStamp, row.SpecialStampType))
	}
	return out, nil
}

// placeStamp is POST /special-stamps/{stampID}/placement with
// {"project_id": 3, "x": 0.4, "y": 0.2, "rotation": -6}. The first call sticks
// a stamp from the box onto the project (rotation is only taken then); later
// calls with the same project move it. → 200 placedStampJSON.
func (a *API) placeStamp(w http.ResponseWriter, r *http.Request) {
	user := auth.User(r.Context())
	id, ok := idParam(w, r, "stampID")
	if !ok {
		return
	}
	var body struct {
		ProjectID int64   `json:"project_id"`
		X         float64 `json:"x"`
		Y         float64 `json:"y"`
		Rotation  float64 `json:"rotation"`
	}
	if !decode(w, r, &body) {
		return
	}
	if math.IsNaN(body.X) || math.IsNaN(body.Y) || math.IsNaN(body.Rotation) {
		respond.Error(w, http.StatusUnprocessableEntity, "x, y and rotation have to be numbers.")
		return
	}
	x := float32(min(max(body.X, 0), 1))
	y := float32(min(max(body.Y, 0), 1))
	rotation := float32(min(max(body.Rotation, -45), 45))

	tx, err := a.Pool.Begin(r.Context())
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())
	q := a.Queries.WithTx(tx)

	locked, err := q.LockSpecialStamp(r.Context(), db.LockSpecialStampParams{ID: id, UserID: user.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		respond.Error(w, http.StatusNotFound, "You don't have that special stamp.")
		return
	}
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	stamp := locked.SpecialStamp

	// Moving a stamp that's already on this project.
	if stamp.ProjectID != nil {
		if *stamp.ProjectID != body.ProjectID {
			respond.Error(w, http.StatusConflict, "That stamp is stuck on another project. Stamps can be moved around their project, but not to a different one.")
			return
		}
		moved, err := q.MoveSpecialStamp(r.Context(), db.MoveSpecialStampParams{ID: stamp.ID, PosX: &x, PosY: &y})
		if err != nil {
			respond.ServerError(w, r, err)
			return
		}
		a.respondPlaced(w, r, q, tx, moved)
		return
	}

	// Sticking it on for the first time.
	if stamp.SubmissionID != nil {
		respond.Error(w, http.StatusConflict, "That stamp was already used on a ship.")
		return
	}
	project, err := q.LockProject(r.Context(), db.LockProjectParams{ID: body.ProjectID, UserID: user.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		respond.Error(w, http.StatusNotFound, "That project doesn't exist.")
		return
	}
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	placed, err := q.PlaceSpecialStamp(r.Context(), db.PlaceSpecialStampParams{
		ID: stamp.ID, ProjectID: &project.ID, PosX: &x, PosY: &y, Rotation: rotation,
	})
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	// Express gives the ship in review priority, if it hasn't got one already.
	if locked.Effect == db.SpecialStampEffectPriorityReview {
		pending, err := q.PendingSubmissionForProject(r.Context(), project.ID)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			// Nothing in review: the next ship picks it up.
		case err != nil:
			respond.ServerError(w, r, err)
			return
		default:
			has, err := q.SubmissionHasSpecialStamp(r.Context(), &pending.ID)
			if err != nil {
				respond.ServerError(w, r, err)
				return
			}
			if !has {
				if err := q.ApplySpecialStamp(r.Context(), db.ApplySpecialStampParams{ID: placed.ID, SubmissionID: &pending.ID}); err != nil {
					respond.ServerError(w, r, err)
					return
				}
				placed.SubmissionID = &pending.ID
			}
		}
	}
	a.respondPlaced(w, r, q, tx, placed)
}

func (a *API) respondPlaced(w http.ResponseWriter, r *http.Request, q *db.Queries, tx pgx.Tx, stamp db.SpecialStamp) {
	t, err := q.GetSpecialStampType(r.Context(), stamp.SpecialStampTypeID)
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		respond.ServerError(w, r, err)
		return
	}
	respond.JSON(w, http.StatusOK, toPlacedStampJSON(stamp, t))
}

// stampBonus is the Golden Spike's extra 10%, rounded to the nearest stamp.
func stampBonus(paid int64) int64 {
	return (paid + 5) / 10
}
