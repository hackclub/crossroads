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
	rows, err := pool.Query(ctx, "select actor_id, user_id, details from audit_log where kind = 'request' order by id")
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
