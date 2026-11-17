package api

// Past reviews and what can be done about rejections afterwards.
//
// A rejection is soft or hard. Soft: they change the project and ship it
// again. Hard (decision.BlockResubmit, for fraud): the project can't be
// shipped again until HQ or an admin allows it. HQ and admins see every
// rejected ship, can switch a project between the two, and can put a rejected
// ship back in the review queue.

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/scherepi/crossroads-backend/auth"
	"github.com/scherepi/crossroads-backend/db"
	"github.com/scherepi/crossroads-backend/respond"
)

// canManageRejections: HQ reviewers and admins.
func canManageRejections(u *db.User) bool { return permissionsOf(u).HQReview }

type pastReviewJSON struct {
	ID              int64     `json:"id"` // submission ID
	ProjectID       int64     `json:"project_id"`
	ProjectName     string    `json:"project_name"`
	UserID          int64     `json:"user_id"`
	UserName        string    `json:"user_name"`
	Status          string    `json:"status"` // approved or rejected
	NewSeconds      int32     `json:"new_seconds"`
	ApprovedSeconds *int32    `json:"approved_seconds"`
	SubmittedAt     time.Time `json:"submitted_at"`
	DecidedAt       time.Time `json:"decided_at"`
	DecidedBy       *string   `json:"decided_by"` // who made the last decision
	Feedback        *string   `json:"feedback"`
	ResubmitBlocked bool      `json:"resubmit_blocked"` // a hard reject
	ShippedAgain    bool      `json:"shipped_again"`    // the project has a newer ship
}

// pastReviews is GET /review/past?scope=mine|all&status=approved|rejected:
// decided ships, the latest decided first, at most 200. Reviewers see the
// ships they decided; HQ and admins can see everyone's (scope=all, the
// default for them).
func (a *API) pastReviews(w http.ResponseWriter, r *http.Request) {
	viewer := auth.User(r.Context())
	params := db.ListPastReviewsParams{MaxRows: 200}
	switch status := r.URL.Query().Get("status"); status {
	case "":
	case "approved", "rejected":
		params.Status = new(db.SubmissionStatus(status))
	default:
		respond.Error(w, http.StatusUnprocessableEntity, `status has to be "approved" or "rejected".`)
		return
	}
	switch scope := r.URL.Query().Get("scope"); {
	case scope == "mine" || !canManageRejections(viewer):
		params.DecidedBy = &viewer.ID
	case scope != "" && scope != "all":
		respond.Error(w, http.StatusUnprocessableEntity, `scope has to be "mine" or "all".`)
		return
	}
	rows, err := a.Queries.ListPastReviews(r.Context(), params)
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	items := []pastReviewJSON{}
	for _, row := range rows {
		items = append(items, pastReviewJSON{
			ID: row.ID, ProjectID: row.ProjectID, ProjectName: row.ProjectName, UserID: row.UserID, UserName: row.UserName,
			Status: string(row.Status), NewSeconds: row.NewSeconds, ApprovedSeconds: row.ApprovedSeconds,
			SubmittedAt: row.SubmittedAt, DecidedAt: row.DecidedAt, DecidedBy: nonEmpty(row.DecidedByName),
			Feedback: row.Feedback, ResubmitBlocked: row.ResubmitBlocked, ShippedAgain: row.ShippedAgain,
		})
	}
	respond.JSON(w, http.StatusOK, struct {
		All   bool             `json:"all"` // everyone's, not just the viewer's
		Items []pastReviewJSON `json:"items"`
	}{params.DecidedBy == nil, items})
}

// resubmitBlockJSON says who stopped a project being shipped again, and why.
type resubmitBlockJSON struct {
	At     time.Time `json:"at"`
	By     *string   `json:"by"`
	Reason *string   `json:"reason"`
}

func (a *API) resubmitBlock(r *http.Request, p db.Project) (*resubmitBlockJSON, error) {
	if p.ResubmitBlockedAt == nil {
		return nil, nil
	}
	out := &resubmitBlockJSON{At: *p.ResubmitBlockedAt, Reason: p.ResubmitBlockReason}
	if p.ResubmitBlockedBy != nil {
		by, err := a.Queries.GetUser(r.Context(), *p.ResubmitBlockedBy)
		if err != nil {
			return nil, err
		}
		out.By = &by.Name
	}
	return out, nil
}

