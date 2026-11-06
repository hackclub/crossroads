package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/scherepi/crossroads-backend/api"
	"github.com/scherepi/crossroads-backend/auth"
	"github.com/scherepi/crossroads-backend/db"
	"github.com/scherepi/crossroads-backend/db/dbtest"
	"github.com/scherepi/crossroads-backend/hackatime"
	"github.com/scherepi/crossroads-backend/hca"
)

var hoursStart = time.Date(2026, time.November, 3, 0, 0, 0, 0, time.UTC)

// fakeHackatime stands in for hackatime.hackclub.com's stats endpoint.
type fakeHackatime struct {
	mu        sync.Mutex
	seconds   map[string]map[string]int // Slack ID → Hackatime project → seconds
	overlaps  []fakeOverlap
	private   map[string]bool
	convicted map[string]bool
	// beforeStats, if set, runs at the start of every request.
	beforeStats func(*http.Request)
}

// fakeOverlap is time that counted toward two Hackatime projects at once. A
// total covering both counts it once, like the real thing.
type fakeOverlap struct {
	slackID, a, b string
	seconds       int
}

func (f *fakeHackatime) set(slackID, project string, seconds int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.seconds[slackID] == nil {
		f.seconds[slackID] = map[string]int{}
	}
	f.seconds[slackID][project] = seconds
}

func (f *fakeHackatime) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if f.beforeStats != nil {
		f.beforeStats(r)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	parts := strings.Split(r.URL.Path, "/") // "", api, v1, users, {slack ID}, stats
	if len(parts) != 6 || parts[5] != "stats" {
		http.NotFound(w, r)
		return
	}
	if got := r.URL.Query().Get("start_date"); got != "2026-11-03T00:00:00Z" {
		http.Error(w, "start_date is "+got, http.StatusUnprocessableEntity)
		return
	}
	slackID := parts[4]
	if f.private[slackID] {
		http.Error(w, `{"error":"private"}`, http.StatusForbidden)
		return
	}
	projects, ok := f.seconds[slackID]
	if !ok {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	var filter []string
	if v := r.URL.Query().Get("filter_by_project"); v != "" {
		filter = strings.Split(v, ",")
	}
	total := 0
	list := []map[string]any{}
	for name, seconds := range projects {
		if filter != nil && !slices.Contains(filter, name) {
			continue
		}
		total += seconds
		list = append(list, map[string]any{"name": name, "total_seconds": float64(seconds)})
	}
	included := func(name string) bool {
		_, ok := projects[name]
		return ok && (filter == nil || slices.Contains(filter, name))
	}
	for _, o := range f.overlaps {
		if o.slackID == slackID && included(o.a) && included(o.b) {
			total -= o.seconds
		}
	}
	trust := "blue"
	if f.convicted[slackID] {
		trust = "red"
	}
	json.NewEncoder(w).Encode(map[string]any{
		"data":         map[string]any{"total_seconds": float64(total), "projects": list},
		"trust_factor": map[string]any{"trust_level": trust},
	})
}

type env struct {
	pool         *pgxpool.Pool
	queries      *db.Queries
	hackatime    *fakeHackatime
	verification *fakeVerification
	router       http.Handler
	api          *api.API
}

// fakeVerification stands in for Hack Club Auth's verification check. It
// says needs_submission for anyone it hasn't been told about.
type fakeVerification struct {
	mu      sync.Mutex
	results map[string]string // identity ID → result
}

func (f *fakeVerification) set(user *db.User, result string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.results[user.HcaID] = result
}

func (f *fakeVerification) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	result, ok := f.results[r.URL.Query().Get("idv_id")]
	if !ok {
		result = "needs_submission"
	}
	json.NewEncoder(w).Encode(map[string]string{"result": result})
}

