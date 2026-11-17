package api_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/scherepi/crossroads-backend/slack"
)

// fakeSlack answers the Slack Web API methods announcements use, and keeps
// what was posted. fail makes a method answer {"ok": false, "error": code}.
type fakeSlack struct {
	mu    sync.Mutex
	calls []slackCall
	fail  map[string]string
}

type slackCall struct {
	Method string
	Body   map[string]any // JSON bodies (chat.*)
	Query  string
}

func (f *fakeSlack) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	method := strings.TrimPrefix(r.URL.Path, "/")
	call := slackCall{Method: method, Query: r.URL.RawQuery}
	if b, _ := io.ReadAll(r.Body); len(b) > 0 {
		json.Unmarshal(b, &call.Body)
	}
	f.mu.Lock()
	f.calls = append(f.calls, call)
	code := f.fail[method]
	f.mu.Unlock()
	if code != "" {
		json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": code})
		return
	}
	var out map[string]any
	switch method {
	case "chat.postMessage":
		channel := call.Body["channel"].(string)
		if strings.HasPrefix(channel, "U") {
			channel = "D" + channel // a DM
		}
		out = map[string]any{"channel": channel, "ts": "1700000000.000100"}
	case "chat.getPermalink":
		out = map[string]any{"permalink": "https://hackclub.slack.com/archives/CSTATION/p1700000000000100"}
	case "users.info":
		out = map[string]any{"user": map[string]any{"profile": map[string]any{"display_name": "conductor super", "image_192": "https://avatars.test/super.png"}}}
	case "reactions.get":
		out = map[string]any{"message": map[string]any{"reply_count": 2, "reactions": []map[string]any{
			{"name": "tada", "count": 2, "users": []string{"U1", "U2"}},
			{"name": "train", "count": 1, "users": []string{"U1"}},
		}}}
	case "conversations.info":
		out = map[string]any{"channel": map[string]any{"id": "CSTATION", "name": "crossroads-station", "num_members": 400, "is_member": true}}
	}
	if out == nil {
		out = map[string]any{}
	}
	out["ok"] = true
	json.NewEncoder(w).Encode(out)
}

func (f *fakeSlack) setFail(method, code string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if code == "" {
		delete(f.fail, method)
	} else {
		f.fail[method] = code
	}
}

func (f *fakeSlack) called(method string) []slackCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	var calls []slackCall
	for _, c := range f.calls {
		if c.Method == method {
			calls = append(calls, c)
		}
	}
	return calls
}

func (e *env) withSlack(t *testing.T) *fakeSlack {
	t.Helper()
	fake := &fakeSlack{fail: map[string]string{}}
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)
	e.api.Slack = slack.New(slack.Config{Token: "xoxb-test", BaseURL: server.URL})
	e.api.AnnounceChannel = "CSTATION"
	return fake
}

type announcement struct {
	ID      int64 `json:"id"`
	Text    string
	Pings   bool `json:"pings"`
	Buttons []struct {
		ActionID           string `json:"action_id"`
		Text               string `json:"text"`
		URL                string `json:"url"`
		SpecialStampTypeID *int64 `json:"special_stamp_type_id"`
	} `json:"buttons"`
	ChannelID  *string `json:"channel_id"`
	MessageTS  *string `json:"message_ts"`
	Permalink  *string `json:"permalink"`
	PostedAt   *string `json:"posted_at"`
	DeletedAt  *string `json:"deleted_at"`
	Engagement *struct {
		Clicks        int `json:"clicks"`
		People        int `json:"people"`
		WithAccounts  int `json:"with_accounts"`
		SignedUpAfter int `json:"signed_up_after"`
		Buttons       []struct {
			ActionID  string  `json:"action_id"`
			Removed   bool    `json:"removed"`
			Clicks    int     `json:"clicks"`
			People    int     `json:"people"`
			StampName *string `json:"stamp_name"`
			Claims    *int    `json:"claims"`
		} `json:"buttons"`
		Clickers []struct {
			SlackID string   `json:"slack_id"`
			Name    *string  `json:"name"`
			Buttons []string `json:"buttons"`
		} `json:"clickers"`
		Reactions    []slack.Reaction `json:"reactions"`
		Reactors     *int             `json:"reactors"`
		Replies      *int             `json:"replies"`
		Members      *int             `json:"members"`
		SlackProblem string           `json:"slack_problem"`
	} `json:"engagement"`
}

