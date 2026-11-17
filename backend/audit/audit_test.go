package audit_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/scherepi/crossroads-backend/audit"
	"github.com/scherepi/crossroads-backend/auth"
	"github.com/scherepi/crossroads-backend/db"
	"github.com/scherepi/crossroads-backend/db/dbtest"
)

type row struct {
	ActorID *int64
	UserID  *int64
	Details map[string]any
}

func TestRecordsWriteRequests(t *testing.T) {
	pool := dbtest.New(t)
	queries := db.New(pool)
	ctx := context.Background()
	admin, err := queries.UpsertUserFromHCA(ctx, db.UpsertUserFromHCAParams{HcaID: "admin", Email: "a@example.com", Name: "Admin"})
	if err != nil {
		t.Fatal(err)
	}
	target, err := queries.UpsertUserFromHCA(ctx, db.UpsertUserFromHCAParams{HcaID: "target", Email: "t@example.com", Name: "Target"})
	if err != nil {
		t.Fatal(err)
	}

	var seen string
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Group(func(r chi.Router) {
		r.Use((&audit.Requests{Queries: queries}).Record)
		r.Route("/api", func(r chi.Router) {
			r.Get("/users/{userID}", func(w http.ResponseWriter, r *http.Request) {})
			r.Post("/users/{userID}/stamps", func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				seen = string(body)
				w.WriteHeader(http.StatusCreated)
			})
			r.Post("/uploads", func(w http.ResponseWriter, r *http.Request) { io.Copy(io.Discard, r.Body) })
			r.Delete("/boom", func(w http.ResponseWriter, r *http.Request) { panic("boom") })
		})
	})
	send := func(method, path, body string) {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req = req.WithContext(auth.WithUser(req.Context(), &admin))
		req.Header.Set("CF-Connecting-IP", "203.0.113.9")
		r.ServeHTTP(httptest.NewRecorder(), req)
	}
	send("GET", "/api/users/1", "")
	send("POST", "/api/users/"+strconv.FormatInt(target.ID, 10)+"/stamps", `{"amount": 5}`)
	upload := strings.Repeat("x", audit.MaxBody+10)
	send("POST", "/api/uploads", upload)
	send("DELETE", "/api/boom", "")

	if seen != `{"amount": 5}` {
		t.Errorf("the handler read %q, want the whole body", seen)
	}
	got := readRows(t, pool)
	if len(got) != 3 {
		t.Fatalf("got %d rows, want the 3 writes and not the GET: %+v", len(got), got)
	}
	stamps, uploads, boom := got[0], got[1], got[2]
	if *stamps.ActorID != admin.ID || stamps.UserID == nil || *stamps.UserID != target.ID ||
		stamps.Details["route"] != "/api/users/{userID}/stamps" || stamps.Details["status"] != float64(201) ||
		stamps.Details["ip"] != "203.0.113.9" || stamps.Details["body"].(map[string]any)["amount"] != float64(5) {
		t.Errorf("the stamps request is %+v", stamps)
	}
	if uploads.Details["body"] != nil || uploads.Details["body_bytes"] != float64(len(upload)) {
		t.Errorf("the upload is %+v, want only its size", uploads)
	}
	if boom.Details["status"] != float64(500) {
		t.Errorf("the panic is %+v, want a 500", boom)
	}
}

// readRows returns the request rows, oldest first.
func readRows(t *testing.T, pool *pgxpool.Pool) []row {
	t.Helper()
	rows, err := pool.Query(context.Background(), "select actor_id, user_id, details from audit_log where kind = 'request' order by id")
	if err != nil {
		t.Fatal(err)
	}
	var got []row
	for rows.Next() {
		var r row
		var details []byte
		if err := rows.Scan(&r.ActorID, &r.UserID, &details); err != nil {
			t.Fatal(err)
		}
		json.Unmarshal(details, &r.Details)
		got = append(got, r)
	}
	return got
}

