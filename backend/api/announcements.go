package api

// Announcements: superadmins write them on the admin page's Announce tab and
// post them to the station channel with the Slack app's bot, under their own
// Slack name and picture. The platform builds the Block Kit blocks, since its
// editor and preview need them anyway; this checks and stores them, posts
// them, and says how they did: clicks on their buttons (recorded by the bot
// in slackbot/ and package slackclicks), stamps claimed through them, and
// the reactions and replies Slack reports.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/scherepi/crossroads-backend/auth"
	"github.com/scherepi/crossroads-backend/db"
	"github.com/scherepi/crossroads-backend/respond"
	"github.com/scherepi/crossroads-backend/slack"
)

type announcementJSON struct {
	ID        int64           `json:"id"`
	Doc       json.RawMessage `json:"doc"` // the editor's copy, for the platform
	Blocks    json.RawMessage `json:"blocks"`
	Text      string          `json:"text"`
	Buttons   json.RawMessage `json:"buttons"` // []announcementButton
	Pings     bool            `json:"pings"`   // @channel
	CreatedBy int64           `json:"created_by"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
	// All null until it's posted.
	PostedBy  *int64     `json:"posted_by"`
	PostedAt  *time.Time `json:"posted_at"`
	ChannelID *string    `json:"channel_id"`
	MessageTS *string    `json:"message_ts"`
	Permalink *string    `json:"permalink"`
	DeletedAt *time.Time `json:"deleted_at"` // deleted from Slack
}

func toAnnouncementJSON(a db.Announcement) announcementJSON {
	return announcementJSON{a.ID, a.Doc, a.Blocks, a.Text, a.Buttons, a.Pings, a.CreatedBy, a.CreatedAt, a.UpdatedAt,
		a.PostedBy, a.PostedAt, a.ChannelID, a.MessageTs, a.Permalink, a.DeletedAt}
}

// announcementButton is a button in an announcement's blocks. Clicks are
// counted by action_id, so the platform keeps a button's when it's edited.
type announcementButton struct {
	ActionID string `json:"action_id"`
	Text     string `json:"text"`
	URL      string `json:"url,omitempty"`
	// For a claim link (/claim/{code}): whose stamp it gives.
	SpecialStampTypeID *int64 `json:"special_stamp_type_id,omitempty"`
}

// announcementFields is the body of POST and PATCH
// /superadmin/announcements.
type announcementFields struct {
	Doc    json.RawMessage `json:"doc"`
	Blocks json.RawMessage `json:"blocks"`
	Text   string          `json:"text"`
}

// checked is an announcement's content once checkAnnouncement has passed it.
type checked struct {
	fields  announcementFields
	buttons []byte
	pings   bool
}

// checkAnnouncement checks the blocks and text are a message Slack will take
// and works out its buttons and whether it pings. If it returns a problem,
// it's for people.
func (a *API) checkAnnouncement(ctx context.Context, f announcementFields) (checked, string, error) {
	f.Text = strings.TrimSpace(f.Text)
	var doc map[string]any
	if json.Unmarshal(f.Doc, &doc) != nil || doc == nil {
		return checked{}, "doc has to be the editor's content, as an object.", nil
	}
	var blocks []any
	if err := json.Unmarshal(f.Blocks, &blocks); err != nil {
		return checked{}, "blocks has to be a list of Block Kit blocks.", nil
	}
	// Slack's limits.
	if len(blocks) == 0 || len(blocks) > 50 {
		return checked{}, fmt.Sprintf("A Slack message has 1 to 50 blocks, and this has %d.", len(blocks)), nil
	}
	if f.Text == "" || len([]rune(f.Text)) > 4000 {
		return checked{}, "The notification text has to be 1 to 4,000 characters.", nil
	}
	for _, block := range blocks {
		if b, ok := block.(map[string]any); !ok || b["type"] == nil {
			return checked{}, "Every block needs a type.", nil
		}
	}

	var buttons []announcementButton
	var pings bool
	var problem string
	var walk func(v any)
	walk = func(v any) {
		switch v := v.(type) {
		case []any:
			for _, item := range v {
				walk(item)
			}
		case map[string]any:
			switch v["type"] {
			case "broadcast":
				pings = true
			case "button":
				button := announcementButton{}
				button.ActionID, _ = v["action_id"].(string)
				if label, ok := v["text"].(map[string]any); ok {
					button.Text, _ = label["text"].(string)
				}
				button.URL, _ = v["url"].(string)
				switch {
				case button.ActionID == "" || len(button.ActionID) > 255:
					problem = fmt.Sprintf("The button %q needs an action_id of up to 255 characters; clicks are counted by it.", button.Text)
				case slices.ContainsFunc(buttons, func(b announcementButton) bool { return b.ActionID == button.ActionID }):
					problem = fmt.Sprintf("Two buttons have the action_id %q, so their clicks couldn't be told apart.", button.ActionID)
				case strings.TrimSpace(button.Text) == "":
					problem = "Every button needs a label."
				case button.URL != "" && !isWebURL(button.URL):
					problem = fmt.Sprintf("The button %q has to link to an http or https address.", button.Text)
				}
				buttons = append(buttons, button)
			}
			for _, child := range v {
				walk(child)
			}
		}
	}
	walk(blocks)
	if problem != "" {
		return checked{}, problem, nil
	}

	// A claim link has to be a stamp type's, or the button gives nothing.
	claimLinks := strings.TrimSuffix(a.PublicURL, "/") + "/claim/"
	for i, button := range buttons {
		code, ok := strings.CutPrefix(button.URL, claimLinks)
		if !ok {
			continue
		}
		code = strings.ToLower(strings.Trim(code, "/"))
		t, err := a.Queries.GetSpecialStampTypeByClaimCode(ctx, &code)
		if errors.Is(err, pgx.ErrNoRows) {
			return checked{}, fmt.Sprintf("The button %q links to /claim/%s, but no special stamp type has that claim code.", button.Text, code), nil
		}
		if err != nil {
			return checked{}, "", err
		}
		buttons[i].SpecialStampTypeID = &t.ID
	}
	if buttons == nil {
		buttons = []announcementButton{}
	}
	b, err := json.Marshal(buttons)
	if err != nil {
		return checked{}, "", err
	}
	return checked{fields: f, buttons: b, pings: pings}, "", nil
}

// readAnnouncement decodes and checks a POST or PATCH body. If it returns
// false, it has already responded.
func (a *API) readAnnouncement(w http.ResponseWriter, r *http.Request) (checked, bool) {
	var f announcementFields
	if !decode(w, r, &f) {
		return checked{}, false
	}
	c, problem, err := a.checkAnnouncement(r.Context(), f)
	if err != nil {
		respond.ServerError(w, r, err)
		return checked{}, false
	}
	if problem != "" {
		respond.Error(w, http.StatusUnprocessableEntity, problem)
		return checked{}, false
	}
	return c, true
}

// listAnnouncements is GET /superadmin/announcements: every announcement,
// drafts first, with its clicks, and where they post to.
func (a *API) listAnnouncements(w http.ResponseWriter, r *http.Request) {
	rows, err := a.Queries.ListAnnouncements(r.Context())
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	type summaryJSON struct {
		ID            int64      `json:"id"`
		Text          string     `json:"text"`
		Pings         bool       `json:"pings"`
		CreatedByName string     `json:"created_by_name"`
		CreatedAt     time.Time  `json:"created_at"`
		UpdatedAt     time.Time  `json:"updated_at"`
		PostedByName  *string    `json:"posted_by_name"`
		PostedAt      *time.Time `json:"posted_at"`
		ChannelID     *string    `json:"channel_id"`
		Permalink     *string    `json:"permalink"`
		DeletedAt     *time.Time `json:"deleted_at"`
		Clicks        int32      `json:"clicks"`
		People        int32      `json:"people"` // different people who clicked
	}
	out := struct {
		// Where posts go, with its name and size if Slack says.
		Channel slack.Channel `json:"channel"`
		// Why posting won't work right now, for people; empty if it should.
		SlackProblem  string        `json:"slack_problem"`
		PublicURL     string        `json:"public_url"` // where buttons to the platform link
		Announcements []summaryJSON `json:"announcements"`
	}{Channel: slack.Channel{ID: a.AnnounceChannel}, PublicURL: strings.TrimSuffix(a.PublicURL, "/"), Announcements: []summaryJSON{}}
	for _, row := range rows {
		out.Announcements = append(out.Announcements, summaryJSON{row.ID, row.Text, row.Pings, row.CreatedByName,
			row.CreatedAt, row.UpdatedAt, row.PostedByName, row.PostedAt, row.ChannelID, row.Permalink, row.DeletedAt,
			row.Clicks, row.People})
	}
	if a.Slack == nil {
		out.SlackProblem = "SLACK_BOT_TOKEN isn't set on the backend, so announcements can be drafted but not sent."
	} else {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		if channel, err := a.Slack.Channel(ctx, a.AnnounceChannel); err != nil {
			out.SlackProblem = slackProblem(err)
		} else {
			out.Channel = channel
		}
	}
	respond.JSON(w, http.StatusOK, out)
}

// createAnnouncement is POST /superadmin/announcements: a new draft.
func (a *API) createAnnouncement(w http.ResponseWriter, r *http.Request) {
	c, ok := a.readAnnouncement(w, r)
	if !ok {
		return
	}
	created, err := a.Queries.CreateAnnouncement(r.Context(), db.CreateAnnouncementParams{
		Doc: c.fields.Doc, Blocks: c.fields.Blocks, Text: c.fields.Text, Buttons: c.buttons, Pings: c.pings,
		CreatedBy: auth.User(r.Context()).ID,
	})
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toAnnouncementJSON(created))
}

// getAnnouncement is GET /superadmin/announcements/{id}: the announcement,
// and once it's posted, how it's doing.
func (a *API) getAnnouncement(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "announcementID")
	if !ok {
		return
	}
	ann, err := a.Queries.GetAnnouncement(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		respond.Error(w, http.StatusNotFound, "That announcement doesn't exist.")
		return
	}
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	out := struct {
		announcementJSON
		Engagement *engagementJSON `json:"engagement"` // null for a draft
	}{announcementJSON: toAnnouncementJSON(ann)}
	if ann.PostedAt != nil {
		out.Engagement, err = a.engagement(r.Context(), ann)
		if err != nil {
			respond.ServerError(w, r, err)
			return
		}
	}
	respond.JSON(w, http.StatusOK, out)
}

type engagementJSON struct {
	Clicks int `json:"clicks"`
	People int `json:"people"` // different people who clicked anything
	// Of those people, how many have a Crossroads account, and how many made
	// it after their first click.
	WithAccounts  int               `json:"with_accounts"`
	SignedUpAfter int               `json:"signed_up_after"`
	Buttons       []buttonStatsJSON `json:"buttons"`
	Clickers      []clickerJSON     `json:"clickers"` // first click first
	// From Slack, as of now. Null if Slack couldn't be asked (or the message
	// was deleted), and SlackProblem says why.
	Reactions    []slack.Reaction `json:"reactions"`
	Reactors     *int             `json:"reactors"` // different people who reacted
	Replies      *int             `json:"replies"`
	Members      *int             `json:"members"` // in the channel, to compare with people
	SlackProblem string           `json:"slack_problem,omitempty"`
}

type buttonStatsJSON struct {
	announcementButton
	Removed bool `json:"removed"` // clicked, then edited out of the message
	Clicks  int  `json:"clicks"`
	People  int  `json:"people"`
	// For a claim link: the stamp, and how many have been claimed since the
	// announcement went out, through this link or any other.
	StampName *string `json:"stamp_name,omitempty"`
	Claims    *int    `json:"claims,omitempty"`
}

type clickerJSON struct {
	SlackID    string     `json:"slack_id"`
	UserID     *int64     `json:"user_id"` // null if they have no account
	Name       *string    `json:"name"`
	SignedUpAt *time.Time `json:"signed_up_at"`
	FirstClick time.Time  `json:"first_click"`
	Clicks     int        `json:"clicks"`
	Buttons    []string   `json:"buttons"` // action_ids
}

func (a *API) engagement(ctx context.Context, ann db.Announcement) (*engagementJSON, error) {
	var buttons []announcementButton
	if err := json.Unmarshal(ann.Buttons, &buttons); err != nil {
		return nil, err
	}
	clicks, err := a.Queries.ListAnnouncementClicks(ctx, db.ListAnnouncementClicksParams{ChannelID: ann.ChannelID, MessageTs: ann.MessageTs})
	if err != nil {
		return nil, err
	}
	e := &engagementJSON{Buttons: []buttonStatsJSON{}, Clickers: []clickerJSON{}}
	buttonIndex, clickerIndex := map[string]int{}, map[string]int{}
	for _, b := range buttons {
		buttonIndex[b.ActionID] = len(e.Buttons)
		e.Buttons = append(e.Buttons, buttonStatsJSON{announcementButton: b})
	}
	for _, c := range clicks {
		i, ok := buttonIndex[c.Button]
		if !ok {
			i = len(e.Buttons)
			buttonIndex[c.Button] = i
			e.Buttons = append(e.Buttons, buttonStatsJSON{announcementButton: announcementButton{ActionID: c.Button}, Removed: true})
		}
		e.Buttons[i].Clicks += int(c.Clicks)
		e.Buttons[i].People++
		e.Clicks += int(c.Clicks)
		j, ok := clickerIndex[c.SlackID]
		if !ok {
			// Rows come first click first, so this is theirs.
			j = len(e.Clickers)
			clickerIndex[c.SlackID] = j
			e.Clickers = append(e.Clickers, clickerJSON{SlackID: c.SlackID, UserID: c.UserID, Name: c.UserName,
				SignedUpAt: c.SignedUpAt, FirstClick: c.FirstClick, Buttons: []string{}})
		}
		e.Clickers[j].Clicks += int(c.Clicks)
		e.Clickers[j].Buttons = append(e.Clickers[j].Buttons, c.Button)
	}
	e.People = len(e.Clickers)
	for _, c := range e.Clickers {
		if c.UserID != nil {
			e.WithAccounts++
			if c.SignedUpAt.After(c.FirstClick) {
				e.SignedUpAfter++
			}
		}
	}
	for i, b := range e.Buttons {
		if b.SpecialStampTypeID == nil {
			continue
		}
		t, err := a.Queries.GetSpecialStampType(ctx, *b.SpecialStampTypeID)
		if err != nil {
			return nil, err
		}
		claims, err := a.Queries.CountStampClaimsSince(ctx, db.CountStampClaimsSinceParams{SpecialStampTypeID: t.ID, Since: *ann.PostedAt})
		if err != nil {
			return nil, err
		}
		e.Buttons[i].StampName, e.Buttons[i].Claims = &t.Name, new(int(claims))
	}

	switch {
	case ann.DeletedAt != nil:
		e.SlackProblem = "It's been deleted from Slack, so Slack has nothing more to say about it."
	case a.Slack == nil:
		e.SlackProblem = "SLACK_BOT_TOKEN isn't set on the backend, so reactions and replies can't be read."
	default:
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		stats, err := a.Slack.Stats(ctx, *ann.ChannelID, *ann.MessageTs)
		if err != nil {
			e.SlackProblem = slackProblem(err)
			break
		}
		e.Reactions, e.Reactors, e.Replies = stats.Reactions, &stats.Reactors, &stats.Replies
		if channel, err := a.Slack.Channel(ctx, *ann.ChannelID); err != nil {
			e.SlackProblem = slackProblem(err)
		} else {
			e.Members = &channel.Members
		}
	}
	return e, nil
}

// updateAnnouncement is PATCH /superadmin/announcements/{id}. Once it's
// posted, the message in Slack is edited too, first.
func (a *API) updateAnnouncement(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "announcementID")
	if !ok {
		return
	}
	c, ok := a.readAnnouncement(w, r)
	if !ok {
		return
	}
	a.withAnnouncement(w, r, id, func(q *db.Queries, ann db.Announcement) (int, any, error) {
		if ann.DeletedAt != nil {
			return http.StatusConflict, "It's been deleted from Slack, so it can't be edited.", nil
		}
		if ann.PostedAt != nil {
			if a.Slack == nil {
				return http.StatusServiceUnavailable, "SLACK_BOT_TOKEN isn't set on the backend, so the message in Slack can't be edited.", nil
			}
			if err := a.Slack.Update(r.Context(), *ann.ChannelID, *ann.MessageTs, c.fields.Text, c.fields.Blocks); err != nil {
				return slackFailure(r, err)
			}
		}
		updated, err := q.UpdateAnnouncement(r.Context(), db.UpdateAnnouncementParams{
			ID: id, Doc: c.fields.Doc, Blocks: c.fields.Blocks, Text: c.fields.Text, Buttons: c.buttons, Pings: c.pings,
		})
		return http.StatusOK, toAnnouncementJSON(updated), err
	})
}

// deleteAnnouncement is DELETE /superadmin/announcements/{id}. A draft is
// gone for good; a posted one is deleted from Slack and kept here, with its
// numbers.
func (a *API) deleteAnnouncement(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "announcementID")
	if !ok {
		return
	}
	a.withAnnouncement(w, r, id, func(q *db.Queries, ann db.Announcement) (int, any, error) {
		if ann.PostedAt == nil {
			_, err := q.DeleteAnnouncementDraft(r.Context(), id)
			return http.StatusNoContent, nil, err
		}
		if ann.DeletedAt != nil {
			return http.StatusConflict, "It's already been deleted from Slack.", nil
		}
		if a.Slack == nil {
			return http.StatusServiceUnavailable, "SLACK_BOT_TOKEN isn't set on the backend, so the message in Slack can't be deleted.", nil
		}
		// Someone deleted it in Slack already: it's gone either way.
		var slackErr *slack.Error
		if err := a.Slack.Delete(r.Context(), *ann.ChannelID, *ann.MessageTs); err != nil &&
			!(errors.As(err, &slackErr) && slackErr.Code == "message_not_found") {
			return slackFailure(r, err)
		}
		deleted, err := q.MarkAnnouncementDeleted(r.Context(), id)
		return http.StatusOK, toAnnouncementJSON(deleted), err
	})
}

// postAnnouncement is POST /superadmin/announcements/{id}/post: post a draft
// to the station channel, under the Slack name and picture of whoever
// posts it.
func (a *API) postAnnouncement(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "announcementID")
	if !ok {
		return
	}
	if a.Slack == nil {
		respond.Error(w, http.StatusServiceUnavailable, "SLACK_BOT_TOKEN isn't set on the backend, so announcements can't be posted.")
		return
	}
	user := auth.User(r.Context())
	a.withAnnouncement(w, r, id, func(q *db.Queries, ann db.Announcement) (int, any, error) {
		if ann.PostedAt != nil {
			return http.StatusConflict, "It's already been posted.", nil
		}
		// Check it again: a claim code could have changed since it was saved.
		c, problem, err := a.checkAnnouncement(r.Context(), announcementFields{Doc: ann.Doc, Blocks: ann.Blocks, Text: ann.Text})
		if err != nil || problem != "" {
			return http.StatusUnprocessableEntity, problem, err
		}
		if _, err := q.UpdateAnnouncement(r.Context(), db.UpdateAnnouncementParams{
			ID: id, Doc: ann.Doc, Blocks: ann.Blocks, Text: ann.Text, Buttons: c.buttons, Pings: c.pings,
		}); err != nil {
			return 0, nil, err
		}
		name, icon := a.poster(r.Context(), user)
		channel, ts, err := a.Slack.Post(r.Context(), slack.Message{
			Channel: a.AnnounceChannel, Text: ann.Text, Blocks: ann.Blocks, Username: name, IconURL: icon,
		})
		if err != nil {
			return slackFailure(r, err)
		}
		var permalink *string
		if link, err := a.Slack.Permalink(r.Context(), channel, ts); err != nil {
			log.Printf("permalink for announcement %d (%s %s): %v", id, channel, ts, err)
		} else {
			permalink = &link
		}
		posted, err := q.MarkAnnouncementPosted(r.Context(), db.MarkAnnouncementPostedParams{
			ID: id, PostedBy: &user.ID, ChannelID: &channel, MessageTs: &ts, Permalink: permalink,
		})
		if err != nil {
			// It's in Slack, but nothing here knows: say where, to fix by hand.
			log.Printf("announcement %d was posted as %s %s, but recording that failed: %v", id, channel, ts, err)
		}
		return http.StatusOK, toAnnouncementJSON(posted), err
	})
}

// testAnnouncement is POST /superadmin/announcements/{id}/test: send it to
// the superadmin's own DMs with the app, exactly as it would be posted.
// Clicks on a test's buttons aren't any announcement's.
func (a *API) testAnnouncement(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "announcementID")
	if !ok {
		return
	}
	if a.Slack == nil {
		respond.Error(w, http.StatusServiceUnavailable, "SLACK_BOT_TOKEN isn't set on the backend, so tests can't be sent.")
		return
	}
	user := auth.User(r.Context())
	if user.SlackID == nil {
		respond.Error(w, http.StatusUnprocessableEntity, "Your account has no Slack ID, so there's nowhere to send a test.")
		return
	}
	ann, err := a.Queries.GetAnnouncement(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		respond.Error(w, http.StatusNotFound, "That announcement doesn't exist.")
		return
	}
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	name, icon := a.poster(r.Context(), user)
	if _, _, err := a.Slack.Post(r.Context(), slack.Message{
		Channel: *user.SlackID, Text: ann.Text, Blocks: ann.Blocks, Username: name, IconURL: icon,
	}); err != nil {
		status, message, _ := slackFailure(r, err)
		respond.Error(w, status, message.(string))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// withAnnouncement runs change on the announcement, locked, in a transaction
// that's committed if change returns a status under 400 and no error. A
// string out is an error message; anything else is the JSON response.
func (a *API) withAnnouncement(w http.ResponseWriter, r *http.Request, id int64,
	change func(q *db.Queries, ann db.Announcement) (status int, out any, err error)) {
	tx, err := a.Pool.Begin(r.Context())
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())
	q := a.Queries.WithTx(tx)
	ann, err := q.LockAnnouncement(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		respond.Error(w, http.StatusNotFound, "That announcement doesn't exist.")
		return
	}
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	status, out, err := change(q, ann)
	if err == nil && status < 400 {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	if message, ok := out.(string); ok {
		respond.Error(w, status, message)
		return
	}
	if out == nil {
		w.WriteHeader(status)
		return
	}
	respond.JSON(w, status, out)
}

// poster is who an announcement shows as in Slack: the superadmin's Slack
// name and picture, or their Crossroads name and the app's picture if Slack
// can't say.
func (a *API) poster(ctx context.Context, user *db.User) (name, icon string) {
	if user.SlackID != nil {
		profile, err := a.Slack.Profile(ctx, *user.SlackID)
		if err == nil && profile.Name != "" {
			return profile.Name, profile.ImageURL
		}
		if err != nil {
			log.Printf("slack profile of user %d: %v", user.ID, err)
		}
	}
	return user.Name, ""
}

// slackFailure is withAnnouncement's answer for a Slack call that failed.
func slackFailure(r *http.Request, err error) (int, any, error) {
	log.Printf("%s %s: %v", r.Method, r.URL.Path, err)
	return http.StatusBadGateway, slackProblem(err), nil
}

// slackProblem says what went wrong talking to Slack, for people.
func slackProblem(err error) string {
	var e *slack.Error
	if !errors.As(err, &e) {
		return "Couldn't reach Slack. Try again in a bit."
	}
	switch e.Code {
	case "not_in_channel", "channel_not_found":
		return "The Crossroads Announcements app isn't in the channel. In Slack, open the channel and run /invite @Crossroads Announcements."
	case "restricted_action", "restricted_action_read_only_channel", "restricted_action_thread_only_channel":
		return "Slack won't let the app post in the channel. In the channel's settings, add Crossroads Announcements to who can post."
	case "is_archived":
		return "The channel is archived."
	case "missing_scope":
		return "The Slack app needs the " + e.Needed + " permission. Add it under OAuth & Permissions (slackbot/manifest.json has them all) and reinstall the app."
	case "not_authed", "invalid_auth", "account_inactive", "token_revoked", "token_expired":
		return "SLACK_BOT_TOKEN doesn't work (Slack says " + e.Code + ")."
	case "invalid_blocks", "invalid_blocks_format":
		return "Slack didn't take the message: " + strings.Join(e.Messages, " ")
	case "msg_too_long", "msg_blocks_too_long":
		return "That's too long for one Slack message."
	case "message_not_found":
		return "That message isn't in Slack anymore."
	case "cant_update_message", "edit_window_closed", "cant_delete_message":
		return "Slack won't let the app change that message (" + e.Code + ")."
	case "ratelimited":
		return "Slack wants us to slow down. Try again in a minute."
	}
	return "Slack said no: " + e.Code + "."
}
