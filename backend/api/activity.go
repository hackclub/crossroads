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
		ShippedSeconds     int64  `json:"shipped_seconds"`
	}
	out := make([]dayJSON, len(rows))
	for i, row := range rows {
		out[i] = dayJSON{row.Day.Format(time.DateOnly), row.ActiveParticipants, row.ActiveProjects, row.Seconds, row.Visitors, row.SignUps, row.Participants, row.ShippedSeconds}
	}
	flow, err := a.Queries.HoursFlow(r.Context())
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	people, err := a.Queries.ParticipantFunnel(r.Context(), goalHours*3600)
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	type rangeJSON struct {
		Visitors           int32 `json:"visitors"`
		ActiveParticipants int32 `json:"active_participants"`
	}
	respond.JSON(w, http.StatusOK, struct {
		Days       []dayJSON    `json:"days"`
		Range      rangeJSON    `json:"range"`
		Hours      hoursFlow    `json:"hours"`
		People     []funnelStep `json:"people"`
		MinMinutes int          `json:"min_minutes"`
		UpdatedAt  *time.Time   `json:"updated_at"`
	}{out, rangeJSON{visitors, active}, hoursFlowFrom(flow), peopleFunnel(people), minMinutes, updatedAt})
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

// hoursFlow is where every hour participants logged has gone, all time, as
// a Sankey (the admin Stats tab, like Stardance's hour funnel): nodes in
// columns left to right, and links that carry hours and ships between them.
// kind is what the chart colours: kept (flows on), held (waiting on
// someone) or lost.
type hoursFlow struct {
	Nodes []flowNode `json:"nodes"`
	Links []flowLink `json:"links"`
}

type flowNode struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	Note    string `json:"note"`
	Column  int    `json:"column"`
	Kind    string `json:"kind"`
	Seconds int64  `json:"seconds"`
	Ships   int64  `json:"ships"`
	// Partial nodes take hours but not whole ships (a cut leaves the ship
	// where it is), so Ships is how many they affect, and the ships view
	// leaves them out.
	Partial bool `json:"partial"`
}

type flowLink struct {
	Source  string `json:"source"`
	Target  string `json:"target"`
	Seconds int64  `json:"seconds"`
	Ships   int64  `json:"ships"`
}

func hoursFlowFrom(f db.HoursFlowRow) hoursFlow {
	notShipped := max(0, f.LoggedSeconds-f.ShippedSeconds)
	waitingUnified := f.ApprovedSeconds - f.UnifiedSeconds
	nodes := []flowNode{
		{"logged", "Logged", "Hackatime time on linked projects", 0, "kept", f.LoggedSeconds, 0, false},
		{"shipped", "Shipped", "", 1, "kept", f.ShippedSeconds, f.ShippedShips, false},
		{"not_shipped", "Not shipped yet", "", 1, "held", notShipped, 0, false},
		{"signed_off", "Signed off by HQ", "what the ships claimed", 2, "kept", f.SignedOffSeconds, f.SignedOffShips, false},
		{"review", "Waiting for a first review", "", 2, "held", f.ReviewSeconds, f.ReviewShips, false},
		{"fraud", "In the fraud check", "", 2, "held", f.FraudSeconds, f.FraudShips, false},
		{"hq", "Waiting for HQ", "", 2, "held", f.HqSeconds, f.HqShips, false},
		{"soft_rejected", "Rejected, soft", "not shipped again yet", 2, "held", f.SoftRejectedSeconds, f.SoftRejectedShips, false},
		{"hard_rejected", "Rejected, hard", "blocked from shipping again", 2, "lost", f.HardRejectedSeconds, f.HardRejectedShips, false},
		{"added", "Added over Hackatime", "approved with a reason", 2, "kept", f.AddedSeconds, 0, true},
		{"approved", "Approved", "", 3, "kept", f.ApprovedSeconds, f.SignedOffShips, false},
		{"cut", "Cut by reviewers", "", 3, "lost", f.CutSeconds, f.CutShips, true},
		{"unified", "In the Unified YSWS DB", "", 4, "kept", f.UnifiedSeconds, f.UnifiedShips, false},
		{"waiting_unified", "Not in Unified yet", "waiting for YSWS eligibility or the sync", 4, "held", waitingUnified, f.SignedOffShips - f.UnifiedShips, false},
	}
	links := []flowLink{
		{"logged", "shipped", min(f.ShippedSeconds, f.LoggedSeconds), 0},
		{"logged", "not_shipped", notShipped, 0},
		{"shipped", "signed_off", f.SignedOffSeconds, f.SignedOffShips},
		{"shipped", "review", f.ReviewSeconds, f.ReviewShips},
		{"shipped", "fraud", f.FraudSeconds, f.FraudShips},
		{"shipped", "hq", f.HqSeconds, f.HqShips},
		{"shipped", "soft_rejected", f.SoftRejectedSeconds, f.SoftRejectedShips},
		{"shipped", "hard_rejected", f.HardRejectedSeconds, f.HardRejectedShips},
		{"signed_off", "approved", f.SignedOffSeconds - f.CutSeconds, f.SignedOffShips},
		{"signed_off", "cut", f.CutSeconds, 0},
		{"added", "approved", f.AddedSeconds, 0},
		{"approved", "unified", f.UnifiedSeconds, f.UnifiedShips},
		{"approved", "waiting_unified", waitingUnified, f.SignedOffShips - f.UnifiedShips},
	}
	return hoursFlow{nodes, links}
}

// goalHours is the approved hours that qualify someone for the trip (the
// platform's GOAL_HOURS).
const goalHours = 60

// funnelStep is one step of the participant funnel: how many participants
// ever got that far.
type funnelStep struct {
	Key    string `json:"key"`
	Label  string `json:"label"`
	People int64  `json:"people"`
}

func peopleFunnel(f db.ParticipantFunnelRow) []funnelStep {
	return []funnelStep{
		{"signed_up", "Signed up", f.SignedUp},
		{"made_project", "Made a project", f.MadeProject},
		{"linked_hackatime", "Linked Hackatime", f.LinkedHackatime},
		{"logged_time", "Logged time", f.LoggedTime},
		{"shipped", "Shipped", f.Shipped},
		{"approved", "Got hours signed off", f.Approved},
		{"reached_goal", fmt.Sprintf("Reached %d approved hours", goalHours), f.ReachedGoal},
		{"rail_pass", "Bought the rail pass", f.RailPass},
	}
}
