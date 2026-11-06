package activity

import (
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/scherepi/crossroads-backend/db"
	"github.com/scherepi/crossroads-backend/db/dbtest"
	"github.com/scherepi/crossroads-backend/hackatime"
)

// at is a time in November 2026, UTC.
func at(day, hour, minute int) time.Time {
	return time.Date(2026, time.November, day, hour, minute, 0, 0, time.UTC)
}

func TestByDay(t *testing.T) {
	spans := []hackatime.Span{
		{Start: at(4, 10, 0), End: at(4, 10, 30)},                           // 30m on the 4th
		{Start: at(4, 23, 30), End: at(5, 0, 45)},                           // 30m on the 4th, 45m on the 5th
		{Start: at(2, 23, 0), End: at(3, 1, 0)},                             // only the hour after from counts
		{Start: at(6, 11, 0), End: at(6, 13, 0)},                            // only the hour before to counts
		{Start: at(1, 9, 0), End: at(1, 10, 0)},                             // all before from
		{Start: at(5, 8, 0), End: at(5, 8, 0)},                              // empty
		{Start: at(5, 9, 0), End: at(5, 9, 0).Add(1400 * time.Millisecond)}, // rounds to 1s
	}
	got := ByDay(spans, at(3, 0, 0), at(6, 12, 0))
	want := map[time.Time]int{at(3, 0, 0): 3600, at(4, 0, 0): 3600, at(5, 0, 0): 2701, at(6, 0, 0): 3600}
	if !maps.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// fakeHackatime serves heartbeat spans. Each span belongs to a Hackatime
// project, and only those in filter_by_project come back.
type fakeHackatime struct {
	mu      sync.Mutex
	spans   map[string][]fakeSpan // Slack ID → their spans
	private map[string]bool
}

type fakeSpan struct {
	project    string
	start, end time.Time
}

func (f *fakeHackatime) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	slackID, ok := strings.CutSuffix(strings.TrimPrefix(r.URL.Path, "/api/v1/users/"), "/heartbeats/spans")
	if !ok {
		http.NotFound(w, r)
		return
	}
	if f.private[slackID] {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	spans, ok := f.spans[slackID]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	start, _ := time.Parse(time.RFC3339, r.URL.Query().Get("start_date"))
	end, _ := time.Parse(time.RFC3339, r.URL.Query().Get("end_date"))
	filter := strings.Split(r.URL.Query().Get("filter_by_project"), ",")
	type span struct {
		StartTime float64 `json:"start_time"`
		EndTime   float64 `json:"end_time"`
		Duration  float64 `json:"duration"`
	}
	out := []span{}
	for _, s := range spans {
		if slices.Contains(filter, s.project) && s.end.After(start) && s.start.Before(end) {
			out = append(out, span{float64(s.start.Unix()), float64(s.end.Unix()), s.end.Sub(s.start).Seconds()})
		}
	}
	json.NewEncoder(w).Encode(map[string][]span{"spans": out})
}

type env struct {
	pool    *pgxpool.Pool
	queries *db.Queries
	fake    *fakeHackatime
	poller  *Poller
}

func newEnv(t *testing.T) *env {
	t.Helper()
	pool := dbtest.New(t)
	fake := &fakeHackatime{spans: map[string][]fakeSpan{}, private: map[string]bool{}}
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)
	queries := db.New(pool)
	return &env{pool, queries, fake, &Poller{
		Pool:      pool,
		Queries:   queries,
		Hackatime: hackatime.New(hackatime.Config{BaseURL: server.URL}),
		Start:     at(3, 0, 0),
		Now:       func() time.Time { return at(6, 12, 0) },
	}}
}

// project makes a project for a new user (with that Slack ID, unless it's
// empty) and links Hackatime projects to it.
func (e *env) project(t *testing.T, slackID, role string, hackatimeProjects ...string) int64 {
	t.Helper()
	ctx := context.Background()
	var slack any
	if slackID != "" {
		slack = slackID
	}
	var userID, projectID int64
	err := e.pool.QueryRow(ctx, `insert into users (hca_id, slack_id, email, name, role)
		values ($1, $2, 'x@example.com', 'Test', $3) returning id`, "ident!"+slackID+t.Name()+role, slack, role).Scan(&userID)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.pool.QueryRow(ctx, "insert into projects (user_id, name) values ($1, 'Racer') returning id", userID).Scan(&projectID); err != nil {
		t.Fatal(err)
	}
	for _, name := range hackatimeProjects {
		if _, err := e.pool.Exec(ctx, "insert into project_hackatime_projects (user_id, hackatime_project, project_id) values ($1, $2, $3)",
			userID, name, projectID); err != nil {
			t.Fatal(err)
		}
	}
	return projectID
}

