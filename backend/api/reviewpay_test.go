package api_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/scherepi/crossroads-backend/db"
)

type reviewerPay struct {
	Reviewers []struct {
		ID          int64  `json:"id"`
		Reviews     int64  `json:"reviews"`
		Unpaid      int64  `json:"unpaid"`
		PaidTickets int64  `json:"paid_tickets"`
		PaidCents   *int64 `json:"paid_cents"`
		Days        []struct {
			Day     string `json:"day"`
			Reviews int64  `json:"reviews"`
			Unpaid  int64  `json:"unpaid"`
		} `json:"days"`
	} `json:"reviewers"`
	Payments []struct {
		Currency string `json:"currency"`
		Amount   int    `json:"amount"`
		Note     string `json:"note"`
	} `json:"payments"`
	CanPayInUSD bool `json:"can_pay_in_usd"`
}

func TestPayingReviewers(t *testing.T) {
	e := newEnv(t)
	reviewer := e.newStageReviewer(t, "UREV", true, false, false)
	admin := e.newAdmin(t)
	super := e.newSuperadmin(t)
	_, first := e.shipFor(t, "U1", 3)
	_, second := e.shipFor(t, "U2", 2)
	e.decide(t, reviewer, first, map[string]any{"action": "approve"})
	e.decide(t, reviewer, second, map[string]any{"action": "reject", "feedback": "no demo"})

	const url = "/api/superadmin/reviewer-pay"
	read := func() reviewerPay {
		t.Helper()
		var got reviewerPay
		e.mustCall(t, super, http.StatusOK, "GET", url, nil, &got)
		return got
	}
	got := read()
	if len(got.Reviewers) != 1 || got.Reviewers[0].Reviews != 2 || got.Reviewers[0].Unpaid != 2 || len(got.Reviewers[0].Days) != 1 {
		t.Fatalf("before paying: %+v", got.Reviewers)
	}
	day := got.Reviewers[0].Days[0].Day
	body := func(currency string, rate, reviews int) map[string]any {
		return map[string]any{"currency": currency, "rate": rate,
			"payments": []map[string]any{{"reviewer_id": reviewer.ID, "days": []string{day}, "reviews": reviews}}}
	}

	// Refused: only superadmins can see it or pay, nobody can pay
	// themselves, and a count that doesn't match what's there pays nothing.
	for _, who := range []*db.User{reviewer, admin} {
		if status := e.call(t, who, "GET", url, nil, nil); status != http.StatusForbidden {
			t.Errorf("%s reading it: got %d, want 403", who.Role, status)
		}
		if status := e.call(t, who, "POST", url, body("tickets", 5, 2), nil); status != http.StatusForbidden {
			t.Errorf("%s paying: got %d, want 403", who.Role, status)
		}
	}
	self := map[string]any{"rate": 5, "payments": []map[string]any{{"reviewer_id": super.ID, "days": []string{day}, "reviews": 1}}}
	if status := e.call(t, super, "POST", url, self, nil); status != http.StatusForbidden {
		t.Errorf("paying yourself: got %d, want 403", status)
	}
	if status := e.call(t, super, "POST", url, body("tickets", 5, 3), nil); status != http.StatusConflict {
		t.Errorf("paying for reviews that aren't there: got %d, want 409", status)
	}
	if status := e.call(t, super, "POST", url, body("tickets", 0, 2), nil); status != http.StatusUnprocessableEntity {
		t.Errorf("a rate of 0: got %d, want 422", status)
	}
	if e.stamps(t, reviewer) != 0 {
		t.Fatal("a refused payment paid something")
	}

	// Tickets: they go to the reviewer, and the reviews are paid.
	e.mustCall(t, super, http.StatusCreated, "POST", url, body("tickets", 5, 2), nil)
	if got := e.stamps(t, reviewer); got != 10 {
		t.Errorf("the reviewer has %d tickets, want 10", got)
	}
	if status := e.call(t, super, "POST", url, body("tickets", 5, 2), nil); status != http.StatusConflict {
		t.Errorf("paying for the same reviews again: got %d, want 409", status)
	}
	got = read()
	if r := got.Reviewers[0]; r.Unpaid != 0 || r.PaidTickets != 10 {
		t.Errorf("after paying: %+v", r)
	}

	// Dollars: recorded, not sent.
	_, third := e.shipFor(t, "U3", 1)
	e.decide(t, reviewer, third, map[string]any{"action": "reject", "feedback": "no demo"})
	e.mustCall(t, super, http.StatusCreated, "POST", url, body("usd", 250, 1), nil)
	if got := e.stamps(t, reviewer); got != 10 {
		t.Errorf("a dollar payment changed their tickets: %d", got)
	}
	got = read()
	if r := got.Reviewers[0]; r.Unpaid != 0 || r.PaidCents == nil || *r.PaidCents != 250 {
		t.Errorf("after paying dollars: %+v", got)
	}
	if len(got.Payments) != 2 || got.Payments[0].Currency != "usd" || !strings.HasPrefix(got.Payments[0].Note, "Reviewer pay: 1 review at $2.50 each, ") {
		t.Errorf("payments: %+v", got.Payments)
	}

	// Both are in the audit log.
	var n int
	if err := e.pool.QueryRow(context.Background(), "select count(*) from audit_log where kind like 'review_pay.%'").Scan(&n); err != nil || n != 2 {
		t.Errorf("audit log has %d review payments (%v), want 2", n, err)
	}
	// And nothing can change them.
	if _, err := e.pool.Exec(context.Background(), "delete from review_payment_events"); err == nil {
		t.Error("a paid review was unmarked")
	}
}

