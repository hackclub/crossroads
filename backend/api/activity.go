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
// the last day is today.
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
	activeOver := func(n int) (int32, error) {
		return a.Queries.CountActiveParticipants(r.Context(), db.CountActiveParticipantsParams{
			StartDay: today.AddDate(0, 0, 1-n), EndDay: today, MinSeconds: minSeconds,
		})
	}
	week, err := activeOver(7)
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	month, err := activeOver(30)
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	visitorsOver := func(n int) (int32, error) {
		return a.Queries.CountVisitors(r.Context(), db.CountVisitorsParams{StartDay: today.AddDate(0, 0, 1-n), EndDay: today})
	}
	visitorsWeek, err := visitorsOver(7)
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	visitorsMonth, err := visitorsOver(30)
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
	}
	out := make([]dayJSON, len(rows))
	for i, row := range rows {
		out[i] = dayJSON{row.Day.Format(time.DateOnly), row.ActiveParticipants, row.ActiveProjects, row.Seconds, row.Visitors, row.SignUps}
	}
	respond.JSON(w, http.StatusOK, struct {
		Days               []dayJSON  `json:"days"`
		Last7Days          int32      `json:"last_7_days"`
		Last30Days         int32      `json:"last_30_days"`
		VisitorsLast7Days  int32      `json:"visitors_last_7_days"`
		VisitorsLast30Days int32      `json:"visitors_last_30_days"`
		MinMinutes         int        `json:"min_minutes"`
		UpdatedAt          *time.Time `json:"updated_at"`
	}{out, week, month, visitorsWeek, visitorsMonth, minMinutes, updatedAt})
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
