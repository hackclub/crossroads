package api

// The shop sells the event ticket, price locks for it, physical items and
// travel stipends for stamps. Buying anything needs an identity verified by
// Hack Club Auth. tickets.go has ticket batches and price locks.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/scherepi/crossroads-backend/auth"
	"github.com/scherepi/crossroads-backend/db"
	"github.com/scherepi/crossroads-backend/hca"
	"github.com/scherepi/crossroads-backend/respond"
)

type shopItemJSON struct {
	ID           int64           `json:"id"`
	Kind         db.ShopItemKind `json:"kind"`
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	ImageURL     *string         `json:"image_url"`
	Price        int32           `json:"price"`
	StipendCents *int32          `json:"stipend_cents"` // stipends only
	Stock        *int32          `json:"stock"`         // how many are left; null is unlimited
	MaxPerUser   *int32          `json:"max_per_user"`  // null is unlimited
	Active       bool            `json:"active"`
	Position     int32           `json:"position"`   // lower shows first in the shop
	BatchSize    *int32          `json:"batch_size"` // tickets sold in batches only
	BatchStep    *int32          `json:"batch_step"` // how much more each batch costs than the one before
	Batches      *batchesJSON    `json:"batches"`    // tickets sold in batches only
	// How many the signed-in person has bought, not counting cancelled
	// orders. Not filled in for admins.
	Bought int64 `json:"bought"`
	// Tickets: the price the signed-in person locked in, if they bought a
	// price lock. Not filled in for admins.
	LockedPrice *int32 `json:"locked_price"`
	// While it's on sale: price is the sale price, and this has the full
	// one. Not filled in for admins, who see and set the full price.
	Sale *saleJSON `json:"sale"`
}

func toShopItemJSON(item db.ShopItem, bought int64) shopItemJSON {
	return shopItemJSON{
		ID:           item.ID,
		Kind:         item.Kind,
		Name:         item.Name,
		Description:  item.Description,
		ImageURL:     item.ImageUrl,
		Price:        item.Price,
		StipendCents: item.StipendCents,
		Stock:        item.Stock,
		MaxPerUser:   item.MaxPerUser,
		Active:       item.Active,
		Position:     item.Position,
		BatchSize:    item.BatchSize,
		BatchStep:    item.BatchStep,
		Bought:       bought,
	}
}

// soldByItem is how many of each shop item everyone has bought, not counting
// cancelled orders.
func (a *API) soldByItem(ctx context.Context) (map[int64]int64, error) {
	rows, err := a.Queries.CountSoldByItem(ctx)
	if err != nil {
		return nil, err
	}
	sold := map[int64]int64{}
	for _, row := range rows {
		sold[row.ShopItemID] = row.Count
	}
	return sold, nil
}

// cleanAddress trims every field and returns a problem to show them if the
// address is missing something.
func cleanAddress(a *db.Address) string {
	for _, field := range []*string{&a.Name, &a.Line1, &a.Line2, &a.City, &a.State, &a.PostalCode, &a.Country} {
		*field = strings.TrimSpace(*field)
		if len(*field) > 200 {
			return "Keep each part of the address under 200 characters."
		}
	}
	var missing []string
	for _, field := range []struct{ label, value string }{
		{"a name", a.Name}, {"a street address", a.Line1}, {"a city", a.City}, {"a country", a.Country},
	} {
		if field.value == "" {
			missing = append(missing, field.label)
		}
	}
	if len(missing) > 0 {
		return "The shipping address needs " + joinList(missing) + "."
	}
	return ""
}

