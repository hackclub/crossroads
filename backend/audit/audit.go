// Package audit records every request that can change something in the audit
// log, for superadmins to read. Migration 24 describes the log; triggers there
// add the other kinds of event.
package audit

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/scherepi/crossroads-backend/auth"
	"github.com/scherepi/crossroads-backend/db"
)

// MaxBody is the most of a request body kept. Bigger bodies, and ones that
// aren't JSON (uploads), are recorded by size only.
const MaxBody = 64 << 10

// DefaultBudget is how many bytes of audit log each person signed in can add
// a minute when Requests.Budget isn't set: far more than clicking around uses,
// and little enough that a script can't fill the database.
const DefaultBudget = 128 << 10

// maxField is the most kept of the path, route parameters, IP, user agent and
// content type. They come from the request, which can be a megabyte of
// headers.
const maxField = 512

type Requests struct {
	Queries *db.Queries
	// Budget is how many bytes of audit log each person signed in can add a
	// minute, and all at once after a quiet minute. Past it, their rows are
	// kept without the body, then not kept; the next row kept says how many
	// weren't (details.unrecorded). Requests a handler vouched for (Vouch)
	// aren't counted. Zero is DefaultBudget.
	Budget int

	mu      sync.Mutex
	buckets map[int64]*bucket // by user ID; one per person, so it stays small
}

type bucket struct {
	left       float64   // bytes
	at         time.Time // when left was worked out
	unrecorded int       // requests not kept since the last one that was
}

type vouchKey struct{}

// Vouch says an anonymous request is one to record: its handler knows who
// sent it some other way than a session, like Sidekick's secret. Other
// anonymous requests aren't recorded, since anyone can send them.
func Vouch(r *http.Request) {
	if vouched, ok := r.Context().Value(vouchKey{}).(*bool); ok {
		*vouched = true
	}
}

// Record is middleware that adds a 'request' row to the audit log for every
// request but GET, HEAD and OPTIONS made by someone signed in or vouched for
// (Vouch): who was signed in, the route and its parameters, the JSON body if
// the request succeeded, the status it got, and where it came from. Headers
// aren't kept, so neither are the secrets Sidekick and the Slack bot send in
// them. It goes after auth.LoadSession, and records once the handler is
// done. Failing to record is logged, not shown to anyone.
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
			"path":       clip(r.URL.Path),
			"ip":         clip(auth.ClientIP(r)),
			"user_agent": clip(r.UserAgent()),
		}
		body, size := readBody(r)
		vouched := false
		r = r.WithContext(context.WithValue(r.Context(), vouchKey{}, &vouched))
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
			if auth.User(r.Context()) != nil || vouched {
				details["status"] = status
				details["ms"] = time.Since(start).Milliseconds()
				if size > 0 {
					details["content_type"] = clip(r.Header.Get("Content-Type"))
					details["body_bytes"] = size
				}
				// A rejected request changed nothing, so its body isn't
				// worth the space.
				if status >= 400 {
					body = nil
				}
				q.record(r, details, body)
			}
			if p != nil {
				panic(p)
			}
		}()
		next.ServeHTTP(ww, r)
	})
}

// readBody returns r's body if it's JSON that fits in MaxBody, and its size,
// then puts it back for the handler.
func readBody(r *http.Request) (json.RawMessage, int64) {
	if r.Body == nil || r.Body == http.NoBody {
		return nil, 0
	}
	body, _ := io.ReadAll(io.LimitReader(r.Body, MaxBody+1))
	r.Body = struct {
		io.Reader
		io.Closer
	}{io.MultiReader(bytes.NewReader(body), r.Body), r.Body}
	size := max(r.ContentLength, int64(len(body)))
	// Postgres can't store \u0000 in jsonb.
	if len(body) > 0 && len(body) <= MaxBody && json.Valid(body) && !bytes.Contains(body, []byte(`\u0000`)) {
		return body, size
	}
	return nil, size
}

func (q *Requests) record(r *http.Request, details map[string]any, body json.RawMessage) {
	row := db.AddAuditEventParams{Kind: "request"}
	user := auth.User(r.Context())
	if user != nil {
		row.ActorID = &user.ID
	}
	if rctx := chi.RouteContext(r.Context()); rctx != nil {
		details["route"] = rctx.RoutePattern()
		params := map[string]string{}
		for i, key := range rctx.URLParams.Keys {
			if key != "*" {
				params[key] = clip(rctx.URLParams.Values[i])
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
	if user != nil {
		withoutBody, _ := json.Marshal(details)
		keep, keepBody, unrecorded := q.spend(user.ID, len(withoutBody), len(body), time.Now())
		if !keep {
			if unrecorded == 1 {
				log.Printf("user %d is over their audit log budget, so their requests aren't being recorded for now", user.ID)
			}
			return
		}
		if !keepBody {
			body = nil
		}
		if unrecorded > 0 {
			details["unrecorded"] = unrecorded
		}
	}
	if body != nil {
		// The body says what the size did.
		delete(details, "content_type")
		delete(details, "body_bytes")
		details["body"] = body
	}
	row.Details, _ = json.Marshal(details)
	if err := q.Queries.AddAuditEvent(context.WithoutCancel(r.Context()), row); err != nil {
		log.Printf("recording %s %s in the audit log: %v", r.Method, r.URL.Path, err)
	}
}

// spend takes a row of size bytes, and its body, from userID's budget. It says
// whether to keep the row and whether to keep its body too. unrecorded is how
// many of their requests in a row weren't kept: before this one if it's kept,
// and up to it if not.
func (q *Requests) spend(userID int64, size, bodySize int, now time.Time) (keep, keepBody bool, unrecorded int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	perMinute := float64(cmp.Or(q.Budget, DefaultBudget))
	if q.buckets == nil {
		q.buckets = map[int64]*bucket{}
	}
	b := q.buckets[userID]
	if b == nil {
		b = &bucket{left: perMinute, at: now}
		q.buckets[userID] = b
	}
	b.left = min(perMinute, b.left+now.Sub(b.at).Minutes()*perMinute)
	b.at = now
	switch {
	case float64(size+bodySize) <= b.left:
		b.left -= float64(size + bodySize)
		keep, keepBody = true, true
	case float64(size) <= b.left:
		b.left -= float64(size)
		keep = true
	default:
		b.unrecorded++
		return false, false, b.unrecorded
	}
	unrecorded, b.unrecorded = b.unrecorded, 0
	return keep, keepBody, unrecorded
}

// clip cuts s to maxField bytes, at the end of a character.
func clip(s string) string {
	if len(s) <= maxField {
		return s
	}
	return strings.ToValidUTF8(s[:maxField], "")
}
