package api

import (
	"net/http"
	"strings"
	"unicode"

	"github.com/scherepi/crossroads-backend/auth"
	"github.com/scherepi/crossroads-backend/db"
	"github.com/scherepi/crossroads-backend/respond"
)

// maxNameLength is the longest name someone can choose, in characters.
const maxNameLength = 50

// updateMe is PATCH /me, with any of:
//   - "name": changes the name the site shows, which signing in then leaves
//     alone. null goes back to the name on their Hack Club account. Their
//     legal first and last names, which the Unified YSWS DB and fulfilment
//     use, don't change.
//   - "dms_enabled": whether Tickets, Please DMs them about their projects
//     (dms.go).
//   - "on_landing_page": whether they ride the landing page's trains
//     (riders.go).
func (a *API) updateMe(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := auth.User(ctx)
	var body struct {
		Name          optional[string] `json:"name"`
		DMsEnabled    *bool            `json:"dms_enabled"`
		OnLandingPage *bool            `json:"on_landing_page"`
	}
	if !decode(w, r, &body) {
		return
	}
	settings := body.DMsEnabled != nil || body.OnLandingPage != nil
	if !body.Name.Set && !settings {
		respond.Error(w, http.StatusUnprocessableEntity, "Send what to change: name, dms_enabled or on_landing_page.")
		return
	}
	params := db.SetUserNameParams{ID: user.ID}
	switch {
	case !body.Name.Set:
	case body.Name.Value == nil:
		params.Name = strings.TrimSpace(user.FirstName + " " + user.LastName)
		if params.Name == "" {
			params.Name = user.Name
		}
	default:
		name, problem := cleanName(*body.Name.Value)
		if problem != "" {
			respond.Error(w, http.StatusUnprocessableEntity, problem)
			return
		}
		params.Name, params.NameCustomized = name, true
	}

	tx, err := a.Pool.Begin(ctx)
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	defer tx.Rollback(ctx)
	q := a.Queries.WithTx(tx)
	updated := *user
	if body.Name.Set {
		if updated, err = q.SetUserName(ctx, params); err != nil {
			respond.ServerError(w, r, err)
			return
		}
	}
	if settings {
		if updated, err = q.SetUserSettings(ctx, db.SetUserSettingsParams{
			ID: user.ID, DmsEnabled: body.DMsEnabled, OnLandingPage: body.OnLandingPage,
		}); err != nil {
			respond.ServerError(w, r, err)
			return
		}
	}
	if err := tx.Commit(ctx); err != nil {
		respond.ServerError(w, r, err)
		return
	}
	// Saying no takes them off the trains now, not at the next refresh.
	if body.OnLandingPage != nil && !*body.OnLandingPage && user.SlackID != nil {
		a.dropRider(*user.SlackID)
	}
	respond.JSON(w, http.StatusOK, struct {
		auth.MeJSON
		StampsPerHour int `json:"stamps_per_hour"`
	}{auth.ToMeJSON(&updated), a.StampsPerHour})
}

// cleanName trims a chosen name and runs of spaces inside it, and says what's
// wrong with it, if anything.
func cleanName(name string) (string, string) {
	name = strings.Join(strings.Fields(name), " ")
	switch {
	case name == "":
		return "", "Your name can't be empty."
	case len([]rune(name)) > maxNameLength:
		return "", "Keep your name under 50 characters."
	case strings.IndexFunc(name, func(r rune) bool { return unicode.IsControl(r) || unicode.Is(unicode.Cf, r) }) >= 0:
		return "", "Your name has a character in it we can't show."
	}
	return name, ""
}
