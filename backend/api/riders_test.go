package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/scherepi/crossroads-backend/cachet"
	"github.com/scherepi/crossroads-backend/db"
)

// fakeCachet answers like Cachet: the Slack picture of everyone in pictures,
// and its placeholder for anyone else.
type fakeCachet struct {
	mu       sync.Mutex
	pictures map[string]string // by Slack ID
	asked    []string
	down     bool
}

func (f *fakeCachet) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := strings.TrimPrefix(r.URL.Path, "/users/")
	f.asked = append(f.asked, id)
	if f.down {
		http.Error(w, "down", http.StatusServiceUnavailable)
		return
	}
	picture, ok := f.pictures[id]
	if !ok {
		picture = "https://l4-bucket.dunkirk.sh/5DjfoBI58Pfw.webp"
		w.WriteHeader(http.StatusAccepted)
	}
	json.NewEncoder(w).Encode(map[string]string{"userId": id, "imageUrl": picture})
}

func (f *fakeCachet) askedSoFar() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.asked)
}

func (e *env) riders(t *testing.T) []string {
	t.Helper()
	var out struct {
		SlackIDs []string `json:"slack_ids"`
	}
	e.mustCall(t, nil, http.StatusOK, "GET", "/api/riders", nil, &out)
	if out.SlackIDs == nil {
		t.Fatal("slack_ids isn't a list")
	}
	slices.Sort(out.SlackIDs)
	return out.SlackIDs
}

func (e *env) refreshRiders(t *testing.T) error {
	t.Helper()
	return e.api.RefreshRiders(context.Background())
}

func TestRiders(t *testing.T) {
	e := newEnv(t)
	fake := &fakeCachet{pictures: map[string]string{}}
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)
	e.api.Cachet = cachet.New(server.URL)
	own := func(slackID string) {
		fake.pictures[slackID] = "https://avatars.slack-edge.com/2026-11-10/1_" + slackID + "_512.jpg"
	}
	// ship ships a project for someone new, gets it reviewed, and gives their
	// answer to riding the trains, if any.
	ship := func(slackID, status string, answer any) *db.User {
		t.Helper()
		user, id := e.shipFor(t, slackID, 1)
		if status != "pending" {
			e.review(t, e.projectOf(t, id), status)
		}
		if answer != nil {
			e.mustCall(t, user, http.StatusOK, "PATCH", "/api/me", map[string]any{"on_landing_page": answer}, nil)
		}
		return user
	}

	// Nobody rides before the first refresh.
	if got := e.riders(t); len(got) != 0 {
		t.Fatalf("riders before refreshing: %v", got)
	}

	// Riders said yes, and have an approved ship and a picture of their own.
	own("U1")
	u1 := ship("U1", "approved", true)
	ship("U2", "approved", true) // Slack's default picture
	fake.pictures["U3"] = "https://secure.gravatar.com/avatar/1.jpg?d=https%3A%2F%2Fa.slack-edge.com%2Fava_0018-512.png"
	ship("U3", "approved", true)
	own("U4")
	ship("U4", "pending", true)
	own("U5")
	ship("U5", "rejected", true)
	own("UNO")
	ship("UNO", "approved", false)
	own("UASK")
	ship("UASK", "approved", nil) // not asked yet
	if err := e.refreshRiders(t); err != nil {
		t.Fatal(err)
	}
	if got := e.riders(t); !slices.Equal(got, []string{"U1"}) {
		t.Fatalf("riders are %v, want [U1]", got)
	}
	// Who's been asked about isn't asked again the same day.
	asked := fake.askedSoFar()
	if err := e.refreshRiders(t); err != nil {
		t.Fatal(err)
	}
	if fake.askedSoFar() != asked {
		t.Errorf("asked Cachet %d more times", fake.askedSoFar()-asked)
	}

	// Cachet being down stops the refresh after one try, and whoever was
	// riding still is.
	own("U6")
	ship("U6", "approved", true)
	own("U7")
	ship("U7", "approved", true)
	fake.down = true
	asked = fake.askedSoFar()
	if err := e.refreshRiders(t); err == nil {
		t.Error("Cachet being down wasn't reported")
	}
	if fake.askedSoFar() != asked+1 {
		t.Errorf("asked Cachet %d times while it was down", fake.askedSoFar()-asked)
	}
	if got := e.riders(t); !slices.Equal(got, []string{"U1"}) {
		t.Errorf("riders while Cachet is down are %v, want [U1]", got)
	}
	fake.down = false
	if err := e.refreshRiders(t); err != nil {
		t.Fatal(err)
	}
	if got := e.riders(t); !slices.Equal(got, []string{"U1", "U6", "U7"}) {
		t.Errorf("riders once Cachet's back are %v", got)
	}

	// At most 8 at a time, a different few each time.
	for _, id := range []string{"U8", "U9", "U10", "U11", "U12", "U13", "U14"} {
		own(id)
		ship(id, "approved", true)
	}
	if err := e.refreshRiders(t); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for range 20 {
		got := e.riders(t)
		if len(got) != 8 {
			t.Fatalf("%d riders, want 8", len(got))
		}
		for _, id := range got {
			seen[id] = true
		}
	}
	if len(seen) != 10 {
		t.Errorf("20 picks only had %d of the 10 riders", len(seen))
	}

	// Saying no takes them off straight away, and the next refresh leaves
	// them off.
	e.mustCall(t, u1, http.StatusOK, "PATCH", "/api/me", map[string]any{"on_landing_page": false}, nil)
	for range 20 {
		if slices.Contains(e.riders(t), "U1") {
			t.Fatal("U1 still rides after saying no")
		}
	}
	if err := e.refreshRiders(t); err != nil {
		t.Fatal(err)
	}
	for range 20 {
		if slices.Contains(e.riders(t), "U1") {
			t.Fatal("U1 rides again after a refresh")
		}
	}
}

func TestNoRidersWithoutCachet(t *testing.T) {
	e := newEnv(t)
	user, id := e.shipFor(t, "U1", 1)
	e.review(t, e.projectOf(t, id), "approved")
	e.mustCall(t, user, http.StatusOK, "PATCH", "/api/me", map[string]any{"on_landing_page": true}, nil)
	if err := e.refreshRiders(t); err != nil {
		t.Fatal(err)
	}
	if got := e.riders(t); len(got) != 0 {
		t.Errorf("riders without Cachet: %v", got)
	}
}