type orderJSON struct {
	ID              int64           `json:"id"`
	Item            shopItemJSON    `json:"item"`
	Price           int32           `json:"price"` // what was paid
	Status          db.OrderStatus  `json:"status"`
	ShippingAddress json.RawMessage `json:"shipping_address"`
	LockedPrice     *int32          `json:"locked_price"` // price locks: the ticket price locked in
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

func toOrderJSON(order db.Order, item db.ShopItem) orderJSON {
	address := json.RawMessage(order.ShippingAddress)
	if address == nil {
		address = json.RawMessage("null")
	}
	return orderJSON{
		ID:              order.ID,
		Item:            toShopItemJSON(item, 0),
		Price:           order.Price,
		Status:          order.Status,
		ShippingAddress: address,
		LockedPrice:     order.LockedPrice,
		CreatedAt:       order.CreatedAt,
		UpdatedAt:       order.UpdatedAt,
	}
}

// listShopItems lists what's in the shop. Prices are what the signed-in
// person would pay now: for a ticket, the batch on sale, or the price they
// locked in if that's lower; for anything on sale, the sale price.
func (a *API) listShopItems(w http.ResponseWriter, r *http.Request) {
	user := auth.User(r.Context())
	items, err := a.Queries.ListShopItems(r.Context())
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	counts, err := a.Queries.CountOrdersByItem(r.Context(), user.ID)
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	bought := map[int64]int64{}
	for _, c := range counts {
		bought[c.ShopItemID] = c.Count
	}
	sold, err := a.soldByItem(r.Context())
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	var locked *int32
	if lock, err := a.Queries.GetPriceLock(r.Context(), user.ID); err == nil {
		locked = lock.LockedPrice
	} else if !errors.Is(err, pgx.ErrNoRows) {
		respond.ServerError(w, r, err)
		return
	}
	now := time.Now()
	out := []shopItemJSON{}
	for _, item := range items {
		j := toShopItemJSON(item, bought[item.ID])
		if a.onSale(item, now) {
			j.Sale = &saleJSON{FullPrice: item.Price, Percent: salePercent, EndsAt: a.SaleEnd}
			j.Price = salePrice(item.Price)
		}
		if item.Kind == db.ShopItemKindTicket {
			if j.Batches = toBatchesJSON(item, sold[item.ID]); j.Batches != nil {
				j.Price = j.Batches.Prices[j.Batches.Current]
			}
			j.LockedPrice = locked
			if locked != nil && *locked < j.Price {
				j.Price = *locked
			}
		}
		out = append(out, j)
	}
	respond.JSON(w, http.StatusOK, out)
}

func (a *API) listOrders(w http.ResponseWriter, r *http.Request) {
	user := auth.User(r.Context())
	rows, err := a.Queries.ListOrders(r.Context(), user.ID)
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	out := []orderJSON{}
	for _, row := range rows {
		out = append(out, toOrderJSON(row.Order, row.ShopItem))
	}
	respond.JSON(w, http.StatusOK, out)
}

// buy orders a shop item. Physical items ship to {"shipping_address": {...}},
// or to the address they gave Hack Club Auth if that's left out. Tickets
// cost the batch on sale, or the buyer's locked price if that's lower, and
// anything on sale costs the sale price. A price lock is done as soon as
// it's bought, so its order starts fulfilled.
func (a *API) buy(w http.ResponseWriter, r *http.Request) {
	user := auth.User(r.Context())
	id, ok := idParam(w, r, "itemID")
	if !ok {
		return
	}
	var body struct {
		ShippingAddress *db.Address `json:"shipping_address"`
	}
	if !decode(w, r, &body) {
		return
	}
	if status, problem := a.checkVerified(r, user); problem != "" {
		respond.Error(w, status, problem)
		return
	}

	tx, err := a.Pool.Begin(r.Context())
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())
	q := a.Queries.WithTx(tx)

	// Lock the buyer so their purchases go one at a time, then the item so
	// its stock can't change underneath us.
	locked, err := q.LockUser(r.Context(), user.ID)
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	item, err := q.LockShopItem(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !item.Active) {
		respond.Error(w, http.StatusNotFound, "That item isn't in the shop.")
		return
	}
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}

	if item.Kind == db.ShopItemKindPhysical && body.ShippingAddress == nil && locked.Address != nil {
		body.ShippingAddress = &db.Address{}
		if err := json.Unmarshal(locked.Address, body.ShippingAddress); err != nil {
			respond.ServerError(w, r, err)
			return
		}
	}
	var address []byte
	switch {
	case item.Kind == db.ShopItemKindPhysical && body.ShippingAddress == nil:
		respond.Error(w, http.StatusUnprocessableEntity, "Add a shipping address so we can send it to you.")
		return
	case item.Kind == db.ShopItemKindPhysical:
		if problem := cleanAddress(body.ShippingAddress); problem != "" {
			respond.Error(w, http.StatusUnprocessableEntity, problem)
			return
		}
		address, _ = json.Marshal(body.ShippingAddress)
	case body.ShippingAddress != nil:
		respond.Error(w, http.StatusUnprocessableEntity, "Only physical items take a shipping address.")
		return
	}

	if item.MaxPerUser != nil {
		bought, err := q.CountOrdersOfItem(r.Context(), db.CountOrdersOfItemParams{UserID: user.ID, ShopItemID: item.ID})
		if err != nil {
			respond.ServerError(w, r, err)
			return
		}
		if bought >= int64(*item.MaxPerUser) {
			switch item.Kind {
			case db.ShopItemKindTicket:
				respond.Error(w, http.StatusConflict, "You've already got your ticket!")
			case db.ShopItemKindPriceLock:
				respond.Error(w, http.StatusConflict, "You've already locked your ticket price.")
			default:
				respond.Error(w, http.StatusConflict, fmt.Sprintf("You can only get %d of these.", *item.MaxPerUser))
			}
			return
		}
	}
	if item.Stock != nil && *item.Stock <= 0 {
		respond.Error(w, http.StatusConflict, "That's sold out.")
		return
	}

	price, status, lockedPrice := item.Price, db.OrderStatusPending, (*int32)(nil)
	if a.onSale(item, time.Now()) {
		price = salePrice(item.Price)
	}
	switch item.Kind {
	case db.ShopItemKindTicket:
		sold, err := q.CountSold(r.Context(), item.ID)
		if err != nil {
			respond.ServerError(w, r, err)
			return
		}
		price = ticketPrice(item, sold)
		if lock, err := q.GetPriceLock(r.Context(), user.ID); err == nil && *lock.LockedPrice < price {
			price = *lock.LockedPrice
		} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			respond.ServerError(w, r, err)
			return
		}
	case db.ShopItemKindPriceLock:
		lockIn, problem, err := lockProblem(r.Context(), q, user.ID)
		if err != nil {
			respond.ServerError(w, r, err)
			return
		}
		if problem != "" {
			respond.Error(w, http.StatusConflict, problem)
			return
		}
		status, lockedPrice = db.OrderStatusFulfilled, &lockIn
	}
	if locked.Stamps < price {
		respond.Error(w, http.StatusUnprocessableEntity, fmt.Sprintf(
			"That costs %d stamps and you have %d, so you need %d more.", price, locked.Stamps, price-locked.Stamps))
		return
	}

	order, err := q.CreateOrder(r.Context(), db.CreateOrderParams{
		UserID:          user.ID,
		ShopItemID:      item.ID,
		Price:           price,
		ShippingAddress: address,
		Status:          status,
		LockedPrice:     lockedPrice,
	})
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	// The ledger refuses to go below zero, so this is the real guard; the
	// check above is just for a friendlier message.
	if _, err := q.AddStampEntry(r.Context(), db.AddStampEntryParams{
		UserID:  user.ID,
		Kind:    db.StampEntryKindPurchase,
		Amount:  -price,
		OrderID: &order.ID,
	}); db.IsNotEnoughStamps(err) {
		respond.Error(w, http.StatusUnprocessableEntity, "You don't have enough stamps for that.")
		return
	} else if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	if err := q.TakeShopItemStock(r.Context(), item.ID); err != nil {
		respond.ServerError(w, r, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		respond.ServerError(w, r, err)
		return
	}
	// That was the last ticket, so price locks nobody used get their stamps
	// back. If this fails, RunPriceLockRefunds tries again.
	if item.Kind == db.ShopItemKindTicket && item.Stock != nil && *item.Stock == 1 {
		if err := a.RefundPriceLocks(context.WithoutCancel(r.Context())); err != nil {
			log.Printf("refunding price locks: %v", err)
		}
	}
	respond.JSON(w, http.StatusCreated, toOrderJSON(order, item))
}

