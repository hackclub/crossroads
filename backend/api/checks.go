package api

// The automatic checks on a ship. They run when a reviewer opens it, and
// again when the first reviewer approves it, and the fraud checks among them
// decide whether it goes to the fraud check (see decide in stages.go).
//
// Most of them are the ones Sidekick runs (its D, G, H and AT checks), plus
// Crossroads' own. A check that needs a service that's down, or isn't set up,
// says it couldn't run rather than failing.

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/scherepi/crossroads-backend/db"
	"github.com/scherepi/crossroads-backend/github"
	"github.com/scherepi/crossroads-backend/hackatime"
	"github.com/scherepi/crossroads-backend/linkcheck"
	"github.com/scherepi/crossroads-backend/unified"
)

// check is one automatic check on a ship. The JSON is close to a Sidekick
// program check, so Sidekick can show it as is.
type check struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Status   string `json:"status"`   // pass, fail or inconclusive
	Severity string `json:"severity"` // info, warn, fail or critical, when it doesn't pass
	Summary  string `json:"summary,omitempty"`
	// A ship whose fraud checks fail, or can't run, goes to the fraud check.
	Fraud bool `json:"fraud"`
}

type checkReport struct {
	Checks         []check
	Hackatime      *reviewHackatimeJSON
	HackatimeError string
	SharedLinks    []sharedLinkProjectJSON
	GitHub         *github.Report
	GitHubError    string
	Unified        []unified.Record // this project's links in the Unified YSWS DB, from other programs
	// Hackatime's time and GitHub's commits per day (UTC) since the window
	// started. Daily totals only: reviewers never see heartbeats.
	Days        []dayJSON
	WindowStart time.Time
}

type dayJSON struct {
	Day     string `json:"day"` // 2006-01-02
	Seconds int    `json:"seconds"`
	Commits int    `json:"commits"`
}

// fraudReasons are the IDs of the fraud checks that didn't pass.
func (r checkReport) fraudReasons() []string {
	reasons := []string{}
	for _, c := range r.Checks {
		if c.Fraud && c.Status != "pass" {
			reasons = append(reasons, c.ID)
		}
	}
	return reasons
}

func fraudReasonText(reasons []string, checks []check) string {
	var lines []string
	for _, reason := range reasons {
		switch reason {
		case "manual":
			lines = append(lines, "The reviewer asked for a fraud check.")
		case "everything":
			lines = append(lines, "Every ship goes through the fraud check right now (admin setting).")
		default:
			for _, c := range checks {
				if c.ID == reason {
					lines = append(lines, c.Name+": "+c.Summary)
				}
			}
		}
	}
	return strings.Join(lines, "\n")
}

// windowStart is when the time a ship claims starts: the project's last
// approved ship before it, or the start of the event.
func (a *API) windowStart(ctx context.Context, s db.Submission) (start time.Time, reship bool, err error) {
	start = a.HoursStart
	earlier, err := a.Queries.ListSubmissions(ctx, s.ProjectID)
	if err != nil {
		return start, false, err
	}
	for _, e := range earlier {
		if e.Status == db.SubmissionStatusApproved && e.ID < s.ID && e.SubmittedAt.After(start) {
			start, reship = e.SubmittedAt, true
		}
	}
	return start, reship, nil
}

