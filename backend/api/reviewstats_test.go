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
	if status := e.call(t, fern, "GET", "/api/admin/reviewer-stats", nil, nil); status != http.StatusForbidden {
		t.Errorf("a reviewer got the pay view: %d", status)
	}
	if status := e.call(t, maker, "GET", "/api/review/stats", nil, nil); status != http.StatusForbidden {
		t.Errorf("a participant got the leaderboard: %d", status)
	}
}