// announcementBody is a message with an @channel, a link button and a claim
// button for code, as the platform's editor would build it.
func announcementBody(code string) map[string]any {
	return map[string]any{
		"doc":  map[string]any{"blocks": []any{}},
		"text": "<!channel> All aboard",
		"blocks": []any{
			map[string]any{"type": "rich_text", "elements": []any{map[string]any{"type": "rich_text_section", "elements": []any{
				map[string]any{"type": "broadcast", "range": "channel"},
				map[string]any{"type": "text", "text": " All aboard"},
			}}}},
			map[string]any{"type": "actions", "elements": []any{
				map[string]any{"type": "button", "action_id": "open", "text": map[string]any{"type": "plain_text", "text": "Open Crossroads"}, "url": "https://crossroads.test/home"},
				map[string]any{"type": "button", "action_id": "claim", "text": map[string]any{"type": "plain_text", "text": "Claim"}, "url": "https://crossroads.test/claim/" + code},
			}},
		},
	}
}

func TestOnlySuperadminsCanAnnounce(t *testing.T) {
	e := newEnv(t)
	e.withSlack(t)
	e.mustCall(t, e.newAdmin(t), http.StatusForbidden, "GET", "/api/superadmin/announcements", nil, nil)
	e.mustCall(t, e.newAdmin(t), http.StatusForbidden, "POST", "/api/superadmin/announcements", announcementBody("x"), nil)
	e.mustCall(t, e.newSuperadmin(t), http.StatusOK, "GET", "/api/superadmin/announcements", nil, nil)
}

func TestAnnouncementsAreCheckedBeforeSaving(t *testing.T) {
	e := newEnv(t)
	superadmin := e.newSuperadmin(t)
	post := func(change func(body map[string]any)) {
		t.Helper()
		body := announcementBody("nobody")
		change(body)
		e.mustCall(t, superadmin, http.StatusUnprocessableEntity, "POST", "/api/superadmin/announcements", body, nil)
	}
	// The claim link in announcementBody is to a code no stamp type has.
	post(func(body map[string]any) {})
	noClaim := func(body map[string]any) {
		actions := body["blocks"].([]any)[1].(map[string]any)
		actions["elements"] = actions["elements"].([]any)[:1]
	}
	post(func(body map[string]any) {
		noClaim(body)
		actions := body["blocks"].([]any)[1].(map[string]any)
		actions["elements"] = append(actions["elements"].([]any), actions["elements"].([]any)[0])
	})
	post(func(body map[string]any) { noClaim(body); body["blocks"] = []any{} })
	post(func(body map[string]any) { noClaim(body); body["text"] = "  " })
	post(func(body map[string]any) { noClaim(body); body["doc"] = nil })
	post(func(body map[string]any) {
		noClaim(body)
		button := body["blocks"].([]any)[1].(map[string]any)["elements"].([]any)[0].(map[string]any)
		button["url"] = "javascript:alert(1)"
	})
	var saved announcement
	body := announcementBody("")
	noClaim(body)
	e.mustCall(t, superadmin, http.StatusCreated, "POST", "/api/superadmin/announcements", body, &saved)
	if !saved.Pings || len(saved.Buttons) != 1 || saved.Buttons[0].ActionID != "open" || saved.Buttons[0].URL != "https://crossroads.test/home" {
		t.Errorf("saved %+v", saved)
	}
}

