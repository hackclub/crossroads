package api

// Reviewers work through submissions here. Approving one pays out stamps and
// grants a box for each milestone the person has now reached.

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

func toReviewerSubmissionJSON(s db.Submission) submissionJSON {
	out := toSubmissionJSON(s)
	out.InternalNotes = s.InternalNotes
	out.ReviewerID = s.ReviewerID
	return out
}

type queueItemJSON struct {
	ID                int64     `json:"id"`
	ProjectID         int64     `json:"project_id"`
	ProjectName       string    `json:"project_name"`
	UserID            int64     `json:"user_id"`
	UserName          string    `json:"user_name"`
	SlackID           *string   `json:"slack_id"`
	HackatimeProjects []string  `json:"hackatime_projects"`
	NewSeconds        int32     `json:"new_seconds"`
	SubmittedAt       time.Time `json:"submitted_at"`
	Priority          bool      `json:"priority"`
}

// reviewQueue lists pending submissions, priority review first, then oldest
// first.
func (a *API) reviewQueue(w http.ResponseWriter, r *http.Request) {
	rows, err := a.Queries.ListReviewQueue(r.Context())
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	out := []queueItemJSON{}
	for _, row := range rows {
		out = append(out, queueItemJSON{
			ID:                row.ID,
			ProjectID:         row.ProjectID,
			ProjectName:       row.ProjectName,
			UserID:            row.UserID,
			UserName:          row.UserName,
			SlackID:           row.SlackID,
			HackatimeProjects: row.HackatimeProjects,
			NewSeconds:        row.NewSeconds,
			SubmittedAt:       row.SubmittedAt,
			Priority:          row.Priority,
		})
	}
	respond.JSON(w, http.StatusOK, out)
}

type reviewUserJSON struct {
	ID                 int64   `json:"id"`
	Name               string  `json:"name"`
	Email              string  `json:"email"`
	SlackID            *string `json:"slack_id"`
	VerificationStatus string  `json:"verification_status"`
	YSWSEligible       bool    `json:"ysws_eligible"`
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

// getSubmissionForReview shows a reviewer everything about a submission: the
// project and its earlier submissions, who made it, and what Hackatime says now.
func (a *API) getSubmissionForReview(w http.ResponseWriter, r *http.Request) {
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

	shared, err := a.Queries.ListProjectsSharingLinks(r.Context(), db.ListProjectsSharingLinksParams{
		ProjectID: row.Project.ID,
		RepoUrl:   row.Project.RepoUrl,
		DemoUrl:   row.Project.DemoUrl,
	})
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}

	out := struct {
		Submission submissionJSON `json:"submission"`
		Project    projectJSON    `json:"project"`
		User       reviewUserJSON `json:"user"`
		// Other projects, anyone's, with the same repo or demo link: a sign
		// of a copied project, or one project split up to be paid twice.
		ProjectsSharingLinks []sharedLinkProjectJSON `json:"projects_sharing_links"`
		Hackatime            *reviewHackatimeJSON    `json:"hackatime"`
		HackatimeError       string                  `json:"hackatime_error,omitempty"`
	}{
		Submission: toReviewerSubmissionJSON(row.Submission),
		Project:    project,
		User: reviewUserJSON{
			ID:                 row.User.ID,
			Name:               row.User.Name,
			Email:              row.User.Email,
			SlackID:            row.User.SlackID,
			VerificationStatus: row.User.VerificationStatus,
			YSWSEligible:       row.User.YswsEligible,
		},
		ProjectsSharingLinks: []sharedLinkProjectJSON{},
	}
	for _, p := range shared {
		out.ProjectsSharingLinks = append(out.ProjectsSharingLinks, sharedLinkProjectJSON{
			ID: p.ID, Name: p.Name, RepoURL: p.RepoUrl, DemoURL: p.DemoUrl, UserID: p.UserID, UserName: p.UserName,
		})
	}
	if row.User.SlackID == nil {
		out.HackatimeError = "They don't have Slack linked to their Hack Club account."
	} else if stats, err := a.Hackatime.Stats(r.Context(), *row.User.SlackID, row.Submission.HackatimeProjects); err != nil {
		out.HackatimeError = err.Error()
	} else {
		out.Hackatime = &reviewHackatimeJSON{Seconds: stats.Seconds, Projects: []hackatimeProjectJSON{}, Convicted: stats.Convicted}
		for _, p := range stats.Projects {
			out.Hackatime.Projects = append(out.Hackatime.Projects, hackatimeProjectJSON{Name: p.Name, Seconds: p.Seconds})
		}
	}
	respond.JSON(w, http.StatusOK, out)
}

// reviewDecision is the body of POST /review/submissions/{id}.
type reviewDecision struct {
	Status db.SubmissionStatus `json:"status"` // approved or rejected
	// How much of the submission's new time to accept. Defaults to all of it.
	// Only for approvals.
	ApprovedSeconds *int32 `json:"approved_seconds"`
	Feedback        string `json:"feedback"` // shown to them; required to reject
	InternalNotes   string `json:"internal_notes"`
}

