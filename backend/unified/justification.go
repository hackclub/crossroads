package unified

import (
	"cmp"
	"fmt"
	"strings"
	"time"
)

// Review is everything a ship's Unified YSWS justification is written from:
// Hackatime's numbers and what each review stage decided and why.
type Review struct {
	HackatimeProjects []string
	HackatimeSeconds  int32 // on those projects when it shipped, since HoursStart
	NewSeconds        int32 // what the ship claimed: Hackatime time nobody had been paid for
	ApprovedSeconds   int32 // what's being sent
	HoursStart        time.Time
	SubmittedAt       time.Time

	ReviewerName      string
	ReviewerSeconds   int32  // what the first reviewer approved
	ReviewerDeflation string // why they approved less than the ship claimed
	TechnicalFeatures string
	Notes             string // the reviewer's justification
	OverrideReason    string // why more than Hackatime tracked, if it is

	FraudReasons    []string // what sent it to the fraud check (check IDs)
	FraudReviewer   string   // empty without a fraud check
	FraudSeconds    *int32   // what the fraud check passed it at
	FraudDeflation  string
	HQName          string // empty before sign-off
	HQDeflation     string
	EarlierApproval bool // an update to a project approved before
}

// reasonLabels name the fraud check's reasons for people reading the
// justification.
var reasonLabels = map[string]string{
	"hackatime_flagged":  "Hackatime flagged the account, or couldn't be asked",
	"hackatime_dropped":  "Hackatime time went down after shipping",
	"shared_links":       "another project uses the same repo or demo",
	"large_ship":         "a big ship",
	"long_day":           "a very long day",
	"unified_double_dip": "already in the Unified YSWS DB from another program",
	"manual":             "the reviewer asked",
	"everything":         "every ship gets one right now",
}

func hours(seconds int32) string { return fmt.Sprintf("%.2fh", float64(seconds)/3600) }

// sentence ends text with a full stop if it doesn't end with one already.
func sentence(text string) string {
	text = strings.TrimSpace(text)
	if text == "" || strings.HasSuffix(text, ".") || strings.HasSuffix(text, "!") || strings.HasSuffix(text, "?") {
		return text
	}
	return text + "."
}

// Justification is the text for "Optional - Override Hours Spent
// Justification": one line per thing a Unified YSWS reviewer would ask about,
// so they can see where every hour came from and who checked it.
func Justification(r Review) string {
	var lines []string
	add := func(format string, args ...any) { lines = append(lines, fmt.Sprintf(format, args...)) }

	add("Reviewed on Crossroads by a reviewer%s and HQ.", map[bool]string{true: ", a fraud check", false: ""}[r.FraudReviewer != ""])
	add("Hackatime: %s of new time on %s (%s tracked from %s to %s, minus time already paid for).",
		hours(r.NewSeconds), strings.Join(r.HackatimeProjects, ", "), hours(r.HackatimeSeconds),
		r.HoursStart.Format(time.DateOnly), r.SubmittedAt.UTC().Format(time.DateOnly))
	if r.EarlierApproval {
		add("This is an update to a project already approved on Crossroads: only time added since then counts.")
	}
	if r.TechnicalFeatures != "" {
		add("Technical features: %s", sentence(r.TechnicalFeatures))
	}

	if r.ReviewerName != "" {
		line := fmt.Sprintf("Reviewer %s approved %s.", r.ReviewerName, hours(r.ReviewerSeconds))
		if r.Notes != "" {
			line += " " + sentence(r.Notes)
		}
		lines = append(lines, line)
	}
	if r.ReviewerSeconds > r.NewSeconds && r.OverrideReason != "" {
		add("More than Hackatime tracked (%s of %s), because: %s", hours(r.ReviewerSeconds), hours(r.NewSeconds), sentence(r.OverrideReason))
	}
	if r.ReviewerName != "" && r.ReviewerSeconds < r.NewSeconds {
		lines = append(lines, deflated(r.NewSeconds, r.ReviewerSeconds, "by the reviewer", r.ReviewerDeflation))
	}

	passedAt := r.ReviewerSeconds
	switch {
	case r.FraudReviewer != "":
		var why []string
		for _, reason := range r.FraudReasons {
			why = append(why, cmp.Or(reasonLabels[reason], reason))
		}
		line := fmt.Sprintf("Sent to a fraud check because: %s. %s passed it", strings.Join(why, "; "), r.FraudReviewer)
		if r.FraudSeconds != nil {
			passedAt = *r.FraudSeconds
			line += " at " + hours(passedAt)
		}
		lines = append(lines, line+".")
		if r.FraudSeconds != nil && *r.FraudSeconds < r.ReviewerSeconds {
			lines = append(lines, deflated(r.ReviewerSeconds, *r.FraudSeconds, "in the fraud check", r.FraudDeflation))
		}
	case r.HQName != "":
		add("No automatic check flagged it for a fraud check (Hackatime trust level, shared links, ship size, longest day, Hackatime time going down, the Unified YSWS DB).")
	}

	if r.HQName != "" {
		add("Signed off by %s at %s.", r.HQName, hours(r.ApprovedSeconds))
		if r.ApprovedSeconds < passedAt {
			lines = append(lines, deflated(passedAt, r.ApprovedSeconds, "at sign-off", r.HQDeflation))
		}
		if r.ApprovedSeconds > r.NewSeconds && r.ReviewerSeconds <= r.NewSeconds && r.OverrideReason != "" {
			add("More than Hackatime tracked (%s of %s), because: %s", hours(r.ApprovedSeconds), hours(r.NewSeconds), sentence(r.OverrideReason))
		}
	}
	return strings.Join(lines, "\n")
}

// deflated is a line about hours cut at one stage, and why.
func deflated(from, to int32, where, why string) string {
	line := fmt.Sprintf("Deflated from %s to %s %s", hours(from), hours(to), where)
	if why = sentence(why); why != "" {
		return line + ": " + why
	}
	return line + "."
}
