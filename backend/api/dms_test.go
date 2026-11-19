package api_test

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/scherepi/crossroads-backend/db"
)

func (e *env) projectDMs(t *testing.T, projectID int64) []db.ProjectDm {
	t.Helper()
	dms, err := e.queries.ListProjectDMs(context.Background(), projectID)
	if err != nil {
		t.Fatal(err)
	}
	return dms
}

func dmKinds(dms []db.ProjectDm) []string {
	var kinds []string
	for _, dm := range dms {
		kinds = append(kinds, string(dm.Kind))
	}
	return kinds
}

func (e *env) sendDMs(t *testing.T) error {
	t.Helper()
	return e.api.SendProjectDMs(context.Background())
}

func TestApprovalDMsWaitForTheSignOff(t *testing.T) {
	e := newEnv(t)
	slack := e.withSlack(t)
	reviewer := e.newStageReviewer(t, "UREV", true, false, false)
	hq := e.newStageReviewer(t, "UHQ", false, false, true)
	_, id := e.shipFor(t, "U1", 3)
	projectID := e.projectOf(t, id)

	// A first approval isn't the end, so it isn't DMed.
	if status, result := e.decide(t, reviewer, id, map[string]any{"action": "approve", "feedback": "Nice!"}); status != http.StatusOK || result.Submission.Status != "awaiting_hq" {
		t.Fatalf("first approval: %d %+v", status, result)
	}
	if dms := e.projectDMs(t, projectID); len(dms) != 0 {
		t.Fatalf("DMs before the sign-off: %v", dmKinds(dms))
	}

	e.decide(t, hq, id, map[string]any{"action": "approve", "stage": "awaiting_hq"})
	if err := e.sendDMs(t); err != nil {
		t.Fatal(err)
	}
	posts := slack.called("chat.postMessage")
	if len(posts) != 1 {
		t.Fatalf("posted %d messages, want 1", len(posts))
	}
	want := "*<https://crossroads.test/projects/" + itoa(projectID) + "|p-U1>* was approved for 3h! You got 30 tickets.\nYour reviewer said:\n> Nice!"
	if got := posts[0].Body; got["channel"] != "U1" || got["username"] != "Tickets, Please" || !strings.HasSuffix(got["text"].(string), want) {
		t.Errorf("posted %+v, want %q to U1 as Tickets, Please", got, want)
	}
	if _, ok := posts[0].Body["blocks"]; ok {
		t.Error("a DM with no blocks sent a blocks field")
	}
	dms := e.projectDMs(t, projectID)
	if len(dms) != 1 || dms[0].Kind != db.ProjectDmKindApproved || dms[0].SentAt == nil || *dms[0].ChannelID != "DU1" || *dms[0].SubmissionID != id {
		t.Errorf("DM after sending: %+v", dms)
	}

	// Each one goes once.
	if err := e.sendDMs(t); err != nil {
		t.Fatal(err)
	}
	if n := len(slack.called("chat.postMessage")); n != 1 {
		t.Errorf("sending again posted %d messages in all, want 1", n)
	}
}

