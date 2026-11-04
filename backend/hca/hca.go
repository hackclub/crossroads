// Package hca talks to Hack Club Auth (auth.hackclub.com), which everyone
// signs in with. See https://auth.hackclub.com/docs/oauth-guide.
package hca

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/oauth2"
)

// basic_info (birthday and phone) and address need the app to be hq_official.
var scopes = []string{"email", "name", "slack_id", "verification_status", "basic_info", "address"}

type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string // has to match one registered on the app
	BaseURL      string // defaults to https://auth.hackclub.com
}

type Client struct {
	oauth   oauth2.Config
	baseURL string
	http    *http.Client
}

func New(cfg Config) *Client {
	baseURL := cmp.Or(cfg.BaseURL, "https://auth.hackclub.com")
	return &Client{
		oauth: oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Scopes:       scopes,
			Endpoint: oauth2.Endpoint{
				AuthURL:   baseURL + "/oauth/authorize",
				TokenURL:  baseURL + "/oauth/token",
				AuthStyle: oauth2.AuthStyleInParams,
			},
		},
		baseURL: baseURL,
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

// AuthorizeURL is where to send someone to sign in. Hack Club Auth sends them
// back to the redirect URL with a code and the same state.
func (c *Client) AuthorizeURL(state string) string {
	return c.oauth.AuthCodeURL(state)
}

// Identity is what Hack Club Auth knows about someone, limited to the scopes
// we asked for.
type Identity struct {
	ID                 string `json:"id"` // like ident!9zfYqvr
	FirstName          string `json:"first_name"`
	LastName           string `json:"last_name"`
	PrimaryEmail       string `json:"primary_email"`
	SlackID            string `json:"slack_id"`
	VerificationStatus string `json:"verification_status"` // needs_submission, pending, verified or ineligible
	YSWSEligible       bool   `json:"ysws_eligible"`
	Birthday           string `json:"birthday"` // like 2008-06-15
	PhoneNumber        string `json:"phone_number"`
	Addresses          []struct {
		FirstName  string `json:"first_name"`
		LastName   string `json:"last_name"`
		Line1      string `json:"line_1"`
		Line2      string `json:"line_2"`
		City       string `json:"city"`
		State      string `json:"state"`
		PostalCode string `json:"postal_code"`
		Country    string `json:"country"`
		Primary    bool   `json:"primary"`
	} `json:"addresses"`
}

// IdentityFromCode trades the code from a sign-in redirect for an access token
// and uses it to look up who signed in. The token isn't kept: everything we
// need comes back on the next sign-in.
func (c *Client) IdentityFromCode(ctx context.Context, code string) (Identity, error) {
	ctx = context.WithValue(ctx, oauth2.HTTPClient, c.http)
	token, err := c.oauth.Exchange(ctx, code)
	if err != nil {
		return Identity{}, fmt.Errorf("exchanging code: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v1/me", nil)
	if err != nil {
		return Identity{}, err
	}
	token.SetAuthHeader(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return Identity{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return Identity{}, fmt.Errorf("GET /api/v1/me: %s: %s", resp.Status, body)
	}

	var body struct {
		Identity Identity `json:"identity"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return Identity{}, fmt.Errorf("GET /api/v1/me: %w", err)
	}
	if body.Identity.ID == "" {
		return Identity{}, errors.New("GET /api/v1/me: identity has no id")
	}
	return body.Identity, nil
}

// Verification results from CheckVerification.
const (
	VerifiedEligible    = "verified_eligible"
	VerifiedButOver18   = "verified_but_over_18"
	VerificationPending = "pending"
	NeedsSubmission     = "needs_submission"
	Rejected            = "rejected"
	NotFound            = "not_found"
)

// CheckVerification asks Hack Club Auth where someone's identity
// verification stands right now, by their identity ID (like ident!9zfYqvr).
// It needs no token, so it works between sign-ins.
func (c *Client) CheckVerification(ctx context.Context, identityID string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.baseURL+"/api/external/check?"+url.Values{"idv_id": {identityID}}.Encode(), nil)
	if err != nil {
		return "", err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("GET /api/external/check: %s: %s", resp.Status, body)
	}
	var body struct {
		Result string `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", fmt.Errorf("GET /api/external/check: %w", err)
	}
	return body.Result, nil
}
