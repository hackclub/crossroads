// Package airtable writes and reads records in one Airtable base. See
// https://airtable.com/developers/web/api/introduction.
package airtable

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

type Config struct {
	Token   string // a personal access token with data.records:read and :write on the base
	BaseID  string
	BaseURL string // defaults to https://api.airtable.com
}

type Client struct {
	token   string
	baseURL string
	http    *http.Client

	mu   sync.Mutex
	last time.Time // when the last request went out
}

func New(cfg Config) *Client {
	return &Client{
		token:   cfg.Token,
		baseURL: cmp.Or(cfg.BaseURL, "https://api.airtable.com") + "/v0/" + cfg.BaseID,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

// Fields are a record's fields by name.
type Fields map[string]any

type Record struct {
	ID     string `json:"id,omitempty"`
	Fields Fields `json:"fields"`
}

// BatchSize is the most records Create and Update take at once.
const BatchSize = 10

// Create adds records to table and returns them with their new IDs, in the
// same order.
func (c *Client) Create(ctx context.Context, table string, records []Record) ([]Record, error) {
	var resp struct {
		Records []Record `json:"records"`
	}
	err := c.do(ctx, http.MethodPost, "/"+url.PathEscape(table), map[string]any{"records": records, "typecast": true}, &resp)
	return resp.Records, err
}

// Update changes the given fields on records, which need IDs, and leaves
// their other fields alone.
func (c *Client) Update(ctx context.Context, table string, records []Record) error {
	return c.do(ctx, http.MethodPatch, "/"+url.PathEscape(table), map[string]any{"records": records, "typecast": true}, nil)
}

// List returns every record in table matching an Airtable formula, with only
// the named fields.
func (c *Client) List(ctx context.Context, table, formula string, fields ...string) ([]Record, error) {
	var all []Record
	offset := ""
	for {
		query := url.Values{"filterByFormula": {formula}}
		for _, f := range fields {
			query.Add("fields[]", f)
		}
		if offset != "" {
			query.Set("offset", offset)
		}
		var resp struct {
			Records []Record `json:"records"`
			Offset  string   `json:"offset"`
		}
		if err := c.do(ctx, http.MethodGet, "/"+url.PathEscape(table)+"?"+query.Encode(), nil, &resp); err != nil {
			return nil, err
		}
		all = append(all, resp.Records...)
		if resp.Offset == "" {
			return all, nil
		}
		offset = resp.Offset
	}
}

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	// Airtable allows 5 requests a second per base.
	c.mu.Lock()
	if wait := time.Until(c.last.Add(250 * time.Millisecond)); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
	c.mu.Unlock()

	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("airtable %s %s: %s: %s", method, path, resp.Status, b)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
