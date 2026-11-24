package api_test

import (
	"context"
	"net/http"
	"slices"
	"testing"
)

type pastReviews struct {
	All   bool `json:"all"`
	Items []struct {
		ID              int64  `json:"id"`
		Status          string `json:"status"`
		ResubmitBlocked bool   `json:"resubmit_blocked"`
		ShippedAgain    bool   `json:"shipped_again"`
	} `json:"items"`
}

func (p pastReviews) ids() []int64 {
	var ids []int64
	for _, item := range p.Items {
		ids = append(ids, item.ID)
	}
	return ids
}

func TestSoftAndHardRejects(t *testing.T) {
	e := newEnv(t)
	reviewer := e.newStageReviewer(t, "UREV", true, false, false)
	fraud := e.newStageReviewer(t, "UFRAUD", true, true, false)
	hq := e.newStageReviewer(t, "UHQ", false, false, true)
	user, first := e.shipFor(t, "U1", 3)
	projectID := e.projectOf(t, first)
	shipAgain := func() (int, int64) {
		t.Helper()
		status, s, _ := e.submit(t, user, projectID)
		return status, s.ID
	}

	// A soft reject: they can ship it again straight away, with the same time.
	if status, result := e.decide(t, reviewer, first, map[string]any{"action": "reject", "feedback": "Add a demo."}); status != http.StatusOK || result.Submission.Status != "rejected" {
		t.Fatalf("soft reject: %d %+v", status, result)
	}
	status, second := shipAgain()
	if status != http.StatusCreated {
		t.Fatalf("shipping again after a soft reject: %d", status)
	}

	// A hard reject: any reviewer can, first stage included (asked for a
	// reason rather than refused), but it needs a reason.
	hard := map[string]any{"action": "reject", "feedback": "This time was made by a script.", "block_resubmit": true}
	if status, _ := e.decide(t, reviewer, second, hard); status != http.StatusUnprocessableEntity {
		t.Errorf("a first-stage reviewer's hard reject without a reason: %d, want 422", status)
	}
	if status, _ := e.decide(t, fraud, second, hard); status != http.StatusUnprocessableEntity {
		t.Errorf("hard reject without a reason: %d, want 422", status)
	}
	if status, _ := e.decide(t, fraud, second, map[string]any{"action": "approve", "block_resubmit": true}); status != http.StatusUnprocessableEntity {
		t.Errorf("an approval that blocks: %d, want 422", status)
	}
	hard["internal_notes"] = "Heartbeats every second for 3 hours."
	if status, result := e.decide(t, fraud, second, hard); status != http.StatusOK || result.Submission.Status != "rejected" {
		t.Fatalf("hard reject: %d %+v", status, result)
	}

	if status, _ := shipAgain(); status != http.StatusForbidden {
		t.Errorf("shipping again after a hard reject: %d, want 403", status)
	}
	var p struct {
		ResubmitBlocked bool `json:"resubmit_blocked"`
	}
	e.mustCall(t, user, http.StatusOK, "GET", "/api/projects/"+itoa(projectID), nil, &p)
	if !p.ResubmitBlocked {
		t.Error("the project doesn't say it can't be shipped again")
	}
	// Its Hackatime time stays with it, so it can't be shipped on another project.
	if status := e.call(t, user, "PATCH", "/api/projects/"+itoa(projectID), map[string]any{"hackatime_projects": []string{}}, nil); status != http.StatusConflict {
		t.Errorf("unlinking its Hackatime projects: %d, want 409", status)
	}
	e.mustCall(t, user, http.StatusOK, "PATCH", "/api/projects/"+itoa(projectID), map[string]any{"name": "Renamed"}, nil)

	// The ship page says who blocked it and why. It's still one rejection.
	var detail struct {
		ResubmitBlock *struct {
			By     string `json:"by"`
			Reason string `json:"reason"`
		} `json:"resubmit_block"`
		Events []struct {
			Kind           string `json:"kind"`
			BlocksResubmit bool   `json:"blocks_resubmit"`
		} `json:"events"`
	}
	e.mustCall(t, hq, http.StatusOK, "GET", "/api/review/submissions/"+itoa(second), nil, &detail)
	if detail.ResubmitBlock == nil || detail.ResubmitBlock.Reason != "Heartbeats every second for 3 hours." {
		t.Errorf("resubmit block: %+v", detail.ResubmitBlock)
	}
	last := detail.Events[len(detail.Events)-1]
	if last.Kind != "rejection" || !last.BlocksResubmit {
		t.Errorf("last event: %+v", last)
	}

	// Only HQ and admins can let it be shipped again.
	block := "/api/review/projects/" + itoa(projectID) + "/resubmit-block"
	if status := e.call(t, fraud, "PUT", block, map[string]any{"blocked": false}, nil); status != http.StatusForbidden {
		t.Errorf("the fraud check unblocked it: %d", status)
	}
	e.mustCall(t, hq, http.StatusOK, "PUT", block, map[string]any{"blocked": false}, nil)
	status, third := shipAgain()
	if status != http.StatusCreated {
		t.Errorf("shipping again once allowed: %d", status)
	}
	if status := e.call(t, hq, "PUT", block, map[string]any{"blocked": true}, nil); status != http.StatusUnprocessableEntity {
		t.Errorf("blocking without a reason: %d, want 422", status)
	}
	// A ship in review has to be hard-rejected instead.
	if status := e.call(t, hq, "PUT", block, map[string]any{"blocked": true, "reason": "Same script again."}, nil); status != http.StatusConflict {
		t.Errorf("blocking it by hand with a ship in review: %d, want 409", status)
	}
	e.approveAllTheWay(t, e.newStageReviewer(t, "UALL", true, true, true), third)
	e.mustCall(t, hq, http.StatusOK, "PUT", block, map[string]any{"blocked": true, "reason": "Same script again."}, nil)
	e.mustCall(t, user, http.StatusOK, "GET", "/api/projects/"+itoa(projectID), nil, &p)
	if !p.ResubmitBlocked {
		t.Error("blocking it by hand didn't")
	}

	// Past reviews: a reviewer sees what they decided; HQ sees everyone's.
	var past pastReviews
	e.mustCall(t, reviewer, http.StatusOK, "GET", "/api/review/past?scope=all", nil, &past)
	if past.All || !slices.Equal(past.ids(), []int64{first}) {
		t.Errorf("reviewer's past reviews: %+v", past)
	}
	e.mustCall(t, hq, http.StatusOK, "GET", "/api/review/past?status=rejected", nil, &past)
	if !past.All || !slices.Equal(past.ids(), []int64{second, first}) {
		t.Errorf("HQ's rejected ships: %+v", past)
	}
	if !past.Items[0].ResubmitBlocked || !past.Items[0].ShippedAgain {
		t.Errorf("the hard-rejected ship: %+v", past.Items[0])
	}
	if status := e.call(t, user, "GET", "/api/review/past", nil, nil); status != http.StatusForbidden {
		t.Errorf("a participant listed past reviews: %d", status)
	}
}

