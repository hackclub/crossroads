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
	"strings"
	"time"

	"github.com/scherepi/crossroads-backend/airtable"
	"github.com/scherepi/crossroads-backend/db"
	"github.com/scherepi/crossroads-backend/unified"
)

const (
	// The YSWS template table that feeds the Unified YSWS DB. The sync ticks
	// its "Automation - Submit to Unified YSWS" box on rows HQ has signed off
	// on Crossroads, once the person is YSWS-eligible. Rows from before HQ
	// sign-off existed are left for a person to tick.
	submissionsTable   = "YSWS Project Submission"
	submitUnifiedField = "Automation - Submit to Unified YSWS"
	ordersTable        = "Shop Orders"
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
		s.submitUnified(ctx),
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
			if readyForUnified(batch[i]) {
				if err := s.Queries.MarkUnifiedSubmitted(ctx, batch[i].Submission.ID); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// readyForUnified says whether a row can go to the Unified YSWS DB: HQ signed
// it off, its owner is YSWS-eligible, and it has everything the Unified YSWS
// DB needs (the review page's unified_ready check says what's missing).
func readyForUnified(row db.ListSubmissionsToSyncRow) bool {
	return row.Submission.HqReviewerID != nil && row.User.YswsEligible && len(unified.Missing(row.User, row.Project)) == 0
}

// submitUnified sends rows that went into Airtable before they were ready
// (their owner wasn't YSWS-eligible yet, or a name, address or screenshot was
// missing) once they are. The whole row goes again, with the box ticked, so
// the Unified YSWS DB gets what was filled in since.
func (s *Syncer) submitUnified(ctx context.Context) error {
	waiting, err := s.Queries.ListSubmissionsToSubmitUnified(ctx)
	if err != nil {
		return err
	}
	var rows []db.ListSubmissionsToSyncRow
	for _, w := range waiting {
		if row := db.ListSubmissionsToSyncRow(w); readyForUnified(row) {
			rows = append(rows, row)
		}
	}
	for batch := range chunks(rows) {
		records := make([]airtable.Record, len(batch))
		for i, row := range batch {
			records[i] = airtable.Record{ID: *row.Submission.AirtableRecordID, Fields: s.submissionFields(row)}
		}
		if err := s.Airtable.Update(ctx, submissionsTable, records); err != nil {
			return fmt.Errorf("submitting to Unified YSWS: %w", err)
		}
		for _, row := range batch {
			if err := s.Queries.MarkUnifiedSubmitted(ctx, row.Submission.ID); err != nil {
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
		"GitHub Username": unified.GitHubUsername(project.RepoUrl),
		"YSWS Eligible":   user.YswsEligible,
		"Hackatime List":  hackatimeProjects,

		"Optional - Override Hours Spent":               hours,
		"Optional - Override Hours Spent Justification": s.justification(row),
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
	if readyForUnified(row) {
		f[submitUnifiedField] = true
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

// justification explains the hours to whoever audits the Unified YSWS DB:
// where the time came from, and who checked it at each stage.
func (s *Syncer) justification(row db.ListSubmissionsToSyncRow) string {
	sub := row.Submission
	h := func(seconds int32) string { return fmt.Sprintf("%.2fh", float64(seconds)/3600) }
	var b strings.Builder
	fmt.Fprintf(&b, "Reviewed on Crossroads. Hackatime counted %s of new time on this ship (Hackatime projects: %s; %s on them from %s to %s, minus time already paid for).",
		h(sub.NewSeconds), strings.Join(sub.HackatimeProjects, ", "), h(sub.HackatimeSeconds),
		s.HoursStart.Format(time.DateOnly), sub.SubmittedAt.UTC().Format(time.DateOnly))
	reviewerSeconds := *sub.ApprovedSeconds
	if row.ReviewerSeconds != nil {
		reviewerSeconds = *row.ReviewerSeconds
	}
	if row.ReviewerName != "" {
		fmt.Fprintf(&b, " Reviewer %s approved %s.", row.ReviewerName, h(reviewerSeconds))
	}
	if sub.OverrideReason != nil && *sub.ApprovedSeconds > sub.NewSeconds {
		fmt.Fprintf(&b, " Approved for more than Hackatime tracked (%s of %s), because: %s",
			h(*sub.ApprovedSeconds), h(sub.NewSeconds), strings.TrimSpace(*sub.OverrideReason))
		if !strings.HasSuffix(*sub.OverrideReason, ".") {
			b.WriteString(".")
		}
	}
	if sub.InternalNotes != nil {
		fmt.Fprintf(&b, " Reviewer's notes: %s", strings.TrimSpace(*sub.InternalNotes))
		if !strings.HasSuffix(*sub.InternalNotes, ".") {
			b.WriteString(".")
		}
	}
	switch {
	case row.FraudReviewerName != "":
		fmt.Fprintf(&b, " Flagged for a fraud check (%s), which %s passed.", strings.Join(sub.FraudReasons, ", "), row.FraudReviewerName)
	case sub.HqReviewerID != nil:
		b.WriteString(" No automatic check flagged it for a fraud check (Hackatime trust level, shared links, ship size, longest day, Hackatime time going down).")
	}
	if row.HqReviewerName != "" {
		fmt.Fprintf(&b, " Signed off by %s at %s.", row.HqReviewerName, h(*sub.ApprovedSeconds))
	}
	if row.EarlierApprovals > 0 {
		b.WriteString(" This is an update to a project already approved on Crossroads: only time added since then is counted.")
	}
	return b.String()
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
