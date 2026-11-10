package api

// The review stages. A ship (a submission) moves through them like this:
//
//	pending ──approve──▶ fraud_check ──pass──▶ awaiting_hq ──approve──▶ approved
//	   │         │            │                     │
//	   │         └──(HQ, not flagged)───────────────┼──────────────────▶ approved
//	   └──reject──▶ rejected ◀──reject──────────────┘
//	             fraud_check and awaiting_hq can also send it back to pending.
//
// A reviewer's approval goes to the fraud check when the automatic checks
// flag it, when the reviewer asks, or when an admin has switched on "every
// ship goes through the fraud check". Otherwise it waits for HQ, unless the
// reviewer has HQ powers themselves, in which case their approval is the
// sign-off. Stamps, boxes and the Unified YSWS submission all wait for the
// sign-off (finalize).
//
// The in-house review pages (review.go) and Sidekick (sidekick.go) both go
// through decide, so the rules are the same whichever one a reviewer uses.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/scherepi/crossroads-backend/db"
)

// Queue names in the API, and the status each one holds.
var stageStatuses = map[string]db.SubmissionStatus{
	"review": db.SubmissionStatusPending,
	"fraud":  db.SubmissionStatusFraudCheck,
	"hq":     db.SubmissionStatusAwaitingHq,
}

func stageName(status db.SubmissionStatus) string {
	for name, s := range stageStatuses {
		if s == status {
			return name
		}
	}
	return ""
}

// inReview is true for a ship at any stage before the end.
func inReview(status db.SubmissionStatus) bool {
	return status == db.SubmissionStatusPending || status == db.SubmissionStatusFraudCheck || status == db.SubmissionStatusAwaitingHq
}

// claimLength is how long a claim lasts; the queries hard-code it too.
const claimLength = 30 * time.Minute

// permissions are which stages someone can decide. Admins can do all of them.
type permissions struct {
	Review      bool `json:"review"`
	FraudReview bool `json:"fraud_review"`
	HQReview    bool `json:"hq_review"`
}

func permissionsOf(u *db.User) permissions {
	if u.Role == db.UserRoleAdmin {
		return permissions{true, true, true}
	}
	if u.Role != db.UserRoleReviewer {
		return permissions{}
	}
	return permissions{u.CanReview, u.CanFraudReview, u.CanHqReview}
}

// canDecide says whether p covers a ship at status.
func (p permissions) canDecide(status db.SubmissionStatus) bool {
	switch status {
	case db.SubmissionStatusPending:
		return p.Review
	case db.SubmissionStatusFraudCheck:
		return p.FraudReview
	case db.SubmissionStatusAwaitingHq:
		return p.HQReview
	}
	return false
}

var stageLabels = map[db.SubmissionStatus]string{
	db.SubmissionStatusPending:    "review",
	db.SubmissionStatusFraudCheck: "the fraud check",
	db.SubmissionStatusAwaitingHq: "HQ sign-off",
}

// reviewError is a problem to show the reviewer as is, with its HTTP status.
type reviewError struct {
	Status  int
	Message string
}

func (e *reviewError) Error() string { return e.Message }

func problem(status int, format string, args ...any) error {
	return &reviewError{status, fmt.Sprintf(format, args...)}
}

// decision is what a reviewer decided about a ship.
type decision struct {
	// approve, reject or send_back. What approving means depends on the
	// stage: deciding the hours, passing the fraud check, or signing off.
	Action string `json:"action"`
	// The status the reviewer saw. If the ship has moved on since, the
	// decision is refused rather than applied to a stage they didn't mean.
	// Optional.
	Stage db.SubmissionStatus `json:"stage"`
	// Approvals: how much time to accept. Defaults to the reviewer's number
	// (all of new_seconds at the first stage). The fraud check can only cut
	// it; HQ can set anything up to new_seconds.
	ApprovedSeconds *int32 `json:"approved_seconds"`
	Feedback        string `json:"feedback"`       // shown to the participant; required to reject
	InternalNotes   string `json:"internal_notes"` // reviewers only; required to send back
	// First stage only: send it to the fraud check even if nothing's flagged.
	SendToFraud bool `json:"send_to_fraud"`
	// Why approved_seconds is more than Hackatime tracked (new_seconds).
	// Required to approve more than that, at the first stage or HQ; the fraud
	// check can't. An approval over Hackatime always waits for HQ sign-off.
	OverrideReason string `json:"override_reason"`
}

type decisionOutcome struct {
	Submission db.Submission
	StampsPaid int64
	Boxes      []db.Box
	Event      db.ShipEvent // the decision itself
}

