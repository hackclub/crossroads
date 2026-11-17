package api

import (
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/scherepi/crossroads-backend/db"
	"github.com/scherepi/crossroads-backend/respond"
)

// auditKinds are the kinds of event in the audit log (migration 24 says what
// each is), and the groups they're in, for checking filters.
func auditKinds() []string {
	kinds := []string{"request", "sign_in", "special_stamp", "special_stamp.issued", "special_stamp.used",
		"box", "box.granted", "stamps", "role", "role.changed", "order"}
	for _, kind := range db.AllStampEntryKindValues() {
		kinds = append(kinds, "stamps."+string(kind))
	}
	for _, status := range db.AllOrderStatusValues() {
		kinds = append(kinds, "order."+string(status))
	}
	return kinds
}

// superadminAuditLog is GET /superadmin/audit-log: everything that's
// happened on the platform, newest first, a page at a time. ?kind= (as many
// as wanted) keeps whole kinds (stamps.payout) or groups (stamps); ?user=ID
// keeps what someone did or what happened to them; ?before=ID is the next
// page, after the last ID of this one; ?limit= is up to 500 (100).
func (a *API) superadminAuditLog(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	limit, ok := queryInt(w, r, "limit", 100, 1, 500)
	if !ok {
		return
	}
	params := db.ListAuditEventsParams{Kinds: []string{}, MaxRows: int32(limit)}
	known := auditKinds()
	for _, kind := range query["kind"] {
		for kind := range strings.SplitSeq(kind, ",") {
			if !slices.Contains(known, kind) {
				respond.Error(w, http.StatusUnprocessableEntity, "kind has to be one of "+strings.Join(known, ", ")+".")
				return
			}
			params.Kinds = append(params.Kinds, kind)
		}
	}
	for name, field := range map[string]**int64{"user": &params.UserID, "before": &params.BeforeID} {
		if s := query.Get(name); s != "" {
			id, err := strconv.ParseInt(s, 10, 64)
			if err != nil {
				respond.Error(w, http.StatusUnprocessableEntity, name+" has to be an ID.")
				return
			}
			*field = &id
		}
	}
	rows, err := a.Queries.ListAuditEvents(r.Context(), params)
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	type eventJSON struct {
		ID        int64           `json:"id"`
		Kind      string          `json:"kind"`
		At        time.Time       `json:"at"`
		ActorID   *int64          `json:"actor_id"` // who did it; null for nobody signed in or the server
		ActorName *string         `json:"actor_name"`
		UserID    *int64          `json:"user_id"` // who it happened to
		UserName  *string         `json:"user_name"`
		Details   json.RawMessage `json:"details"`
	}
	out := []eventJSON{}
	for _, row := range rows {
		out = append(out, eventJSON{row.ID, row.Kind, row.At, row.ActorID, row.ActorName, row.UserID, row.UserName, row.Details})
	}
	respond.JSON(w, http.StatusOK, out)
}
