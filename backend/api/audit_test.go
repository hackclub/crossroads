package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/scherepi/crossroads-backend/db"
)

func (e *env) newSuperadmin(t *testing.T) *db.User {
	t.Helper()
	superadmin := e.newUser(t, "USUPER")
	if _, err := e.pool.Exec(context.Background(), "update users set role = 'superadmin' where id = $1", superadmin.ID); err != nil {
		t.Fatal(err)
	}
	superadmin.Role = db.UserRoleSuperadmin
	return superadmin
}

type auditEvent struct {
	ID        int64          `json:"id"`
	Kind      string         `json:"kind"`
	ActorID   *int64         `json:"actor_id"`
	ActorName *string        `json:"actor_name"`
	UserID    *int64         `json:"user_id"`
	Details   map[string]any `json:"details"`
}

func (e *env) auditLog(t *testing.T, superadmin *db.User, query string) []auditEvent {
	t.Helper()
	var events []auditEvent
	e.mustCall(t, superadmin, http.StatusOK, "GET", "/api/superadmin/audit-log"+query, nil, &events)
	return events
}

func TestOnlySuperadminsCanReadTheAuditLog(t *testing.T) {
	e := newEnv(t)
	e.mustCall(t, e.newUser(t, "U1"), http.StatusForbidden, "GET", "/api/superadmin/audit-log", nil, nil)
	e.mustCall(t, e.newReviewer(t), http.StatusForbidden, "GET", "/api/superadmin/audit-log", nil, nil)
	e.mustCall(t, e.newAdmin(t), http.StatusForbidden, "GET", "/api/superadmin/audit-log", nil, nil)
	superadmin := e.newSuperadmin(t)
	e.mustCall(t, superadmin, http.StatusOK, "GET", "/api/superadmin/audit-log", nil, nil)
	e.mustCall(t, superadmin, http.StatusUnprocessableEntity, "GET", "/api/superadmin/audit-log?kind=stamp", nil, nil)
	// Superadmins can do everything admins can.
	e.mustCall(t, superadmin, http.StatusOK, "GET", "/api/admin/users", nil, nil)
	e.mustCall(t, superadmin, http.StatusOK, "GET", "/api/review/queue", nil, nil)
}

func TestAuditLogRecordsSpecialStampsAndWhereTheyCameFrom(t *testing.T) {
	e := newEnv(t)
	admin := e.newAdmin(t)
	superadmin := e.newSuperadmin(t)
	user := e.newUser(t, "U1")
	express, plain, boxTypeID := e.setUpBox(t, admin, 1)
	boxID := e.grantBoxes(t, admin, user, boxTypeID, 1)[0]
	var opened specialStamp
	e.mustCall(t, user, http.StatusOK, "POST", "/api/boxes/"+itoa(boxID)+"/open", nil, &opened)
	e.mustCall(t, admin, http.StatusCreated, "POST", "/api/admin/users/"+itoa(user.ID)+"/special-stamps",
		map[string]any{"special_stamp_type_id": plain.ID}, nil)

	issued := e.auditLog(t, superadmin, "?kind=special_stamp.issued")
	if len(issued) != 2 {
		t.Fatalf("got %d special_stamp.issued events, want 2: %+v", len(issued), issued)
	}
	// Newest first: the admin's gift, then the box.
	gift, fromBox := issued[0], issued[1]
	if *gift.ActorID != admin.ID || *gift.UserID != user.ID || gift.Details["source"] != "admin" || gift.Details["box_id"] != nil {
		t.Errorf("the admin's gift is %+v", gift)
	}
	if *fromBox.ActorID != user.ID || *fromBox.UserID != user.ID || fromBox.Details["source"] != "box" ||
		fromBox.Details["special_stamp_type"] != express.Name || fromBox.Details["box_id"] != float64(boxID) ||
		fromBox.Details["box_type"] != "Starter" || fromBox.Details["box_granted_by"] != float64(admin.ID) {
		t.Errorf("the stamp from the box is %+v", fromBox)
	}

	// Each kind filters on its own.
	if boxes := e.auditLog(t, superadmin, "?kind=box"); len(boxes) != 1 || boxes[0].Kind != "box.granted" ||
		*boxes[0].ActorID != admin.ID || boxes[0].Details["source"] != "admin" {
		t.Errorf("?kind=box is %+v, want the one box the admin gave", boxes)
	}
	for _, event := range e.auditLog(t, superadmin, "?kind=box.granted&kind=special_stamp") {
		if event.Kind != "box.granted" && event.Kind != "special_stamp.issued" {
			t.Errorf("?kind=box.granted&kind=special_stamp has a %s", event.Kind)
		}
	}
}

