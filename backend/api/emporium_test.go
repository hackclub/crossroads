package api_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/scherepi/crossroads-backend/db"
)

type grantItem struct {
	ID                int64  `json:"id"`
	Kind              string `json:"kind"`
	Price             int    `json:"price"`
	Stock             *int   `json:"stock"`
	Emporium          bool   `json:"emporium"`
	GrantCents        *int   `json:"grant_cents"`
	GrantCentsPerHour *int   `json:"grant_cents_per_hour"`
}

type grantOrder struct {
	ID         int64  `json:"id"`
	Price      int    `json:"price"`
	Quantity   int    `json:"quantity"`
	GrantCents *int   `json:"grant_cents"`
	Error      string `json:"error"`
}

func (e *env) reload(t *testing.T, user *db.User) *db.User {
	t.Helper()
	fresh, err := e.queries.GetUser(context.Background(), user.ID)
	if err != nil {
		t.Fatal(err)
	}
	return &fresh
}

func TestSmugglersEmporium(t *testing.T) {
	e := newEnv(t)
	admin := e.newAdmin(t)
	reviewer := e.newReviewer(t)
	maker := e.verifiedUser(t, admin, "UMAKER", 500)

	// A grant defaults to $5 pieces at $6.50 an hour, in the Emporium. At 10
	// stamps an hour, a piece is 7.7 stamps, rounded up.
	var grant grantItem
	e.mustCall(t, admin, http.StatusCreated, "POST", "/api/admin/shop-items",
		map[string]any{"kind": "grant", "name": "Project grant", "price": 1, "stock": 100}, &grant)
	if !grant.Emporium || grant.GrantCents == nil || *grant.GrantCents != 500 || *grant.GrantCentsPerHour != 650 || grant.Price != 8 {
		t.Fatalf("grant: %+v", grant)
	}
	var crate grantItem
	e.mustCall(t, admin, http.StatusCreated, "POST", "/api/admin/shop-items",
		map[string]any{"kind": "physical", "name": "Contraband crate", "price": 50, "emporium": true}, &crate)
	for _, body := range []map[string]any{
		{"kind": "grant", "name": "Out in the open", "price": 1, "emporium": false},
		{"kind": "grant", "name": "Capped", "price": 1, "max_per_user": 3},
		{"kind": "ticket", "name": "Smuggled ticket", "price": 10, "emporium": true},
		{"kind": "physical", "name": "Fake grant", "price": 10, "grant_cents": 500, "grant_cents_per_hour": 650},
	} {
		if status := e.call(t, admin, "POST", "/api/admin/shop-items", body, nil); status != http.StatusUnprocessableEntity {
			t.Errorf("%v: got %d, want 422", body["name"], status)
		}
	}

	// Outside, it doesn't exist: not in the shop, not listed, not for sale.
	var shop []grantItem
	e.mustCall(t, maker, http.StatusOK, "GET", "/api/shop/items", nil, &shop)
	for _, item := range shop {
		if item.ID == grant.ID || item.ID == crate.ID {
			t.Errorf("the shop lists an Emporium item: %+v", item)
		}
	}
	if status := e.call(t, maker, "GET", "/api/emporium/items", nil, nil); status != http.StatusNotFound {
		t.Errorf("a participant listed the Emporium: %d", status)
	}
	if status := e.call(t, maker, "POST", "/api/shop/items/"+itoa(grant.ID)+"/orders", map[string]any{"quantity": 1}, nil); status != http.StatusNotFound {
		t.Errorf("a participant bought a grant: %d", status)
	}

	// Reviewers are in.
	var listed []grantItem
	e.mustCall(t, reviewer, http.StatusOK, "GET", "/api/emporium/items", nil, &listed)
	if len(listed) != 2 {
		t.Errorf("a reviewer sees %d Emporium items, want 2", len(listed))
	}

	// An admin lets the maker in, for their project.
	var person struct {
		InEmporium   bool    `json:"in_emporium"`
		EmporiumNote *string `json:"emporium_note"`
	}
	e.mustCall(t, admin, http.StatusOK, "PATCH", "/api/admin/users/"+itoa(maker.ID),
		map[string]any{"emporium_access": true, "emporium_note": "  for Tiny Compiler "}, &person)
	if !person.InEmporium || person.EmporiumNote == nil || *person.EmporiumNote != "for Tiny Compiler" {
		t.Fatalf("after letting them in: %+v", person)
	}
	maker = e.reload(t, maker)
	var me struct {
		Emporium bool `json:"emporium"`
	}
	e.mustCall(t, maker, http.StatusOK, "GET", "/api/me", nil, &me)
	if !me.Emporium {
		t.Error("/me doesn't say they're in")
	}
	e.mustCall(t, maker, http.StatusOK, "GET", "/api/emporium/items", nil, &listed)

	// $50 of grant: 10 pieces, 76.9 stamps rounded up.
	var o grantOrder
	if status := e.call(t, maker, "POST", "/api/shop/items/"+itoa(grant.ID)+"/orders", map[string]any{"quantity": 10}, &o); status != http.StatusCreated {
		t.Fatalf("buying a grant: %d %q", status, o.Error)
	}
	if o.Price != 77 || o.Quantity != 10 || o.GrantCents == nil || *o.GrantCents != 5000 {
		t.Errorf("grant order: %+v", o)
	}
	if got := e.stamps(t, maker); got != 500-77 {
		t.Errorf("balance %d, want %d", got, 500-77)
	}
	e.mustCall(t, maker, http.StatusOK, "GET", "/api/emporium/items", nil, &listed)
	for _, item := range listed {
		if item.ID == grant.ID && (item.Stock == nil || *item.Stock != 90) {
			t.Errorf("grant stock after 10 pieces: %+v", item)
		}
	}
	for _, c := range []struct {
		item grantItem
		body map[string]any
		want int
	}{
		{grant, map[string]any{"quantity": 0}, http.StatusUnprocessableEntity},
		{grant, map[string]any{"quantity": 201}, http.StatusUnprocessableEntity},
		{grant, map[string]any{"quantity": 95}, http.StatusConflict}, // only 90 left
		{crate, map[string]any{"quantity": 2}, http.StatusUnprocessableEntity},
	} {
		if status := e.call(t, maker, "POST", "/api/shop/items/"+itoa(c.item.ID)+"/orders", c.body, nil); status != c.want {
			t.Errorf("%v: got %d, want %d", c.body, status, c.want)
		}
	}

	// Cancelling gives back the stamps and every piece.
	e.mustCall(t, admin, http.StatusOK, "PATCH", "/api/admin/orders/"+itoa(o.ID), map[string]any{"status": "cancelled"}, nil)
	if got := e.stamps(t, maker); got != 500 {
		t.Errorf("balance after cancelling %d, want 500", got)
	}
	e.mustCall(t, maker, http.StatusOK, "GET", "/api/emporium/items", nil, &listed)
	for _, item := range listed {
		if item.ID == grant.ID && *item.Stock != 100 {
			t.Errorf("grant stock after cancelling: %d, want 100", *item.Stock)
		}
	}

	// Taking access away hides it again.
	e.mustCall(t, admin, http.StatusOK, "PATCH", "/api/admin/users/"+itoa(maker.ID), map[string]any{"emporium_access": false}, &person)
	if person.InEmporium {
		t.Error("still in after taking access away")
	}
	maker = e.reload(t, maker)
	if status := e.call(t, maker, "POST", "/api/shop/items/"+itoa(crate.ID)+"/orders", nil, nil); status != http.StatusNotFound {
		t.Errorf("bought from the Emporium after leaving it: %d", status)
	}
	if status := e.call(t, reviewer, "PATCH", "/api/admin/users/"+itoa(maker.ID), map[string]any{"emporium_access": true}, nil); status != http.StatusForbidden {
		t.Errorf("a reviewer let someone in: %d", status)
	}
}
