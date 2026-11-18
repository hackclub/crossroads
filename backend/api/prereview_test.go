package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/scherepi/crossroads-backend/db"
)

type prereviewPrompt struct {
	Prompt  string  `json:"prompt"`
	ShipIDs []int64 `json:"ship_ids"`
	Waiting int     `json:"waiting"`
}

type prereviewSaved struct {
	Saved  []int64 `json:"saved"`
	Failed []struct {
		ShipID int64  `json:"ship_id"`
		Error  string `json:"error"`
	} `json:"failed"`
}

func TestPrereviewsByPasting(t *testing.T) {
	e := newEnv(t)
	reviewer := e.newStageReviewer(t, "UREV", true, true, false) // with the fraud check, so it can see them
	user, first := e.shipFor(t, "U1", 3)
	_, second := e.shipFor(t, "U2", 2)
	_, decided := e.shipFor(t, "U3", 2)
	e.decide(t, reviewer, decided, map[string]any{"action": "reject", "feedback": "Add a demo."})

	if status := e.call(t, user, "GET", "/api/review/prereviews/prompt", nil, nil); status != http.StatusForbidden {
		t.Errorf("a participant got the prompt: %d", status)
	}
	var p prereviewPrompt
	e.mustCall(t, reviewer, http.StatusOK, "GET", "/api/review/prereviews/prompt", nil, &p)
	if !slices.Equal(p.ShipIDs, []int64{first, second}) || p.Waiting != 2 {
		t.Errorf("ships in the prompt: %v, waiting %d", p.ShipIDs, p.Waiting)
	}
	for _, want := range []string{"never suggest an hours number", fmt.Sprintf("<ship id=%d>", first), "Project: p-U1", `{"prereviews": [`} {
		if !strings.Contains(p.Prompt, want) {
			t.Errorf("the prompt doesn't have %q", want)
		}
	}

	// Claude's answer as pasted: words around it, a code fence, one ship
	// already decided and one with a made-up level.
	answer := fmt.Sprintf("Here you go:\n```json\n%s\n```\nLet me know!", mustJSON(t, map[string]any{"prereviews": []map[string]any{
		{"ship_id": first, "summary": "a todo app in svelte", "scale": "2 pages", "features": []string{"todo list", "dark mode"},
			"ai_usage": map[string]any{"level": "Medium", "evidence": []string{"README: em dashes"}}, "readme": "written for it",
			"demo": "a live site", "contributions": []map[string]any{{"name": "alice", "commits": 12, "areas": "everything"}}, "notes": []string{}},
		{"ship_id": second, "summary": "a bot", "ai_usage": map[string]any{"level": "very"}},
		{"ship_id": decided, "summary": "a game", "ai_usage": map[string]any{"level": "none"}},
	}}))
	countEvents := func() (n int) {
		t.Helper()
		if err := e.pool.QueryRow(context.Background(), "select count(*) from ship_events").Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	eventsBefore := countEvents()
	var saved prereviewSaved
	e.mustCall(t, reviewer, http.StatusOK, "POST", "/api/review/prereviews", map[string]any{"text": answer}, &saved)
	if !slices.Equal(saved.Saved, []int64{first}) || len(saved.Failed) != 2 {
		t.Errorf("saved %v, failed %+v", saved.Saved, saved.Failed)
	}

	var detail struct {
		Prereview *struct {
			Summary string `json:"summary"`
			AIUsage struct {
				Level string `json:"level"`
			} `json:"ai_usage"`
			Source string `json:"source"`
			By     string `json:"by"`
		} `json:"prereview"`
	}
	e.mustCall(t, reviewer, http.StatusOK, "GET", "/api/review/submissions/"+itoa(first), nil, &detail)
	if pr := detail.Prereview; pr == nil || pr.Summary != "a todo app in svelte" || pr.AIUsage.Level != "medium" || pr.Source != "paste" || pr.By != "Test" {
		t.Errorf("the ship's pre-review: %+v", detail.Prereview)
	}
	e.mustCall(t, reviewer, http.StatusOK, "GET", "/api/review/submissions/"+itoa(second), nil, &detail)
	if detail.Prereview != nil {
		t.Errorf("a refused pre-review was saved: %+v", detail.Prereview)
	}

	// The next prompt leaves out the ship that has one.
	e.mustCall(t, reviewer, http.StatusOK, "GET", "/api/review/prereviews/prompt", nil, &p)
	if !slices.Equal(p.ShipIDs, []int64{second}) || p.Waiting != 1 {
		t.Errorf("ships in the next prompt: %v, waiting %d", p.ShipIDs, p.Waiting)
	}

	for _, bad := range []string{"", "no json here", "{\"prereviews\": []}", "{\"hello\": 1}"} {
		if status := e.call(t, reviewer, "POST", "/api/review/prereviews", map[string]any{"text": bad}, nil); status != http.StatusUnprocessableEntity {
			t.Errorf("pasting %q: %d, want 422", bad, status)
		}
	}

	// A pre-review is never a review: nobody's paid for it.
	if n := countEvents() - eventsBefore; n != 0 {
		t.Errorf("pasting a pre-review wrote %d ship events", n)
	}
}

// Reviewers with only the Review permission see nothing of pre-reviews
// until an admin turns that on; the fraud check, HQ and admins always do.
func TestPrereviewsHiddenFromBaseReviewers(t *testing.T) {
	e := newEnv(t)
	base := e.newStageReviewer(t, "UBASE", true, false, false)
	hq := e.newStageReviewer(t, "UHQ", false, false, true)
	admin := e.newAdmin(t)
	_, ship := e.shipFor(t, "U1", 3)
	e.mustCall(t, hq, http.StatusOK, "POST", "/api/review/prereviews",
		map[string]any{"prereviews": []map[string]any{{"ship_id": ship, "summary": "a cli", "ai_usage": map[string]any{"level": "low"}}}}, nil)

	type sees struct {
		me, prompt, batch, save int
		visible, has            bool
	}
	look := func(u *db.User) sees {
		t.Helper()
		var me struct {
			CanPrereview bool `json:"can_prereview"`
		}
		e.mustCall(t, u, http.StatusOK, "GET", "/api/me", nil, &me)
		var detail struct {
			Prereview         *struct{} `json:"prereview"`
			PrereviewsVisible bool      `json:"prereviews_visible"`
		}
		e.mustCall(t, u, http.StatusOK, "GET", "/api/review/submissions/"+itoa(ship), nil, &detail)
		meOK := 0
		if me.CanPrereview {
			meOK = 1
		}
		return sees{
			me:      meOK,
			prompt:  e.call(t, u, "GET", "/api/review/prereviews/prompt", nil, nil),
			batch:   e.call(t, u, "GET", "/api/review/prereviews/batch", nil, nil),
			save:    e.call(t, u, "POST", "/api/review/prereviews", map[string]any{"text": `{"ship_id": 1, "summary": "x"}`}, nil),
			visible: detail.PrereviewsVisible,
			has:     detail.Prereview != nil,
		}
	}
	if got, want := look(base), (sees{0, 403, 403, 403, false, false}); got != want {
		t.Errorf("a base reviewer, turned off: %+v, want %+v", got, want)
	}
	if got := look(hq); got.me != 1 || got.prompt != 200 || !got.visible || !got.has {
		t.Errorf("HQ: %+v", got)
	}
	e.mustCall(t, admin, http.StatusOK, "PATCH", "/api/admin/review-settings", map[string]any{"prereviews_for_reviewers": true}, nil)
	if got := look(base); got.me != 1 || got.prompt != 200 || got.batch != 200 || got.save != 200 || !got.visible || !got.has {
		t.Errorf("a base reviewer, turned on: %+v", got)
	}
}

func TestPrereviewsFromTheScript(t *testing.T) {
	e := newEnv(t)
	_, ship := e.shipFor(t, "U1", 3)
	// Without PREREVIEW_TOKEN the token routes don't exist.
	if status := e.call(t, nil, "GET", "/api/prereviews", nil, nil); status == http.StatusOK {
		t.Errorf("/api/prereviews without a token set: %d", status)
	}
	e.api.PrereviewToken = "secret"
	r := chi.NewRouter()
	r.Mount("/api", e.api.Routes())
	e.router = r

	send := func(method, token string, body any, out any) int {
		t.Helper()
		var b []byte
		if body != nil {
			b = mustJSON(t, body)
		}
		req := httptest.NewRequest(method, "/api/prereviews", bytes.NewReader(b))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		e.router.ServeHTTP(rec, req)
		if out != nil {
			if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
				t.Fatalf("decoding %q: %v", rec.Body.String(), err)
			}
		}
		return rec.Code
	}
	if status := send("GET", "wrong", nil, nil); status != http.StatusUnauthorized {
		t.Errorf("a wrong token: %d", status)
	}
	var batch struct {
		Instructions string `json:"instructions"`
		Ships        []struct {
			ID      int64  `json:"id"`
			Context string `json:"context"`
		} `json:"ships"`
	}
	if status := send("GET", "secret", nil, &batch); status != http.StatusOK || len(batch.Ships) != 1 || batch.Ships[0].ID != ship {
		t.Fatalf("batch: %d %+v", status, batch)
	}
	if !strings.Contains(batch.Instructions, "cloned in the current working directory") || !strings.Contains(batch.Ships[0].Context, "Project: p-U1") {
		t.Errorf("the batch's instructions or context are off: %q", batch.Instructions[:80])
	}
	var saved prereviewSaved
	body := map[string]any{"model": "sonnet", "prereviews": []map[string]any{{"ship_id": ship, "summary": "a cli", "ai_usage": map[string]any{"level": "low"}}}}
	if status := send("POST", "secret", body, &saved); status != http.StatusOK || !slices.Equal(saved.Saved, []int64{ship}) {
		t.Errorf("uploading: %d %+v", status, saved)
	}
	var source, model string
	if err := e.pool.QueryRow(context.Background(), "select source, model from ship_prereviews where submission_id = $1", ship).Scan(&source, &model); err != nil {
		t.Fatal(err)
	}
	if source != "script" || model != "sonnet" {
		t.Errorf("stored as %s by %s", source, model)
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
