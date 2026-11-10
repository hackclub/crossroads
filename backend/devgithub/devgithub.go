// Package devgithub is a stand-in for GitHub's API for local development, so
// the review page's GitHub stats and checks work on the made-up example
// projects. It's only mounted when DEV_MODE=1 (see main.go), and serves the
// few endpoints the github package reads.
//
// Repos whose owner starts with "dev-" exist, with commits made up from the
// repo's name, so they're the same every time. Words in the repo name pick a
// case to try:
//
//	private, missing, broken   the repo doesn't exist or is private
//	stale                      no commits since the event started
//	template                   the README is a framework's template
//	no-readme                  there's no README
//	team                       someone else made a third of the commits
package devgithub

import (
	"encoding/base64"
	"fmt"
	"hash/fnv"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/scherepi/crossroads-backend/respond"
)

type Fake struct {
	// When the example repos were started, and when time starts counting.
	Created, HoursStart time.Time
}

func (f *Fake) Routes() chi.Router {
	r := chi.NewRouter()
	r.Route("/repos/{owner}/{repo}", func(r chi.Router) {
		r.Get("/", f.repo)
		r.Get("/commits", f.commits)
		r.Get("/commits/{sha}", f.commit)
		r.Get("/readme", f.readme)
	})
	return r
}

func has(r *http.Request, word string) bool {
	return strings.Contains(strings.ToLower(chi.URLParam(r, "repo")), word)
}

func exists(r *http.Request) bool {
	return strings.HasPrefix(strings.ToLower(chi.URLParam(r, "owner")), "dev-") &&
		!has(r, "private") && !has(r, "missing") && !has(r, "broken")
}

func notFound(w http.ResponseWriter) {
	respond.JSON(w, http.StatusNotFound, map[string]string{"message": "Not Found"})
}

func (f *Fake) repo(w http.ResponseWriter, r *http.Request) {
	if !exists(r) {
		notFound(w)
		return
	}
	owner, repo := chi.URLParam(r, "owner"), chi.URLParam(r, "repo")
	commits := f.all(r)
	pushed := f.Created
	if len(commits) > 0 {
		pushed = commits[0].date
	}
	respond.JSON(w, http.StatusOK, map[string]any{
		"full_name":        owner + "/" + repo,
		"html_url":         "https://github.com/" + owner + "/" + repo,
		"description":      "An example project (dev mode)",
		"language":         []string{"TypeScript", "Rust", "Python", "C", "Go", "Svelte"}[hash(repo)%6],
		"stargazers_count": hash(repo) % 40,
		"private":          false,
		"fork":             false,
		"archived":         false,
		"created_at":       f.Created,
		"pushed_at":        pushed,
		"default_branch":   "main",
	})
}

type commit struct {
	sha, message, author string
	date                 time.Time
}

var messages = []string{
	"Add the first screen", "Fix crash on start", "Tidy up the README", "Make it faster", "Add settings",
	"Handle empty input", "Style the buttons", "Add tests", "Fix typo", "Save progress between runs",
	"Refactor the main loop", "Add a dark theme", "Fix off-by-one in the parser", "Deploy config",
}

// all makes up a repo's commits, newest first: one every 9 to 19 hours from
// Created until now.
func (f *Fake) all(r *http.Request) []commit {
	owner, repo := chi.URLParam(r, "owner"), chi.URLParam(r, "repo")
	end := time.Now()
	if has(r, "stale") {
		end = f.HoursStart.Add(-24 * time.Hour)
	}
	var out []commit
	t := f.Created
	for i := 0; ; i++ {
		h := hash(fmt.Sprintf("%s/%s#%d", owner, repo, i))
		t = t.Add(time.Duration(9+h%11) * time.Hour)
		if t.After(end) {
			break
		}
		author := owner
		if has(r, "team") && i%3 == 2 {
			author = "dev-friend"
		}
		out = append(out, commit{
			sha: fmt.Sprintf("%040x", uint64(h)*2654435761+uint64(i)), message: messages[h%uint32(len(messages))],
			author: author, date: t.UTC(),
		})
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

func (f *Fake) commits(w http.ResponseWriter, r *http.Request) {
	if !exists(r) {
		notFound(w)
		return
	}
	var matching []commit
	since, _ := time.Parse(time.RFC3339, r.URL.Query().Get("since"))
	until, _ := time.Parse(time.RFC3339, r.URL.Query().Get("until"))
	for _, c := range f.all(r) {
		if (since.IsZero() || !c.date.Before(since)) && (until.IsZero() || !c.date.After(until)) {
			matching = append(matching, c)
		}
	}
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	perPage, page = max(perPage, 1), max(page, 1)
	if pages := (len(matching) + perPage - 1) / perPage; pages > 1 {
		w.Header().Set("Link", fmt.Sprintf(`<%s?per_page=%d&page=%d>; rel="last"`, r.URL.Path, perPage, pages))
	}
	body := []map[string]any{}
	for i := (page - 1) * perPage; i < min(page*perPage, len(matching)); i++ {
		c := matching[i]
		body = append(body, map[string]any{
			"sha":    c.sha,
			"commit": map[string]any{"message": c.message, "author": map[string]any{"name": c.author, "date": c.date}},
			"author": map[string]any{"login": c.author},
		})
	}
	respond.JSON(w, http.StatusOK, body)
}

func (f *Fake) commit(w http.ResponseWriter, r *http.Request) {
	if !exists(r) {
		notFound(w)
		return
	}
	h := hash(chi.URLParam(r, "sha"))
	respond.JSON(w, http.StatusOK, map[string]any{
		"stats": map[string]int{"additions": 4 + int(h%140), "deletions": int(h % 45)},
	})
}

const readme = `# %s

A small project made during Crossroads. It does one thing well, and this
README says how to run it, what it does, and what's next.

## Running it

Clone it, install the dependencies, and start it.
`

const templateReadme = `# React + Vite

This template provides a minimal setup to get React working in Vite with HMR and some ESLint rules.
`

func (f *Fake) readme(w http.ResponseWriter, r *http.Request) {
	if !exists(r) || has(r, "no-readme") {
		notFound(w)
		return
	}
	text := fmt.Sprintf(readme, chi.URLParam(r, "repo"))
	if has(r, "template") {
		text = templateReadme
	}
	respond.JSON(w, http.StatusOK, map[string]string{
		"content": base64.StdEncoding.EncodeToString([]byte(text)), "encoding": "base64",
	})
}

func hash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}