func TestPostingAnAnnouncementAndHowItDid(t *testing.T) {
	e := newEnv(t)
	fake := e.withSlack(t)
	admin := e.newAdmin(t)
	superadmin := e.newSuperadmin(t)
	var stamp stampType
	e.mustCall(t, admin, http.StatusCreated, "POST", "/api/admin/special-stamp-types",
		map[string]any{"name": "Station Pass", "rarity": "rare", "effect": "none", "claim_code": "station"}, &stamp)

	var draft announcement
	e.mustCall(t, superadmin, http.StatusCreated, "POST", "/api/superadmin/announcements", announcementBody("station"), &draft)
	if len(draft.Buttons) != 2 || draft.Buttons[1].SpecialStampTypeID == nil || *draft.Buttons[1].SpecialStampTypeID != stamp.ID {
		t.Fatalf("the claim button should be the Station Pass's: %+v", draft.Buttons)
	}
	path := "/api/superadmin/announcements/" + itoa(draft.ID)

	// A test goes to the superadmin's DMs, under their Slack name, and needs
	// no saving.
	test := announcementBody("station")
	delete(test, "doc")
	e.mustCall(t, superadmin, http.StatusNoContent, "POST", "/api/superadmin/announcements/test", test, nil)
	test["blocks"] = []any{}
	e.mustCall(t, superadmin, http.StatusUnprocessableEntity, "POST", "/api/superadmin/announcements/test", test, nil)
	if posts := fake.called("chat.postMessage"); len(posts) != 1 || posts[0].Body["channel"] != "USUPER" ||
		posts[0].Body["username"] != "conductor super" || posts[0].Body["icon_url"] != "https://avatars.test/super.png" {
		t.Fatalf("the test was %+v", posts)
	}

	var posted announcement
	e.mustCall(t, superadmin, http.StatusOK, "POST", path+"/post", nil, &posted)
	if posts := fake.called("chat.postMessage"); len(posts) != 2 || posts[1].Body["channel"] != "CSTATION" || posts[1].Body["text"] != "<!channel> All aboard" {
		t.Fatalf("the post was %+v", posts)
	}
	if posted.PostedAt == nil || *posted.ChannelID != "CSTATION" || *posted.MessageTS != "1700000000.000100" || posted.Permalink == nil {
		t.Errorf("posted %+v", posted)
	}
	e.mustCall(t, superadmin, http.StatusConflict, "POST", path+"/post", nil, nil)

	// Clicks on it, one on a test message, and a claim.
	ctx := context.Background()
	for _, click := range [][3]string{{"U1", "open", "CSTATION"}, {"U1", "open", "CSTATION"}, {"U1", "claim", "CSTATION"},
		{"U2", "claim", "CSTATION"}, {"U3", "gone", "CSTATION"}, {"U4", "open", "DUSUPER"}} {
		if _, err := e.pool.Exec(ctx, "insert into slack_clicks (slack_id, button, channel_id, message_ts) values ($1, $2, $3, '1700000000.000100')",
			click[0], click[1], click[2]); err != nil {
			t.Fatal(err)
		}
	}
	clicker := e.newUser(t, "U1") // signed up after clicking
	e.mustCall(t, clicker, http.StatusCreated, "POST", "/api/special-stamps/claims", map[string]any{"code": "station"}, nil)

	var got announcement
	e.mustCall(t, superadmin, http.StatusOK, "GET", path, nil, &got)
	en := got.Engagement
	if en == nil {
		t.Fatal("no engagement")
	}
	if en.Clicks != 5 || en.People != 3 || en.WithAccounts != 1 || en.SignedUpAfter != 1 {
		t.Errorf("engagement is %+v", en)
	}
	byID := map[string]int{}
	for i, b := range en.Buttons {
		byID[b.ActionID] = i
	}
	open, claim, gone := en.Buttons[byID["open"]], en.Buttons[byID["claim"]], en.Buttons[byID["gone"]]
	if open.Clicks != 2 || open.People != 1 || claim.Clicks != 2 || claim.People != 2 || !gone.Removed {
		t.Errorf("buttons are %+v", en.Buttons)
	}
	if claim.StampName == nil || *claim.StampName != "Station Pass" || claim.Claims == nil || *claim.Claims != 1 {
		t.Errorf("the claim button is %+v", claim)
	}
	if len(en.Clickers) != 3 || en.Clickers[0].SlackID != "U1" || en.Clickers[0].Name == nil || len(en.Clickers[0].Buttons) != 2 {
		t.Errorf("clickers are %+v", en.Clickers)
	}
	if len(en.Reactions) != 2 || *en.Reactors != 2 || *en.Replies != 2 || *en.Members != 400 || en.SlackProblem != "" {
		t.Errorf("from Slack: %+v", en)
	}

	// Editing it edits the message in Slack.
	body := announcementBody("station")
	body["text"] = "All aboard, again"
	e.mustCall(t, superadmin, http.StatusOK, "PATCH", path, body, nil)
	if updates := fake.called("chat.update"); len(updates) != 1 || updates[0].Body["ts"] != "1700000000.000100" || updates[0].Body["text"] != "All aboard, again" {
		t.Errorf("updates are %+v", updates)
	}
	// If Slack says no, nothing changes here.
	fake.setFail("chat.update", "cant_update_message")
	body["text"] = "Never saved"
	e.mustCall(t, superadmin, http.StatusBadGateway, "PATCH", path, body, nil)
	e.mustCall(t, superadmin, http.StatusOK, "GET", path, nil, &got)
	if got.Text != "All aboard, again" {
		t.Errorf("after Slack refused the edit, the text is %q", got.Text)
	}

	// Deleting it deletes the message and keeps the numbers.
	e.mustCall(t, superadmin, http.StatusOK, "DELETE", path, nil, &got)
	if got.DeletedAt == nil || len(fake.called("chat.delete")) != 1 {
		t.Errorf("deleted %+v", got)
	}
	e.mustCall(t, superadmin, http.StatusConflict, "PATCH", path, body, nil)
	e.mustCall(t, superadmin, http.StatusOK, "GET", path, nil, &got)
	if got.Engagement.Clicks != 5 || got.Engagement.Reactions != nil {
		t.Errorf("after deleting, engagement is %+v", got.Engagement)
	}
}