// A reviewer changing a review that was sent back to them isn't paid for it
// again; someone else picking the ship up is a new review.
func TestRedoneReviewsArentPaidAgain(t *testing.T) {
	e := newEnv(t)
	rita := e.newStageReviewer(t, "URITA", true, false, false)
	omar := e.newStageReviewer(t, "UOMAR", true, false, false)
	hq := e.newStageReviewer(t, "UHQ", false, false, true)
	super := e.newSuperadmin(t)
	_, mine := e.shipFor(t, "U1", 3)
	_, theirs := e.shipFor(t, "U2", 3)

	for _, ship := range []int64{mine, theirs} {
		e.decide(t, rita, ship, map[string]any{"action": "approve"})
		if status, result := e.decide(t, hq, ship, map[string]any{"action": "send_back", "internal_notes": "say more"}); status != http.StatusOK {
			t.Fatalf("sending back: got %d %q", status, result.Error)
		}
	}
	// Rita changes hers; Omar picks up the other while she's away.
	if status, result := e.decide(t, rita, mine, map[string]any{"action": "approve"}); status != http.StatusOK {
		t.Fatalf("Rita's redo: got %d %q", status, result.Error)
	}
	e.mustCall(t, omar, http.StatusOK, "POST", "/api/review/submissions/"+itoa(theirs)+"/claim", nil, nil)
	if status, result := e.decide(t, omar, theirs, map[string]any{"action": "approve"}); status != http.StatusOK {
		t.Fatalf("Omar's review: got %d %q", status, result.Error)
	}

	var got reviewerPay
	e.mustCall(t, super, http.StatusOK, "GET", "/api/superadmin/reviewer-pay", nil, &got)
	want := map[int64]int64{rita.ID: 2, omar.ID: 1, hq.ID: 2} // Rita's two first reviews, not her redo; HQ's send-backs
	for _, r := range got.Reviewers {
		if r.Unpaid != want[r.ID] {
			t.Errorf("reviewer %d has %d payable reviews, want %d", r.ID, r.Unpaid, want[r.ID])
		}
		delete(want, r.ID)
	}
	if len(want) > 0 {
		t.Errorf("missing reviewers: %v", want)
	}
}
