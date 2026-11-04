package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/scherepi/crossroads-backend/auth"
	"github.com/scherepi/crossroads-backend/db"
)

type shopItem struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Stock  *int   `json:"stock"`
	Bought int    `json:"bought"`
}

type order struct {
	ID              int64          `json:"id"`
	Status          string         `json:"status"`
	Price           int            `json:"price"`
	ShippingAddress map[string]any `json:"shipping_address"`
	Error           string         `json:"error"`
}

// verifiedUser makes someone Hack Club Auth says is verified and eligible,
// with balance stamps.
func (e *env) verifiedUser(t *testing.T, admin *db.User, slackID string, balance int) *db.User {
	t.Helper()
	user := e.newUser(t, slackID)
	e.verification.set(user, "verified_eligible")
	if balance != 0 {
		e.mustCall(t, admin, http.StatusCreated, "POST", "/api/admin/users/"+itoa(user.ID)+"/stamp-adjustments",
			map[string]any{"amount": balance, "note": "test stamps"}, nil)
	}
	return user
}

func (e *env) newItem(t *testing.T, admin *db.User, body map[string]any) shopItem {
	t.Helper()
	var item shopItem
	e.mustCall(t, admin, http.StatusCreated, "POST", "/api/admin/shop-items", body, &item)
	return item
}

func (e *env) buy(t *testing.T, user *db.User, item shopItem, body map[string]any) (int, order) {
	t.Helper()
	var o order
	status := e.call(t, user, "POST", "/api/shop/items/"+itoa(item.ID)+"/orders", body, &o)
	return status, o
}

var address = map[string]any{"shipping_address": map[string]any{
	"name": "Heidi Trashworth", "line_1": "15 Falls Rd", "city": "Shelburne", "state": "VT", "postal_code": "05482", "country": "US",
}}

func TestBuyingNeedsVerification(t *testing.T) {
	e := newEnv(t)
	admin := e.newAdmin(t)
	ticket := e.newItem(t, admin, map[string]any{"kind": "ticket", "name": "Ticket", "price": 600})
	user := e.newUser(t, "U1")
	e.mustCall(t, admin, http.StatusCreated, "POST", "/api/admin/users/"+itoa(user.ID)+"/stamp-adjustments",
		map[string]any{"amount": 600, "note": "test stamps"}, nil)

	for result, want := range map[string]string{
		"needs_submission":     "Verify your identity",
		"pending":              "still being reviewed",
		"verified_but_over_18": "not eligible",
	} {
		e.verification.set(user, result)
		status, o := e.buy(t, user, ticket, nil)
		if status != http.StatusForbidden || !strings.Contains(o.Error, want) {
			t.Errorf("%s: got %d %q", result, status, o.Error)
		}
	}

	// Getting verified after signing in is picked up at checkout.
	e.verification.set(user, "verified_eligible")
	if status, o := e.buy(t, user, ticket, nil); status != http.StatusCreated {
		t.Fatalf("verified: got %d %q", status, o.Error)
	}
	saved, _ := e.queries.GetUser(context.Background(), user.ID)
	if saved.VerificationStatus != "verified" || !saved.YswsEligible {
		t.Errorf("saved verification is %q eligible=%v", saved.VerificationStatus, saved.YswsEligible)
	}
}

func TestBuyingTheTicket(t *testing.T) {
	e := newEnv(t)
	admin := e.newAdmin(t)
	ticket := e.newItem(t, admin, map[string]any{"kind": "ticket", "name": "Ticket", "price": 600})
	user := e.verifiedUser(t, admin, "U1", 1000)

	status, o := e.buy(t, user, ticket, nil)
	if status != http.StatusCreated || o.Price != 600 || o.Status != "pending" {
		t.Fatalf("got %d %+v", status, o)
	}
	if got := e.stamps(t, user); got != 400 {
		t.Errorf("balance is %d, want 400", got)
	}
	if status, o := e.buy(t, user, ticket, nil); status != http.StatusConflict || !strings.Contains(o.Error, "ticket") {
		t.Errorf("buying a second ticket: got %d %q", status, o.Error)
	}

	var items []shopItem
	e.mustCall(t, user, http.StatusOK, "GET", "/api/shop/items", nil, &items)
	if len(items) != 1 || items[0].Bought != 1 {
		t.Errorf("items are %+v, want the ticket marked bought", items)
	}
	var orders []order
	e.mustCall(t, user, http.StatusOK, "GET", "/api/orders", nil, &orders)
	if len(orders) != 1 || orders[0].ID != o.ID {
		t.Errorf("orders are %+v", orders)
	}
}

