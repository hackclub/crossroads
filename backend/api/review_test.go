package api_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/scherepi/crossroads-backend/db"
)

// newReviewer is a reviewer who can decide every stage, so their approvals
// are signed off straight away unless a check flags them.
func (e *env) newReviewer(t *testing.T) *db.User {
	t.Helper()
	return e.newStageReviewer(t, "UREVIEWER", true, true, true)
}

// newStageReviewer is a reviewer who can decide only some stages.
func (e *env) newStageReviewer(t *testing.T, slackID string, review, fraud, hq bool) *db.User {
	t.Helper()
	reviewer := e.newUser(t, slackID)
	if _, err := e.pool.Exec(context.Background(),
		"update users set role = 'reviewer', can_review = $2, can_fraud_review = $3, can_hq_review = $4 where id = $1",
		reviewer.ID, review, fraud, hq); err != nil {
		t.Fatal(err)
	}
	reviewer.Role, reviewer.CanReview, reviewer.CanFraudReview, reviewer.CanHqReview = db.UserRoleReviewer, review, fraud, hq
	return reviewer
}

// queue is GET /api/review/queue?stage=…, decoded.
type queueResponse struct {
	Counts struct {
		Review, Fraud, HQ int
	} `json:"counts"`
	Items []struct {
		ID           int64    `json:"id"`
		ProjectID    int64    `json:"project_id"`
		Priority     bool     `json:"priority"`
		Status       string   `json:"status"`
		FraudReasons []string `json:"fraud_reasons"`
		Claim        *struct {
			UserID int64 `json:"user_id"`
		} `json:"claim"`
		SkippedByMe bool `json:"skipped_by_me"`
	} `json:"items"`
}

type decisionResult struct {
	Submission struct {
		Status          string  `json:"status"`
		ApprovedSeconds *int    `json:"approved_seconds"`
		ReviewerID      *int64  `json:"reviewer_id"`
		InternalNotes   *string `json:"internal_notes"`
	} `json:"submission"`
	StampsPaid   int `json:"stamps_paid"`
	BoxesGranted []struct {
		MilestoneID *int64 `json:"milestone_id"`
	} `json:"boxes_granted"`
	Error string `json:"error"`
}

func (e *env) decide(t *testing.T, reviewer *db.User, submissionID int64, body map[string]any) (int, decisionResult) {
	t.Helper()
	withReviewText(body)
	var result decisionResult
	status := e.call(t, reviewer, "POST", "/api/review/submissions/"+itoa(submissionID), body, &result)
	return status, result
}

// approveAllTheWay approves a ship at every stage it reaches, as a reviewer
// who can decide them all, until it's signed off.
func (e *env) approveAllTheWay(t *testing.T, reviewer *db.User, submissionID int64) {
	t.Helper()
	for range 3 {
		status, result := e.decide(t, reviewer, submissionID, map[string]any{"action": "approve"})
		if status != http.StatusOK {
			t.Fatalf("approving: got %d %q", status, result.Error)
		}
		if result.Submission.Status == "approved" {
			return
		}
	}
	t.Fatal("still not signed off after three approvals")
}

func (e *env) stamps(t *testing.T, user *db.User) int {
	t.Helper()
	var stamps int
	if err := e.pool.QueryRow(context.Background(), "select stamps from users where id = $1", user.ID).Scan(&stamps); err != nil {
		t.Fatal(err)
	}
	return stamps
}

// submitHours sets the project's Hackatime time to hours and submits it.
func (e *env) submitHours(t *testing.T, user *db.User, slackID string, p project, hackatimeProject string, seconds int) int64 {
	t.Helper()
	e.hackatime.set(slackID, hackatimeProject, seconds)
	status, s, problem := e.submit(t, user, p.ID)
	if status != http.StatusCreated {
		t.Fatalf("submitting: got %d %q", status, problem.Error)
	}
	return s.ID
}

