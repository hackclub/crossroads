package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/scherepi/crossroads-backend/auth"
	"github.com/scherepi/crossroads-backend/db"
	"github.com/scherepi/crossroads-backend/hackatime"
	"github.com/scherepi/crossroads-backend/respond"
)

type projectJSON struct {
	ID                int64     `json:"id"`
	Name              string    `json:"name"`
	Description       string    `json:"description"`
	RepoURL           *string   `json:"repo_url"`
	DemoURL           *string   `json:"demo_url"`
	ScreenshotURL     *string   `json:"screenshot_url"`
	HackatimeProjects []string  `json:"hackatime_projects"`
	ApprovedSeconds   int       `json:"approved_seconds"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`

	// Only on GET /projects/{id}, fresh from Hackatime. If Hackatime couldn't
	// be asked, HackatimeError says why instead.
	HackatimeSeconds   *int   `json:"hackatime_seconds,omitempty"`
	UnsubmittedSeconds *int   `json:"unsubmitted_seconds,omitempty"`
	HackatimeError     string `json:"hackatime_error,omitempty"`

	Submissions []submissionJSON `json:"submissions"` // newest first

	// Only on GET /projects/{id}: special stamps stuck on it. Nothing
	// private, so it can be shown to anyone viewing the project.
	Stamps []placedStampJSON `json:"stamps,omitempty"`
}

type submissionJSON struct {
	ID                int64               `json:"id"`
	Status            db.SubmissionStatus `json:"status"`
	HackatimeProjects []string            `json:"hackatime_projects"`
	HackatimeSeconds  int32               `json:"hackatime_seconds"`
	NewSeconds        int32               `json:"new_seconds"`
	ApprovedSeconds   *int32              `json:"approved_seconds"`
	Feedback          *string             `json:"feedback"`
	SubmittedAt       time.Time           `json:"submitted_at"`
	ReviewedAt        *time.Time          `json:"reviewed_at"`

	// Only filled in for reviewers.
	InternalNotes *string `json:"internal_notes,omitempty"`
	ReviewerID    *int64  `json:"reviewer_id,omitempty"`
}

func toProjectJSON(project db.Project, links []string, submissions []db.Submission) projectJSON {
	p := projectJSON{
		ID:                project.ID,
		Name:              project.Name,
		Description:       project.Description,
		RepoURL:           project.RepoUrl,
		DemoURL:           project.DemoUrl,
		ScreenshotURL:     project.ScreenshotUrl,
		HackatimeProjects: links,
		CreatedAt:         project.CreatedAt,
		UpdatedAt:         project.UpdatedAt,
		Submissions:       []submissionJSON{},
	}
	if p.HackatimeProjects == nil {
		p.HackatimeProjects = []string{}
	}
	for _, s := range submissions {
		if s.Status == db.SubmissionStatusApproved {
			p.ApprovedSeconds += int(*s.ApprovedSeconds)
		}
		p.Submissions = append(p.Submissions, toSubmissionJSON(s))
	}
	return p
}

func toSubmissionJSON(s db.Submission) submissionJSON {
	return submissionJSON{
		ID:                s.ID,
		Status:            s.Status,
		HackatimeProjects: s.HackatimeProjects,
		HackatimeSeconds:  s.HackatimeSeconds,
		NewSeconds:        s.NewSeconds,
		ApprovedSeconds:   s.ApprovedSeconds,
		Feedback:          s.Feedback,
		SubmittedAt:       s.SubmittedAt,
		ReviewedAt:        s.ReviewedAt,
	}
}

// accountedFor looks at a project's submissions and returns how much of its
// Hackatime time its approved submissions have covered, including time a
// reviewer didn't accept, and every Hackatime project paid out on it.
func accountedFor(submissions []db.Submission) (seconds int, paid map[string]bool) {
	paid = map[string]bool{}
	for _, s := range submissions {
		if s.Status != db.SubmissionStatusApproved {
			continue
		}
		seconds += int(s.NewSeconds)
		for _, name := range s.HackatimeProjects {
			paid[name] = true
		}
	}
	return seconds, paid
}