// Every reviewer can hard reject, not just the fraud check and HQ.
func TestFirstStageReviewerHardRejects(t *testing.T) {
	e := newEnv(t)
	reviewer := e.newStageReviewer(t, "UREV", true, false, false)
	user, id := e.shipFor(t, "U1", 3)
	projectID := e.projectOf(t, id)
	hard := map[string]any{"action": "reject", "feedback": "This time was made by a script.", "internal_notes": "Script.", "block_resubmit": true}
	if status, result := e.decide(t, reviewer, id, hard); status != http.StatusOK || result.Submission.Status != "rejected" {
		t.Fatalf("a first-stage reviewer's hard reject: %d %+v", status, result)
	}
	if status, _, _ := e.submit(t, user, projectID); status != http.StatusForbidden {
		t.Errorf("shipping again after a first-stage hard reject: %d, want 403", status)
	}
}

// A hard-rejected project's time can't be shipped, so none of it is
// unsubmitted until it's allowed to be shipped again.
func TestHardRejectedTimeIsntUnsubmitted(t *testing.T) {
	e := newEnv(t)
	reviewer := e.newReviewer(t)
	user, id := e.shipFor(t, "U1", 3)
	projectID := e.projectOf(t, id)
	unsubmitted := func() int {
		t.Helper()
		var p project
		e.mustCall(t, user, http.StatusOK, "GET", "/api/projects/"+itoa(projectID), nil, &p)
		if p.UnsubmittedSeconds == nil {
			t.Fatalf("no unsubmitted time: %q", p.HackatimeError)
		}
		return *p.UnsubmittedSeconds
	}

	hard := map[string]any{"action": "reject", "feedback": "This time was made by a script.", "internal_notes": "Script.", "block_resubmit": true}
	if status, result := e.decide(t, reviewer, id, hard); status != http.StatusOK || result.Submission.Status != "rejected" {
		t.Fatalf("hard reject: %d %+v", status, result)
	}
	if got := unsubmitted(); got != 0 {
		t.Errorf("hard-rejected project shows %d seconds unsubmitted, want none", got)
	}

	e.mustCall(t, reviewer, http.StatusOK, "PUT", "/api/review/projects/"+itoa(projectID)+"/resubmit-block", map[string]any{"blocked": false}, nil)
	if got := unsubmitted(); got != 3*3600 {
		t.Errorf("once allowed again it shows %d seconds unsubmitted, want its 3 hours back", got)
	}
}