// decide applies a decision to a ship at whatever stage it's at. perms are
// what the reviewer may do: their own permissions on the review pages, or
// what Sidekick vouches for.
func (a *API) decide(ctx context.Context, reviewer *db.User, id int64, d decision, perms permissions) (decisionOutcome, error) {
	var out decisionOutcome
	d.Feedback = strings.TrimSpace(d.Feedback)
	d.InternalNotes = strings.TrimSpace(d.InternalNotes)
	d.OverrideReason = strings.TrimSpace(d.OverrideReason)
	switch {
	case d.Action != "approve" && d.Action != "reject" && d.Action != "send_back":
		return out, problem(http.StatusUnprocessableEntity, `action has to be "approve", "reject" or "send_back".`)
	case d.Stage != "" && !d.Stage.Valid():
		return out, problem(http.StatusUnprocessableEntity, "stage isn't a status.")
	case d.Action == "reject" && d.Feedback == "":
		return out, problem(http.StatusUnprocessableEntity, "Tell them why it was rejected.")
	case d.Action == "send_back" && d.InternalNotes == "":
		return out, problem(http.StatusUnprocessableEntity, "Say why you're sending it back, for the reviewer who picks it up.")
	case d.Action != "approve" && d.ApprovedSeconds != nil:
		return out, problem(http.StatusUnprocessableEntity, "Only approvals take approved_seconds.")
	case d.Action != "approve" && d.SendToFraud:
		return out, problem(http.StatusUnprocessableEntity, "Only approvals can be sent to the fraud check.")
	case d.Action != "approve" && d.OverrideReason != "":
		return out, problem(http.StatusUnprocessableEntity, "Only approvals can go over Hackatime's hours.")
	case len(d.Feedback) > 5000 || len(d.InternalNotes) > 5000 || len(d.OverrideReason) > 5000:
		return out, problem(http.StatusUnprocessableEntity, "Keep feedback and notes under 5,000 characters.")
	}

	row, err := a.Queries.GetSubmissionForReview(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, problem(http.StatusNotFound, "That submission doesn't exist.")
	}
	if err != nil {
		return out, err
	}
	// A first-stage approval runs the automatic checks, which ask Hackatime.
	// That's slow, so it happens before taking any locks. If the ship moves on
	// in the meantime, the stage check below refuses the decision.
	var report checkReport
	settings, err := a.Queries.GetReviewSettings(ctx)
	if err != nil {
		return out, err
	}
	if row.Submission.Status == db.SubmissionStatusPending && d.Action == "approve" {
		if report, err = a.runChecks(ctx, row, settings); err != nil {
			return out, err
		}
	}

	tx, err := a.Pool.Begin(ctx)
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	q := a.Queries.WithTx(tx)

	owner := row.User
	if owner.ID == reviewer.ID {
		return out, problem(http.StatusForbidden, "You can't review your own project.")
	}
	// Decisions for the same person go one at a time, so each payout sees the
	// running total the last one left.
	if _, err := q.LockUser(ctx, owner.ID); err != nil {
		return out, err
	}
	s, err := q.LockSubmission(ctx, id)
	if err != nil {
		return out, err
	}
	before := s.Status
	switch {
	case !inReview(s.Status):
		return out, problem(http.StatusConflict, "This submission has already been %s.", s.Status)
	case d.Stage != "" && d.Stage != s.Status:
		return out, problem(http.StatusConflict, "Someone else got to this first: it's at %s now. Reload to see what changed.", stageLabels[s.Status])
	case !perms.canDecide(s.Status):
		return out, problem(http.StatusForbidden, "You can't decide ships at %s.", stageLabels[s.Status])
	}
	if err := checkClaim(ctx, q, s, reviewer); err != nil {
		return out, err
	}

	now := time.Now()
	event := db.AddShipEventParams{SubmissionID: s.ID, ActorID: &reviewer.ID, Stage: s.Status,
		Message: nonEmpty(d.Feedback), Internal: nonEmpty(d.InternalNotes)}
	var flagged *db.AddShipEventParams
	finalize := false

	switch {
	case d.Action == "reject":
		event.Kind = db.ShipEventKindRejection
		switch s.Status {
		case db.SubmissionStatusPending:
			s.ReviewerID, s.ReviewedAt = &reviewer.ID, &now
			s.InternalNotes = nonEmpty(d.InternalNotes)
		case db.SubmissionStatusFraudCheck:
			s.FraudReviewerID, s.FraudReviewedAt = &reviewer.ID, &now
		case db.SubmissionStatusAwaitingHq:
			s.HqReviewerID, s.HqReviewedAt = &reviewer.ID, &now
		}
		s.Status, s.ApprovedSeconds, s.Feedback = db.SubmissionStatusRejected, nil, nonEmpty(d.Feedback)

	case d.Action == "send_back":
		if s.Status == db.SubmissionStatusPending {
			return out, problem(http.StatusConflict, "It's already waiting for a reviewer.")
		}
		event.Kind = db.ShipEventKindSentBack
		s = sentBack(s)

	case s.Status == db.SubmissionStatusPending: // approving the hours
		seconds := s.NewSeconds
		if d.ApprovedSeconds != nil {
			seconds = *d.ApprovedSeconds
		}
		if err := checkOverride(&s, seconds, d.OverrideReason); err != nil {
			return out, err
		}
		event.Kind, event.Seconds = db.ShipEventKindApproval, &seconds
		event.Internal = withOverride(event.Internal, s, seconds)
		s.ApprovedSeconds, s.Feedback, s.InternalNotes = &seconds, nonEmpty(d.Feedback), nonEmpty(d.InternalNotes)
		s.ReviewerID, s.ReviewedAt = &reviewer.ID, &now
		s.Checks, err = json.Marshal(report.Checks)
		if err != nil {
			return out, err
		}
		s.FraudReasons = report.fraudReasons()
		if d.SendToFraud {
			s.FraudReasons = append(s.FraudReasons, "manual")
		}
		if settings.FraudCheckEverything {
			s.FraudReasons = append(s.FraudReasons, "everything")
		}
		switch {
		case len(s.FraudReasons) > 0:
			s.Status = db.SubmissionStatusFraudCheck
			flagged = &db.AddShipEventParams{SubmissionID: s.ID, Kind: db.ShipEventKindFlagged,
				Stage: db.SubmissionStatusFraudCheck, Internal: new(fraudReasonText(s.FraudReasons, report.Checks))}
		case perms.HQReview && s.OverrideReason == nil:
			// Over Hackatime always takes a second person, so no shortcut.
			s.Status, s.HqReviewerID, s.HqReviewedAt = db.SubmissionStatusApproved, &reviewer.ID, &now
			finalize = true
		default:
			s.Status = db.SubmissionStatusAwaitingHq
		}

	case s.Status == db.SubmissionStatusFraudCheck: // passing the fraud check
		if d.OverrideReason != "" {
			return out, problem(http.StatusUnprocessableEntity, "The fraud check can only cut hours, so it can't go over Hackatime.")
		}
		seconds := *s.ApprovedSeconds
		if d.ApprovedSeconds != nil {
			seconds = *d.ApprovedSeconds
		}
		if seconds < 0 || seconds > *s.ApprovedSeconds {
			return out, problem(http.StatusUnprocessableEntity, "The fraud check can only cut hours: approve at most the %s the reviewer approved.", hoursText(int(*s.ApprovedSeconds)))
		}
		event.Kind, event.Seconds = db.ShipEventKindFraudPass, &seconds
		s.ApprovedSeconds = &seconds
		if seconds <= s.NewSeconds {
			s.OverrideReason = nil // cut back to what Hackatime tracked
		}
		s.FraudReviewerID, s.FraudReviewedAt = &reviewer.ID, &now
		s.Status = db.SubmissionStatusAwaitingHq

	case s.Status == db.SubmissionStatusAwaitingHq: // HQ sign-off
		seconds := *s.ApprovedSeconds
		if d.ApprovedSeconds != nil {
			seconds = *d.ApprovedSeconds
		}
		if err := checkOverride(&s, seconds, d.OverrideReason); err != nil {
			return out, err
		}
		event.Kind, event.Seconds = db.ShipEventKindFinalApproval, &seconds
		if d.OverrideReason != "" {
			event.Internal = withOverride(event.Internal, s, seconds)
		}
		s.ApprovedSeconds = &seconds
		if d.Feedback != "" {
			s.Feedback = nonEmpty(d.Feedback)
		}
		s.Status, s.HqReviewerID, s.HqReviewedAt = db.SubmissionStatusApproved, &reviewer.ID, &now
		finalize = true
	}

	saved, err := q.SaveSubmissionReview(ctx, db.SaveSubmissionReviewParams{
		ID: s.ID, Status: s.Status, ApprovedSeconds: s.ApprovedSeconds, Feedback: s.Feedback, InternalNotes: s.InternalNotes,
		ReviewerID: s.ReviewerID, ReviewedAt: s.ReviewedAt, Checks: s.Checks, FraudReasons: s.FraudReasons,
		FraudReviewerID: s.FraudReviewerID, FraudReviewedAt: s.FraudReviewedAt, HqReviewerID: s.HqReviewerID, HqReviewedAt: s.HqReviewedAt,
		OverrideReason: s.OverrideReason,
	})
	if err != nil {
		return out, err
	}
	out.Submission = saved
	event.Stage = before
	if out.Event, err = q.AddShipEvent(ctx, event); err != nil {
		return out, err
	}
	if flagged != nil {
		if _, err := q.AddShipEvent(ctx, *flagged); err != nil {
			return out, err
		}
	}
	if finalize {
		if out.StampsPaid, out.Boxes, err = a.payOut(ctx, q, owner.ID, saved); err != nil {
			return out, err
		}
	}
	return out, tx.Commit(ctx)
}

