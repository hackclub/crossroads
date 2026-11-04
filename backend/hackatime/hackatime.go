// Package hackatime reads people's coding time from Hackatime
// (hackatime.hackclub.com). See https://hackatime.hackclub.com/api-docs.
package hackatime

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var (
	ErrNoAccount = errors.New("no Hackatime account for that Slack ID")
	ErrPrivate   = errors.New("Hackatime stats are private")
)

type Config struct {
	BaseURL string    // defaults to https://hackatime.hackclub.com
	Start   time.Time // only time after this counts
}

type Client struct {
	baseURL string
	start   time.Time
	http    *http.Client
}

func New(cfg Config) *Client {
	return &Client{
		baseURL: cmp.Or(cfg.BaseURL, "https://hackatime.hackclub.com"),
		start:   cfg.Start,
		http:    &http.Client{Timeout: 15 * time.Second},
	}
}

type Project struct {
	Name    string
	Seconds int
}

type Stats struct {
	// Seconds is the time across Projects. Time that counted toward more than
	// one of them is only counted once, so it can be less than their sum.
	Seconds  int
	Projects []Project
	// Convicted means Hackatime caught them cheating (trust level red).
	Convicted bool
}

// Stats returns someone's time since Start, looking them up by Slack ID. If
// projects isn't empty, only those Hackatime projects count. Project names
// can't contain commas.
func (c *Client) Stats(ctx context.Context, slackID string, projects []string) (Stats, error) {
	query := url.Values{
		"features":   {"projects"},
		"start_date": {c.start.UTC().Format(time.RFC3339)},
	}
	if len(projects) > 0 {
		query.Set("filter_by_project", strings.Join(projects, ","))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.baseURL+"/api/v1/users/"+url.PathEscape(slackID)+"/stats?"+query.Encode(), nil)
	if err != nil {
		return Stats{}, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return Stats{}, err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return Stats{}, ErrNoAccount
	case http.StatusForbidden:
		return Stats{}, ErrPrivate
	default:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return Stats{}, fmt.Errorf("hackatime stats for %s: %s: %s", slackID, resp.Status, body)
	}

	var body struct {
		Data struct {
			TotalSeconds float64 `json:"total_seconds"`
			Projects     []struct {
				Name         string  `json:"name"`
				TotalSeconds float64 `json:"total_seconds"`
			} `json:"projects"`
		} `json:"data"`
		TrustFactor struct {
			TrustLevel string `json:"trust_level"`
		} `json:"trust_factor"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return Stats{}, fmt.Errorf("hackatime stats for %s: %w", slackID, err)
	}
	stats := Stats{
		Seconds:   int(body.Data.TotalSeconds),
		Convicted: body.TrustFactor.TrustLevel == "red",
	}
	for _, p := range body.Data.Projects {
		stats.Projects = append(stats.Projects, Project{Name: p.Name, Seconds: int(p.TotalSeconds)})
	}
	return stats, nil
}