func TestRejectionDMs(t *testing.T) {
	e := newEnv(t)
	slack := e.withSlack(t)
	reviewer := e.newStageReviewer(t, "UREV", true, false, false)
	hq := e.newStageReviewer(t, "UHQ", true, true, true)
	user, first := e.shipFor(t, "U1", 3)
	projectID := e.projectOf(t, first)
	shipAgain := func() int64 {
		t.Helper()
		status, s, _ := e.submit(t, user, projectID)
		if status != http.StatusCreated {
			t.Fatalf("shipping again: %d", status)
		}
		return s.ID
	}
	hard := map[string]any{"action": "reject", "feedback": "This time was made by a script.", "internal_notes": "Script.", "block_resubmit": true}
	block := "/api/review/projects/" + itoa(projectID) + "/resubmit-block"

	// A soft reject, with feedback that has to be escaped for Slack.
	e.decide(t, reviewer, first, map[string]any{"action": "reject", "feedback": "Add a <demo> & a README.\n<!channel>"})
	// A hard reject, then let it be shipped again. Allowing it twice is one DM.
	e.decide(t, hq, shipAgain(), hard)
	e.mustCall(t, hq, http.StatusOK, "PUT", block, map[string]any{"blocked": false}, nil)
	e.mustCall(t, hq, http.StatusOK, "PUT", block, map[string]any{"blocked": false}, nil)
	// Blocking by hand isn't DMed, but letting it ship again is, however it
	// was blocked: they could see it was blocked on its page.
	e.mustCall(t, hq, http.StatusOK, "PUT", block, map[string]any{"blocked": true, "reason": "Same script."}, nil)
	e.mustCall(t, hq, http.StatusOK, "PUT", block, map[string]any{"blocked": false}, nil)
	// Another hard reject, undone: one DM, though it lets it ship again too.
	third := shipAgain()
	e.decide(t, hq, third, hard)
	e.mustCall(t, hq, http.StatusNoContent, "POST", "/api/review/submissions/"+itoa(third)+"/reopen", map[string]any{"reason": "A Hackatime bug."}, nil)

	dms := e.projectDMs(t, projectID)
	if got := dmKinds(dms); !slices.Equal(got, []string{"rejected", "rejected", "resubmit_allowed", "resubmit_allowed", "rejected", "reopened"}) {
		t.Fatalf("DMs are %v", got)
	}
	if err := e.sendDMs(t); err != nil {
		t.Fatal(err)
	}
	posts := slack.called("chat.postMessage")
	if len(posts) != len(dms) {
		t.Fatalf("posted %d messages for %d DMs", len(posts), len(dms))
	}
	link := "*<https://crossroads.test/projects/" + itoa(projectID) + "|p-U1>*"
	for i, want := range []string{
		link + " wasn't approved this time. Your reviewer said:\n> Add a &lt;demo&gt; &amp; a README.\n> &lt;!channel&gt;\n\nMake some changes and ship it again!",
		"Sorry, " + link + " was rejected and can't be shipped again :pensivecowboy: Your reviewer said:\n> This time was made by a script.\n\nIf you think that's a mistake, ask an organizer, like guac or max.",
		"Good news: " + link + " can be shipped again.",
		"Good news: " + link + " can be shipped again.",
		"Sorry, " + link + " was rejected",
		"Good news: the rejection of " + link + " was undone, so it's back in review.",
	} {
		if got := posts[i].Body["text"].(string); !strings.HasPrefix(got, want) {
			t.Errorf("DM %d (%s) is %q, want it to start %q", i, dms[i].Kind, got, want)
		}
	}
}

func TestDMsFromADevBuildGoToTheTestChannel(t *testing.T) {
	e := newEnv(t)
	slack := e.withSlack(t)
	e.api.DMRedirect = "CTEST"
	reviewer := e.newStageReviewer(t, "UREV", true, false, false)
	user, id := e.shipFor(t, "U1", 3)
	e.decide(t, reviewer, id, map[string]any{"action": "reject", "feedback": "Add a demo."})
	// Even someone with no Slack ID, since nobody is DMed.
	if _, err := e.pool.Exec(context.Background(), "update users set slack_id = null where id = $1", user.ID); err != nil {
		t.Fatal(err)
	}

	if err := e.sendDMs(t); err != nil {
		t.Fatal(err)
	}
	posts := slack.called("chat.postMessage")
	if len(posts) != 1 || posts[0].Body["channel"] != "CTEST" ||
		!strings.HasPrefix(posts[0].Body["text"].(string), "_To Test (no Slack ID), sent here because this isn't the real site:_\n*<") {
		t.Errorf("posted %+v", posts)
	}
}

