package auth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/scherepi/crossroads-backend/auth"
	"github.com/scherepi/crossroads-backend/db"
	"github.com/scherepi/crossroads-backend/db/dbtest"
	"github.com/scherepi/crossroads-backend/hca"
)

const (
	clientID     = "test-client"
	clientSecret = "test-secret"
	goodCode     = "good-code"
	accessToken  = "idntk.test"
)

// fakeHCA stands in for auth.hackclub.com. identity is what /api/v1/me returns.
func fakeHCA(t *testing.T, identity *map[string]any) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /oauth/token", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if r.Form.Get("code") != goodCode || r.Form.Get("client_id") != clientID ||
			r.Form.Get("client_secret") != clientSecret || r.Form.Get("grant_type") != "authorization_code" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":"invalid_grant"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": accessToken, "token_type": "Bearer", "expires_in": 15778800,
		})
	})
	mux.HandleFunc("GET /api/v1/me", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+accessToken {
			http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"identity": *identity})
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

type app struct {
	url    string
	client *http.Client
	pool   *pgxpool.Pool
}

func newApp(t *testing.T, identity *map[string]any) *app {
	t.Helper()
	pool := dbtest.New(t)
	hcaServer := fakeHCA(t, identity)

	a := &auth.Auth{Queries: db.New(pool)}
	r := chi.NewRouter()
	r.Use(a.LoadSession)
	r.Mount("/auth", a.Routes())
	r.With(auth.RequireUser).Get("/api/me", auth.Me)
	r.With(auth.RequireRole(db.UserRoleAdmin)).Get("/api/admin", func(w http.ResponseWriter, r *http.Request) {})
	server := httptest.NewServer(r)
	t.Cleanup(server.Close)

	a.HCA = hca.New(hca.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  server.URL + "/auth/hcacallback",
		BaseURL:      hcaServer.URL,
	})

	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar: jar,
		// Stop at each redirect so the tests can look at it.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	return &app{url: server.URL, client: client, pool: pool}
}

func (a *app) do(t *testing.T, method, path string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, a.url+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := a.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

// startLogin hits /auth/login and returns the state it sent to Hack Club Auth.
func (a *app) startLogin(t *testing.T, next string) string {
	t.Helper()
	resp := a.do(t, "GET", "/auth/login?next="+url.QueryEscape(next))
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("login: got %s, want a redirect", resp.Status)
	}
	location, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	return location.Query().Get("state")
}

// signIn goes through the whole sign-in flow and returns where it landed.
func (a *app) signIn(t *testing.T, next string) string {
	t.Helper()
	state := a.startLogin(t, next)
	resp := a.do(t, "GET", "/auth/hcacallback?code="+goodCode+"&state="+url.QueryEscape(state))
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("callback: got %s, want a redirect", resp.Status)
	}
	return resp.Header.Get("Location")
}

func (a *app) me(t *testing.T) (int, map[string]any) {
	t.Helper()
	resp := a.do(t, "GET", "/api/me")
	var body map[string]any
	if resp.StatusCode == http.StatusOK {
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
	}
	return resp.StatusCode, body
}

func heidi() map[string]any {
	return map[string]any{
		"id":                  "ident!9zfYqvr",
		"first_name":          "Heidi",
		"last_name":           "Trashworth",
		"primary_email":       "heidi@example.com",
		"slack_id":            "U00001533",
		"verification_status": "needs_submission",
		"ysws_eligible":       true,
		"birthday":            "2009-04-01",
		"phone_number":        "+18028675309",
		"addresses": []map[string]any{
			{"first_name": "Old", "last_name": "Address", "line_1": "1 Old Rd", "city": "Burlington", "country": "US", "primary": false},
			{"first_name": "Heidi", "last_name": "Trashworth", "line_1": "15 Falls Rd", "city": "Shelburne", "state": "VT", "postal_code": "05482", "country": "US", "primary": true},
		},
	}
}

func TestLoginRedirectsToHCA(t *testing.T) {
	identity := heidi()
	app := newApp(t, &identity)

	resp := app.do(t, "GET", "/auth/login")
	location, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if location.Path != "/oauth/authorize" {
		t.Errorf("redirected to %s, want /oauth/authorize", location.Path)
	}
	query := location.Query()
	for param, want := range map[string]string{
		"client_id":     clientID,
		"redirect_uri":  app.url + "/auth/hcacallback",
		"response_type": "code",
		"scope":         "email name slack_id verification_status basic_info address",
	} {
		if got := query.Get(param); got != want {
			t.Errorf("%s is %q, want %q", param, got, want)
		}
	}
	if query.Get("state") == "" {
		t.Error("no state")
	}
}

