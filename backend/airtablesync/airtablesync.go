// Package airtablesync copies approved submissions and shop orders into the
// Crossroads Airtable base, and picks up orders marked fulfilled there.
// Postgres stays the source of truth: apart from the Fulfilled checkbox on
// orders, nothing comes back.
package airtablesync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net/url"
	"strings"
	"time"

	"github.com/scherepi/crossroads-backend/airtable"
	"github.com/scherepi/crossroads-backend/db"
)

const (
	// The YSWS template table that feeds the Unified YSWS DB. A person ticks
	// its "Automation - Submit to Unified YSWS" box after checking a row; the
	// sync never does.
	submissionsTable = "YSWS Project Submission"
	ordersTable      = "Shop Orders"
)

type Syncer struct {
	Queries  *db.Queries
	Airtable *airtable.Client
	// HoursStart is when Hackatime time started counting, for the hours
	// justification.
	HoursStart time.Time
}

// Run syncs every interval until ctx is done.
func (s *Syncer) Run(ctx context.Context, interval time.Duration) {
	for {
		if err := s.SyncOnce(ctx); err != nil {
			log.Printf("airtable sync: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}
	}
}

// SyncOnce does one pass. Anything that fails is retried on the next one.
func (s *Syncer) SyncOnce(ctx context.Context) error {
	return errors.Join(
		s.syncSubmissions(ctx),
		// Before pushing orders, so a newly fulfilled one shows as fulfilled
		// in the same pass.
		s.pullFulfilled(ctx),
		s.syncOrders(ctx),
	)
}

func (s *Syncer) syncSubmissions(ctx context.Context) error {
	rows, err := s.Queries.ListSubmissionsToSync(ctx)
	if err != nil {
		return err
	}
	for batch := range chunks(rows) {
		records := make([]airtable.Record, len(batch))
		for i, row := range batch {
			records[i] = airtable.Record{Fields: s.submissionFields(row)}
		}
		created, err := s.Airtable.Create(ctx, submissionsTable, records)
		if err != nil {
			return fmt.Errorf("adding submissions: %w", err)
		}
		for i, record := range created {
			if err := s.Queries.SetSubmissionAirtableRecord(ctx, db.SetSubmissionAirtableRecordParams{
				ID: batch[i].Submission.ID, AirtableRecordID: &record.ID,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Syncer) submissionFields(row db.ListSubmissionsToSyncRow) airtable.Fields {
	sub, project, user := row.Submission, row.Project, row.User
	hours := math.Round(float64(*sub.ApprovedSeconds)/3600*100) / 100
	hackatimeProjects := strings.Join(sub.HackatimeProjects, ", ")
	f := airtable.Fields{
		"Code URL":        project.RepoUrl,
		"Playable URL":    project.DemoUrl,
		"Description":     project.Description,
		"First Name":      user.FirstName,
		"Last Name":       user.LastName,
		"Email":           user.Email,
		"Phone Number":    user.PhoneNumber,
		"Slack ID":        user.SlackID,
		"GitHub Username": githubUsername(project.RepoUrl),
		"YSWS Eligible":   user.YswsEligible,
		"Hackatime List":  hackatimeProjects,

		"Optional - Override Hours Spent": hours,
		"Optional - Override Hours Spent Justification": fmt.Sprintf(
			"Reviewed on Crossroads. The reviewer approved %.2f of the %.2f new Hackatime hours on this submission.",
			hours, float64(sub.NewSeconds)/3600),
		"Justification - Hackatime Project Name(s) + Date Range(s)": fmt.Sprintf("%s, %s to %s",
			hackatimeProjects, s.HoursStart.Format(time.DateOnly), sub.SubmittedAt.UTC().Format(time.DateOnly)),
		"Justification - Submitter Hackatime ID": user.SlackID,
	}
	if project.ScreenshotUrl != nil {
		f["Screenshot"] = []map[string]string{{"url": *project.ScreenshotUrl}}
	}
	if user.Birthday != nil {
		f["Birthday"] = user.Birthday.Format(time.DateOnly)
	}
	if row.EarlierApprovals > 0 {
		f["Optional - Override Duplicate Justification"] = "An update to a project already approved on Crossroads. " +
			"These hours are only the time added since then; Crossroads never pays for the same Hackatime time twice."
	}
	var address db.Address
	if json.Unmarshal(user.Address, &address) == nil {
		f["Address (Line 1)"] = address.Line1
		f["Address (Line 2)"] = address.Line2
		f["City"] = address.City
		f["State / Province"] = address.State
		f["ZIP / Postal Code"] = address.PostalCode
		f["Country"] = address.Country
	}
	return f
}

// githubUsername returns the owner of a github.com repo link, or nil.
func githubUsername(repoURL *string) *string {
	if repoURL == nil {
		return nil
	}
	u, err := url.Parse(*repoURL)
	if err != nil || strings.TrimPrefix(strings.ToLower(u.Host), "www.") != "github.com" {
		return nil
	}
	owner, _, _ := strings.Cut(strings.Trim(u.Path, "/"), "/")
	if owner == "" {
		return nil
	}
	return &owner
}

// pullFulfilled marks orders fulfilled when someone ticks Fulfilled on them in
// Airtable.
func (s *Syncer) pullFulfilled(ctx context.Context) error {
	records, err := s.Airtable.List(ctx, ordersTable, "AND({Fulfilled}, {Status} = 'pending')", "Order ID")
	if err != nil {
		return fmt.Errorf("checking for fulfilled orders: %w", err)
	}
	for _, record := range records {
		if _, err := s.Queries.FulfillOrderFromAirtable(ctx, &record.ID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Syncer) syncOrders(ctx context.Context) error {
	rows, err := s.Queries.ListOrdersToSync(ctx)
	if err != nil {
		return err
	}
	var fresh, changed []db.ListOrdersToSyncRow
	for _, row := range rows {
		if row.Order.AirtableRecordID == nil {
			fresh = append(fresh, row)
		} else {
			changed = append(changed, row)
		}
	}

	for batch := range chunks(fresh) {
		records := make([]airtable.Record, len(batch))
		for i, row := range batch {
			records[i] = airtable.Record{Fields: orderFields(row)}
		}
		created, err := s.Airtable.Create(ctx, ordersTable, records)
		if err != nil {
			return fmt.Errorf("adding orders: %w", err)
		}
		for i, record := range created {
			if err := s.markSynced(ctx, batch[i], record.ID); err != nil {
				return err
			}
		}
	}
	for batch := range chunks(changed) {
		records := make([]airtable.Record, len(batch))
		for i, row := range batch {
			records[i] = airtable.Record{ID: *row.Order.AirtableRecordID, Fields: orderFields(row)}
		}
		if err := s.Airtable.Update(ctx, ordersTable, records); err != nil {
			return fmt.Errorf("updating orders: %w", err)
		}
		for _, row := range batch {
			if err := s.markSynced(ctx, row, *row.Order.AirtableRecordID); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Syncer) markSynced(ctx context.Context, row db.ListOrdersToSyncRow, recordID string) error {
	return s.Queries.MarkOrderSynced(ctx, db.MarkOrderSyncedParams{
		ID: row.Order.ID, AirtableRecordID: &recordID, AirtableSyncedAt: &row.Order.UpdatedAt,
	})
}

// orderFields are the order's fields the backend owns. Fulfilled and
// Fulfillment Notes are left out so the sync never overwrites them.
func orderFields(row db.ListOrdersToSyncRow) airtable.Fields {
	order, item, user := row.Order, row.ShopItem, row.User
	f := airtable.Fields{
		"Order":       fmt.Sprintf("#%d %s", order.ID, item.Name),
		"Order ID":    order.ID,
		"Item":        item.Name,
		"Kind":        string(item.Kind),
		"Price":       order.Price,
		"Status":      string(order.Status),
		"Name":        user.Name,
		"Email":       user.Email,
		"Slack ID":    user.SlackID,
		"Admin Notes": order.Notes,
		"Ordered At":  order.CreatedAt.UTC().Format(time.RFC3339),
	}
	if item.StipendCents != nil {
		f["Stipend"] = float64(*item.StipendCents) / 100
	}
	var address db.Address
	if json.Unmarshal(order.ShippingAddress, &address) == nil {
		f["Name"] = address.Name
		f["Address (Line 1)"] = address.Line1
		f["Address (Line 2)"] = address.Line2
		f["City"] = address.City
		f["State / Province"] = address.State
		f["ZIP / Postal Code"] = address.PostalCode
		f["Country"] = address.Country
	}
	return f
}

// chunks splits rows into batches Airtable will take in one request.
func chunks[T any](rows []T) func(yield func([]T) bool) {
	return func(yield func([]T) bool) {
		for start := 0; start < len(rows); start += airtable.BatchSize {
			if !yield(rows[start:min(start+airtable.BatchSize, len(rows))]) {
				return
			}
		}
	}
}
