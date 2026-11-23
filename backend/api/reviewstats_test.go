package api_test

import (
	"context"
	"encoding/csv"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/scherepi/crossroads-backend/auth"
	"github.com/scherepi/crossroads-backend/db"
)

// payRow is a row of GET /api/admin/reviewer-stats.
type payRow struct {
	Name                 string `json:"name"`
	Email                string `json:"email"`
	Reviews              int64  `json:"reviews"`
	FirstReviews         int64  `json:"first_reviews"`
	FirstApprovals       int64  `json:"first_approvals"`
	FirstRejections      int64  `json:"first_rejections"`
	FraudChecks          int64  `json:"fraud_checks"`
	HQDecisions          int64  `json:"hq_decisions"`
	LaterSentBack        int64  `json:"later_sent_back"`
	SecondsFirstReviewed int64  `json:"seconds_first_reviewed"`
}

// The counts reviewers are paid by: one per decision, by stage, and nothing
// for comments; a first review that's later sent back still counts, and is
// marked.
func TestReviewerStats(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	rita := e.newStageReviewer(t, "URITA", true, false, false)
	fern := e.newStageReviewer(t, "UFERN", false, true, false)
	hugo := e.newStageReviewer(t, "UHUGO", false, false, true)
	admin := e.newAdmin(t)
	for id, name := range map[int64]string{rita.ID: "Rita", fern.ID: "Fern", hugo.ID: "Hugo"} {
		if _, err := e.pool.Exec(ctx, "update users set name = $2, email = $3 where id = $1", id, name, strings.ToLower(name)+"@example.com"); err != nil {
			t.Fatal(err)
		}
	}
	maker, ship := e.shipFor(t, "UMAKER", 10)
	_, other := e.shipFor(t, "UOTHER", 4)
	if _, err := e.pool.Exec(ctx, "update projects set name = '=HYPERLINK(\"evil\")' where user_id = $1", maker.ID); err != nil {
		t.Fatal(err)
	}

	decide := func(who *db.User, id int64, body map[string]any) {
		t.Helper()
		if status, result := e.decide(t, who, id, body); status != http.StatusOK {
			t.Fatalf("%v: got %d %q", body, status, result.Error)
		}
	}
	decide(rita, ship, map[string]any{"action": "approve", "send_to_fraud": true})
	decide(fern, ship, map[string]any{"action": "approve"})
	decide(hugo, ship, map[string]any{"action": "send_back", "internal_notes": "Check day 3."})
	decide(rita, ship, map[string]any{"action": "approve"})
	decide(hugo, ship, map[string]any{"action": "approve"})
	decide(rita, other, map[string]any{"action": "reject", "feedback": "The demo is broken."})
	if status := e.call(t, rita, "POST", "/api/review/submissions/"+itoa(other)+"/comments",
		map[string]any{"message": "Comments don't count.", "internal": true}, nil); status != http.StatusCreated && status != http.StatusOK {
		t.Fatalf("comment: %d", status)
	}

	newYork, _ := time.LoadLocation("America/New_York")
	today := time.Now().In(newYork).Format(time.DateOnly)
	var pay struct {
		From, To  string
		Reviews   int64
		Reviewers []payRow
	}
	e.mustCall(t, admin, http.StatusOK, "GET", "/api/admin/reviewer-stats?from="+today+"&to="+today, nil, &pay)
	by := map[string]payRow{}
	for _, r := range pay.Reviewers {
		by[r.Name] = r
	}
	if pay.Reviews != 6 || len(pay.Reviewers) != 3 {
		t.Fatalf("%d reviews by %d people, want 6 by 3: %+v", pay.Reviews, len(pay.Reviewers), pay.Reviewers)
	}
	if r := by["Rita"]; r.Reviews != 3 || r.FirstReviews != 3 || r.FirstApprovals != 2 || r.FirstRejections != 1 ||
		r.LaterSentBack != 1 || r.SecondsFirstReviewed != 24*3600 || r.Email != "rita@example.com" {
		t.Errorf("Rita: %+v", r)
	}
	if r := by["Fern"]; r.Reviews != 1 || r.FraudChecks != 1 || r.FirstReviews != 0 {
		t.Errorf("Fern: %+v", r)
	}
	if r := by["Hugo"]; r.Reviews != 2 || r.HQDecisions != 2 {
		t.Errorf("Hugo: %+v", r)
	}
	if pay.Reviewers[0].Name != "Rita" {
		t.Errorf("most reviews first: %+v", pay.Reviewers)
	}

	// Another period has none of them; a bad one is refused.
	e.mustCall(t, admin, http.StatusOK, "GET", "/api/admin/reviewer-stats?from=2026-01-01&to=2026-01-31", nil, &pay)
	if pay.Reviews != 0 || len(pay.Reviewers) != 0 {
		t.Errorf("January: %+v", pay)
	}
	for _, q := range []string{"from=2026-02-01&to=2026-01-01", "from=yesterday&to=today", "from=2026-01-01"} {
		if status := e.call(t, admin, "GET", "/api/admin/reviewer-stats?"+q, nil, nil); status != http.StatusUnprocessableEntity {
			t.Errorf("%s: got %d, want 422", q, status)
		}
	}

	// The line items add up to the counts, and can be narrowed to one person.
	var log []struct {
		ReviewerName  string `json:"reviewer_name"`
		Stage, Kind   string
		LaterSentBack bool `json:"later_sent_back"`
	}
	e.mustCall(t, admin, http.StatusOK, "GET", "/api/admin/reviews?from="+today+"&to="+today, nil, &log)
	if len(log) != 6 {
		t.Fatalf("%d reviews in the log, want 6", len(log))
	}
	if !log[0].LaterSentBack || log[0].ReviewerName != "Rita" || log[3].LaterSentBack {
		t.Errorf("sent-back marks: %+v", log)
	}
	e.mustCall(t, admin, http.StatusOK, "GET", "/api/admin/reviews?from="+today+"&to="+today+"&reviewer="+itoa(hugo.ID), nil, &log)
	if len(log) != 2 || log[0].Kind != "sent_back" || log[1].Kind != "final_approval" {
		t.Errorf("Hugo's reviews: %+v", log)
	}

	// The CSV has the same rows, and a project named like a formula can't run as one.
	req := httptest.NewRequest("GET", "/api/admin/reviews?from="+today+"&to="+today+"&format=csv", nil)
	req = req.WithContext(auth.WithUser(req.Context(), admin))
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/csv") ||
		!strings.Contains(rec.Header().Get("Content-Disposition"), "attachment") {
		t.Fatalf("CSV: %d %v", rec.Code, rec.Header())
	}
	records, err := csv.NewReader(rec.Body).ReadAll()
	if err != nil || len(records) != 7 {
		t.Fatalf("CSV: %d rows, %v", len(records), err)
	}
	if records[1][3] != "First review" || records[1][4] != "Approved" || records[1][10] != "yes" ||
		!strings.HasPrefix(records[1][6], "'=") {
		t.Errorf("first CSV row: %q", records[1])
	}

	// Reviewers see the leaderboard with names only; only admins see the pay view.
	var stats struct {
		Today string `json:"today"`
		Days  []struct {
			Day     string `json:"day"`
			Reviews int64  `json:"reviews"`
		} `json:"days"`
		Week []struct {
			Name    string `json:"name"`
			Reviews int64  `json:"reviews"`
		} `json:"week"`
		Pace struct {
			Reviewed int64 `json:"reviewed"`
			Waiting  int64 `json:"waiting"`
		} `json:"pace"`
	}
	req = httptest.NewRequest("GET", "/api/review/stats", nil)
	req = req.WithContext(auth.WithUser(req.Context(), fern))
	rec = httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "@example.com") {
		t.Fatalf("stats: %d, leaks an email: %v", rec.Code, strings.Contains(rec.Body.String(), "@example.com"))
	}
	e.mustCall(t, fern, http.StatusOK, "GET", "/api/review/stats", nil, &stats)
	if len(stats.Week) != 3 || stats.Week[0].Name != "Rita" || stats.Week[0].Reviews != 3 || stats.Pace.Reviewed != 6 {
		t.Errorf("leaderboard: %+v, pace %+v", stats.Week, stats.Pace)
	}
	// Seven days, Monday first, and today has all six.
	if len(stats.Days) != 7 || stats.Today != today {
		t.Fatalf("days: %+v, today %q", stats.Days, stats.Today)
	}
	if monday, _ := time.Parse(time.DateOnly, stats.Days[0].Day); monday.Weekday() != time.Monday {
		t.Errorf("the week starts on %s", stats.Days[0].Day)
	}
	for _, d := range stats.Days {
		if want := map[bool]int64{true: 6, false: 0}[d.Day == today]; d.Reviews != want {
			t.Errorf("%s: %d reviews, want %d", d.Day, d.Reviews, want)
		}
	}
	if status := e.call(t, fern, "GET", "/api/admin/reviewer-stats", nil, nil); status != http.StatusForbidden {
		t.Errorf("a reviewer got the pay view: %d", status)
	}
	if status := e.call(t, maker, "GET", "/api/review/stats", nil, nil); status != http.StatusForbidden {
		t.Errorf("a participant got the leaderboard: %d", status)
	}
}