func TestSignInAndOut(t *testing.T) {
	identity := heidi()
	app := newApp(t, &identity)

	if status, _ := app.me(t); status != http.StatusUnauthorized {
		t.Fatalf("before signing in, /api/me is %d, want 401", status)
	}

	if landed := app.signIn(t, "/shop?tab=stickers"); landed != "/shop?tab=stickers" {
		t.Errorf("landed on %q, want /shop?tab=stickers", landed)
	}
	status, me := app.me(t)
	if status != http.StatusOK {
		t.Fatalf("after signing in, /api/me is %d, want 200", status)
	}
	for field, want := range map[string]any{
		"name":                "Heidi Trashworth",
		"email":               "heidi@example.com",
		"slack_id":            "U00001533",
		"role":                "participant",
		"stamps":              float64(0),
		"verification_status": "needs_submission",
		"ysws_eligible":       true,
	} {
		if me[field] != want {
			t.Errorf("%s is %v, want %v", field, me[field], want)
		}
	}

	if resp := app.do(t, "POST", "/auth/logout"); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("logout: got %s", resp.Status)
	}
	if status, _ := app.me(t); status != http.StatusUnauthorized {
		t.Errorf("after signing out, /api/me is %d, want 401", status)
	}
	var sessions int
	app.pool.QueryRow(context.Background(), "select count(*) from sessions").Scan(&sessions)
	if sessions != 0 {
		t.Errorf("%d sessions left after signing out, want 0", sessions)
	}
}

func TestSignInSavesTheHCAProfile(t *testing.T) {
	identity := heidi()
	app := newApp(t, &identity)
	app.signIn(t, "/")

	var first, last, birthday, phone, line1, city string
	err := app.pool.QueryRow(context.Background(), `select first_name, last_name, birthday::text, phone_number,
		address->>'line_1', address->>'city' from users`).Scan(&first, &last, &birthday, &phone, &line1, &city)
	if err != nil {
		t.Fatal(err)
	}
	if first != "Heidi" || last != "Trashworth" || birthday != "2009-04-01" || phone != "+18028675309" {
		t.Errorf("saved %q %q born %q, phone %q", first, last, birthday, phone)
	}
	if line1 != "15 Falls Rd" || city != "Shelburne" {
		t.Errorf("saved address %q, %q; want the primary one", line1, city)
	}
}

func TestSigningInAgainUpdatesTheUser(t *testing.T) {
	identity := heidi()
	app := newApp(t, &identity)
	app.signIn(t, "/")

	identity["verification_status"] = "verified"
	identity["first_name"] = "Hedy"
	app.signIn(t, "/")

	_, me := app.me(t)
	if me["verification_status"] != "verified" || me["name"] != "Hedy Trashworth" {
		t.Errorf("got %v, want the updated name and verification status", me)
	}
	var users int
	app.pool.QueryRow(context.Background(), "select count(*) from users").Scan(&users)
	if users != 1 {
		t.Errorf("%d users after signing in twice, want 1", users)
	}
}

func TestCallbackRejectsWrongState(t *testing.T) {
	identity := heidi()
	app := newApp(t, &identity)
	app.startLogin(t, "/")

	resp := app.do(t, "GET", "/auth/hcacallback?code="+goodCode+"&state=forged")
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("got %s, want 400", resp.Status)
	}
	if status, _ := app.me(t); status != http.StatusUnauthorized {
		t.Error("signed in with a forged state")
	}
}

func TestCallbackWithoutLoginCookie(t *testing.T) {
	identity := heidi()
	app := newApp(t, &identity)

	resp := app.do(t, "GET", "/auth/hcacallback?code="+goodCode+"&state=anything")
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("got %s, want 400", resp.Status)
	}
}

func TestCallbackWhenHCARejectsCode(t *testing.T) {
	identity := heidi()
	app := newApp(t, &identity)
	state := app.startLogin(t, "/")

	resp := app.do(t, "GET", "/auth/hcacallback?code=bad-code&state="+url.QueryEscape(state))
	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("got %s, want 502", resp.Status)
	}
	if status, _ := app.me(t); status != http.StatusUnauthorized {
		t.Error("signed in with a code Hack Club Auth rejected")
	}
}

