package api_test

// Ways people could try to get the same time paid twice.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/scherepi/crossroads-backend/auth"
)

func TestOverlappingTimePaysOnce(t *testing.T) {
	e := newEnv(t)
	reviewer := e.newReviewer(t)
	user := e.newUser(t, "U1")
	// 10 hours on the client and 4 on the server, 3 of them at the same
	// time: 11 hours of real work.
	e.hackatime.set("U1", "client", 10*3600)
	e.hackatime.set("U1", "server", 4*3600)
	e.hackatime.overlaps = []fakeOverlap{{"U1", "client", "server", 3 * 3600}}
	client := e.createProject(t, user, completeProject("client"))
	server := e.createProject(t, user, completeProject("server"))

	_, first, _ := e.submit(t, user, client.ID)
	if first.NewSeconds != 10*3600 {
		t.Fatalf("client claimed %d, want 10 hours", first.NewSeconds)
	}

	var detail project
	e.call(t, user, "GET", "/api/projects/"+itoa(server.ID), nil, &detail)
	if detail.UnsubmittedSeconds == nil || *detail.UnsubmittedSeconds != 3600 {
		t.Errorf("server shows %v unsubmitted, want the 1 hour that didn't overlap", detail.UnsubmittedSeconds)
	}
	_, second, _ := e.submit(t, user, server.ID)
	if second.NewSeconds != 3600 {
		t.Fatalf("server claimed %d, want only the 1 hour that didn't overlap", second.NewSeconds)
	}

	e.decide(t, reviewer, first.ID, map[string]any{"status": "approved"})
	e.decide(t, reviewer, second.ID, map[string]any{"status": "approved"})
	if got := e.stamps(t, user); got != 11*10 {
		t.Errorf("balance is %d, want %d for 11 hours at 10 an hour", got, 11*10)
	}
	status, _, problem := e.submit(t, user, server.ID)
	if status != http.StatusUnprocessableEntity || !strings.Contains(problem.Error, "another of your projects") {
		t.Errorf("resubmitting the overlap: got %d %q", status, problem.Error)
	}
}

func TestRenamedHackatimeProjectPaysOnce(t *testing.T) {
	e := newEnv(t)
	reviewer := e.newReviewer(t)
	user := e.newUser(t, "U1")
	e.hackatime.set("U1", "old-name", 5*3600)
	first := e.createProject(t, user, completeProject("old-name"))
	_, s, _ := e.submit(t, user, first.ID)
	e.decide(t, reviewer, s.ID, map[string]any{"status": "approved"})

	// Renaming the project in Hackatime moves its time to the new name.
	e.hackatime.set("U1", "old-name", 0)
	e.hackatime.set("U1", "new-name", 5*3600)
	second := e.createProject(t, user, completeProject("new-name"))
	if status, _, _ := e.submit(t, user, second.ID); status != http.StatusUnprocessableEntity {
		t.Errorf("claiming renamed time again: got %d, want 422", status)
	}

	// Work done after the rename still counts.
	e.hackatime.set("U1", "new-name", 7*3600)
	status, s, _ := e.submit(t, user, second.ID)
	if status != http.StatusCreated || s.NewSeconds != 2*3600 {
		t.Errorf("got %d claiming %d, want the 2 new hours", status, s.NewSeconds)
	}
}

func TestRejectedTimeGoesBackToThePool(t *testing.T) {
	e := newEnv(t)
	reviewer := e.newReviewer(t)
	user := e.newUser(t, "U1")
	e.hackatime.set("U1", "client", 10*3600)
	e.hackatime.set("U1", "server", 4*3600)
	e.hackatime.overlaps = []fakeOverlap{{"U1", "client", "server", 3 * 3600}}
	client := e.createProject(t, user, completeProject("client"))
	server := e.createProject(t, user, completeProject("server"))

	_, s, _ := e.submit(t, user, client.ID)
	e.decide(t, reviewer, s.ID, map[string]any{"status": "rejected", "feedback": "nope"})
	_, s, _ = e.submit(t, user, server.ID)
	if s.NewSeconds != 4*3600 {
		t.Errorf("server claimed %d, want all 4 hours now that the client was rejected", s.NewSeconds)
	}
}

func TestSimultaneousSubmissionsDontBothClaimTheOverlap(t *testing.T) {
	e := newEnv(t)
	user := e.newUser(t, "U1")
	e.hackatime.set("U1", "client", 10*3600)
	e.hackatime.set("U1", "server", 4*3600)
	e.hackatime.overlaps = []fakeOverlap{{"U1", "client", "server", 3 * 3600}}
	ids := []int64{
		e.createProject(t, user, completeProject("client")).ID,
		e.createProject(t, user, completeProject("server")).ID,
	}

	// Hold each submission at its last Hackatime call, the one covering both
	// projects, until the other gets there too, so they hit the database
	// together.
	var arrived sync.WaitGroup
	arrived.Add(2)
	e.hackatime.beforeStats = func(r *http.Request) {
		if strings.Contains(r.URL.Query().Get("filter_by_project"), ",") {
			arrived.Done()
			arrived.Wait()
		}
	}

	var wg sync.WaitGroup
	for _, id := range ids {
		wg.Go(func() {
			req := httptest.NewRequest("POST", "/api/projects/"+itoa(id)+"/submissions", nil)
			req = req.WithContext(auth.WithUser(req.Context(), user))
			e.router.ServeHTTP(httptest.NewRecorder(), req)
		})
	}
	wg.Wait()

	var claimed int
	e.pool.QueryRow(context.Background(), "select coalesce(sum(new_seconds), 0) from submissions").Scan(&claimed)
	if claimed != 11*3600 {
		t.Errorf("claimed %d between them, want exactly the 11 real hours", claimed)
	}
}

func TestReviewersSeeProjectsSharingLinks(t *testing.T) {
	e := newEnv(t)
	reviewer := e.newReviewer(t)
	original := e.newUser(t, "U1")
	copier := e.newUser(t, "U2")
	bystander := e.newUser(t, "U3")

	theirs := completeProject("racer")
	theirs["repo_url"] = "https://github.com/someone/racer"
	originalProject := e.createProject(t, original, theirs)

	copied := completeProject("racer")
	copied["repo_url"] = "http://www.GitHub.com/someone/racer.git/"
	copied["demo_url"] = "https://copier.example/racer"
	copiedProject := e.createProject(t, copier, copied)

	unrelated := completeProject("racer")
	unrelated["repo_url"] = "https://github.com/someone/racer-2"
	unrelated["demo_url"] = "https://bystander.example"
	e.createProject(t, bystander, unrelated)

	e.hackatime.set("U2", "racer", 3600)
	_, s, _ := e.submit(t, copier, copiedProject.ID)

	var detail struct {
		ProjectsSharingLinks []struct {
			ID     int64 `json:"id"`
			UserID int64 `json:"user_id"`
		} `json:"projects_sharing_links"`
	}
	e.call(t, reviewer, "GET", "/api/review/submissions/"+itoa(s.ID), nil, &detail)
	got := detail.ProjectsSharingLinks
	if len(got) != 1 || got[0].ID != originalProject.ID || got[0].UserID != original.ID {
		t.Errorf("projects sharing links: %+v, want just the original", got)
	}
}
