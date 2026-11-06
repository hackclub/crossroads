package api_test

import (
	"net/http"
	"testing"
	"time"
)

func TestLaunchSale(t *testing.T) {
	e := newEnv(t)
	admin := e.newAdmin(t)
	e.api.SaleEnd = time.Now().Add(time.Hour).Truncate(time.Second)
	sticker := e.newItem(t, admin, map[string]any{"kind": "physical", "name": "Sticker", "price": 155})
	stipend := e.newItem(t, admin, map[string]any{"kind": "stipend", "name": "Stipend", "price": 1000, "stipend_cents": 10000})
	pin := e.newItem(t, admin, map[string]any{"kind": "physical", "name": "Pin", "price": 1})
	ticket := e.newItem(t, admin, map[string]any{"kind": "ticket", "name": "Ticket", "price": 600, "stock": 30, "batch_size": 6, "batch_step": 50})
	lock := e.newItem(t, admin, map[string]any{"kind": "price_lock", "name": "Price lock", "price": 50})
	user := e.verifiedUser(t, admin, "U1", 5000)

	type sale struct {
		FullPrice int       `json:"full_price"`
		Percent   int       `json:"percent"`
		EndsAt    time.Time `json:"ends_at"`
	}
	type item struct {
		ID    int64 `json:"id"`
		Price int   `json:"price"`
		Sale  *sale `json:"sale"`
	}
	list := func() map[int64]item {
		var items []item
		e.mustCall(t, user, http.StatusOK, "GET", "/api/shop/items", nil, &items)
		byID := map[int64]item{}
		for _, i := range items {
			byID[i.ID] = i
		}
		return byID
	}

	// 10% off, rounded down, but never free; not the ticket or the lock.
	items := list()
	for _, c := range []struct {
		id         int64
		price      int
		fullPrice  int
		discounted bool
	}{{sticker.ID, 139, 155, true}, {stipend.ID, 900, 1000, true}, {pin.ID, 1, 1, true}, {ticket.ID, 600, 0, false}, {lock.ID, 50, 0, false}} {
		got := items[c.id]
		if got.Price != c.price || (got.Sale != nil) != c.discounted {
			t.Errorf("item %d: got price %d, sale %+v", c.id, got.Price, got.Sale)
			continue
		}
		if c.discounted && (got.Sale.FullPrice != c.fullPrice || got.Sale.Percent != 10 || !got.Sale.EndsAt.Equal(e.api.SaleEnd)) {
			t.Errorf("item %d: got sale %+v", c.id, got.Sale)
		}
	}

	// Buying charges the sale price; the admin still sees the full one.
	if status, o := e.buy(t, user, stipend, nil); status != http.StatusCreated || o.Price != 900 {
		t.Fatalf("buying on sale: got %d, price %d", status, o.Price)
	}
	if got := e.stamps(t, user); got != 4100 {
		t.Errorf("after buying on sale: %d stamps, want 4100", got)
	}
	var adminItems []item
	e.mustCall(t, admin, http.StatusOK, "GET", "/api/admin/shop-items", nil, &adminItems)
	for _, i := range adminItems {
		if i.ID == stipend.ID && (i.Price != 1000 || i.Sale != nil) {
			t.Errorf("admin sees %+v", i)
		}
	}

	// Once it's over, everything's full price again.
	e.api.SaleEnd = time.Now().Add(-time.Second)
	if got := list()[sticker.ID]; got.Price != 155 || got.Sale != nil {
		t.Errorf("after the sale: got %+v", got)
	}
	if status, o := e.buy(t, user, stipend, nil); status != http.StatusCreated || o.Price != 1000 {
		t.Errorf("buying after the sale: got %d, price %d", status, o.Price)
	}
}