// runChecks runs the automatic checks on a ship, with Hackatime's and
// GitHub's numbers as of now. The outside services are asked at the same time.
func (a *API) runChecks(ctx context.Context, row db.GetSubmissionForReviewRow, settings db.ReviewSetting) (checkReport, error) {
	s, project, user := row.Submission, row.Project, row.User
	var report checkReport
	since, reship, err := a.windowStart(ctx, s)
	if err != nil {
		return report, err
	}
	report.WindowStart = since

	var (
		wg         sync.WaitGroup
		stats      hackatime.Stats
		statsErr   error
		gh         *github.Report
		ghErr      error
		repoLink   linkcheck.Result
		demoLink   linkcheck.Result
		inUnified  []unified.Record
		unifiedErr error
	)
	repoURL, demoURL := deref(project.RepoUrl), deref(project.DemoUrl)
	_, _, onGitHub := github.ParseRepo(repoURL)
	if user.SlackID != nil {
		wg.Go(func() { stats, statsErr = a.Hackatime.Stats(ctx, *user.SlackID, s.HackatimeProjects) })
	}
	switch {
	case onGitHub && a.GitHub != nil:
		wg.Go(func() { gh, ghErr = a.GitHub.Activity(ctx, repoURL, since, s.SubmittedAt) })
	case !onGitHub && repoURL != "" && a.Links != nil:
		wg.Go(func() { repoLink = a.Links.Check(ctx, repoURL) })
	}
	if demoURL != "" && a.Links != nil && linkcheck.Invalid(demoURL) == "" && linkcheck.NotADemo(demoURL) == "" {
		wg.Go(func() { demoLink = a.Links.Check(ctx, demoURL) })
	}
	if a.Unified != nil && (repoURL != "" || demoURL != "") {
		wg.Go(func() { inUnified, unifiedErr = a.Unified.FindByLinks(ctx, repoURL, demoURL) })
	}
	wg.Wait()

	// Hackatime (Sidekick's H1, and Crossroads' dropped-time check).
	hackatimeCheck := check{ID: "hackatime_flagged", Name: "Hackatime hasn't flagged them", Fraud: true, Severity: "critical"}
	drop := check{ID: "hackatime_dropped", Name: "Hackatime time hasn't gone down since shipping", Fraud: true, Severity: "warn"}
	switch {
	case user.SlackID == nil:
		report.HackatimeError = "They don't have Slack linked to their Hack Club account."
	case statsErr != nil:
		report.HackatimeError = statsErr.Error()
	default:
		report.Hackatime = &reviewHackatimeJSON{Seconds: stats.Seconds, Projects: []hackatimeProjectJSON{}, Convicted: stats.Convicted}
		for _, p := range stats.Projects {
			report.Hackatime.Projects = append(report.Hackatime.Projects, hackatimeProjectJSON{Name: p.Name, Seconds: p.Seconds})
		}
	}
	switch {
	case report.Hackatime == nil:
		hackatimeCheck.Status, hackatimeCheck.Summary = "inconclusive", "Couldn't ask Hackatime: "+report.HackatimeError
		drop.Status, drop.Summary = "inconclusive", hackatimeCheck.Summary
	default:
		hackatimeCheck.Status = passIf(!report.Hackatime.Convicted)
		if report.Hackatime.Convicted {
			hackatimeCheck.Summary = "Hackatime has caught this account cheating (trust level red)."
		}
		// Heartbeats deleted after shipping are a sign of something to hide.
		drop.Status = passIf(report.Hackatime.Seconds >= int(s.HackatimeSeconds)-60)
		if drop.Status == "fail" {
			drop.Summary = fmt.Sprintf("Hackatime had %s on these projects when they shipped, and has %s now.",
				hoursText(int(s.HackatimeSeconds)), hoursText(report.Hackatime.Seconds))
		}
	}
	report.Checks = append(report.Checks, hackatimeCheck, drop)

	// The repo (Sidekick's D3 and G1).
	repoCheck := check{ID: "repo_works", Name: "Repo link opens, and the repo is public", Severity: "fail"}
	switch {
	case repoURL == "":
		repoCheck.Status, repoCheck.Summary = "fail", "There's no repo link."
	case linkcheck.Invalid(repoURL) != "":
		repoCheck.Status, repoCheck.Summary = "fail", "The repo link doesn't work: "+linkcheck.Invalid(repoURL)+"."
	case onGitHub && a.GitHub == nil:
		repoCheck.Status, repoCheck.Summary = "inconclusive", "GitHub isn't set up on this server."
	case onGitHub && errors.Is(ghErr, github.ErrNotFound):
		repoCheck.Status, repoCheck.Summary = "fail", "GitHub says this repo doesn't exist or is private, so reviewers can't see the code."
	case onGitHub && ghErr != nil:
		repoCheck.Status, repoCheck.Summary = "inconclusive", "Couldn't ask GitHub: "+ghErr.Error()
		report.GitHubError = ghErr.Error()
	case onGitHub:
		report.GitHub = gh
		repoCheck.Status = "pass"
		repoCheck.Summary = fmt.Sprintf("Public on GitHub, with %d commit%s.", gh.TotalCommits, plural(gh.TotalCommits))
		if gh.Fork {
			repoCheck.Summary += " It's a fork: check what they added."
		}
	case a.Links == nil:
		repoCheck.Status, repoCheck.Summary = "inconclusive", "Link checks are off on this server."
	default:
		repoCheck.Status, repoCheck.Summary = linkStatus(repoLink), "The repo link "+repoLink.Detail+". It isn't on GitHub, so there are no commit stats."
	}
	report.Checks = append(report.Checks, repoCheck)

	// The demo (Sidekick's D2 and D4).
	demoCheck := check{ID: "demo_works", Name: "Demo link opens", Severity: "fail"}
	switch {
	case demoURL == "":
		demoCheck.Status, demoCheck.Summary = "fail", "There's no demo link."
	case linkcheck.Invalid(demoURL) != "":
		demoCheck.Status, demoCheck.Summary = "fail", "The demo link doesn't work: "+linkcheck.Invalid(demoURL)+"."
	case linkcheck.NotADemo(demoURL) != "":
		demoCheck.Status = "fail"
		demoCheck.Summary = "The demo is on " + linkcheck.NotADemo(demoURL) + ", which hosts notebooks or puzzles, not a project people can try."
	case a.Links == nil:
		demoCheck.Status, demoCheck.Summary = "inconclusive", "Link checks are off on this server."
	default:
		demoCheck.Status, demoCheck.Summary = linkStatus(demoLink), "The demo link "+demoLink.Detail+"."
		if unified.Normalize(demoURL) == unified.Normalize(repoURL) {
			demoCheck.Summary += " It's the same as the repo link: is there really nothing to try?"
		}
	}
	report.Checks = append(report.Checks, demoCheck)

	// The README and the commits (Sidekick's G2 and G3), when GitHub answered.
	if gh != nil && report.GitHub != nil {
		report.Checks = append(report.Checks, readmeCheck(gh.Readme))
		commits := check{ID: "new_commits", Severity: "fail", Status: passIf(len(gh.Commits) > 0)}
		when := "since Crossroads started"
		if reship {
			when = "since the last approved ship"
		}
		commits.Name = "Commits " + when
		if len(gh.Commits) == 0 {
			commits.Summary = fmt.Sprintf("No commits %s (%s) up to shipping. Is the work pushed, or was it done before?", when, since.Format("Jan 2"))
		} else {
			commits.Summary = fmt.Sprintf("%d%s commit%s %s.", len(gh.Commits), more(gh.Truncated), plural(len(gh.Commits)), when)
		}
		report.Checks = append(report.Checks, commits)
	}

	// Projects sharing links, here and in the Unified YSWS DB (Sidekick's AT1).
	shared, err := a.Queries.ListProjectsSharingLinks(ctx, db.ListProjectsSharingLinksParams{
		ProjectID: project.ID, RepoUrl: project.RepoUrl, DemoUrl: project.DemoUrl,
	})
	if err != nil {
		return report, err
	}
	report.SharedLinks = []sharedLinkProjectJSON{}
	var sharedNames []string
	for _, p := range shared {
		report.SharedLinks = append(report.SharedLinks, sharedLinkProjectJSON{
			ID: p.ID, Name: p.Name, RepoURL: p.RepoUrl, DemoURL: p.DemoUrl, UserID: p.UserID, UserName: p.UserName,
		})
		sharedNames = append(sharedNames, fmt.Sprintf("%q by %s", p.Name, p.UserName))
	}
	sharedCheck := check{ID: "shared_links", Name: "No other project uses the same repo or demo", Status: passIf(len(shared) == 0), Severity: "fail", Fraud: true}
	if len(shared) > 0 {
		sharedCheck.Summary = "Also used by " + strings.Join(sharedNames, ", ") + ". A copied project, or one project split up to be paid twice?"
	}
	report.Checks = append(report.Checks, sharedCheck)

	if a.Unified != nil {
		dip := check{ID: "unified_double_dip", Name: "Not already paid for by another program", Severity: "critical"}
		report.Unified = []unified.Record{}
		for _, r := range inUnified {
			if !strings.Contains(strings.ToLower(r.Program), "crossroads") {
				report.Unified = append(report.Unified, r)
			}
		}
		switch {
		case unifiedErr != nil:
			dip.Status, dip.Summary = "inconclusive", "Couldn't search the Unified YSWS DB: "+unifiedErr.Error()
		case len(report.Unified) > 0:
			dip.Status, dip.Fraud = "fail", true
			var programs []string
			for _, r := range report.Unified {
				programs = append(programs, fmt.Sprintf("%s (%sh)", cmp.Or(r.Program, "an unnamed program"), trimFloat(r.Hours)))
			}
			dip.Summary = "This repo or demo is already in the Unified YSWS DB from " + strings.Join(programs, ", ") +
				". Only time after that can count."
		default:
			dip.Status = "pass"
		}
		report.Checks = append(report.Checks, dip)
	}

	large := check{ID: "large_ship", Name: fmt.Sprintf("Claims %s or less", hoursText(int(settings.LargeShipSeconds))),
		Status: passIf(s.NewSeconds <= settings.LargeShipSeconds), Severity: "warn", Fraud: true}
	if large.Status == "fail" {
		large.Summary = fmt.Sprintf("This ship claims %s. Big ships always get a fraud check.", hoursText(int(s.NewSeconds)))
	}
	report.Checks = append(report.Checks, large)

	// Daily totals since the window started, and the busiest day.
	days, err := a.Queries.ListProjectDays(ctx, db.ListProjectDaysParams{ProjectID: project.ID, Since: since.UTC()})
	if err != nil {
		return report, err
	}
	long := check{ID: "long_day", Name: fmt.Sprintf("No day over %s", hoursText(int(settings.LongDaySeconds))), Status: "pass", Severity: "warn", Fraud: true}
	var busiest db.ListProjectDaysRow
	for _, d := range days {
		if d.Seconds > busiest.Seconds {
			busiest = d
		}
	}
	if busiest.Seconds > settings.LongDaySeconds {
		long.Status = "fail"
		long.Summary = fmt.Sprintf("Hackatime counted %s on this project on %s (UTC). Check for keep-alive scripts or autotyping.",
			hoursText(int(busiest.Seconds)), busiest.Day.Format("Jan 2"))
	}
	report.Checks = append(report.Checks, long)
	report.Days = dailyTotals(days, report.GitHub, since)

	// What the Unified YSWS DB needs.
	name := check{ID: "full_name", Name: "Real full name on file", Severity: "fail"}
	if problem := unified.NameProblem(user.FirstName, user.LastName); problem != "" {
		name.Status, name.Summary = "fail", problem+" It comes from their Hack Club account, and the Unified YSWS DB needs it."
	} else {
		name.Status, name.Summary = "pass", user.FirstName+" "+user.LastName
	}
	report.Checks = append(report.Checks, name)

	ready := check{ID: "unified_ready", Name: "Has everything the Unified YSWS DB needs", Severity: "warn"}
	if missing := unified.Missing(user, project); len(missing) > 0 {
		ready.Status = "fail"
		ready.Summary = "Missing " + list(missing) + ". It can still be approved, but it won't go to the Unified YSWS DB until that's filled in."
	} else {
		ready.Status = "pass"
	}
	report.Checks = append(report.Checks, ready)

	identity := check{ID: "identity", Name: "Identity verified and YSWS-eligible", Status: passIf(user.YswsEligible), Severity: "info"}
	if !user.YswsEligible {
		identity.Summary = fmt.Sprintf("Verification: %s. They can still ship, but can't buy anything, and the ship won't go to the Unified YSWS DB until they're verified.", user.VerificationStatus)
	}
	report.Checks = append(report.Checks, identity)
	return report, nil
}

