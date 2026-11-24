package api

// Tickets, Please: Slack DMs telling people what happened to their projects.
// The changes that call for one (a ship approved and paid, a rejection, a
// rejection undone, a project allowed to ship again) write it with
// addProjectDM in their own transaction, so there's a DM exactly when the
// change happened. RunProjectDMs sends them with the Slack app's bot, under
// the name Tickets, Please, and tries again later when Slack doesn't take
// one. People can turn them off in the platform's settings (dms_enabled);
// what's waiting for them then isn't sent.
//
// Only the real site DMs people. Every other build sends its DMs to
// DMRedirect, the test channel: dev-db is a copy of the real database, with
// everyone's real Slack IDs.

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/scherepi/crossroads-backend/db"
	"github.com/scherepi/crossroads-backend/slack"
)

const (
	dmName = "Tickets, Please"
	// A DM that hasn't gone out after this long isn't news any more, so it's
	// dropped instead of sent late (say, when a backend first gets a bot
	// token).
	dmFreshFor = 24 * time.Hour
	// Retries wait twice as long each time, up to this.
	dmMaxWait = time.Hour
)

// Slack errors that trying again won't fix.
var dmUndeliverable = map[string]bool{
	"channel_not_found":     true,
	"user_not_found":        true,
	"cannot_dm_bot":         true,
	"is_archived":           true,
	"messages_tab_disabled": true,
	"msg_too_long":          true,
}

// addProjectDM writes a DM to project's owner, to go out once the
// transaction q is in commits. submissionID is the ship it's about, if any.
func addProjectDM(ctx context.Context, q *db.Queries, project db.Project, submissionID *int64, kind db.ProjectDmKind, text string) error {
	_, err := q.AddProjectDM(ctx, db.AddProjectDMParams{
		UserID: project.UserID, ProjectID: project.ID, SubmissionID: submissionID, Kind: kind, Text: text,
	})
	return err
}

// approvedDM is for a ship signed off at s.ApprovedSeconds, which paid
// tickets (Golden Spike included) and brought boxes boxes.
func (a *API) approvedDM(project db.Project, s db.Submission, tickets int64, boxes int) string {
	var b strings.Builder
	fmt.Fprintf(&b, ":beretparrot: %s was approved for %s!", a.projectLink(project), hoursText(int(*s.ApprovedSeconds)))
	switch {
	case tickets > 0 && boxes > 0:
		fmt.Fprintf(&b, " You got %d ticket%s, and %s to open on %s.", tickets, plural(int(tickets)), boxesText(boxes), a.homeLink())
	case tickets > 0:
		fmt.Fprintf(&b, " You got %d ticket%s.", tickets, plural(int(tickets)))
	case boxes > 0:
		fmt.Fprintf(&b, " You got %s to open on %s.", boxesText(boxes), a.homeLink())
	}
	if feedback := deref(s.Feedback); feedback != "" {
		b.WriteString("\nYour reviewer said:\n" + quoted(feedback))
	}
	return b.String()
}

func boxesText(n int) string {
	if n == 1 {
		return "a box"
	}
	return strconv.Itoa(n) + " boxes"
}

// rejectedDM is for a rejection with feedback. A hard reject (blocked) means
// the project can't be shipped again. The blank line after the feedback ends
// the quote, so what follows isn't read as the reviewer's.
func (a *API) rejectedDM(project db.Project, feedback string, blocked bool, forfeited int64) string {
	if blocked {
		taken := ""
		if forfeited > 0 {
			taken = fmt.Sprintf(", and the %d ticket%s it earned were taken back", forfeited, plural(int(forfeited)))
		}
		return fmt.Sprintf("Sorry, %s was rejected and can't be shipped again%s :pensivecowboy: Your reviewer said:\n%s\n\nIf you think that's a mistake, ask an organizer, like guac or max.",
			a.projectLink(project), taken, quoted(feedback))
	}
	return fmt.Sprintf("%s wasn't approved this time. Your reviewer said:\n%s\n\nMake some changes and ship it again!",
		a.projectLink(project), quoted(feedback))
}

func (a *API) reopenedDM(project db.Project, restored int64) string {
	return fmt.Sprintf("Good news: the rejection of %s was undone, so it's back in review%s. A reviewer will look it back over soon.",
		a.projectLink(project), givenBack(restored))
}

func (a *API) resubmitAllowedDM(project db.Project, restored int64) string {
	return fmt.Sprintf("Good news: %s can be shipped again%s. Ship it from its page whenever you're ready.", a.projectLink(project), givenBack(restored))
}

