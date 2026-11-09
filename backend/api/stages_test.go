package api_test

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/scherepi/crossroads-backend/db"
)

// projectNamed is a finished project with links nobody else uses, so the
// shared-links check doesn't send it to the fraud check.
func projectNamed(name string, hackatimeProjects ...string) map[string]any {
	p := completeProject(hackatimeProjects...)
	p["name"] = name
	p["repo_url"] = "https://github.com/example/" + name
	p["demo_url"] = "https://example.github.io/" + name
	return p
}

// shipFor makes a project for a new participant and ships hours of it.
func (e *env) shipFor(t *testing.T, slackID string, hours int) (*db.User, int64) {
	t.Helper()
	user := e.newUser(t, slackID)
	p := e.createProject(t, user, projectNamed("p-"+slackID, "ht-"+slackID))
	return user, e.submitHours(t, user, slackID, p, "ht-"+slackID, hours*3600)
}

type reviewDetail struct {
	Submission struct {
		Status          string   `json:"status"`
		ApprovedSeconds *int     `json:"approved_seconds"`
		FraudReasons    []string `json:"fraud_reasons"`
	} `json:"submission"`
	Stage  string `json:"stage"`
	Events []struct {
		Kind     string  `json:"kind"`
		Seconds  *int    `json:"seconds"`
		Message  *string `json:"message"`
		Internal *string `json:"internal"`
		Actor    *struct {
			ID int64 `json:"id"`
		} `json:"actor"`
	} `json:"events"`
	Checks []struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	} `json:"checks"`
	CanDecide        bool   `json:"can_decide"`
	CantDecideReason string `json:"cant_decide_reason"`
}

func (e *env) detail(t *testing.T, reviewer *db.User, id int64) reviewDetail {
	t.Helper()
	var d reviewDetail
	e.mustCall(t, reviewer, http.StatusOK, "GET", "/api/review/submissions/"+itoa(id), nil, &d)
	return d
}

func (e *env) eventKinds(t *testing.T, reviewer *db.User, id int64) []string {
	t.Helper()
	var kinds []string
	for _, ev := range e.detail(t, reviewer, id).Events {
		kinds = append(kinds, ev.Kind)
	}
	return kinds
}

func (e *env) setReviewSettings(t *testing.T, body map[string]any) {
	t.Helper()
	e.mustCall(t, e.newAdmin(t), http.StatusOK, "PATCH", "/api/admin/review-settings", body, nil)
}

func TestReviewThenHQSignOff(t *testing.T) {
	e := newEnv(t)
	reviewer := e.newStageReviewer(t, "UREV", true, false, false)
	hq := e.newStageReviewer(t, "UHQ", false, false, true)
	user, id := e.shipFor(t, "U1", 3)

	// HQ can't take the first stage, and the reviewer can't sign off.
	if status, _ := e.decide(t, hq, id, map[string]any{"action": "approve"}); status != http.StatusForbidden {
		t.Errorf("HQ-only reviewer at the first stage: got %d, want 403", status)
	}
	status, result := e.decide(t, reviewer, id, map[string]any{"action": "approve", "feedback": "Nice!", "internal_notes": "commits match"})
	if status != http.StatusOK || result.Submission.Status != "awaiting_hq" || result.StampsPaid != 0 {
		t.Fatalf("first approval: got %d %+v, want awaiting_hq with nothing paid", status, result)
	}
	if e.stamps(t, user) != 0 {
		t.Error("stamps were paid before HQ signed off")
	}
	// The participant sees it as approved, waiting for a final check.
	var p project
	e.call(t, user, "GET", "/api/projects/"+itoa(e.projectOf(t, id)), nil, &p)
	if p.Submissions[0].Status != "final_check" {
		t.Errorf("participant sees %q, want final_check", p.Submissions[0].Status)
	}
	if status, _ := e.decide(t, reviewer, id, map[string]any{"action": "approve"}); status != http.StatusForbidden {
		t.Errorf("review-only reviewer signing off: got %d, want 403", status)
	}

	// HQ signs off on less than the reviewer approved.
	status, result = e.decide(t, hq, id, map[string]any{"action": "approve", "approved_seconds": 2 * 3600, "stage": "awaiting_hq"})
	if status != http.StatusOK || result.Submission.Status != "approved" || result.StampsPaid != 20 {
		t.Fatalf("sign-off: got %d %+v, want approved and 20 stamps", status, result)
	}
	if got := e.eventKinds(t, hq, id); !slices.Equal(got, []string{"approval", "final_approval"}) {
		t.Errorf("events are %v", got)
	}
}