func TestReviewNeedsAReviewer(t *testing.T) {
	e := newEnv(t)
	participant := e.newUser(t, "U1")
	if status := e.call(t, participant, "GET", "/api/review/queue", nil, nil); status != http.StatusForbidden {
		t.Errorf("participant on the review queue: got %d, want 403", status)
	}
	if status := e.call(t, e.newReviewer(t), "GET", "/api/review/queue", nil, nil); status != http.StatusOK {
		t.Errorf("reviewer on the review queue: got %d, want 200", status)
	}
}

func TestReviewQueueOrder(t *testing.T) {
	e := newEnv(t)
	reviewer := e.newReviewer(t)
	var ids []int64
	for _, slackID := range []string{"UA", "UB", "UC"} {
		user := e.newUser(t, slackID)
		p := e.createProject(t, user, completeProject("racer"))
		ids = append(ids, e.submitHours(t, user, slackID, p, "racer", 3600))
	}
	// The last one used a priority review stamp.
	ctx := context.Background()
	var stampType int64
	e.pool.QueryRow(ctx, `insert into special_stamp_types (name, rarity, effect) values ('Express', 'rare', 'priority_review') returning id`).Scan(&stampType)
	if _, err := e.pool.Exec(ctx, `insert into special_stamps (user_id, special_stamp_type_id, granted_by, submission_id)
		select projects.user_id, $1, $2, submissions.id from submissions join projects on projects.id = submissions.project_id
		where submissions.id = $3`, stampType, reviewer.ID, ids[2]); err != nil {
		t.Fatal(err)
	}

	var resp queueResponse
	e.call(t, reviewer, "GET", "/api/review/queue", nil, &resp)
	queue := resp.Items
	if len(queue) != 3 || queue[0].ID != ids[2] || !queue[0].Priority || queue[1].ID != ids[0] || queue[2].ID != ids[1] {
		t.Errorf("queue is %+v, want the priority one, then oldest first", queue)
	}
}

func TestApprovalPaysStamps(t *testing.T) {
	e := newEnv(t)
	reviewer := e.newReviewer(t)
	user := e.newUser(t, "U1")
	p := e.createProject(t, user, completeProject("racer"))
	id := e.submitHours(t, user, "U1", p, "racer", 3*3600)

	status, result := e.decide(t, reviewer, id, map[string]any{"action": "approve", "internal_notes": "looks legit"})
	if status != http.StatusOK {
		t.Fatalf("approving: got %d %q", status, result.Error)
	}
	if result.StampsPaid != 30 || e.stamps(t, user) != 30 {
		t.Errorf("paid %d, balance %d, want 30 for 3 hours at 10 an hour", result.StampsPaid, e.stamps(t, user))
	}
	if s := result.Submission; s.Status != "approved" || s.ApprovedSeconds == nil || *s.ApprovedSeconds != 3*3600 || s.ReviewerID == nil || *s.ReviewerID != reviewer.ID {
		t.Errorf("submission is %+v", s)
	}

	var kind string
	e.pool.QueryRow(context.Background(), "select kind from stamp_ledger where submission_id = $1", id).Scan(&kind)
	if kind != "payout" {
		t.Errorf("ledger entry kind is %q, want payout", kind)
	}
}

func TestApprovingPartOfTheTime(t *testing.T) {
	e := newEnv(t)
	reviewer := e.newReviewer(t)
	user := e.newUser(t, "U1")
	p := e.createProject(t, user, completeProject("racer"))
	id := e.submitHours(t, user, "U1", p, "racer", 3*3600)

	if status, _ := e.decide(t, reviewer, id, map[string]any{"action": "approve", "approved_seconds": 4 * 3600}); status != http.StatusUnprocessableEntity {
		t.Errorf("approving more than was submitted: got %d, want 422", status)
	}
	status, result := e.decide(t, reviewer, id, map[string]any{"action": "approve", "approved_seconds": 3600, "feedback": "Only counted the first hour"})
	if status != http.StatusOK || result.StampsPaid != 10 {
		t.Errorf("approving 1 of 3 hours: got %d, paid %d, want 10", status, result.StampsPaid)
	}
}