func TestDMsSlackDoesntTake(t *testing.T) {
	e := newEnv(t)
	slack := e.withSlack(t)
	reviewer := e.newStageReviewer(t, "UREV", true, false, false)
	reject := func(slackID string) db.ProjectDm {
		t.Helper()
		_, id := e.shipFor(t, slackID, 1)
		e.decide(t, reviewer, id, map[string]any{"action": "reject", "feedback": "No."})
		dms := e.projectDMs(t, e.projectOf(t, id))
		return dms[len(dms)-1]
	}
	reload := func(dm db.ProjectDm) db.ProjectDm {
		t.Helper()
		for _, fresh := range e.projectDMs(t, dm.ProjectID) {
			if fresh.ID == dm.ID {
				return fresh
			}
		}
		t.Fatalf("DM %d is gone", dm.ID)
		return dm
	}
	posts := func() int { return len(slack.called("chat.postMessage")) }

	// Slack failing is tried again later, not on the next pass.
	down := reject("U1")
	slack.setFail("chat.postMessage", "fatal_error")
	if err := e.sendDMs(t); err == nil {
		t.Error("a failed DM wasn't reported")
	}
	down = reload(down)
	if down.SentAt != nil || down.AbandonedAt != nil || down.Attempts != 1 || down.Error == nil || !down.NextTryAt.After(time.Now()) {
		t.Errorf("after failing: %+v", down)
	}
	e.sendDMs(t)
	if posts() != 1 {
		t.Errorf("tried again straight away: %d tries", posts())
	}
	slack.setFail("chat.postMessage", "")
	if _, err := e.pool.Exec(context.Background(), "update project_dms set next_try_at = now() where id = $1", down.ID); err != nil {
		t.Fatal(err)
	}
	if err := e.sendDMs(t); err != nil {
		t.Fatal(err)
	}
	if down = reload(down); down.SentAt == nil || down.Error != nil || down.Attempts != 2 {
		t.Errorf("after Slack came back: %+v", down)
	}

	// Being rate limited stops the pass.
	limited := []db.ProjectDm{reject("U2"), reject("U3")}
	slack.setFail("chat.postMessage", "ratelimited")
	before := posts()
	e.sendDMs(t)
	if posts() != before+1 {
		t.Errorf("rate limited, but tried %d DMs", posts()-before)
	}
	slack.setFail("chat.postMessage", "")
	if _, err := e.pool.Exec(context.Background(), "update project_dms set next_try_at = now()"); err != nil {
		t.Fatal(err)
	}
	e.sendDMs(t)
	for _, dm := range limited {
		if reload(dm).SentAt == nil {
			t.Errorf("DM %d wasn't sent once the rate limit passed", dm.ID)
		}
	}

	// Slack can't deliver it, they have no Slack ID, or it's too old: given
	// up on, without asking Slack again.
	undeliverable := reject("U4")
	slack.setFail("chat.postMessage", "channel_not_found")
	e.sendDMs(t)
	slack.setFail("chat.postMessage", "")
	noSlack := reject("U5")
	if _, err := e.pool.Exec(context.Background(), "update users set slack_id = null where id = $1", noSlack.UserID); err != nil {
		t.Fatal(err)
	}
	old := reject("U6")
	if _, err := e.pool.Exec(context.Background(), "update project_dms set created_at = now() - interval '2 days' where id = $1", old.ID); err != nil {
		t.Fatal(err)
	}
	before = posts()
	e.sendDMs(t)
	if _, err := e.pool.Exec(context.Background(), "update project_dms set next_try_at = now()"); err != nil {
		t.Fatal(err)
	}
	e.sendDMs(t)
	if posts() != before {
		t.Errorf("asked Slack %d more times", posts()-before)
	}
	for _, dm := range []db.ProjectDm{undeliverable, noSlack, old} {
		if dm = reload(dm); dm.AbandonedAt == nil || dm.SentAt != nil || dm.Error == nil {
			t.Errorf("DM to %d wasn't given up on: %+v", dm.UserID, dm)
		}
	}

	// Without a bot token, DMs wait.
	e.api.Slack = nil
	waiting := reject("U7")
	if err := e.sendDMs(t); err != nil {
		t.Fatal(err)
	}
	if waiting = reload(waiting); waiting.SentAt != nil || waiting.AbandonedAt != nil || waiting.Attempts != 0 {
		t.Errorf("without a token: %+v", waiting)
	}
}

func TestDMsCanBeTurnedOff(t *testing.T) {
	e := newEnv(t)
	slack := e.withSlack(t)
	reviewer := e.newStageReviewer(t, "UREV", true, false, false)
	user, first := e.shipFor(t, "U1", 3)
	projectID := e.projectOf(t, first)

	// Turning them off stops what's waiting, too.
	e.decide(t, reviewer, first, map[string]any{"action": "reject", "feedback": "Add a demo."})
	e.mustCall(t, user, http.StatusOK, "PATCH", "/api/me", map[string]any{"dms_enabled": false}, nil)
	if err := e.sendDMs(t); err == nil {
		t.Error("a DM that wasn't sent wasn't reported")
	}
	if posts := slack.called("chat.postMessage"); len(posts) != 0 {
		t.Fatalf("DMed someone who turned DMs off: %+v", posts)
	}
	dms := e.projectDMs(t, projectID)
	if len(dms) != 1 || dms[0].AbandonedAt == nil || dms[0].Error == nil || *dms[0].Error != "They turned DMs off." {
		t.Fatalf("after turning DMs off: %+v", dms)
	}

	// Turned back on, the next one goes.
	user = e.reload(t, user)
	e.mustCall(t, user, http.StatusOK, "PATCH", "/api/me", map[string]any{"dms_enabled": true}, nil)
	status, s, _ := e.submit(t, user, projectID)
	if status != http.StatusCreated {
		t.Fatalf("shipping again: %d", status)
	}
	e.decide(t, reviewer, s.ID, map[string]any{"action": "reject", "feedback": "Still no demo."})
	if err := e.sendDMs(t); err != nil {
		t.Fatal(err)
	}
	if posts := slack.called("chat.postMessage"); len(posts) != 1 {
		t.Errorf("posted %d DMs after turning them back on, want 1", len(posts))
	}
}
