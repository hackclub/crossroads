package api

// The review pages' routes: the three queues, claiming, the review page, and
// decisions, comments, notes and feedback templates. The rules for each stage
// are in stages.go.

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/scherepi/crossroads-backend/auth"
	"github.com/scherepi/crossroads-backend/db"
	"github.com/scherepi/crossroads-backend/github"
	"github.com/scherepi/crossroads-backend/respond"
	"github.com/scherepi/crossroads-backend/unified"
)

// respondProblem answers with err's message if it's a reviewError, and as a
// server error otherwise.
func respondProblem(w http.ResponseWriter, r *http.Request, err error) {
	var p *reviewError
	if errors.As(err, &p) {
		respond.Error(w, p.Status, p.Message)
		return
	}
	respond.ServerError(w, r, err)
}

func toReviewerSubmissionJSON(s db.Submission) submissionJSON {
	out := toSubmissionJSON(s)
	out.Status = string(s.Status) // the real stage, not the participant's view of it
	out.InternalNotes = s.InternalNotes
	out.ReviewerID = s.ReviewerID
	out.Checks = nullableRaw(s.Checks)
	out.FraudReasons = s.FraudReasons
	out.FraudReviewerID = s.FraudReviewerID
	out.FraudReviewedAt = s.FraudReviewedAt
	out.HQReviewerID = s.HqReviewerID
	out.HQReviewedAt = s.HqReviewedAt
	out.OverrideReason = s.OverrideReason
	return out
}

func nullableRaw(b []byte) json.RawMessage {
	if len(b) == 0 {
		return nil
	}
	return b
}

type personJSON struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type claimJSON struct {
	UserID    int64     `json:"user_id"`
	Name      string    `json:"name"`
	ExpiresAt time.Time `json:"expires_at"`
}

// activeClaim is the claim on a ship, or nil if there's none or it ran out.
func activeClaim(claimedBy *int64, expires *time.Time, name string) *claimJSON {
	if claimedBy == nil || expires == nil || !expires.After(time.Now()) {
		return nil
	}
	return &claimJSON{UserID: *claimedBy, Name: name, ExpiresAt: *expires}
}

type queueItemJSON struct {
	ID                int64      `json:"id"` // submission ID
	ProjectID         int64      `json:"project_id"`
	ProjectName       string     `json:"project_name"`
	UserID            int64      `json:"user_id"`
	UserName          string     `json:"user_name"`
	SlackID           *string    `json:"slack_id"`
	Status            string     `json:"status"`
	HackatimeProjects []string   `json:"hackatime_projects"`
	NewSeconds        int32      `json:"new_seconds"`
	ApprovedSeconds   *int32     `json:"approved_seconds"`
	SubmittedAt       time.Time  `json:"submitted_at"`
	ReviewedAt        *time.Time `json:"reviewed_at"`
	ReviewerName      *string    `json:"reviewer_name"`
	FraudReasons      []string   `json:"fraud_reasons"`
	OverrideReason    *string    `json:"override_reason"` // set when it's approved for more than Hackatime tracked
	Priority          bool       `json:"priority"`
	Claim             *claimJSON `json:"claim"`
	SkippedByMe       bool       `json:"skipped_by_me"`
	Mine              bool       `json:"mine"` // their own project: they can't review it
}

type queueCountsJSON struct {
	Review int64 `json:"review"`
	Fraud  int64 `json:"fraud"`
	HQ     int64 `json:"hq"`
}

func (a *API) queueCounts(r *http.Request) (queueCountsJSON, error) {
	var counts queueCountsJSON
	rows, err := a.Queries.CountOpenSubmissions(r.Context())
	for _, row := range rows {
		switch row.Status {
		case db.SubmissionStatusPending:
			counts.Review = row.Count
		case db.SubmissionStatusFraudCheck:
			counts.Fraud = row.Count
		case db.SubmissionStatusAwaitingHq:
			counts.HQ = row.Count
		}
	}
	return counts, err
}

// stageParam reads a queue name ("review", "fraud" or "hq"). If it returns
// false, it has already responded.
func stageParam(w http.ResponseWriter, name string) (db.SubmissionStatus, bool) {
	if name == "" {
		name = "review"
	}
	status, ok := stageStatuses[name]
	if !ok {
		respond.Error(w, http.StatusUnprocessableEntity, `stage has to be "review", "fraud" or "hq".`)
	}
	return status, ok
}