// givenBack ends a sentence about undoing a hard reject with the tickets it
// gave back, if any.
func givenBack(restored int64) string {
	if restored == 0 {
		return ""
	}
	return fmt.Sprintf(", and the %d ticket%s it earned are yours again", restored, plural(int(restored)))
}

// projectLink is the project's name in bold, linking to its page.
func (a *API) projectLink(p db.Project) string {
	return fmt.Sprintf("*<%s/projects/%d|%s>*", strings.TrimSuffix(a.PublicURL, "/"), p.ID, slackEscape(p.Name))
}

func (a *API) homeLink() string {
	return "<" + strings.TrimSuffix(a.PublicURL, "/") + "/home|Crossroads>"
}

// quoted is a reviewer's words as a Slack quote.
func quoted(text string) string {
	return "> " + strings.ReplaceAll(slackEscape(strings.TrimSpace(text)), "\n", "\n> ")
}

// slackEscape keeps text people wrote from being read as Slack links or
// mentions.
var slackEscape = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace

// RunProjectDMs sends the DMs that are due every interval until ctx is done.
func (a *API) RunProjectDMs(ctx context.Context, interval time.Duration) {
	for {
		if err := a.SendProjectDMs(ctx); err != nil {
			log.Printf("sending project DMs: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}
	}
}

// SendProjectDMs tries each DM that's due once. Without a Slack bot token
// they wait, until they're too old to send.
func (a *API) SendProjectDMs(ctx context.Context) error {
	if a.Slack == nil {
		return nil
	}
	rows, err := a.Queries.ListProjectDMsToSend(ctx)
	if err != nil {
		return err
	}
	var errs []error
	for _, row := range rows {
		err := a.sendProjectDM(ctx, row)
		if err == nil {
			continue
		}
		errs = append(errs, fmt.Errorf("DM %d: %w", row.ProjectDm.ID, err))
		// Slack wants a break; the rest wait for the next pass.
		var slackErr *slack.Error
		if errors.As(err, &slackErr) && slackErr.Code == "ratelimited" {
			break
		}
	}
	return errors.Join(errs...)
}

// sendProjectDM tries to send one DM and records how it went. It returns
// what went wrong, if anything, for the log.
func (a *API) sendProjectDM(ctx context.Context, row db.ListProjectDMsToSendRow) error {
	dm := row.ProjectDm
	channel, text := a.DMRedirect, dm.Text
	switch {
	case time.Since(dm.CreatedAt) > dmFreshFor:
		return a.abandonProjectDM(ctx, dm, "It waited too long to still be news.")
	case !row.DmsEnabled:
		return a.abandonProjectDM(ctx, dm, "They turned DMs off.")
	case channel != "":
		to := "no Slack ID"
		if row.SlackID != nil {
			to = *row.SlackID
		}
		text = fmt.Sprintf("_To %s (%s), sent here because this isn't the real site:_\n%s", slackEscape(row.Name), to, text)
	case row.SlackID == nil:
		return a.abandonProjectDM(ctx, dm, "They have no Slack ID.")
	default:
		channel = *row.SlackID
	}

	posted, ts, err := a.Slack.Post(ctx, slack.Message{Channel: channel, Text: text, Username: dmName})
	if err == nil {
		return a.Queries.MarkProjectDMSent(ctx, db.MarkProjectDMSentParams{ID: dm.ID, ChannelID: &posted, MessageTs: &ts})
	}
	var slackErr *slack.Error
	isSlack := errors.As(err, &slackErr)
	if isSlack && dmUndeliverable[slackErr.Code] {
		return a.abandonProjectDM(ctx, dm, err.Error())
	}
	wait := min(time.Minute<<min(dm.Attempts, 6), dmMaxWait)
	if isSlack && slackErr.RetryAfter > 0 {
		wait = slackErr.RetryAfter
	}
	return errors.Join(err, a.Queries.RetryProjectDM(ctx, db.RetryProjectDMParams{
		ID: dm.ID, Error: new(err.Error()), NextTryAt: time.Now().Add(wait),
	}))
}

func (a *API) abandonProjectDM(ctx context.Context, dm db.ProjectDm, reason string) error {
	if err := a.Queries.AbandonProjectDM(ctx, db.AbandonProjectDMParams{ID: dm.ID, Error: &reason}); err != nil {
		return err
	}
	return errors.New("gave up: " + reason)
}
