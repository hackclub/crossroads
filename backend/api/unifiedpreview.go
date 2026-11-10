package api

import (
	"context"
	"math"
	"strings"

	"github.com/scherepi/crossroads-backend/db"
	"github.com/scherepi/crossroads-backend/unified"
)

// unifiedPreviewJSON is what the Airtable sync will send to the Unified YSWS
// DB for a ship, as it stands: the hours, the justification (written by the
// same code the sync uses), and the other fields, with personal details shown
// only as on file or missing.
type unifiedPreviewJSON struct {
	Hours         float64            `json:"hours"`
	Justification string             `json:"justification"`
	SignedOff     bool               `json:"signed_off"` // the sync only sends signed-off ships
	Missing       []string           `json:"missing"`    // what holds back "Submit to Unified YSWS"
	Fields        []previewFieldJSON `json:"fields"`
}

type previewFieldJSON struct {
	Name    string `json:"name"`
	Value   string `json:"value"`
	Missing bool   `json:"missing"`
}

// unifiedPreview is nil until a reviewer has approved the ship: before that
// there are no hours or justification to show.
func (a *API) unifiedPreview(ctx context.Context, row db.GetSubmissionForReviewRow, events []db.ListProjectShipEventsRow,
	submissions []db.Submission) (*unifiedPreviewJSON, error) {
	s, project, user := row.Submission, row.Project, row.User
	if s.ApprovedSeconds == nil || s.Status == db.SubmissionStatusRejected {
		return nil, nil
	}
	r := unified.Review{
		HackatimeProjects: s.HackatimeProjects, HackatimeSeconds: s.HackatimeSeconds, NewSeconds: s.NewSeconds,
		ApprovedSeconds: *s.ApprovedSeconds, HoursStart: a.HoursStart, SubmittedAt: s.SubmittedAt,
		ReviewerSeconds: *s.ApprovedSeconds, TechnicalFeatures: deref(s.TechnicalFeatures), Notes: deref(s.InternalNotes),
		OverrideReason: deref(s.OverrideReason), FraudReasons: s.FraudReasons,
	}
	for _, e := range submissions {
		if e.ID < s.ID && e.Status == db.SubmissionStatusApproved {
			r.EarlierApproval = true
		}
	}
	// This ship's events since it was last sent back, oldest first.
	var mine []db.ListProjectShipEventsRow
	for _, e := range events {
		if e.ShipEvent.SubmissionID != s.ID {
			continue
		}
		if e.ShipEvent.Kind == db.ShipEventKindSentBack {
			mine = nil
			continue
		}
		mine = append(mine, e)
	}
	for _, e := range mine {
		name := deref(e.ActorName)
		switch e.ShipEvent.Kind {
		case db.ShipEventKindApproval:
			r.ReviewerName, r.ReviewerDeflation = name, deref(e.ShipEvent.DeflationReason)
			if e.ShipEvent.Seconds != nil {
				r.ReviewerSeconds = *e.ShipEvent.Seconds
			}
		case db.ShipEventKindFraudPass:
			r.FraudReviewer, r.FraudSeconds, r.FraudDeflation = name, e.ShipEvent.Seconds, deref(e.ShipEvent.DeflationReason)
		case db.ShipEventKindFinalApproval:
			r.HQName, r.HQDeflation = name, deref(e.ShipEvent.DeflationReason)
		}
	}
	if s.HqReviewerID != nil && r.HQName == "" {
		// Signed off by a reviewer with HQ powers, in their approval.
		r.HQName = r.ReviewerName
	}

	p := &unifiedPreviewJSON{
		Hours:         math.Round(float64(*s.ApprovedSeconds)/3600*100) / 100,
		Justification: unified.Justification(r),
		SignedOff:     s.Status == db.SubmissionStatusApproved,
		Missing:       unified.Missing(user, project),
	}
	if p.Missing == nil {
		p.Missing = []string{}
	}
	if !user.YswsEligible {
		p.Missing = append(p.Missing, "YSWS eligibility (identity verification)")
	}
	text := func(name string, value *string) {
		v := strings.TrimSpace(deref(value))
		p.Fields = append(p.Fields, previewFieldJSON{Name: name, Value: v, Missing: v == ""})
	}
	onFile := func(name string, ok bool) {
		v := "On file"
		if !ok {
			v = "Missing"
		}
		p.Fields = append(p.Fields, previewFieldJSON{Name: name, Value: v, Missing: !ok})
	}
	text("Code URL", project.RepoUrl)
	text("Playable URL", project.DemoUrl)
	text("Description", &project.Description)
	onFile("Screenshot", project.ScreenshotUrl != nil && *project.ScreenshotUrl != "")
	text("GitHub Username", unified.GitHubUsername(project.RepoUrl))
	text("First Name", &user.FirstName)
	text("Last Name", &user.LastName)
	text("Email", &user.Email)
	onFile("Birthday", user.Birthday != nil)
	onFile("Address", len(unified.AddressGaps(user.Address)) == 0)
	return p, nil
}
