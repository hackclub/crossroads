package api_test

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/scherepi/crossroads-backend/db"
	"github.com/scherepi/crossroads-backend/devgithub"
	"github.com/scherepi/crossroads-backend/github"
	"github.com/scherepi/crossroads-backend/linkcheck"
	"github.com/scherepi/crossroads-backend/unified"
)

// withOutsideChecks gives the checks the dev-mode GitHub, link checker and
// Unified YSWS DB, which act out cases by the words in a link (see devgithub
// and linkcheck.Dev).
func (e *env) withOutsideChecks(t *testing.T) {
	t.Helper()
	fake := &devgithub.Fake{Created: time.Now().AddDate(0, 0, -30), HoursStart: time.Now().AddDate(0, 0, -10)}
	server := httptest.NewServer(fake.Routes())
	t.Cleanup(server.Close)
	e.api.GitHub = github.New(github.Config{BaseURL: server.URL})
	e.api.Links = linkcheck.Dev{}
	e.api.Unified = unified.Dev{}
	e.api.HoursStart = time.Now().AddDate(0, 0, -10)
}

// shipLinks ships 3 hours of a project with these links.
func (e *env) shipLinks(t *testing.T, slackID, repo, demo string) (*db.User, int64) {
	t.Helper()
	user := e.newUser(t, slackID)
	body := projectNamed("p-"+slackID, "ht-"+slackID)
	body["repo_url"], body["demo_url"] = repo, demo
	p := e.createProject(t, user, body)
	return user, e.submitHours(t, user, slackID, p, "ht-"+slackID, 3*3600)
}

type checksDetail struct {
	Checks []struct {
		ID      string `json:"id"`
		Status  string `json:"status"`
		Summary string `json:"summary"`
		Fraud   bool   `json:"fraud"`
	} `json:"checks"`
	GitHub *struct {
		TotalCommits int `json:"total_commits"`
		Commits      []struct {
			SHA string `json:"sha"`
		} `json:"commits"`
		Authors []struct {
			Author  string `json:"author"`
			Commits int    `json:"commits"`
		} `json:"authors"`
		Additions    int `json:"additions"`
		LinesCounted int `json:"lines_counted"`
	} `json:"github"`
	UnifiedRecords []unified.Record `json:"unified_records"`
	Days           []struct {
		Day     string `json:"day"`
		Seconds int    `json:"seconds"`
		Commits int    `json:"commits"`
	} `json:"days"`
	TelescreenURL string `json:"telescreen_url"`
}

func (e *env) checks(t *testing.T, reviewer *db.User, id int64) (checksDetail, map[string]string) {
	t.Helper()
	var d checksDetail
	e.mustCall(t, reviewer, http.StatusOK, "GET", "/api/review/submissions/"+itoa(id), nil, &d)
	status := map[string]string{}
	for _, c := range d.Checks {
		status[c.ID] = c.Status
	}
	return d, status
}

func TestGitHubStatsAndLinkChecks(t *testing.T) {
	e := newEnv(t)
	e.withOutsideChecks(t)
	reviewer := e.newReviewer(t)

	// A public repo with commits, a README and a demo that opens.
	_, ship := e.shipLinks(t, "UGOOD", "https://github.com/dev-ana/racer", "https://dev-ana.example.com/racer")
	d, status := e.checks(t, reviewer, ship)
	for _, id := range []string{"repo_works", "demo_works", "readme", "new_commits", "unified_double_dip"} {
		if status[id] != "pass" {
			t.Errorf("%s is %q, want pass", id, status[id])
		}
	}
	if d.GitHub == nil || len(d.GitHub.Commits) == 0 || d.GitHub.TotalCommits < len(d.GitHub.Commits) {
		t.Fatalf("GitHub stats: %+v", d.GitHub)
	}
	if d.GitHub.LinesCounted != len(d.GitHub.Commits) || d.GitHub.Additions == 0 {
		t.Errorf("lines: %d added over %d of %d commits", d.GitHub.Additions, d.GitHub.LinesCounted, len(d.GitHub.Commits))
	}
	if len(d.GitHub.Authors) != 1 || d.GitHub.Authors[0].Author != "dev-ana" {
		t.Errorf("authors: %+v", d.GitHub.Authors)
	}
	commitDays := 0
	for _, day := range d.Days {
		commitDays += day.Commits
	}
	if commitDays != len(d.GitHub.Commits) {
		t.Errorf("the daily totals have %d commits, GitHub %d", commitDays, len(d.GitHub.Commits))
	}

	cases := []struct {
		name, repo, demo string
		want             map[string]string
	}{
		{"private repo", "https://github.com/dev-bo/private-racer", "https://dev-bo.example.com", map[string]string{"repo_works": "fail", "readme": "", "new_commits": ""}},
		{"no commits in the window", "https://github.com/dev-cy/stale-racer", "https://dev-cy.example.com", map[string]string{"repo_works": "pass", "new_commits": "fail"}},
		{"template README", "https://github.com/dev-di/template-racer", "https://dev-di.example.com", map[string]string{"readme": "fail"}},
		{"no README", "https://github.com/dev-ed/no-readme-racer", "https://dev-ed.example.com", map[string]string{"readme": "fail"}},
		{"broken demo", "https://github.com/dev-fi/racer", "https://dev-fi.example.com/broken", map[string]string{"demo_works": "fail"}},
		{"notebook demo", "https://github.com/dev-gu/racer", "https://colab.research.google.com/drive/abc", map[string]string{"demo_works": "fail"}},
	}
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, ship := e.shipLinks(t, "UCASE"+itoa(int64(i)), c.repo, c.demo)
			_, status := e.checks(t, reviewer, ship)
			for id, want := range c.want {
				if status[id] != want {
					t.Errorf("%s is %q, want %q", id, status[id], want)
				}
			}
		})
	}
}

