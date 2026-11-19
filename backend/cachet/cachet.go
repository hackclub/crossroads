// Package cachet asks Cachet (cachet.dunkirk.sh), Hack Club's cache of Slack
// profiles, about people's Slack pictures. The landing page shows them from
// there too.
package cachet

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type Client struct {
	baseURL string
	http    *http.Client
}

// New talks to Cachet at baseURL, or at https://cachet.dunkirk.sh if it's
// empty.
func New(baseURL string) *Client {
	return &Client{baseURL: cmp.Or(baseURL, "https://cachet.dunkirk.sh"), http: &http.Client{Timeout: 10 * time.Second}}
}

// OwnPicture says whether someone put up a Slack picture of their own. For
// Slack's default pictures, and people it can't find, Cachet has a
// placeholder of its own instead of a picture from Slack.
func (c *Client) OwnPicture(ctx context.Context, slackID string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/users/"+url.PathEscape(slackID), nil)
	if err != nil {
		return false, err
	}
	res, err := c.http.Do(req)
	if err != nil {
		return false, err
	}
	defer res.Body.Close()
	// 202 is someone it hasn't looked up yet, with the placeholder for now.
	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusAccepted {
		return false, fmt.Errorf("cachet: %s", res.Status)
	}
	var user struct {
		ImageURL string `json:"imageUrl"`
	}
	if err := json.NewDecoder(res.Body).Decode(&user); err != nil {
		return false, fmt.Errorf("cachet: %w", err)
	}
	picture, err := url.Parse(user.ImageURL)
	return err == nil && picture.Host == "avatars.slack-edge.com", nil
}
