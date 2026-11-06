package activity

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/scherepi/crossroads-backend/auth"
	"github.com/scherepi/crossroads-backend/db"
	"github.com/scherepi/crossroads-backend/db/dbtest"
)

func TestVisits(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	queries := db.New(pool)
	var userID int64
	if err := pool.QueryRow(ctx, `insert into users (hca_id, email, name) values ('ident!visits', 'x@example.com', 'Test')
		returning id`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	user, err := queries.GetUser(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}

	now := at(5, 23, 0)
	visits := &Visits{Queries: queries, Now: func() time.Time { return now }}
	reached := 0
	handler := visits.Record(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached++ }))
	visit := func(signedIn bool) {
		req := httptest.NewRequest("GET", "/api/me", nil)
		if signedIn {
			req = req.WithContext(auth.WithUser(req.Context(), &user))
		}
		handler.ServeHTTP(httptest.NewRecorder(), req)
	}
	days := func() []string {
		t.Helper()
		rows, err := pool.Query(ctx, "select day from platform_activity where user_id = $1 order by day", userID)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var out []string
		for rows.Next() {
			var day time.Time
			if err := rows.Scan(&day); err != nil {
				t.Fatal(err)
			}
			out = append(out, day.Format(time.DateOnly))
		}
		return out
	}

	visit(false)
	visit(true)
	visit(true)
	if got, want := days(), []string{"2026-11-05"}; !slices.Equal(got, want) {
		t.Errorf("after visits on the 5th: got %v, want %v", got, want)
	}
	if _, ok := visits.seen[userID]; !ok {
		t.Error("the day wasn't remembered, so every request would write")
	}

	// Past midnight UTC is a new day.
	now = at(6, 0, 30)
	visit(true)
	if got, want := days(), []string{"2026-11-05", "2026-11-06"}; !slices.Equal(got, want) {
		t.Errorf("after a visit on the 6th: got %v, want %v", got, want)
	}

	// Someone remembered by another server instance still only gets one row.
	visits.seen = nil
	visit(true)
	if got := days(); len(got) != 2 {
		t.Errorf("a repeated insert added a row: %v", got)
	}
	if reached != 5 {
		t.Errorf("%d of 5 requests reached the handler", reached)
	}
}
