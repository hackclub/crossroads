package api_test

import (
	"net/http"
	"strings"
	"testing"
)

func TestReviewersFixDemoLinksAndScreenshots(t *testing.T) {
	e := newEnv(t)
	reviewer := e.newStageReviewer(t, "UREV", true, false, false)
	user, id := e.shipFor(t, "U1", 3)
	projectID := e.projectOf(t, id)
	path := "/api/review/projects/" + itoa(projectID)

	// While it's in review, a reviewer swaps the repo link for a real demo.
	var p struct {
		DemoURL       string `json:"demo_url"`
		ScreenshotURL string `json:"screenshot_url"`
	}
	e.mustCall(t, reviewer, http.StatusOK, "PATCH", path, map[string]any{
		"demo_url": " https://example.itch.io/game ", "screenshot_url": "https://example.com/shot.png",
	}, &p)
	if p.DemoURL != "https://example.itch.io/game" || p.ScreenshotURL != "https://example.com/shot.png" {
		t.Errorf("after the change: %+v", p)
	}
	// The maker sees what changed, on their project.
	var mine struct {
		DemoURL     string `json:"demo_url"`
		Submissions []struct {
			Messages []struct {
				Kind    string `json:"kind"`
				Message string `json:"message"`
			} `json:"messages"`
		} `json:"submissions"`
	}
	e.mustCall(t, user, http.StatusOK, "GET", "/api/projects/"+itoa(projectID), nil, &mine)
	msgs := mine.Submissions[0].Messages
	if mine.DemoURL != "https://example.itch.io/game" || len(msgs) != 1 ||
		!strings.Contains(msgs[0].Message, "demo link to https://example.itch.io/game") || !strings.Contains(msgs[0].Message, "screenshot") {
		t.Errorf("the maker sees %+v", mine)
	}
	// It's still in review, and the reviewer can still decide it.
	if d := e.detail(t, reviewer, id); d.Submission.Status != "pending" || !d.CanDecide {
		t.Errorf("after editing: %+v", d)
	}

	// The same link again changes nothing and says nothing.
	e.mustCall(t, reviewer, http.StatusOK, "PATCH", path, map[string]any{"demo_url": "https://example.itch.io/game"}, nil)
	e.mustCall(t, user, http.StatusOK, "GET", "/api/projects/"+itoa(projectID), nil, &mine)
	if n := len(mine.Submissions[0].Messages); n != 1 {
		t.Errorf("%d messages after a change that wasn't one", n)
	}

	for _, c := range []struct {
		who  string
		body map[string]any
		want int
	}{
		{"reviewer", map[string]any{"demo_url": ""}, http.StatusUnprocessableEntity},
		{"reviewer", map[string]any{"demo_url": "javascript:alert(1)"}, http.StatusUnprocessableEntity},
		{"reviewer", map[string]any{"name": "Renamed"}, http.StatusBadRequest},
		{"owner", map[string]any{"demo_url": "https://example.com"}, http.StatusForbidden},
	} {
		who := reviewer
		if c.who == "owner" {
			who = user
		}
		if status := e.call(t, who, "PATCH", path, c.body, nil); status != c.want {
			t.Errorf("%s %v: got %d, want %d", c.who, c.body, status, c.want)
		}
	}
	// Not their own project, even as a reviewer.
	own := e.newStageReviewer(t, "UREV2", true, false, false)
	p2 := e.createProject(t, own, projectNamed("mine", "ht-mine"))
	e.submitHours(t, own, "UREV2", p2, "ht-mine", 3600)
	if status := e.call(t, own, "PATCH", "/api/review/projects/"+itoa(p2.ID), map[string]any{"demo_url": "https://example.com"}, nil); status != http.StatusForbidden {
		t.Errorf("a reviewer changed their own project: %d", status)
	}
}
