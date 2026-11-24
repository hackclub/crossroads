package api

// Past reviews and what can be done about rejections afterwards.
//
// A rejection is soft or hard. Soft: they change the project and ship it
// again. Hard (decision.BlockResubmit, for fraud): the project can't be
// shipped again until HQ or an admin allows it, and every ticket it earned is
// forfeit. HQ and admins see every rejected ship, can switch a project between
// the two, and can put a rejected ship back in the review queue. Undoing a
// hard reject can give the forfeited tickets back.

import (
	"context"
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
	ForfeitedStamps int64     `json:"forfeited_stamps"` // taken back by the hard reject, if it's one
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
			Feedback: row.Feedback, ResubmitBlocked: row.ResubmitBlocked, ForfeitedStamps: row.ForfeitedStamps, ShippedAgain: row.ShippedAgain,
		})
	}
	respond.JSON(w, http.StatusOK, struct {
		All   bool             `json:"all"` // everyone's, not just the viewer's
		Items []pastReviewJSON `json:"items"`
	}{params.DecidedBy == nil, items})
}

// resubmitBlockJSON says who stopped a project being shipped again, and why,
// and how many of its tickets that took back.
type resubmitBlockJSON struct {
	At              time.Time `json:"at"`
	By              *string   `json:"by"`
	Reason          *string   `json:"reason"`
	ForfeitedStamps int64     `json:"forfeited_stamps"`
}

func (a *API) resubmitBlock(r *http.Request, p db.Project) (*resubmitBlockJSON, error) {
	if p.ResubmitBlockedAt == nil {
		return nil, nil
	}
	out := &resubmitBlockJSON{At: *p.ResubmitBlockedAt, Reason: p.ResubmitBlockReason}
	forfeited, err := a.Queries.ForfeitedStampsForProject(r.Context(), p.ID)
	if err != nil {
		return nil, err
	}
	out.ForfeitedStamps = forfeited
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
// {"blocked": false} to let a project be shipped again (with
// "restore_stamps": true, giving back the tickets the hard reject took), or
// {"blocked": true, "reason": "..."} to stop it, which takes them. HQ and
// admins only.
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
		Blocked       *bool  `json:"blocked"`
		Reason        string `json:"reason"`
		RestoreStamps bool   `json:"restore_stamps"`
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
		err = a.blockResubmit(r.Context(), viewer, project, body.Reason)
	case project.ResubmitBlockedAt != nil:
		err = a.allowResubmit(r.Context(), viewer, project, body.RestoreStamps)
	}
	if err != nil {
		respondProblem(w, r, err)
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

// blockResubmit stops a project being shipped again, by hand, and forfeits
// its tickets. A ship in review has to be hard-rejected instead, so nothing
// is paid on a project after its tickets were taken.
func (a *API) blockResubmit(ctx context.Context, viewer *db.User, project db.Project, reason string) error {
	tx, err := a.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := a.Queries.WithTx(tx)
	if _, err := q.LockUser(ctx, project.UserID); err != nil {
		return err
	}
	// Blocking first locks the project, so it can't be shipped meanwhile.
	if err := q.BlockResubmit(ctx, db.BlockResubmitParams{ID: project.ID, ResubmitBlockedBy: &viewer.ID, ResubmitBlockReason: &reason}); err != nil {
		return err
	}
	submissions, err := q.ListSubmissions(ctx, project.ID)
	if err != nil {
		return err
	}
	if hasPending(submissions) {
		return problem(http.StatusConflict, "It has a ship in review. Hard-reject that ship instead.")
	}
	if _, err := forfeitStamps(ctx, q, project.ID, viewer.ID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// allowResubmit lets a hard-rejected project be shipped again, giving back
// the tickets it forfeited if restore is set, and DMs its owner that it can
// be.
func (a *API) allowResubmit(ctx context.Context, viewer *db.User, project db.Project, restore bool) error {
	tx, err := a.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := a.Queries.WithTx(tx)
	if _, err := q.LockUser(ctx, project.UserID); err != nil {
		return err
	}
	project, err = q.AllowResubmit(ctx, project.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil // someone else just did
	}
	if err != nil {
		return err
	}
	var restored int64
	if restore {
		if restored, err = restoreStamps(ctx, q, project.ID, viewer.ID); err != nil {
			return err
		}
	}
	if err := addProjectDM(ctx, q, project, nil, db.ProjectDmKindResubmitAllowed, a.resubmitAllowedDM(project, restored)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// forfeitStamps takes back every ticket a hard-rejected project has earned,
// as by, and returns how many. Its owner's row has to be locked.
func forfeitStamps(ctx context.Context, q *db.Queries, projectID, by int64) (int64, error) {
	entries, err := q.ForfeitProjectStamps(ctx, db.ForfeitProjectStampsParams{ProjectID: projectID, CreatedBy: by})
	return -ledgerTotal(entries), err
}

// restoreStamps gives back the tickets a project forfeited, as by, and
// returns how many. Its owner's row has to be locked.
func restoreStamps(ctx context.Context, q *db.Queries, projectID, by int64) (int64, error) {
	entries, err := q.RestoreProjectStamps(ctx, db.RestoreProjectStampsParams{ProjectID: projectID, CreatedBy: by})
	return ledgerTotal(entries), err
}

func ledgerTotal(entries []db.StampLedger) int64 {
	var total int64
	for _, e := range entries {
		total += int64(e.Amount)
	}
	return total
}

// reopenSubmission is POST /review/submissions/{id}/reopen {"reason": "..."}:
// undoes a rejection, putting the ship back in the review queue as if it had
// just been shipped, and lets its project be shipped again. With
// "restore_stamps": true, it gives back the tickets a hard reject took. HQ
// and admins only. It isn't a review, so it doesn't count toward pay.
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
		Reason        string `json:"reason"`
		RestoreStamps bool   `json:"restore_stamps"`
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
	if err := a.reopen(r, viewer, id, body.Reason, body.RestoreStamps); err != nil {
		respondProblem(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) reopen(r *http.Request, viewer *db.User, id int64, reason string, restore bool) error {
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
		ReturnedTo: s.ReturnedTo,
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
	var restored int64
	if restore {
		if restored, err = restoreStamps(ctx, q, s.ProjectID, viewer.ID); err != nil {
			return err
		}
	}
	// One DM, even if it was a hard reject: being back in review says it
	// can be shipped again.
	if err := addProjectDM(ctx, q, row.Project, &s.ID, db.ProjectDmKindReopened, a.reopenedDM(row.Project, restored)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
