package slackclicks_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/scherepi/crossroads-backend/db"
	"github.com/scherepi/crossroads-backend/db/dbtest"
	"github.com/scherepi/crossroads-backend/slackclicks"
)

func TestRecordingClicks(t *testing.T) {
	pool := dbtest.New(t)
	handler := &slackclicks.Handler{Queries: db.New(pool), Secret: "s3cret"}
	post := func(auth, body string) int {
		req := httptest.NewRequest("POST", "/slack/clicks", strings.NewReader(body))
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec.Code
	}

	click := `{"slack_id": "U1", "button": "open_platform", "channel_id": "C1", "message_ts": "1.2"}`
	if status := post("", click); status != http.StatusUnauthorized {
		t.Errorf("without the secret: got %d, want 401", status)
	}
	if status := post("Bearer wrong", click); status != http.StatusUnauthorized {
		t.Errorf("with the wrong secret: got %d, want 401", status)
	}
	if status := post("Bearer s3cret", `{"slack_id": "U1"}`); status != http.StatusUnprocessableEntity {
		t.Errorf("without a button: got %d, want 422", status)
	}
	if status := post("Bearer s3cret", click); status != http.StatusNoContent {
		t.Fatalf("recording: got %d, want 204", status)
	}

	var slackID, button, channelID string
	if err := pool.QueryRow(context.Background(), "select slack_id, button, channel_id from slack_clicks").Scan(&slackID, &button, &channelID); err != nil {
		t.Fatal(err)
	}
	if slackID != "U1" || button != "open_platform" || channelID != "C1" {
		t.Errorf("recorded %s %s %s", slackID, button, channelID)
	}
}
