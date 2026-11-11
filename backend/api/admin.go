package api

// Admins set up special stamp types, box types and their drops, and
// milestones, and can give people boxes and special stamps directly.

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/scherepi/crossroads-backend/auth"
	"github.com/scherepi/crossroads-backend/db"
	"github.com/scherepi/crossroads-backend/respond"
)

// optional is a JSON field that can be left out, set to null, or set to a
// value. Set is false if it was left out.
type optional[T any] struct {
	Set   bool
	Value *T
}

func (o *optional[T]) UnmarshalJSON(b []byte) error {
	o.Set = true
	if string(b) == "null" {
		o.Value = nil
		return nil
	}
	o.Value = new(T)
	return json.Unmarshal(b, o.Value)
}

type adminSpecialStampTypeJSON struct {
	specialStampTypeJSON
	SupplyCap    *int32  `json:"supply_cap"` // null is unlimited
	IssuedCount  int32   `json:"issued_count"`
	AppliedCount int64   `json:"applied_count"` // used on submissions
	ClaimCode    *string `json:"claim_code"`    // claimable at /claim/{code}; null if it isn't
}

// specialStampTypeFields is the body of POST and PATCH
// /admin/special-stamp-types. Fields left out aren't changed.
type specialStampTypeFields struct {
	Name        *string                `json:"name"`
	Description *string                `json:"description"`
	ImageURL    optional[string]       `json:"image_url"`
	Rarity      *db.SpecialStampRarity `json:"rarity"`
	Effect      *db.SpecialStampEffect `json:"effect"`
	SupplyCap   optional[int32]        `json:"supply_cap"` // null is unlimited
	ClaimCode   optional[string]       `json:"claim_code"` // null or "" stops claiming
}

func (f specialStampTypeFields) apply(t *db.SpecialStampType) string {
	if f.Name != nil {
		t.Name = strings.TrimSpace(*f.Name)
	}
	if f.Description != nil {
		t.Description = strings.TrimSpace(*f.Description)
	}
	if f.ImageURL.Set {
		t.ImageUrl = f.ImageURL.Value
	}
	if f.Rarity != nil {
		t.Rarity = *f.Rarity
	}
	if f.Effect != nil {
		t.Effect = *f.Effect
	}
	if f.SupplyCap.Set {
		t.SupplyCap = f.SupplyCap.Value
	}
	if f.ClaimCode.Set {
		t.ClaimCode = nil
		if f.ClaimCode.Value != nil {
			if code := strings.ToLower(strings.TrimSpace(*f.ClaimCode.Value)); code != "" {
				t.ClaimCode = &code
			}
		}
	}
	switch {
	case t.Name == "" || len(t.Name) > 100:
		return "Give it a name under 100 characters."
	case t.ImageUrl != nil && !isWebURL(*t.ImageUrl):
		return "image_url has to start with http:// or https://."
	case !t.Rarity.Valid():
		return fmt.Sprintf("rarity has to be one of %v.", db.AllSpecialStampRarityValues())
	case !t.Effect.Valid():
		return fmt.Sprintf("effect has to be one of %v.", db.AllSpecialStampEffectValues())
	case t.SupplyCap != nil && *t.SupplyCap < 0:
		return "supply_cap can't be negative."
	case t.ClaimCode != nil && (len(*t.ClaimCode) > 64 || strings.Trim(*t.ClaimCode, "abcdefghijklmnopqrstuvwxyz0123456789-") != ""):
		return "claim_code can only have letters, numbers and dashes, up to 64."
	}
	return ""
}

func (a *API) adminListSpecialStampTypes(w http.ResponseWriter, r *http.Request) {
	rows, err := a.Queries.ListSpecialStampTypes(r.Context())
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	out := []adminSpecialStampTypeJSON{}
	for _, row := range rows {
		out = append(out, adminSpecialStampTypeJSON{
			specialStampTypeJSON: toSpecialStampTypeJSON(db.SpecialStampType{
				ID: row.ID, Name: row.Name, Description: row.Description, ImageUrl: row.ImageUrl, Rarity: row.Rarity, Effect: row.Effect,
			}),
			SupplyCap:    row.SupplyCap,
			IssuedCount:  row.IssuedCount,
			AppliedCount: row.AppliedCount,
			ClaimCode:    row.ClaimCode,
		})
	}
	respond.JSON(w, http.StatusOK, out)
}