// checkVerified returns a problem, and the status to respond with, unless
// user's identity is verified and they're eligible for YSWS programs. What we
// stored at sign-in can be out of date, so if it doesn't say they're
// verified, it asks Hack Club Auth again and saves the answer.
func (a *API) checkVerified(r *http.Request, user *db.User) (int, string) {
	if user.VerificationStatus == "verified" && user.YswsEligible {
		return 0, ""
	}
	result, err := a.HCA.CheckVerification(r.Context(), user.HcaID)
	if err != nil {
		log.Printf("checking verification for user %d: %v", user.ID, err)
		return http.StatusBadGateway, "Couldn't check your verification with Hack Club Auth. Please try again in a bit."
	}
	status, eligible := "", false
	switch result {
	case hca.VerifiedEligible:
		status, eligible = "verified", true
	case hca.VerifiedButOver18:
		status = "verified"
	case hca.VerificationPending:
		status = "pending"
	case hca.NeedsSubmission:
		status = "needs_submission"
	case hca.Rejected:
		status = "ineligible"
	}
	if status != "" {
		if _, err := a.Queries.SetUserVerification(r.Context(), db.SetUserVerificationParams{
			ID: user.ID, VerificationStatus: status, YswsEligible: eligible,
		}); err != nil {
			log.Printf("saving verification for user %d: %v", user.ID, err)
		}
	}

	switch result {
	case hca.VerifiedEligible:
		return 0, ""
	case hca.VerificationPending:
		return http.StatusForbidden, "Your identity verification is still being reviewed. You can buy things once it's approved."
	case hca.VerifiedButOver18, hca.Rejected:
		return http.StatusForbidden, "Hack Club Auth says you're not eligible for YSWS programs, so you can't buy from the shop."
	default:
		return http.StatusForbidden, "Verify your identity at auth.hackclub.com before buying anything."
	}
}

