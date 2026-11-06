// Package activity tracks how much people work on their Crossroads projects
// each day: the time Hackatime saw on each project's linked Hackatime
// projects, split into UTC days and kept in project_activity. The admin
// activity page counts daily active participants from it.
package activity

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/scherepi/crossroads-backend/db"
	"github.com/scherepi/crossroads-backend/hackatime"
)

// lockKey is the Postgres advisory lock that keeps polling to one server at
// a time.
const lockKey = 7_201_126

type Poller struct {
	Pool      *pgxpool.Pool
	Queries   *db.Queries
	Hackatime *hackatime.Client
	// Start is when Hackatime time started counting; nothing before it is kept.
	Start time.Time
	// Now is the clock, replaceable in tests. Defaults to time.Now.
	Now func() time.Time
}

func (p *Poller) now() time.Time {
	if p.Now != nil {
		return p.Now()
	}
	return time.Now()
}

// Run polls every interval until ctx is done. The first pass of each UTC day
// goes back to Start, which fills in a new database, anything missed while
// the server was down, and time on Hackatime projects linked since. The rest
// only redo yesterday and today, when Hackatime's heartbeats still arrive.
// A project that fails is logged and tried again on the next pass.
func (p *Poller) Run(ctx context.Context, interval time.Duration) {
	var lastFull time.Time
	for {
		today := Day(p.now())
		since := today.AddDate(0, 0, -1)
		if !lastFull.Equal(today) {
			since, lastFull = p.Start, today
		}
		if err := p.Poll(ctx, since); err != nil {
			log.Printf("project activity: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}
	}
}

// Poll refreshes every tracked project's time from the start of since's day
// to now. People whose Hackatime is missing or private are skipped; they
// can't submit either. If another server holds the lock, it does nothing.
func (p *Poller) Poll(ctx context.Context, since time.Time) error {
	conn, err := p.Pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	var locked bool
	if err := conn.QueryRow(ctx, "select pg_try_advisory_lock($1)", lockKey).Scan(&locked); err != nil {
		return err
	}
	if !locked {
		return nil
	}
	defer conn.Exec(context.WithoutCancel(ctx), "select pg_advisory_unlock($1)", lockKey)

	projects, err := p.Queries.ListTrackedProjects(ctx)
	if err != nil {
		return err
	}
	now := p.now()
	from := Day(since)
	if from.Before(p.Start) {
		from = p.Start
	}
	var errs []error
	for _, project := range projects {
		spans, err := p.Hackatime.Spans(ctx, project.SlackID, project.HackatimeProjects, from, now)
		switch {
		case errors.Is(err, hackatime.ErrNoAccount), errors.Is(err, hackatime.ErrPrivate):
			continue
		case err != nil:
			errs = append(errs, fmt.Errorf("project %d: %w", project.ID, err))
			continue
		}
		if err := p.save(ctx, project.ID, from, now, ByDay(spans, from, now)); err != nil {
			errs = append(errs, fmt.Errorf("project %d: %w", project.ID, err))
		}
	}
	return errors.Join(errs...)
}

// save replaces a project's days from from to to with days.
func (p *Poller) save(ctx context.Context, projectID int64, from, to time.Time, days map[time.Time]int) error {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := p.Queries.WithTx(tx)
	if err := q.ClearProjectActivity(ctx, db.ClearProjectActivityParams{ProjectID: projectID, FromDay: Day(from), ToDay: Day(to)}); err != nil {
		return err
	}
	for day, seconds := range days {
		if seconds == 0 {
			continue
		}
		if err := q.SetProjectActivity(ctx, db.SetProjectActivityParams{ProjectID: projectID, Day: day, Seconds: int32(seconds)}); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// ByDay adds up spans' time per UTC day, counting only what falls between
// from and to. A span that crosses midnight counts on both days.
func ByDay(spans []hackatime.Span, from, to time.Time) map[time.Time]int {
	totals := map[time.Time]time.Duration{}
	for _, s := range spans {
		start, end := s.Start, s.End
		if start.Before(from) {
			start = from
		}
		if end.After(to) {
			end = to
		}
		for start.Before(end) {
			next := Day(start).AddDate(0, 0, 1)
			cut := end
			if next.Before(end) {
				cut = next
			}
			totals[Day(start)] += cut.Sub(start)
			start = cut
		}
	}
	days := make(map[time.Time]int, len(totals))
	for day, d := range totals {
		days[day] = int(math.Round(d.Seconds()))
	}
	return days
}

// Day is the start of t's UTC day.
func Day(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
