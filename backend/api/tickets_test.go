package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/scherepi/crossroads-backend/auth"
	"github.com/scherepi/crossroads-backend/db"
)

type ticketItem struct {
	ID      int64 `json:"id"`
	Kind    string
	Price   int `json:"price"`
	Stock   int `json:"stock"`
	Batches *struct {
		Size        int   `json:"size"`
		Prices      []int `json:"prices"`
		Current     int   `json:"current"`
		LeftInBatch int   `json:"left_in_batch"`
	} `json:"batches"`
	LockedPrice *int `json:"locked_price"`
}

type lockOrder struct {
	ID          int64  `json:"id"`
	Status      string `json:"status"`
	Price       int    `json:"price"`
	LockedPrice *int   `json:"locked_price"`
	Error       string `json:"error"`
}

// ticketFor is the ticket as user sees it in the shop.
func (e *env) ticketFor(t *testing.T, user *db.User) ticketItem {
	t.Helper()
	var items []ticketItem
	e.mustCall(t, user, http.StatusOK, "GET", "/api/shop/items", nil, &items)
	for _, item := range items {
		if item.Kind == "ticket" {
			return item
		}
	}
	t.Fatal("no ticket in the shop")
	return ticketItem{}
}

func (e *env) buyAs(t *testing.T, user *db.User, itemID int64) (int, lockOrder) {
	t.Helper()
	var o lockOrder
	status := e.call(t, user, "POST", "/api/shop/items/"+itoa(itemID)+"/orders", nil, &o)
	return status, o
}

