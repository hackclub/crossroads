package api

// Dev-mode helpers, only mounted when DEV_MODE=1 (see main.go). Remove with
// dev mode (planning/DEV_MODE.md).

import (
	"net/http"
	"slices"

	"github.com/scherepi/crossroads-backend/auth"
	"github.com/scherepi/crossroads-backend/db"
	"github.com/scherepi/crossroads-backend/respond"
)

// DevGrantBox is POST /dev/box: gives the signed-in person one box of the
// first box type, to open on the Boxes page. It's recorded as granted by
// themselves, like an admin gift. With {"project": true} it comes with their
// newest project instead, like a per-project milestone's box (of that
// milestone's box type, if there is one), and takes over Home. → 201 Box.
func (a *API) DevGrantBox(w http.ResponseWriter, r *http.Request) {
	user := auth.User(r.Context())
	var body struct {
		Project bool `json:"project"`
	}
	if !decode(w, r, &body) {
		return
	}
	types, err := a.Queries.ListBoxTypes(r.Context())
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	if len(types) == 0 {
		respond.Error(w, http.StatusConflict, "There are no box types yet. Make one in Admin → Boxes first.")
		return
	}
	params := db.GrantBoxParams{UserID: user.ID, BoxTypeID: types[0].ID, GrantedBy: &user.ID}
	if body.Project {
		projects, err := a.Queries.ListProjects(r.Context(), user.ID)
		if err != nil {
			respond.ServerError(w, r, err)
			return
		}
		if len(projects) == 0 {
			respond.Error(w, http.StatusConflict, "Make a project first: the box comes with one.")
			return
		}
		params.ProjectID = &projects[0].ID
		milestones, err := a.Queries.ListMilestones(r.Context())
		if err != nil {
			respond.ServerError(w, r, err)
			return
		}
		if i := slices.IndexFunc(milestones, func(m db.Milestone) bool { return m.PerProject }); i >= 0 {
			params.BoxTypeID = milestones[i].BoxTypeID
		}
	}
	box, err := a.Queries.GrantBox(r.Context(), params)
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toBoxJSON(box, nil))
}