func TestAuditLogRecordsStampChanges(t *testing.T) {
	e := newEnv(t)
	admin := e.newAdmin(t)
	superadmin := e.newSuperadmin(t)
	user := e.newUser(t, "U1")
	other := e.newUser(t, "U2")
	e.mustCall(t, admin, http.StatusCreated, "POST", "/api/admin/users/"+itoa(user.ID)+"/stamp-adjustments",
		map[string]any{"amount": 50, "note": "welcome"}, nil)
	e.mustCall(t, admin, http.StatusCreated, "POST", "/api/admin/users/"+itoa(other.ID)+"/stamp-adjustments",
		map[string]any{"amount": -5, "note": "clawback"}, nil)

	events := e.auditLog(t, superadmin, "?kind=stamps&user="+itoa(user.ID))
	if len(events) != 1 {
		t.Fatalf("got %+v, want the one adjustment for user", events)
	}
	if got := events[0]; got.Kind != "stamps.adjustment" || *got.ActorID != admin.ID || got.Details["amount"] != float64(50) ||
		got.Details["balance"] != float64(50) || got.Details["note"] != "welcome" {
		t.Errorf("the adjustment is %+v", got)
	}

	// The request that made it is there too, about them.
	requests := e.auditLog(t, superadmin, "?kind=request&user="+itoa(user.ID))
	if len(requests) != 0 {
		t.Errorf("got requests %+v; the API alone doesn't record them (package audit does)", requests)
	}
}

func TestAuditLogPages(t *testing.T) {
	e := newEnv(t)
	admin := e.newAdmin(t)
	superadmin := e.newSuperadmin(t)
	user := e.newUser(t, "U1")
	for range 3 {
		e.mustCall(t, admin, http.StatusCreated, "POST", "/api/admin/users/"+itoa(user.ID)+"/stamp-adjustments",
			map[string]any{"amount": 1, "note": "x"}, nil)
	}
	first := e.auditLog(t, superadmin, "?kind=stamps&limit=2")
	if len(first) != 2 {
		t.Fatalf("got %d events, want a page of 2", len(first))
	}
	rest := e.auditLog(t, superadmin, "?kind=stamps&limit=2&before="+itoa(first[1].ID))
	if len(rest) != 1 || rest[0].ID >= first[1].ID {
		t.Errorf("the next page is %+v, want the oldest one", rest)
	}
}

func TestAuditLogRecordsRoleChanges(t *testing.T) {
	e := newEnv(t)
	admin := e.newAdmin(t)
	superadmin := e.newSuperadmin(t)
	user := e.newUser(t, "U1")
	e.mustCall(t, admin, http.StatusOK, "PATCH", "/api/admin/users/"+itoa(user.ID), map[string]any{"role": "reviewer"}, nil)

	events := e.auditLog(t, superadmin, "?kind=role&user="+itoa(user.ID))
	if len(events) != 1 || events[0].Kind != "role.changed" || *events[0].UserID != user.ID ||
		events[0].Details["from"] != "participant" || events[0].Details["to"] != "reviewer" {
		t.Errorf("got %+v, want participant → reviewer", events)
	}
}