// shopItemFields is the body of POST and PATCH /admin/shop-items. Fields left
// out aren't changed.
type shopItemFields struct {
	Kind         *db.ShopItemKind `json:"kind"`
	Name         *string          `json:"name"`
	Description  *string          `json:"description"`
	ImageURL     optional[string] `json:"image_url"`
	Price        *int32           `json:"price"`
	StipendCents optional[int32]  `json:"stipend_cents"`
	Stock        optional[int32]  `json:"stock"`
	MaxPerUser   optional[int32]  `json:"max_per_user"`
	Active       *bool            `json:"active"`
	Position     *int32           `json:"position"`
	BatchSize    optional[int32]  `json:"batch_size"`
	BatchStep    optional[int32]  `json:"batch_step"`
}

func (f shopItemFields) apply(item *db.ShopItem) string {
	if f.Kind != nil {
		item.Kind = *f.Kind
	}
	if f.Name != nil {
		item.Name = strings.TrimSpace(*f.Name)
	}
	if f.Description != nil {
		item.Description = strings.TrimSpace(*f.Description)
	}
	if f.ImageURL.Set {
		item.ImageUrl = f.ImageURL.Value
	}
	if f.Price != nil {
		item.Price = *f.Price
	}
	if f.StipendCents.Set {
		item.StipendCents = f.StipendCents.Value
	}
	if f.Stock.Set {
		item.Stock = f.Stock.Value
	}
	if f.MaxPerUser.Set {
		item.MaxPerUser = f.MaxPerUser.Value
	}
	if f.Active != nil {
		item.Active = *f.Active
	}
	if f.Position != nil {
		item.Position = *f.Position
	}
	if f.BatchSize.Set {
		item.BatchSize = f.BatchSize.Value
	}
	if f.BatchStep.Set {
		item.BatchStep = f.BatchStep.Value
	}
	switch {
	case !item.Kind.Valid():
		return fmt.Sprintf("kind has to be one of %v.", db.AllShopItemKindValues())
	case item.Name == "" || len(item.Name) > 100:
		return "Give it a name under 100 characters."
	case item.ImageUrl != nil && !isWebURL(*item.ImageUrl):
		return "image_url has to start with http:// or https://."
	case item.Price <= 0:
		return "price has to be above 0."
	case (item.Kind == db.ShopItemKindStipend) != (item.StipendCents != nil):
		return "Stipends need stipend_cents, and nothing else can have it."
	case item.StipendCents != nil && *item.StipendCents <= 0:
		return "stipend_cents has to be above 0."
	case item.Stock != nil && *item.Stock < 0:
		return "stock can't be negative."
	case item.MaxPerUser != nil && *item.MaxPerUser <= 0:
		return "max_per_user has to be above 0."
	case item.Kind == db.ShopItemKindTicket && (item.MaxPerUser == nil || *item.MaxPerUser != 1):
		return "Tickets are one per person, so max_per_user has to be 1."
	case item.Kind == db.ShopItemKindPriceLock && (item.MaxPerUser == nil || *item.MaxPerUser != 1):
		return "Price locks are one per person, so max_per_user has to be 1."
	case (item.BatchSize == nil) != (item.BatchStep == nil):
		return "Set batch_size and batch_step together, or neither."
	case item.BatchSize != nil && item.Kind != db.ShopItemKindTicket:
		return "Only tickets sell in batches."
	case item.BatchSize != nil && (*item.BatchSize <= 0 || *item.BatchStep <= 0):
		return "batch_size and batch_step have to be above 0."
	case item.BatchSize != nil && item.Stock == nil:
		return "Tickets sold in batches need a stock: how many there are across every batch."
	}
	return ""
}