func (a *API) adminCreateSpecialStampType(w http.ResponseWriter, r *http.Request) {
	var fields specialStampTypeFields
	if !decode(w, r, &fields) {
		return
	}
	t := db.SpecialStampType{Effect: db.SpecialStampEffectNone}
	if problem := fields.apply(&t); problem != "" {
		respond.Error(w, http.StatusUnprocessableEntity, problem)
		return
	}
	t, err := a.Queries.CreateSpecialStampType(r.Context(), db.CreateSpecialStampTypeParams{
		Name: t.Name, Description: t.Description, ImageUrl: t.ImageUrl, Rarity: t.Rarity, Effect: t.Effect, SupplyCap: t.SupplyCap, ClaimCode: t.ClaimCode,
	})
	if db.IsViolation(err, "special_stamp_types_claim_code_key") {
		respond.Error(w, http.StatusConflict, "Another stamp type already has that claim code.")
		return
	}
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	respond.JSON(w, http.StatusCreated, adminSpecialStampTypeJSON{
		specialStampTypeJSON: toSpecialStampTypeJSON(t),
		SupplyCap:            t.SupplyCap,
		IssuedCount:          t.IssuedCount,
		ClaimCode:            t.ClaimCode,
	})
}

// adminUpdateSpecialStampType edits a stamp type. Lowering supply_cap below
// how many have been issued just stops further drops; nobody loses theirs.
func (a *API) adminUpdateSpecialStampType(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "typeID")
	if !ok {
		return
	}
	var fields specialStampTypeFields
	if !decode(w, r, &fields) {
		return
	}
	tx, err := a.Pool.Begin(r.Context())
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())
	q := a.Queries.WithTx(tx)

	t, err := q.GetSpecialStampType(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		respond.Error(w, http.StatusNotFound, "That special stamp type doesn't exist.")
		return
	}
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	wasUnlimited := t.SupplyCap == nil
	if problem := fields.apply(&t); problem != "" {
		respond.Error(w, http.StatusUnprocessableEntity, problem)
		return
	}
	// Every box has to keep at least one stamp type that can't run out, or it
	// could come up empty.
	if wasUnlimited && t.SupplyCap != nil {
		stranded, err := q.BoxTypesOnlyUncappedBy(r.Context(), t.ID)
		if err != nil {
			respond.ServerError(w, r, err)
			return
		}
		if len(stranded) > 0 {
			respond.Error(w, http.StatusConflict, fmt.Sprintf(
				"It's the only unlimited stamp in box types %v, which could then run empty. Give them another unlimited stamp first.", stranded))
			return
		}
	}
	t, err = q.UpdateSpecialStampType(r.Context(), db.UpdateSpecialStampTypeParams{
		ID: t.ID, Name: t.Name, Description: t.Description, ImageUrl: t.ImageUrl, Rarity: t.Rarity, Effect: t.Effect, SupplyCap: t.SupplyCap, ClaimCode: t.ClaimCode,
	})
	if db.IsViolation(err, "special_stamp_types_claim_code_key") {
		respond.Error(w, http.StatusConflict, "Another stamp type already has that claim code.")
		return
	}
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		respond.ServerError(w, r, err)
		return
	}
	respond.JSON(w, http.StatusOK, adminSpecialStampTypeJSON{
		specialStampTypeJSON: toSpecialStampTypeJSON(t),
		SupplyCap:            t.SupplyCap,
		IssuedCount:          t.IssuedCount,
		ClaimCode:            t.ClaimCode,
	})
}

