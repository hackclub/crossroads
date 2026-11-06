package api

// The community goal on the home page ("Repair Bridge"): the in-person event
// only runs if participants together ship enough hours between launch and
// the deadline. A ship counts as soon as it's submitted, without waiting for
// review, and stops counting if it's rejected.

import (
	"net/http"
	"time"

	"github.com/scherepi/crossroads-backend/db"
	"github.com/scherepi/crossroads-backend/respond"
)

// CommunityGoal is the target and when it closes. Time counts from
// API.HoursStart.
type CommunityGoal struct {
	Seconds int64     // how much time the community needs
	Ends    time.Time // when it stops counting
}

// community is GET /community: hours shipped from HoursStart to the deadline.
func (a *API) community(w http.ResponseWriter, r *http.Request) {
	seconds, err := a.Queries.CommunitySeconds(r.Context(), db.CommunitySecondsParams{StartsAt: a.HoursStart, EndsAt: a.CommunityGoal.Ends})
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	respond.JSON(w, http.StatusOK, struct {
		Seconds     int64     `json:"seconds"`      // shipped so far
		GoalSeconds int64     `json:"goal_seconds"` // what's needed
		StartsAt    time.Time `json:"starts_at"`
		EndsAt      time.Time `json:"ends_at"`
	}{seconds, a.CommunityGoal.Seconds, a.HoursStart, a.CommunityGoal.Ends})
}
