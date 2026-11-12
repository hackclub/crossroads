// Package github reads public facts about a project's repo from GitHub's API:
// whether it exists and is public, its commits during a ship, the lines they
// changed, and its README. Reviewers see these next to Hackatime's numbers,
// and the review checks use them (see api/stages.go).
//
// With a gh-proxy key, requests go through gh-proxy.hackclub.com, Hack Club's
// pool of donated GitHub tokens. Without one it asks GitHub directly, with or
// without a token, but with no token GitHub allows 60 requests an hour for the
// whole server, so line counts (one request per commit) are only fetched with
// a token or a proxy key. Results are cached for a few minutes, and commit
// line counts for good, since a commit never changes.
package github

import (
	"cmp"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	// ErrNotFound means the repo doesn't exist, or is private: GitHub answers
	// the same for both.
	ErrNotFound    = errors.New("GitHub says the repo doesn't exist or is private")
	ErrRateLimited = errors.New("GitHub's rate limit ran out (set GH_PROXY_API_KEY or GITHUB_TOKEN for a higher one)")
)

// ProxyURL is gh-proxy's mirror of https://api.github.com.
const ProxyURL = "https://gh-proxy.hackclub.com/gh"

type Config struct {
	BaseURL string // defaults to https://api.github.com, or ProxyURL with a ProxyKey
	Token   string // optional; any token works, it needs no scopes for public repos
	// Optional. A gh-proxy API key. With one, Token isn't used: the proxy
	// brings its own tokens, and ours shouldn't leave the server.
	ProxyKey string
}

type Client struct {
	baseURL  string
	token    string
	proxyKey string
	http     *http.Client

	mu      sync.Mutex
	reports map[string]cachedReport
	lines   map[string][2]int // "owner/repo@sha" → additions, deletions
}

type cachedReport struct {
	report  *Report
	err     error
	expires time.Time
}

func New(cfg Config) *Client {
	baseURL := "https://api.github.com"
	if cfg.ProxyKey != "" {
		baseURL, cfg.Token = ProxyURL, ""
	}
	return &Client{
		baseURL:  strings.TrimSuffix(cmp.Or(cfg.BaseURL, baseURL), "/"),
		token:    cfg.Token,
		proxyKey: cfg.ProxyKey,
		http:     &http.Client{Timeout: 10 * time.Second},
		reports:  map[string]cachedReport{},
		lines:    map[string][2]int{},
	}
}

// ParseRepo finds the owner and repo in a github.com link, including links to
// a branch or file in it. ok is false for anything else.
func ParseRepo(raw string) (owner, repo string, ok bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.") != "github.com" {
		return "", "", false
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], strings.TrimSuffix(parts[1], ".git"), true
}

type Commit struct {
	SHA     string    `json:"sha"`
	Message string    `json:"message"` // the first line
	Author  string    `json:"author"`  // their GitHub login, or the name in the commit
	Date    time.Time `json:"date"`
	// Lines changed, when they were fetched (HasLines).
	Additions int  `json:"additions"`
	Deletions int  `json:"deletions"`
	HasLines  bool `json:"has_lines"`
}

type AuthorCount struct {
	Author  string `json:"author"`
	Commits int    `json:"commits"`
}

// Report is what GitHub says about a repo, and its commits between Since and
// Until.
type Report struct {
	Owner         string    `json:"owner"`
	Repo          string    `json:"repo"`
	URL           string    `json:"url"`
	Description   string    `json:"description"`
	Language      string    `json:"language"`
	Stars         int       `json:"stars"`
	Fork          bool      `json:"fork"`
	Archived      bool      `json:"archived"`
	CreatedAt     time.Time `json:"created_at"`
	PushedAt      time.Time `json:"pushed_at"`
	DefaultBranch string    `json:"default_branch"`
	TotalCommits  int       `json:"total_commits"` // on the default branch, all time

	Since time.Time `json:"since"`
	Until time.Time `json:"until"`
	// The window's commits, newest first. At most maxCommits; Truncated says
	// there were more.
	Commits   []Commit      `json:"commits"`
	Truncated bool          `json:"truncated"`
	Authors   []AuthorCount `json:"authors"`
	// Lines added and removed by the window's commits. LinesCounted is how
	// many commits that covers: line counts take a request per commit, so
	// only the newest maxLineCommits are counted, and none without a token.
	Additions    int `json:"additions"`
	Deletions    int `json:"deletions"`
	LinesCounted int `json:"lines_counted"`
	// Why line counts are missing, when they are.
	LinesNote string `json:"lines_note,omitempty"`

	Readme string `json:"-"` // empty if there isn't one
}

const (
	maxCommits     = 300
	maxLineCommits = 60
	cacheFor       = 10 * time.Minute
)

