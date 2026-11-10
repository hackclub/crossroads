package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const sidekickSecret = "sk_test_secret"

// sidekick calls POST /api/sidekick as Sidekick would.
func (e *env) sidekick(t *testing.T, action string, input any, out any) int {
	t.Helper()
	if in, ok := input.(map[string]any); ok && in["action"] == "approve" {
		fields, _ := in["fields"].(map[string]any)
		if fields == nil {
			fields = map[string]any{}
		}
		if _, ok := fields["technical_features"]; !ok {
			fields["technical_features"] = "A test feature."
		}
		if _, ok := fields["deflation_reason"]; !ok {
			fields["deflation_reason"] = "A test deflation."
		}
		in["fields"] = fields
	}
	body, _ := json.Marshal(map[string]any{"action": action, "input": input})
	req := httptest.NewRequest("POST", "/api/sidekick", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+sidekickSecret)
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	if out != nil {
		if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
			t.Fatalf("%s: decoding %q: %v", action, rec.Body.String(), err)
		}
	}
	return rec.Code
}

type skProjectOut struct {
	ID       string `json:"id"`
	AuthorID string `json:"authorId"`
	Tags     []struct {
		Label string `json:"label"`
	} `json:"tags"`
	Ships []struct {
		ID            string `json:"id"`
		Status        string `json:"status"`
		ApproveFields []any  `json:"approveFields"`
		Checks        []struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"checks"`
	} `json:"ships"`
}

func TestSidekickNeedsTheSecret(t *testing.T) {
	e := newEnv(t)
	req := httptest.NewRequest("POST", "/api/sidekick", strings.NewReader(`{"action":"HEALTH_CHECK","input":{}}`))
	req.Header.Set("Authorization", "Bearer wrong")
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("wrong secret: got %d, want 401", rec.Code)
	}
	var health struct {
		OK bool `json:"ok"`
	}
	if status := e.sidekick(t, "HEALTH_CHECK", map[string]any{}, &health); status != http.StatusOK || !health.OK {
		t.Errorf("health check: got %d %+v", status, health)
	}
	var problem struct {
		Error string `json:"error"`
	}
	if status := e.sidekick(t, "FETCH_ORDERS", map[string]any{}, &problem); status != http.StatusBadRequest || problem.Error != "INVALID_ACTION" {
		t.Errorf("shop action: got %d %+v, want INVALID_ACTION", status, problem)
	}
}

func TestSidekickTwoStageReview(t *testing.T) {
	e := newEnv(t)
	reviewer := e.newUser(t, "UREVIEWER") // any Crossroads account; Sidekick vouches for them
	hq := e.newUser(t, "UHQ")
	user, id := e.shipFor(t, "U1", 3)
	ship := itoa(id)

	var list struct {
		Projects   []skProjectOut `json:"projects"`
		TotalCount int            `json:"totalCount"`
	}
	e.sidekick(t, "FETCH_PROJECTS", map[string]any{"status": "pending"}, &list)
	if list.TotalCount != 1 || len(list.Projects) != 1 || list.Projects[0].AuthorID != "U1" || list.Projects[0].Ships[0].Status != "pending" {
		t.Fatalf("pending projects are %+v", list)
	}
	var detail skProjectOut
	e.sidekick(t, "FETCH_PROJECT_DETAIL", map[string]any{"projectId": list.Projects[0].ID}, &detail)
	if len(detail.Ships[0].Checks) == 0 || len(detail.Ships[0].ApproveFields) != 4 {
		t.Errorf("the ship in review has no checks or fields: %+v", detail.Ships[0])
	}

	// Someone who's never signed in to Crossroads can't be credited.
	var problem struct {
		Message string `json:"message"`
	}
	status := e.sidekick(t, "SUBMIT_REVIEW_ACTION", map[string]any{"shipId": ship, "reviewerId": "UNOBODY", "action": "approve",
		"hoursAssigned": 3, "feedbackMessage": "ok", "justification": "ok"}, &problem)
	if status != http.StatusBadRequest || !strings.Contains(problem.Message, "never signed in") {
		t.Errorf("unknown reviewer: got %d %q", status, problem.Message)
	}

	// A non-HQ approval waits for sign-off.
	if status := e.sidekick(t, "SUBMIT_REVIEW_ACTION", map[string]any{"shipId": ship, "reviewerId": "UREVIEWER", "action": "approve",
		"hoursAssigned": 2.5, "feedbackMessage": "Lovely", "justification": "commits match", "isHq": false}, nil); status != http.StatusOK {
		t.Fatalf("approving: got %d", status)
	}
	e.sidekick(t, "FETCH_PROJECTS", map[string]any{"status": "pending_hq"}, &list)
	if list.TotalCount != 1 || e.stamps(t, user) != 0 {
		t.Fatalf("pending_hq is %+v, balance %d", list, e.stamps(t, user))
	}

	// HQ sends it back, then it's approved again and authorized.
	e.sidekick(t, "SUBMIT_REVIEW_ACTION", map[string]any{"shipId": ship, "reviewerId": "UHQ", "action": "deauthorize", "message": "check day 3"}, nil)
	e.sidekick(t, "SUBMIT_REVIEW_ACTION", map[string]any{"shipId": ship, "reviewerId": "UREVIEWER", "action": "approve",
		"hoursAssigned": 2, "feedbackMessage": "Lovely", "justification": "day 3 cut", "isHq": false}, nil)
	// Hours can change while it waits.
	if status := e.sidekick(t, "UPDATE_REVIEW_ACTION", map[string]any{"shipId": ship, "reviewerId": "UREVIEWER", "type": "approval",
		"feedbackMessage": "Lovely work", "justification": "day 3 cut", "hoursAssigned": 2.5}, nil); status != http.StatusOK {
		t.Errorf("editing the approval: got %d", status)
	}
	if status := e.sidekick(t, "SUBMIT_REVIEW_ACTION", map[string]any{"shipId": ship, "reviewerId": "UHQ", "action": "authorize",
		"justification": "spot-checked"}, nil); status != http.StatusOK {
		t.Fatalf("authorizing: got %d", status)
	}
	if got := e.stamps(t, user); got != 25 {
		t.Errorf("balance is %d, want 25 for the edited 2.5 hours", got)
	}
	// Hours are fixed once it's signed off.
	if status := e.sidekick(t, "UPDATE_REVIEW_ACTION", map[string]any{"shipId": ship, "reviewerId": "UREVIEWER", "type": "approval",
		"feedbackMessage": "x", "hoursAssigned": 3}, nil); status != http.StatusBadRequest {
		t.Errorf("editing hours after sign-off: got %d, want 400", status)
	}

	var timeline struct {
		Events []struct {
			Type               string `json:"type"`
			ActorID            string `json:"actorId"`
			DiscardedByActorID string `json:"discardedByActorId"`
		} `json:"events"`
	}
	e.sidekick(t, "FETCH_PROJECT_TIMELINE", map[string]any{"projectId": list.Projects[0].ID}, &timeline)
	var types []string
	for _, ev := range timeline.Events {
		types = append(types, ev.Type)
		if ev.Type == "discarded_approval" && (ev.ActorID != "UREVIEWER" || ev.DiscardedByActorID != "UHQ") {
			t.Errorf("discarded approval is %+v", ev)
		}
	}
	want := "ship discarded_approval comment approval system"
	if strings.Join(types, " ") != want {
		t.Errorf("timeline is %v, want %s", types, want)
	}
	_ = reviewer
	_ = hq
}

func TestSidekickHoursOverride(t *testing.T) {
	e := newEnv(t)
	e.newUser(t, "UREVIEWER")
	_, id := e.shipFor(t, "U1", 2)
	approve := map[string]any{"shipId": itoa(id), "reviewerId": "UREVIEWER", "action": "approve",
		"hoursAssigned": 3, "feedbackMessage": "ok", "justification": "ok", "isHq": true}
	if status := e.sidekick(t, "SUBMIT_REVIEW_ACTION", approve, nil); status != http.StatusBadRequest {
		t.Errorf("over Hackatime without a reason: got %d, want 400", status)
	}
	approve["fields"] = map[string]any{"hours_override_reason": "Wired the LED matrix by hand."}
	if status := e.sidekick(t, "SUBMIT_REVIEW_ACTION", approve, nil); status != http.StatusOK {
		t.Fatalf("over Hackatime with a reason: got %d", status)
	}
	var list struct {
		TotalCount int `json:"totalCount"`
	}
	e.sidekick(t, "FETCH_PROJECTS", map[string]any{"status": "pending_hq"}, &list)
	if list.TotalCount != 1 {
		t.Error("an isHq override didn't wait for HQ sign-off")
	}
}

func TestSidekickCantAuthorizePastTheFraudCheck(t *testing.T) {
	e := newEnv(t)
	e.newUser(t, "UREVIEWER")
	e.newUser(t, "UHQ")
	_, id := e.shipFor(t, "U1", 3)
	e.sidekick(t, "SUBMIT_REVIEW_ACTION", map[string]any{"shipId": itoa(id), "reviewerId": "UREVIEWER", "action": "approve",
		"hoursAssigned": 3, "feedbackMessage": "ok", "justification": "ok", "isHq": true, "fields": map[string]any{"send_to_fraud": true}}, nil)
	var list struct {
		Projects []skProjectOut `json:"projects"`
	}
	e.sidekick(t, "FETCH_PROJECTS", map[string]any{"status": "pending_hq"}, &list)
	if len(list.Projects) != 1 || list.Projects[0].Tags[0].Label != "Fraud check" {
		t.Fatalf("pending_hq is %+v, want it tagged for the fraud check", list)
	}
	var problem struct {
		Message string `json:"message"`
	}
	if status := e.sidekick(t, "SUBMIT_REVIEW_ACTION", map[string]any{"shipId": itoa(id), "reviewerId": "UHQ", "action": "authorize"}, &problem); status != http.StatusConflict {
		t.Errorf("authorizing before the fraud check: got %d %q, want 409", status, problem.Message)
	}
}

func TestSidekickUserNotes(t *testing.T) {
	e := newEnv(t)
	e.newUser(t, "U1")
	var note struct {
		Note *string `json:"note"`
	}
	e.sidekick(t, "FETCH_USER_NOTE", map[string]any{"userId": "UNOBODY"}, &note)
	if note.Note != nil {
		t.Errorf("unknown user's note is %q", *note.Note)
	}
	e.sidekick(t, "UPDATE_USER_NOTE", map[string]any{"userId": "U1", "note": "Hackatime broken until Nov 5", "editorId": "UX"}, nil)
	e.sidekick(t, "FETCH_USER_NOTE", map[string]any{"userId": "U1"}, &note)
	if note.Note == nil || *note.Note != "Hackatime broken until Nov 5" {
		t.Errorf("note is %v", note.Note)
	}
	if status := e.sidekick(t, "UPDATE_USER_NOTE", map[string]any{"userId": "UNOBODY", "note": "x"}, nil); status != http.StatusNotFound {
		t.Errorf("note on an unknown user: got %d, want 404", status)
	}
}