// reviewQueue is GET /review/queue?stage=review|fraud|hq: the ships at one
// stage, priority review first, then oldest first, and how many are at each.
func (a *API) reviewQueue(w http.ResponseWriter, r *http.Request) {
	viewer := auth.User(r.Context())
	status, ok := stageParam(w, r.URL.Query().Get("stage"))
	if !ok {
		return
	}
	rows, err := a.Queries.ListReviewQueue(r.Context(), db.ListReviewQueueParams{ViewerID: viewer.ID, Status: status})
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	counts, err := a.queueCounts(r)
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	items := []queueItemJSON{}
	for _, row := range rows {
		items = append(items, queueItemJSON{
			ID:                row.ID,
			ProjectID:         row.ProjectID,
			ProjectName:       row.ProjectName,
			UserID:            row.UserID,
			UserName:          row.UserName,
			SlackID:           row.SlackID,
			Status:            string(row.Status),
			HackatimeProjects: row.HackatimeProjects,
			NewSeconds:        row.NewSeconds,
			ApprovedSeconds:   row.ApprovedSeconds,
			SubmittedAt:       row.SubmittedAt,
			ReviewedAt:        row.ReviewedAt,
			ReviewerName:      nonEmpty(row.ReviewerName),
			FraudReasons:      row.FraudReasons,
			OverrideReason:    row.OverrideReason,
			Priority:          row.Priority,
			Claim:             activeClaim(row.ClaimedBy, row.ClaimExpiresAt, row.ClaimedByName),
			SkippedByMe:       row.Skipped,
			Mine:              row.UserID == viewer.ID,
		})
	}
	respond.JSON(w, http.StatusOK, struct {
		Counts queueCountsJSON `json:"counts"`
		Items  []queueItemJSON `json:"items"`
	}{counts, items})
}

// claimNext is POST /review/next {"stage": "review"}: claims the next ship at
// a stage and says which, or 204 if there's nothing left.
func (a *API) claimNext(w http.ResponseWriter, r *http.Request) {
	reviewer := auth.User(r.Context())
	var body struct {
		Stage string `json:"stage"`
	}
	if !decode(w, r, &body) {
		return
	}
	status, ok := stageParam(w, body.Stage)
	if !ok {
		return
	}
	if !permissionsOf(reviewer).canDecide(status) {
		respond.Error(w, http.StatusForbidden, "You can't decide ships at "+stageLabels[status]+".")
		return
	}
	s, err := a.Queries.ClaimNextSubmission(r.Context(), db.ClaimNextSubmissionParams{UserID: &reviewer.ID, Status: status})
	if errors.Is(err, pgx.ErrNoRows) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	respond.JSON(w, http.StatusOK, struct {
		SubmissionID int64      `json:"submission_id"`
		Claim        *claimJSON `json:"claim"`
	}{s.ID, activeClaim(s.ClaimedBy, s.ClaimExpiresAt, reviewer.Name)})
}

