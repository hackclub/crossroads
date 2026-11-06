package api

// Tickets can sell in batches, each costing more than the last: the first
// batch_size cost the ticket's price, and every batch after that costs
// batch_step more. The next batch goes on sale when one sells out.
//
// A price lock is a shop item of its own. Buying one fixes the buyer's
// ticket at the price of the batch on sale then, for whenever they buy it.
// It only fixes the price; it doesn't hold a ticket. If the tickets sell out
// before they buy one, the lock is refunded.

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/scherepi/crossroads-backend/db"
)

// ticketPrice is what the next ticket costs once sold have been bought,
// before any price lock.
func ticketPrice(ticket db.ShopItem, sold int64) int32 {
	if ticket.BatchSize == nil {
		return ticket.Price
	}
	return ticket.Price + *ticket.BatchStep*int32(sold/int64(*ticket.BatchSize))
}

type batchesJSON struct {
	Size        int32   `json:"size"`          // tickets in each batch
	Prices      []int32 `json:"prices"`        // each batch's price, first to last
	Current     int     `json:"current"`       // the batch on sale, as an index into prices; the last once sold out
	LeftInBatch int32   `json:"left_in_batch"` // how many are left at the current price
}

// toBatchesJSON describes a ticket's batches once sold have been bought, or
// returns nil if it isn't sold in batches.
func toBatchesJSON(ticket db.ShopItem, sold int64) *batchesJSON {
	if ticket.BatchSize == nil || ticket.Stock == nil {
		return nil
	}
	size, left := int64(*ticket.BatchSize), int64(*ticket.Stock)
	count := max((sold+left+size-1)/size, 1)
	prices := make([]int32, count)
	for i := range prices {
		prices[i] = ticket.Price + *ticket.BatchStep*int32(i)
	}
	return &batchesJSON{
		Size:        int32(size),
		Prices:      prices,
		Current:     int(min(sold/size, count-1)),
		LeftInBatch: int32(min(size-sold%size, left)),
	}
}

// lockProblem says why someone can't buy a price lock right now, and the
// status to respond with, or returns the ticket price it would lock in. It
// locks the ticket's row, after the buyer's and the price lock's.
func lockProblem(ctx context.Context, q *db.Queries, userID int64) (int32, string, error) {
	if _, err := q.GetPriceLock(ctx, userID); err == nil {
		return 0, "You've already locked your ticket price.", nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return 0, "", err
	}
	ticket, err := q.LockTicket(ctx)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && ticket.BatchSize == nil) {
		return 0, "The ticket's price doesn't change, so there's nothing to lock.", nil
	}
	if err != nil {
		return 0, "", err
	}
	if has, err := q.HasTicket(ctx, userID); err != nil {
		return 0, "", err
	} else if has {
		return 0, "You've already got your ticket!", nil
	}
	if *ticket.Stock <= 0 {
		return 0, "The tickets are sold out.", nil
	}
	sold, err := q.CountSold(ctx, ticket.ID)
	if err != nil {
		return 0, "", err
	}
	batches := toBatchesJSON(ticket, sold)
	if batches.Current == len(batches.Prices)-1 {
		return 0, "This is the last batch of tickets, so the price won't go up.", nil
	}
	return ticketPrice(ticket, sold), "", nil
}

// RunPriceLockRefunds calls RefundPriceLocks every interval until ctx is
// done. Buying the last ticket refunds them straight away; this catches
// anything that failed then, and tickets an admin marks sold out.
func (a *API) RunPriceLockRefunds(ctx context.Context, interval time.Duration) {
	for {
		if err := a.RefundPriceLocks(ctx); err != nil {
			log.Printf("refunding price locks: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}
	}
}

// RefundPriceLocks gives back the stamps for every price lock that can't be
// used: the ticket is sold out and the holder never got one.
func (a *API) RefundPriceLocks(ctx context.Context) error {
	ticket, err := a.Queries.GetTicket(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if ticket.Stock == nil || *ticket.Stock > 0 {
		return nil
	}
	locks, err := a.Queries.ListUnusedPriceLocks(ctx)
	if err != nil {
		return err
	}
	var errs []error
	for _, lock := range locks {
		if err := a.refundPriceLock(ctx, lock.ID, lock.UserID); err != nil {
			errs = append(errs, fmt.Errorf("price lock order %d: %w", lock.ID, err))
		}
	}
	return errors.Join(errs...)
}

// refundPriceLock cancels one price lock order and refunds it, unless it's
// been refunded or its holder got a ticket since it was listed. It locks the
// holder first, as buying does.
func (a *API) refundPriceLock(ctx context.Context, orderID, userID int64) error {
	tx, err := a.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := a.Queries.WithTx(tx)

	if _, err := q.LockUser(ctx, userID); err != nil {
		return err
	}
	order, err := q.LockOrder(ctx, orderID)
	if err != nil {
		return err
	}
	if order.Status == db.OrderStatusCancelled {
		return nil
	}
	if has, err := q.HasTicket(ctx, userID); err != nil || has {
		return err
	}
	note := "Refunded automatically: the tickets sold out before they bought one."
	if _, err := q.SetOrderStatus(ctx, db.SetOrderStatusParams{ID: order.ID, Status: db.OrderStatusCancelled, Notes: &note}); err != nil {
		return err
	}
	if _, err := q.AddStampEntry(ctx, db.AddStampEntryParams{
		UserID:  userID,
		Kind:    db.StampEntryKindRefund,
		Amount:  order.Price,
		OrderID: &order.ID,
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