// hackatimeTotals is what Hackatime says about a project and its owner.
type hackatimeTotals struct {
	Project int // on the project's Hackatime projects
	// On every Hackatime project its owner has linked to any of their
	// projects, with time that counted toward several counted once.
	User      int
	Convicted bool
}

// unclaimed is how much of a project's time can still be submitted: what's
// been added since its approved submissions, but never more than its owner has
// left unclaimed across all their projects. The second limit is what stops
// time that counted toward two projects at once, or that moved to a renamed
// Hackatime project, from being paid twice.
func (t hackatimeTotals) unclaimed(projectAccountedFor, userClaimed int) int {
	return max(0, min(t.Project-projectAccountedFor, t.User-userClaimed))
}

func linkNames(links []db.ProjectHackatimeProject) []string {
	names := make([]string, len(links))
	for i, link := range links {
		names[i] = link.HackatimeProject
	}
	return names
}

func hasPending(submissions []db.Submission) bool {
	return slices.ContainsFunc(submissions, func(s db.Submission) bool {
		return s.Status == db.SubmissionStatusPending
	})
}

// projectFields is the body of POST and PATCH /projects. Fields left out
// aren't changed. A URL set to "" is cleared. hackatime_projects replaces the
// whole list.
type projectFields struct {
	Name              *string  `json:"name"`
	Description       *string  `json:"description"`
	RepoURL           *string  `json:"repo_url"`
	DemoURL           *string  `json:"demo_url"`
	ScreenshotURL     *string  `json:"screenshot_url"`
	HackatimeProjects []string `json:"hackatime_projects"`
}

// apply copies the fields that were sent onto project. It returns a problem to
// show the person if the result isn't valid.
func (f projectFields) apply(project *db.Project) string {
	if f.Name != nil {
		project.Name = strings.TrimSpace(*f.Name)
	}
	if f.Description != nil {
		project.Description = strings.TrimSpace(*f.Description)
	}
	for _, link := range []struct {
		label string
		value *string
		field **string
	}{
		{"The repo link", f.RepoURL, &project.RepoUrl},
		{"The demo link", f.DemoURL, &project.DemoUrl},
		{"The screenshot link", f.ScreenshotURL, &project.ScreenshotUrl},
	} {
		if link.value == nil {
			continue
		}
		value := strings.TrimSpace(*link.value)
		if value == "" {
			*link.field = nil
			continue
		}
		if !isWebURL(value) {
			return link.label + " has to start with http:// or https://."
		}
		*link.field = &value
	}
	switch {
	case project.Name == "":
		return "Give the project a name."
	case len(project.Name) > 100:
		return "Keep the name under 100 characters."
	case len(project.Description) > 5000:
		return "Keep the description under 5,000 characters."
	}
	return ""
}

func isWebURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" && len(s) <= 2048
}

// cleanHackatimeProjects trims, dedupes and sorts names, and returns a problem
// to show the person if any won't work.
func cleanHackatimeProjects(names []string) ([]string, string) {
	var clean []string
	for _, name := range names {
		name = strings.TrimSpace(name)
		switch {
		case name == "" || slices.Contains(clean, name):
			continue
		case strings.Contains(name, ","):
			return nil, "Hackatime project names can't contain commas."
		case len(name) > 200:
			return nil, "That Hackatime project name is too long."
		}
		clean = append(clean, name)
	}
	if len(clean) > 20 {
		return nil, "Link at most 20 Hackatime projects."
	}
	slices.Sort(clean)
	return clean, ""
}

