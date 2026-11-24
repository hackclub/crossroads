// Package unified knows what the Unified YSWS DB needs from a ship, so the
// review checks can say what's missing and the Airtable sync can hold back a
// row until it's complete. It can also look a project's links up in the
// Unified YSWS DB, to catch a project another program already paid for.
package unified

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/scherepi/crossroads-backend/airtable"
	"github.com/scherepi/crossroads-backend/db"
)

// placeholders aren't anyone's name.
var placeholders = []string{
	"test", "testing", "asdf", "qwerty", "n/a", "na", "none", "unknown", "null", "nil", "first", "last",
	"firstname", "lastname", "name", "user", "anonymous", "anon", "xxx", "abc", "-", ".",
}

// NameProblem says what's wrong with a first and last name, or "" if they
// look like a real full name. It errs toward passing: names come in every
// shape, so it only catches the empty, the placeholder and the obviously
// not-a-name.
func NameProblem(first, last string) string {
	first, last = strings.TrimSpace(first), strings.TrimSpace(last)
	switch {
	case first == "" && last == "":
		return "No name on file."
	case first == "":
		return "No first name on file."
	case last == "":
		return "No last name on file, only a first name (" + first + ")."
	}
	for _, part := range []struct{ label, name string }{{"first name", first}, {"last name", last}} {
		letters := 0
		for _, r := range part.name {
			switch {
			case unicode.IsLetter(r):
				letters++
			case unicode.IsMark(r), unicode.IsSpace(r), strings.ContainsRune("-'’.", r):
			default:
				return fmt.Sprintf("The %s on file (%q) has characters that aren't in names.", part.label, part.name)
			}
		}
		if letters == 0 {
			return fmt.Sprintf("The %s on file (%q) has no letters.", part.label, part.name)
		}
		if slices.Contains(placeholders, strings.ToLower(part.name)) {
			return fmt.Sprintf("The %s on file (%q) looks like a placeholder.", part.label, part.name)
		}
	}
	return ""
}

// GitHubUsername is the owner of a github.com repo link, which the Unified
// YSWS DB takes as the GitHub username.
func GitHubUsername(repoURL *string) *string {
	if repoURL == nil {
		return nil
	}
	u, err := url.Parse(strings.TrimSpace(*repoURL))
	if err != nil || strings.TrimPrefix(strings.ToLower(u.Host), "www.") != "github.com" {
		return nil
	}
	owner, _, _ := strings.Cut(strings.Trim(u.Path, "/"), "/")
	if owner == "" {
		return nil
	}
	return &owner
}

func isLink(s *string) bool {
	if s == nil {
		return false
	}
	u, err := url.Parse(strings.TrimSpace(*s))
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && strings.Contains(u.Hostname(), ".")
}

// Missing lists what a ship's row needs before it can go to the Unified YSWS
// DB, besides the person being YSWS-eligible: an empty list means it's ready.
func Missing(user db.User, project db.Project) []string {
	var missing []string
	if NameProblem(user.FirstName, user.LastName) != "" {
		missing = append(missing, "a real full name")
	}
	if strings.TrimSpace(user.Email) == "" {
		missing = append(missing, "an email")
	}
	if user.Birthday == nil {
		missing = append(missing, "a birthday")
	}
	switch parts := AddressGaps(user.Address); {
	case len(parts) == 4:
		missing = append(missing, "an address")
	case len(parts) > 0:
		missing = append(missing, "a full address (no "+strings.Join(parts, ", ")+")")
	}
	if strings.TrimSpace(project.Description) == "" {
		missing = append(missing, "a project description")
	}
	if project.ScreenshotUrl == nil || strings.TrimSpace(*project.ScreenshotUrl) == "" {
		missing = append(missing, "a screenshot")
	}
	if !isLink(project.RepoUrl) {
		missing = append(missing, "a repo link")
	} else if GitHubUsername(project.RepoUrl) == nil {
		missing = append(missing, "a repo on GitHub (the Unified YSWS DB asks for a GitHub username)")
	}
	if !isLink(project.DemoUrl) {
		missing = append(missing, "a demo link")
	}
	return missing
}