type adminBoxTypeJSON struct {
	ID          int64      `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	ImageURL    *string    `json:"image_url"`
	Drops       []dropJSON `json:"drops"` // including stamp types that have run out
}

type dropJSON struct {
	SpecialStampTypeID int64 `json:"special_stamp_type_id"`
	Weight             int32 `json:"weight"`
}

type boxTypeFields struct {
	Name        *string          `json:"name"`
	Description *string          `json:"description"`
	ImageURL    optional[string] `json:"image_url"`
}

func (f boxTypeFields) apply(b *db.BoxType) string {
	if f.Name != nil {
		b.Name = strings.TrimSpace(*f.Name)
	}
	if f.Description != nil {
		b.Description = strings.TrimSpace(*f.Description)
	}
	if f.ImageURL.Set {
		b.ImageUrl = f.ImageURL.Value
	}
	switch {
	case b.Name == "" || len(b.Name) > 100:
		return "Give it a name under 100 characters."
	case b.ImageUrl != nil && !isWebURL(*b.ImageUrl):
		return "image_url has to start with http:// or https://."
	}
	return ""
}

func toAdminBoxTypeJSON(b db.BoxType, drops []db.BoxDrop) adminBoxTypeJSON {
	out := adminBoxTypeJSON{ID: b.ID, Name: b.Name, Description: b.Description, ImageURL: b.ImageUrl, Drops: []dropJSON{}}
	for _, d := range drops {
		if d.BoxTypeID == b.ID {
			out.Drops = append(out.Drops, dropJSON{SpecialStampTypeID: d.SpecialStampTypeID, Weight: d.Weight})
		}
	}
	return out
}

func (a *API) adminListBoxTypes(w http.ResponseWriter, r *http.Request) {
	boxTypes, err := a.Queries.ListBoxTypes(r.Context())
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	drops, err := a.Queries.ListBoxDrops(r.Context())
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	out := []adminBoxTypeJSON{}
	for _, b := range boxTypes {
		out = append(out, toAdminBoxTypeJSON(b, drops))
	}
	respond.JSON(w, http.StatusOK, out)
}

// adminCreateBoxType makes a box type with no drops. Set them with PUT
// /admin/box-types/{id}/drops before handing any out.
func (a *API) adminCreateBoxType(w http.ResponseWriter, r *http.Request) {
	var fields boxTypeFields
	if !decode(w, r, &fields) {
		return
	}
	var b db.BoxType
	if problem := fields.apply(&b); problem != "" {
		respond.Error(w, http.StatusUnprocessableEntity, problem)
		return
	}
	b, err := a.Queries.CreateBoxType(r.Context(), db.CreateBoxTypeParams{Name: b.Name, Description: b.Description, ImageUrl: b.ImageUrl})
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toAdminBoxTypeJSON(b, nil))
}

func (a *API) adminUpdateBoxType(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "boxTypeID")
	if !ok {
		return
	}
	var fields boxTypeFields
	if !decode(w, r, &fields) {
		return
	}
	b, err := a.Queries.GetBoxType(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		respond.Error(w, http.StatusNotFound, "That box type doesn't exist.")
		return
	}
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	if problem := fields.apply(&b); problem != "" {
		respond.Error(w, http.StatusUnprocessableEntity, problem)
		return
	}
	b, err = a.Queries.UpdateBoxType(r.Context(), db.UpdateBoxTypeParams{ID: b.ID, Name: b.Name, Description: b.Description, ImageUrl: b.ImageUrl})
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	drops, err := a.Queries.ListBoxDrops(r.Context())
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	respond.JSON(w, http.StatusOK, toAdminBoxTypeJSON(b, drops))
}

// adminSetBoxDrops replaces a box type's whole drop table. At least one of
// the stamp types has to be unlimited, so the box can never come up empty.
func (a *API) adminSetBoxDrops(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "boxTypeID")
	if !ok {
		return
	}
	var drops []dropJSON
	if !decode(w, r, &drops) {
		return
	}
	tx, err := a.Pool.Begin(r.Context())
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())
	q := a.Queries.WithTx(tx)

	b, err := q.GetBoxType(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		respond.Error(w, http.StatusNotFound, "That box type doesn't exist.")
		return
	}
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	var seen []int64
	hasUnlimited := false
	for _, d := range drops {
		if d.Weight <= 0 {
			respond.Error(w, http.StatusUnprocessableEntity, "Every weight has to be above 0.")
			return
		}
		if slices.Contains(seen, d.SpecialStampTypeID) {
			respond.Error(w, http.StatusUnprocessableEntity, "Each special stamp type can only be in a box once.")
			return
		}
		seen = append(seen, d.SpecialStampTypeID)
		t, err := q.GetSpecialStampType(r.Context(), d.SpecialStampTypeID)
		if errors.Is(err, pgx.ErrNoRows) {
			respond.Error(w, http.StatusUnprocessableEntity, fmt.Sprintf("Special stamp type %d doesn't exist.", d.SpecialStampTypeID))
			return
		}
		if err != nil {
			respond.ServerError(w, r, err)
			return
		}
		if t.SupplyCap == nil {
			hasUnlimited = true
		}
	}
	if !hasUnlimited {
		respond.Error(w, http.StatusUnprocessableEntity, "At least one special stamp type in the box has to be unlimited, so it can never come up empty.")
		return
	}
	if err := q.DeleteBoxDrops(r.Context(), b.ID); err != nil {
		respond.ServerError(w, r, err)
		return
	}
	for _, d := range drops {
		if err := q.AddBoxDrop(r.Context(), db.AddBoxDropParams{BoxTypeID: b.ID, SpecialStampTypeID: d.SpecialStampTypeID, Weight: d.Weight}); err != nil {
			respond.ServerError(w, r, err)
			return
		}
	}
	saved, err := q.ListBoxDrops(r.Context())
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		respond.ServerError(w, r, err)
		return
	}
	respond.JSON(w, http.StatusOK, toAdminBoxTypeJSON(b, saved))
}

type milestoneJSON struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	ApprovedSeconds int32  `json:"approved_seconds"`
	BoxTypeID       int64  `json:"box_type_id"`
}

func (a *API) adminListMilestones(w http.ResponseWriter, r *http.Request) {
	milestones, err := a.Queries.ListMilestones(r.Context())
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	out := []milestoneJSON{}
	for _, m := range milestones {
		out = append(out, milestoneJSON{ID: m.ID, Name: m.Name, ApprovedSeconds: m.ApprovedSeconds, BoxTypeID: m.BoxTypeID})
	}
	respond.JSON(w, http.StatusOK, out)
}

// adminCreateMilestone adds a milestone and gives its box straight away to
// everyone who's already past it.
func (a *API) adminCreateMilestone(w http.ResponseWriter, r *http.Request) {
	var body milestoneJSON
	if !decode(w, r, &body) {
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	switch {
	case body.Name == "" || len(body.Name) > 100:
		respond.Error(w, http.StatusUnprocessableEntity, "Give it a name under 100 characters.")
		return
	case body.ApprovedSeconds <= 0:
		respond.Error(w, http.StatusUnprocessableEntity, "approved_seconds has to be above 0.")
		return
	}
	tx, err := a.Pool.Begin(r.Context())
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())
	q := a.Queries.WithTx(tx)

	if _, err := q.GetBoxType(r.Context(), body.BoxTypeID); errors.Is(err, pgx.ErrNoRows) {
		respond.Error(w, http.StatusUnprocessableEntity, "That box type doesn't exist.")
		return
	} else if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	m, err := q.CreateMilestone(r.Context(), db.CreateMilestoneParams{Name: body.Name, ApprovedSeconds: body.ApprovedSeconds, BoxTypeID: body.BoxTypeID})
	if db.IsViolation(err, "milestones_approved_seconds_key") {
		respond.Error(w, http.StatusConflict, "There's already a milestone at that many hours.")
		return
	}
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	granted, err := q.GrantMilestoneToQualifiedUsers(r.Context(), db.GrantMilestoneToQualifiedUsersParams{
		BoxTypeID: m.BoxTypeID, MilestoneID: m.ID, ApprovedSeconds: m.ApprovedSeconds,
	})
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		respond.ServerError(w, r, err)
		return
	}
	respond.JSON(w, http.StatusCreated, struct {
		Milestone    milestoneJSON `json:"milestone"`
		BoxesGranted int           `json:"boxes_granted"`
	}{milestoneJSON{ID: m.ID, Name: m.Name, ApprovedSeconds: m.ApprovedSeconds, BoxTypeID: m.BoxTypeID}, len(granted)})
}

func (a *API) adminGrantBox(w http.ResponseWriter, r *http.Request) {
	admin := auth.User(r.Context())
	userID, ok := idParam(w, r, "userID")
	if !ok {
		return
	}
	var body struct {
		BoxTypeID int64 `json:"box_type_id"`
	}
	if !decode(w, r, &body) {
		return
	}
	if _, err := a.Queries.GetUser(r.Context(), userID); errors.Is(err, pgx.ErrNoRows) {
		respond.Error(w, http.StatusNotFound, "That user doesn't exist.")
		return
	} else if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	if _, err := a.Queries.GetBoxType(r.Context(), body.BoxTypeID); errors.Is(err, pgx.ErrNoRows) {
		respond.Error(w, http.StatusUnprocessableEntity, "That box type doesn't exist.")
		return
	} else if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	box, err := a.Queries.GrantBox(r.Context(), db.GrantBoxParams{UserID: userID, BoxTypeID: body.BoxTypeID, GrantedBy: &admin.ID})
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toBoxJSON(box, nil))
}

// adminGrantSpecialStamp gives someone a special stamp directly. It still
// comes out of the stamp type's supply.
func (a *API) adminGrantSpecialStamp(w http.ResponseWriter, r *http.Request) {
	admin := auth.User(r.Context())
	userID, ok := idParam(w, r, "userID")
	if !ok {
		return
	}
	var body struct {
		SpecialStampTypeID int64 `json:"special_stamp_type_id"`
	}
	if !decode(w, r, &body) {
		return
	}
	tx, err := a.Pool.Begin(r.Context())
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())
	q := a.Queries.WithTx(tx)

	if _, err := q.GetUser(r.Context(), userID); errors.Is(err, pgx.ErrNoRows) {
		respond.Error(w, http.StatusNotFound, "That user doesn't exist.")
		return
	} else if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	if _, err := q.GetSpecialStampType(r.Context(), body.SpecialStampTypeID); errors.Is(err, pgx.ErrNoRows) {
		respond.Error(w, http.StatusUnprocessableEntity, "That special stamp type doesn't exist.")
		return
	} else if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	t, err := q.ClaimSpecialStampSupply(r.Context(), body.SpecialStampTypeID)
	if errors.Is(err, pgx.ErrNoRows) {
		respond.Error(w, http.StatusConflict, "That special stamp type has run out. Raise its supply_cap first.")
		return
	}
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	stamp, err := q.CreateSpecialStamp(r.Context(), db.CreateSpecialStampParams{UserID: userID, SpecialStampTypeID: t.ID, GrantedBy: &admin.ID})
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		respond.ServerError(w, r, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toSpecialStampJSON(stamp, t))
}

type adminUserJSON struct {
	ID                 int64       `json:"id"`
	Name               string      `json:"name"`
	Email              string      `json:"email"`
	SlackID            *string     `json:"slack_id"`
	Role               db.UserRole `json:"role"`
	Stamps             int32       `json:"stamps"`
	VerificationStatus string      `json:"verification_status"`
	YSWSEligible       bool        `json:"ysws_eligible"`
	// Which review stages a reviewer can decide. Admins can decide all of
	// them whatever these say.
	CanReview      bool `json:"can_review"`
	CanFraudReview bool `json:"can_fraud_review"`
	CanHQReview    bool `json:"can_hq_review"`
}

func toAdminUserJSON(u db.User) adminUserJSON {
	return adminUserJSON{
		ID:                 u.ID,
		Name:               u.Name,
		Email:              u.Email,
		SlackID:            u.SlackID,
		Role:               u.Role,
		Stamps:             u.Stamps,
		VerificationStatus: u.VerificationStatus,
		YSWSEligible:       u.YswsEligible,
		CanReview:          u.CanReview,
		CanFraudReview:     u.CanFraudReview,
		CanHQReview:        u.CanHqReview,
	}
}

// adminListUsers finds people by part of their name or email, or their whole
// Slack ID: ?q=heidi. Without q it lists the newest 50.
func (a *API) adminListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := a.Queries.SearchUsers(r.Context(), strings.TrimSpace(r.URL.Query().Get("q")))
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	out := []adminUserJSON{}
	for _, u := range users {
		out = append(out, toAdminUserJSON(u))
	}
	respond.JSON(w, http.StatusOK, out)
}

// adminUpdateUser changes someone's role: {"role": "reviewer"}.
// adminUpdateUser changes someone's role and which review stages they can
// decide. Fields left out aren't changed.
func (a *API) adminUpdateUser(w http.ResponseWriter, r *http.Request) {
	admin := auth.User(r.Context())
	userID, ok := idParam(w, r, "userID")
	if !ok {
		return
	}
	var body struct {
		Role           *db.UserRole `json:"role"`
		CanReview      *bool        `json:"can_review"`
		CanFraudReview *bool        `json:"can_fraud_review"`
		CanHQReview    *bool        `json:"can_hq_review"`
	}
	if !decode(w, r, &body) {
		return
	}
	if body.Role != nil && !body.Role.Valid() {
		respond.Error(w, http.StatusUnprocessableEntity, fmt.Sprintf("role has to be one of %v.", db.AllUserRoleValues()))
		return
	}
	if userID == admin.ID && body.Role != nil && *body.Role != db.UserRoleAdmin {
		respond.Error(w, http.StatusUnprocessableEntity, "You can't take away your own admin access. Ask another admin.")
		return
	}
	user, err := a.Queries.GetUser(r.Context(), userID)
	if errors.Is(err, pgx.ErrNoRows) {
		respond.Error(w, http.StatusNotFound, "That user doesn't exist.")
		return
	}
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	if body.Role != nil {
		user.Role = *body.Role
	}
	for field, value := range map[*bool]*bool{&user.CanReview: body.CanReview, &user.CanFraudReview: body.CanFraudReview, &user.CanHqReview: body.CanHQReview} {
		if value != nil {
			*field = *value
		}
	}
	if user.Role == db.UserRoleParticipant && (user.CanReview || user.CanFraudReview || user.CanHqReview) {
		respond.Error(w, http.StatusUnprocessableEntity, "Make them a reviewer before giving them review stages.")
		return
	}
	tx, err := a.Pool.Begin(r.Context())
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())
	q := a.Queries.WithTx(tx)
	if _, err := q.SetUserRole(r.Context(), db.SetUserRoleParams{ID: userID, Role: user.Role}); err != nil {
		respond.ServerError(w, r, err)
		return
	}
	user, err = q.SetUserPermissions(r.Context(), db.SetUserPermissionsParams{
		ID: userID, CanReview: user.CanReview, CanFraudReview: user.CanFraudReview, CanHqReview: user.CanHqReview,
	})
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		respond.ServerError(w, r, err)
		return
	}
	respond.JSON(w, http.StatusOK, toAdminUserJSON(user))
}

type reviewSettingsJSON struct {
	// Every approved ship goes through the fraud check, flagged or not.
	FraudCheckEverything bool `json:"fraud_check_everything"`
	// Ships claiming more than this always get a fraud check.
	LargeShipSeconds int32 `json:"large_ship_seconds"`
	// So do ships whose project has a UTC day with more than this.
	LongDaySeconds int32     `json:"long_day_seconds"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func toReviewSettingsJSON(s db.ReviewSetting) reviewSettingsJSON {
	return reviewSettingsJSON{s.FraudCheckEverything, s.LargeShipSeconds, s.LongDaySeconds, s.UpdatedAt}
}

func (a *API) adminGetReviewSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := a.Queries.GetReviewSettings(r.Context())
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	respond.JSON(w, http.StatusOK, toReviewSettingsJSON(settings))
}

