package api_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/scherepi/crossroads-backend/api"
)

func TestCommunityGoal(t *testing.T) {
	e := newEnv(t)
	e.api.HoursStart = time.Date(2026, time.November, 3, 0, 0, 0, 0, time.UTC)
	e.api.CommunityGoal = api.CommunityGoal{
		Seconds: 200 * 3600,
		Ends:    time.Date(2026, time.November, 15, 23, 59, 0, 0, time.FixedZone("EST", -5*3600)),
	}
	user := e.newUser(t, "U1")
	reviewer := e.newReviewer(t)
	ctx := context.Background()
	project := func(owner int64, name string) (id int64) {
		if err := e.pool.QueryRow(ctx, "insert into projects (user_id, name) values ($1, $2) returning id", owner, name).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	// One project per ship, since a project can only have one in review.
	for _, ship := range []struct {
		owner   int64
		status  string
		seconds int
		at      string
	}{
		{user.ID, "pending", 3600, "2026-11-04T12:00:00Z"},      // in review: counts without waiting
		{user.ID, "approved", 1800, "2026-11-10T12:00:00Z"},     // counts
		{user.ID, "rejected", 7200, "2026-11-11T12:00:00Z"},     // rejected: doesn't
		{user.ID, "pending", 900, "2026-11-02T23:00:00Z"},       // before launch: doesn't
		{user.ID, "pending", 600, "2026-11-16T04:58:00Z"},       // 11:58 PM EST on the 15th: counts
		{user.ID, "pending", 300, "2026-11-16T05:00:00Z"},       // after the deadline: doesn't
		{reviewer.ID, "pending", 36000, "2026-11-10T12:00:00Z"}, // not a participant: doesn't
	} {
		if _, err := e.pool.Exec(ctx, `insert into submissions (project_id, hackatime_projects, status, hackatime_seconds, new_seconds, submitted_at,
			approved_seconds, reviewer_id, reviewed_at, feedback)
			values ($1, '{}', $2::submission_status, $3::integer, $3::integer, $4::timestamptz,
			case when $2::text = 'approved' then $3::integer end,
			case when $2::text <> 'pending' then $5::bigint end,
			case when $2::text <> 'pending' then now() end,
			case when $2::text = 'rejected' then 'no' end)`,
			project(ship.owner, ship.at), ship.status, ship.seconds, ship.at, reviewer.ID); err != nil {
			t.Fatal(err)
		}
	}
	var got struct {
		Seconds     int64 `json:"seconds"`
		GoalSeconds int64 `json:"goal_seconds"`
	}
	e.mustCall(t, user, http.StatusOK, "GET", "/api/community", nil, &got)
	if got.Seconds != 3600+1800+600 || got.GoalSeconds != 720000 {
		t.Errorf("got %+v, want 6000 of 720000", got)
	}
}