// templateReadmes are lines from the READMEs project generators write.
// Sidekick looks for the same ones.
var templateReadmes = []string{
	"bootstrapped with [Create React App]", "bootstrapped with [`create-next-app`]", "bootstrapped with [`create-next-app",
	"powered by [`create-svelte`]", "generated with [Angular CLI]", "generated using [Angular CLI]",
	"This template provides a minimal setup to get React working in Vite",
	"This template should help get you started developing with Vue",
	"This template should help get you started developing with Svelte",
	"# React + TypeScript + Vite", "# React + Vite", "# Vue 3 + TypeScript + Vite", "Welcome to your new Gatsby site",
	"Bootstrapped with Create React Native App", "This README was generated automatically",
	"This project was bootstrapped with Create React App", "This is a starter template for",
	"# Getting Started with Create React App", "# create-svelte",
	"## Developing\n\nOnce you've created a project and installed dependencies",
	"To learn more about Next.js, take a look at the following resources",
	"## Getting Started\n\nFirst, run the development server", "This project uses [`next/font`]",
}

func readmeCheck(readme string) check {
	c := check{ID: "readme", Name: "README written for this project", Severity: "warn", Status: "fail"}
	text := strings.TrimSpace(readme)
	// Without the title line.
	body := text
	if strings.HasPrefix(body, "#") {
		_, body, _ = strings.Cut(body, "\n")
	}
	switch {
	case text == "":
		c.Summary = "The repo has no README, or it's empty."
	case len(strings.TrimSpace(body)) < 30:
		c.Summary = "The README is only a title."
	default:
		for _, t := range templateReadmes {
			if strings.Contains(text, t) {
				c.Summary = "The README is the template a project generator wrote, not one about this project."
				return c
			}
		}
		c.Status = "pass"
	}
	return c
}