func TestBrokenLinksDontSendToFraud(t *testing.T) {
	e := newEnv(t)
	e.withOutsideChecks(t)
	reviewer := e.newStageReviewer(t, "UREV", true, false, false)
	_, ship := e.shipLinks(t, "UBROKEN", "https://github.com/dev-io/private-racer", "https://dev-io.example.com/broken")
	if status, _ := e.decide(t, reviewer, ship, map[string]any{"action": "approve", "approved_seconds": 3600}); status != http.StatusOK {
		t.Fatalf("approving: %d", status)
	}
	if d := e.detail(t, reviewer, ship); d.Submission.Status != "awaiting_hq" {
		t.Errorf("a broken link is for the reviewer to judge, but the ship went to %s (%v)", d.Submission.Status, d.Submission.FraudReasons)
	}
}

func TestDoubleDippingGoesToFraud(t *testing.T) {
	e := newEnv(t)
	e.withOutsideChecks(t)
	reviewer := e.newStageReviewer(t, "UREV", true, false, false)
	_, ship := e.shipLinks(t, "UDIP", "https://github.com/dev-jo/double-dip", "https://dev-jo.example.com")
	d, status := e.checks(t, reviewer, ship)
	if status["unified_double_dip"] != "fail" || len(d.UnifiedRecords) != 1 {
		t.Fatalf("double dip: %q, records %+v", status["unified_double_dip"], d.UnifiedRecords)
	}
	if code, _ := e.decide(t, reviewer, ship, map[string]any{"action": "approve", "approved_seconds": 3600}); code != http.StatusOK {
		t.Fatalf("approving: %d", code)
	}
	after := e.detail(t, reviewer, ship)
	if after.Submission.Status != "fraud_check" || !slices.Contains(after.Submission.FraudReasons, "unified_double_dip") {
		t.Errorf("went to %s for %v", after.Submission.Status, after.Submission.FraudReasons)
	}
}

func TestFullNameAndUnifiedReadiness(t *testing.T) {
	e := newEnv(t)
	reviewer := e.newReviewer(t)
	user, ship := e.shipFor(t, "UNAME", 2)

	// Test users only have a display name.
	_, status := e.checks(t, reviewer, ship)
	if status["full_name"] != "fail" || status["unified_ready"] != "fail" {
		t.Errorf("no first or last name: full_name %q, unified_ready %q", status["full_name"], status["unified_ready"])
	}

	if _, err := e.pool.Exec(t.Context(), `update users set first_name = 'Ada', last_name = 'Lovelace', birthday = '2009-04-01',
		address = '{"line_1": "1 Main St", "city": "Shelburne", "postal_code": "05482", "country": "US"}' where id = $1`, user.ID); err != nil {
		t.Fatal(err)
	}
	_, status = e.checks(t, reviewer, ship)
	if status["full_name"] != "pass" || status["unified_ready"] != "pass" {
		t.Errorf("a complete profile: full_name %q, unified_ready %q", status["full_name"], status["unified_ready"])
	}
}

func TestTelescreenLink(t *testing.T) {
	e := newEnv(t)
	reviewer := e.newReviewer(t)
	_, ship := e.shipFor(t, "UTELE", 2)
	if d, _ := e.checks(t, reviewer, ship); d.TelescreenURL != "" {
		t.Errorf("a Telescreen link without TELESCREEN_PROFILE_URL: %q", d.TelescreenURL)
	}
	e.api.TelescreenProfileURL = "https://telescreen.example/people/{slack_id}"
	if d, _ := e.checks(t, reviewer, ship); d.TelescreenURL != "https://telescreen.example/people/UTELE" {
		t.Errorf("Telescreen link %q", d.TelescreenURL)
	}
}