func newEnv(t *testing.T) *env {
	t.Helper()
	pool := dbtest.New(t)
	fake := &fakeHackatime{seconds: map[string]map[string]int{}, private: map[string]bool{}, convicted: map[string]bool{}}
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)

	verification := &fakeVerification{results: map[string]string{}}
	hcaServer := httptest.NewServer(verification)
	t.Cleanup(hcaServer.Close)

	a := &api.API{
		Pool:          pool,
		Queries:       db.New(pool),
		Hackatime:     hackatime.New(hackatime.Config{BaseURL: server.URL, Start: hoursStart}),
		HCA:           hca.New(hca.Config{BaseURL: hcaServer.URL}),
		StampsPerHour: 10,
	}
	r := chi.NewRouter()
	r.Mount("/api", a.Routes())
	return &env{pool: pool, queries: db.New(pool), hackatime: fake, verification: verification, router: r, api: a}
}

func (e *env) newUser(t *testing.T, slackID string) *db.User {
	t.Helper()
	var slack *string
	if slackID != "" {
		slack = &slackID
	}
	user, err := e.queries.UpsertUserFromHCA(context.Background(), db.UpsertUserFromHCAParams{
		HcaID: "ident!" + slackID + t.Name(), SlackID: slack, Email: "x@example.com", Name: "Test",
	})
	if err != nil {
		t.Fatal(err)
	}
	return &user
}

// call makes a request as user and decodes the JSON response into out, if
// out isn't nil.
func (e *env) call(t *testing.T, user *db.User, method, path string, body any, out any) int {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	if user != nil {
		req = req.WithContext(auth.WithUser(req.Context(), user))
	}
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	if out != nil && rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
			t.Fatalf("%s %s: decoding %q: %v", method, path, rec.Body.String(), err)
		}
	}
	return rec.Code
}

// review finishes a project's pending submission the way a reviewer would.
func (e *env) review(t *testing.T, projectID int64, status string) {
	t.Helper()
	reviewer := e.newUser(t, "UREVIEWER")
	tag, err := e.pool.Exec(context.Background(), `
		update submissions set status = $2::submission_status, reviewer_id = $3, reviewed_at = now(),
			approved_seconds = case when $2::submission_status = 'approved' then new_seconds end
		where project_id = $1 and status = 'pending'`, projectID, status, reviewer.ID)
	if err != nil || tag.RowsAffected() != 1 {
		t.Fatalf("reviewing project %d: %v (%d rows)", projectID, err, tag.RowsAffected())
	}
}

type project struct {
	ID                 int64        `json:"id"`
	Name               string       `json:"name"`
	HackatimeProjects  []string     `json:"hackatime_projects"`
	ApprovedSeconds    int          `json:"approved_seconds"`
	HackatimeSeconds   *int         `json:"hackatime_seconds"`
	UnsubmittedSeconds *int         `json:"unsubmitted_seconds"`
	HackatimeError     string       `json:"hackatime_error"`
	Submissions        []submission `json:"submissions"`
}

type submission struct {
	ID         int64  `json:"id"`
	Status     string `json:"status"`
	NewSeconds int    `json:"new_seconds"`
}

type apiError struct {
	Error string `json:"error"`
}

func completeProject(hackatimeProjects ...string) map[string]any {
	return map[string]any{
		"name":               "Crossroads Racer",
		"description":        "A racing game",
		"repo_url":           "https://github.com/example/racer",
		"demo_url":           "https://example.github.io/racer",
		"screenshot_url":     "https://cdn.example.com/racer.png",
		"hackatime_projects": hackatimeProjects,
	}
}

func (e *env) createProject(t *testing.T, user *db.User, body map[string]any) project {
	t.Helper()
	var p project
	if status := e.call(t, user, "POST", "/api/projects", body, &p); status != http.StatusCreated {
		t.Fatalf("creating project: got %d", status)
	}
	return p
}

func (e *env) submit(t *testing.T, user *db.User, projectID int64) (int, submission, apiError) {
	t.Helper()
	var raw json.RawMessage
	status := e.call(t, user, "POST", "/api/projects/"+itoa(projectID)+"/submissions", nil, &raw)
	var s submission
	var problem apiError
	json.Unmarshal(raw, &s)
	json.Unmarshal(raw, &problem)
	return status, s, problem
}

