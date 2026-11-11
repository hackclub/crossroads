package api

// Claim links: a special stamp type with a claim code (set by admins) can be
// claimed at /claim/{code} on the platform, once per person, until its supply
// runs out. It's how a link in an announcement gives out a stamp.

import (
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/scherepi/crossroads-backend/auth"
	"github.com/scherepi/crossroads-backend/db"
	"github.com/scherepi/crossroads-backend/respond"
)

type stampClaimJSON struct {
	SpecialStamp specialStampJSON `json:"special_stamp"`
	// True if they'd claimed it before, in which case this is that stamp and
	// nothing new was given.
	AlreadyClaimed bool `json:"already_claimed"`
}

func (a *API) claimSpecialStamp(w http.ResponseWriter, r *http.Request) {
	user := auth.User(r.Context())
	var body struct {
		Code string `json:"code"`
	}
	if !decode(w, r, &body) {
		return
	}
	code := strings.ToLower(strings.TrimSpace(body.Code))
	tx, err := a.Pool.Begin(r.Context())
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())
	q := a.Queries.WithTx(tx)

	t, err := q.GetSpecialStampTypeByClaimCode(r.Context(), &code)
	if errors.Is(err, pgx.ErrNoRows) {
		respond.Error(w, http.StatusNotFound, "That claim link doesn't work. Check it's the whole link.")
		return
	}
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	existing, err := q.GetClaimedSpecialStamp(r.Context(), db.GetClaimedSpecialStampParams{UserID: user.ID, SpecialStampTypeID: t.ID})
	if err == nil {
		respond.JSON(w, http.StatusOK, stampClaimJSON{SpecialStamp: toSpecialStampJSON(existing, t), AlreadyClaimed: true})
		return
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		respond.ServerError(w, r, err)
		return
	}
	t, err = q.ClaimSpecialStampSupply(r.Context(), t.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		respond.Error(w, http.StatusConflict, "Every one of these has been claimed already.")
		return
	}
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	stamp, err := q.CreateClaimedSpecialStamp(r.Context(), db.CreateClaimedSpecialStampParams{UserID: user.ID, SpecialStampTypeID: t.ID})
	// Another request of theirs claimed it at the same moment.
	if db.IsViolation(err, "special_stamps_one_claim_per_type") {
		respond.Error(w, http.StatusConflict, "You've already claimed this stamp.")
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
	respond.JSON(w, http.StatusCreated, stampClaimJSON{SpecialStamp: toSpecialStampJSON(stamp, t)})
}