func dailyTotals(days []db.ListProjectDaysRow, gh *github.Report, since time.Time) []dayJSON {
	byDay := map[string]*dayJSON{}
	var order []string
	add := func(day string) *dayJSON {
		if d, ok := byDay[day]; ok {
			return d
		}
		byDay[day] = &dayJSON{Day: day}
		order = append(order, day)
		return byDay[day]
	}
	for _, d := range days {
		add(d.Day.Format(time.DateOnly)).Seconds = int(d.Seconds)
	}
	if gh != nil {
		for _, c := range gh.Commits {
			if !c.Date.Before(since) {
				add(c.Date.UTC().Format(time.DateOnly)).Commits++
			}
		}
	}
	slices.Sort(order)
	out := []dayJSON{}
	for _, day := range order {
		out = append(out, *byDay[day])
	}
	return out
}

func linkStatus(r linkcheck.Result) string {
	switch r.State {
	case linkcheck.OK:
		return "pass"
	case linkcheck.Broken:
		return "fail"
	}
	return "inconclusive"
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return strings.TrimSpace(*s)
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func more(truncated bool) string {
	if truncated {
		return "+"
	}
	return ""
}

func trimFloat(f float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.1f", f), "0"), ".")
}

// list joins things for a sentence: "a, b and c".
func list(items []string) string {
	if len(items) <= 1 {
		return strings.Join(items, "")
	}
	return strings.Join(items[:len(items)-1], ", ") + " and " + items[len(items)-1]
}

func passIf(ok bool) string {
	if ok {
		return "pass"
	}
	return "fail"
}

// hoursText is seconds as hours for people: "1.5h".
func hoursText(seconds int) string {
	h := float64(seconds) / 3600
	if h == float64(int(h)) {
		return fmt.Sprintf("%dh", int(h))
	}
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", h), "0"), ".") + "h"
}