// setHackatimeLinks makes project's linked Hackatime projects exactly want. It
// returns a problem to show the person if a link can't be added or removed.
func setHackatimeLinks(ctx context.Context, q *db.Queries, project db.Project, want []string, paid map[string]bool) (string, error) {
	have, err := q.ListProjectHackatimeLinks(ctx, project.ID)
	if err != nil {
		return "", err
	}
	for _, name := range have {
		if slices.Contains(want, name) {
			continue
		}
		if paid[name] {
			return fmt.Sprintf("%s has already been paid out on this project, so it can't be unlinked.", name), nil
		}
		if err := q.UnlinkHackatimeProject(ctx, db.UnlinkHackatimeProjectParams{UserID: project.UserID, HackatimeProject: name}); err != nil {
			return "", err
		}
	}
	for _, name := range want {
		if slices.Contains(have, name) {
			continue
		}
		err := q.LinkHackatimeProject(ctx, db.LinkHackatimeProjectParams{UserID: project.UserID, HackatimeProject: name, ProjectID: project.ID})
		if db.IsViolation(err, "project_hackatime_projects_pkey") {
			return fmt.Sprintf("%s already counts toward another of your projects.", name), nil
		}
		if err != nil {
			return "", err
		}
	}
	return "", nil
}

func (a *API) listProjects(w http.ResponseWriter, r *http.Request) {
	user := auth.User(r.Context())
	projects, err := a.Queries.ListProjects(r.Context(), user.ID)
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	links, err := a.Queries.ListHackatimeLinks(r.Context(), user.ID)
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	submissions, err := a.Queries.ListSubmissionsForUser(r.Context(), user.ID)
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}

	linksByProject := map[int64][]string{}
	for _, link := range links {
		linksByProject[link.ProjectID] = append(linksByProject[link.ProjectID], link.HackatimeProject)
	}
	submissionsByProject := map[int64][]db.Submission{}
	for _, s := range submissions {
		submissionsByProject[s.ProjectID] = append(submissionsByProject[s.ProjectID], s)
	}
	out := []projectJSON{}
	for _, project := range projects {
		out = append(out, toProjectJSON(project, linksByProject[project.ID], submissionsByProject[project.ID]))
	}
	respond.JSON(w, http.StatusOK, out)
}

