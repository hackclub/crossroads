package api

// POST /api/sidekick: the master endpoint for Sidekick, Hack Club's review
// platform (github.com/hackclub/sidekick, docs/PROTOCOL.md). Sidekick calls it
// to list and show ships and to send back reviewers' decisions; Crossroads
// keeps all the data. Decisions go through decide (stages.go), so Sidekick
// and the review pages follow exactly the same rules.
//
// How the stages look from Sidekick:
//   - pending                    → "pending"
//   - fraud_check, awaiting_hq   → "pending_hq" (fraud_check also gets a tag
//     and a failing check, since only a Crossroads fraud reviewer can move it on)
//   - approved, rejected         → the same
//
// Sidekick's "isHq" approvals skip HQ sign-off, like an approval on the
// review pages by someone who can sign off. The fraud check still happens
// first if anything flags it. Turn on "Report held approvals upstream" for
// Crossroads in Sidekick, so approvals arrive straight away and Crossroads'
// stages stay the one source of truth.
//
// Shop and fulfilment actions aren't implemented: orders are fulfilled
// through Airtable. They answer INVALID_ACTION, which Sidekick treats as "not
// supported".

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/scherepi/crossroads-backend/db"
	"github.com/scherepi/crossroads-backend/respond"
)

// sidekickError is the error body the protocol expects.
func sidekickError(w http.ResponseWriter, status int, code, message string) {
	respond.JSON(w, status, map[string]string{"error": code, "message": message})
}

// sidekickProblem answers with a reviewError's message, or as a server error.
func sidekickProblem(w http.ResponseWriter, r *http.Request, err error) {
	var p *reviewError
	if !errors.As(err, &p) {
		respond.ServerError(w, r, err) // logs it
		return
	}
	code := map[int]string{
		http.StatusNotFound: "NOT_FOUND", http.StatusConflict: "CONFLICT", http.StatusForbidden: "FORBIDDEN",
	}[p.Status]
	status := p.Status
	if code == "" {
		// 422 and the like: the protocol calls bad input a 400.
		code, status = "VALIDATION_ERROR", http.StatusBadRequest
	}
	sidekickError(w, status, code, p.Message)
}