// A hard reject takes back every ticket the project earned. Undoing it gives
// them back only if asked to, and nothing is ever taken or given twice.
func TestHardRejectForfeitsTickets(t *testing.T) {
	e := newEnv(t)
	reviewer := e.newReviewer(t)
	user := e.newUser(t, "U1")
	p := e.createProject(t, user, completeProject("racer"))
	e.approveAllTheWay(t, reviewer, e.submitHours(t, user, "U1", p, "racer", 3*3600))
	if got := e.stamps(t, user); got != 30 {
		t.Fatalf("balance after 3 approved hours is %d, want 30", got)
	}
	balance := func(want int, when string) {
		t.Helper()
		if got := e.stamps(t, user); got != want {
			t.Errorf("%s: balance is %d, want %d", when, got, want)
		}
	}
	forfeited := func() int {
		t.Helper()
		var detail struct {
			ResubmitBlock *struct {
				ForfeitedStamps int `json:"forfeited_stamps"`
			} `json:"resubmit_block"`
		}
		e.mustCall(t, reviewer, http.StatusOK, "GET", "/api/review/submissions/"+itoa(p.Submissions[0].ID), nil, &detail)
		if detail.ResubmitBlock == nil {
			return -1
		}
		return detail.ResubmitBlock.ForfeitedStamps
	}

	second := e.submitHours(t, user, "U1", p, "racer", 5*3600)
	hard := map[string]any{"action": "reject", "feedback": "This time was made by a script.", "internal_notes": "Script.", "block_resubmit": true}
	if status, result := e.decide(t, reviewer, second, hard); status != http.StatusOK || result.StampsForfeited != 30 {
		t.Fatalf("hard reject: %d %+v, want the 30 tickets taken back", status, result)
	}
	balance(0, "after the hard reject")
	e.mustCall(t, user, http.StatusOK, "GET", "/api/projects/"+itoa(p.ID), nil, &p)
	if got := forfeited(); got != 30 {
		t.Errorf("the ship page says %d tickets were taken back, want 30", got)
	}
	var actor int64
	if err := e.pool.QueryRow(context.Background(), "select actor_id from audit_log where kind = 'stamps.forfeit'").Scan(&actor); err != nil || actor != reviewer.ID {
		t.Errorf("the forfeit's audit row is by %d (%v), want the reviewer", actor, err)
	}

	block := "/api/review/projects/" + itoa(p.ID) + "/resubmit-block"
	e.mustCall(t, reviewer, http.StatusOK, "PUT", block, map[string]any{"blocked": true, "reason": "Again."}, nil)
	balance(0, "blocking it again")
	e.mustCall(t, reviewer, http.StatusOK, "PUT", block, map[string]any{"blocked": false}, nil)
	balance(0, "letting it ship again without giving them back")
	e.mustCall(t, reviewer, http.StatusOK, "PUT", block, map[string]any{"blocked": true, "reason": "Again."}, nil)
	balance(0, "blocking it after they weren't given back")
	e.mustCall(t, reviewer, http.StatusOK, "PUT", block, map[string]any{"blocked": false, "restore_stamps": true}, nil)
	balance(30, "giving them back")
	e.mustCall(t, reviewer, http.StatusOK, "PUT", block, map[string]any{"blocked": false, "restore_stamps": true}, nil)
	balance(30, "giving them back twice")

	e.mustCall(t, reviewer, http.StatusOK, "PUT", block, map[string]any{"blocked": true, "reason": "Again."}, nil)
	balance(0, "blocking it once they were given back")
	e.mustCall(t, reviewer, http.StatusNoContent, "POST", "/api/review/submissions/"+itoa(second)+"/reopen",
		map[string]any{"reason": "It was fine.", "restore_stamps": true}, nil)
	balance(30, "undoing the rejection and giving them back")
	if status := e.call(t, reviewer, "PUT", block, map[string]any{"blocked": true, "reason": "Again."}, nil); status != http.StatusConflict {
		t.Errorf("blocking it with a ship in review: %d, want 409", status)
	}
	balance(30, "the refused block")
}