// sentBack is a ship returned to the first stage, with every decision on it
// cleared. Its events keep the history.
func sentBack(s db.Submission) db.Submission {
	s.Status = db.SubmissionStatusPending
	s.ApprovedSeconds, s.Feedback, s.InternalNotes = nil, nil, nil
	s.ReviewerID, s.ReviewedAt = nil, nil
	s.Checks, s.FraudReasons = nil, []string{}
	s.FraudReviewerID, s.FraudReviewedAt = nil, nil
	s.HqReviewerID, s.HqReviewedAt = nil, nil
	s.OverrideReason = nil
	return s
}

// checkOverride checks approving seconds on a ship, and sets or clears its
// override reason. More than Hackatime tracked needs a reason: a new one, or
// (at HQ) the one the reviewer already gave.
func checkOverride(s *db.Submission, seconds int32, reason string) error {
	switch {
	case seconds < 0:
		return problem(http.StatusUnprocessableEntity, "approved_seconds can't be negative.")
	case seconds <= s.NewSeconds:
		s.OverrideReason = nil
	case reason != "":
		s.OverrideReason = &reason
	case s.OverrideReason == nil:
		return problem(http.StatusUnprocessableEntity,
			"That's more than the %s Hackatime tracked. Tick the override and say why.", hoursText(int(s.NewSeconds)))
	}
	return nil
}