func itoa(id int64) string {
	return strings.TrimSpace(string(must(json.Marshal(id))))
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

func TestCreateAndListProjects(t *testing.T) {
	e := newEnv(t)
	user := e.newUser(t, "U1")

	created := e.createProject(t, user, completeProject("racer-client", "racer-server", " racer-client "))
	if !slices.Equal(created.HackatimeProjects, []string{"racer-client", "racer-server"}) {
		t.Errorf("linked %v, want them trimmed, deduped and sorted", created.HackatimeProjects)
	}

	var list []project
	if status := e.call(t, user, "GET", "/api/projects", nil, &list); status != http.StatusOK {
		t.Fatalf("listing: got %d", status)
	}
	if len(list) != 1 || list[0].ID != created.ID || len(list[0].HackatimeProjects) != 2 {
		t.Errorf("got %+v, want the one project with both links", list)
	}
}

func TestProjectValidation(t *testing.T) {
	e := newEnv(t)
	user := e.newUser(t, "U1")

	for _, tc := range []struct {
		body map[string]any
		want int
	}{
		{map[string]any{"name": "  "}, http.StatusUnprocessableEntity},
		{map[string]any{"name": "x", "repo_url": "javascript:alert(1)"}, http.StatusUnprocessableEntity},
		{map[string]any{"name": "x", "demo_url": "not a url"}, http.StatusUnprocessableEntity},
		{map[string]any{"name": "x", "hackatime_projects": []string{"a,b"}}, http.StatusUnprocessableEntity},
		{map[string]any{"name": "x", "stamps": 1000}, http.StatusBadRequest},
	} {
		var problem apiError
		if status := e.call(t, user, "POST", "/api/projects", tc.body, &problem); status != tc.want {
			t.Errorf("%v: got %d, want %d", tc.body, status, tc.want)
		}
		if problem.Error == "" {
			t.Errorf("%v: no error message", tc.body)
		}
	}
}

func TestHackatimeProjectLinksToOneProject(t *testing.T) {
	e := newEnv(t)
	user := e.newUser(t, "U1")
	e.createProject(t, user, completeProject("racer"))

	var problem apiError
	if status := e.call(t, user, "POST", "/api/projects", completeProject("racer"), &problem); status != http.StatusConflict {
		t.Fatalf("linking racer twice: got %d, want 409", status)
	}
	if !strings.Contains(problem.Error, "racer") {
		t.Errorf("error %q doesn't name the project", problem.Error)
	}
	var list []project
	e.call(t, user, "GET", "/api/projects", nil, &list)
	if len(list) != 1 {
		t.Errorf("%d projects, want 1: the failed one shouldn't be half-created", len(list))
	}

	// Someone else can have a Hackatime project with the same name.
	other := e.newUser(t, "U2")
	e.createProject(t, other, completeProject("racer"))
}

func TestSubmitNeedsAFinishedProject(t *testing.T) {
	e := newEnv(t)
	user := e.newUser(t, "U1")
	p := e.createProject(t, user, map[string]any{"name": "Half done"})

	status, _, problem := e.submit(t, user, p.ID)
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("got %d, want 422", status)
	}
	for _, want := range []string{"description", "repo link", "demo link", "screenshot", "Hackatime project"} {
		if !strings.Contains(problem.Error, want) {
			t.Errorf("error %q doesn't mention %s", problem.Error, want)
		}
	}
}