// AddressGaps lists the parts of an address the Unified YSWS DB needs that
// aren't filled in: street, city, postal code and country.
func AddressGaps(raw []byte) []string {
	var address db.Address
	_ = json.Unmarshal(raw, &address)
	var gaps []string
	for _, p := range []struct{ label, value string }{
		{"street", address.Line1}, {"city", address.City}, {"postal code", address.PostalCode}, {"country", address.Country},
	} {
		if strings.TrimSpace(p.value) == "" {
			gaps = append(gaps, p.label)
		}
	}
	return gaps
}

// A Record is a project already in the Unified YSWS DB. It holds no personal
// details: reviewers only need to know which program paid for what.
type Record struct {
	Program     string  `json:"program"`
	Hours       float64 `json:"hours"`
	ApprovedAt  string  `json:"approved_at,omitempty"`
	CodeURL     string  `json:"code_url,omitempty"`
	PlayableURL string  `json:"playable_url,omitempty"`
	// The start of the project's description; only from FindByPerson.
	Description string `json:"description,omitempty"`
}

// Person is someone's history in the Unified YSWS DB: their row in YSWS
// Authors (trust level and weighted grants, when they have one) and the
// projects every program approved for them, newest first.
type Person struct {
	Found          bool     `json:"found"` // they're in YSWS Authors
	TrustLevel     string   `json:"trust_level,omitempty"`
	WeightedGrants float64  `json:"weighted_grants"`
	Projects       []Record `json:"projects"`
}

// A Lookup searches the Unified YSWS DB: for projects with the same repo or
// demo link, and for everything one person has had approved.
type Lookup interface {
	FindByLinks(ctx context.Context, links ...string) ([]Record, error)
	FindByPerson(ctx context.Context, email, slackID string) (Person, error)
}

// The Unified YSWS DB's approved projects table (the same one Sidekick reads),
// and its YSWS Authors table: one row per person, linked to their projects.
const (
	BaseID         = "app3A5kJwYqxMLOgh"
	TableID        = "tblzWWGUYHVH7Zyqf"
	AuthorsTableID = "tblRf1BQs5H8298gW"
)

// maxPersonProjects is the most of someone's approved projects kept.
const maxPersonProjects = 50

// Airtable reads the Unified YSWS DB. It only ever reads.
type Airtable struct {
	client *airtable.Client
	// What it found lately. Each search reads through a table of every
	// project every program has approved, which takes Airtable seconds, and a
	// review page asks three of them every time it loads.
	links  cache[[]Record]
	people cache[Person]
}

// How long an answer is good for. A project another program approved shows up
// here this long after it's in the Unified YSWS DB, where people add rows by
// hand, days after the work.
const rememberFor = 5 * time.Minute

// cache remembers answers by key for rememberFor. The zero value is ready to use.
type cache[T any] struct {
	mu    sync.Mutex
	found map[string]cached[T]
}

type cached[T any] struct {
	value   T
	expires time.Time
}

// recall is what was found for key, if that's still good.
func (c *cache[T]) recall(key string) (T, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	hit, ok := c.found[key]
	if !ok || time.Now().After(hit.expires) {
		delete(c.found, key)
		var none T
		return none, false
	}
	return hit.value, true
}

func (c *cache[T]) remember(key string, value T) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.found == nil {
		c.found = map[string]cached[T]{}
	}
	// Most keys are never asked for again once their ship is decided, so
	// what's expired goes whenever something is added.
	now := time.Now()
	for k, old := range c.found {
		if now.After(old.expires) {
			delete(c.found, k)
		}
	}
	c.found[key] = cached[T]{value, now.Add(rememberFor)}
}

// NewAirtable needs a personal access token with data.records:read on the
// Unified YSWS base. A read-only token is all it should be given.
func NewAirtable(token string) *Airtable {
	return &Airtable{client: airtable.New(airtable.Config{Token: token, BaseID: BaseID})}
}