// withOverride adds the override reason to an approval's notes, so the
// timeline shows it.
func withOverride(notes *string, s db.Submission, seconds int32) *string {
	if s.OverrideReason == nil {
		return notes
	}
	line := fmt.Sprintf("Over Hackatime (%s of %s tracked): %s", hoursText(int(seconds)), hoursText(int(s.NewSeconds)), *s.OverrideReason)
	if notes == nil {
		return &line
	}
	return new(*notes + "\n" + line)
}

// checkClaim refuses to let reviewer act on a ship someone else is holding.
func checkClaim(ctx context.Context, q *db.Queries, s db.Submission, reviewer *db.User) error {
	if s.ClaimedBy == nil || *s.ClaimedBy == reviewer.ID || !s.ClaimExpiresAt.After(time.Now()) {
		return nil
	}
	holder, err := q.GetUser(ctx, *s.ClaimedBy)
	if err != nil {
		return err
	}
	return problem(http.StatusConflict, "%s is reviewing this right now (until %s UTC).", holder.Name, s.ClaimExpiresAt.UTC().Format("15:04"))
}

// payOut pays stamps for a signed-off ship and grants a box for every
// milestone its owner has now reached. The owner's row has to be locked.
func (a *API) payOut(ctx context.Context, q *db.Queries, ownerID int64, s db.Submission) (int64, []db.Box, error) {
	after, err := q.ApprovedSecondsForUser(ctx, ownerID)
	if err != nil {
		return 0, nil, err
	}
	before := after - int64(*s.ApprovedSeconds)
	// Paying on the running total means rounding never loses anyone a
	// stamp: 60 approved hours is always exactly 60 hours of stamps,
	// however it was split up.
	paid := a.stampsFor(after) - a.stampsFor(before)
	if paid > 0 {
		if _, err := q.AddStampEntry(ctx, db.AddStampEntryParams{
			UserID: ownerID, Kind: db.StampEntryKindPayout, Amount: int32(paid), SubmissionID: &s.ID,
		}); err != nil {
			return 0, nil, err
		}
		// A Golden Spike on the project pays 10% more, as its own entry.
		spiked, err := q.ProjectHasStampMultiplier(ctx, &s.ProjectID)
		if err != nil {
			return 0, nil, err
		}
		if bonus := stampBonus(paid); spiked && bonus > 0 {
			if _, err := q.AddStampEntry(ctx, db.AddStampEntryParams{
				UserID: ownerID, Kind: db.StampEntryKindBonus, Amount: int32(bonus), SubmissionID: &s.ID,
			}); err != nil {
				return 0, nil, err
			}
			paid += bonus
		}
	}
	boxes, err := q.GrantMilestoneBoxes(ctx, db.GrantMilestoneBoxesParams{UserID: ownerID, ApprovedSeconds: int32(after)})
	return paid, boxes, err
}