func TestSubmitAndResubmit(t *testing.T) {
	e := newEnv(t)
	user := e.newUser(t, "U1")
	e.hackatime.set("U1", "racer", 3*3600)
	e.hackatime.set("U1", "unrelated", 50*3600)
	p := e.createProject(t, user, completeProject("racer"))

	status, first, _ := e.submit(t, user, p.ID)
	if status != http.StatusCreated || first.NewSeconds != 3*3600 || first.Status != "pending" {
		t.Fatalf("first submission: got %d %+v, want a pending one with 3 hours", status, first)
	}
	if status, _, _ := e.submit(t, user, p.ID); status != http.StatusConflict {
		t.Errorf("submitting while pending: got %d, want 409", status)
	}
	if status := e.call(t, user, "PATCH", "/api/projects/"+itoa(p.ID), map[string]any{"name": "Renamed"}, nil); status != http.StatusConflict {
		t.Errorf("editing while pending: got %d, want 409", status)
	}

	e.review(t, p.ID, "approved")
	e.hackatime.set("U1", "racer", 5*3600)

	var detail project
	e.call(t, user, "GET", "/api/projects/"+itoa(p.ID), nil, &detail)
	if detail.UnsubmittedSeconds == nil || *detail.UnsubmittedSeconds != 2*3600 {
		t.Errorf("unsubmitted time is %v, want 2 hours", detail.UnsubmittedSeconds)
	}

	status, second, _ := e.submit(t, user, p.ID)
	if status != http.StatusCreated || second.NewSeconds != 2*3600 {
		t.Fatalf("second submission: got %d %+v, want only the 2 new hours", status, second)
	}
	e.review(t, p.ID, "approved")

	e.call(t, user, "GET", "/api/projects/"+itoa(p.ID), nil, &detail)
	if detail.ApprovedSeconds != 5*3600 || len(detail.Submissions) != 2 {
		t.Errorf("got %d approved seconds over %d submissions, want 5 hours over 2", detail.ApprovedSeconds, len(detail.Submissions))
	}
}

func TestRejectedTimeCanBeSubmittedAgain(t *testing.T) {
	e := newEnv(t)
	user := e.newUser(t, "U1")
	e.hackatime.set("U1", "racer", 3*3600)
	p := e.createProject(t, user, completeProject("racer"))

	e.submit(t, user, p.ID)
	e.review(t, p.ID, "rejected")
	status, again, _ := e.submit(t, user, p.ID)
	if status != http.StatusCreated || again.NewSeconds != 3*3600 {
		t.Errorf("got %d %+v, want all 3 hours again", status, again)
	}
}

func TestNoNewTime(t *testing.T) {
	e := newEnv(t)
	user := e.newUser(t, "U1")
	e.hackatime.set("U1", "racer", 3*3600)
	p := e.createProject(t, user, completeProject("racer"))
	e.submit(t, user, p.ID)
	e.review(t, p.ID, "approved")

	if status, _, _ := e.submit(t, user, p.ID); status != http.StatusUnprocessableEntity {
		t.Errorf("submitting with no new time: got %d, want 422", status)
	}
}

func TestPaidHackatimeProjectsStayLinked(t *testing.T) {
	e := newEnv(t)
	user := e.newUser(t, "U1")
	e.hackatime.set("U1", "racer", 3*3600)
	p := e.createProject(t, user, completeProject("racer"))
	e.submit(t, user, p.ID)
	e.review(t, p.ID, "approved")

	var problem apiError
	if status := e.call(t, user, "PATCH", "/api/projects/"+itoa(p.ID), map[string]any{"hackatime_projects": []string{}}, &problem); status != http.StatusConflict {
		t.Errorf("unlinking a paid Hackatime project: got %d, want 409", status)
	}

	var updated project
	body := map[string]any{"hackatime_projects": []string{"racer", "racer-assets"}}
	if status := e.call(t, user, "PATCH", "/api/projects/"+itoa(p.ID), body, &updated); status != http.StatusOK {
		t.Fatalf("adding a link: got %d", status)
	}
	if !slices.Equal(updated.HackatimeProjects, []string{"racer", "racer-assets"}) {
		t.Errorf("links are %v", updated.HackatimeProjects)
	}
	// An unpaid one can come off again.
	if status := e.call(t, user, "PATCH", "/api/projects/"+itoa(p.ID), map[string]any{"hackatime_projects": []string{"racer"}}, nil); status != http.StatusOK {
		t.Errorf("removing an unpaid link: got %d, want 200", status)
	}
}

