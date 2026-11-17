// Package slack posts the announcements superadmins write on the admin page
// (api/announcements.go) with the Crossroads Slack app's bot token, and reads
// back what Slack knows about them: reactions, replies, and how many people
// are in the channel. Clicks on their buttons arrive another way, through the
// bot in slackbot/ and package slackclicks.
//
// The bot token needs chat:write, chat:write.customize (to post under the
// poster's name and picture), users:read, reactions:read, channels:read and
// groups:read; slackbot/manifest.json asks for them.
package slack

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Config struct {
	Token   string // the bot token, xoxb-...
	BaseURL string // defaults to https://slack.com/api
}

type Client struct {
	token   string
	baseURL string
	http    *http.Client
}

func New(cfg Config) *Client {
	return &Client{
		token:   cfg.Token,
		baseURL: strings.TrimSuffix(cmp.Or(cfg.BaseURL, "https://slack.com/api"), "/"),
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

// Error is Slack turning a call down, like {"ok": false, "error":
// "not_in_channel"}.
type Error struct {
	Method string
	Code   string // Slack's error code
	// The scope a missing_scope error wants, and what's wrong with the blocks
	// for invalid_blocks.
	Needed   string
	Messages []string
	// How long a ratelimited error says to wait.
	RetryAfter time.Duration
}

func (e *Error) Error() string {
	msg := e.Method + ": " + e.Code
	if e.Needed != "" {
		msg += " (needs " + e.Needed + ")"
	}
	if len(e.Messages) > 0 {
		msg += ": " + strings.Join(e.Messages, "; ")
	}
	return msg
}

// Message is a message to post.
type Message struct {
	Channel string // a channel ID, or a user ID for a DM from the app
	Text    string // the notification and screen reader text
	Blocks  json.RawMessage
	// Who it shows as: chat:write.customize lets the app post under someone's
	// name and picture, with an APP tag. Empty is the app's own.
	Username string
	IconURL  string
}

// Post posts m and returns the message's ts, its ID within the channel, and
// the channel it went to: a DM's ID when m.Channel was a user.
func (c *Client) Post(ctx context.Context, m Message) (channel, ts string, err error) {
	var out struct {
		Channel string `json:"channel"`
		TS      string `json:"ts"`
	}
	err = c.call(ctx, "chat.postMessage", map[string]any{
		"channel":      m.Channel,
		"text":         m.Text,
		"blocks":       m.Blocks,
		"username":     m.Username,
		"icon_url":     m.IconURL,
		"unfurl_links": false,
		"unfurl_media": false,
	}, &out)
	return out.Channel, out.TS, err
}

// Update rewrites a message the app posted. The name and picture it was
// posted under stay.
func (c *Client) Update(ctx context.Context, channel, ts, text string, blocks json.RawMessage) error {
	return c.call(ctx, "chat.update", map[string]any{"channel": channel, "ts": ts, "text": text, "blocks": blocks}, nil)
}

// Delete deletes a message the app posted.
func (c *Client) Delete(ctx context.Context, channel, ts string) error {
	return c.call(ctx, "chat.delete", map[string]any{"channel": channel, "ts": ts}, nil)
}

// Permalink is a link that opens a message in Slack.
func (c *Client) Permalink(ctx context.Context, channel, ts string) (string, error) {
	var out struct {
		Permalink string `json:"permalink"`
	}
	err := c.get(ctx, "chat.getPermalink", url.Values{"channel": {channel}, "message_ts": {ts}}, &out)
	return out.Permalink, err
}

// Profile is how someone appears in Slack.
type Profile struct {
	Name     string // their display name, or their full name if they haven't set one
	ImageURL string
}

func (c *Client) Profile(ctx context.Context, userID string) (Profile, error) {
	var out struct {
		User struct {
			RealName string `json:"real_name"`
			Profile  struct {
				DisplayName string `json:"display_name"`
				RealName    string `json:"real_name"`
				Image192    string `json:"image_192"`
			} `json:"profile"`
		} `json:"user"`
	}
	if err := c.get(ctx, "users.info", url.Values{"user": {userID}}, &out); err != nil {
		return Profile{}, err
	}
	p := out.User.Profile
	return Profile{Name: cmp.Or(p.DisplayName, p.RealName, out.User.RealName), ImageURL: p.Image192}, nil
}

type Reaction struct {
	Name  string `json:"name"` // the emoji, without colons
	Count int    `json:"count"`
}

// Stats is what Slack says about a message's reception.
type Stats struct {
	Reactions []Reaction
	Reactors  int // different people who reacted
	Replies   int // in its thread
}

func (c *Client) Stats(ctx context.Context, channel, ts string) (Stats, error) {
	var out struct {
		Message struct {
			Reactions []struct {
				Name  string   `json:"name"`
				Count int      `json:"count"`
				Users []string `json:"users"`
			} `json:"reactions"`
			ReplyCount int `json:"reply_count"`
		} `json:"message"`
	}
	err := c.get(ctx, "reactions.get", url.Values{"channel": {channel}, "timestamp": {ts}, "full": {"true"}}, &out)
	if err != nil {
		return Stats{}, err
	}
	stats := Stats{Reactions: []Reaction{}, Replies: out.Message.ReplyCount}
	people := map[string]bool{}
	for _, r := range out.Message.Reactions {
		stats.Reactions = append(stats.Reactions, Reaction{r.Name, r.Count})
		for _, u := range r.Users {
			people[u] = true
		}
	}
	stats.Reactors = len(people)
	return stats, nil
}

type Channel struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Private bool   `json:"is_private"`
	Members int    `json:"num_members"`
	Member  bool   `json:"is_member"` // whether the app is in it
}

func (c *Client) Channel(ctx context.Context, id string) (Channel, error) {
	var out struct {
		Channel Channel `json:"channel"`
	}
	err := c.get(ctx, "conversations.info", url.Values{"channel": {id}, "include_num_members": {"true"}}, &out)
	return out.Channel, err
}

// call sends a JSON body to a method that takes one (the chat.* writes) and
// decodes the response into out, if it isn't nil.
func (c *Client) call(ctx context.Context, method string, body map[string]any, out any) error {
	for k, v := range body {
		if v == "" {
			delete(body, k)
		}
	}
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/"+method, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	return c.do(req, method, out)
}

// get calls a method that only takes query parameters.
func (c *Client) get(ctx context.Context, method string, params url.Values, out any) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/"+method+"?"+params.Encode(), nil)
	if err != nil {
		return err
	}
	return c.do(req, method, out)
}

func (c *Client) do(req *http.Request, method string, out any) error {
	req.Header.Set("Authorization", "Bearer "+c.token)
	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%s: %w", method, err)
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusTooManyRequests {
		wait, _ := time.ParseDuration(res.Header.Get("Retry-After") + "s")
		return &Error{Method: method, Code: "ratelimited", RetryAfter: wait}
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return fmt.Errorf("%s: %w", method, err)
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: Slack answered %s", method, res.Status)
	}
	var status struct {
		OK               bool   `json:"ok"`
		Error            string `json:"error"`
		Needed           string `json:"needed"`
		ResponseMetadata struct {
			Messages []string `json:"messages"`
		} `json:"response_metadata"`
	}
	if err := json.Unmarshal(b, &status); err != nil {
		return fmt.Errorf("%s: reading Slack's answer: %w", method, err)
	}
	if !status.OK {
		return &Error{Method: method, Code: status.Error, Needed: status.Needed, Messages: status.ResponseMetadata.Messages}
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(b, out); err != nil {
		return fmt.Errorf("%s: reading Slack's answer: %w", method, err)
	}
	return nil
}
