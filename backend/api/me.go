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

// updateMe is PATCH /me {"name": "Max"}: changes the name the site shows,
// which signing in then leaves alone. {"name": null} goes back to the name
// on their Hack Club account. Their legal first and last names, which the
// Unified YSWS DB and fulfilment use, don't change.
func (a *API) updateMe(w http.ResponseWriter, r *http.Request) {
	user := auth.User(r.Context())
	var body struct {
		Name optional[string] `json:"name"`
	}
	if !decode(w, r, &body) {
		return
	}
	if !body.Name.Set {
		respond.Error(w, http.StatusUnprocessableEntity, "Send the name to change to, or null for your Hack Club name.")
		return
	}
	params := db.SetUserNameParams{ID: user.ID}
	if body.Name.Value == nil {
		params.Name = strings.TrimSpace(user.FirstName + " " + user.LastName)
		if params.Name == "" {
			params.Name = user.Name
		}
	} else {
		name, problem := cleanName(*body.Name.Value)
		if problem != "" {
			respond.Error(w, http.StatusUnprocessableEntity, problem)
			return
		}
		params.Name, params.NameCustomized = name, true
	}
	updated, err := a.Queries.SetUserName(r.Context(), params)
	if err != nil {
		respond.ServerError(w, r, err)
		return
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