func TestAuditLogIsAppendOnly(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	if err := e.queries.AddAuditEvent(ctx, db.AddAuditEventParams{Kind: "request", Details: []byte("{}")}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.pool.Exec(ctx, "update audit_log set details = '{}'"); err == nil {
		t.Error("updated an audit log row")
	}
	if _, err := e.pool.Exec(ctx, "delete from audit_log"); err == nil {
		t.Error("deleted an audit log row")
	}
	// A user_id that isn't anyone's is dropped, not the whole row.
	missing := int64(999999)
	if err := e.queries.AddAuditEvent(ctx, db.AddAuditEventParams{Kind: "request", UserID: &missing, Details: []byte("{}")}); err != nil {
		t.Errorf("recording a request about a missing user: %v", err)
	}
}

func TestAdminsCantGrantOrRemoveSuperadmin(t *testing.T) {
	e := newEnv(t)
	admin := e.newAdmin(t)
	superadmin := e.newSuperadmin(t)
	user := e.newUser(t, "U1")
	e.mustCall(t, admin, http.StatusUnprocessableEntity, "PATCH", "/api/admin/users/"+itoa(user.ID), map[string]any{"role": "superadmin"}, nil)
	e.mustCall(t, admin, http.StatusUnprocessableEntity, "PATCH", "/api/admin/users/"+itoa(superadmin.ID), map[string]any{"role": "admin"}, nil)
	e.mustCall(t, superadmin, http.StatusUnprocessableEntity, "PATCH", "/api/admin/users/"+itoa(user.ID), map[string]any{"role": "superadmin"}, nil)
	e.mustCall(t, admin, http.StatusUnprocessableEntity, "POST", "/api/admin/role-invites", map[string]any{"email": "new@example.com", "role": "superadmin"}, nil)
	// Other changes to a superadmin are fine, and so is saving the role they have.
	var got struct {
		Role      string `json:"role"`
		CanReview bool   `json:"can_review"`
	}
	e.mustCall(t, admin, http.StatusOK, "PATCH", "/api/admin/users/"+itoa(superadmin.ID), map[string]any{"role": "superadmin", "emporium_access": true}, &got)
	if got.Role != "superadmin" {
		t.Errorf("role is %s, want superadmin", got.Role)
	}

	var me map[string]any
	e.mustCall(t, superadmin, http.StatusOK, "GET", "/api/me", nil, &me)
	if me["role"] != "superadmin" || me["can_hq_review"] != true || me["emporium"] != true {
		t.Errorf("/api/me for a superadmin is %v, want an admin's powers", me)
	}
}

func TestSyncSuperadmins(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	listed, former, other := e.newUser(t, "ULISTED"), e.newSuperadmin(t), e.newUser(t, "UOTHER")
	if err := e.queries.SyncSuperadmins(ctx, []string{"ULISTED"}); err != nil {
		t.Fatal(err)
	}
	for user, want := range map[*db.User]db.UserRole{listed: db.UserRoleSuperadmin, former: db.UserRoleAdmin, other: db.UserRoleParticipant} {
		got, err := e.queries.GetUser(ctx, user.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.Role != want {
			t.Errorf("%s is %s, want %s", *user.SlackID, got.Role, want)
		}
	}
	var changes []json.RawMessage
	rows, _ := e.pool.Query(ctx, "select details from audit_log where kind = 'role.changed' order by id")
	for rows.Next() {
		var details json.RawMessage
		rows.Scan(&details)
		changes = append(changes, details)
	}
	if len(changes) != 3 { // newSuperadmin's own update, then the two the sync made
		t.Errorf("got role changes %s, want 3", changes)
	}
}

func TestAuditLogRecordsPayoutsWithWhoSignedOff(t *testing.T) {
	e := newEnv(t)
	superadmin := e.newSuperadmin(t)
	reviewer := e.newReviewer(t)
	user, submissionID := e.shipFor(t, "U1", 2)
	e.approveAllTheWay(t, reviewer, submissionID)

	events := e.auditLog(t, superadmin, "?kind=stamps.payout")
	if len(events) != 1 {
		t.Fatalf("got %+v, want one payout", events)
	}
	if got := events[0]; *got.UserID != user.ID || got.ActorID == nil || *got.ActorID != reviewer.ID ||
		got.Details["amount"] != float64(20) || got.Details["submission_id"] != float64(submissionID) || got.Details["project"] != "p-U1" {
		t.Errorf("the payout is %+v", got)
	}
}
