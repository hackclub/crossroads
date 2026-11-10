package airtablesync

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/scherepi/crossroads-backend/airtable"
	"github.com/scherepi/crossroads-backend/db"
	"github.com/scherepi/crossroads-backend/db/dbtest"
	"github.com/scherepi/crossroads-backend/unified"
)

// fakeAirtable keeps records in memory. Its only formula is the one
// pullFulfilled uses.
type fakeAirtable struct {
	mu       sync.Mutex
	tables   map[string][]*airtable.Record
	nextID   int
	requests []string // like "POST Shop Orders"
}

func (f *fakeAirtable) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.Header.Get("Authorization") != "Bearer test-token" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	table := strings.TrimPrefix(r.URL.Path, "/v0/appTEST/")
	f.requests = append(f.requests, r.Method+" "+table)

	switch r.Method {
	case http.MethodPost, http.MethodPatch:
		var body struct {
			Records []airtable.Record `json:"records"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		for _, in := range body.Records {
			if r.Method == http.MethodPost {
				f.nextID++
				record := &airtable.Record{ID: fmt.Sprintf("rec%d", f.nextID), Fields: in.Fields}
				f.tables[table] = append(f.tables[table], record)
				continue
			}
			record := f.find(table, in.ID)
			if record == nil {
				http.Error(w, "no record "+in.ID, http.StatusNotFound)
				return
			}
			for k, v := range in.Fields {
				record.Fields[k] = v
			}
		}
		json.NewEncoder(w).Encode(map[string]any{"records": f.tables[table][len(f.tables[table])-len(body.Records):]})
	case http.MethodGet:
		if formula := r.URL.Query().Get("filterByFormula"); formula != "AND({Fulfilled}, {Status} = 'pending')" {
			http.Error(w, "unexpected formula "+formula, http.StatusUnprocessableEntity)
			return
		}
		matches := []*airtable.Record{}
		for _, record := range f.tables[table] {
			if record.Fields["Fulfilled"] == true && record.Fields["Status"] == "pending" {
				matches = append(matches, record)
			}
		}
		json.NewEncoder(w).Encode(map[string]any{"records": matches})
	}
}

func (f *fakeAirtable) find(table, id string) *airtable.Record {
	for _, record := range f.tables[table] {
		if record.ID == id {
			return record
		}
	}
	return nil
}

func (f *fakeAirtable) records(table string) []*airtable.Record {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.tables[table]
}

func (f *fakeAirtable) writes() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, r := range f.requests {
		if !strings.HasPrefix(r, "GET") {
			n++
		}
	}
	return n
}

type env struct {
	pool     *pgxpool.Pool
	queries  *db.Queries
	airtable *fakeAirtable
	syncer   *Syncer
	user     db.User
}

func newEnv(t *testing.T) *env {
	t.Helper()
	pool := dbtest.New(t)
	fake := &fakeAirtable{tables: map[string][]*airtable.Record{}}
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)

	queries := db.New(pool)
	birthday := time.Date(2009, 4, 1, 0, 0, 0, 0, time.UTC)
	slack := "U1"
	address, _ := json.Marshal(db.Address{Name: "Heidi Trashworth", Line1: "15 Falls Rd", City: "Shelburne", State: "VT", PostalCode: "05482", Country: "US"})
	user, err := queries.UpsertUserFromHCA(context.Background(), db.UpsertUserFromHCAParams{
		HcaID: "ident!heidi", SlackID: &slack, Email: "heidi@example.com", Name: "Heidi Trashworth",
		FirstName: "Heidi", LastName: "Trashworth", Birthday: &birthday, Address: address,
		VerificationStatus: "verified", YswsEligible: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return &env{
		pool:     pool,
		queries:  queries,
		airtable: fake,
		syncer: &Syncer{
			Queries:    queries,
			Airtable:   airtable.New(airtable.Config{Token: "test-token", BaseID: "appTEST", BaseURL: server.URL}),
			HoursStart: time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC),
		},
		user: user,
	}
}

func (e *env) exec(t *testing.T, sql string, args ...any) int64 {
	t.Helper()
	var id int64
	if err := e.pool.QueryRow(context.Background(), sql+" returning id", args...).Scan(&id); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return id
}

func (e *env) sync(t *testing.T) {
	t.Helper()
	if err := e.syncer.SyncOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func (e *env) submission(t *testing.T, projectID int64, status string, seconds int) {
	t.Helper()
	reviewer := e.exec(t, "insert into users (hca_id, email, name, role) values ($1, 'r@example.com', 'Reviewer', 'reviewer') on conflict (hca_id) do update set name = excluded.name", "ident!reviewer")
	if status == "pending" {
		e.exec(t, `insert into submissions (project_id, hackatime_projects, hackatime_seconds, new_seconds)
			values ($1, '{racer}', $2, $2)`, projectID, seconds)
		return
	}
	approved := any(nil)
	if status == "approved" {
		approved = seconds
	}
	e.exec(t, `insert into submissions (project_id, status, hackatime_projects, hackatime_seconds, new_seconds,
		approved_seconds, reviewer_id, reviewed_at) values ($1, $2, '{racer}', $3, $3, $4, $5, now())`,
		projectID, status, seconds, approved, reviewer)
}

func TestApprovedSubmissionsAreCopiedOnce(t *testing.T) {
	e := newEnv(t)
	project := e.exec(t, `insert into projects (user_id, name, description, repo_url, demo_url, screenshot_url)
		values ($1, 'Racer', 'A racing game', 'https://github.com/heidi/racer', 'https://heidi.example/racer', 'https://cdn.example/racer.png')`, e.user.ID)
	e.submission(t, project, "rejected", 3600)
	e.submission(t, project, "approved", 2*3600)
	e.submission(t, project, "pending", 3600)

	e.sync(t)
	e.sync(t)
	records := e.airtable.records(submissionsTable)
	if len(records) != 1 {
		t.Fatalf("%d rows in Airtable, want just the 1 approved submission", len(records))
	}
	got := records[0].Fields
	for field, want := range map[string]any{
		"Code URL":                        "https://github.com/heidi/racer",
		"Playable URL":                    "https://heidi.example/racer",
		"First Name":                      "Heidi",
		"Last Name":                       "Trashworth",
		"Email":                           "heidi@example.com",
		"Slack ID":                        "U1",
		"GitHub Username":                 "heidi",
		"Birthday":                        "2009-04-01",
		"Address (Line 1)":                "15 Falls Rd",
		"ZIP / Postal Code":               "05482",
		"YSWS Eligible":                   true,
		"Hackatime List":                  "racer",
		"Optional - Override Hours Spent": float64(2),
	} {
		if got[field] != want {
			t.Errorf("%s is %v, want %v", field, got[field], want)
		}
	}
	if screenshot, _ := json.Marshal(got["Screenshot"]); string(screenshot) != `[{"url":"https://cdn.example/racer.png"}]` {
		t.Errorf("Screenshot is %s", screenshot)
	}
	if _, ok := got["Automation - Submit to Unified YSWS"]; ok {
		t.Error("the sync ticked the Unified YSWS box; a person is meant to")
	}
	if _, ok := got["Optional - Override Duplicate Justification"]; ok {
		t.Error("a first submission got a duplicate justification")
	}

	// An update to the same project says it's an update.
	e.exec(t, "update submissions set status = 'approved', approved_seconds = new_seconds, reviewer_id = (select id from users where hca_id = 'ident!reviewer'), reviewed_at = now() where status = 'pending'")
	e.sync(t)
	records = e.airtable.records(submissionsTable)
	if len(records) != 2 || records[1].Fields["Optional - Override Duplicate Justification"] == nil {
		t.Errorf("got %d rows; the update's duplicate justification is %v", len(records), records[len(records)-1].Fields["Optional - Override Duplicate Justification"])
	}
}

func TestOrdersSyncAndComeBackFulfilled(t *testing.T) {
	e := newEnv(t)
	hoodie := e.exec(t, "insert into shop_items (kind, name, price) values ('physical', 'Hoodie', 300)")
	stipend := e.exec(t, "insert into shop_items (kind, name, price, stipend_cents) values ('stipend', 'Travel stipend', 600, 5000)")
	address := `{"name": "Heidi T", "line_1": "15 Falls Rd", "city": "Shelburne", "country": "US"}`
	hoodieOrder := e.exec(t, "insert into orders (user_id, shop_item_id, price, shipping_address) values ($1, $2, 300, $3)", e.user.ID, hoodie, address)
	stipendOrder := e.exec(t, "insert into orders (user_id, shop_item_id, price) values ($1, $2, 600)", e.user.ID, stipend)

	e.sync(t)
	records := e.airtable.records(ordersTable)
	if len(records) != 2 {
		t.Fatalf("%d orders in Airtable, want 2", len(records))
	}
	if f := records[0].Fields; f["Order"] != fmt.Sprintf("#%d Hoodie", hoodieOrder) || f["Status"] != "pending" || f["City"] != "Shelburne" || f["Name"] != "Heidi T" {
		t.Errorf("hoodie order is %v", f)
	}
	if f := records[1].Fields; f["Stipend"] != float64(50) || f["Kind"] != "stipend" {
		t.Errorf("stipend order is %v", f)
	}

	writes := e.airtable.writes()
	e.sync(t)
	if e.airtable.writes() != writes {
		t.Error("syncing again with nothing changed wrote to Airtable")
	}

	// Someone ships the hoodie and ticks it off in Airtable.
	e.airtable.mu.Lock()
	records[0].Fields["Fulfilled"] = true
	records[0].Fields["Fulfillment Notes"] = "USPS 9400..."
	e.airtable.mu.Unlock()
	e.sync(t)

	var status string
	e.pool.QueryRow(context.Background(), "select status from orders where id = $1", hoodieOrder).Scan(&status)
	if status != "fulfilled" {
		t.Errorf("the order is %s in the backend, want fulfilled", status)
	}
	if f := e.airtable.records(ordersTable)[0].Fields; f["Status"] != "fulfilled" || f["Fulfilled"] != true || f["Fulfillment Notes"] != "USPS 9400..." {
		t.Errorf("after fulfilling, the row is %v", f)
	}

	// Cancelling in the backend updates the same row.
	e.exec(t, "update orders set status = 'cancelled' where id = $1", stipendOrder)
	e.sync(t)
	records = e.airtable.records(ordersTable)
	if len(records) != 2 || records[1].Fields["Status"] != "cancelled" {
		t.Errorf("after cancelling: %d rows, stipend status %v", len(records), records[1].Fields["Status"])
	}
}

func TestGithubUsername(t *testing.T) {
	for in, want := range map[string]string{
		"https://github.com/heidi/racer":      "heidi",
		"https://www.GitHub.com/heidi/racer/": "heidi",
		"https://gitlab.com/heidi/racer":      "",
		"https://github.com/":                 "",
		"https://heidi.github.io/racer":       "",
	} {
		got := unified.GitHubUsername(&in)
		if (got == nil && want != "") || (got != nil && *got != want) {
			t.Errorf("GitHubUsername(%q) = %v, want %q", in, got, want)
		}
	}
}

func TestHQSignedOffShipsGoToUnifiedYSWS(t *testing.T) {
	e := newEnv(t)
	e.exec(t, "update users set ysws_eligible = false where id = $1", e.user.ID)
	project := e.exec(t, `insert into projects (user_id, name, description, repo_url) values ($1, 'Racer', 'A racing game', 'https://github.com/heidi/racer')`, e.user.ID)
	reviewer := e.exec(t, "insert into users (hca_id, email, name, role) values ('ident!rev', 'r@example.com', 'Rita', 'reviewer')")
	fraud := e.exec(t, "insert into users (hca_id, email, name, role) values ('ident!fraud', 'f@example.com', 'Fern', 'reviewer')")
	hq := e.exec(t, "insert into users (hca_id, email, name, role) values ('ident!hq', 'h@example.com', 'Hugo', 'reviewer')")
	ship := e.exec(t, `insert into submissions (project_id, status, hackatime_projects, hackatime_seconds, new_seconds, approved_seconds,
		reviewer_id, reviewed_at, internal_notes, fraud_reasons, fraud_reviewer_id, fraud_reviewed_at, hq_reviewer_id, hq_reviewed_at,
		technical_features, deflation_reason)
		values ($1, 'approved', '{racer}', 36000, 36000, 25200, $2, now(), 'Commits match.', '{long_day}', $3, now(), $4, now(),
		'Split-screen multiplayer and a track editor', 'An hour of Hackatime was a tutorial.')`,
		project, reviewer, fraud, hq)
	e.exec(t, `insert into ship_events (submission_id, kind, actor_id, stage, seconds, deflation_reason)
		values ($1, 'approval', $2, 'pending', 32400, 'An hour of Hackatime was a tutorial')`, ship, reviewer)
	e.exec(t, `insert into ship_events (submission_id, kind, actor_id, stage, seconds, deflation_reason)
		values ($1, 'fraud_pass', $2, 'fraud_check', 25200, 'The IDE was left open overnight on day 3')`, ship, fraud)

	e.sync(t)
	records := e.airtable.records(submissionsTable)
	if len(records) != 1 {
		t.Fatalf("%d rows, want 1", len(records))
	}
	fields := records[0].Fields
	if _, ok := fields[submitUnifiedField]; ok {
		t.Error("ticked for Unified YSWS before they're YSWS-eligible")
	}
	justification, _ := fields["Optional - Override Hours Spent Justification"].(string)
	for _, want := range []string{
		"Hackatime: 10.00h of new time on racer",
		"Technical features: Split-screen multiplayer and a track editor.",
		"Reviewer Rita approved 9.00h. Commits match.",
		"Deflated from 10.00h to 9.00h by the reviewer: An hour of Hackatime was a tutorial.",
		"Sent to a fraud check because: a very long day. Fern passed it at 7.00h.",
		"Deflated from 9.00h to 7.00h in the fraud check: The IDE was left open overnight on day 3.",
		"Signed off by Hugo at 7.00h.",
	} {
		if !strings.Contains(justification, want) {
			t.Errorf("justification %q doesn't say %q", justification, want)
		}
	}

	// Verified, but the project has no screenshot or demo link, which the
	// Unified YSWS DB needs: still not ticked.
	e.exec(t, "update users set ysws_eligible = true where id = $1", e.user.ID)
	e.sync(t)
	if _, ok := e.airtable.records(submissionsTable)[0].Fields[submitUnifiedField]; ok {
		t.Error("ticked for Unified YSWS without a screenshot or demo link")
	}

	// Once they're added, the next pass sends the whole row again, ticked,
	// and only once.
	e.exec(t, `update projects set demo_url = 'https://heidi.example.com/racer', screenshot_url = 'https://heidi.example.com/racer.png' where id = $1`, project)
	e.sync(t)
	writes := e.airtable.writes()
	e.sync(t)
	fields = e.airtable.records(submissionsTable)[0].Fields
	if fields[submitUnifiedField] != true {
		t.Error("not ticked once it was complete")
	}
	if fields["Playable URL"] == nil || fields["Screenshot"] == nil {
		t.Errorf("the demo link and screenshot weren't sent with the tick: %v, %v", fields["Playable URL"], fields["Screenshot"])
	}
	if e.airtable.writes() != writes {
		t.Error("ticked again on the next pass")
	}

	// A new signed-off ship from someone eligible is ticked straight away.
	e.exec(t, `insert into submissions (project_id, status, hackatime_projects, hackatime_seconds, new_seconds, approved_seconds,
		reviewer_id, reviewed_at, hq_reviewer_id, hq_reviewed_at) values ($1, 'approved', '{racer}', 39600, 3600, 3600, $2, now(), $2, now())`,
		project, hq)
	e.sync(t)
	records = e.airtable.records(submissionsTable)
	if len(records) != 2 || records[1].Fields[submitUnifiedField] != true {
		t.Errorf("the second ship's Unified box is %v", records[len(records)-1].Fields[submitUnifiedField])
	}
}
