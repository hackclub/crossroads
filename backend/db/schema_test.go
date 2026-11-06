package db_test

// These tests check the rules the schema enforces on its own. They need a
// Postgres to run against; see dbtest.New.

import (
	"context"
	"crypto/rand"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/scherepi/crossroads-backend/db"
	"github.com/scherepi/crossroads-backend/db/dbtest"
)

// insert runs an insert ... returning id and returns the id.
func insert(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) int64 {
	t.Helper()
	var id int64
	if err := pool.QueryRow(context.Background(), sql+" returning id", args...).Scan(&id); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return id
}

func insertUser(t *testing.T, pool *pgxpool.Pool, role db.UserRole) int64 {
	t.Helper()
	hcaID := rand.Text()
	return insert(t, pool, `insert into users (hca_id, email, name, role) values ($1, $2, 'Test', $3)`,
		hcaID, hcaID+"@example.com", role)
}

func balance(t *testing.T, q *db.Queries, userID int64) int32 {
	t.Helper()
	user, err := q.GetUser(context.Background(), userID)
	if err != nil {
		t.Fatal(err)
	}
	return user.Stamps
}

func ptr[T any](v T) *T { return &v }

// wantViolation fails the test unless err is Postgres rejecting a write because
// of the named constraint.
func wantViolation(t *testing.T, err error, constraint string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.ConstraintName != constraint {
		t.Errorf("got %v, want a %s violation", err, constraint)
	}
}

func TestPurchaseCantOverdraw(t *testing.T) {
	pool := dbtest.New(t)
	q := db.New(pool)
	ctx := context.Background()
	admin := insertUser(t, pool, db.UserRoleAdmin)
	user := insertUser(t, pool, db.UserRoleParticipant)

	if _, err := q.AddStampEntry(ctx, db.AddStampEntryParams{
		UserID: user, Kind: db.StampEntryKindAdjustment, Amount: 10, CreatedBy: &admin, Note: ptr("test"),
	}); err != nil {
		t.Fatal(err)
	}

	item := insert(t, pool, `insert into shop_items (kind, name, price) values ('physical', 'Sticker', 20)`)
	order := insert(t, pool, `insert into orders (user_id, shop_item_id, price) values ($1, $2, 20)`, user, item)
	_, err := q.AddStampEntry(ctx, db.AddStampEntryParams{
		UserID: user, Kind: db.StampEntryKindPurchase, Amount: -20, OrderID: &order,
	})
	if !db.IsNotEnoughStamps(err) {
		t.Fatalf("got %v, want a not-enough-stamps error", err)
	}
	if got := balance(t, q, user); got != 10 {
		t.Errorf("balance is %d after a failed purchase, want 10", got)
	}

	if _, err := q.AddStampEntry(ctx, db.AddStampEntryParams{
		UserID: user, Kind: db.StampEntryKindPurchase, Amount: -10, OrderID: &order,
	}); err != nil {
		t.Fatalf("spending the exact balance: %v", err)
	}
	if got := balance(t, q, user); got != 0 {
		t.Errorf("balance is %d, want 0", got)
	}
}

func TestAdjustmentCanGoNegative(t *testing.T) {
	pool := dbtest.New(t)
	q := db.New(pool)
	ctx := context.Background()
	admin := insertUser(t, pool, db.UserRoleAdmin)
	user := insertUser(t, pool, db.UserRoleParticipant)

	if _, err := q.AddStampEntry(ctx, db.AddStampEntryParams{
		UserID: user, Kind: db.StampEntryKindAdjustment, Amount: -50, CreatedBy: &admin, Note: ptr("clawback"),
	}); err != nil {
		t.Fatal(err)
	}
	if got := balance(t, q, user); got != -50 {
		t.Errorf("balance is %d, want -50", got)
	}

	// Payouts still land while someone is in the negative.
	project := insert(t, pool, `insert into projects (user_id, name) values ($1, 'p')`, user)
	submission := insert(t, pool, `insert into submissions (project_id, hackatime_projects, status, hackatime_seconds, new_seconds,
		approved_seconds, reviewer_id, reviewed_at) values ($1, '{}', 'approved', 3600, 3600, 3600, $2, now())`, project, admin)
	if _, err := q.AddStampEntry(ctx, db.AddStampEntryParams{
		UserID: user, Kind: db.StampEntryKindPayout, Amount: 20, SubmissionID: &submission,
	}); err != nil {
		t.Fatal(err)
	}
	if got := balance(t, q, user); got != -30 {
		t.Errorf("balance is %d, want -30", got)
	}
}

