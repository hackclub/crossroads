package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/scherepi/crossroads-backend/auth"
	"github.com/scherepi/crossroads-backend/db"
)

func (e *env) newAdmin(t *testing.T) *db.User {
	t.Helper()
	admin := e.newUser(t, "UADMIN")
	if _, err := e.pool.Exec(context.Background(), "update users set role = 'admin' where id = $1", admin.ID); err != nil {
		t.Fatal(err)
	}
	admin.Role = db.UserRoleAdmin
	return admin
}

// mustCall is call, failing the test unless the response has status want.
func (e *env) mustCall(t *testing.T, user *db.User, want int, method, path string, body any, out any) {
	t.Helper()
	var response any
	if out == nil {
		out = &response
	}
	if status := e.call(t, user, method, path, body, out); status != want {
		t.Fatalf("%s %s: got %d, want %d (%+v)", method, path, status, want, out)
	}
}

type stampType struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Effect      string `json:"effect"`
	IssuedCount int    `json:"issued_count"`
}

type specialStamp struct {
	ID           int64     `json:"id"`
	Type         stampType `json:"type"`
	SubmissionID *int64    `json:"submission_id"`
}

// setUpBox makes two stamp types, a capped priority review stamp that's
// almost always what drops and an unlimited plain one, and a box type with
// both.
func (e *env) setUpBox(t *testing.T, admin *db.User, expressCap int) (express, plain stampType, boxTypeID int64) {
	t.Helper()
	e.mustCall(t, admin, http.StatusCreated, "POST", "/api/admin/special-stamp-types",
		map[string]any{"name": "Express", "rarity": "rare", "effect": "priority_review", "supply_cap": expressCap}, &express)
	e.mustCall(t, admin, http.StatusCreated, "POST", "/api/admin/special-stamp-types",
		map[string]any{"name": "Plain", "rarity": "common"}, &plain)
	var boxType struct {
		ID int64 `json:"id"`
	}
	e.mustCall(t, admin, http.StatusCreated, "POST", "/api/admin/box-types", map[string]any{"name": "Starter"}, &boxType)
	e.mustCall(t, admin, http.StatusOK, "PUT", "/api/admin/box-types/"+itoa(boxType.ID)+"/drops", []map[string]any{
		{"special_stamp_type_id": express.ID, "weight": 1_000_000},
		{"special_stamp_type_id": plain.ID, "weight": 1},
	}, nil)
	return express, plain, boxType.ID
}

func (e *env) grantBoxes(t *testing.T, admin, user *db.User, boxTypeID int64, n int) []int64 {
	t.Helper()
	var ids []int64
	for range n {
		var box struct {
			ID int64 `json:"id"`
		}
		e.mustCall(t, admin, http.StatusCreated, "POST", "/api/admin/users/"+itoa(user.ID)+"/boxes", map[string]any{"box_type_id": boxTypeID}, &box)
		ids = append(ids, box.ID)
	}
	return ids
}

func TestOpeningBoxesRespectsSupplyCaps(t *testing.T) {
	e := newEnv(t)
	admin := e.newAdmin(t)
	user := e.newUser(t, "U1")
	express, _, boxTypeID := e.setUpBox(t, admin, 1)

	counts := map[string]int{}
	for _, id := range e.grantBoxes(t, admin, user, boxTypeID, 5) {
		var got specialStamp
		e.mustCall(t, user, http.StatusOK, "POST", "/api/boxes/"+itoa(id)+"/open", nil, &got)
		counts[got.Type.Name]++
	}
	if counts["Express"] != 1 || counts["Plain"] != 4 {
		t.Errorf("got %v from 5 boxes, want the 1 Express there was and Plain after", counts)
	}

	var boxTypes []struct {
		Odds []struct {
			SpecialStampType stampType `json:"special_stamp_type"`
			Chance           float64   `json:"chance"`
		} `json:"odds"`
	}
	e.mustCall(t, user, http.StatusOK, "GET", "/api/box-types", nil, &boxTypes)
	if len(boxTypes) != 1 || len(boxTypes[0].Odds) != 1 || boxTypes[0].Odds[0].SpecialStampType.ID == express.ID || boxTypes[0].Odds[0].Chance != 1 {
		t.Errorf("odds are %+v, want only Plain at 100%% now Express has run out", boxTypes)
	}

	var types []stampType
	e.mustCall(t, admin, http.StatusOK, "GET", "/api/admin/special-stamp-types", nil, &types)
	for _, st := range types {
		if st.ID == express.ID && st.IssuedCount != 1 {
			t.Errorf("Express issued_count is %d, want 1", st.IssuedCount)
		}
	}
}