func (a *API) adminListShopItems(w http.ResponseWriter, r *http.Request) {
	items, err := a.Queries.ListAllShopItems(r.Context())
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	sold, err := a.soldByItem(r.Context())
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	out := []shopItemJSON{}
	for _, item := range items {
		j := toShopItemJSON(item, 0)
		if item.Kind == db.ShopItemKindTicket {
			j.Batches = toBatchesJSON(item, sold[item.ID])
		}
		out = append(out, j)
	}
	respond.JSON(w, http.StatusOK, out)
}

// adminCreateShopItem adds an item. Tickets and price locks default to one
// per person.
func (a *API) adminCreateShopItem(w http.ResponseWriter, r *http.Request) {
	var fields shopItemFields
	if !decode(w, r, &fields) {
		return
	}
	item := db.ShopItem{Active: true}
	if fields.Kind != nil && (*fields.Kind == db.ShopItemKindTicket || *fields.Kind == db.ShopItemKindPriceLock) && !fields.MaxPerUser.Set {
		one := int32(1)
		item.MaxPerUser = &one
	}
	if problem := fields.apply(&item); problem != "" {
		respond.Error(w, http.StatusUnprocessableEntity, problem)
		return
	}
	item, err := a.Queries.CreateShopItem(r.Context(), db.CreateShopItemParams{
		Kind: item.Kind, Name: item.Name, Description: item.Description, ImageUrl: item.ImageUrl, Price: item.Price,
		StipendCents: item.StipendCents, Stock: item.Stock, MaxPerUser: item.MaxPerUser, Active: item.Active,
		Position: item.Position, BatchSize: item.BatchSize, BatchStep: item.BatchStep,
	})
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toShopItemJSON(item, 0))
}

// adminUpdateShopItem edits an item. Price changes don't affect existing
// orders, which keep what was paid.
func (a *API) adminUpdateShopItem(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "itemID")
	if !ok {
		return
	}
	var fields shopItemFields
	if !decode(w, r, &fields) {
		return
	}
	tx, err := a.Pool.Begin(r.Context())
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())
	q := a.Queries.WithTx(tx)

	item, err := q.LockShopItem(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		respond.Error(w, http.StatusNotFound, "That item doesn't exist.")
		return
	}
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	if problem := fields.apply(&item); problem != "" {
		respond.Error(w, http.StatusUnprocessableEntity, problem)
		return
	}
	item, err = q.UpdateShopItem(r.Context(), db.UpdateShopItemParams{
		ID: item.ID, Kind: item.Kind, Name: item.Name, Description: item.Description, ImageUrl: item.ImageUrl, Price: item.Price,
		StipendCents: item.StipendCents, Stock: item.Stock, MaxPerUser: item.MaxPerUser, Active: item.Active,
		Position: item.Position, BatchSize: item.BatchSize, BatchStep: item.BatchStep,
	})
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		respond.ServerError(w, r, err)
		return
	}
	respond.JSON(w, http.StatusOK, toShopItemJSON(item, 0))
}

type adminOrderJSON struct {
	orderJSON
	User  adminUserJSON `json:"user"`
	Notes *string       `json:"notes"`
}

