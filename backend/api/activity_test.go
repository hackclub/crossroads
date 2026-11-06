package api_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/scherepi/crossroads-backend/activity"
)

func TestAdminActivity(t *testing.T) {
	e := newEnv(t)
	admin := e.newAdmin(t)
	participant := e.newUser(t, "U1")
	lapsed := e.newUser(t, "U2")
	p := e.createProject(t, participant, completeProject("racer"))
	today := activity.Day(time.Now())
	for daysAgo, seconds := range map[int]int{0: 1800, 2: 7200, 10: 600} {
		if _, err := e.pool.Exec(context.Background(), "insert into project_activity (project_id, day, seconds) values ($1, $2, $3)",
			p.ID, today.AddDate(0, 0, -daysAgo), seconds); err != nil {
			t.Fatal(err)
		}
	}
	// The admin's visit doesn't count, and the lapsed participant's is only in
	// the last 30 days.
	for _, v := range []struct {
		user    int64
		daysAgo int
	}{{participant.ID, 0}, {admin.ID, 0}, {lapsed.ID, 9}} {
		if _, err := e.pool.Exec(context.Background(), "insert into platform_activity (user_id, day) values ($1, $2)",
			v.user, today.AddDate(0, 0, -v.daysAgo)); err != nil {
			t.Fatal(err)
		}
	}

	type day struct {
		Day                string `json:"day"`
		ActiveParticipants int    `json:"active_participants"`
		ActiveProjects     int    `json:"active_projects"`
		Seconds            int    `json:"seconds"`
		Visitors           int    `json:"visitors"`
		SignUps            int    `json:"sign_ups"`
		Participants       int    `json:"participants"`
	}
	var out struct {
		Days  []day `json:"days"`
		Range struct {
			Visitors           int `json:"visitors"`
			ActiveParticipants int `json:"active_participants"`
		} `json:"range"`
		MinMinutes int        `json:"min_minutes"`
		UpdatedAt  *time.Time `json:"updated_at"`
	}
	e.mustCall(t, admin, http.StatusOK, "GET", "/api/admin/activity?days=7", nil, &out)
	if len(out.Days) != 7 {
		t.Fatalf("got %d days, want 7", len(out.Days))
	}
	last := out.Days[6]
	if last.Day != today.Format(time.DateOnly) || last.ActiveParticipants != 1 || last.Seconds != 1800 {
		t.Errorf("today: got %+v", last)
	}
	if out.Days[4].Seconds != 7200 || out.Days[5].ActiveParticipants != 0 {
		t.Errorf("earlier days: got %+v", out.Days)
	}
	if out.Range.ActiveParticipants != 1 || out.Range.Visitors != 1 || out.UpdatedAt == nil {
		t.Errorf("the last 7 days: got %+v", out)
	}
	// Both participants signed up today; the admin isn't one.
	if last.Visitors != 1 || last.SignUps != 2 || last.Participants != 2 || out.Days[5].Visitors != 0 || out.Days[5].SignUps != 0 || out.Days[5].Participants != 0 {
		t.Errorf("platform today and yesterday: got %+v, %+v", last, out.Days[5])
	}

	// The lapsed participant's visit 9 days ago is in the last 30.
	e.mustCall(t, admin, http.StatusOK, "GET", "/api/admin/activity?days=30", nil, &out)
	if len(out.Days) != 30 || out.Range.Visitors != 2 || out.Range.ActiveParticipants != 1 {
		t.Errorf("the last 30 days: got %d days, %+v", len(out.Days), out.Range)
	}

	// With an hour minimum, only the day with two hours counts.
	e.mustCall(t, admin, http.StatusOK, "GET", "/api/admin/activity?days=7&min_minutes=60", nil, &out)
	if out.Days[6].ActiveParticipants != 0 || out.Days[4].ActiveParticipants != 1 || out.MinMinutes != 60 {
		t.Errorf("hour minimum: got %+v", out)
	}

	e.mustCall(t, admin, http.StatusUnprocessableEntity, "GET", "/api/admin/activity?days=0", nil, nil)
	e.mustCall(t, admin, http.StatusUnprocessableEntity, "GET", "/api/admin/activity?min_minutes=lots", nil, nil)
	e.mustCall(t, participant, http.StatusForbidden, "GET", "/api/admin/activity", nil, nil)
}