// addComment adds a comment to a ship: for the participant, or internal for
// reviewers only.
func (a *API) addComment(ctx context.Context, author *db.User, submissionID int64, message string, internal bool) (db.ShipEvent, error) {
	message = strings.TrimSpace(message)
	switch {
	case message == "":
		return db.ShipEvent{}, problem(http.StatusUnprocessableEntity, "Write something first.")
	case len(message) > 5000:
		return db.ShipEvent{}, problem(http.StatusUnprocessableEntity, "Keep comments under 5,000 characters.")
	}
	row, err := a.Queries.GetSubmissionForReview(ctx, submissionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return db.ShipEvent{}, problem(http.StatusNotFound, "That submission doesn't exist.")
	}
	if err != nil {
		return db.ShipEvent{}, err
	}
	if row.User.ID == author.ID {
		return db.ShipEvent{}, problem(http.StatusForbidden, "You can't review your own project.")
	}
	params := db.AddShipEventParams{SubmissionID: submissionID, Kind: db.ShipEventKindComment, ActorID: &author.ID,
		Stage: row.Submission.Status, Message: &message}
	if internal {
		params.Kind, params.Message, params.Internal = db.ShipEventKindInternalComment, nil, &message
	}
	return a.Queries.AddShipEvent(ctx, params)
}

// editDecision changes the words on a reviewer's latest approval or
// rejection of a ship (Sidekick's UPDATE_REVIEW_ACTION). Hours can only
// change while the approval is waiting for the fraud check or HQ: once it's
// signed off, stamps have been paid on it.
func (a *API) editDecision(ctx context.Context, actor *db.User, submissionID int64, kind db.ShipEventKind,
	feedback string, internal *string, seconds *int32) error {
	feedback = strings.TrimSpace(feedback)
	if len(feedback) > 5000 || (internal != nil && len(*internal) > 5000) {
		return problem(http.StatusUnprocessableEntity, "Keep feedback and notes under 5,000 characters.")
	}
	tx, err := a.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := a.Queries.WithTx(tx)
	s, err := q.LockSubmission(ctx, submissionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return problem(http.StatusNotFound, "That submission doesn't exist.")
	}
	if err != nil {
		return err
	}
	event, err := q.LockLatestShipEvent(ctx, db.LockLatestShipEventParams{SubmissionID: s.ID, Kind: kind, ActorID: &actor.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		return problem(http.StatusNotFound, "There's no %s by them on that ship to edit.", kind)
	}
	if err != nil {
		return err
	}
	// Whether this event is the decision the ship is at now, as opposed to
	// one that was sent back or superseded.
	current := s.ReviewerID != nil && *s.ReviewerID == actor.ID &&
		((kind == db.ShipEventKindApproval && s.ApprovedSeconds != nil) || (kind == db.ShipEventKindRejection && s.Status == db.SubmissionStatusRejected))
	if seconds != nil {
		switch {
		case kind != db.ShipEventKindApproval:
			return problem(http.StatusBadRequest, "Only approvals have hours.")
		case !current || (s.Status != db.SubmissionStatusFraudCheck && s.Status != db.SubmissionStatusAwaitingHq):
			return problem(http.StatusBadRequest, "Hours can only change before the approval is signed off; stamps may already have been paid on it.")
		case *seconds < 0 || (*seconds > s.NewSeconds && s.OverrideReason == nil):
			return problem(http.StatusBadRequest, "Hours have to be between 0 and what Hackatime tracked, unless the approval gave a reason for going over.")
		}
		event.Seconds = seconds
		s.ApprovedSeconds = seconds
	}
	notes := event.Internal // left as it was unless a new one was sent
	if internal != nil {
		notes = nonEmpty(strings.TrimSpace(*internal))
	}
	if _, err := q.EditShipEvent(ctx, db.EditShipEventParams{ID: event.ID, Seconds: event.Seconds, Message: nonEmpty(feedback), Internal: notes}); err != nil {
		return err
	}
	if current {
		if err := q.SetSubmissionMessages(ctx, db.SetSubmissionMessagesParams{ID: s.ID, ApprovedSeconds: s.ApprovedSeconds,
			Feedback: nonEmpty(feedback), InternalNotes: notes}); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
