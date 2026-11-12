// Package audit records every request that can change something in the audit
// log, for superadmins to read. Migration 24 describes the log; triggers there
// add the other kinds of event.
package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/scherepi/crossroads-backend/auth"
	"github.com/scherepi/crossroads-backend/db"
)

// MaxBody is the most of a request body kept. Bigger bodies, and ones that
// aren't JSON (uploads), are recorded by size only.
const MaxBody = 64 << 10

type Requests struct {
	Queries *db.Queries
}

// Record is middleware that adds a 'request' row to the audit log for every
// request but GET, HEAD and OPTIONS: who was signed in, the route and its
// parameters, the JSON body, the status it got, and where it came from.
// Headers aren't kept, so neither are the secrets Sidekick and the Slack bot
// send in them. It goes after auth.LoadSession, and records once the handler
// is done. Failing to record is logged, not shown to anyone.
func (q *Requests) Record(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		details := map[string]any{
			"method":     r.Method,
			"path":       r.URL.Path,
			"ip":         auth.ClientIP(r),
			"user_agent": r.UserAgent(),
		}
		readBody(r, details)
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		defer func() {
			// A panic still gets recorded, as the 500 Recoverer answers it with.
			p := recover()
			status := ww.Status()
			switch {
			case p != nil:
				status = http.StatusInternalServerError
			case status == 0:
				status = http.StatusOK
			}
			details["status"] = status
			details["ms"] = time.Since(start).Milliseconds()
			q.record(r, details)
			if p != nil {
				panic(p)
			}
		}()
		next.ServeHTTP(ww, r)
	})
}

// readBody puts r's body in details if it's JSON, and its size otherwise,
// then puts it back for the handler.
func readBody(r *http.Request, details map[string]any) {
	if r.Body == nil || r.Body == http.NoBody {
		return
	}
	body, _ := io.ReadAll(io.LimitReader(r.Body, MaxBody+1))
	r.Body = struct {
		io.Reader
		io.Closer
	}{io.MultiReader(bytes.NewReader(body), r.Body), r.Body}
	switch {
	case len(body) == 0:
	// Postgres can't store \u0000 in jsonb.
	case len(body) <= MaxBody && json.Valid(body) && !bytes.Contains(body, []byte(`\u0000`)):
		details["body"] = json.RawMessage(body)
	default:
		details["content_type"] = r.Header.Get("Content-Type")
		details["body_bytes"] = max(r.ContentLength, int64(len(body)))
	}
}

func (q *Requests) record(r *http.Request, details map[string]any) {
	row := db.AddAuditEventParams{Kind: "request"}
	if user := auth.User(r.Context()); user != nil {
		row.ActorID = &user.ID
	}
	if rctx := chi.RouteContext(r.Context()); rctx != nil {
		details["route"] = rctx.RoutePattern()
		params := map[string]string{}
		for i, key := range rctx.URLParams.Keys {
			if key != "*" {
				params[key] = rctx.URLParams.Values[i]
			}
		}
		if len(params) > 0 {
			details["params"] = params
		}
		// Requests about someone (/api/admin/users/{userID}/...) show up when
		// filtering on them.
		if id, err := strconv.ParseInt(params["userID"], 10, 64); err == nil {
			row.UserID = &id
		}
	}
	row.Details, _ = json.Marshal(details)
	if err := q.Queries.AddAuditEvent(context.WithoutCancel(r.Context()), row); err != nil {
		log.Printf("recording %s %s in the audit log: %v", r.Method, r.URL.Path, err)
	}
}