func TestLoginWontRedirectOffSite(t *testing.T) {
	identity := heidi()
	app := newApp(t, &identity)
	for _, next := range []string{"https://evil.example", "//evil.example", "/\\evil.example", "javascript:alert(1)"} {
		if landed := app.signIn(t, next); landed != "/" {
			t.Errorf("next=%q landed on %q, want /", next, landed)
		}
	}
}

func TestExpiredSession(t *testing.T) {
	identity := heidi()
	app := newApp(t, &identity)
	app.signIn(t, "/")

	if _, err := app.pool.Exec(context.Background(), "update sessions set expires_at = now()"); err != nil {
		t.Fatal(err)
	}
	if status, _ := app.me(t); status != http.StatusUnauthorized {
		t.Errorf("with an expired session, /api/me is %d, want 401", status)
	}
}

func TestRequireRole(t *testing.T) {
	identity := heidi()
	app := newApp(t, &identity)
	app.signIn(t, "/")

	if resp := app.do(t, "GET", "/api/admin"); resp.StatusCode != http.StatusForbidden {
		t.Errorf("participant got %s on an admin route, want 403", resp.Status)
	}
	if _, err := app.pool.Exec(context.Background(), "update users set role = 'admin'"); err != nil {
		t.Fatal(err)
	}
	if resp := app.do(t, "GET", "/api/admin"); resp.StatusCode != http.StatusOK {
		t.Errorf("admin got %s on an admin route, want 200", resp.Status)
	}
}

func TestSessionCookieIsHashedInDatabase(t *testing.T) {
	identity := heidi()
	app := newApp(t, &identity)
	app.signIn(t, "/")

	appURL, _ := url.Parse(app.url)
	var token string
	for _, cookie := range app.client.Jar.Cookies(appURL) {
		if cookie.Name == "crossroads_session" {
			token = cookie.Value
		}
	}
	if token == "" {
		t.Fatal("no session cookie")
	}
	var stored []byte
	app.pool.QueryRow(context.Background(), "select token_hash from sessions").Scan(&stored)
	if strings.Contains(string(stored), token) {
		t.Error("the database has the raw session token")
	}
}

func TestSigningInClaimsARoleInvite(t *testing.T) {
	identity := heidi()
	app := newApp(t, &identity)
	ctx := context.Background()
	if _, err := app.pool.Exec(ctx, "insert into role_invites (email, role) values ('heidi@example.com', 'admin')"); err != nil {
		t.Fatal(err)
	}
	// Their Hack Club Auth email's case doesn't have to match.
	identity["primary_email"] = "Heidi@Example.com"
	app.signIn(t, "/")

	if _, me := app.me(t); me["role"] != "admin" {
		t.Errorf("role is %v, want admin", me["role"])
	}
	if resp := app.do(t, "GET", "/api/admin"); resp.StatusCode != http.StatusOK {
		t.Errorf("/api/admin is %s, want 200", resp.Status)
	}
	var invites int
	app.pool.QueryRow(ctx, "select count(*) from role_invites").Scan(&invites)
	if invites != 0 {
		t.Errorf("%d invites left, want the used one gone", invites)
	}

	// The invite is used up: a later demotion sticks.
	if _, err := app.pool.Exec(ctx, "update users set role = 'participant'"); err != nil {
		t.Fatal(err)
	}
	app.signIn(t, "/")
	if _, me := app.me(t); me["role"] != "participant" {
		t.Errorf("role is %v after signing in again, want participant", me["role"])
	}
}

// A reviewer by email starts on first reviews, so they can review straight away.
func TestSigningInAsAnInvitedReviewer(t *testing.T) {
	identity := heidi()
	app := newApp(t, &identity)
	ctx := context.Background()
	if _, err := app.pool.Exec(ctx, "insert into role_invites (email, role) values ('heidi@example.com', 'reviewer')"); err != nil {
		t.Fatal(err)
	}
	app.signIn(t, "/")
	_, me := app.me(t)
	if me["role"] != "reviewer" || me["can_review"] != true || me["can_fraud_review"] != false || me["can_hq_review"] != false {
		t.Errorf("signed in as %v, can review %v, fraud %v, hq %v; want a reviewer on first reviews only",
			me["role"], me["can_review"], me["can_fraud_review"], me["can_hq_review"])
	}
}
