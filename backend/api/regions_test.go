package api_test

import (
	"context"
	"net/http"
	"slices"
	"testing"

	"github.com/scherepi/crossroads-backend/db"
)

// livesIn gives someone a Hack Club Auth home address in country.
func (e *env) livesIn(t *testing.T, user *db.User, country string) {
	t.Helper()
	address := `{"name": "H", "line_1": "1 Rd", "city": "C", "country": "` + country + `"}`
	if _, err := e.pool.Exec(context.Background(), "update users set address = $2 where id = $1", user.ID, address); err != nil {
		t.Fatal(err)
	}
	user.Address = []byte(address)
}

// Items can go only to some countries, or everywhere except some; someone
// whose home address is somewhere an item can't go doesn't see it and can't
// buy it.
func TestItemRegions(t *testing.T) {
	e := newEnv(t)
	admin := e.newAdmin(t)
	usOnly := e.newItem(t, admin, map[string]any{"kind": "physical", "name": "US only", "price": 10,
		"regions": map[string]any{"rule": "only", "countries": []string{"us"}}})
	notIndia := e.newItem(t, admin, map[string]any{"kind": "physical", "name": "Not India", "price": 10,
		"regions": map[string]any{"rule": "except", "countries": []string{"IN", " in "}}})
	anywhere := e.newItem(t, admin, map[string]any{"kind": "physical", "name": "Anywhere", "price": 10})

	var listed []struct {
		ID      int64 `json:"id"`
		Regions struct {
			Rule      string   `json:"rule"`
			Countries []string `json:"countries"`
		} `json:"regions"`
	}
	e.mustCall(t, admin, http.StatusOK, "GET", "/api/admin/shop-items", nil, &listed)
	for _, i := range listed {
		switch i.ID {
		case usOnly.ID:
			if i.Regions.Rule != "only" || !slices.Equal(i.Regions.Countries, []string{"US"}) {
				t.Errorf("US only: %+v", i.Regions)
			}
		case notIndia.ID:
			if i.Regions.Rule != "except" || !slices.Equal(i.Regions.Countries, []string{"IN"}) {
				t.Errorf("not India: %+v", i.Regions)
			}
		case anywhere.ID:
			if i.Regions.Rule != "everywhere" || len(i.Regions.Countries) != 0 {
				t.Errorf("anywhere: %+v", i.Regions)
			}
		}
	}

	for _, bad := range []map[string]any{
		{"rule": "only", "countries": []string{}},
		{"rule": "only", "countries": []string{"USA"}},
		{"rule": "everywhere", "countries": []string{"US"}},
		{"rule": "nowhere", "countries": []string{"US"}},
	} {
		if status := e.call(t, admin, "PATCH", "/api/admin/shop-items/"+itoa(anywhere.ID), map[string]any{"regions": bad}, nil); status != http.StatusUnprocessableEntity {
			t.Errorf("regions %v: %d, want 422", bad, status)
		}
	}
	if status := e.call(t, admin, "POST", "/api/admin/shop-items", map[string]any{"kind": "ticket", "name": "Pass", "price": 600,
		"regions": map[string]any{"rule": "only", "countries": []string{"US"}}}, nil); status != http.StatusUnprocessableEntity {
		t.Errorf("a rail pass for the US only: %d, want 422", status)
	}

	shop := func(u *db.User) []int64 {
		t.Helper()
		var items []shopItem
		e.mustCall(t, u, http.StatusOK, "GET", "/api/shop/items", nil, &items)
		var ids []int64
		for _, i := range items {
			ids = append(ids, i.ID)
		}
		return ids
	}
	in := e.verifiedUser(t, admin, "UIN", 100)
	e.livesIn(t, in, "IN")
	if ids := shop(in); slices.Contains(ids, usOnly.ID) || slices.Contains(ids, notIndia.ID) || !slices.Contains(ids, anywhere.ID) {
		t.Errorf("someone in India sees %v", ids)
	}
	us := e.verifiedUser(t, admin, "UUS", 100)
	e.livesIn(t, us, "us")
	if ids := shop(us); !slices.Contains(ids, usOnly.ID) || !slices.Contains(ids, notIndia.ID) {
		t.Errorf("someone in the US sees %v", ids)
	}
	// Without a home address there's nothing to judge by.
	nowhere := e.verifiedUser(t, admin, "UNONE", 100)
	if ids := shop(nowhere); !slices.Contains(ids, usOnly.ID) || !slices.Contains(ids, notIndia.ID) {
		t.Errorf("someone with no address sees %v", ids)
	}

	if status, o := e.buy(t, in, usOnly, address); status != http.StatusUnprocessableEntity {
		t.Errorf("buying a US-only item from India: %d %q", status, o.Error)
	}
	// From the US, but shipping it to India.
	toIndia := map[string]any{"shipping_address": map[string]any{"name": "H", "line_1": "1 Rd", "city": "Delhi", "country": "IN"}}
	if status, o := e.buy(t, us, notIndia, toIndia); status != http.StatusUnprocessableEntity {
		t.Errorf("shipping an item India's excluded from to India: %d %q", status, o.Error)
	}
	if status, o := e.buy(t, us, notIndia, address); status != http.StatusCreated {
		t.Errorf("buying it in the US: %d %q", status, o.Error)
	}
}

// The admin Stats tab counts participants by home country, and only counts.
func TestParticipantCountries(t *testing.T) {
	e := newEnv(t)
	admin := e.newAdmin(t)
	e.livesIn(t, e.newUser(t, "U1"), "BR")
	e.livesIn(t, e.newUser(t, "U2"), " br ")
	e.livesIn(t, e.newUser(t, "U3"), "IN")
	e.newUser(t, "U4") // no address
	var stats struct {
		Countries []struct {
			Country string `json:"country"`
			People  int64  `json:"people"`
			Shipped int64  `json:"shipped"`
		} `json:"countries"`
	}
	e.mustCall(t, admin, http.StatusOK, "GET", "/api/admin/activity", nil, &stats)
	got := map[string]int64{}
	for _, c := range stats.Countries {
		got[c.Country] = c.People
	}
	if got["BR"] != 2 || got["IN"] != 1 || len(stats.Countries) < 2 || stats.Countries[0].Country != "BR" {
		t.Errorf("countries: %+v", stats.Countries)
	}
}