func TestDraftsAreDeletedForGood(t *testing.T) {
	e := newEnv(t)
	fake := e.withSlack(t)
	superadmin := e.newSuperadmin(t)
	var draft announcement
	body := announcementBody("")
	body["blocks"] = body["blocks"].([]any)[:1]
	e.mustCall(t, superadmin, http.StatusCreated, "POST", "/api/superadmin/announcements", body, &draft)
	e.mustCall(t, superadmin, http.StatusNoContent, "DELETE", "/api/superadmin/announcements/"+itoa(draft.ID), nil, nil)
	e.mustCall(t, superadmin, http.StatusNotFound, "GET", "/api/superadmin/announcements/"+itoa(draft.ID), nil, nil)
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.calls) != 0 {
		t.Errorf("deleting a draft called Slack: %+v", fake.calls)
	}
}

func TestSlackProblemsAreExplained(t *testing.T) {
	e := newEnv(t)
	superadmin := e.newSuperadmin(t)
	body := announcementBody("")
	body["blocks"] = body["blocks"].([]any)[:1]
	var draft announcement
	e.mustCall(t, superadmin, http.StatusCreated, "POST", "/api/superadmin/announcements", body, &draft)
	path := "/api/superadmin/announcements/" + itoa(draft.ID)

	// Without a bot token, drafts work and sending doesn't.
	var list struct {
		SlackProblem  string `json:"slack_problem"`
		Announcements []announcement
	}
	e.mustCall(t, superadmin, http.StatusOK, "GET", "/api/superadmin/announcements", nil, &list)
	if !strings.Contains(list.SlackProblem, "SLACK_BOT_TOKEN") || len(list.Announcements) != 1 {
		t.Errorf("without Slack, the list is %+v", list)
	}
	e.mustCall(t, superadmin, http.StatusServiceUnavailable, "POST", path+"/post", nil, nil)

	fake := e.withSlack(t)
	fake.setFail("chat.postMessage", "not_in_channel")
	fake.setFail("conversations.info", "channel_not_found")
	var failed map[string]string
	if status := e.call(t, superadmin, "POST", path+"/post", nil, &failed); status != http.StatusBadGateway || !strings.Contains(failed["error"], "/invite") {
		t.Errorf("posting where the app isn't: %d %v", status, failed)
	}
	e.mustCall(t, superadmin, http.StatusOK, "GET", "/api/superadmin/announcements", nil, &list)
	if !strings.Contains(list.SlackProblem, "/invite") {
		t.Errorf("the list says %q", list.SlackProblem)
	}
	// It's still a draft, to post once the app's been invited.
	fake.setFail("chat.postMessage", "")
	e.mustCall(t, superadmin, http.StatusOK, "POST", path+"/post", nil, nil)
}