// claimSubmission is POST /review/submissions/{id}/claim: claims a ship, or
// renews the claim, for 30 minutes.
func (a *API) claimSubmission(w http.ResponseWriter, r *http.Request) {
	reviewer := auth.User(r.Context())
	id, ok := idParam(w, r, "submissionID")
	if !ok {
		return
	}
	row, err := a.Queries.GetSubmissionForReview(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		respond.Error(w, http.StatusNotFound, "That submission doesn't exist.")
		return
	}
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	switch {
	case row.User.ID == reviewer.ID:
		respond.Error(w, http.StatusForbidden, "You can't review your own project.")
		return
	case !inReview(row.Submission.Status):
		respond.Error(w, http.StatusConflict, "This submission has already been "+string(row.Submission.Status)+".")
		return
	case !permissionsOf(reviewer).canDecide(row.Submission.Status):
		respond.Error(w, http.StatusForbidden, "You can't decide ships at "+stageLabels[row.Submission.Status]+".")
		return
	}
	s, err := a.Queries.ClaimSubmission(r.Context(), db.ClaimSubmissionParams{UserID: &reviewer.ID, ID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		// Someone else holds it, or it was decided a moment ago.
		latest, err := a.Queries.GetSubmissionForReview(r.Context(), id)
		if err != nil {
			respond.ServerError(w, r, err)
			return
		}
		if err := checkClaim(r.Context(), a.Queries, latest.Submission, reviewer); err != nil {
			respondProblem(w, r, err)
			return
		}
		respond.Error(w, http.StatusConflict, "That ship just moved on. Reload to see what changed.")
		return
	}
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	respond.JSON(w, http.StatusOK, activeClaim(s.ClaimedBy, s.ClaimExpiresAt, reviewer.Name))
}

// releaseClaim is DELETE /review/submissions/{id}/claim: lets go of a ship.
// Admins can let go of anyone's claim.
func (a *API) releaseClaim(w http.ResponseWriter, r *http.Request) {
	reviewer := auth.User(r.Context())
	id, ok := idParam(w, r, "submissionID")
	if !ok {
		return
	}
	if _, err := a.Queries.ReleaseClaim(r.Context(), db.ReleaseClaimParams{
		ID: id, UserID: &reviewer.ID, Anyone: reviewer.Role == db.UserRoleAdmin,
	}); err != nil {
		respond.ServerError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// skipSubmission is POST /review/submissions/{id}/skip: lets go of a ship and
// keeps "Next" from handing it to them again for 3 hours.
func (a *API) skipSubmission(w http.ResponseWriter, r *http.Request) {
	reviewer := auth.User(r.Context())
	id, ok := idParam(w, r, "submissionID")
	if !ok {
		return
	}
	if _, err := a.Queries.ReleaseClaim(r.Context(), db.ReleaseClaimParams{ID: id, UserID: &reviewer.ID}); err != nil {
		respond.ServerError(w, r, err)
		return
	}
	err := a.Queries.SkipSubmission(r.Context(), db.SkipSubmissionParams{UserID: reviewer.ID, SubmissionID: id})
	if db.IsViolation(err, "review_skips_submission_id_fkey") {
		respond.Error(w, http.StatusNotFound, "That submission doesn't exist.")
		return
	}
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type reviewUserJSON struct {
	ID                 int64   `json:"id"`
	Name               string  `json:"name"`
	Email              string  `json:"email"`
	SlackID            *string `json:"slack_id"`
	VerificationStatus string  `json:"verification_status"`
	YSWSEligible       bool    `json:"ysws_eligible"`
	ReviewerNote       *string `json:"reviewer_note"`
	// Whether they're in the Smugglers' Emporium, and why, so an admin can
	// let them in for a project worth it.
	InEmporium   bool    `json:"in_emporium"`
	EmporiumNote *string `json:"emporium_note"`
}

type sharedLinkProjectJSON struct {
	ID       int64   `json:"id"`
	Name     string  `json:"name"`
	RepoURL  *string `json:"repo_url"`
	DemoURL  *string `json:"demo_url"`
	UserID   int64   `json:"user_id"`
	UserName string  `json:"user_name"`
}

type reviewHackatimeJSON struct {
	// Time since the event started on the submission's Hackatime projects,
	// as of now rather than when it was submitted.
	Seconds   int                    `json:"seconds"`
	Projects  []hackatimeProjectJSON `json:"projects"`
	Convicted bool                   `json:"convicted"`
}

// shipEventJSON is one thing on a project's timeline, as reviewers see it.
type shipEventJSON struct {
	ID           int64       `json:"id"`
	SubmissionID int64       `json:"submission_id"`
	Kind         string      `json:"kind"`
	Stage        string      `json:"stage"` // the ship's status when it happened
	Actor        *personJSON `json:"actor"` // null for automatic events
	Seconds      *int32      `json:"seconds"`
	Message      *string     `json:"message"`  // what the participant sees
	Internal     *string     `json:"internal"` // reviewers only
	// Why this approval was lower than what came in, if it was.
	DeflationReason *string    `json:"deflation_reason"`
	CreatedAt       time.Time  `json:"created_at"`
	EditedAt        *time.Time `json:"edited_at"`
}

func toShipEventJSON(row db.ListProjectShipEventsRow) shipEventJSON {
	e := row.ShipEvent
	out := shipEventJSON{
		ID: e.ID, SubmissionID: e.SubmissionID, Kind: string(e.Kind), Stage: string(e.Stage),
		Seconds: e.Seconds, Message: e.Message, Internal: e.Internal, DeflationReason: e.DeflationReason,
		CreatedAt: e.CreatedAt, EditedAt: e.EditedAt,
	}
	if e.ActorID != nil {
		out.Actor = &personJSON{ID: *e.ActorID, Name: *row.ActorName}
	}
	return out
}

// getSubmissionForReview shows a reviewer everything about a ship: the
// project and its earlier ships, its whole timeline, who made it, what
// Hackatime says now, the automatic checks, and who's holding it.
func (a *API) getSubmissionForReview(w http.ResponseWriter, r *http.Request) {
	viewer := auth.User(r.Context())
	id, ok := idParam(w, r, "submissionID")
	if !ok {
		return
	}
	row, err := a.Queries.GetSubmissionForReview(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		respond.Error(w, http.StatusNotFound, "That submission doesn't exist.")
		return
	}
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	links, err := a.Queries.ListProjectHackatimeLinks(r.Context(), row.Project.ID)
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	submissions, err := a.Queries.ListSubmissions(r.Context(), row.Project.ID)
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	project := toProjectJSON(row.Project, links, submissions)
	for i, s := range submissions {
		project.Submissions[i] = toReviewerSubmissionJSON(s)
	}
	settings, err := a.Queries.GetReviewSettings(r.Context())
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	report, err := a.runChecks(r.Context(), row, settings)
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	eventRows, err := a.Queries.ListProjectShipEvents(r.Context(), row.Project.ID)
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	events := []shipEventJSON{}
	for _, e := range eventRows {
		events = append(events, toShipEventJSON(e))
	}
	preview, err := a.unifiedPreview(r.Context(), row, eventRows, submissions)
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	s := row.Submission
	var claim *claimJSON
	if s.ClaimedBy != nil {
		holder, err := a.Queries.GetUser(r.Context(), *s.ClaimedBy)
		if err != nil {
			respond.ServerError(w, r, err)
			return
		}
		claim = activeClaim(s.ClaimedBy, s.ClaimExpiresAt, holder.Name)
	}

	// Whether the viewer can decide it right now, and if not, why.
	var cantDecide string
	switch {
	case !inReview(s.Status):
		cantDecide = "It's been " + string(s.Status) + "."
	case row.User.ID == viewer.ID:
		cantDecide = "It's your own project."
	case !permissionsOf(viewer).canDecide(s.Status):
		cantDecide = "You can't decide ships at " + stageLabels[s.Status] + "."
	case claim != nil && claim.UserID != viewer.ID:
		cantDecide = claim.Name + " is reviewing this right now."
	}

	respond.JSON(w, http.StatusOK, struct {
		Submission submissionJSON  `json:"submission"`
		Stage      string          `json:"stage"` // review, fraud or hq; empty once it's decided
		Project    projectJSON     `json:"project"`
		User       reviewUserJSON  `json:"user"`
		Events     []shipEventJSON `json:"events"` // the project's timeline, oldest first
		Checks     []check         `json:"checks"` // run just now
		// Other projects, anyone's, with the same repo or demo link: a sign
		// of a copied project, or one project split up to be paid twice.
		ProjectsSharingLinks []sharedLinkProjectJSON `json:"projects_sharing_links"`
		Hackatime            *reviewHackatimeJSON    `json:"hackatime"`
		HackatimeError       string                  `json:"hackatime_error,omitempty"`
		// The repo on GitHub, and its commits during this ship; null when the
		// repo isn't on GitHub, or GitHub couldn't be asked (GitHubError).
		GitHub      *github.Report `json:"github"`
		GitHubError string         `json:"github_error,omitempty"`
		// This project in the Unified YSWS DB under other programs; null when
		// the Unified YSWS DB isn't being searched.
		UnifiedRecords []unified.Record `json:"unified_records"`
		// Hackatime time and commits per day since WindowStart: the last
		// approved ship, or the event's start. Daily totals, never heartbeats.
		Days          []dayJSON `json:"days"`
		WindowStart   time.Time `json:"window_start"`
		TelescreenURL string    `json:"telescreen_url,omitempty"`
		// What the Airtable sync will send to the Unified YSWS DB; null until
		// a reviewer approves it.
		UnifiedPreview   *unifiedPreviewJSON `json:"unified_preview"`
		Claim            *claimJSON          `json:"claim"`
		CanDecide        bool                `json:"can_decide"`
		CantDecideReason string              `json:"cant_decide_reason,omitempty"`
	}{
		Submission: toReviewerSubmissionJSON(s),
		Stage:      stageName(s.Status),
		Project:    project,
		User: reviewUserJSON{
			ID: row.User.ID, Name: row.User.Name, Email: row.User.Email, SlackID: row.User.SlackID,
			VerificationStatus: row.User.VerificationStatus, YSWSEligible: row.User.YswsEligible, ReviewerNote: row.User.ReviewerNote,
			InEmporium: row.User.InEmporium(), EmporiumNote: row.User.EmporiumNote,
		},
		Events:               events,
		Checks:               report.Checks,
		ProjectsSharingLinks: report.SharedLinks,
		Hackatime:            report.Hackatime,
		HackatimeError:       report.HackatimeError,
		GitHub:               report.GitHub,
		GitHubError:          report.GitHubError,
		UnifiedRecords:       report.Unified,
		Days:                 report.Days,
		WindowStart:          report.WindowStart,
		TelescreenURL:        a.telescreenURL(row.User.SlackID),
		UnifiedPreview:       preview,
		Claim:                claim,
		CanDecide:            cantDecide == "",
		CantDecideReason:     cantDecide,
	})
}

// reviewSubmission is POST /review/submissions/{id}: a decision at whatever
// stage the ship is at (see decision in stages.go).
func (a *API) reviewSubmission(w http.ResponseWriter, r *http.Request) {
	reviewer := auth.User(r.Context())
	id, ok := idParam(w, r, "submissionID")
	if !ok {
		return
	}
	var d decision
	if !decode(w, r, &d) {
		return
	}
	out, err := a.decide(r.Context(), reviewer, id, d, permissionsOf(reviewer))
	if err != nil {
		respondProblem(w, r, err)
		return
	}
	boxes := []boxJSON{}
	for _, b := range out.Boxes {
		boxes = append(boxes, toBoxJSON(b, nil))
	}
	respond.JSON(w, http.StatusOK, struct {
		Submission   submissionJSON `json:"submission"`
		StampsPaid   int64          `json:"stamps_paid"`
		BoxesGranted []boxJSON      `json:"boxes_granted"`
	}{toReviewerSubmissionJSON(out.Submission), out.StampsPaid, boxes})
}

// commentOnSubmission is POST /review/submissions/{id}/comments
// {"message": "...", "internal": true}.
func (a *API) commentOnSubmission(w http.ResponseWriter, r *http.Request) {
	author := auth.User(r.Context())
	id, ok := idParam(w, r, "submissionID")
	if !ok {
		return
	}
	var body struct {
		Message  string `json:"message"`
		Internal bool   `json:"internal"`
	}
	if !decode(w, r, &body) {
		return
	}
	e, err := a.addComment(r.Context(), author, id, body.Message, body.Internal)
	if err != nil {
		respondProblem(w, r, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toShipEventJSON(db.ListProjectShipEventsRow{ShipEvent: e, ActorName: &author.Name}))
}

// setReviewerNote is PUT /review/users/{id}/note {"note": "..."}: the note
// about a person that reviewers see on all their ships. "" clears it.
func (a *API) setReviewerNote(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "userID")
	if !ok {
		return
	}
	var body struct {
		Note string `json:"note"`
	}
	if !decode(w, r, &body) {
		return
	}
	body.Note = strings.TrimSpace(body.Note)
	if len(body.Note) > 5000 {
		respond.Error(w, http.StatusUnprocessableEntity, "Keep the note under 5,000 characters.")
		return
	}
	user, err := a.Queries.SetReviewerNote(r.Context(), db.SetReviewerNoteParams{ID: id, ReviewerNote: nonEmpty(body.Note)})
	if errors.Is(err, pgx.ErrNoRows) {
		respond.Error(w, http.StatusNotFound, "That user doesn't exist.")
		return
	}
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	respond.JSON(w, http.StatusOK, struct {
		Note *string `json:"note"`
	}{user.ReviewerNote})
}

// stampsFor is how many stamps a running total of approved time is worth.
func (a *API) stampsFor(approvedSeconds int64) int64 {
	return approvedSeconds * int64(a.StampsPerHour) / 3600
}

func nonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// telescreenURL is the person's Telescreen profile, for fraud reviewers.
func (a *API) telescreenURL(slackID *string) string {
	if a.TelescreenProfileURL == "" || slackID == nil {
		return ""
	}
	return strings.ReplaceAll(a.TelescreenProfileURL, "{slack_id}", url.PathEscape(*slackID))
}
