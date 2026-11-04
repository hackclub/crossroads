package auth

import (
	"encoding/json"
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/scherepi/crossroads-backend/db"
)

var devName = regexp.MustCompile(`^[a-z0-9-]{1,30}$`)

// DevLogin signs the browser in as a local test user without Hack Club Auth.
// It's only mounted when DEV_MODE=1 (see main.go), and must never be reachable
// in production: anyone could sign in as anyone.
//
// GET /dev/login?name=alice&role=participant|reviewer|admin&verified=1&next=/home
//
// The user is created on first use with a fake Slack ID (so the dev Hackatime
// fake can find them), a test address, and verified identity unless
// verified=0. The role is set every time, so the same name can switch roles.
func (a *Auth) DevLogin(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	name := strings.ToLower(strings.TrimSpace(q.Get("name")))
	if !devName.MatchString(name) {
		http.Error(w, "name has to be 1-30 lowercase letters, digits or dashes", http.StatusBadRequest)
		return
	}
	role := db.UserRole(q.Get("role"))
	if role == "" {
		role = db.UserRoleParticipant
	}
	if !role.Valid() {
		http.Error(w, "role has to be participant, reviewer or admin", http.StatusBadRequest)
		return
	}
	verified := q.Get("verified") != "0"

	slackID := "UDEV" + strings.ToUpper(strings.ReplaceAll(name, "-", ""))
	address, _ := json.Marshal(db.Address{
		Name: name + " (test)", Line1: "1 Test Street", City: "Testville",
		State: "VT", PostalCode: "05401", Country: "US",
	})
	status := "needs_submission"
	if verified {
		status = "verified"
	}
	first := strings.ToUpper(name[:1]) + name[1:]
	user, err := a.Queries.UpsertUserFromHCA(r.Context(), db.UpsertUserFromHCAParams{
		HcaID:              "dev-" + name,
		SlackID:            &slackID,
		Email:              name + "@dev.localhost",
		Name:               first + " (dev)",
		FirstName:          first,
		LastName:           "(dev)",
		Address:            address,
		VerificationStatus: status,
		YswsEligible:       verified,
	})
	if err == nil {
		_, err = a.Queries.SetUserRole(r.Context(), db.SetUserRoleParams{ID: user.ID, Role: role})
	}
	var token string
	if err == nil {
		token, err = a.startSession(r.Context(), user.ID)
	}
	if err != nil {
		log.Printf("dev login as %s: %v", name, err)
		http.Error(w, "dev login failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   int(sessionLength.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, safeNext(q.Get("next")), http.StatusFound)
}
