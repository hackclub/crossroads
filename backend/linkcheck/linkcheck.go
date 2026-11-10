// Package linkcheck checks that a project's demo or repo link opens, the way
// Sidekick's D3 and D4 checks do: a HEAD request, then a GET if the site
// doesn't answer HEAD, following redirects.
package linkcheck

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// What a check found.
const (
	OK      = "ok"
	Broken  = "broken"  // the link is wrong, or the site is down
	Unknown = "unknown" // couldn't tell: a timeout, or a site that blocks bots
)

type Result struct {
	State  string
	Status int    // the HTTP status, when there was one
	Detail string // for people: "returns 404 Not Found"
}

// Checker checks links. Results are cached for a few minutes, so a review
// page and the decision after it don't wait twice.
type Checker interface {
	Check(ctx context.Context, link string) Result
}

type HTTP struct {
	client *http.Client
	mu     sync.Mutex
	cache  map[string]cached
}

type cached struct {
	result  Result
	expires time.Time
}

func New() *HTTP {
	return &HTTP{client: &http.Client{Timeout: 10 * time.Second}, cache: map[string]cached{}}
}

// Browsers get pages some sites refuse to scripts.
const userAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/137.0.0.0 Safari/537.36"

func (c *HTTP) Check(ctx context.Context, link string) Result {
	c.mu.Lock()
	hit, ok := c.cache[link]
	c.mu.Unlock()
	if ok && time.Now().Before(hit.expires) {
		return hit.result
	}
	result := c.check(ctx, link)
	if ctx.Err() == nil {
		c.mu.Lock()
		c.cache[link] = cached{result, time.Now().Add(5 * time.Minute)}
		c.mu.Unlock()
	}
	return result
}

func (c *HTTP) check(ctx context.Context, link string) Result {
	if problem := Invalid(link); problem != "" {
		return Result{State: Broken, Detail: problem}
	}
	var last Result
	for _, method := range []string{http.MethodHead, http.MethodGet} {
		req, err := http.NewRequestWithContext(ctx, method, link, nil)
		if err != nil {
			return Result{State: Broken, Detail: err.Error()}
		}
		req.Header.Set("User-Agent", userAgent)
		req.Header.Set("Accept", "text/html,application/xhtml+xml,*/*")
		resp, err := c.client.Do(req)
		if err != nil {
			var dns *net.DNSError
			if errors.As(err, &dns) && dns.IsNotFound {
				return Result{State: Broken, Detail: "its domain doesn't exist (" + dns.Name + ")"}
			}
			last = Result{State: Unknown, Detail: "couldn't load it: " + shortError(err)}
			continue
		}
		io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
		resp.Body.Close()
		status := fmt.Sprintf("returns %s", resp.Status)
		switch {
		case resp.StatusCode < 400:
			return Result{State: OK, Status: resp.StatusCode, Detail: status}
		case method == http.MethodHead:
			// Plenty of sites answer HEAD wrongly. Ask again with GET.
			last = Result{State: Broken, Status: resp.StatusCode, Detail: status}
			continue
		case resp.StatusCode == 401 || resp.StatusCode == 403 || resp.StatusCode == 429:
			return Result{State: Unknown, Status: resp.StatusCode, Detail: status + ", which can mean it blocks automated checks. Open it yourself."}
		default:
			return Result{State: Broken, Status: resp.StatusCode, Detail: status}
		}
	}
	return last
}

func shortError(err error) string {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		err = urlErr.Err
	}
	if errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "Client.Timeout") {
		return "it took too long to answer"
	}
	return err.Error()
}

// Invalid says what's wrong with a link that isn't a full http(s) URL, or ""
// for one that is.
func Invalid(link string) string {
	link = strings.TrimSpace(link)
	if link == "" {
		return "there's no link"
	}
	u, err := url.Parse(link)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return fmt.Sprintf("%q isn't a full http(s) link", link)
	}
	if !strings.Contains(u.Hostname(), ".") && u.Hostname() != "localhost" {
		return fmt.Sprintf("%q has no real domain", link)
	}
	if u.Hostname() == "localhost" || strings.HasPrefix(u.Hostname(), "127.") || strings.HasPrefix(u.Hostname(), "192.168.") {
		return fmt.Sprintf("%q only works on their own computer", link)
	}
	return ""
}

// notDemos are sites that host notebooks or puzzle answers rather than a
// project people can try. Sidekick turns the same ones away.
var notDemos = []string{
	"colab.research.google.com", "colab.google.com", "kaggle.com", "mybinder.org", "binder.org",
	"notebooks.gesis.org", "leetcode.com",
}

// NotADemo names the site if link is on one that doesn't count as a demo.
func NotADemo(link string) string {
	u, err := url.Parse(strings.TrimSpace(link))
	if err != nil {
		return ""
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	for _, site := range notDemos {
		if host == site || strings.HasSuffix(host, "."+site) {
			return site
		}
	}
	return ""
}

// Dev is for DEV_MODE, where the example projects have made-up links: links
// on made-up dev- hosts open, unless they have "broken" in them, and anything
// else is checked for real.
type Dev struct{ Real Checker }

func (d Dev) Check(ctx context.Context, link string) Result {
	if problem := Invalid(link); problem != "" {
		return Result{State: Broken, Detail: problem}
	}
	u, _ := url.Parse(link)
	host := strings.ToLower(u.Hostname())
	fake := strings.HasPrefix(host, "dev-") || strings.HasPrefix(strings.ToLower(u.Path), "/dev-")
	switch {
	case fake && strings.Contains(strings.ToLower(link), "broken"):
		return Result{State: Broken, Status: 404, Detail: "returns 404 Not Found (dev mode: the link has \"broken\" in it)"}
	case fake:
		return Result{State: OK, Status: 200, Detail: "returns 200 OK (dev mode: made-up links open)"}
	}
	return d.Real.Check(ctx, link)
}