// days returns a project's recorded seconds per day.
func (e *env) days(t *testing.T, projectID int64) map[string]int {
	t.Helper()
	rows, err := e.pool.Query(context.Background(), "select day, seconds from project_activity where project_id = $1", projectID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var day time.Time
		var seconds int
		if err := rows.Scan(&day, &seconds); err != nil {
			t.Fatal(err)
		}
		out[day.Format(time.DateOnly)] = seconds
	}
	return out
}

func TestPoll(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	racer := e.project(t, "U1", "participant", "racer", "racer-api")
	private := e.project(t, "U2", "participant", "secret")
	noSlack := e.project(t, "", "participant", "orphan")
	reviewers := e.project(t, "U3", "reviewer", "tool")
	e.fake.spans["U1"] = []fakeSpan{
		{"racer", at(4, 10, 0), at(4, 11, 0)},
		{"racer-api", at(4, 23, 0), at(5, 1, 0)},
		{"other", at(5, 9, 0), at(5, 10, 0)}, // not linked, so it doesn't count
	}
	e.fake.spans["U3"] = []fakeSpan{{"tool", at(6, 9, 0), at(6, 10, 0)}}
	e.fake.private["U2"] = true

	if err := e.poller.Poll(ctx, e.poller.Start); err != nil {
		t.Fatal(err)
	}
	if got, want := e.days(t, racer), map[string]int{"2026-11-04": 7200, "2026-11-05": 3600}; !maps.Equal(got, want) {
		t.Errorf("racer: got %v, want %v", got, want)
	}
	if got := e.days(t, private); len(got) != 0 {
		t.Errorf("private Hackatime should be skipped, got %v", got)
	}
	if got := e.days(t, noSlack); len(got) != 0 {
		t.Errorf("no Slack ID should be skipped, got %v", got)
	}
	if got, want := e.days(t, reviewers), map[string]int{"2026-11-06": 3600}; !maps.Equal(got, want) {
		t.Errorf("reviewer's project: got %v, want %v", got, want)
	}

	// A later pass only replaces the days it covers: the 5th loses its time
	// and the 6th gains some, while the 4th stays as it was.
	e.fake.spans["U1"] = []fakeSpan{{"racer", at(6, 8, 0), at(6, 8, 30)}}
	if err := e.poller.Poll(ctx, at(5, 15, 0)); err != nil {
		t.Fatal(err)
	}
	if got, want := e.days(t, racer), map[string]int{"2026-11-04": 7200, "2026-11-06": 1800}; !maps.Equal(got, want) {
		t.Errorf("racer after a recent pass: got %v, want %v", got, want)
	}

	// Daily numbers count participants only, and min_seconds sets how much
	// time makes someone active.
	daily := func(minSeconds int32) map[string][3]int64 {
		t.Helper()
		rows, err := e.queries.DailyProjectActivity(ctx, db.DailyProjectActivityParams{StartDay: at(3, 0, 0), EndDay: at(6, 0, 0), MinSeconds: minSeconds})
		if err != nil {
			t.Fatal(err)
		}
		out := map[string][3]int64{}
		for _, r := range rows {
			out[r.Day.Format(time.DateOnly)] = [3]int64{int64(r.ActiveParticipants), int64(r.ActiveProjects), r.Seconds}
		}
		return out
	}
	want := map[string][3]int64{
		"2026-11-03": {0, 0, 0},
		"2026-11-04": {1, 1, 7200},
		"2026-11-05": {0, 0, 0},
		"2026-11-06": {1, 1, 1800},
	}
	if got := daily(0); !maps.Equal(got, want) {
		t.Errorf("daily: got %v, want %v", got, want)
	}
	want["2026-11-06"] = [3]int64{0, 1, 1800} // 30 minutes isn't an hour
	if got := daily(3600); !maps.Equal(got, want) {
		t.Errorf("daily with an hour minimum: got %v, want %v", got, want)
	}
	for minSeconds, want := range map[int32]int32{0: 1, 3600: 1, 7201: 0} {
		got, err := e.queries.CountActiveParticipants(ctx, db.CountActiveParticipantsParams{StartDay: at(3, 0, 0), EndDay: at(6, 0, 0), MinSeconds: minSeconds})
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("active participants with at least %ds on a day: got %d, want %d", minSeconds, got, want)
		}
	}
}