// The review page's Review stats tab: every reviewer sees everyone's counts,
// but no emails or Slack IDs, and lists only their own reviews. Admins see
// what the admin page does.
func TestReviewStatsTab(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	rita := e.newStageReviewer(t, "URITA", true, false, false)
	fern := e.newStageReviewer(t, "UFERN", true, false, false)
	admin := e.newAdmin(t)
	maker, ship := e.shipFor(t, "UMAKER", 10)
	_, other := e.shipFor(t, "UOTHER", 4)
	for _, u := range []*db.User{rita, fern} {
		if _, err := e.pool.Exec(ctx, "update users set email = $2 where id = $1", u.ID, *u.SlackID+"@example.com"); err != nil {
			t.Fatal(err)
		}
	}
	for who, id := range map[*db.User]int64{rita: ship, fern: other} {
		if status, result := e.decide(t, who, id, map[string]any{"action": "approve"}); status != http.StatusOK {
			t.Fatalf("approving: %d %q", status, result.Error)
		}
	}

	newYork, _ := time.LoadLocation("America/New_York")
	today := time.Now().In(newYork).Format(time.DateOnly)
	period := "?from=" + today + "&to=" + today
	raw := func(u *db.User, path string) (int, string) {
		t.Helper()
		req := httptest.NewRequest("GET", path, nil)
		req = req.WithContext(auth.WithUser(req.Context(), u))
		rec := httptest.NewRecorder()
		e.router.ServeHTTP(rec, req)
		return rec.Code, rec.Body.String()
	}

	status, body := raw(rita, "/api/review/reviewer-stats"+period)
	if status != http.StatusOK || strings.Contains(body, "@example.com") || strings.Contains(body, "slack_id") {
		t.Fatalf("a reviewer's counts: %d %s", status, body)
	}
	var counts struct {
		Reviews   int64    `json:"reviews"`
		Reviewers []payRow `json:"reviewers"`
	}
	e.mustCall(t, rita, http.StatusOK, "GET", "/api/review/reviewer-stats"+period, nil, &counts)
	if counts.Reviews != 2 || len(counts.Reviewers) != 2 {
		t.Errorf("a reviewer sees %+v", counts)
	}
	if status, _ := raw(rita, "/api/review/reviewer-stats"+period+"&format=csv"); status != http.StatusForbidden {
		t.Errorf("a reviewer downloaded everyone's counts: %d", status)
	}

	var log []struct {
		ReviewerID int64 `json:"reviewer_id"`
	}
	e.mustCall(t, rita, http.StatusOK, "GET", "/api/review/reviews"+period, nil, &log)
	if len(log) != 1 || log[0].ReviewerID != rita.ID {
		t.Errorf("a reviewer's own reviews: %+v", log)
	}
	if status, _ := raw(rita, "/api/review/reviews"+period+"&reviewer="+itoa(fern.ID)); status != http.StatusForbidden {
		t.Errorf("a reviewer listed someone else's reviews: %d", status)
	}
	if status, body := raw(rita, "/api/review/reviews"+period+"&format=csv"); status != http.StatusOK || strings.Contains(body, "UOTHER") {
		t.Errorf("a reviewer's own CSV: %d %s", status, body)
	}

	// Admins get the emails and anyone's reviews.
	if status, body := raw(admin, "/api/review/reviewer-stats"+period); status != http.StatusOK || !strings.Contains(body, "UFERN@example.com") {
		t.Errorf("an admin's counts: %d %s", status, body)
	}
	e.mustCall(t, admin, http.StatusOK, "GET", "/api/review/reviews"+period+"&reviewer="+itoa(fern.ID), nil, &log)
	if len(log) != 1 || log[0].ReviewerID != fern.ID {
		t.Errorf("an admin's look at Fern's reviews: %+v", log)
	}
	if status, _ := raw(maker, "/api/review/reviewer-stats"+period); status != http.StatusForbidden {
		t.Errorf("a participant got the counts: %d", status)
	}
}

