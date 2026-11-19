package api_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

type meResponse struct {
	Name           string `json:"name"`
	NameCustomized bool   `json:"name_customized"`
	Error          string `json:"error"`
}

func TestChangingYourName(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	user := e.newUser(t, "UMAX")
	if _, err := e.pool.Exec(ctx, "update users set first_name = 'Maximilian', last_name = 'Hurley', name = 'Maximilian Hurley' where id = $1", user.ID); err != nil {
		t.Fatal(err)
	}
	user = e.reload(t, user)

	var me meResponse
	e.mustCall(t, user, http.StatusOK, "PATCH", "/api/me", map[string]any{"name": "  Captain   Max "}, &me)
	if me.Name != "Captain Max" || !me.NameCustomized {
		t.Fatalf("after changing it: %+v", me)
	}
	// The legal name, for the Unified YSWS DB and fulfilment, doesn't change.
	var first, last string
	if err := e.pool.QueryRow(ctx, "select first_name, last_name from users where id = $1", user.ID).Scan(&first, &last); err != nil {
		t.Fatal(err)
	}
	if first != "Maximilian" || last != "Hurley" {
		t.Errorf("legal name changed to %q %q", first, last)
	}

	// Signing in again leaves a chosen name alone.
	if again := e.newUser(t, "UMAX"); again.Name != "Captain Max" {
		t.Errorf("signing in again changed the name to %q", again.Name)
	}

	// Going back uses the Hack Club name, and signing in can change it again.
	if _, err := e.pool.Exec(ctx, "update users set first_name = 'Maximilian', last_name = 'Hurley' where id = $1", user.ID); err != nil {
		t.Fatal(err)
	}
	user = e.reload(t, user)
	e.mustCall(t, user, http.StatusOK, "PATCH", "/api/me", map[string]any{"name": nil}, &me)
	if me.Name != "Maximilian Hurley" || me.NameCustomized {
		t.Fatalf("after going back: %+v", me)
	}
	if again := e.newUser(t, "UMAX"); again.Name != "Test" {
		t.Errorf("signing in after going back kept %q, want Hack Club's name", again.Name)
	}

	for _, body := range []map[string]any{
		{"name": "   "},
		{"name": strings.Repeat("a", 51)},
		{"name": "Max\u202eevil"},
		{},
	} {
		if status := e.call(t, user, "PATCH", "/api/me", body, &me); status != http.StatusUnprocessableEntity {
			t.Errorf("%q: got %d, want 422", body["name"], status)
		}
	}
	// Line breaks and runs of spaces become single spaces.
	e.mustCall(t, user, http.StatusOK, "PATCH", "/api/me", map[string]any{"name": "Max\n\tHurley"}, &me)
	if me.Name != "Max Hurley" {
		t.Errorf("whitespace: got %q", me.Name)
	}
	if status := e.call(t, user, "PATCH", "/api/me", map[string]any{"name": strings.Repeat("é", 50)}, nil); status != http.StatusOK {
		t.Errorf("50 accented letters: got %d, want 200", status)
	}
	if status := e.call(t, nil, "PATCH", "/api/me", map[string]any{"name": "Nobody"}, nil); status != http.StatusUnauthorized {
		t.Errorf("signed out: got %d, want 401", status)
	}
}

func TestSettings(t *testing.T) {
	e := newEnv(t)
	user := e.newUser(t, "U1")
	type settings struct {
		Name          string `json:"name"`
		DMsEnabled    bool   `json:"dms_enabled"`
		OnLandingPage *bool  `json:"on_landing_page"`
	}

	// DMs are on until they're turned off, and nobody's on the landing page
	// until they've been asked.
	var me settings
	e.mustCall(t, user, http.StatusOK, "GET", "/api/me", nil, &me)
	if !me.DMsEnabled || me.OnLandingPage != nil {
		t.Fatalf("to start with: %+v", me)
	}

	// Each changes alone, and leaves the name alone.
	e.mustCall(t, user, http.StatusOK, "PATCH", "/api/me", map[string]any{"dms_enabled": false}, &me)
	if me.DMsEnabled || me.OnLandingPage != nil || me.Name != "Test" {
		t.Fatalf("after turning DMs off: %+v", me)
	}
	user = e.reload(t, user)
	e.mustCall(t, user, http.StatusOK, "PATCH", "/api/me", map[string]any{"on_landing_page": true}, &me)
	if me.DMsEnabled || me.OnLandingPage == nil || !*me.OnLandingPage {
		t.Fatalf("after saying yes: %+v", me)
	}
	// All at once, too.
	user = e.reload(t, user)
	e.mustCall(t, user, http.StatusOK, "PATCH", "/api/me", map[string]any{"name": "Max", "dms_enabled": true, "on_landing_page": false}, &me)
	if me.Name != "Max" || !me.DMsEnabled || me.OnLandingPage == nil || *me.OnLandingPage {
		t.Fatalf("after changing everything: %+v", me)
	}
	// A bad name changes nothing.
	user = e.reload(t, user)
	if status := e.call(t, user, "PATCH", "/api/me", map[string]any{"name": " ", "dms_enabled": false}, nil); status != http.StatusUnprocessableEntity {
		t.Errorf("a bad name with a setting: got %d, want 422", status)
	}
	e.mustCall(t, e.reload(t, user), http.StatusOK, "GET", "/api/me", nil, &me)
	if !me.DMsEnabled {
		t.Error("a bad name still turned DMs off")
	}
	if status := e.call(t, user, "PATCH", "/api/me", map[string]any{"dms_enabled": "no"}, nil); status != http.StatusBadRequest {
		t.Errorf("a setting that isn't true or false: got %d, want 400", status)
	}
}