// adminUpdateReviewSettings changes the fraud check settings. Fields left
// out aren't changed. They apply to approvals from now on; ships already
// past the first stage stay where they are.
func (a *API) adminUpdateReviewSettings(w http.ResponseWriter, r *http.Request) {
	var body struct {
		FraudCheckEverything *bool  `json:"fraud_check_everything"`
		LargeShipSeconds     *int32 `json:"large_ship_seconds"`
		LongDaySeconds       *int32 `json:"long_day_seconds"`
	}
	if !decode(w, r, &body) {
		return
	}
	settings, err := a.Queries.GetReviewSettings(r.Context())
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	if body.FraudCheckEverything != nil {
		settings.FraudCheckEverything = *body.FraudCheckEverything
	}
	if body.LargeShipSeconds != nil {
		settings.LargeShipSeconds = *body.LargeShipSeconds
	}
	if body.LongDaySeconds != nil {
		settings.LongDaySeconds = *body.LongDaySeconds
	}
	if settings.LargeShipSeconds < 1 || settings.LongDaySeconds < 1 || settings.LongDaySeconds > 86400 {
		respond.Error(w, http.StatusUnprocessableEntity, "large_ship_seconds has to be over 0, and long_day_seconds between 1 and 86,400 (a day).")
		return
	}
	settings, err = a.Queries.UpdateReviewSettings(r.Context(), db.UpdateReviewSettingsParams{
		FraudCheckEverything: settings.FraudCheckEverything, LargeShipSeconds: settings.LargeShipSeconds, LongDaySeconds: settings.LongDaySeconds,
	})
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	respond.JSON(w, http.StatusOK, toReviewSettingsJSON(settings))
}

