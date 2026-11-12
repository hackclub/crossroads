package auth

import (
	"cmp"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/scherepi/crossroads-backend/db"

	"github.com/jackc/pgx/v5"
)

var devName = regexp.MustCompile(`^[a-z0-9-]{1,30}$`)

// DevLogin signs the browser in as a local test user without Hack Club Auth.
// It's only mounted when DEV_MODE=1 (see main.go), and must never be reachable
// in production: anyone could sign in as anyone.
//
// GET /dev/login?name=alice&role=participant|reviewer|admin&verified=1&next=/home
//
// Reviewers can decide every review stage unless stages= lists some of
// review, fraud and hq (stages=hq for an HQ-only reviewer).
//
// The user is created on first use with a fake Slack ID (so the dev Hackatime
// fake can find them), a test address, and verified identity unless
// verified=0. A role= is set every time, so the same name can switch roles.
// Without one, they keep the role they have, or get one an admin set for
// their email (name@dev.localhost) before they first signed in, as signing in
// with Hack Club Auth does.
func (a *Auth) DevLogin(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	name := strings.ToLower(strings.TrimSpace(q.Get("name")))
	if !devName.MatchString(name) {
		http.Error(w, "name has to be 1-30 lowercase letters, digits or dashes", http.StatusBadRequest)
		return
	}
	role := db.UserRole(q.Get("role"))
	if role != "" && !role.Valid() {
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
	if err == nil && role == "" {
		if _, err = a.Queries.ClaimRoleInvite(r.Context(), db.ClaimRoleInviteParams{Email: user.Email, ID: user.ID}); errors.Is(err, pgx.ErrNoRows) {
			err = nil
		}
	}
	if err == nil && role != "" {
		_, err = a.Queries.SetUserRole(r.Context(), db.SetUserRoleParams{ID: user.ID, Role: role})
	}
	if err == nil && role != "" {
		stages := cmp.Or(q.Get("stages"), "review,fraud,hq")
		reviewer := role == db.UserRoleReviewer
		_, err = a.Queries.SetUserPermissions(r.Context(), db.SetUserPermissionsParams{
			ID:             user.ID,
			CanReview:      reviewer && strings.Contains(stages, "review"),
			CanFraudReview: reviewer && strings.Contains(stages, "fraud"),
			CanHqReview:    reviewer && strings.Contains(stages, "hq"),
		})
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