// A hard-rejected project's time is forfeit, so time that also counted
// toward another project can't be shipped there instead.
func TestHardRejectedTimeCantMove(t *testing.T) {
	e := newEnv(t)
	reviewer := e.newReviewer(t)
	user := e.newUser(t, "U1")
	e.hackatime.set("U1", "client", 10*3600)
	e.hackatime.set("U1", "server", 4*3600)
	e.hackatime.overlaps = []fakeOverlap{{"U1", "client", "server", 3 * 3600}}
	client := e.createProject(t, user, completeProject("client"))
	server := e.createProject(t, user, completeProject("server"))

	_, ship, _ := e.submit(t, user, client.ID)
	hard := map[string]any{"action": "reject", "feedback": "This time was made by a script.", "internal_notes": "Script.", "block_resubmit": true}
	if status, _ := e.decide(t, reviewer, ship.ID, hard); status != http.StatusOK {
		t.Fatalf("hard reject: %d", status)
	}
	_, shipped, _ := e.submit(t, user, server.ID)
	if shipped.NewSeconds != 3600 {
		t.Errorf("server claimed %d seconds, want only the 1 hour that didn't overlap the forfeit time", shipped.NewSeconds)
	}
}

func TestUndoingARejection(t *testing.T) {
	e := newEnv(t)
	fraud := e.newStageReviewer(t, "UFRAUD", true, true, false)
	hq := e.newStageReviewer(t, "UHQ", true, true, true)
	user, id := e.shipFor(t, "U1", 3)
	projectID := e.projectOf(t, id)
	e.decide(t, fraud, id, map[string]any{"action": "reject", "feedback": "Script.", "internal_notes": "Script.", "block_resubmit": true})

	reopen := "/api/review/submissions/" + itoa(id) + "/reopen"
	if status := e.call(t, fraud, "POST", reopen, map[string]any{"reason": "Wrong person."}, nil); status != http.StatusForbidden {
		t.Errorf("the fraud check undid a rejection: %d", status)
	}
	if status := e.call(t, hq, "POST", reopen, nil, nil); status != http.StatusUnprocessableEntity {
		t.Errorf("undoing without a reason: %d, want 422", status)
	}
	e.mustCall(t, hq, http.StatusNoContent, "POST", reopen, map[string]any{"reason": "The heartbeats were a Hackatime bug."}, nil)

	d := e.detail(t, hq, id)
	if d.Submission.Status != "pending" || d.Submission.ApprovedSeconds != nil {
		t.Errorf("after undoing: %+v", d.Submission)
	}
	if got := e.eventKinds(t, hq, id); !slices.Equal(got, []string{"rejection", "reopened"}) {
		t.Errorf("events are %v", got)
	}
	var p struct {
		ResubmitBlocked bool `json:"resubmit_blocked"`
	}
	e.mustCall(t, user, http.StatusOK, "GET", "/api/projects/"+itoa(projectID), nil, &p)
	if p.ResubmitBlocked {
		t.Error("undoing the rejection left the project blocked")
	}
	if status := e.call(t, hq, "POST", reopen, map[string]any{"reason": "Again."}, nil); status != http.StatusConflict {
		t.Errorf("undoing a ship that isn't rejected: %d, want 409", status)
	}
	// Undoing isn't a review: it doesn't count toward anyone's pay.
	var reviews int
	if err := e.pool.QueryRow(context.Background(), "select count(*) from ship_events where actor_id = $1 and kind in ('approval', 'rejection', 'fraud_pass', 'final_approval', 'sent_back')", hq.ID).Scan(&reviews); err != nil {
		t.Fatal(err)
	}
	if reviews != 0 {
		t.Errorf("undoing counted as %d reviews", reviews)
	}

	// It's back in review: approve it, reject it softly, ship again, and
	// the old rejection can't be undone over the newer ship.
	e.decide(t, hq, id, map[string]any{"action": "reject", "feedback": "Needs a README."})
	if status, _, _ := e.submit(t, user, projectID); status != http.StatusCreated {
		t.Fatalf("shipping again: %d", status)
	}
	if status := e.call(t, hq, "POST", reopen, map[string]any{"reason": "Oops."}, nil); status != http.StatusConflict {
		t.Errorf("undoing a rejection after shipping again: %d, want 409", status)
	}
}
