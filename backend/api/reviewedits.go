package api

// Reviewers fixing a project's demo link and screenshot, so a ship that only
// needs a better link doesn't have to be rejected and shipped again.

import (
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/scherepi/crossroads-backend/auth"
	"github.com/scherepi/crossroads-backend/db"
	"github.com/scherepi/crossroads-backend/respond"
)

// reviewerEditProject is PATCH /review/projects/{id}
// {"demo_url": "https://...", "screenshot_url": "https://..."}: anyone who
// can review changes a shipped project's demo link or screenshot, even while
// it's in review. Fields left out aren't changed; neither can be emptied.
// Each change goes on the timeline of the project's latest ship as a comment
// the maker sees, saying what it was before. Not a review: it isn't paid.
func (a *API) reviewerEditProject(w http.ResponseWriter, r *http.Request) {
	reviewer := auth.User(r.Context())
	if p := permissionsOf(reviewer); !p.Review && !p.FraudReview && !p.HQReview {
		respond.Error(w, http.StatusForbidden, "Only reviewers can change a project's links.")
		return
	}
	id, ok := idParam(w, r, "projectID")
	if !ok {
		return
	}
	var body struct {
		DemoURL       *string `json:"demo_url"`
		ScreenshotURL *string `json:"screenshot_url"`
	}
	if !decode(w, r, &body) {
		return
	}
	for _, v := range []*string{body.DemoURL, body.ScreenshotURL} {
		if v != nil && strings.TrimSpace(*v) == "" {
			respond.Error(w, http.StatusUnprocessableEntity, "Give a link: a shipped project needs a demo and a screenshot.")
			return
		}
	}

	found, err := a.Queries.GetAnyProject(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		respond.Error(w, http.StatusNotFound, "That project doesn't exist.")
		return
	}
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	if found.UserID == reviewer.ID {
		respond.Error(w, http.StatusForbidden, "You can't change your own project from here. Edit it on its page.")
		return
	}

	tx, err := a.Pool.Begin(r.Context())
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())
	q := a.Queries.WithTx(tx)
	if _, err := q.LockUser(r.Context(), found.UserID); err != nil {
		respond.ServerError(w, r, err)
		return
	}
	project, err := q.LockProject(r.Context(), db.LockProjectParams{ID: id, UserID: found.UserID})
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	submissions, err := q.ListSubmissions(r.Context(), project.ID)
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	if len(submissions) == 0 {
		respond.Error(w, http.StatusConflict, "This project hasn't been shipped, so there's nothing to review.")
		return
	}
	before := project
	if problem := (projectFields{DemoURL: body.DemoURL, ScreenshotURL: body.ScreenshotURL}).apply(&project); problem != "" {
		respond.Error(w, http.StatusUnprocessableEntity, problem)
		return
	}

	// What changed, in words the maker sees on their project.
	var changes []string
	if deref(before.DemoUrl) != deref(project.DemoUrl) {
		changes = append(changes, "the demo link to "+deref(project.DemoUrl)+" (it was "+orNone(before.DemoUrl)+")")
	}
	if deref(before.ScreenshotUrl) != deref(project.ScreenshotUrl) {
		changes = append(changes, "the screenshot")
	}
	if len(changes) > 0 {
		project, err = q.UpdateProject(r.Context(), db.UpdateProjectParams{
			ID: project.ID, UserID: project.UserID, Name: project.Name, Description: project.Description,
			RepoUrl: project.RepoUrl, DemoUrl: project.DemoUrl, ScreenshotUrl: project.ScreenshotUrl,
		})
		if err != nil {
			respond.ServerError(w, r, err)
			return
		}
		message := "A reviewer changed " + strings.Join(changes, " and ") + "."
		if _, err := q.AddShipEvent(r.Context(), db.AddShipEventParams{SubmissionID: submissions[0].ID, Kind: db.ShipEventKindComment,
			ActorID: &reviewer.ID, Stage: submissions[0].Status, Message: &message}); err != nil {
			respond.ServerError(w, r, err)
			return
		}
	}
	links, err := q.ListProjectHackatimeLinks(r.Context(), project.ID)
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		respond.ServerError(w, r, err)
		return
	}
	respond.JSON(w, http.StatusOK, toProjectJSON(project, links, submissions))
}

func orNone(s *string) string {
	if s == nil || *s == "" {
		return "empty"
	}
	return *s
}