func TestSimultaneousOpensDontOverIssue(t *testing.T) {
	e := newEnv(t)
	admin := e.newAdmin(t)
	user := e.newUser(t, "U1")
	_, _, boxTypeID := e.setUpBox(t, admin, 3)
	ids := e.grantBoxes(t, admin, user, boxTypeID, 10)

	var wg sync.WaitGroup
	for _, id := range ids {
		wg.Go(func() {
			req := httptest.NewRequest("POST", "/api/boxes/"+itoa(id)+"/open", nil)
			req = req.WithContext(auth.WithUser(req.Context(), user))
			e.router.ServeHTTP(httptest.NewRecorder(), req)
		})
	}
	wg.Wait()

	var express, opened int
	e.pool.QueryRow(context.Background(), `select count(*) from special_stamps
		join special_stamp_types on special_stamp_types.id = special_stamps.special_stamp_type_id
		where special_stamp_types.name = 'Express'`).Scan(&express)
	e.pool.QueryRow(context.Background(), "select count(*) from boxes where opened_at is not null").Scan(&opened)
	if express != 3 || opened != 10 {
		t.Errorf("%d Express from %d opened boxes, want exactly the 3 there were from all 10", express, opened)
	}
}

func TestOpeningABoxOnce(t *testing.T) {
	e := newEnv(t)
	admin := e.newAdmin(t)
	user := e.newUser(t, "U1")
	_, _, boxTypeID := e.setUpBox(t, admin, 1)
	id := e.grantBoxes(t, admin, user, boxTypeID, 1)[0]

	stranger := e.newUser(t, "U2")
	e.mustCall(t, stranger, http.StatusNotFound, "POST", "/api/boxes/"+itoa(id)+"/open", nil, nil)
	e.mustCall(t, user, http.StatusOK, "POST", "/api/boxes/"+itoa(id)+"/open", nil, nil)
	e.mustCall(t, user, http.StatusConflict, "POST", "/api/boxes/"+itoa(id)+"/open", nil, nil)

	var boxes []struct {
		SpecialStampID *int64 `json:"special_stamp_id"`
	}
	e.mustCall(t, user, http.StatusOK, "GET", "/api/boxes", nil, &boxes)
	if len(boxes) != 1 || boxes[0].SpecialStampID == nil {
		t.Errorf("boxes are %+v, want the one opened box with what it dropped", boxes)
	}
}

func TestBoxesCantRunEmpty(t *testing.T) {
	e := newEnv(t)
	admin := e.newAdmin(t)
	express, plain, boxTypeID := e.setUpBox(t, admin, 1)

	e.mustCall(t, admin, http.StatusUnprocessableEntity, "PUT", "/api/admin/box-types/"+itoa(boxTypeID)+"/drops",
		[]map[string]any{{"special_stamp_type_id": express.ID, "weight": 1}}, nil)
	e.mustCall(t, admin, http.StatusConflict, "PATCH", "/api/admin/special-stamp-types/"+itoa(plain.ID),
		map[string]any{"supply_cap": 100}, nil)

	// Once there's another unlimited stamp in the box, Plain can be capped.
	var spare stampType
	e.mustCall(t, admin, http.StatusCreated, "POST", "/api/admin/special-stamp-types", map[string]any{"name": "Spare", "rarity": "common"}, &spare)
	e.mustCall(t, admin, http.StatusOK, "PUT", "/api/admin/box-types/"+itoa(boxTypeID)+"/drops", []map[string]any{
		{"special_stamp_type_id": plain.ID, "weight": 1}, {"special_stamp_type_id": spare.ID, "weight": 1},
	}, nil)
	e.mustCall(t, admin, http.StatusOK, "PATCH", "/api/admin/special-stamp-types/"+itoa(plain.ID), map[string]any{"supply_cap": 100}, nil)
	// Uncapping is always fine.
	e.mustCall(t, admin, http.StatusOK, "PATCH", "/api/admin/special-stamp-types/"+itoa(plain.ID), map[string]any{"supply_cap": nil}, nil)
}