// setResubmitBlock is PUT /review/projects/{id}/resubmit-block
// {"blocked": false} to let a project be shipped again, or
// {"blocked": true, "reason": "..."} to stop it. HQ and admins only.
func (a *API) setResubmitBlock(w http.ResponseWriter, r *http.Request) {
	viewer := auth.User(r.Context())
	if !canManageRejections(viewer) {
		respond.Error(w, http.StatusForbidden, "Only HQ and admins can change whether a project can be shipped again.")
		return
	}
	id, ok := idParam(w, r, "projectID")
	if !ok {
		return
	}
	var body struct {
		Blocked *bool  `json:"blocked"`
		Reason  string `json:"reason"`
	}
	if !decode(w, r, &body) {
		return
	}
	body.Reason = strings.TrimSpace(body.Reason)
	switch {
	case body.Blocked == nil:
		respond.Error(w, http.StatusUnprocessableEntity, "Say whether it's blocked: blocked has to be true or false.")
		return
	case *body.Blocked && body.Reason == "":
		respond.Error(w, http.StatusUnprocessableEntity, "Say why it can't be shipped again.")
		return
	case len(body.Reason) > 5000:
		respond.Error(w, http.StatusUnprocessableEntity, "Keep the reason under 5,000 characters.")
		return
	}
	project, err := a.Queries.GetAnyProject(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		respond.Error(w, http.StatusNotFound, "That project doesn't exist.")
		return
	}
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	if project.UserID == viewer.ID {
		respond.Error(w, http.StatusForbidden, "You can't change this for your own project.")
		return
	}
	switch {
	case *body.Blocked:
		err = a.Queries.BlockResubmit(r.Context(), db.BlockResubmitParams{ID: id, ResubmitBlockedBy: &viewer.ID, ResubmitBlockReason: &body.Reason})
	case project.ResubmitBlockedAt != nil:
		_, err = a.Queries.AllowResubmit(r.Context(), id)
	}
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	if project, err = a.Queries.GetAnyProject(r.Context(), id); err != nil {
		respond.ServerError(w, r, err)
		return
	}
	block, err := a.resubmitBlock(r, project)
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	respond.JSON(w, http.StatusOK, struct {
		ResubmitBlocked bool               `json:"resubmit_blocked"`
		ResubmitBlock   *resubmitBlockJSON `json:"resubmit_block"`
	}{block != nil, block})
}

// reopenSubmission is POST /review/submissions/{id}/reopen {"reason": "..."}:
// undoes a rejection, putting the ship back in the review queue as if it had
// just been shipped, and lets its project be shipped again. HQ and admins
// only. It isn't a review, so it doesn't count toward pay.
func (a *API) reopenSubmission(w http.ResponseWriter, r *http.Request) {
	viewer := auth.User(r.Context())
	if !canManageRejections(viewer) {
		respond.Error(w, http.StatusForbidden, "Only HQ and admins can undo a rejection.")
		return
	}
	id, ok := idParam(w, r, "submissionID")
	if !ok {
		return
	}
	var body struct {
		Reason string `json:"reason"`
	}
	if !decode(w, r, &body) {
		return
	}
	body.Reason = strings.TrimSpace(body.Reason)
	switch {
	case body.Reason == "":
		respond.Error(w, http.StatusUnprocessableEntity, "Say why you're undoing the rejection, for the reviewer who picks it up.")
		return
	case len(body.Reason) > 5000:
		respond.Error(w, http.StatusUnprocessableEntity, "Keep the reason under 5,000 characters.")
		return
	}
	if err := a.reopen(r, viewer, id, body.Reason); err != nil {
		respondProblem(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) reopen(r *http.Request, viewer *db.User, id int64, reason string) error {
	ctx := r.Context()
	row, err := a.Queries.GetSubmissionForReview(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return problem(http.StatusNotFound, "That submission doesn't exist.")
	}
	if err != nil {
		return err
	}
	if row.User.ID == viewer.ID {
		return problem(http.StatusForbidden, "You can't undo a rejection of your own project.")
	}
	tx, err := a.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := a.Queries.WithTx(tx)
	// Like a decision: one at a time per person, so what they've claimed
	// stays right.
	if _, err := q.LockUser(ctx, row.User.ID); err != nil {
		return err
	}
	s, err := q.LockSubmission(ctx, id)
	if err != nil {
		return err
	}
	if s.Status != db.SubmissionStatusRejected {
		return problem(http.StatusConflict, "It isn't rejected: it's %s.", s.Status)
	}
	// Its time went back to them when it was rejected. Once they've shipped
	// the project again, that time may be in the newer ship.
	newer, err := q.NewerSubmissionExists(ctx, db.NewerSubmissionExistsParams{ProjectID: s.ProjectID, ID: s.ID})
	if err != nil {
		return err
	}
	if newer {
		return problem(http.StatusConflict, "They've shipped this project again since, so review the newer ship instead.")
	}
	s = sentBack(s)
	if _, err := q.SaveSubmissionReview(ctx, db.SaveSubmissionReviewParams{
		ID: s.ID, Status: s.Status, ApprovedSeconds: s.ApprovedSeconds, Feedback: s.Feedback, InternalNotes: s.InternalNotes,
		ReviewerID: s.ReviewerID, ReviewedAt: s.ReviewedAt, Checks: s.Checks, FraudReasons: s.FraudReasons,
		FraudReviewerID: s.FraudReviewerID, FraudReviewedAt: s.FraudReviewedAt, HqReviewerID: s.HqReviewerID, HqReviewedAt: s.HqReviewedAt,
		OverrideReason: s.OverrideReason, TechnicalFeatures: s.TechnicalFeatures, DeflationReason: s.DeflationReason,
	}); err != nil {
		return err
	}
	if _, err := q.AddShipEvent(ctx, db.AddShipEventParams{SubmissionID: s.ID, Kind: db.ShipEventKindReopened,
		ActorID: &viewer.ID, Stage: db.SubmissionStatusRejected, Internal: &reason}); err != nil {
		return err
	}
	if _, err := q.AllowResubmit(ctx, s.ProjectID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	return tx.Commit(ctx)
}
