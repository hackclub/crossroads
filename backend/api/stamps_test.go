package api_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/scherepi/crossroads-backend/db"
)

// giveStamp gives user a special stamp of a new type with the given effect.
func (e *env) giveStamp(t *testing.T, admin, user *db.User, effect string) int64 {
	t.Helper()
	ctx := context.Background()
	var typeID, stampID int64
	if err := e.pool.QueryRow(ctx, `insert into special_stamp_types (name, rarity, effect) values ($1, 'rare', $2::special_stamp_effect) returning id`, "stamp "+effect, effect).Scan(&typeID); err != nil {
		t.Fatal(err)
	}
	if err := e.pool.QueryRow(ctx, `insert into special_stamps (user_id, special_stamp_type_id, granted_by) values ($1, $2, $3) returning id`,
		user.ID, typeID, admin.ID).Scan(&stampID); err != nil {
		t.Fatal(err)
	}
	return stampID
}

type placed struct {
	ID           int64   `json:"id"`
	X            float64 `json:"x"`
	Y            float64 `json:"y"`
	Rotation     float64 `json:"rotation"`
	SubmissionID *int64  `json:"submission_id"`
	Error        string  `json:"error"`
}

func (e *env) place(t *testing.T, user *db.User, stampID, projectID int64, x, y float64) (int, placed) {
	t.Helper()
	var out placed
	status := e.call(t, user, "POST", "/api/special-stamps/"+itoa(stampID)+"/placement",
		map[string]any{"project_id": projectID, "x": x, "y": y, "rotation": -8}, &out)
	return status, out
}

func TestPlacingStamps(t *testing.T) {
	e := newEnv(t)
	admin := e.newAdmin(t)
	user := e.newUser(t, "U1")
	other := e.newUser(t, "U2")
	p1 := e.createProject(t, user, map[string]any{"name": "one"})
	p2 := e.createProject(t, user, map[string]any{"name": "two"})
	theirs := e.createProject(t, other, map[string]any{"name": "theirs"})
	stamp := e.giveStamp(t, admin, user, "none")

	if status, _ := e.place(t, user, stamp, theirs.ID, 0.5, 0.5); status != http.StatusNotFound {
		t.Errorf("onto someone else's project: got %d, want 404", status)
	}
	if status, _ := e.place(t, other, stamp, theirs.ID, 0.5, 0.5); status != http.StatusNotFound {
		t.Errorf("someone else's stamp: got %d, want 404", status)
	}
	status, got := e.place(t, user, stamp, p1.ID, 1.7, 0.25)
	if status != http.StatusOK || got.X != 1 || got.Y != 0.25 || got.Rotation != -8 {
		t.Fatalf("placing: got %d %+v, want x clamped to 1", status, got)
	}
	// Moving it around its project is fine; rotation stays as it was.
	if status, got := e.place(t, user, stamp, p1.ID, 0.1, 0.9); status != http.StatusOK || got.X != 0.1 || got.Rotation != -8 {
		t.Errorf("moving: got %d %+v", status, got)
	}
	// Moving it to another project isn't.
	if status, got := e.place(t, user, stamp, p2.ID, 0.5, 0.5); status != http.StatusConflict {
		t.Errorf("onto another project: got %d %q, want 409", status, got.Error)
	}

	var p struct {
		Stamps []placed `json:"stamps"`
	}
	e.mustCall(t, user, http.StatusOK, "GET", "/api/projects/"+itoa(p1.ID), nil, &p)
	if len(p.Stamps) != 1 || p.Stamps[0].ID != stamp || p.Stamps[0].Y != 0.9 {
		t.Errorf("project stamps are %+v", p.Stamps)
	}
	var mine []struct {
		ID        int64  `json:"id"`
		ProjectID *int64 `json:"project_id"`
	}
	e.mustCall(t, user, http.StatusOK, "GET", "/api/special-stamps", nil, &mine)
	if len(mine) != 1 || mine[0].ProjectID == nil || *mine[0].ProjectID != p1.ID {
		t.Errorf("special stamps are %+v, want it on project one", mine)
	}
}

func TestExpressGivesPriority(t *testing.T) {
	e := newEnv(t)
	admin := e.newAdmin(t)
	reviewer := e.newReviewer(t)
	user := e.newUser(t, "U1")
	inReview := e.createProject(t, user, completeProject("racer"))
	later := e.createProject(t, user, completeProject("kart"))
	first := e.submitHours(t, user, "U1", inReview, "racer", 3600)

	// Stuck on while a ship is in review: that ship gets priority.
	status, got := e.place(t, user, e.giveStamp(t, admin, user, "priority_review"), inReview.ID, 0.5, 0.5)
	if status != http.StatusOK || got.SubmissionID == nil || *got.SubmissionID != first {
		t.Fatalf("placing on a project in review: got %d %+v", status, got)
	}
	// Stuck on with nothing in review: the next ship gets it.
	status, got = e.place(t, user, e.giveStamp(t, admin, user, "priority_review"), later.ID, 0.5, 0.5)
	if status != http.StatusOK || got.SubmissionID != nil {
		t.Fatalf("placing on a project not in review: got %d %+v", status, got)
	}
	second := e.submitHours(t, user, "U1", later, "kart", 3600)

	var queue []struct {
		ID       int64 `json:"id"`
		Priority bool  `json:"priority"`
	}
	e.call(t, reviewer, "GET", "/api/review/queue", nil, &queue)
	if len(queue) != 2 || !queue[0].Priority || !queue[1].Priority {
		t.Errorf("queue is %+v, want both ships with priority", queue)
	}
	_ = second
}

func TestGoldenSpikeBonus(t *testing.T) {
	e := newEnv(t)
	admin := e.newAdmin(t)
	reviewer := e.newReviewer(t)
	user := e.newUser(t, "U1")
	p := e.createProject(t, user, completeProject("racer"))
	if status, got := e.place(t, user, e.giveStamp(t, admin, user, "stamp_multiplier"), p.ID, 0.5, 0.5); status != http.StatusOK {
		t.Fatalf("placing the spike: got %d %q", status, got.Error)
	}
	id := e.submitHours(t, user, "U1", p, "racer", 3*3600)

	status, result := e.decide(t, reviewer, id, map[string]any{"status": "approved"})
	if status != http.StatusOK {
		t.Fatalf("approving: got %d %q", status, result.Error)
	}
	// 3 hours at 10 an hour, plus 10%.
	if result.StampsPaid != 33 || e.stamps(t, user) != 33 {
		t.Errorf("paid %d, balance %d, want 33", result.StampsPaid, e.stamps(t, user))
	}
	var bonus int
	e.pool.QueryRow(context.Background(), "select amount from stamp_ledger where submission_id = $1 and kind = 'bonus'", id).Scan(&bonus)
	if bonus != 3 {
		t.Errorf("bonus entry is %d, want 3", bonus)
	}
}