// Activity reports on the repo at repoURL and its commits between since and
// until. It returns ErrNotFound for a repo that doesn't exist or is private.
func (c *Client) Activity(ctx context.Context, repoURL string, since, until time.Time) (*Report, error) {
	owner, repo, ok := ParseRepo(repoURL)
	if !ok {
		return nil, fmt.Errorf("%q isn't a GitHub repo link", repoURL)
	}
	key := fmt.Sprintf("%s/%s %d %d", strings.ToLower(owner), strings.ToLower(repo), since.Unix(), until.Unix())
	c.mu.Lock()
	cached, hit := c.reports[key]
	c.mu.Unlock()
	if hit && time.Now().Before(cached.expires) {
		return cached.report, cached.err
	}
	report, err := c.activity(ctx, owner, repo, since, until)
	// Don't remember a cancelled request, or a rate limit for long.
	if ctx.Err() == nil {
		expires := time.Now().Add(cacheFor)
		if errors.Is(err, ErrRateLimited) {
			expires = time.Now().Add(time.Minute)
		}
		c.mu.Lock()
		c.reports[key] = cachedReport{report, err, expires}
		c.mu.Unlock()
	}
	return report, err
}

func (c *Client) activity(ctx context.Context, owner, repo string, since, until time.Time) (*Report, error) {
	path := "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo)
	var info struct {
		FullName      string    `json:"full_name"`
		HTMLURL       string    `json:"html_url"`
		Description   *string   `json:"description"`
		Language      *string   `json:"language"`
		Stars         int       `json:"stargazers_count"`
		Fork          bool      `json:"fork"`
		Archived      bool      `json:"archived"`
		Private       bool      `json:"private"`
		CreatedAt     time.Time `json:"created_at"`
		PushedAt      time.Time `json:"pushed_at"`
		DefaultBranch string    `json:"default_branch"`
	}
	if _, err := c.get(ctx, path, nil, &info); err != nil {
		return nil, err
	}
	if info.Private {
		// Only with a token that can see it. Reviewers can't, so it counts
		// as not public.
		return nil, ErrNotFound
	}
	owner, repo, _ = strings.Cut(info.FullName, "/")
	r := &Report{
		Owner: owner, Repo: repo, URL: info.HTMLURL, Stars: info.Stars, Fork: info.Fork, Archived: info.Archived,
		CreatedAt: info.CreatedAt, PushedAt: info.PushedAt, DefaultBranch: info.DefaultBranch,
		Since: since, Until: until, Commits: []Commit{}, Authors: []AuthorCount{},
	}
	if info.Description != nil {
		r.Description = *info.Description
	}
	if info.Language != nil {
		r.Language = *info.Language
	}

	// All-time commits: ask for one per page and read how many pages there are.
	var one []json.RawMessage
	header, err := c.get(ctx, path+"/commits", url.Values{"per_page": {"1"}}, &one)
	switch {
	case err != nil && !errors.Is(err, errEmptyRepo):
		return nil, err
	case err == nil:
		r.TotalCommits = len(one)
		if last := lastPage.FindStringSubmatch(header.Get("Link")); last != nil {
			r.TotalCommits, _ = strconv.Atoi(last[1])
		}
	}

	if r.TotalCommits > 0 {
		if err := c.windowCommits(ctx, path, r); err != nil {
			return nil, err
		}
	}
	if err := c.countLines(ctx, path, r); err != nil {
		return nil, err
	}

	var readme struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	switch _, err := c.get(ctx, path+"/readme", nil, &readme); {
	case errors.Is(err, ErrNotFound):
	case err != nil:
		return nil, err
	case readme.Encoding == "base64":
		text, _ := base64.StdEncoding.DecodeString(strings.ReplaceAll(readme.Content, "\n", ""))
		r.Readme = string(text)
	default:
		r.Readme = readme.Content
	}
	return r, nil
}

var lastPage = regexp.MustCompile(`[?&]page=(\d+)[^>]*>;\s*rel="last"`)

