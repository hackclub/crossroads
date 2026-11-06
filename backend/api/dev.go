package api

// Dev-mode helpers, only mounted when DEV_MODE=1 (see main.go). Remove with
// dev mode (planning/DEV_MODE.md).

import (
	"net/http"

	"github.com/scherepi/crossroads-backend/auth"
	"github.com/scherepi/crossroads-backend/db"
	"github.com/scherepi/crossroads-backend/respond"
)

// DevGrantBox is POST /dev/box: gives the signed-in person one box of the
// first box type, to open on the Boxes page. It's recorded as granted by
// themselves, like an admin gift. → 201 Box.
func (a *API) DevGrantBox(w http.ResponseWriter, r *http.Request) {
	user := auth.User(r.Context())
	types, err := a.Queries.ListBoxTypes(r.Context())
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	if len(types) == 0 {
		respond.Error(w, http.StatusConflict, "There are no box types yet. Make one in Admin → Boxes first.")
		return
	}
	box, err := a.Queries.GrantBox(r.Context(), db.GrantBoxParams{UserID: user.ID, BoxTypeID: types[0].ID, GrantedBy: &user.ID})
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toBoxJSON(box, nil))
}
