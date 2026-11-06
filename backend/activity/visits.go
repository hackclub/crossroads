package activity

import (
	"context"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/scherepi/crossroads-backend/auth"
	"github.com/scherepi/crossroads-backend/db"
)

// Visits records which UTC days each person uses the platform, for the
// admin activity page. Their first signed-in request each day adds a row to
// platform_activity; after that, it remembers the day so later requests
// don't touch the database. Each server remembers on its own, which only
// costs a repeated insert that does nothing.
type Visits struct {
	Queries *db.Queries
	// Now is the clock, replaceable in tests. Defaults to time.Now.
	Now func() time.Time

	mu   sync.Mutex
	seen map[int64]time.Time // user ID → the last day recorded
}

// Record is middleware that records a visit for whoever is signed in. It
// goes after auth.LoadSession. Failing to record is logged, not shown to
// anyone, and tried again on their next request.
func (v *Visits) Record(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if user := auth.User(r.Context()); user != nil {
			v.record(r.Context(), user.ID)
		}
		next.ServeHTTP(w, r)
	})
}

func (v *Visits) record(ctx context.Context, userID int64) {
	now := time.Now
	if v.Now != nil {
		now = v.Now
	}
	today := Day(now())
	v.mu.Lock()
	done := v.seen[userID].Equal(today)
	v.mu.Unlock()
	if done {
		return
	}
	// A request that's cancelled mid-insert still counts as a visit.
	if err := v.Queries.RecordVisit(context.WithoutCancel(ctx), db.RecordVisitParams{UserID: userID, Day: today}); err != nil {
		log.Printf("recording a visit: %v", err)
		return
	}
	v.mu.Lock()
	if v.seen == nil {
		v.seen = map[int64]time.Time{}
	}
	v.seen[userID] = today
	v.mu.Unlock()
}