// reviewSubmission approves or rejects a pending submission. Approving pays
// out stamps for the accepted time and grants boxes for newly reached
// milestones, all in the same transaction.
func (a *API) reviewSubmission(w http.ResponseWriter, r *http.Request) {
	reviewer := auth.User(r.Context())
	id, ok := idParam(w, r, "submissionID")
	if !ok {
		return
	}
	var decision reviewDecision
	if !decode(w, r, &decision) {
		return
	}
	decision.Feedback = strings.TrimSpace(decision.Feedback)
	decision.InternalNotes = strings.TrimSpace(decision.InternalNotes)
	switch {
	case decision.Status != db.SubmissionStatusApproved && decision.Status != db.SubmissionStatusRejected:
		respond.Error(w, http.StatusUnprocessableEntity, `status has to be "approved" or "rejected".`)
		return
	case decision.Status == db.SubmissionStatusRejected && decision.Feedback == "":
		respond.Error(w, http.StatusUnprocessableEntity, "Tell them why it was rejected.")
		return
	case decision.Status == db.SubmissionStatusRejected && decision.ApprovedSeconds != nil:
		respond.Error(w, http.StatusUnprocessableEntity, "Only approvals take approved_seconds.")
		return
	case len(decision.Feedback) > 5000 || len(decision.InternalNotes) > 5000:
		respond.Error(w, http.StatusUnprocessableEntity, "Keep feedback and notes under 5,000 characters.")
		return
	}

	tx, err := a.Pool.Begin(r.Context())
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())
	q := a.Queries.WithTx(tx)

	row, err := q.GetSubmissionForReview(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		respond.Error(w, http.StatusNotFound, "That submission doesn't exist.")
		return
	}
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	owner := row.User
	if owner.ID == reviewer.ID {
		respond.Error(w, http.StatusForbidden, "You can't review your own project.")
		return
	}
	// Approvals for the same person go one at a time, so each one sees the
	// running total the last one left.
	if _, err := q.LockUser(r.Context(), owner.ID); err != nil {
		respond.ServerError(w, r, err)
		return
	}
	submission, err := q.LockSubmission(r.Context(), id)
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	if submission.Status != db.SubmissionStatusPending {
		respond.Error(w, http.StatusConflict, "This submission has already been reviewed.")
		return
	}

	var approvedSeconds *int32
	if decision.Status == db.SubmissionStatusApproved {
		seconds := submission.NewSeconds
		if decision.ApprovedSeconds != nil {
			seconds = *decision.ApprovedSeconds
		}
		if seconds < 0 || seconds > submission.NewSeconds {
			respond.Error(w, http.StatusUnprocessableEntity, "approved_seconds has to be between 0 and the submission's new_seconds.")
			return
		}
		approvedSeconds = &seconds
	}
	submission, err = q.ReviewSubmission(r.Context(), db.ReviewSubmissionParams{
		ID:              submission.ID,
		Status:          decision.Status,
		ApprovedSeconds: approvedSeconds,
		Feedback:        nonEmpty(decision.Feedback),
		InternalNotes:   nonEmpty(decision.InternalNotes),
		ReviewerID:      &reviewer.ID,
	})
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}

	var stampsPaid int64
	boxes := []boxJSON{}
	if decision.Status == db.SubmissionStatusApproved {
		after, err := q.ApprovedSecondsForUser(r.Context(), owner.ID)
		if err != nil {
			respond.ServerError(w, r, err)
			return
		}
		before := after - int64(*approvedSeconds)
		// Paying on the running total means rounding never loses anyone a
		// stamp: 60 approved hours is always exactly 60 hours of stamps,
		// however it was split up.
		stampsPaid = a.stampsFor(after) - a.stampsFor(before)
		if stampsPaid > 0 {
			if _, err := q.AddStampEntry(r.Context(), db.AddStampEntryParams{
				UserID:       owner.ID,
				Kind:         db.StampEntryKindPayout,
				Amount:       int32(stampsPaid),
				SubmissionID: &submission.ID,
			}); err != nil {
				respond.ServerError(w, r, err)
				return
			}
			// A Golden Spike on the project pays 10% more, as its own entry.
			spiked, err := q.ProjectHasStampMultiplier(r.Context(), &submission.ProjectID)
			if err != nil {
				respond.ServerError(w, r, err)
				return
			}
			if bonus := stampBonus(stampsPaid); spiked && bonus > 0 {
				if _, err := q.AddStampEntry(r.Context(), db.AddStampEntryParams{
					UserID:       owner.ID,
					Kind:         db.StampEntryKindBonus,
					Amount:       int32(bonus),
					SubmissionID: &submission.ID,
				}); err != nil {
					respond.ServerError(w, r, err)
					return
				}
				stampsPaid += bonus
			}
		}
		granted, err := q.GrantMilestoneBoxes(r.Context(), db.GrantMilestoneBoxesParams{UserID: owner.ID, ApprovedSeconds: int32(after)})
		if err != nil {
			respond.ServerError(w, r, err)
			return
		}
		for _, b := range granted {
			boxes = append(boxes, toBoxJSON(b, nil))
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		respond.ServerError(w, r, err)
		return
	}
	respond.JSON(w, http.StatusOK, struct {
		Submission   submissionJSON `json:"submission"`
		StampsPaid   int64          `json:"stamps_paid"`
		BoxesGranted []boxJSON      `json:"boxes_granted"`
	}{toReviewerSubmissionJSON(submission), stampsPaid, boxes})
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