func (a *API) sidekick(w http.ResponseWriter, r *http.Request) {
	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok || subtle.ConstantTimeCompare([]byte(token), []byte(a.SidekickSecret)) != 1 {
		sidekickError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid secret.")
		return
	}
	var req struct {
		Action string          `json:"action"`
		Input  json.RawMessage `json:"input"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		sidekickError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Couldn't read that request: "+err.Error())
		return
	}
	if len(req.Input) == 0 || string(req.Input) == "null" {
		req.Input = json.RawMessage("{}")
	}
	handlers := map[string]func(http.ResponseWriter, *http.Request, json.RawMessage){
		"HEALTH_CHECK":           a.skHealthCheck,
		"GET_PROGRAM_STATS":      a.skStats,
		"FETCH_PROJECTS":         a.skFetchProjects,
		"FETCH_PROJECT_DETAIL":   a.skFetchProjectDetail,
		"FETCH_PROJECT_TIMELINE": a.skFetchTimeline,
		"FETCH_AUTHOR_PROJECTS":  a.skFetchAuthorProjects,
		"FETCH_USER_NOTE":        a.skFetchUserNote,
		"UPDATE_USER_NOTE":       a.skUpdateUserNote,
		"SUBMIT_REVIEW_ACTION":   a.skSubmitReviewAction,
		"UPDATE_REVIEW_ACTION":   a.skUpdateReviewAction,
	}
	handler, ok := handlers[req.Action]
	if !ok {
		sidekickError(w, http.StatusBadRequest, "INVALID_ACTION",
			fmt.Sprintf("Crossroads doesn't support %s. Shop orders are fulfilled through Airtable.", req.Action))
		return
	}
	handler(w, r, req.Input)
}

// skInput decodes an action's input. Unknown fields are allowed: newer
// Sidekicks may send more. If it returns false, it has already responded.
func skInput(w http.ResponseWriter, input json.RawMessage, v any) bool {
	if err := json.Unmarshal(input, v); err != nil {
		sidekickError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Couldn't read the input: "+err.Error())
		return false
	}
	return true
}

func (a *API) skHealthCheck(w http.ResponseWriter, r *http.Request, _ json.RawMessage) {
	respond.JSON(w, http.StatusOK, map[string]any{"ok": true, "version": "crossroads"})
}

func (a *API) skStats(w http.ResponseWriter, r *http.Request, _ json.RawMessage) {
	stats, err := a.Queries.SidekickStats(r.Context())
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]int64{
		"pendingReviewCount": stats.PendingReview,
		"pendingHqCount":     stats.PendingHq,
		// Fulfilment happens in Airtable, not Sidekick.
		"pendingFulfillmentCount": 0,
	})
}

// Sidekick's ship statuses, and the Crossroads statuses behind each.
var skStatuses = map[string][]db.SubmissionStatus{
	"pending":    {db.SubmissionStatusPending},
	"pending_hq": {db.SubmissionStatusFraudCheck, db.SubmissionStatusAwaitingHq},
	"approved":   {db.SubmissionStatusApproved},
	"rejected":   {db.SubmissionStatusRejected},
	"all":        db.AllSubmissionStatusValues(),
}

func skStatus(s db.SubmissionStatus) string {
	switch s {
	case db.SubmissionStatusFraudCheck, db.SubmissionStatusAwaitingHq:
		return "pending_hq"
	}
	return string(s)
}

type skTag struct {
	Label string `json:"label"`
	Color string `json:"color,omitempty"`
}

type skField struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	Type        string `json:"type"`
	Placeholder string `json:"placeholder,omitempty"`
}

type skCheck struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	Severity string `json:"severity,omitempty"`
	Group    string `json:"group,omitempty"`
	Summary  string `json:"summary,omitempty"`
	URL      string `json:"url,omitempty"`
}

type skShip struct {
	ID             string    `json:"id"`
	HoursSubmitted float64   `json:"hoursSubmitted"`
	SubmittedAt    time.Time `json:"submittedAt"`
	Status         string    `json:"status"`
	ApproveFields  []skField `json:"approveFields,omitempty"`
	RejectFields   []skField `json:"rejectFields,omitempty"`
	Checks         []skCheck `json:"checks,omitempty"`
}

type skProject struct {
	ID                   string         `json:"id"`
	Title                string         `json:"title"`
	Description          string         `json:"description"`
	CodeURL              string         `json:"codeUrl"`
	DemoURL              *string        `json:"demoUrl,omitempty"`
	ScreenshotURL        *string        `json:"screenshotUrl,omitempty"`
	AuthorID             string         `json:"authorId"`
	AuthorHcaName        string         `json:"authorHcaName"`
	HackatimeID          *string        `json:"hackatimeId,omitempty"`
	HackatimeProjectKeys []string       `json:"hackatimeProjectKeys"`
	HackatimeStartDate   string         `json:"hackatimeStartDate"`
	Tags                 []skTag        `json:"tags"`
	PlatformURL          string         `json:"platformUrl,omitempty"`
	Ships                []skShip       `json:"ships"`
	Metadata             map[string]any `json:"metadata"`
}

// actorID is how Sidekick names a person: their Slack ID, or their Hack Club
// Auth identity ID if they have no Slack.
func actorID(slackID *string, hcaID string) string {
	if slackID != nil {
		return *slackID
	}
	return hcaID
}

func hoursFrom(seconds int32) float64 {
	return math.Round(float64(seconds)/36) / 100
}

func secondsFrom(hours float64) int32 {
	return int32(math.Round(hours * 3600))
}

// skProjects turns projects into Sidekick's shape, loading their ships and
// Hackatime links in one go.
func (a *API) skProjects(r *http.Request, projects []db.Project, owners []db.User) ([]skProject, error) {
	ids := make([]int64, len(projects))
	for i, p := range projects {
		ids[i] = p.ID
	}
	submissions, err := a.Queries.ListSubmissionsForProjects(r.Context(), ids)
	if err != nil {
		return nil, err
	}
	links, err := a.Queries.ListHackatimeLinksForProjects(r.Context(), ids)
	if err != nil {
		return nil, err
	}
	var shipIDs []int64
	for _, s := range submissions {
		shipIDs = append(shipIDs, s.ID)
	}
	priority, err := a.Queries.PriorityShipIDs(r.Context(), shipIDs)
	if err != nil {
		return nil, err
	}

	out := make([]skProject, len(projects))
	for i, p := range projects {
		owner := owners[i]
		sp := skProject{
			ID: strconv.FormatInt(p.ID, 10), Title: p.Name, Description: p.Description,
			CodeURL: *cmpOr(p.RepoUrl, new("")), DemoURL: p.DemoUrl, ScreenshotURL: p.ScreenshotUrl,
			AuthorID: actorID(owner.SlackID, owner.HcaID), AuthorHcaName: owner.Name, HackatimeID: owner.SlackID,
			HackatimeProjectKeys: []string{}, HackatimeStartDate: a.HoursStart.UTC().Format(time.DateOnly),
			Tags: []skTag{}, Ships: []skShip{}, Metadata: map[string]any{},
		}
		for _, l := range links {
			if l.ProjectID == p.ID {
				sp.HackatimeProjectKeys = append(sp.HackatimeProjectKeys, l.HackatimeProject)
			}
		}
		approvedBefore := false
		var latest *db.Submission
		for _, s := range submissions {
			if s.ProjectID != p.ID {
				continue
			}
			sp.Ships = append(sp.Ships, skShip{
				ID: strconv.FormatInt(s.ID, 10), HoursSubmitted: hoursFrom(s.NewSeconds), SubmittedAt: s.SubmittedAt, Status: skStatus(s.Status),
			})
			if inReview(s.Status) {
				if approvedBefore {
					sp.Tags = append(sp.Tags, skTag{"Update", "#3b82f6"})
				}
				if slices.Contains(priority, s.ID) {
					sp.Tags = append(sp.Tags, skTag{"Priority", "#f59e0b"})
				}
				switch s.Status {
				case db.SubmissionStatusFraudCheck:
					sp.Tags = append(sp.Tags, skTag{"Fraud check", "#ef4444"})
				case db.SubmissionStatusAwaitingHq:
					sp.Tags = append(sp.Tags, skTag{"Awaiting HQ", "#8b5cf6"})
				}
				sp.Metadata["crossroadsStatus"] = string(s.Status)
				sp.Metadata["fraudReasons"] = s.FraudReasons
			}
			if s.Status == db.SubmissionStatusApproved {
				approvedBefore = true
			}
			latest = &s
		}
		if !owner.YswsEligible {
			sp.Tags = append(sp.Tags, skTag{"Not verified", "#6b7280"})
		}
		if latest != nil && a.PublicURL != "" {
			sp.PlatformURL = strings.TrimSuffix(a.PublicURL, "/") + "/review/" + strconv.FormatInt(latest.ID, 10)
		}
		out[i] = sp
	}
	return out, nil
}

func cmpOr[T any](p, fallback *T) *T {
	if p == nil {
		return fallback
	}
	return p
}

func (a *API) skFetchProjects(w http.ResponseWriter, r *http.Request, input json.RawMessage) {
	var in struct {
		Status string `json:"status"`
		Cursor string `json:"cursor"`
		Limit  int    `json:"limit"`
	}
	if !skInput(w, input, &in) {
		return
	}
	statuses, ok := skStatuses[cmpOrString(in.Status, "all")]
	if !ok {
		sidekickError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Unknown status "+in.Status+".")
		return
	}
	limit := in.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	offset := 0
	if in.Cursor != "" {
		n, err := strconv.Atoi(strings.TrimPrefix(in.Cursor, "o"))
		if err != nil || n < 0 || n > math.MaxInt32 {
			sidekickError(w, http.StatusBadRequest, "VALIDATION_ERROR", "That cursor isn't one of ours.")
			return
		}
		offset = n
	}
	names := make([]string, len(statuses))
	for i, s := range statuses {
		names[i] = string(s)
	}
	rows, err := a.Queries.ListProjectsForSidekick(r.Context(), db.ListProjectsForSidekickParams{
		Statuses: names, PageSize: int32(limit), PageOffset: int32(offset),
	})
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	var projects []db.Project
	var owners []db.User
	var total int64
	for _, row := range rows {
		projects, owners, total = append(projects, row.Project), append(owners, row.User), row.Total
	}
	out, err := a.skProjects(r, projects, owners)
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	resp := map[string]any{"projects": out, "totalCount": total, "explicitlySorted": true}
	if offset+len(rows) < int(total) {
		resp["nextCursor"] = "o" + strconv.Itoa(offset+len(rows))
	}
	respond.JSON(w, http.StatusOK, resp)
}

func cmpOrString(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

// skProjectID reads a project ID. If it returns false, it has already
// responded.
func skParseID(w http.ResponseWriter, id, what string) (int64, bool) {
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		sidekickError(w, http.StatusNotFound, "NOT_FOUND", "No "+what+" with ID "+id+".")
		return 0, false
	}
	return n, true
}

func (a *API) skFetchProjectDetail(w http.ResponseWriter, r *http.Request, input json.RawMessage) {
	var in struct {
		ProjectID string `json:"projectId"`
	}
	if !skInput(w, input, &in) {
		return
	}
	id, ok := skParseID(w, in.ProjectID, "project")
	if !ok {
		return
	}
	row, err := a.Queries.GetProjectWithOwner(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		sidekickError(w, http.StatusNotFound, "NOT_FOUND", "No project with ID "+in.ProjectID+".")
		return
	}
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	projects, err := a.skProjects(r, []db.Project{row.Project}, []db.User{row.User})
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	project := projects[0]

	// The ship in review gets its checks and review fields.
	for i, ship := range project.Ships {
		shipID, _ := strconv.ParseInt(ship.ID, 10, 64)
		review, err := a.Queries.GetSubmissionForReview(r.Context(), shipID)
		if err != nil {
			respond.ServerError(w, r, err)
			return
		}
		if !inReview(review.Submission.Status) {
			continue
		}
		settings, err := a.Queries.GetReviewSettings(r.Context())
		if err != nil {
			respond.ServerError(w, r, err)
			return
		}
		report, err := a.runChecks(r.Context(), review, settings)
		if err != nil {
			respond.ServerError(w, r, err)
			return
		}
		ship.Checks = []skCheck{}
		if review.Submission.Status == db.SubmissionStatusFraudCheck {
			ship.Checks = append(ship.Checks, skCheck{
				ID: "stage", Name: "Waiting for Crossroads' fraud check", Status: "fail", Severity: "critical", Group: "Crossroads stages",
				Summary: "A Crossroads fraud reviewer has to pass this before it can be authorized. Flagged for: " +
					fraudReasonText(review.Submission.FraudReasons, report.Checks),
				URL: project.PlatformURL,
			})
		}
		for _, c := range report.Checks {
			group := "Crossroads checks"
			if c.Fraud {
				group = "Crossroads fraud signals"
			}
			ship.Checks = append(ship.Checks, skCheck{ID: c.ID, Name: c.Name, Status: c.Status, Severity: c.Severity, Group: group, Summary: c.Summary})
		}
		if review.Submission.Status == db.SubmissionStatusPending {
			ship.ApproveFields = []skField{{
				Name: "technical_features", Label: "Technical features", Type: "markdown",
				Placeholder: "Required. The specific features that account for these hours. Goes to the Unified YSWS DB.",
			}, {
				Name: "deflation_reason", Label: "Why deflated?", Type: "markdown",
				Placeholder: "Required if you approve fewer hours than the ship claimed. Goes to the Unified YSWS DB.",
			}, {
				Name: "send_to_fraud", Label: "Send to Crossroads' fraud check", Type: "boolean",
				Placeholder: "Even if no automatic check flags it.",
			}, {
				Name: "hours_override_reason", Label: "Why more hours than Hackatime tracked?", Type: "markdown",
				Placeholder: "Only if you're approving more than the hours submitted. HQ will have to sign it off.",
			}}
		}
		project.Ships[i] = ship
	}
	respond.JSON(w, http.StatusOK, project)
}

func (a *API) skFetchAuthorProjects(w http.ResponseWriter, r *http.Request, input json.RawMessage) {
	var in struct {
		AuthorID         string `json:"authorId"`
		ExcludeProjectID string `json:"excludeProjectId"`
	}
	if !skInput(w, input, &in) {
		return
	}
	author, err := a.Queries.GetUserByActorID(r.Context(), in.AuthorID)
	if errors.Is(err, pgx.ErrNoRows) {
		respond.JSON(w, http.StatusOK, map[string]any{"projects": []skProject{}})
		return
	}
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	exclude, _ := strconv.ParseInt(in.ExcludeProjectID, 10, 64)
	rows, err := a.Queries.ListProjectsWithOwnerByUser(r.Context(), db.ListProjectsWithOwnerByUserParams{UserID: author.ID, ExcludeProjectID: exclude})
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	var projects []db.Project
	var owners []db.User
	for _, row := range rows {
		projects, owners = append(projects, row.Project), append(owners, row.User)
	}
	out, err := a.skProjects(r, projects, owners)
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{"projects": out})
}

// skEvent is one Sidekick timeline event. Which fields are set depends on
// Type.
type skEvent struct {
	Type               string     `json:"type"`
	ID                 string     `json:"id,omitempty"`
	ShipID             string     `json:"shipId,omitempty"`
	ActorID            string     `json:"actorId,omitempty"`
	DiscardedByActorID string     `json:"discardedByActorId,omitempty"`
	HoursSubmitted     *float64   `json:"hoursSubmitted,omitempty"`
	HoursAssigned      *float64   `json:"hoursAssigned,omitempty"`
	FeedbackMessage    *string    `json:"feedbackMessage,omitempty"`
	Justification      *string    `json:"justification,omitempty"`
	InternalMessage    *string    `json:"internalMessage,omitempty"`
	Message            *string    `json:"message,omitempty"`
	IsInternal         *bool      `json:"isInternal,omitempty"`
	DisplayFields      []skField2 `json:"displayFields,omitempty"`
	Source             *skSource  `json:"source,omitempty"`
	Summary            string     `json:"summary,omitempty"`
	Tone               string     `json:"tone,omitempty"`
	Blocks             []skBlock  `json:"blocks,omitempty"`
	Timestamp          time.Time  `json:"timestamp"`
}

type skField2 struct {
	Label      string `json:"label"`
	Value      string `json:"value"`
	IsInternal bool   `json:"isInternal,omitempty"`
}

type skSource struct {
	Name string `json:"name"`
}

type skBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// skEventFrom turns one of Crossroads' ship events into Sidekick's shape.
func skEventFrom(e db.ShipEvent, actor string, actorName string) skEvent {
	out := skEvent{ShipID: strconv.FormatInt(e.SubmissionID, 10), ActorID: actor, Timestamp: e.CreatedAt}
	str := func(p *string) *string {
		if p == nil {
			return new("")
		}
		return p
	}
	hours := func() *float64 {
		if e.Seconds == nil {
			return nil
		}
		return new(hoursFrom(*e.Seconds))
	}
	switch e.Kind {
	case db.ShipEventKindComment:
		out.Type, out.Message, out.IsInternal = "comment", e.Message, new(false)
	case db.ShipEventKindInternalComment:
		out.Type, out.Message, out.IsInternal = "comment", e.Internal, new(true)
	case db.ShipEventKindApproval:
		out.Type, out.HoursAssigned, out.FeedbackMessage, out.Justification = "approval", hours(), str(e.Message), str(e.Internal)
	case db.ShipEventKindRejection:
		out.Type, out.FeedbackMessage, out.InternalMessage = "rejection", str(e.Message), e.Internal
	default:
		// Things only Crossroads does show up as system events.
		out.Type, out.ActorID, out.ID, out.Source = "system", "", fmt.Sprintf("crossroads:%d", e.ID), &skSource{"Crossroads"}
		switch e.Kind {
		case db.ShipEventKindFlagged:
			out.Summary, out.Tone = "sent this ship to the fraud check", "warning"
		case db.ShipEventKindFraudPass:
			out.Summary, out.Tone = fmt.Sprintf("fraud check passed by %s at %.2fh", actorName, *hours()), "success"
		case db.ShipEventKindFinalApproval:
			out.Summary, out.Tone = fmt.Sprintf("HQ sign-off by %s at %.2fh", actorName, *hours()), "success"
		case db.ShipEventKindSentBack:
			out.Summary, out.Tone = "sent back to review by "+actorName, "warning"
		}
		for _, text := range []*string{e.Message, e.Internal} {
			if text != nil {
				out.Blocks = append(out.Blocks, skBlock{Type: "section", Text: *text})
			}
		}
	}
	return out
}

func (a *API) skFetchTimeline(w http.ResponseWriter, r *http.Request, input json.RawMessage) {
	var in struct {
		ProjectID string `json:"projectId"`
	}
	if !skInput(w, input, &in) {
		return
	}
	id, ok := skParseID(w, in.ProjectID, "project")
	if !ok {
		return
	}
	project, err := a.Queries.GetProjectWithOwner(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		sidekickError(w, http.StatusNotFound, "NOT_FOUND", "No project with ID "+in.ProjectID+".")
		return
	}
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	submissions, err := a.Queries.ListSubmissionsForProjects(r.Context(), []int64{id})
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	rows, err := a.Queries.ListProjectShipEvents(r.Context(), id)
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	author := actorID(project.User.SlackID, project.User.HcaID)
	var events []skEvent
	for _, s := range submissions {
		events = append(events, skEvent{
			Type: "ship", ShipID: strconv.FormatInt(s.ID, 10), ActorID: author, HoursSubmitted: new(hoursFrom(s.NewSeconds)),
			DisplayFields: []skField2{{Label: "Hackatime projects", Value: strings.Join(s.HackatimeProjects, ", ")}},
			Timestamp:     s.SubmittedAt,
		})
	}
	// An approval that was later sent back is shown as discarded, attributed
	// to whoever sent it back.
	lastApproval := map[int64]int{}
	for _, row := range rows {
		e := row.ShipEvent
		actor := ""
		if row.ActorHcaID != nil {
			actor = actorID(row.ActorSlackID, *row.ActorHcaID)
		}
		if e.Kind == db.ShipEventKindSentBack {
			if i, ok := lastApproval[e.SubmissionID]; ok {
				approval := events[i]
				approval.Type, approval.ID, approval.DiscardedByActorID = "discarded_approval", fmt.Sprintf("sent_back:%d", e.ID), actor
				approval.Timestamp = e.CreatedAt
				events[i] = approval
				delete(lastApproval, e.SubmissionID)
			}
			events = append(events, skEvent{Type: "comment", ShipID: strconv.FormatInt(e.SubmissionID, 10), ActorID: actor,
				Message: new("Sent back to review: " + *cmpOr(e.Internal, new(""))), IsInternal: new(true), Timestamp: e.CreatedAt})
			continue
		}
		if e.Kind == db.ShipEventKindApproval {
			lastApproval[e.SubmissionID] = len(events)
		}
		events = append(events, skEventFrom(e, actor, *cmpOr(row.ActorName, new(""))))
	}
	slices.SortStableFunc(events, func(x, y skEvent) int { return x.Timestamp.Compare(y.Timestamp) })
	if events == nil {
		events = []skEvent{}
	}
	respond.JSON(w, http.StatusOK, map[string]any{"events": events})
}

func (a *API) skFetchUserNote(w http.ResponseWriter, r *http.Request, input json.RawMessage) {
	var in struct {
		UserID string `json:"userId"`
	}
	if !skInput(w, input, &in) {
		return
	}
	user, err := a.Queries.GetUserByActorID(r.Context(), in.UserID)
	if errors.Is(err, pgx.ErrNoRows) {
		respond.JSON(w, http.StatusOK, map[string]any{"note": nil})
		return
	}
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{"note": user.ReviewerNote})
}

func (a *API) skUpdateUserNote(w http.ResponseWriter, r *http.Request, input json.RawMessage) {
	var in struct {
		UserID string  `json:"userId"`
		Note   *string `json:"note"`
	}
	if !skInput(w, input, &in) {
		return
	}
	user, err := a.Queries.GetUserByActorID(r.Context(), in.UserID)
	if errors.Is(err, pgx.ErrNoRows) {
		sidekickError(w, http.StatusNotFound, "NOT_FOUND", "Crossroads doesn't know "+in.UserID+".")
		return
	}
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	note := strings.TrimSpace(*cmpOr(in.Note, new("")))
	if len(note) > 5000 {
		sidekickError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Keep the note under 5,000 characters.")
		return
	}
	if _, err := a.Queries.SetReviewerNote(r.Context(), db.SetReviewerNoteParams{ID: user.ID, ReviewerNote: nonEmpty(note)}); err != nil {
		respond.ServerError(w, r, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]bool{"success": true})
}

// skReviewer finds the Crossroads account behind a Sidekick reviewer. They
// need one so decisions are credited to someone.
func (a *API) skReviewer(r *http.Request, actor string) (*db.User, error) {
	user, err := a.Queries.GetUserByActorID(r.Context(), actor)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, problem(http.StatusUnprocessableEntity,
			"%s has never signed in to Crossroads. Sign in once at %s so your reviews can be credited to you, then try again.", actor, cmpOrString(a.PublicURL, "the Crossroads site"))
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (a *API) skSubmitReviewAction(w http.ResponseWriter, r *http.Request, input json.RawMessage) {
	var in struct {
		ShipID                string         `json:"shipId"`
		ReviewerID            string         `json:"reviewerId"`
		Action                string         `json:"action"`
		HoursAssigned         *float64       `json:"hoursAssigned"`
		RewardedHoursOverride *float64       `json:"rewardedHoursOverride"`
		FeedbackMessage       string         `json:"feedbackMessage"`
		Justification         string         `json:"justification"`
		InternalMessage       string         `json:"internalMessage"`
		CommentText           string         `json:"commentText"`
		Message               string         `json:"message"`
		IsHq                  bool           `json:"isHq"`
		Fields                map[string]any `json:"fields"`
	}
	if !skInput(w, input, &in) {
		return
	}
	if in.RewardedHoursOverride != nil {
		sidekickError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Crossroads doesn't use rewarded hours overrides.")
		return
	}
	shipID, ok := skParseID(w, in.ShipID, "ship")
	if !ok {
		return
	}
	reviewer, err := a.skReviewer(r, in.ReviewerID)
	if err != nil {
		sidekickProblem(w, r, err)
		return
	}
	var seconds *int32
	if in.HoursAssigned != nil {
		seconds = new(secondsFrom(*in.HoursAssigned))
	}
	ship, err := a.Queries.GetSubmissionForReview(r.Context(), shipID)
	if errors.Is(err, pgx.ErrNoRows) {
		sidekickError(w, http.StatusNotFound, "NOT_FOUND", "No ship with ID "+in.ShipID+".")
		return
	}
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	status := ship.Submission.Status

	var d decision
	var perms permissions
	switch in.Action {
	case "comment", "internal_comment":
		e, err := a.addComment(r.Context(), reviewer, shipID, in.CommentText, in.Action == "internal_comment")
		if err != nil {
			sidekickProblem(w, r, err)
			return
		}
		respond.JSON(w, http.StatusOK, map[string]any{"success": true, "event": skEventFrom(e, in.ReviewerID, reviewer.Name)})
		return
	case "approve":
		if status != db.SubmissionStatusPending {
			sidekickError(w, http.StatusConflict, "CONFLICT", "This ship has already been approved; it's at "+stageLabels[status]+" on Crossroads.")
			return
		}
		sendToFraud, _ := in.Fields["send_to_fraud"].(bool)
		overrideReason, _ := in.Fields["hours_override_reason"].(string)
		features, _ := in.Fields["technical_features"].(string)
		deflation, _ := in.Fields["deflation_reason"].(string)
		d = decision{Action: "approve", Stage: status, ApprovedSeconds: seconds, Feedback: in.FeedbackMessage,
			InternalNotes: in.Justification, SendToFraud: sendToFraud, OverrideReason: overrideReason,
			TechnicalFeatures: features, DeflationReason: deflation}
		perms = permissions{Review: true, HQReview: in.IsHq}
	case "reject":
		d = decision{Action: "reject", Stage: status, Feedback: in.FeedbackMessage, InternalNotes: in.InternalMessage}
		// Anyone can reject a ship waiting for its first review; later stages
		// take HQ powers.
		perms = permissions{Review: true, FraudReview: in.IsHq, HQReview: in.IsHq}
	case "authorize":
		if status == db.SubmissionStatusFraudCheck {
			sidekickError(w, http.StatusConflict, "CONFLICT",
				"This ship is waiting for Crossroads' fraud check, which has to pass it first: "+
					strings.TrimSuffix(a.PublicURL, "/")+"/review/"+in.ShipID)
			return
		}
		// Authorizing has no fields of its own: if HQ cuts the hours, the
		// justification says why.
		d = decision{Action: "approve", Stage: db.SubmissionStatusAwaitingHq, ApprovedSeconds: seconds, InternalNotes: in.Justification}
		if seconds != nil && ship.Submission.ApprovedSeconds != nil && *seconds < *ship.Submission.ApprovedSeconds {
			d.DeflationReason = in.Justification
		}
		perms = permissions{HQReview: true}
	case "deauthorize":
		if status != db.SubmissionStatusFraudCheck && status != db.SubmissionStatusAwaitingHq {
			sidekickError(w, http.StatusConflict, "CONFLICT", "Only approvals waiting for sign-off can be sent back.")
			return
		}
		d = decision{Action: "send_back", Stage: status, InternalNotes: cmpOrString(in.Message, "Sent back from Sidekick.")}
		perms = permissions{FraudReview: true, HQReview: true}
	default:
		sidekickError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Unknown review action "+in.Action+".")
		return
	}
	out, err := a.decide(r.Context(), reviewer, shipID, d, perms)
	if err != nil {
		sidekickProblem(w, r, err)
		return
	}
	event := skEventFrom(out.Event, in.ReviewerID, reviewer.Name)
	if out.Event.Kind == db.ShipEventKindSentBack {
		event = skEvent{Type: "comment", ShipID: in.ShipID, ActorID: in.ReviewerID, Message: new("Sent back to review: " + d.InternalNotes),
			IsInternal: new(true), Timestamp: out.Event.CreatedAt}
	}
	respond.JSON(w, http.StatusOK, map[string]any{"success": true, "event": event})
}

func (a *API) skUpdateReviewAction(w http.ResponseWriter, r *http.Request, input json.RawMessage) {
	var in struct {
		ShipID                string   `json:"shipId"`
		ReviewerID            string   `json:"reviewerId"`
		Type                  string   `json:"type"`
		FeedbackMessage       string   `json:"feedbackMessage"`
		Justification         *string  `json:"justification"`
		InternalMessage       *string  `json:"internalMessage"`
		HoursAssigned         *float64 `json:"hoursAssigned"`
		RewardedHoursOverride *float64 `json:"rewardedHoursOverride"`
	}
	if !skInput(w, input, &in) {
		return
	}
	if in.RewardedHoursOverride != nil {
		sidekickError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Crossroads doesn't use rewarded hours overrides.")
		return
	}
	shipID, ok := skParseID(w, in.ShipID, "ship")
	if !ok {
		return
	}
	reviewer, err := a.skReviewer(r, in.ReviewerID)
	if err != nil {
		sidekickProblem(w, r, err)
		return
	}
	var seconds *int32
	if in.HoursAssigned != nil {
		seconds = new(secondsFrom(*in.HoursAssigned))
	}
	switch in.Type {
	case "approval":
		err = a.editDecision(r.Context(), reviewer, shipID, db.ShipEventKindApproval, in.FeedbackMessage, in.Justification, seconds)
	case "rejection":
		err = a.editDecision(r.Context(), reviewer, shipID, db.ShipEventKindRejection, in.FeedbackMessage, in.InternalMessage, seconds)
	default:
		sidekickError(w, http.StatusBadRequest, "VALIDATION_ERROR", `type has to be "approval" or "rejection".`)
		return
	}
	if err != nil {
		sidekickProblem(w, r, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]bool{"success": true})
}
