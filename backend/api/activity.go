package api

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/scherepi/crossroads-backend/activity"
	"github.com/scherepi/crossroads-backend/db"
	"github.com/scherepi/crossroads-backend/respond"
)

// adminActivity is how much participants work on their projects, from the
// Hackatime time the activity poller records per project per UTC day, and
// how many use the platform, from their signed-in requests. A participant is
// active on a day if their projects got at least min_minutes between them;
// the last day is today. range counts each participant once over all the
// days returned.
//
// GET /api/admin/activity?days=30&min_minutes=0
func (a *API) adminActivity(w http.ResponseWriter, r *http.Request) {
	days, ok := queryInt(w, r, "days", 30, 1, 366)
	if !ok {
		return
	}
	minMinutes, ok := queryInt(w, r, "min_minutes", 0, 0, 24*60)
	if !ok {
		return
	}
	minSeconds := int32(minMinutes * 60)
	today := activity.Day(time.Now())
	start := today.AddDate(0, 0, 1-days)
	// Nothing counts before Hackatime time does, so those days aren't shown.
	if first := activity.Day(a.HoursStart); start.Before(first) && !first.After(today) {
		start = first
	}

	rows, err := a.Queries.DailyActivity(r.Context(), db.DailyActivityParams{
		StartDay: start, EndDay: today, MinSeconds: minSeconds,
	})
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	active, err := a.Queries.CountActiveParticipants(r.Context(), db.CountActiveParticipantsParams{
		StartDay: start, EndDay: today, MinSeconds: minSeconds,
	})
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	visitors, err := a.Queries.CountVisitors(r.Context(), db.CountVisitorsParams{StartDay: start, EndDay: today})
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	var updatedAt *time.Time
	switch t, err := a.Queries.LastProjectActivityUpdate(r.Context()); {
	case err == nil:
		updatedAt = &t
	case !errors.Is(err, pgx.ErrNoRows):
		respond.ServerError(w, r, err)
		return
	}

	type dayJSON struct {
		Day                string `json:"day"`
		ActiveParticipants int32  `json:"active_participants"`
		ActiveProjects     int32  `json:"active_projects"`
		Seconds            int64  `json:"seconds"`
		Visitors           int32  `json:"visitors"`
		SignUps            int32  `json:"sign_ups"`
		Participants       int32  `json:"participants"`
	}
	out := make([]dayJSON, len(rows))
	for i, row := range rows {
		out[i] = dayJSON{row.Day.Format(time.DateOnly), row.ActiveParticipants, row.ActiveProjects, row.Seconds, row.Visitors, row.SignUps, row.Participants}
	}
	type rangeJSON struct {
		Visitors           int32 `json:"visitors"`
		ActiveParticipants int32 `json:"active_participants"`
	}
	respond.JSON(w, http.StatusOK, struct {
		Days       []dayJSON  `json:"days"`
		Range      rangeJSON  `json:"range"`
		MinMinutes int        `json:"min_minutes"`
		UpdatedAt  *time.Time `json:"updated_at"`
	}{out, rangeJSON{visitors, active}, minMinutes, updatedAt})
}

// queryInt reads an optional whole-number query parameter from lo to hi,
// responding 422 if it's anything else.
func queryInt(w http.ResponseWriter, r *http.Request, name string, fallback, lo, hi int) (int, bool) {
	s := r.URL.Query().Get(name)
	if s == "" {
		return fallback, true
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < lo || n > hi {
		respond.Error(w, http.StatusUnprocessableEntity, fmt.Sprintf("%s has to be a whole number from %d to %d.", name, lo, hi))
		return 0, false
	}
	return n, true
}