func TestSkipsWhatAnyoneCouldSend(t *testing.T) {
	pool := dbtest.New(t)
	queries := db.New(pool)
	user, err := queries.UpsertUserFromHCA(context.Background(), db.UpsertUserFromHCAParams{HcaID: "u", Email: "u@example.com", Name: "U"})
	if err != nil {
		t.Fatal(err)
	}

	r := chi.NewRouter()
	r.Group(func(r chi.Router) {
		r.Use((&audit.Requests{Queries: queries}).Record)
		r.Route("/api", func(r chi.Router) {
			r.Post("/sidekick", func(w http.ResponseWriter, r *http.Request) { audit.Vouch(r) })
			r.Post("/logout", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
			r.With(auth.RequireUser).Post("/projects/{projectID}", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
			})
		})
	})
	send := func(user *db.User, path, body, userAgent string) {
		req := httptest.NewRequest("POST", path, strings.NewReader(body))
		if user != nil {
			req = req.WithContext(auth.WithUser(req.Context(), user))
		}
		req.Header.Set("User-Agent", userAgent)
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(httptest.NewRecorder(), req)
	}
	junk := `{"junk": "` + strings.Repeat("x", 60<<10) + `"}`
	send(nil, "/api/projects/1", junk, "") // 401
	send(nil, "/api/nowhere", junk, "")    // 404
	send(nil, "/api/logout", junk, "")     // 204, but nobody's signed in
	send(nil, "/api/sidekick", `{"action": "HEALTH_CHECK"}`, "")
	long := strings.Repeat("9", 2000)
	send(&user, "/api/projects/"+long, junk, "x"+strings.Repeat("é", 1000))

	got := readRows(t, pool)
	if len(got) != 2 {
		t.Fatalf("got %d rows, want Sidekick's and the signed-in one: %+v", len(got), got)
	}
	sidekick, rejected := got[0], got[1]
	if sidekick.ActorID != nil || sidekick.Details["body"].(map[string]any)["action"] != "HEALTH_CHECK" {
		t.Errorf("Sidekick's request is %+v", sidekick)
	}
	if *rejected.ActorID != user.ID || rejected.Details["status"] != float64(400) ||
		rejected.Details["body"] != nil || rejected.Details["body_bytes"] != float64(len(junk)) {
		t.Errorf("the rejected request is %+v, want its body by size only", rejected)
	}
	// Cut at the end of a character: the 256th é would go past 512 bytes.
	if ua := rejected.Details["user_agent"]; ua != "x"+strings.Repeat("é", 255) {
		t.Errorf("the user agent is %q, want it cut to 511 bytes", ua)
	}
	if path := rejected.Details["path"].(string); len(path) != 512 {
		t.Errorf("the path is %d bytes, want 512", len(path))
	}
	if id := rejected.Details["params"].(map[string]any)["projectID"].(string); len(id) != 512 {
		t.Errorf("projectID is %d bytes, want 512", len(id))
	}
}

func TestBudget(t *testing.T) {
	pool := dbtest.New(t)
	queries := db.New(pool)
	user, err := queries.UpsertUserFromHCA(context.Background(), db.UpsertUserFromHCAParams{HcaID: "u", Email: "u@example.com", Name: "U"})
	if err != nil {
		t.Fatal(err)
	}
	r := chi.NewRouter()
	r.With((&audit.Requests{Queries: queries, Budget: 4000}).Record).Post("/api/stamps", func(w http.ResponseWriter, r *http.Request) {})
	body := `{"note": "` + strings.Repeat("x", 3000) + `"}`
	for range 20 {
		req := httptest.NewRequest("POST", "/api/stamps", strings.NewReader(body))
		r.ServeHTTP(httptest.NewRecorder(), req.WithContext(auth.WithUser(req.Context(), &user)))
	}

	got := readRows(t, pool)
	if len(got) < 2 || len(got) >= 20 {
		t.Fatalf("got %d rows, want the first two and not all 20", len(got))
	}
	if got[0].Details["body"] == nil {
		t.Errorf("the first request is %+v, want its body", got[0])
	}
	if got[1].Details["body"] != nil || got[1].Details["body_bytes"] != float64(len(body)) {
		t.Errorf("the second request is %+v, want its body by size only", got[1])
	}
}