func TestApplyingSpecialStamps(t *testing.T) {
	e := newEnv(t)
	admin := e.newAdmin(t)
	reviewer := e.newReviewer(t)
	user := e.newUser(t, "U1")
	express, plain, _ := e.setUpBox(t, admin, 10)

	var expressStamp, plainStamp specialStamp
	e.mustCall(t, admin, http.StatusCreated, "POST", "/api/admin/users/"+itoa(user.ID)+"/special-stamps", map[string]any{"special_stamp_type_id": express.ID}, &expressStamp)
	e.mustCall(t, admin, http.StatusCreated, "POST", "/api/admin/users/"+itoa(user.ID)+"/special-stamps", map[string]any{"special_stamp_type_id": plain.ID}, &plainStamp)

	e.hackatime.set("U1", "first", 3600)
	e.hackatime.set("U1", "second", 3600)
	e.hackatime.set("U2", "theirs", 3600)
	first := e.createProject(t, user, completeProject("first"))
	second := e.createProject(t, user, completeProject("second"))
	submissions := func(id int64) string { return "/api/projects/" + itoa(id) + "/submissions" }

	e.mustCall(t, user, http.StatusUnprocessableEntity, "POST", submissions(first.ID), map[string]any{"special_stamp_id": plainStamp.ID}, nil)
	e.mustCall(t, user, http.StatusCreated, "POST", submissions(first.ID), map[string]any{"special_stamp_id": expressStamp.ID}, nil)
	e.mustCall(t, user, http.StatusConflict, "POST", submissions(second.ID), map[string]any{"special_stamp_id": expressStamp.ID}, nil)

	// Someone else can't use it.
	other := e.newUser(t, "U2")
	theirs := e.createProject(t, other, completeProject("theirs"))
	e.mustCall(t, other, http.StatusNotFound, "POST", submissions(theirs.ID), map[string]any{"special_stamp_id": plainStamp.ID}, nil)

	var resp queueResponse
	e.mustCall(t, reviewer, http.StatusOK, "GET", "/api/review/queue", nil, &resp)
	queue := resp.Items
	if len(queue) != 1 || queue[0].ProjectID != first.ID || !queue[0].Priority {
		t.Errorf("queue is %+v, want the first project with priority", queue)
	}

	var stamps []specialStamp
	e.mustCall(t, user, http.StatusOK, "GET", "/api/special-stamps", nil, &stamps)
	for _, s := range stamps {
		if (s.ID == expressStamp.ID) != (s.SubmissionID != nil) {
			t.Errorf("stamp %+v: only Express should be used", s)
		}
	}
}

func TestAdminGrantsComeOutOfSupply(t *testing.T) {
	e := newEnv(t)
	admin := e.newAdmin(t)
	user := e.newUser(t, "U1")
	express, _, _ := e.setUpBox(t, admin, 1)

	path := "/api/admin/users/" + itoa(user.ID) + "/special-stamps"
	e.mustCall(t, admin, http.StatusCreated, "POST", path, map[string]any{"special_stamp_type_id": express.ID}, nil)
	e.mustCall(t, admin, http.StatusConflict, "POST", path, map[string]any{"special_stamp_type_id": express.ID}, nil)
}

func TestNewMilestonesReachPeopleAlreadyPastThem(t *testing.T) {
	e := newEnv(t)
	admin := e.newAdmin(t)
	reviewer := e.newReviewer(t)
	_, _, boxTypeID := e.setUpBox(t, admin, 1)

	ahead := e.newUser(t, "U1")
	p := e.createProject(t, ahead, completeProject("racer"))
	id := e.submitHours(t, ahead, "U1", p, "racer", 12*3600)
	e.decide(t, reviewer, id, map[string]any{"action": "approve"})
	behind := e.newUser(t, "U2")
	p = e.createProject(t, behind, completeProject("racer"))
	id = e.submitHours(t, behind, "U2", p, "racer", 3*3600)
	e.decide(t, reviewer, id, map[string]any{"action": "approve"})

	var created struct {
		BoxesGranted int `json:"boxes_granted"`
	}
	e.mustCall(t, admin, http.StatusCreated, "POST", "/api/admin/milestones",
		map[string]any{"name": "10 hours", "approved_seconds": 36000, "box_type_id": boxTypeID}, &created)
	if created.BoxesGranted != 1 {
		t.Errorf("granted %d boxes, want 1 for the person past 10 hours", created.BoxesGranted)
	}
	var boxes []any
	e.mustCall(t, ahead, http.StatusOK, "GET", "/api/boxes", nil, &boxes)
	if len(boxes) != 1 {
		t.Errorf("the person past it has %d boxes, want 1", len(boxes))
	}
	e.mustCall(t, admin, http.StatusConflict, "POST", "/api/admin/milestones",
		map[string]any{"name": "Also 10 hours", "approved_seconds": 36000, "box_type_id": boxTypeID}, nil)
}