func TestNotEnoughStamps(t *testing.T) {
	e := newEnv(t)
	admin := e.newAdmin(t)
	ticket := e.newItem(t, admin, map[string]any{"kind": "ticket", "name": "Ticket", "price": 600})
	user := e.verifiedUser(t, admin, "U1", 100)

	status, o := e.buy(t, user, ticket, nil)
	if status != http.StatusUnprocessableEntity || !strings.Contains(o.Error, "need 500 more") {
		t.Errorf("got %d %q", status, o.Error)
	}
	var orders []order
	e.mustCall(t, user, http.StatusOK, "GET", "/api/orders", nil, &orders)
	if len(orders) != 0 || e.stamps(t, user) != 100 {
		t.Errorf("a failed purchase left %d orders and %d stamps", len(orders), e.stamps(t, user))
	}
}

func TestSimultaneousPurchasesCantOverspend(t *testing.T) {
	e := newEnv(t)
	admin := e.newAdmin(t)
	user := e.verifiedUser(t, admin, "U1", 600)
	var items []shopItem
	for _, name := range []string{"Hoodie", "Keyboard", "Lamp", "Mug"} {
		items = append(items, e.newItem(t, admin, map[string]any{"kind": "physical", "name": name, "price": 400}))
	}

	var wg sync.WaitGroup
	for _, item := range items {
		wg.Go(func() {
			req := httptest.NewRequest("POST", "/api/shop/items/"+itoa(item.ID)+"/orders", strings.NewReader(`{"shipping_address":{"name":"H","line_1":"1 Rd","city":"C","country":"US"}}`))
			req = req.WithContext(auth.WithUser(req.Context(), user))
			e.router.ServeHTTP(httptest.NewRecorder(), req)
		})
	}
	wg.Wait()

	var orders int
	e.pool.QueryRow(context.Background(), "select count(*) from orders").Scan(&orders)
	if orders != 1 || e.stamps(t, user) != 200 {
		t.Errorf("%d orders left %d stamps, want 1 order and 200 left", orders, e.stamps(t, user))
	}
}

func TestStockRunsOut(t *testing.T) {
	e := newEnv(t)
	admin := e.newAdmin(t)
	mug := e.newItem(t, admin, map[string]any{"kind": "physical", "name": "Mug", "price": 100, "stock": 2})
	var buyers []*db.User
	for _, slackID := range []string{"U1", "U2", "U3", "U4", "U5"} {
		buyers = append(buyers, e.verifiedUser(t, admin, slackID, 100))
	}

	var wg sync.WaitGroup
	for _, buyer := range buyers {
		wg.Go(func() {
			req := httptest.NewRequest("POST", "/api/shop/items/"+itoa(mug.ID)+"/orders", strings.NewReader(`{"shipping_address":{"name":"H","line_1":"1 Rd","city":"C","country":"US"}}`))
			req = req.WithContext(auth.WithUser(req.Context(), buyer))
			e.router.ServeHTTP(httptest.NewRecorder(), req)
		})
	}
	wg.Wait()

	var orders, stock int
	e.pool.QueryRow(context.Background(), "select count(*) from orders").Scan(&orders)
	e.pool.QueryRow(context.Background(), "select stock from shop_items where id = $1", mug.ID).Scan(&stock)
	if orders != 2 || stock != 0 {
		t.Errorf("%d orders with %d left in stock, want 2 orders and 0 left", orders, stock)
	}
}

func TestPhysicalItemsNeedAnAddress(t *testing.T) {
	e := newEnv(t)
	admin := e.newAdmin(t)
	hoodie := e.newItem(t, admin, map[string]any{"kind": "physical", "name": "Hoodie", "price": 100})
	stipend := e.newItem(t, admin, map[string]any{"kind": "stipend", "name": "$50 travel stipend", "price": 100, "stipend_cents": 5000})
	user := e.verifiedUser(t, admin, "U1", 1000)

	if status, _ := e.buy(t, user, hoodie, nil); status != http.StatusUnprocessableEntity {
		t.Errorf("no address: got %d, want 422", status)
	}
	incomplete := map[string]any{"shipping_address": map[string]any{"name": "Heidi", "country": "US"}}
	if status, o := e.buy(t, user, hoodie, incomplete); status != http.StatusUnprocessableEntity || !strings.Contains(o.Error, "street address") {
		t.Errorf("incomplete address: got %d %q", status, o.Error)
	}
	if status, _ := e.buy(t, user, stipend, address); status != http.StatusUnprocessableEntity {
		t.Errorf("address on a stipend: got %d, want 422", status)
	}
	status, o := e.buy(t, user, hoodie, address)
	if status != http.StatusCreated || o.ShippingAddress["city"] != "Shelburne" {
		t.Errorf("with an address: got %d %+v", status, o)
	}
	if status, _ := e.buy(t, user, stipend, nil); status != http.StatusCreated {
		t.Errorf("stipend: got %d, want 201", status)
	}
}