// adminListOrders lists every order, oldest first: ?status=pending for the
// ones still to fulfill.
func (a *API) adminListOrders(w http.ResponseWriter, r *http.Request) {
	var status *db.OrderStatus
	if s := db.OrderStatus(r.URL.Query().Get("status")); s != "" {
		if !s.Valid() {
			respond.Error(w, http.StatusUnprocessableEntity, fmt.Sprintf("status has to be one of %v.", db.AllOrderStatusValues()))
			return
		}
		status = &s
	}
	rows, err := a.Queries.AdminListOrders(r.Context(), status)
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	out := []adminOrderJSON{}
	for _, row := range rows {
		out = append(out, adminOrderJSON{
			orderJSON: toOrderJSON(row.Order, row.ShopItem),
			User:      toAdminUserJSON(row.User),
			Notes:     row.Order.Notes,
		})
	}
	respond.JSON(w, http.StatusOK, out)
}

// adminUpdateOrder marks a pending order fulfilled or cancelled: {"status":
// "fulfilled", "notes": "..."}. Cancelling refunds the stamps and puts the
// item back in stock.
func (a *API) adminUpdateOrder(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "orderID")
	if !ok {
		return
	}
	var body struct {
		Status db.OrderStatus `json:"status"`
		Notes  *string        `json:"notes"`
	}
	if !decode(w, r, &body) {
		return
	}
	if body.Status != db.OrderStatusFulfilled && body.Status != db.OrderStatusCancelled {
		respond.Error(w, http.StatusUnprocessableEntity, `status has to be "fulfilled" or "cancelled".`)
		return
	}

	tx, err := a.Pool.Begin(r.Context())
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())
	q := a.Queries.WithTx(tx)

	order, err := q.LockOrder(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		respond.Error(w, http.StatusNotFound, "That order doesn't exist.")
		return
	}
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	if order.Status != db.OrderStatusPending {
		respond.Error(w, http.StatusConflict, fmt.Sprintf("This order is already %s.", order.Status))
		return
	}
	notes := order.Notes
	if body.Notes != nil {
		notes = nonEmpty(strings.TrimSpace(*body.Notes))
	}
	order, err = q.SetOrderStatus(r.Context(), db.SetOrderStatusParams{ID: order.ID, Status: body.Status, Notes: notes})
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	if body.Status == db.OrderStatusCancelled {
		if _, err := q.AddStampEntry(r.Context(), db.AddStampEntryParams{
			UserID:  order.UserID,
			Kind:    db.StampEntryKindRefund,
			Amount:  order.Price,
			OrderID: &order.ID,
		}); err != nil {
			respond.ServerError(w, r, err)
			return
		}
		if err := q.ReturnShopItemStock(r.Context(), order.ShopItemID); err != nil {
			respond.ServerError(w, r, err)
			return
		}
	}
	item, err := q.GetShopItem(r.Context(), order.ShopItemID)
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		respond.ServerError(w, r, err)
		return
	}
	respond.JSON(w, http.StatusOK, toOrderJSON(order, item))
}

// adminAdjustStamps adds or takes away stamps by hand, with a note saying
// why: {"amount": -300, "note": "clawback: copied project"}. It can take
// someone below zero.
func (a *API) adminAdjustStamps(w http.ResponseWriter, r *http.Request) {
	admin := auth.User(r.Context())
	userID, ok := idParam(w, r, "userID")
	if !ok {
		return
	}
	var body struct {
		Amount int32  `json:"amount"`
		Note   string `json:"note"`
	}
	if !decode(w, r, &body) {
		return
	}
	body.Note = strings.TrimSpace(body.Note)
	switch {
	case body.Amount == 0:
		respond.Error(w, http.StatusUnprocessableEntity, "amount can't be 0.")
		return
	case body.Note == "" || len(body.Note) > 1000:
		respond.Error(w, http.StatusUnprocessableEntity, "Add a note under 1,000 characters saying why.")
		return
	}
	tx, err := a.Pool.Begin(r.Context())
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())
	q := a.Queries.WithTx(tx)

	if _, err := q.LockUser(r.Context(), userID); errors.Is(err, pgx.ErrNoRows) {
		respond.Error(w, http.StatusNotFound, "That user doesn't exist.")
		return
	} else if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	if _, err := q.AddStampEntry(r.Context(), db.AddStampEntryParams{
		UserID:    userID,
		Kind:      db.StampEntryKindAdjustment,
		Amount:    body.Amount,
		CreatedBy: &admin.ID,
		Note:      &body.Note,
	}); err != nil {
		respond.ServerError(w, r, err)
		return
	}
	user, err := q.GetUser(r.Context(), userID)
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		respond.ServerError(w, r, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toAdminUserJSON(user))
}