func TestAdminRoutesAndRoles(t *testing.T) {
	e := newEnv(t)
	admin := e.newAdmin(t)
	user := e.newUser(t, "U1")

	e.mustCall(t, user, http.StatusForbidden, "GET", "/api/admin/users", nil, nil)
	e.mustCall(t, e.newReviewer(t), http.StatusForbidden, "GET", "/api/admin/box-types", nil, nil)

	var found []struct {
		ID int64 `json:"id"`
	}
	e.mustCall(t, admin, http.StatusOK, "GET", "/api/admin/users?q=U1", nil, &found)
	if len(found) != 1 || found[0].ID != user.ID {
		t.Errorf("searching by Slack ID found %+v", found)
	}
	e.mustCall(t, admin, http.StatusOK, "PATCH", "/api/admin/users/"+itoa(user.ID), map[string]any{"role": "reviewer"}, nil)
	// Roles are read from the database on every request, so refetch.
	updated, _ := e.queries.GetUser(context.Background(), user.ID)
	e.mustCall(t, &updated, http.StatusOK, "GET", "/api/review/queue", nil, nil)

	e.mustCall(t, admin, http.StatusUnprocessableEntity, "PATCH", "/api/admin/users/"+itoa(admin.ID), map[string]any{"role": "participant"}, nil)
	e.mustCall(t, admin, http.StatusUnprocessableEntity, "PATCH", "/api/admin/users/"+itoa(user.ID), map[string]any{"role": "overlord"}, nil)
}

func TestRoleInvites(t *testing.T) {
	e := newEnv(t)
	admin := e.newAdmin(t)

	type invite struct {
		ID            int64   `json:"id"`
		Email         string  `json:"email"`
		Role          string  `json:"role"`
		InvitedByName *string `json:"invited_by_name"`
	}
	var created invite
	e.mustCall(t, admin, http.StatusCreated, "POST", "/api/admin/role-invites", map[string]any{"email": " Orpheus@HackClub.com ", "role": "reviewer"}, &created)
	if created.Email != "orpheus@hackclub.com" || created.Role != "reviewer" || created.InvitedByName == nil || *created.InvitedByName != admin.Name {
		t.Errorf("created %+v", created)
	}
	// Inviting the same email again changes its role.
	e.mustCall(t, admin, http.StatusCreated, "POST", "/api/admin/role-invites", map[string]any{"email": "orpheus@hackclub.com", "role": "admin"}, nil)
	var invites []invite
	e.mustCall(t, admin, http.StatusOK, "GET", "/api/admin/role-invites", nil, &invites)
	if len(invites) != 1 || invites[0].ID != created.ID || invites[0].Role != "admin" {
		t.Errorf("listed %+v, want one admin invite", invites)
	}

	for _, body := range []map[string]any{
		{"email": "not an email", "role": "admin"},
		{"email": "Orpheus <orpheus@hackclub.com>", "role": "admin"},
		{"email": "heidi@example.com", "role": "participant"},
		{"email": "heidi@example.com", "role": "overlord"},
	} {
		e.mustCall(t, admin, http.StatusUnprocessableEntity, "POST", "/api/admin/role-invites", body, nil)
	}
	// Someone who has already signed in gets their role changed directly.
	e.mustCall(t, admin, http.StatusConflict, "POST", "/api/admin/role-invites", map[string]any{"email": "X@example.com", "role": "admin"}, nil)
	e.mustCall(t, e.newUser(t, "U1"), http.StatusForbidden, "GET", "/api/admin/role-invites", nil, nil)

	e.mustCall(t, admin, http.StatusNoContent, "DELETE", "/api/admin/role-invites/"+itoa(created.ID), nil, nil)
	e.mustCall(t, admin, http.StatusNotFound, "DELETE", "/api/admin/role-invites/"+itoa(created.ID), nil, nil)
	e.mustCall(t, admin, http.StatusOK, "GET", "/api/admin/role-invites", nil, &invites)
	if len(invites) != 0 {
		t.Errorf("%d invites left after deleting, want 0", len(invites))
	}
}
