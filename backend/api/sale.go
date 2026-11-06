package api

import (
	"time"

	"github.com/scherepi/crossroads-backend/db"
)

// Until API.SaleEnd, everything in the shop is salePercent off, except
// tickets and price locks: the ticket's price is what qualifying costs, so
// it stays put, and so does the lock on it.
const salePercent = 10

type saleJSON struct {
	FullPrice int32     `json:"full_price"` // what it costs once the sale's over
	Percent   int32     `json:"percent"`
	EndsAt    time.Time `json:"ends_at"`
}

// onSale says whether item is discounted at now.
func (a *API) onSale(item db.ShopItem, now time.Time) bool {
	return now.Before(a.SaleEnd) && item.Kind != db.ShopItemKindTicket && item.Kind != db.ShopItemKindPriceLock
}

// salePrice is price with the sale's discount, rounded down so it's never
// less than salePercent off, but never free.
func salePrice(price int32) int32 {
	return max(price*(100-salePercent)/100, 1)
}
