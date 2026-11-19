package api

// The landing page's trains carry the team and a few participants, as their
// Slack pictures. Riders are people who said yes to it (on_landing_page; the
// platform asks everyone once) with an approved ship and a Slack picture of
// their own, not Slack's default, which Cachet tells us.
// RunRiders keeps the list up to date in the background, so the landing page
// never waits on Cachet, and GET /api/riders picks a few at random for anyone
// who asks, signed in or not.

import (
	"context"
	"fmt"
	"log"
	"math/rand/v2"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/scherepi/crossroads-backend/respond"
)

const (
	ridersShown = 8 // how many GET /api/riders picks
	// How long Cachet's word on someone's picture holds before it's asked again.
	pictureFreshFor = 24 * time.Hour
)

type riders struct {
	refreshing sync.Mutex // one refresh at a time; it guards checked
	checked    map[string]pictureCheck

	mu   sync.Mutex
	ids  []string             // Slack IDs
	left map[string]time.Time // who said no since a refresh, and when
}

type pictureCheck struct {
	own bool
	at  time.Time
}

// RunRiders refreshes the riders every interval until ctx is done.
func (a *API) RunRiders(ctx context.Context, interval time.Duration) {
	for {
		if err := a.RefreshRiders(ctx); err != nil {
			log.Printf("finding riders: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}
	}
}

// RefreshRiders asks Cachet about the pictures of everyone who could ride
// and hasn't been asked about lately. Without Cachet nobody rides.
func (a *API) RefreshRiders(ctx context.Context) error {
	if a.Cachet == nil {
		return nil
	}
	asOf := time.Now()
	ids, err := a.Queries.ListRiders(ctx)
	if err != nil {
		return err
	}
	r := &a.riders
	r.refreshing.Lock()
	defer r.refreshing.Unlock()
	if r.checked == nil {
		r.checked = map[string]pictureCheck{}
	}
	var riding []string
	var cachetErr error
	for _, id := range ids {
		check, ok := r.checked[id]
		if cachetErr == nil && (!ok || time.Since(check.at) > pictureFreshFor) {
			own, err := a.Cachet.OwnPicture(ctx, id)
			if err != nil {
				// Cachet's struggling: everyone else keeps what it said
				// before, and gets asked next time.
				cachetErr = fmt.Errorf("asking Cachet about %s: %w", id, err)
			} else {
				check, ok = pictureCheck{own: own, at: time.Now()}, true
				r.checked[id] = check
			}
		}
		if ok && check.own {
			riding = append(riding, id)
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	// Whoever said no after ListRiders stays off; anyone before, it left out.
	for id, at := range r.left {
		if at.After(asOf) {
			riding = slices.DeleteFunc(riding, func(rider string) bool { return rider == id })
		} else {
			delete(r.left, id)
		}
	}
	r.ids = riding
	return cachetErr
}

// dropRider takes someone off the trains now, without waiting for the next
// refresh.
func (a *API) dropRider(slackID string) {
	r := &a.riders
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ids = slices.DeleteFunc(r.ids, func(id string) bool { return id == slackID })
	if r.left == nil {
		r.left = map[string]time.Time{}
	}
	r.left[slackID] = time.Now()
}

// listRiders is GET /api/riders: the Slack IDs of up to ridersShown riders, a
// different few each time.
func (a *API) listRiders(w http.ResponseWriter, r *http.Request) {
	picked := []string{}
	a.riders.mu.Lock()
	for _, i := range rand.Perm(len(a.riders.ids))[:min(len(a.riders.ids), ridersShown)] {
		picked = append(picked, a.riders.ids[i])
	}
	a.riders.mu.Unlock()
	respond.JSON(w, http.StatusOK, struct {
		SlackIDs []string `json:"slack_ids"`
	}{picked})
}
