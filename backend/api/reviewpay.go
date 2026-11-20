package api

// Paying reviewers per review (the admin page's Reviewer pay tab, for
// superadmins only): in tickets, which go to the reviewer as a stamp
// adjustment, or in US dollars, which only keeps track (the money's sent some
// other way). Either marks the reviews it covers as paid, so none is paid
// twice. See migration 38.

import (
	"cmp"
	"errors"
	"fmt"
	"math"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/scherepi/crossroads-backend/auth"
	"github.com/scherepi/crossroads-backend/db"
	"github.com/scherepi/crossroads-backend/respond"
)

const (
	payInTickets = "tickets"
	payInUSD     = "usd"
)

// maxRate is the most a review can be paid, per currency: 1,000 tickets, or
// $1,000 in cents.
var maxRate = map[string]int64{payInTickets: 1000, payInUSD: 100_000}

type payableDayJSON struct {
	Day     string `json:"day"` // YYYY-MM-DD, Eastern
	Reviews int64  `json:"reviews"`
	Unpaid  int64  `json:"unpaid"`
}

type payableReviewerJSON struct {
	ID          int64            `json:"id"`
	Name        string           `json:"name"`
	SlackID     *string          `json:"slack_id"`
	Reviews     int64            `json:"reviews"`
	Unpaid      int64            `json:"unpaid"`
	PaidTickets int64            `json:"paid_tickets"`
	PaidCents   *int64           `json:"paid_cents,omitempty"` // superadmins only
	Days        []payableDayJSON `json:"days"`                 // newest first
}

type reviewPaymentJSON struct {
	ID           int64     `json:"id"`
	ReviewerID   int64     `json:"reviewer_id"`
	ReviewerName string    `json:"reviewer_name"`
	PaidByName   string    `json:"paid_by_name"`
	Currency     string    `json:"currency"` // tickets or usd
	Reviews      int32     `json:"reviews"`
	Rate         int32     `json:"rate"`   // per review: tickets, or US cents
	Amount       int32     `json:"amount"` // tickets, or US cents
	Note         string    `json:"note"`
	CreatedAt    time.Time `json:"created_at"`
}

// reviewerToPay is one reviewer in a payment: which of their days, and how
// many unpaid reviews the admin saw on them.
type reviewerToPay struct {
	ReviewerID int64    `json:"reviewer_id"`
	Days       []string `json:"days"`
	Reviews    int64    `json:"reviews"`
}

// canPayInUSD is whether someone can see and record dollar payments.
func canPayInUSD(u *db.User) bool { return u.Role == db.UserRoleSuperadmin }

// adminReviewerPay is GET /superadmin/reviewer-pay: everyone who has reviewed,
// with their reviews by day and how many of those are unpaid, and the latest
// payments. Superadmins also get dollar amounts and payments.
func (a *API) adminReviewerPay(w http.ResponseWriter, r *http.Request) {
	viewer := auth.User(r.Context())
	usd := canPayInUSD(viewer)
	people, err := a.Queries.ListReviewersForPay(r.Context())
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	days, err := a.Queries.ReviewerPayDays(r.Context())
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	byReviewer := map[int64][]payableDayJSON{}
	for _, d := range days {
		byReviewer[d.ReviewerID] = append(byReviewer[d.ReviewerID], payableDayJSON{Day: d.Day, Reviews: d.Reviews, Unpaid: d.Unpaid})
	}
	reviewers := []payableReviewerJSON{}
	for _, p := range people {
		out := payableReviewerJSON{ID: p.ID, Name: p.Name, SlackID: p.SlackID, PaidTickets: p.PaidTickets, Days: byReviewer[p.ID]}
		if usd {
			out.PaidCents = &p.PaidCents
		}
		if out.Days == nil {
			out.Days = []payableDayJSON{}
		}
		for _, d := range out.Days {
			out.Reviews += d.Reviews
			out.Unpaid += d.Unpaid
		}
		reviewers = append(reviewers, out)
	}
	currencies := []string{payInTickets}
	if usd {
		currencies = append(currencies, payInUSD)
	}
	rows, err := a.Queries.ListReviewPayments(r.Context(), currencies)
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	payments := []reviewPaymentJSON{}
	for _, row := range rows {
		p := row.ReviewPayment
		payments = append(payments, reviewPaymentJSON{ID: p.ID, ReviewerID: p.ReviewerID, ReviewerName: row.ReviewerName,
			PaidByName: row.PaidByName, Currency: p.Currency, Reviews: p.Reviews, Rate: p.Rate, Amount: p.Amount,
			Note: p.Note, CreatedAt: p.CreatedAt})
	}
	respond.JSON(w, http.StatusOK, struct {
		Reviewers   []payableReviewerJSON `json:"reviewers"`
		Payments    []reviewPaymentJSON   `json:"payments"`
		CanPayInUSD bool                  `json:"can_pay_in_usd"`
	}{reviewers, payments, usd})
}