func (c *Client) windowCommits(ctx context.Context, path string, r *Report) error {
	authors := map[string]int{}
	for page := 1; len(r.Commits) < maxCommits; page++ {
		var batch []struct {
			SHA    string `json:"sha"`
			Commit struct {
				Message string `json:"message"`
				Author  struct {
					Name string    `json:"name"`
					Date time.Time `json:"date"`
				} `json:"author"`
			} `json:"commit"`
			Author *struct {
				Login string `json:"login"`
			} `json:"author"`
		}
		query := url.Values{
			"since": {r.Since.UTC().Format(time.RFC3339)}, "until": {r.Until.UTC().Format(time.RFC3339)},
			"per_page": {"100"}, "page": {strconv.Itoa(page)},
		}
		if _, err := c.get(ctx, path+"/commits", query, &batch); err != nil {
			return err
		}
		for _, b := range batch {
			if len(r.Commits) == maxCommits {
				r.Truncated = true
				break
			}
			author := b.Commit.Author.Name
			if b.Author != nil && b.Author.Login != "" {
				author = b.Author.Login
			}
			message, _, _ := strings.Cut(b.Commit.Message, "\n")
			r.Commits = append(r.Commits, Commit{SHA: b.SHA, Message: message, Author: author, Date: b.Commit.Author.Date})
			authors[author]++
		}
		if len(batch) < 100 {
			break
		}
	}
	for author, n := range authors {
		r.Authors = append(r.Authors, AuthorCount{author, n})
	}
	slices.SortFunc(r.Authors, func(a, b AuthorCount) int {
		return cmp.Or(b.Commits-a.Commits, strings.Compare(a.Author, b.Author))
	})
	return nil
}

// countLines fills in the lines changed by the newest window commits.
func (c *Client) countLines(ctx context.Context, path string, r *Report) error {
	if len(r.Commits) == 0 {
		return nil
	}
	if c.token == "" && c.baseURL == "https://api.github.com" {
		r.LinesNote = "Line counts need GH_PROXY_API_KEY or GITHUB_TOKEN on the backend."
		return nil
	}
	n := min(len(r.Commits), maxLineCommits)
	errs := make([]error, n)
	var wg sync.WaitGroup
	limit := make(chan struct{}, 6)
	for i := range n {
		wg.Go(func() {
			limit <- struct{}{}
			defer func() { <-limit }()
			commit := &r.Commits[i]
			key := path + "@" + commit.SHA
			c.mu.Lock()
			lines, ok := c.lines[key]
			c.mu.Unlock()
			if !ok {
				var detail struct {
					Stats struct {
						Additions int `json:"additions"`
						Deletions int `json:"deletions"`
					} `json:"stats"`
				}
				if _, err := c.get(ctx, path+"/commits/"+commit.SHA, nil, &detail); err != nil {
					errs[i] = err
					return
				}
				lines = [2]int{detail.Stats.Additions, detail.Stats.Deletions}
				c.mu.Lock()
				c.lines[key] = lines
				c.mu.Unlock()
			}
			commit.Additions, commit.Deletions, commit.HasLines = lines[0], lines[1], true
		})
	}
	wg.Wait()
	for i := range n {
		if errs[i] != nil {
			if errors.Is(errs[i], ErrRateLimited) {
				r.LinesNote = "GitHub's rate limit ran out partway through counting lines."
				break
			}
			return errs[i]
		}
	}
	for _, commit := range r.Commits {
		if commit.HasLines {
			r.Additions += commit.Additions
			r.Deletions += commit.Deletions
			r.LinesCounted++
		}
	}
	if r.LinesNote == "" && r.LinesCounted < len(r.Commits) {
		r.LinesNote = fmt.Sprintf("Lines are from the newest %d of %d commits.", r.LinesCounted, len(r.Commits))
	}
	return nil
}

var errEmptyRepo = errors.New("the repo has no commits")

func (c *Client) get(ctx context.Context, path string, query url.Values, out any) (http.Header, error) {
	u := c.baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "crossroads-review")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if c.proxyKey != "" {
		req.Header.Set("X-API-Key", c.proxyKey)
	}
	var resp *http.Response
	for tries := 1; ; tries++ {
		if resp, err = c.http.Do(req); err != nil {
			return nil, err
		}
		// gh-proxy allows each key a few requests a second, and a 429 says
		// when the next second starts. Counting lines alone sends dozens, so
		// wait those out. Longer waits are GitHub's own limits, and fail.
		wait, err := strconv.Atoi(resp.Header.Get("Retry-After"))
		if resp.StatusCode != http.StatusTooManyRequests || err != nil || wait > 5 || tries == 5 {
			break
		}
		resp.Body.Close()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Duration(max(wait, 1)) * time.Second):
		}
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusOK:
	case resp.StatusCode == http.StatusNotFound:
		return nil, ErrNotFound
	case resp.StatusCode == http.StatusConflict: // "Git Repository is empty."
		return nil, errEmptyRepo
	case (resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests) &&
		(resp.Header.Get("X-RateLimit-Remaining") == "0" || resp.Header.Get("Retry-After") != ""):
		return nil, ErrRateLimited
	default:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("GitHub %s: %s: %s", path, resp.Status, strings.TrimSpace(string(body)))
	}
	return resp.Header, json.NewDecoder(resp.Body).Decode(out)
}