func (a *Airtable) FindByLinks(ctx context.Context, links ...string) ([]Record, error) {
	var wanted, conditions []string
	for _, link := range links {
		n := Normalize(link)
		if n == "" || slices.Contains(wanted, n) {
			continue
		}
		wanted = append(wanted, n)
		quoted := formulaString(n)
		for _, field := range []string{"Code URL", "Playable URL"} {
			conditions = append(conditions, fmt.Sprintf(`FIND("%s", LOWER({%s}))`, quoted, field))
		}
	}
	if len(conditions) == 0 {
		return nil, nil
	}
	key := strings.Join(wanted, "\n")
	if records, ok := a.links.recall(key); ok {
		return records, nil
	}
	rows, err := a.client.List(ctx, TableID, "OR("+strings.Join(conditions, ",")+")",
		"Code URL", "Playable URL", "Hours Spent", "Override Hours Spent", "YSWS–Name", "Approved At")
	if err != nil {
		return nil, err
	}
	var records []Record
	for _, row := range rows {
		r := toRecord(row)
		// FIND matches inside longer links too: keep only the same project.
		if !slices.ContainsFunc(wanted, func(w string) bool { return sameProject(w, r.CodeURL) || sameProject(w, r.PlayableURL) }) {
			continue
		}
		records = append(records, r)
	}
	a.links.remember(key, records)
	return records, nil
}

// FindByPerson finds everything a person has had approved, by the email on
// their projects, and their YSWS Authors row, by Slack ID or Hack Club email.
func (a *Airtable) FindByPerson(ctx context.Context, email, slackID string) (Person, error) {
	person := Person{Projects: []Record{}}
	email = strings.ToLower(strings.TrimSpace(email))
	var who []string
	if slackID != "" {
		who = append(who, fmt.Sprintf(`{Slack ID} = "%s"`, formulaString(slackID)))
	}
	if email != "" {
		who = append(who, fmt.Sprintf(`LOWER(TRIM({Hack Club Auth Email})) = "%s"`, formulaString(email)))
	}
	key := email + "\n" + slackID
	if p, ok := a.people.recall(key); ok {
		return p, nil
	}

	// Their projects and their YSWS Authors row are two tables, asked for at
	// the same time.
	var (
		wg                      sync.WaitGroup
		projects, authors       []airtable.Record
		projectsErr, authorsErr error
	)
	if email != "" {
		wg.Go(func() {
			projects, projectsErr = a.client.List(ctx, TableID, fmt.Sprintf(`LOWER(TRIM({Email})) = "%s"`, formulaString(email)),
				"Code URL", "Playable URL", "Hours Spent", "Override Hours Spent", "YSWS–Name", "Approved At", "Description")
		})
	}
	if len(who) > 0 {
		wg.Go(func() {
			authors, authorsErr = a.client.List(ctx, AuthorsTableID, "OR("+strings.Join(who, ",")+")", "Unified Trust Level", "Weighted Grants–Total")
		})
	}
	wg.Wait()
	if projectsErr != nil {
		return person, projectsErr
	}
	for _, row := range projects {
		person.Projects = append(person.Projects, toRecord(row))
	}
	slices.SortStableFunc(person.Projects, func(x, y Record) int { return strings.Compare(y.ApprovedAt, x.ApprovedAt) })
	if len(person.Projects) > maxPersonProjects {
		person.Projects = person.Projects[:maxPersonProjects]
	}
	if authorsErr != nil {
		return person, authorsErr
	}
	if len(authors) > 0 {
		person.Found = true
		person.TrustLevel = text(authors[0].Fields["Unified Trust Level"])
		person.WeightedGrants = number(authors[0].Fields["Weighted Grants–Total"])
	}
	a.people.remember(key, person)
	return person, nil
}