func TestRoundingNeverLosesStamps(t *testing.T) {
	e := newEnv(t)
	reviewer := e.newReviewer(t)
	user := e.newUser(t, "U1")
	p := e.createProject(t, user, completeProject("racer"))

	// Three 20-minute approvals at 10 stamps an hour are worth 3.33 each.
	var paid []int
	for _, total := range []int{1200, 2400, 3600} {
		id := e.submitHours(t, user, "U1", p, "racer", total)
		_, result := e.decide(t, reviewer, id, map[string]any{"action": "approve"})
		paid = append(paid, result.StampsPaid)
	}
	if paid[0] != 3 || paid[1] != 3 || paid[2] != 4 || e.stamps(t, user) != 10 {
		t.Errorf("paid %v for a balance of %d, want [3 3 4] for exactly 10", paid, e.stamps(t, user))
	}
}

func TestRejection(t *testing.T) {
	e := newEnv(t)
	reviewer := e.newReviewer(t)
	user := e.newUser(t, "U1")
	p := e.createProject(t, user, completeProject("racer"))
	id := e.submitHours(t, user, "U1", p, "racer", 3*3600)

	if status, _ := e.decide(t, reviewer, id, map[string]any{"action": "reject"}); status != http.StatusUnprocessableEntity {
		t.Errorf("rejecting without feedback: got %d, want 422", status)
	}
	status, result := e.decide(t, reviewer, id, map[string]any{
		"action": "reject", "feedback": "The demo link is broken.", "internal_notes": "probably fine otherwise",
	})
	if status != http.StatusOK || result.StampsPaid != 0 || e.stamps(t, user) != 0 {
		t.Errorf("rejecting: got %d, paid %d", status, result.StampsPaid)
	}

	// They see the feedback but not the internal notes.
	var detail struct {
		Submissions []map[string]any `json:"submissions"`
	}
	e.call(t, user, "GET", "/api/projects/"+itoa(p.ID), nil, &detail)
	got := detail.Submissions[0]
	if got["feedback"] != "The demo link is broken." {
		t.Errorf("feedback is %v", got["feedback"])
	}
	if _, ok := got["internal_notes"]; ok {
		t.Error("the participant can see internal notes")
	}
	if _, ok := got["reviewer_id"]; ok {
		t.Error("the participant can see who reviewed it")
	}
}

func TestReviewingTwiceOrYourOwn(t *testing.T) {
	e := newEnv(t)
	reviewer := e.newReviewer(t)
	user := e.newUser(t, "U1")
	p := e.createProject(t, user, completeProject("racer"))
	id := e.submitHours(t, user, "U1", p, "racer", 3*3600)

	e.decide(t, reviewer, id, map[string]any{"action": "approve"})
	if status, _ := e.decide(t, reviewer, id, map[string]any{"action": "approve"}); status != http.StatusConflict {
		t.Errorf("approving twice: got %d, want 409", status)
	}
	if e.stamps(t, user) != 30 {
		t.Errorf("balance is %d after a double approval, want 30", e.stamps(t, user))
	}

	ownProject := e.createProject(t, reviewer, completeProject("my-game"))
	ownID := e.submitHours(t, reviewer, "UREVIEWER", ownProject, "my-game", 3600)
	if status, _ := e.decide(t, reviewer, ownID, map[string]any{"action": "approve"}); status != http.StatusForbidden {
		t.Errorf("reviewing your own project: got %d, want 403", status)
	}
}