// The Review stats tab's queue numbers, and the admin Stats tab's hours
// funnel.
func TestReviewInsightsAndHoursFunnel(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	rita := e.newStageReviewer(t, "URITA", true, false, false)
	hugo := e.newStageReviewer(t, "UHUGO", true, false, true) // can review first and hard reject
	admin := e.newAdmin(t)
	_, signed := e.shipFor(t, "U1", 5)
	_, soft := e.shipFor(t, "U2", 3)
	_, hard := e.shipFor(t, "U3", 2)
	_, waiting := e.shipFor(t, "U4", 1)
	decide := func(who *db.User, id int64, body map[string]any) {
		t.Helper()
		if status, result := e.decide(t, who, id, body); status != http.StatusOK {
			t.Fatalf("%v: got %d %q", body, status, result.Error)
		}
	}
	four := int32(4 * 3600)
	decide(rita, signed, map[string]any{"action": "approve", "approved_seconds": four})
	decide(hugo, signed, map[string]any{"action": "approve"})
	decide(rita, soft, map[string]any{"action": "reject", "feedback": "Add a demo."})
	decide(hugo, hard, map[string]any{"action": "reject", "block_resubmit": true, "feedback": "This was copied.", "internal_notes": "Copied."})
	if _, err := e.pool.Exec(ctx, "update submissions set unified_submitted_at = now() where id = $1", signed); err != nil {
		t.Fatal(err)
	}

	var in struct {
		Waiting []struct {
			Stage    string `json:"stage"`
			Waiting  int64  `json:"waiting"`
			OldestID *int64 `json:"oldest_id"`
		} `json:"waiting"`
		Decisions []struct {
			Stage    string `json:"stage"`
			Passed   int64  `json:"passed"`
			Rejected int64  `json:"rejected"`
		} `json:"decisions"`
		SecondsSignedOff int64 `json:"seconds_signed_off"`
		Days             []struct {
			Reviews int64 `json:"reviews"`
		} `json:"days"`
	}
	e.mustCall(t, rita, http.StatusOK, "GET", "/api/review/insights", nil, &in)
	if len(in.Waiting) != 3 || in.Waiting[0].Waiting != 1 || in.Waiting[0].OldestID == nil || *in.Waiting[0].OldestID != waiting || in.Waiting[2].Waiting != 0 {
		t.Errorf("waiting: %+v", in.Waiting)
	}
	if d := in.Decisions; len(d) != 3 || d[0].Passed != 1 || d[0].Rejected != 2 || d[2].Passed != 1 {
		t.Errorf("decisions: %+v", d)
	}
	var raw struct {
		Funnel map[string]int64 `json:"funnel"`
	}
	e.mustCall(t, rita, http.StatusOK, "GET", "/api/review/insights", nil, &raw)
	want := map[string]int64{"shipped": 4, "first_reviewed": 3, "passed_first_review": 1, "reached_hq": 1, "signed_off": 1, "rejected": 2, "in_review": 1}
	for k, v := range want {
		if raw.Funnel[k] != v {
			t.Errorf("funnel %s: %d, want %d", k, raw.Funnel[k], v)
		}
	}
	if in.SecondsSignedOff != int64(four) {
		t.Errorf("signed off %d seconds", in.SecondsSignedOff)
	}
	var reviews int64
	for _, d := range in.Days {
		reviews += d.Reviews
	}
	if reviews != 4 {
		t.Errorf("%d reviews in the days, want 4", reviews)
	}

	var stats struct {
		Days []struct {
			ShippedSeconds int64 `json:"shipped_seconds"`
		} `json:"days"`
		People []struct {
			Key    string `json:"key"`
			People int64  `json:"people"`
		} `json:"people"`
		Hours struct {
			Nodes []struct {
				Key     string `json:"key"`
				Seconds int64  `json:"seconds"`
				Ships   int64  `json:"ships"`
			} `json:"nodes"`
			Links []struct {
				Source, Target string
				Seconds        int64 `json:"seconds"`
			} `json:"links"`
		} `json:"hours"`
	}
	e.mustCall(t, admin, http.StatusOK, "GET", "/api/admin/activity?days=7", nil, &stats)
	h := int64(3600)
	type at struct{ seconds, ships int64 }
	got := map[string]at{}
	for _, n := range stats.Hours.Nodes {
		got[n.Key] = at{n.Seconds, n.Ships}
	}
	for key, want := range map[string]at{"shipped": {11 * h, 4}, "signed_off": {5 * h, 1}, "review": {h, 1}, "soft_rejected": {3 * h, 1},
		"hard_rejected": {2 * h, 1}, "approved": {4 * h, 1}, "cut": {h, 1}, "unified": {4 * h, 1}, "waiting_unified": {0, 0}} {
		if got[key] != want {
			t.Errorf("hours flow %s: %+v, want %+v", key, got[key], want)
		}
	}
	// Every node but the first and the added hours gets what flows into it.
	inflow := map[string]int64{}
	for _, l := range stats.Hours.Links {
		inflow[l.Target] += l.Seconds
	}
	for _, n := range stats.Hours.Nodes {
		if n.Key != "logged" && n.Key != "added" && n.Key != "shipped" && inflow[n.Key] != n.Seconds {
			t.Errorf("%s holds %d but gets %d", n.Key, n.Seconds, inflow[n.Key])
		}
	}
	people := map[string]int64{}
	for _, step := range stats.People {
		people[step.Key] = step.People
	}
	for key, want := range map[string]int64{"signed_up": 4, "made_project": 4, "linked_hackatime": 4, "shipped": 4, "approved": 1, "reached_goal": 0, "rail_pass": 0} {
		if people[key] != want {
			t.Errorf("participant funnel %s: %d, want %d", key, people[key], want)
		}
	}
	if last := stats.Days[len(stats.Days)-1]; last.ShippedSeconds != int64(11*h) {
		t.Errorf("shipped today: %d", last.ShippedSeconds)
	}
}