// toRecord reads an Approved Projects row.
func toRecord(row airtable.Record) Record {
	r := Record{CodeURL: text(row.Fields["Code URL"]), PlayableURL: text(row.Fields["Playable URL"]), ApprovedAt: text(row.Fields["Approved At"])}
	r.Hours = number(row.Fields["Hours Spent"])
	if override, ok := row.Fields["Override Hours Spent"].(float64); ok {
		r.Hours = override
	}
	switch program := row.Fields["YSWS–Name"].(type) {
	case []any:
		if len(program) > 0 {
			r.Program = text(program[0])
		}
	default:
		r.Program = text(program)
	}
	if d := strings.Join(strings.Fields(text(row.Fields["Description"])), " "); d != "" {
		if runes := []rune(d); len(runes) > 200 {
			d = string(runes[:200]) + "…"
		}
		r.Description = d
	}
	return r
}

// formulaString makes a link safe inside an Airtable formula's string
// literal: anything that isn't part of a link is dropped, and the two
// characters Airtable gives meaning to are escaped.
func formulaString(link string) string {
	var b strings.Builder
	for _, r := range link {
		switch {
		case r == '\\' || r == '"':
			b.WriteRune('\\')
			b.WriteRune(r)
		case r < 0x20 || r == 0x7f:
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func text(v any) string {
	s, _ := v.(string)
	return s
}

func number(v any) float64 {
	f, _ := v.(float64)
	return f
}

// Normalize makes two links to the same project compare equal: no scheme,
// www, query, trailing slash or .git, and a GitHub link to a branch or file
// is the repo.
func Normalize(link string) string {
	s := strings.ToLower(strings.TrimSpace(link))
	s = strings.TrimPrefix(strings.TrimPrefix(s, "https://"), "http://")
	s = strings.TrimPrefix(s, "www.")
	s, _, _ = strings.Cut(s, "#")
	s, _, _ = strings.Cut(s, "?")
	s = strings.TrimSuffix(strings.TrimRight(s, "/"), ".git")
	if strings.HasPrefix(s, "github.com/") {
		parts := strings.SplitN(s, "/", 4)
		if len(parts) >= 3 {
			s = strings.Join(parts[:3], "/")
		}
	}
	return s
}

func sameProject(normalized, stored string) bool {
	s := Normalize(stored)
	return s != "" && (s == normalized || strings.HasPrefix(s, normalized+"/") || strings.HasPrefix(normalized, s+"/"))
}

// Dev is for DEV_MODE, which never reads the real Unified YSWS DB: a link with
// "double-dip" in it is found there, under another program.
type Dev struct{}

func (Dev) FindByLinks(ctx context.Context, links ...string) ([]Record, error) {
	for _, link := range links {
		if strings.Contains(strings.ToLower(link), "double-dip") {
			return []Record{{Program: "Summer of Making (dev mode example)", Hours: 12, ApprovedAt: "2026-08-14", CodeURL: link}}, nil
		}
	}
	return nil, nil
}

// FindByPerson in dev mode: someone whose email has "sana" or "maya" in it has
// a history from other programs; nobody else has one.
func (Dev) FindByPerson(ctx context.Context, email, slackID string) (Person, error) {
	e := strings.ToLower(email)
	if !strings.Contains(e, "sana") && !strings.Contains(e, "maya") {
		return Person{Projects: []Record{}}, nil
	}
	return Person{Found: true, TrustLevel: "YSWS Resident", WeightedGrants: 3.5, Projects: []Record{
		{Program: "Summer of Making (dev mode example)", Hours: 12, ApprovedAt: "2026-08-14", CodeURL: "https://github.com/dev/flashcards-double-dip",
			Description: "Spaced-repetition flashcards for learning vocabulary."},
		{Program: "Athena (dev mode example)", Hours: 6.5, ApprovedAt: "2026-03-02", CodeURL: "https://github.com/dev/weather-cli",
			PlayableURL: "https://dev.github.io/weather-cli", Description: "A command-line weather app with ASCII-art forecasts."},
	}}, nil
}