func TestMilestoneBoxes(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	var boxType, ten, twenty int64
	e.pool.QueryRow(ctx, "insert into box_types (name) values ('Starter') returning id").Scan(&boxType)
	e.pool.QueryRow(ctx, "insert into milestones (name, approved_seconds, box_type_id) values ('10 hours', 36000, $1) returning id", boxType).Scan(&ten)
	e.pool.QueryRow(ctx, "insert into milestones (name, approved_seconds, box_type_id) values ('20 hours', 72000, $1) returning id", boxType).Scan(&twenty)

	reviewer := e.newReviewer(t)
	user := e.newUser(t, "U1")
	p := e.createProject(t, user, completeProject("racer"))

	id := e.submitHours(t, user, "U1", p, "racer", 12*3600)
	_, result := e.decide(t, reviewer, id, map[string]any{"action": "approve"})
	if len(result.BoxesGranted) != 1 || *result.BoxesGranted[0].MilestoneID != ten {
		t.Errorf("at 12 hours got %+v, want the 10 hour box", result.BoxesGranted)
	}

	id = e.submitHours(t, user, "U1", p, "racer", 24*3600)
	_, result = e.decide(t, reviewer, id, map[string]any{"action": "approve"})
	if len(result.BoxesGranted) != 1 || *result.BoxesGranted[0].MilestoneID != twenty {
		t.Errorf("at 24 hours got %+v, want just the 20 hour box", result.BoxesGranted)
	}

	// Trimmed time doesn't count toward milestones.
	other := e.newUser(t, "U2")
	op := e.createProject(t, other, completeProject("racer"))
	id = e.submitHours(t, other, "U2", op, "racer", 12*3600)
	_, result = e.decide(t, reviewer, id, map[string]any{"action": "approve", "approved_seconds": 9 * 3600})
	if len(result.BoxesGranted) != 0 {
		t.Errorf("at 9 approved hours got %+v, want no boxes", result.BoxesGranted)
	}
}

func TestReviewDetail(t *testing.T) {
	e := newEnv(t)
	reviewer := e.newReviewer(t)
	user := e.newUser(t, "U1")
	p := e.createProject(t, user, completeProject("racer"))
	first := e.submitHours(t, user, "U1", p, "racer", 3600)
	e.decide(t, reviewer, first, map[string]any{"action": "reject", "feedback": "broken", "internal_notes": "check the repo history"})
	second := e.submitHours(t, user, "U1", p, "racer", 2*3600)

	var detail struct {
		Submission struct {
			ID int64 `json:"id"`
		} `json:"submission"`
		Project struct {
			Submissions []struct {
				InternalNotes *string `json:"internal_notes"`
			} `json:"submissions"`
		} `json:"project"`
		User struct {
			SlackID string `json:"slack_id"`
		} `json:"user"`
		Hackatime *struct {
			Seconds  int `json:"seconds"`
			Projects []struct {
				Name string `json:"name"`
			} `json:"projects"`
		} `json:"hackatime"`
	}
	if status := e.call(t, reviewer, "GET", "/api/review/submissions/"+itoa(second), nil, &detail); status != http.StatusOK {
		t.Fatalf("got %d", status)
	}
	if detail.Submission.ID != second || detail.User.SlackID != "U1" {
		t.Errorf("got submission %d for %q", detail.Submission.ID, detail.User.SlackID)
	}
	if len(detail.Project.Submissions) != 2 || detail.Project.Submissions[1].InternalNotes == nil ||
		!strings.Contains(*detail.Project.Submissions[1].InternalNotes, "repo history") {
		t.Errorf("history doesn't have the earlier review's notes: %+v", detail.Project.Submissions)
	}
	if detail.Hackatime == nil || detail.Hackatime.Seconds != 2*3600 || len(detail.Hackatime.Projects) != 1 {
		t.Errorf("hackatime is %+v", detail.Hackatime)
	}
}

// withReviewText gives an approval the technical features and deflation
// reason it needs, unless the test sets them itself.
func withReviewText(body map[string]any) {
	if body["action"] != "approve" {
		return
	}
	if _, ok := body["technical_features"]; !ok {
		body["technical_features"] = "A test feature."
	}
	if _, ok := body["deflation_reason"]; !ok {
		body["deflation_reason"] = "A test deflation."
	}
}