type roleInviteJSON struct {
	ID            int64       `json:"id"`
	Email         string      `json:"email"`
	Role          db.UserRole `json:"role"`
	InvitedByName *string     `json:"invited_by_name"` // null if that admin's account is gone
	CreatedAt     time.Time   `json:"created_at"`
}

// adminListRoleInvites lists the roles waiting for people who haven't signed
// in yet.
func (a *API) adminListRoleInvites(w http.ResponseWriter, r *http.Request) {
	invites, err := a.Queries.ListRoleInvites(r.Context())
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	out := []roleInviteJSON{}
	for _, i := range invites {
		out = append(out, roleInviteJSON{i.ID, i.Email, i.Role, i.InvitedByName, i.CreatedAt})
	}
	respond.JSON(w, http.StatusOK, out)
}

// adminInviteRole gives a role to whoever first signs in with an email:
// {"email": "orpheus@hackclub.com", "role": "admin"}. The email is matched
// against their Hack Club Auth email, ignoring case. Someone who has already
// signed in gets their role changed in the people table instead.
func (a *API) adminInviteRole(w http.ResponseWriter, r *http.Request) {
	admin := auth.User(r.Context())
	var body struct {
		Email string      `json:"email"`
		Role  db.UserRole `json:"role"`
	}
	if !decode(w, r, &body) {
		return
	}
	email := strings.ToLower(strings.TrimSpace(body.Email))
	if address, err := mail.ParseAddress(email); err != nil || address.Address != email || len(email) > 254 {
		respond.Error(w, http.StatusUnprocessableEntity, "That doesn't look like an email address.")
		return
	}
	if body.Role != db.UserRoleReviewer && body.Role != db.UserRoleAdmin {
		respond.Error(w, http.StatusUnprocessableEntity, "role has to be reviewer or admin.")
		return
	}
	switch user, err := a.Queries.GetUserByEmail(r.Context(), email); {
	case err == nil:
		respond.Error(w, http.StatusConflict, fmt.Sprintf(
			"%s has already signed in, as %s (%s). Change their role in the table instead.", email, user.Name, user.Role))
		return
	case !errors.Is(err, pgx.ErrNoRows):
		respond.ServerError(w, r, err)
		return
	}
	invite, err := a.Queries.UpsertRoleInvite(r.Context(), db.UpsertRoleInviteParams{Email: email, Role: body.Role, InvitedBy: &admin.ID})
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	respond.JSON(w, http.StatusCreated, roleInviteJSON{invite.ID, invite.Email, invite.Role, &admin.Name, invite.CreatedAt})
}

func (a *API) adminDeleteRoleInvite(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "inviteID")
	if !ok {
		return
	}
	deleted, err := a.Queries.DeleteRoleInvite(r.Context(), id)
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	if deleted == 0 {
		respond.Error(w, http.StatusNotFound, "That invite doesn't exist. They may have signed in already.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