func (a *API) getProject(w http.ResponseWriter, r *http.Request) {
	user := auth.User(r.Context())
	id, ok := idParam(w, r, "projectID")
	if !ok {
		return
	}
	project, err := a.Queries.GetProject(r.Context(), db.GetProjectParams{ID: id, UserID: user.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		respond.Error(w, http.StatusNotFound, "That project doesn't exist.")
		return
	}
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	links, err := a.Queries.ListProjectHackatimeLinks(r.Context(), project.ID)
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	submissions, err := a.Queries.ListSubmissions(r.Context(), project.ID)
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	out := toProjectJSON(project, links, submissions)
	if out.Stamps, err = a.projectStamps(r, a.Queries, project.ID); err != nil {
		respond.ServerError(w, r, err)
		return
	}

	if len(links) > 0 {
		userLinks, err := a.Queries.ListHackatimeLinks(r.Context(), user.ID)
		if err != nil {
			respond.ServerError(w, r, err)
			return
		}
		claimed, err := a.Queries.ClaimedSecondsForUser(r.Context(), user.ID)
		if err != nil {
			respond.ServerError(w, r, err)
			return
		}
		if totals, problem := a.hackatimeTotals(r, user, links, linkNames(userLinks)); problem != "" {
			out.HackatimeError = problem
		} else {
			accounted, _ := accountedFor(submissions)
			unsubmitted := totals.unclaimed(accounted, int(claimed))
			out.HackatimeSeconds = &totals.Project
			out.UnsubmittedSeconds = &unsubmitted
		}
	}
	respond.JSON(w, http.StatusOK, out)
}

func (a *API) createProject(w http.ResponseWriter, r *http.Request) {
	user := auth.User(r.Context())
	var fields projectFields
	if !decode(w, r, &fields) {
		return
	}
	var project db.Project
	if problem := fields.apply(&project); problem != "" {
		respond.Error(w, http.StatusUnprocessableEntity, problem)
		return
	}
	links, problem := cleanHackatimeProjects(fields.HackatimeProjects)
	if problem != "" {
		respond.Error(w, http.StatusUnprocessableEntity, problem)
		return
	}

	tx, err := a.Pool.Begin(r.Context())
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())
	q := a.Queries.WithTx(tx)

	if _, err := q.LockUser(r.Context(), user.ID); err != nil {
		respond.ServerError(w, r, err)
		return
	}
	project, err = q.CreateProject(r.Context(), db.CreateProjectParams{
		UserID:        user.ID,
		Name:          project.Name,
		Description:   project.Description,
		RepoUrl:       project.RepoUrl,
		DemoUrl:       project.DemoUrl,
		ScreenshotUrl: project.ScreenshotUrl,
	})
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	if problem, err := setHackatimeLinks(r.Context(), q, project, links, nil); err != nil {
		respond.ServerError(w, r, err)
		return
	} else if problem != "" {
		respond.Error(w, http.StatusConflict, problem)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		respond.ServerError(w, r, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toProjectJSON(project, links, nil))
}

func (a *API) updateProject(w http.ResponseWriter, r *http.Request) {
	user := auth.User(r.Context())
	id, ok := idParam(w, r, "projectID")
	if !ok {
		return
	}
	var fields projectFields
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

	if _, err := q.LockUser(r.Context(), user.ID); err != nil {
		respond.ServerError(w, r, err)
		return
	}
	project, err := q.LockProject(r.Context(), db.LockProjectParams{ID: id, UserID: user.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		respond.Error(w, http.StatusNotFound, "That project doesn't exist.")
		return
	}
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	submissions, err := q.ListSubmissions(r.Context(), project.ID)
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	// Reviewers have to see the project as it was submitted.
	if hasPending(submissions) {
		respond.Error(w, http.StatusConflict, "This project is waiting for review, so it can't be changed until that's done.")
		return
	}

	if problem := fields.apply(&project); problem != "" {
		respond.Error(w, http.StatusUnprocessableEntity, problem)
		return
	}
	project, err = q.UpdateProject(r.Context(), db.UpdateProjectParams{
		ID:            project.ID,
		UserID:        user.ID,
		Name:          project.Name,
		Description:   project.Description,
		RepoUrl:       project.RepoUrl,
		DemoUrl:       project.DemoUrl,
		ScreenshotUrl: project.ScreenshotUrl,
	})
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	if fields.HackatimeProjects != nil {
		links, problem := cleanHackatimeProjects(fields.HackatimeProjects)
		if problem != "" {
			respond.Error(w, http.StatusUnprocessableEntity, problem)
			return
		}
		_, paid := accountedFor(submissions)
		if problem, err := setHackatimeLinks(r.Context(), q, project, links, paid); err != nil {
			respond.ServerError(w, r, err)
			return
		} else if problem != "" {
			respond.Error(w, http.StatusConflict, problem)
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

func (a *API) deleteProject(w http.ResponseWriter, r *http.Request) {
	user := auth.User(r.Context())
	id, ok := idParam(w, r, "projectID")
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

	if _, err := q.LockUser(r.Context(), user.ID); err != nil {
		respond.ServerError(w, r, err)
		return
	}
	project, err := q.LockProject(r.Context(), db.LockProjectParams{ID: id, UserID: user.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		respond.Error(w, http.StatusNotFound, "That project doesn't exist.")
		return
	}
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	submissions, err := q.ListSubmissions(r.Context(), project.ID)
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	if len(submissions) > 0 {
		respond.Error(w, http.StatusConflict, "Projects that have been submitted can't be deleted.")
		return
	}
	if err := q.DeleteProject(r.Context(), db.DeleteProjectParams{ID: project.ID, UserID: user.ID}); err != nil {
		respond.ServerError(w, r, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		respond.ServerError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// submit sends a project for review, counting the Hackatime time that hasn't
// been claimed yet (see hackatimeTotals.unclaimed). The body can name a
// special stamp to use on it: {"special_stamp_id": 1}.
func (a *API) submit(w http.ResponseWriter, r *http.Request) {
	user := auth.User(r.Context())
	id, ok := idParam(w, r, "projectID")
	if !ok {
		return
	}
	var body struct {
		SpecialStampID *int64 `json:"special_stamp_id"`
	}
	if !decode(w, r, &body) {
		return
	}
	project, err := a.Queries.GetProject(r.Context(), db.GetProjectParams{ID: id, UserID: user.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		respond.Error(w, http.StatusNotFound, "That project doesn't exist.")
		return
	}
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	links, err := a.Queries.ListProjectHackatimeLinks(r.Context(), project.ID)
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	var missing []string
	if project.Description == "" {
		missing = append(missing, "a description")
	}
	if project.RepoUrl == nil {
		missing = append(missing, "a repo link")
	}
	if project.DemoUrl == nil {
		missing = append(missing, "a demo link")
	}
	if project.ScreenshotUrl == nil {
		missing = append(missing, "a screenshot")
	}
	if len(links) == 0 {
		missing = append(missing, "at least one Hackatime project")
	}
	if len(missing) > 0 {
		respond.Error(w, http.StatusUnprocessableEntity, "Before submitting, add "+joinList(missing)+".")
		return
	}

	userLinks, err := a.Queries.ListHackatimeLinks(r.Context(), user.ID)
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}

	// Ask Hackatime before taking any locks; it can be slow.
	totals, problem := a.hackatimeTotals(r, user, links, linkNames(userLinks))
	if problem != "" {
		respond.Error(w, http.StatusUnprocessableEntity, problem)
		return
	}
	if totals.Convicted {
		respond.Error(w, http.StatusForbidden, "Hackatime has flagged your account, so you can't submit right now.")
		return
	}

	tx, err := a.Pool.Begin(r.Context())
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())
	q := a.Queries.WithTx(tx)

	// Lock the person so their submissions go one at a time and none of their
	// projects or links can change underneath this one.
	if _, err := q.LockUser(r.Context(), user.ID); err != nil {
		respond.ServerError(w, r, err)
		return
	}
	_, err = q.LockProject(r.Context(), db.LockProjectParams{ID: project.ID, UserID: user.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		respond.Error(w, http.StatusNotFound, "That project doesn't exist.")
		return
	}
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	lockedLinks, err := q.ListHackatimeLinks(r.Context(), user.ID)
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	if !slices.Equal(lockedLinks, userLinks) {
		respond.Error(w, http.StatusConflict, "Your Hackatime projects just changed. Please try again.")
		return
	}
	submissions, err := q.ListSubmissions(r.Context(), project.ID)
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	if hasPending(submissions) {
		respond.Error(w, http.StatusConflict, "This project is already waiting for review.")
		return
	}
	accounted, _ := accountedFor(submissions)
	claimed, err := q.ClaimedSecondsForUser(r.Context(), user.ID)
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	newSeconds := totals.unclaimed(accounted, int(claimed))
	if newSeconds == 0 {
		if totals.Project > accounted {
			respond.Error(w, http.StatusUnprocessableEntity, "This project's new time was also counted on another of your projects, which already claimed it, so there's nothing new to submit.")
		} else {
			respond.Error(w, http.StatusUnprocessableEntity, "There's no new Hackatime time on this project since it was last approved.")
		}
		return
	}

	if body.SpecialStampID != nil {
		stamp, err := q.LockSpecialStamp(r.Context(), db.LockSpecialStampParams{ID: *body.SpecialStampID, UserID: user.ID})
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			respond.Error(w, http.StatusNotFound, "You don't have that special stamp.")
			return
		case err != nil:
			respond.ServerError(w, r, err)
			return
		case stamp.SpecialStamp.SubmissionID != nil:
			respond.Error(w, http.StatusConflict, "You've already used that special stamp.")
			return
		case stamp.Effect == db.SpecialStampEffectNone:
			respond.Error(w, http.StatusUnprocessableEntity, "That special stamp is just for collecting. It doesn't do anything on a submission.")
			return
		}
	}

	submission, err := q.CreateSubmission(r.Context(), db.CreateSubmissionParams{
		ProjectID:         project.ID,
		HackatimeProjects: links,
		HackatimeSeconds:  int32(totals.Project),
		NewSeconds:        int32(newSeconds),
	})
	if db.IsViolation(err, "submissions_one_pending_per_project") {
		respond.Error(w, http.StatusConflict, "This project is already waiting for review.")
		return
	}
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	if body.SpecialStampID != nil {
		if err := q.ApplySpecialStamp(r.Context(), db.ApplySpecialStampParams{ID: *body.SpecialStampID, SubmissionID: &submission.ID}); err != nil {
			respond.ServerError(w, r, err)
			return
		}
	} else {
		// An Express stamp stuck on the project while nothing was in review
		// gives this ship priority.
		waiting, err := q.WaitingPriorityStamp(r.Context(), &project.ID)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
		case err != nil:
			respond.ServerError(w, r, err)
			return
		default:
			if err := q.ApplySpecialStamp(r.Context(), db.ApplySpecialStampParams{ID: waiting, SubmissionID: &submission.ID}); err != nil {
				respond.ServerError(w, r, err)
				return
			}
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		respond.ServerError(w, r, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toSubmissionJSON(submission))
}

type hackatimeProjectJSON struct {
	Name      string `json:"name"`
	Seconds   int    `json:"seconds"`
	ProjectID *int64 `json:"project_id"` // the project it counts toward, if any
}

// listHackatimeProjects lists the signed-in person's Hackatime projects with
// their time since the event started, for picking which to link.
func (a *API) listHackatimeProjects(w http.ResponseWriter, r *http.Request) {
	user := auth.User(r.Context())
	stats, problem := a.hackatimeStats(r, user, nil)
	if problem != "" {
		respond.Error(w, http.StatusUnprocessableEntity, problem)
		return
	}
	links, err := a.Queries.ListHackatimeLinks(r.Context(), user.ID)
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	linkedTo := map[string]int64{}
	for _, link := range links {
		linkedTo[link.HackatimeProject] = link.ProjectID
	}
	out := []hackatimeProjectJSON{}
	for _, p := range stats.Projects {
		project := hackatimeProjectJSON{Name: p.Name, Seconds: p.Seconds}
		if id, ok := linkedTo[p.Name]; ok {
			project.ProjectID = &id
		}
		out = append(out, project)
	}
	respond.JSON(w, http.StatusOK, out)
}

// hackatimeTotals asks Hackatime for the time on a project's links and on all
// of its owner's links. If that fails, it returns a problem to show them.
func (a *API) hackatimeTotals(r *http.Request, user *db.User, projectLinks, userLinks []string) (hackatimeTotals, string) {
	stats, problem := a.hackatimeStats(r, user, projectLinks)
	if problem != "" {
		return hackatimeTotals{}, problem
	}
	totals := hackatimeTotals{Project: stats.Seconds, User: stats.Seconds, Convicted: stats.Convicted}
	if len(userLinks) > len(projectLinks) {
		all, problem := a.hackatimeStats(r, user, userLinks)
		if problem != "" {
			return hackatimeTotals{}, problem
		}
		totals.User = all.Seconds
	}
	return totals, ""
}

// hackatimeStats asks Hackatime for user's time on projects (or on everything,
// if projects is empty). If that fails, it returns a problem to show them
// instead.
func (a *API) hackatimeStats(r *http.Request, user *db.User, projects []string) (hackatime.Stats, string) {
	if user.SlackID == nil {
		return hackatime.Stats{}, "Link Slack to your Hack Club account first. That's how we find you on Hackatime."
	}
	stats, err := a.Hackatime.Stats(r.Context(), *user.SlackID, projects)
	switch {
	case errors.Is(err, hackatime.ErrNoAccount):
		return stats, "We couldn't find a Hackatime account for your Slack. Sign in to Hackatime with the same Slack account."
	case errors.Is(err, hackatime.ErrPrivate):
		return stats, "Your Hackatime stats are private. Turn on public stats in your Hackatime settings so we can count your hours."
	case err != nil:
		log.Printf("hackatime stats for user %d: %v", user.ID, err)
		return stats, "Couldn't reach Hackatime. Please try again in a bit."
	}
	return stats, ""
}

// joinList joins items like "a, b and c".
func joinList(items []string) string {
	if len(items) == 1 {
		return items[0]
	}
	return strings.Join(items[:len(items)-1], ", ") + " and " + items[len(items)-1]
}