// adminPayReviewers is POST /superadmin/reviewer-pay
//
//	{"currency": "tickets", "rate": 5,
//	 "payments": [{"reviewer_id": 3, "days": ["2026-11-17", "2026-11-18"], "reviews": 14}]}
//
// pays each reviewer for their unpaid reviews on those days (Eastern) at
// rate per review: tickets, or US cents ("usd", superadmins only, recorded
// but not sent). reviews is how many unpaid reviews the admin saw; if it
// isn't what's there now, nothing is paid (409), so nobody's paid for
// reviews the admin didn't see. All the payments are made or none are.
// Nobody can pay themselves.
func (a *API) adminPayReviewers(w http.ResponseWriter, r *http.Request) {
	admin := auth.User(r.Context())
	var body struct {
		Currency string          `json:"currency"`
		Rate     int64           `json:"rate"`
		Payments []reviewerToPay `json:"payments"`
	}
	if !decode(w, r, &body) {
		return
	}
	body.Currency = cmp.Or(body.Currency, payInTickets)
	limit, ok := maxRate[body.Currency]
	switch {
	case !ok:
		respond.Error(w, http.StatusUnprocessableEntity, `currency has to be "tickets" or "usd".`)
		return
	case body.Currency == payInUSD && !canPayInUSD(admin):
		respond.Error(w, http.StatusForbidden, "Only superadmins can record dollar payments.")
		return
	case body.Rate < 1 || body.Rate > limit:
		respond.Error(w, http.StatusUnprocessableEntity, fmt.Sprintf("The rate has to be between 1 and %d per review.", limit))
		return
	case len(body.Payments) == 0:
		respond.Error(w, http.StatusUnprocessableEntity, "Pick some reviews to pay for.")
		return
	case len(body.Payments) > 500:
		respond.Error(w, http.StatusUnprocessableEntity, "That's too many reviewers at once.")
		return
	}
	seen := map[int64]bool{}
	for i, p := range body.Payments {
		switch {
		case p.ReviewerID == admin.ID:
			respond.Error(w, http.StatusForbidden, "You can't pay yourself. Ask another admin.")
			return
		case seen[p.ReviewerID]:
			respond.Error(w, http.StatusUnprocessableEntity, "Each reviewer can only be in a payment once.")
			return
		case len(p.Days) == 0 || len(p.Days) > 1000:
			respond.Error(w, http.StatusUnprocessableEntity, "Pick between 1 and 1,000 days for each reviewer.")
			return
		case p.Reviews < 1 || p.Reviews > 1_000_000:
			respond.Error(w, http.StatusUnprocessableEntity, "Each reviewer needs between 1 and 1,000,000 unpaid reviews.")
			return
		case p.Reviews*body.Rate > math.MaxInt32: // can't overflow: both are capped above
			respond.Error(w, http.StatusUnprocessableEntity, "That's too much for one payment.")
			return
		}
		seen[p.ReviewerID] = true
		for _, day := range p.Days {
			if _, err := time.Parse(time.DateOnly, day); err != nil {
				respond.Error(w, http.StatusUnprocessableEntity, "Days have to be YYYY-MM-DD.")
				return
			}
		}
		slices.Sort(body.Payments[i].Days)
		body.Payments[i].Days = slices.Compact(body.Payments[i].Days)
	}
	// One reviewer at a time, in the same order every time, so two payments
	// at once can't deadlock.
	slices.SortFunc(body.Payments, func(x, y reviewerToPay) int { return cmp.Compare(x.ReviewerID, y.ReviewerID) })

	tx, err := a.Pool.Begin(r.Context())
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())
	q := a.Queries.WithTx(tx)

	made := []reviewPaymentJSON{}
	for _, p := range body.Payments {
		reviewer, err := q.LockUser(r.Context(), p.ReviewerID)
		if errors.Is(err, pgx.ErrNoRows) {
			respond.Error(w, http.StatusNotFound, "One of those reviewers doesn't exist.")
			return
		}
		if err != nil {
			respond.ServerError(w, r, err)
			return
		}
		ids, err := q.LockUnpaidReviews(r.Context(), db.LockUnpaidReviewsParams{ReviewerID: &p.ReviewerID, Days: p.Days})
		if err != nil {
			respond.ServerError(w, r, err)
			return
		}
		if int64(len(ids)) != p.Reviews {
			respond.Error(w, http.StatusConflict, fmt.Sprintf("%s has %d unpaid reviews on those days now, not %d. Reload and check again.",
				reviewer.Name, len(ids), p.Reviews))
			return
		}
		reviews, rate := int32(p.Reviews), int32(body.Rate)
		amount := reviews * rate
		note := reviewPayNote(body.Currency, reviews, rate, p.Days)
		var ledgerID *int64
		if body.Currency == payInTickets {
			entry, err := q.AddStampEntry(r.Context(), db.AddStampEntryParams{UserID: reviewer.ID, Kind: db.StampEntryKindAdjustment,
				Amount: amount, CreatedBy: &admin.ID, Note: &note})
			if err != nil {
				respond.ServerError(w, r, err)
				return
			}
			ledgerID = &entry.ID
		}
		payment, err := q.CreateReviewPayment(r.Context(), db.CreateReviewPaymentParams{ReviewerID: reviewer.ID, PaidBy: admin.ID,
			Currency: body.Currency, Reviews: reviews, Rate: rate, Amount: amount, LedgerID: ledgerID, Note: note})
		if err != nil {
			respond.ServerError(w, r, err)
			return
		}
		if n, err := q.MarkReviewsPaid(r.Context(), db.MarkReviewsPaidParams{EventIds: ids, PaymentID: payment.ID}); err != nil || n != int64(len(ids)) {
			respond.ServerError(w, r, cmp.Or(err, fmt.Errorf("marked %d of %d reviews paid", n, len(ids))))
			return
		}
		made = append(made, reviewPaymentJSON{ID: payment.ID, ReviewerID: reviewer.ID, ReviewerName: reviewer.Name, PaidByName: admin.Name,
			Currency: payment.Currency, Reviews: payment.Reviews, Rate: payment.Rate, Amount: payment.Amount, Note: payment.Note,
			CreatedAt: payment.CreatedAt})
	}
	if err := tx.Commit(r.Context()); err != nil {
		respond.ServerError(w, r, err)
		return
	}
	respond.JSON(w, http.StatusCreated, made)
}

// reviewPayNote says what a payment was for, on the reviewer's ticket
// history and in the audit log: "Reviewer pay: 14 reviews at 5 tickets
// each, Nov 17 to Nov 19 (3 days)."
func reviewPayNote(currency string, reviews, rate int32, days []string) string {
	each := fmt.Sprintf("%d ticket%s", rate, map[bool]string{true: "", false: "s"}[rate == 1])
	if currency == payInUSD {
		each = fmt.Sprintf("$%d.%02d", rate/100, rate%100)
	}
	label := func(day string) string {
		t, _ := time.Parse(time.DateOnly, day)
		return t.Format("Jan 2, 2006")
	}
	when := label(days[0])
	if len(days) > 1 {
		when = fmt.Sprintf("%s to %s (%d days)", label(days[0]), label(days[len(days)-1]), len(days))
	}
	return strings.TrimSpace(fmt.Sprintf("Reviewer pay: %d review%s at %s each, %s.", reviews,
		map[bool]string{true: "", false: "s"}[reviews == 1], each, when))
}
