package api_test

import (
	"net/http"
	"testing"

	"github.com/scherepi/crossroads-backend/db"
)

type claim struct {
	SpecialStamp   specialStamp `json:"special_stamp"`
	AlreadyClaimed bool         `json:"already_claimed"`
	Error          string       `json:"error"`
}

func (e *env) claim(t *testing.T, user *db.User, code string) (int, claim) {
	t.Helper()
	var out claim
	status := e.call(t, user, "POST", "/api/special-stamps/claims", map[string]any{"code": code}, &out)
	return status, out
}

func TestClaimingStamps(t *testing.T) {
	e := newEnv(t)
	admin := e.newAdmin(t)
	first := e.newUser(t, "U1")
	second := e.newUser(t, "U2")

	var announcements stampType
	e.mustCall(t, admin, http.StatusCreated, "POST", "/api/admin/special-stamp-types",
		map[string]any{"name": "Announcements", "rarity": "rare", "supply_cap": 1, "claim_code": " Heard-It-First "}, &announcements)

	if status, got := e.claim(t, first, "nope"); status != http.StatusNotFound {
		t.Errorf("a code nobody has: got %d %q, want 404", status, got.Error)
	}
	status, got := e.claim(t, first, "heard-it-first")
	if status != http.StatusCreated || got.AlreadyClaimed || got.SpecialStamp.Type.ID != announcements.ID {
		t.Fatalf("claiming: got %d %+v", status, got)
	}
	// Claiming again gives back the same stamp, and doesn't use up supply.
	if status, again := e.claim(t, first, "HEARD-IT-FIRST"); status != http.StatusOK || !again.AlreadyClaimed || again.SpecialStamp.ID != got.SpecialStamp.ID {
		t.Errorf("claiming again: got %d %+v, want 200 with the same stamp", status, again)
	}
	if status, got := e.claim(t, second, "heard-it-first"); status != http.StatusConflict {
		t.Errorf("past the supply cap: got %d %q, want 409", status, got.Error)
	}

	var mine []specialStamp
	e.mustCall(t, first, http.StatusOK, "GET", "/api/special-stamps", nil, &mine)
	if len(mine) != 1 || mine[0].ID != got.SpecialStamp.ID {
		t.Errorf("their stamps are %+v", mine)
	}

	// Codes are unique and URL-safe, and clearing one stops claiming.
	var other stampType
	e.mustCall(t, admin, http.StatusCreated, "POST", "/api/admin/special-stamp-types", map[string]any{"name": "Other", "rarity": "common"}, &other)
	path := "/api/admin/special-stamp-types/" + itoa(other.ID)
	e.mustCall(t, admin, http.StatusConflict, "PATCH", path, map[string]any{"claim_code": "heard-it-first"}, nil)
	e.mustCall(t, admin, http.StatusUnprocessableEntity, "PATCH", path, map[string]any{"claim_code": "no spaces"}, nil)
	e.mustCall(t, admin, http.StatusOK, "PATCH", "/api/admin/special-stamp-types/"+itoa(announcements.ID), map[string]any{"claim_code": ""}, nil)
	if status, _ := e.claim(t, first, "heard-it-first"); status != http.StatusNotFound {
		t.Errorf("after clearing the code: got %d, want 404", status)
	}
}