func TestLedgerIsAppendOnly(t *testing.T) {
	pool := dbtest.New(t)
	q := db.New(pool)
	ctx := context.Background()
	admin := insertUser(t, pool, db.UserRoleAdmin)

	entry, err := q.AddStampEntry(ctx, db.AddStampEntryParams{
		UserID: admin, Kind: db.StampEntryKindAdjustment, Amount: 5, CreatedBy: &admin, Note: ptr("test"),
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, sql := range []string{
		`update stamp_ledger set amount = 500 where id = $1`,
		`delete from stamp_ledger where id = $1`,
	} {
		_, err := pool.Exec(ctx, sql, entry.ID)
		if err == nil || !strings.Contains(err.Error(), "append-only") {
			t.Errorf("%s: got %v, want an append-only error", sql, err)
		}
	}
}

func TestSubmissionPaysOutOnce(t *testing.T) {
	pool := dbtest.New(t)
	q := db.New(pool)
	ctx := context.Background()
	reviewer := insertUser(t, pool, db.UserRoleReviewer)
	user := insertUser(t, pool, db.UserRoleParticipant)
	project := insert(t, pool, `insert into projects (user_id, name) values ($1, 'p')`, user)
	submission := insert(t, pool, `insert into submissions (project_id, hackatime_projects, status, hackatime_seconds, new_seconds,
		approved_seconds, reviewer_id, reviewed_at) values ($1, '{}', 'approved', 3600, 3600, 3600, $2, now())`, project, reviewer)

	payout := db.AddStampEntryParams{UserID: user, Kind: db.StampEntryKindPayout, Amount: 10, SubmissionID: &submission}
	if _, err := q.AddStampEntry(ctx, payout); err != nil {
		t.Fatal(err)
	}
	_, err := q.AddStampEntry(ctx, payout)
	wantViolation(t, err, "stamp_ledger_one_entry_per_submission_kind")

	// A Golden Spike bonus sits alongside the payout, once.
	bonus := db.AddStampEntryParams{UserID: user, Kind: db.StampEntryKindBonus, Amount: 1, SubmissionID: &submission}
	if _, err := q.AddStampEntry(ctx, bonus); err != nil {
		t.Fatal(err)
	}
	_, err = q.AddStampEntry(ctx, bonus)
	wantViolation(t, err, "stamp_ledger_one_entry_per_submission_kind")
}

func TestOnePendingSubmissionPerProject(t *testing.T) {
	pool := dbtest.New(t)
	user := insertUser(t, pool, db.UserRoleParticipant)
	project := insert(t, pool, `insert into projects (user_id, name) values ($1, 'p')`, user)

	submit := `insert into submissions (project_id, hackatime_projects, hackatime_seconds, new_seconds) values ($1, '{}', 3600, 3600)`
	insert(t, pool, submit, project)
	_, err := pool.Exec(context.Background(), submit, project)
	wantViolation(t, err, "submissions_one_pending_per_project")
}

func TestApprovedSecondsCantExceedNewSeconds(t *testing.T) {
	pool := dbtest.New(t)
	reviewer := insertUser(t, pool, db.UserRoleReviewer)
	user := insertUser(t, pool, db.UserRoleParticipant)
	project := insert(t, pool, `insert into projects (user_id, name) values ($1, 'p')`, user)

	_, err := pool.Exec(context.Background(), `insert into submissions (project_id, hackatime_projects, status, hackatime_seconds,
		new_seconds, approved_seconds, reviewer_id, reviewed_at) values ($1, '{}', 'approved', 7200, 3600, 7200, $2, now())`,
		project, reviewer)
	wantViolation(t, err, "submissions_approved_within_new")
}

func TestHackatimeProjectCountsTowardOneProject(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	user := insertUser(t, pool, db.UserRoleParticipant)
	other := insertUser(t, pool, db.UserRoleParticipant)
	first := insert(t, pool, `insert into projects (user_id, name) values ($1, 'first')`, user)
	second := insert(t, pool, `insert into projects (user_id, name) values ($1, 'second')`, user)
	othersProject := insert(t, pool, `insert into projects (user_id, name) values ($1, 'theirs')`, other)

	link := `insert into project_hackatime_projects (user_id, hackatime_project, project_id) values ($1, $2, $3)`
	if _, err := pool.Exec(ctx, link, user, "crossroads-game", first); err != nil {
		t.Fatal(err)
	}
	_, err := pool.Exec(ctx, link, user, "crossroads-game", second)
	wantViolation(t, err, "project_hackatime_projects_pkey")
	_, err = pool.Exec(ctx, link, user, "other-game", othersProject)
	wantViolation(t, err, "project_hackatime_projects_project_id_user_id_fkey")
}

func TestOneBoxPerMilestone(t *testing.T) {
	pool := dbtest.New(t)
	user := insertUser(t, pool, db.UserRoleParticipant)
	boxType := insert(t, pool, `insert into box_types (name) values ('Starter box')`)
	milestone := insert(t, pool, `insert into milestones (name, approved_seconds, box_type_id)
		values ('First 10 hours', 36000, $1)`, boxType)

	grant := `insert into boxes (user_id, box_type_id, milestone_id) values ($1, $2, $3)`
	insert(t, pool, grant, user, boxType, milestone)
	_, err := pool.Exec(context.Background(), grant, user, boxType, milestone)
	wantViolation(t, err, "boxes_user_id_milestone_id_key")
}

func TestNoStackingSpecialStamps(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	admin := insertUser(t, pool, db.UserRoleAdmin)
	user := insertUser(t, pool, db.UserRoleParticipant)
	priority := insert(t, pool, `insert into special_stamp_types (name, rarity, effect)
		values ('Express', 'rare', 'priority_review')`)
	project := insert(t, pool, `insert into projects (user_id, name) values ($1, 'p')`, user)

	stamp := `insert into special_stamps (user_id, special_stamp_type_id, granted_by, submission_id) values ($1, $2, $3, $4)`
	firstSubmission := insert(t, pool, `insert into submissions (project_id, hackatime_projects, hackatime_seconds, new_seconds)
		values ($1, '{}', 3600, 3600)`, project)
	insert(t, pool, stamp, user, priority, admin, firstSubmission)
	_, err := pool.Exec(ctx, stamp, user, priority, admin, firstSubmission)
	wantViolation(t, err, "special_stamps_submission_id_key")
}

func TestMigrationsRoundTrip(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	provider, closeDB, err := db.Migrator(pool)
	if err != nil {
		t.Fatal(err)
	}
	defer closeDB()

	if _, err := provider.DownTo(ctx, 0); err != nil {
		t.Fatalf("migrating down: %v", err)
	}
	var leftover []string
	rows, err := pool.Query(ctx, `
		select table_name from information_schema.tables
		where table_schema = current_schema() and table_name <> 'goose_db_version'
		union all
		select typname from pg_type
		where typnamespace = current_schema()::regnamespace and typtype = 'e'
		union all
		select proname from pg_proc where pronamespace = current_schema()::regnamespace`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		leftover = append(leftover, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(leftover) > 0 {
		t.Errorf("left behind after migrating down: %v", leftover)
	}

	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("migrating back up: %v", err)
	}
}