func TestPhysicalItemsShipToTheHCAAddressByDefault(t *testing.T) {
	e := newEnv(t)
	admin := e.newAdmin(t)
	hoodie := e.newItem(t, admin, map[string]any{"kind": "physical", "name": "Hoodie", "price": 100})
	user := e.verifiedUser(t, admin, "U1", 100)
	if _, err := e.pool.Exec(context.Background(), `update users set address = '{"name": "Heidi T", "line_1": "15 Falls Rd", "city": "Shelburne", "country": "US"}' where id = $1`, user.ID); err != nil {
		t.Fatal(err)
	}
	status, o := e.buy(t, user, hoodie, nil)
	if status != http.StatusCreated || o.ShippingAddress["line_1"] != "15 Falls Rd" {
		t.Errorf("got %d shipping to %v, want their Hack Club Auth address", status, o.ShippingAddress)
	}
}

func TestCancellingAnOrderRefundsIt(t *testing.T) {
	e := newEnv(t)
	admin := e.newAdmin(t)
	mug := e.newItem(t, admin, map[string]any{"kind": "physical", "name": "Mug", "price": 100, "stock": 1})
	user := e.verifiedUser(t, admin, "U1", 100)
	_, o := e.buy(t, user, mug, address)

	var pending []order
	e.mustCall(t, admin, http.StatusOK, "GET", "/api/admin/orders?status=pending", nil, &pending)
	if len(pending) != 1 || pending[0].ShippingAddress["line_1"] != "15 Falls Rd" {
		t.Errorf("pending orders are %+v", pending)
	}

	e.mustCall(t, admin, http.StatusOK, "PATCH", "/api/admin/orders/"+itoa(o.ID), map[string]any{"status": "cancelled", "notes": "out of mugs"}, nil)
	var stock int
	e.pool.QueryRow(context.Background(), "select stock from shop_items where id = $1", mug.ID).Scan(&stock)
	if e.stamps(t, user) != 100 || stock != 1 {
		t.Errorf("after cancelling: %d stamps and %d in stock, want 100 and 1", e.stamps(t, user), stock)
	}
	e.mustCall(t, admin, http.StatusConflict, "PATCH", "/api/admin/orders/"+itoa(o.ID), map[string]any{"status": "cancelled"}, nil)
	if e.stamps(t, user) != 100 {
		t.Errorf("cancelling twice refunded twice: %d stamps", e.stamps(t, user))
	}

	// Fulfilling one doesn't touch stamps or stock.
	_, o = e.buy(t, user, mug, address)
	e.mustCall(t, admin, http.StatusOK, "PATCH", "/api/admin/orders/"+itoa(o.ID), map[string]any{"status": "fulfilled"}, nil)
	if e.stamps(t, user) != 0 {
		t.Errorf("after fulfilling: %d stamps, want 0", e.stamps(t, user))
	}
}

func TestAdminShopItemRules(t *testing.T) {
	e := newEnv(t)
	admin := e.newAdmin(t)
	user := e.verifiedUser(t, admin, "U1", 1000)

	for _, body := range []map[string]any{
		{"kind": "ticket", "name": "Ticket", "price": 600, "max_per_user": 2},
		{"kind": "stipend", "name": "Stipend", "price": 100},
		{"kind": "physical", "name": "Mug", "price": 100, "stipend_cents": 500},
		{"kind": "physical", "name": "Mug", "price": 0},
		{"kind": "spaceship", "name": "Rocket", "price": 100},
	} {
		e.mustCall(t, admin, http.StatusUnprocessableEntity, "POST", "/api/admin/shop-items", body, nil)
	}

	hidden := e.newItem(t, admin, map[string]any{"kind": "physical", "name": "Secret", "price": 100, "active": false})
	var items []shopItem
	e.mustCall(t, user, http.StatusOK, "GET", "/api/shop/items", nil, &items)
	if len(items) != 0 {
		t.Errorf("participants see %+v, want inactive items hidden", items)
	}
	if status, _ := e.buy(t, user, hidden, address); status != http.StatusNotFound {
		t.Errorf("buying an inactive item: got %d, want 404", status)
	}
	e.mustCall(t, user, http.StatusForbidden, "POST", "/api/admin/shop-items", map[string]any{"kind": "physical", "name": "Mine", "price": 1}, nil)
}

func TestStampAdjustments(t *testing.T) {
	e := newEnv(t)
	admin := e.newAdmin(t)
	user := e.newUser(t, "U1")
	path := "/api/admin/users/" + itoa(user.ID) + "/stamp-adjustments"

	e.mustCall(t, admin, http.StatusUnprocessableEntity, "POST", path, map[string]any{"amount": 50}, nil)
	e.mustCall(t, admin, http.StatusCreated, "POST", path, map[string]any{"amount": 50, "note": "bug bounty"}, nil)
	e.mustCall(t, admin, http.StatusCreated, "POST", path, map[string]any{"amount": -80, "note": "clawback: copied project"}, nil)
	if got := e.stamps(t, user); got != -30 {
		t.Errorf("balance is %d, want -30", got)
	}
	e.mustCall(t, e.newReviewer(t), http.StatusForbidden, "POST", path, map[string]any{"amount": 50, "note": "x"}, nil)
}