func (e *env) projectOf(t *testing.T, submissionID int64) int64 {
	t.Helper()
	var id int64
	if err := e.pool.QueryRow(context.Background(), "select project_id from submissions where id = $1", submissionID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestHQApprovalSignsOffStraightAway(t *testing.T) {
	e := newEnv(t)
	user, id := e.shipFor(t, "U1", 3)
	status, result := e.decide(t, e.newReviewer(t), id, map[string]any{"action": "approve"})
	if status != http.StatusOK || result.Submission.Status != "approved" || e.stamps(t, user) != 30 {
		t.Errorf("got %d %+v, balance %d; want approved and 30 stamps", status, result, e.stamps(t, user))
	}
}

func TestFlaggedShipsGoThroughTheFraudCheck(t *testing.T) {
	e := newEnv(t)
	e.setReviewSettings(t, map[string]any{"large_ship_seconds": 5 * 3600})
	reviewer := e.newStageReviewer(t, "UREV", true, false, true) // review and HQ, but not fraud
	fraud := e.newStageReviewer(t, "UFRAUD", false, true, false)
	user, id := e.shipFor(t, "U1", 8)

	status, result := e.decide(t, reviewer, id, map[string]any{"action": "approve"})
	if status != http.StatusOK || result.Submission.Status != "fraud_check" {
		t.Fatalf("approving a big ship: got %d %q, want fraud_check even for an HQ reviewer", status, result.Submission.Status)
	}
	d := e.detail(t, fraud, id)
	if !slices.Equal(d.Submission.FraudReasons, []string{"large_ship"}) || d.Stage != "fraud" || !d.CanDecide {
		t.Errorf("fraud reasons %v, stage %q, can decide %v", d.Submission.FraudReasons, d.Stage, d.CanDecide)
	}
	if status, _ := e.decide(t, reviewer, id, map[string]any{"action": "approve"}); status != http.StatusForbidden {
		t.Errorf("non-fraud reviewer passing the fraud check: got %d, want 403", status)
	}
	// The fraud check can cut hours but never add them.
	if status, _ := e.decide(t, fraud, id, map[string]any{"action": "approve", "approved_seconds": 9 * 3600}); status != http.StatusUnprocessableEntity {
		t.Errorf("fraud check adding hours: got %d, want 422", status)
	}
	status, result = e.decide(t, fraud, id, map[string]any{"action": "approve", "approved_seconds": 6 * 3600, "internal_notes": "two days look scripted"})
	if status != http.StatusOK || result.Submission.Status != "awaiting_hq" || *result.Submission.ApprovedSeconds != 6*3600 {
		t.Fatalf("fraud pass: got %d %+v", status, result.Submission)
	}
	if e.stamps(t, user) != 0 {
		t.Error("paid before HQ signed off")
	}
	status, result = e.decide(t, reviewer, id, map[string]any{"action": "approve"})
	if status != http.StatusOK || result.StampsPaid != 60 {
		t.Errorf("sign-off: got %d, paid %d, want the 6 hours the fraud check left (60)", status, result.StampsPaid)
	}
	if got := e.eventKinds(t, reviewer, id); !slices.Equal(got, []string{"approval", "flagged", "fraud_pass", "final_approval"}) {
		t.Errorf("events are %v", got)
	}
}

func TestEverythingThroughFraudAndManualFraud(t *testing.T) {
	e := newEnv(t)
	reviewer := e.newReviewer(t)
	_, id := e.shipFor(t, "U1", 2)
	_, result := e.decide(t, reviewer, id, map[string]any{"action": "approve", "send_to_fraud": true})
	if result.Submission.Status != "fraud_check" {
		t.Errorf("send_to_fraud: got %q, want fraud_check", result.Submission.Status)
	}

	e.setReviewSettings(t, map[string]any{"fraud_check_everything": true})
	_, id = e.shipFor(t, "U2", 2)
	_, result = e.decide(t, reviewer, id, map[string]any{"action": "approve"})
	if result.Submission.Status != "fraud_check" {
		t.Errorf("with every ship going through fraud: got %q, want fraud_check", result.Submission.Status)
	}
	if reasons := e.detail(t, reviewer, id).Submission.FraudReasons; !slices.Equal(reasons, []string{"everything"}) {
		t.Errorf("reasons are %v", reasons)
	}
}

func TestSharedLinksAndFlaggedHackatimeAreFraudSignals(t *testing.T) {
	e := newEnv(t)
	reviewer := e.newReviewer(t)
	a := e.newUser(t, "UA")
	b := e.newUser(t, "UB")
	e.createProject(t, a, completeProject("one"))
	p := e.createProject(t, b, completeProject("two")) // same repo link
	id := e.submitHours(t, b, "UB", p, "two", 3600)
	e.hackatime.convicted["UB"] = true

	d := e.detail(t, reviewer, id)
	failing := map[string]bool{}
	for _, c := range d.Checks {
		if c.Status != "pass" {
			failing[c.ID] = true
		}
	}
	if !failing["shared_links"] || !failing["hackatime_flagged"] || !failing["identity"] || failing["large_ship"] {
		t.Errorf("failing checks are %v", failing)
	}
	_, result := e.decide(t, reviewer, id, map[string]any{"action": "approve"})
	if result.Submission.Status != "fraud_check" {
		t.Errorf("got %q, want fraud_check", result.Submission.Status)
	}
}

func TestSendingBack(t *testing.T) {
	e := newEnv(t)
	reviewer := e.newStageReviewer(t, "UREV", true, false, false)
	hq := e.newStageReviewer(t, "UHQ", false, false, true)
	_, id := e.shipFor(t, "U1", 3)
	e.decide(t, reviewer, id, map[string]any{"action": "approve", "feedback": "great"})

	if status, _ := e.decide(t, hq, id, map[string]any{"action": "send_back"}); status != http.StatusUnprocessableEntity {
		t.Errorf("sending back without a reason: got %d, want 422", status)
	}
	status, result := e.decide(t, hq, id, map[string]any{"action": "send_back", "internal_notes": "hours look high"})
	if status != http.StatusOK || result.Submission.Status != "pending" || result.Submission.ApprovedSeconds != nil || result.Submission.ReviewerID != nil {
		t.Fatalf("sending back: got %d %+v", status, result.Submission)
	}
	// It's back with the reviewers, and the approval's still in the history.
	status, result = e.decide(t, reviewer, id, map[string]any{"action": "approve", "approved_seconds": 3600})
	if status != http.StatusOK || result.Submission.Status != "awaiting_hq" {
		t.Errorf("approving again: got %d %q", status, result.Submission.Status)
	}
	if got := e.eventKinds(t, hq, id); !slices.Equal(got, []string{"approval", "sent_back", "approval"}) {
		t.Errorf("events are %v", got)
	}
}

func TestRejectingAtTheFraudCheck(t *testing.T) {
	e := newEnv(t)
	reviewer := e.newReviewer(t)
	user, id := e.shipFor(t, "U1", 3)
	e.decide(t, reviewer, id, map[string]any{"action": "approve", "send_to_fraud": true})
	if status, _ := e.decide(t, reviewer, id, map[string]any{"action": "reject"}); status != http.StatusUnprocessableEntity {
		t.Errorf("rejecting without feedback: got %d, want 422", status)
	}
	status, result := e.decide(t, reviewer, id, map[string]any{"action": "reject", "feedback": "This time was made by a script."})
	if status != http.StatusOK || result.Submission.Status != "rejected" || e.stamps(t, user) != 0 {
		t.Fatalf("got %d %+v", status, result.Submission)
	}
	var p project
	e.call(t, user, "GET", "/api/projects/"+itoa(e.projectOf(t, id)), nil, &p)
	if p.Submissions[0].Status != "rejected" {
		t.Errorf("participant sees %q", p.Submissions[0].Status)
	}
}

func TestStaleDecisionsAreRefused(t *testing.T) {
	e := newEnv(t)
	reviewer := e.newStageReviewer(t, "UREV", true, true, false)
	_, id := e.shipFor(t, "U1", 3)
	e.decide(t, reviewer, id, map[string]any{"action": "approve", "send_to_fraud": true, "stage": "pending"})
	// A second tab still showing the first stage mustn't pass the fraud check.
	status, result := e.decide(t, reviewer, id, map[string]any{"action": "approve", "stage": "pending"})
	if status != http.StatusConflict || !strings.Contains(result.Error, "fraud check") {
		t.Errorf("stale approval: got %d %q, want 409", status, result.Error)
	}
}

func TestClaimsAndNext(t *testing.T) {
	e := newEnv(t)
	ana := e.newStageReviewer(t, "UANA", true, false, false)
	ben := e.newStageReviewer(t, "UBEN", true, false, false)
	_, first := e.shipFor(t, "U1", 1)
	_, second := e.shipFor(t, "U2", 1)

	var next struct {
		SubmissionID int64 `json:"submission_id"`
	}
	e.mustCall(t, ana, http.StatusOK, "POST", "/api/review/next", map[string]any{"stage": "review"}, &next)
	if next.SubmissionID != first {
		t.Fatalf("Ana got %d, want the oldest, %d", next.SubmissionID, first)
	}
	// Asking again gives her the same one: her own claim comes first.
	e.mustCall(t, ana, http.StatusOK, "POST", "/api/review/next", map[string]any{"stage": "review"}, &next)
	if next.SubmissionID != first {
		t.Errorf("Ana's second Next gave %d, want her claim %d", next.SubmissionID, first)
	}
	e.mustCall(t, ben, http.StatusOK, "POST", "/api/review/next", map[string]any{"stage": "review"}, &next)
	if next.SubmissionID != second {
		t.Errorf("Ben got %d, want %d since Ana holds the first", next.SubmissionID, second)
	}
	status, result := e.decide(t, ben, first, map[string]any{"action": "approve"})
	if status != http.StatusConflict || !strings.Contains(result.Error, "reviewing this right now") {
		t.Errorf("deciding someone else's claim: got %d %q", status, result.Error)
	}
	e.mustCall(t, ben, http.StatusConflict, "POST", "/api/review/submissions/"+itoa(first)+"/claim", nil, nil)

	// Ben skips his; Next has nothing else for him.
	e.mustCall(t, ben, http.StatusNoContent, "POST", "/api/review/submissions/"+itoa(second)+"/skip", nil, nil)
	if status := e.call(t, ben, "POST", "/api/review/next", map[string]any{"stage": "review"}, nil); status != http.StatusNoContent {
		t.Errorf("Ben's Next after skipping: got %d, want 204", status)
	}
	// Ana lets go, and Ben can take it.
	e.mustCall(t, ana, http.StatusNoContent, "DELETE", "/api/review/submissions/"+itoa(first)+"/claim", nil, nil)
	e.mustCall(t, ben, http.StatusOK, "POST", "/api/review/submissions/"+itoa(first)+"/claim", nil, nil)

	// Only stages you can decide.
	if status := e.call(t, ben, "POST", "/api/review/next", map[string]any{"stage": "hq"}, nil); status != http.StatusForbidden {
		t.Errorf("Next for a stage Ben can't decide: got %d, want 403", status)
	}

	var resp queueResponse
	e.mustCall(t, ana, http.StatusOK, "GET", "/api/review/queue?stage=review", nil, &resp)
	if resp.Counts.Review != 2 || len(resp.Items) != 2 || resp.Items[0].Claim == nil || resp.Items[0].Claim.UserID != ben.ID {
		t.Errorf("queue is %+v", resp)
	}
	// Deciding drops the claim.
	e.decide(t, ben, first, map[string]any{"action": "approve"})
	e.mustCall(t, ana, http.StatusOK, "GET", "/api/review/queue?stage=hq", nil, &resp)
	if len(resp.Items) != 1 || resp.Items[0].Claim != nil || resp.Counts.HQ != 1 {
		t.Errorf("HQ queue is %+v", resp)
	}
}

func TestCommentsAndMessages(t *testing.T) {
	e := newEnv(t)
	reviewer := e.newReviewer(t)
	user, id := e.shipFor(t, "U1", 1)
	path := "/api/review/submissions/" + itoa(id) + "/comments"
	e.mustCall(t, reviewer, http.StatusCreated, "POST", path, map[string]any{"message": "Can you add a README?"}, nil)
	e.mustCall(t, reviewer, http.StatusCreated, "POST", path, map[string]any{"message": "they're in my class", "internal": true}, nil)
	e.mustCall(t, reviewer, http.StatusUnprocessableEntity, "POST", path, map[string]any{"message": " "}, nil)
	e.mustCall(t, user, http.StatusForbidden, "POST", path, map[string]any{"message": "hi"}, nil)

	var p struct {
		Submissions []struct {
			Messages []struct {
				Kind    string `json:"kind"`
				Message string `json:"message"`
			} `json:"messages"`
		} `json:"submissions"`
	}
	e.call(t, user, "GET", "/api/projects/"+itoa(e.projectOf(t, id)), nil, &p)
	if m := p.Submissions[0].Messages; len(m) != 1 || m[0].Message != "Can you add a README?" {
		t.Errorf("participant sees %+v, want only the public comment", m)
	}
	if got := e.eventKinds(t, reviewer, id); !slices.Equal(got, []string{"comment", "internal_comment"}) {
		t.Errorf("events are %v", got)
	}
}

func TestFeedbackTemplates(t *testing.T) {
	e := newEnv(t)
	reviewer := e.newReviewer(t)
	var created struct {
		ID int64 `json:"id"`
	}
	e.mustCall(t, reviewer, http.StatusCreated, "POST", "/api/review/templates", map[string]any{"title": "No demo", "body": "Your demo link doesn't load."}, &created)
	e.mustCall(t, reviewer, http.StatusUnprocessableEntity, "POST", "/api/review/templates", map[string]any{"title": "", "body": "x"}, nil)
	e.mustCall(t, reviewer, http.StatusOK, "PATCH", "/api/review/templates/"+itoa(created.ID), map[string]any{"title": "No demo", "body": "Your demo doesn't load for us."}, nil)
	var list []struct {
		Body string `json:"body"`
	}
	e.mustCall(t, reviewer, http.StatusOK, "GET", "/api/review/templates", nil, &list)
	if len(list) != 1 || list[0].Body != "Your demo doesn't load for us." {
		t.Errorf("templates are %+v", list)
	}
	e.mustCall(t, reviewer, http.StatusNoContent, "DELETE", "/api/review/templates/"+itoa(created.ID), nil, nil)
	e.mustCall(t, reviewer, http.StatusNotFound, "DELETE", "/api/review/templates/"+itoa(created.ID), nil, nil)
	e.mustCall(t, e.newUser(t, "U1"), http.StatusForbidden, "GET", "/api/review/templates", nil, nil)
}

func TestAdminSetsReviewPermissions(t *testing.T) {
	e := newEnv(t)
	admin := e.newAdmin(t)
	person := e.newUser(t, "U1")
	path := "/api/admin/users/" + itoa(person.ID)
	e.mustCall(t, admin, http.StatusUnprocessableEntity, "PATCH", path, map[string]any{"can_review": true}, nil)
	var updated struct {
		Role           string `json:"role"`
		CanReview      bool   `json:"can_review"`
		CanFraudReview bool   `json:"can_fraud_review"`
	}
	e.mustCall(t, admin, http.StatusOK, "PATCH", path, map[string]any{"role": "reviewer", "can_fraud_review": true}, &updated)
	if updated.Role != "reviewer" || updated.CanReview || !updated.CanFraudReview {
		t.Errorf("got %+v", updated)
	}
	refreshed, _ := e.queries.GetUser(context.Background(), person.ID)
	var me struct {
		CanReview      bool `json:"can_review"`
		CanFraudReview bool `json:"can_fraud_review"`
		CanHQReview    bool `json:"can_hq_review"`
	}
	e.mustCall(t, &refreshed, http.StatusOK, "GET", "/api/me", nil, &me)
	if me.CanReview || !me.CanFraudReview || me.CanHQReview {
		t.Errorf("/me says %+v", me)
	}
	var settings struct {
		FraudCheckEverything bool `json:"fraud_check_everything"`
	}
	e.mustCall(t, admin, http.StatusOK, "PATCH", "/api/admin/review-settings", map[string]any{"fraud_check_everything": true}, &settings)
	if !settings.FraudCheckEverything {
		t.Error("the setting didn't stick")
	}
	e.mustCall(t, admin, http.StatusUnprocessableEntity, "PATCH", "/api/admin/review-settings", map[string]any{"long_day_seconds": 0}, nil)
	e.mustCall(t, &refreshed, http.StatusForbidden, "GET", "/api/admin/review-settings", nil, nil)
}

func TestSimultaneousNextsGiveDifferentShips(t *testing.T) {
	e := newEnv(t)
	for _, slackID := range []string{"U1", "U2", "U3", "U4", "U5", "U6"} {
		e.shipFor(t, slackID, 1)
	}
	reviewers := []*db.User{}
	for _, slackID := range []string{"UA", "UB", "UC", "UD", "UE", "UF"} {
		reviewers = append(reviewers, e.newStageReviewer(t, slackID, true, false, false))
	}
	got := make([]int64, len(reviewers))
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i, reviewer := range reviewers {
		wg.Go(func() {
			<-start
			var next struct {
				SubmissionID int64 `json:"submission_id"`
			}
			e.call(t, reviewer, "POST", "/api/review/next", map[string]any{"stage": "review"}, &next)
			got[i] = next.SubmissionID
		})
	}
	close(start)
	wg.Wait()
	slices.Sort(got)
	if len(slices.Compact(slices.Clone(got))) != len(got) || got[0] == 0 {
		t.Errorf("reviewers got %v, want six different ships", got)
	}
}
