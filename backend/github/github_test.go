package github_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/scherepi/crossroads-backend/devgithub"
	"github.com/scherepi/crossroads-backend/github"
)

// TestProxy asks a stand-in for gh-proxy, which checks the key and, like the
// real one, sometimes says to come back in a second.
func TestProxy(t *testing.T) {
	now := time.Now()
	fake := (&devgithub.Fake{Created: now.AddDate(0, 0, -10), HoursStart: now.AddDate(0, 0, -5)}).Routes()
	var calls, throttled atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "key" || r.Header.Get("Authorization") != "" {
			t.Errorf("%s: X-API-Key %q, Authorization %q", r.URL, r.Header.Get("X-API-Key"), r.Header.Get("Authorization"))
		}
		// The first request, and one while counting lines.
		if n := calls.Add(1); n == 1 || n == 5 {
			throttled.Add(1)
			w.Header().Set("Retry-After", "1")
			http.Error(w, `{"error":{"code":"RATE_LIMIT_EXCEEDED"}}`, http.StatusTooManyRequests)
			return
		}
		http.StripPrefix("/gh", fake).ServeHTTP(w, r)
	}))
	defer proxy.Close()

	c := github.New(github.Config{BaseURL: proxy.URL + "/gh", Token: "not for the proxy", ProxyKey: "key"})
	r, err := c.Activity(context.Background(), "https://github.com/dev-ada/racer", now.AddDate(0, 0, -5), now)
	if err != nil {
		t.Fatal(err)
	}
	if throttled.Load() != 2 {
		t.Errorf("turned away %d requests, want 2", throttled.Load())
	}
	if len(r.Commits) == 0 || r.LinesCounted != len(r.Commits) || r.LinesNote != "" {
		t.Errorf("%d commits, %d with lines counted, note %q", len(r.Commits), r.LinesCounted, r.LinesNote)
	}
	if r.Readme == "" {
		t.Error("no README")
	}
}

// A long wait is GitHub's own limit, and isn't waited out.
func TestRateLimited(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c := github.New(github.Config{BaseURL: srv.URL, ProxyKey: "key"})
	_, err := c.Activity(context.Background(), "https://github.com/ada/racer", time.Now().AddDate(0, 0, -1), time.Now())
	if !errors.Is(err, github.ErrRateLimited) || calls.Load() != 1 {
		t.Errorf("got %v after %d requests, want ErrRateLimited after 1", err, calls.Load())
	}
}