func TestHackatimeProblems(t *testing.T) {
	e := newEnv(t)

	convicted := e.newUser(t, "UBAD")
	e.hackatime.set("UBAD", "racer", 3*3600)
	e.hackatime.convicted["UBAD"] = true
	p := e.createProject(t, convicted, completeProject("racer"))
	if status, _, _ := e.submit(t, convicted, p.ID); status != http.StatusForbidden {
		t.Errorf("convicted: got %d, want 403", status)
	}

	private := e.newUser(t, "UPRIVATE")
	e.hackatime.private["UPRIVATE"] = true
	p = e.createProject(t, private, completeProject("racer"))
	status, _, problem := e.submit(t, private, p.ID)
	if status != http.StatusUnprocessableEntity || !strings.Contains(problem.Error, "private") {
		t.Errorf("private stats: got %d %q", status, problem.Error)
	}

	noAccount := e.newUser(t, "UNOBODY")
	p = e.createProject(t, noAccount, completeProject("racer"))
	if status, _, _ := e.submit(t, noAccount, p.ID); status != http.StatusUnprocessableEntity {
		t.Errorf("no Hackatime account: got %d, want 422", status)
	}

	noSlack := e.newUser(t, "")
	p = e.createProject(t, noSlack, completeProject("racer"))
	status, _, problem = e.submit(t, noSlack, p.ID)
	if status != http.StatusUnprocessableEntity || !strings.Contains(problem.Error, "Slack") {
		t.Errorf("no Slack: got %d %q", status, problem.Error)
	}
}

func TestOtherPeoplesProjects(t *testing.T) {
	e := newEnv(t)
	owner := e.newUser(t, "U1")
	e.hackatime.set("U1", "racer", 3*3600)
	p := e.createProject(t, owner, completeProject("racer"))
	stranger := e.newUser(t, "U2")
	e.hackatime.set("U2", "racer", 3*3600)

	path := "/api/projects/" + itoa(p.ID)
	for _, req := range []struct{ method, path string }{
		{"GET", path}, {"PATCH", path}, {"DELETE", path}, {"POST", path + "/submissions"},
	} {
		var body any
		if req.method == "PATCH" {
			body = map[string]any{"name": "mine now"}
		}
		if status := e.call(t, stranger, req.method, req.path, body, nil); status != http.StatusNotFound {
			t.Errorf("%s %s as someone else: got %d, want 404", req.method, req.path, status)
		}
	}
	if status := e.call(t, nil, "GET", "/api/projects", nil, nil); status != http.StatusUnauthorized {
		t.Errorf("signed out: got %d, want 401", status)
	}
}

func TestDeleteProject(t *testing.T) {
	e := newEnv(t)
	user := e.newUser(t, "U1")
	e.hackatime.set("U1", "racer", 3*3600)

	draft := e.createProject(t, user, map[string]any{"name": "Draft", "hackatime_projects": []string{"draft"}})
	if status := e.call(t, user, "DELETE", "/api/projects/"+itoa(draft.ID), nil, nil); status != http.StatusNoContent {
		t.Errorf("deleting a draft: got %d, want 204", status)
	}

	submitted := e.createProject(t, user, completeProject("racer"))
	e.submit(t, user, submitted.ID)
	if status := e.call(t, user, "DELETE", "/api/projects/"+itoa(submitted.ID), nil, nil); status != http.StatusConflict {
		t.Errorf("deleting a submitted project: got %d, want 409", status)
	}

	// The draft's Hackatime project is free to link again.
	e.createProject(t, user, completeProject("draft"))
}

func TestListHackatimeProjects(t *testing.T) {
	e := newEnv(t)
	user := e.newUser(t, "U1")
	e.hackatime.set("U1", "racer", 3*3600)
	e.hackatime.set("U1", "dotfiles", 600)
	p := e.createProject(t, user, completeProject("racer"))

	var list []struct {
		Name      string `json:"name"`
		Seconds   int    `json:"seconds"`
		ProjectID *int64 `json:"project_id"`
	}
	if status := e.call(t, user, "GET", "/api/hackatime/projects", nil, &list); status != http.StatusOK {
		t.Fatalf("got %d", status)
	}
	if len(list) != 2 {
		t.Fatalf("got %+v, want 2 projects", list)
	}
	for _, h := range list {
		switch h.Name {
		case "racer":
			if h.ProjectID == nil || *h.ProjectID != p.ID || h.Seconds != 3*3600 {
				t.Errorf("racer: %+v", h)
			}
		case "dotfiles":
			if h.ProjectID != nil {
				t.Errorf("dotfiles shouldn't be linked: %+v", h)
			}
		}
	}
}