func TestTicketBatchesAndPriceLocks(t *testing.T) {
	e := newEnv(t)
	admin := e.newAdmin(t)
	// Three batches of two: 60, 65 and 70.
	ticket := e.newItem(t, admin, map[string]any{"kind": "ticket", "name": "Ticket", "price": 60, "stock": 6, "batch_size": 2, "batch_step": 5})
	lock := e.newItem(t, admin, map[string]any{"kind": "price_lock", "name": "Price lock", "price": 5})
	early := e.verifiedUser(t, admin, "UEARLY", 100)
	late := e.verifiedUser(t, admin, "ULATE", 100)
	var buyers []*db.User
	for _, id := range []string{"U1", "U2", "U3", "U4", "U5"} {
		buyers = append(buyers, e.verifiedUser(t, admin, id, 100))
	}
	buyTicket := func(user *db.User, wantPrice int) {
		t.Helper()
		if status, o := e.buyAs(t, user, ticket.ID); status != http.StatusCreated || o.Price != wantPrice {
			t.Fatalf("buying a ticket: got %d %+v, want %d at %d", status, o, http.StatusCreated, wantPrice)
		}
	}

	seen := e.ticketFor(t, early)
	if seen.Price != 60 || seen.Batches == nil || !slices.Equal(seen.Batches.Prices, []int{60, 65, 70}) ||
		seen.Batches.Current != 0 || seen.Batches.LeftInBatch != 2 || seen.LockedPrice != nil {
		t.Fatalf("before any sales: got %+v %+v", seen, seen.Batches)
	}

	// A lock fixes the price of the batch on sale, and is done right away.
	status, o := e.buyAs(t, early, lock.ID)
	if status != http.StatusCreated || o.Status != "fulfilled" || o.Price != 5 || o.LockedPrice == nil || *o.LockedPrice != 60 {
		t.Fatalf("locking in batch 1: got %d %+v", status, o)
	}
	if status, o := e.buyAs(t, early, lock.ID); status != http.StatusConflict || !strings.Contains(o.Error, "already locked") {
		t.Errorf("a second lock: got %d %q", status, o.Error)
	}

	// Two sales use up the first batch.
	buyTicket(buyers[0], 60)
	buyTicket(buyers[1], 60)
	if seen := e.ticketFor(t, buyers[2]); seen.Price != 65 || seen.Batches.Current != 1 || seen.Batches.LeftInBatch != 2 {
		t.Errorf("batch 2 for someone without a lock: got %d, %+v", seen.Price, seen.Batches)
	}
	if seen := e.ticketFor(t, early); seen.Price != 60 || seen.LockedPrice == nil || *seen.LockedPrice != 60 {
		t.Errorf("batch 2 for the early lock: got %+v", seen)
	}

	// Locking now gets 65; the early lock still buys at 60.
	if status, o := e.buyAs(t, late, lock.ID); status != http.StatusCreated || *o.LockedPrice != 65 {
		t.Fatalf("locking in batch 2: got %d %+v", status, o)
	}
	buyTicket(early, 60)
	if got := e.stamps(t, early); got != 100-5-60 {
		t.Errorf("the early lock paid %d in all, want 65", 100-got)
	}
	if status, o := e.buyAs(t, buyers[0], lock.ID); status != http.StatusConflict || !strings.Contains(o.Error, "already got your ticket") {
		t.Errorf("locking with a ticket: got %d %q", status, o.Error)
	}

	// Into the last batch, where a lock can't help.
	buyTicket(buyers[2], 65)
	if status, o := e.buyAs(t, buyers[3], lock.ID); status != http.StatusConflict || !strings.Contains(o.Error, "last batch") {
		t.Errorf("locking in the last batch: got %d %q", status, o.Error)
	}

	// Selling out refunds the lock nobody used.
	buyTicket(buyers[3], 70)
	buyTicket(buyers[4], 70)
	if got := e.stamps(t, late); got != 100 {
		t.Errorf("after the sell-out, the late lock's holder has %d stamps, want all 100 back", got)
	}
	var orders []lockOrder
	e.mustCall(t, late, http.StatusOK, "GET", "/api/orders", nil, &orders)
	if len(orders) != 1 || orders[0].Status != "cancelled" {
		t.Errorf("the late lock's order: got %+v", orders)
	}
	if status, _ := e.buyAs(t, late, ticket.ID); status != http.StatusConflict {
		t.Errorf("buying after the sell-out: got %d", status)
	}
	if seen := e.ticketFor(t, late); seen.Price != 70 || seen.Batches.Current != 2 || seen.Batches.LeftInBatch != 0 || seen.LockedPrice != nil {
		t.Errorf("sold out: got %+v %+v", seen, seen.Batches)
	}
	// Refunding again changes nothing.
	if err := e.api.RefundPriceLocks(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := e.stamps(t, late); got != 100 {
		t.Errorf("a second refund pass left %d stamps, want 100", got)
	}
}

func TestPriceLockRefundWhenAdminSellsOut(t *testing.T) {
	e := newEnv(t)
	admin := e.newAdmin(t)
	ticket := e.newItem(t, admin, map[string]any{"kind": "ticket", "name": "Ticket", "price": 60, "stock": 4, "batch_size": 2, "batch_step": 5})
	lock := e.newItem(t, admin, map[string]any{"kind": "price_lock", "name": "Price lock", "price": 5})
	holder := e.verifiedUser(t, admin, "U1", 100)
	if status, _ := e.buyAs(t, holder, lock.ID); status != http.StatusCreated {
		t.Fatalf("locking: got %d", status)
	}
	e.mustCall(t, admin, http.StatusOK, "PATCH", "/api/admin/shop-items/"+itoa(ticket.ID), map[string]any{"stock": 0}, nil)
	if err := e.api.RefundPriceLocks(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := e.stamps(t, holder); got != 100 {
		t.Errorf("after an admin sold the tickets out, the holder has %d stamps, want 100", got)
	}
}

func TestTicketBatchFields(t *testing.T) {
	e := newEnv(t)
	admin := e.newAdmin(t)
	for name, body := range map[string]map[string]any{
		"batches on a physical item":  {"kind": "physical", "name": "Pin", "price": 5, "stock": 4, "batch_size": 2, "batch_step": 5},
		"batches without a stock":     {"kind": "ticket", "name": "Ticket", "price": 60, "batch_size": 2, "batch_step": 5},
		"a batch size without a step": {"kind": "ticket", "name": "Ticket", "price": 60, "stock": 4, "batch_size": 2},
		"a zero step":                 {"kind": "ticket", "name": "Ticket", "price": 60, "stock": 4, "batch_size": 2, "batch_step": 0},
		"two locks each":              {"kind": "price_lock", "name": "Lock", "price": 5, "max_per_user": 2},
	} {
		if status := e.call(t, admin, "POST", "/api/admin/shop-items", body, nil); status != http.StatusUnprocessableEntity {
			t.Errorf("%s: got %d, want 422", name, status)
		}
	}

	// A price lock needs a ticket whose price changes.
	e.newItem(t, admin, map[string]any{"kind": "ticket", "name": "Ticket", "price": 60})
	lock := e.newItem(t, admin, map[string]any{"kind": "price_lock", "name": "Lock", "price": 5})
	user := e.verifiedUser(t, admin, "U1", 100)
	if status, o := e.buyAs(t, user, lock.ID); status != http.StatusConflict || !strings.Contains(o.Error, "nothing to lock") {
		t.Errorf("locking a fixed price: got %d %q", status, o.Error)
	}

	var me struct {
		ID            int64 `json:"id"`
		StampsPerHour int   `json:"stamps_per_hour"`
	}
	e.mustCall(t, user, http.StatusOK, "GET", "/api/me", nil, &me)
	if me.ID != user.ID || me.StampsPerHour != 10 {
		t.Errorf("/api/me: got %+v", me)
	}
}

// Everyone buying at once still gets each batch's price exactly batch_size
// times, and nobody gets a ticket past the last.
func TestTicketBatchesAtOnce(t *testing.T) {
	e := newEnv(t)
	admin := e.newAdmin(t)
	ticket := e.newItem(t, admin, map[string]any{"kind": "ticket", "name": "Ticket", "price": 60, "stock": 6, "batch_size": 2, "batch_step": 5})
	var buyers []*db.User
	for i := range 8 {
		buyers = append(buyers, e.verifiedUser(t, admin, "U"+itoa(int64(i)), 100))
	}
	var wg sync.WaitGroup
	for _, buyer := range buyers {
		wg.Go(func() {
			req := httptest.NewRequest("POST", "/api/shop/items/"+itoa(ticket.ID)+"/orders", nil)
			req = req.WithContext(auth.WithUser(req.Context(), buyer))
			e.router.ServeHTTP(httptest.NewRecorder(), req)
		})
	}
	wg.Wait()

	var prices []int
	rows, err := e.pool.Query(context.Background(), "select price from orders order by price")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var price int
		if err := rows.Scan(&price); err != nil {
			t.Fatal(err)
		}
		prices = append(prices, price)
	}
	if want := []int{60, 60, 65, 65, 70, 70}; !slices.Equal(prices, want) {
		t.Errorf("got prices %v, want %v", prices, want)
	}
}
