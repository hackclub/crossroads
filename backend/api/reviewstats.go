package api

// Review counts: the reviewers' dashboard at the top of /review, the Review
// stats tab, and the admin's per-person counts that reviewers are paid by. Both count the same
// thing, a decision on a ship (see db/queries/reviewstats.sql), so the
// numbers agree wherever they're shown.

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata" // periods are in New York time, whatever the server has installed

	"github.com/scherepi/crossroads-backend/auth"
	"github.com/scherepi/crossroads-backend/db"
	"github.com/scherepi/crossroads-backend/respond"
)

// reviewZone is where days and weeks start and end: Eastern time, like the
// community goal's deadline.
var reviewZone = mustZone("America/New_York")

func mustZone(name string) *time.Location {
	zone, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return zone
}

// reviewWeek is the week now is in: Monday 12:00 AM to the next Monday
// 12:00 AM, Eastern time.
func reviewWeek(now time.Time) (time.Time, time.Time) {
	local := now.In(reviewZone)
	daysSinceMonday := (int(local.Weekday()) + 6) % 7
	start := time.Date(local.Year(), local.Month(), local.Day()-daysSinceMonday, 0, 0, 0, 0, reviewZone)
	return start, start.AddDate(0, 0, 7)
}

// reviewsSince is far enough back to mean "all time".
var reviewsSince = time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)

type leaderJSON struct {
	ID                   int64     `json:"id"`
	Name                 string    `json:"name"`
	Reviews              int64     `json:"reviews"`
	FirstReviews         int64     `json:"first_reviews"`
	FraudChecks          int64     `json:"fraud_checks"`
	HQDecisions          int64     `json:"hq_decisions"`
	SecondsFirstReviewed int64     `json:"seconds_first_reviewed"`
	LastReviewAt         time.Time `json:"last_review_at"`
}

func leadersFrom(rows []db.ReviewerStatsRow) []leaderJSON {
	out := []leaderJSON{}
	for _, r := range rows {
		out = append(out, leaderJSON{ID: r.ID, Name: r.Name, Reviews: r.Reviews, FirstReviews: r.FirstReviews,
			FraudChecks: r.FraudChecks, HQDecisions: r.HqDecisions, SecondsFirstReviewed: r.SecondsFirstReviewed,
			LastReviewAt: r.LastReviewAt})
	}
	return out
}

// reviewStats is GET /review/stats: this week's leaderboard and the all-time
// one, and how the queue is doing. Names only: no emails or Slack IDs.
func (a *API) reviewStats(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	weekStart, weekEnd := reviewWeek(now)
	week, err := a.Queries.ReviewerStats(r.Context(), db.ReviewerStatsParams{Starts: weekStart, Ends: weekEnd})
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	allTime, err := a.Queries.ReviewerStats(r.Context(), db.ReviewerStatsParams{Starts: reviewsSince, Ends: now.Add(time.Hour)})
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	pace, err := a.Queries.ReviewPace(r.Context(), weekStart)
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	perDay, err := a.Queries.ReviewsPerDay(r.Context(), db.ReviewsPerDayParams{Starts: weekStart, Ends: weekEnd})
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	// Every day of the week, Monday first, with none for days without any.
	type dayJSON struct {
		Day     string `json:"day"` // YYYY-MM-DD, Eastern
		Reviews int64  `json:"reviews"`
	}
	counts := map[string]int64{}
	for _, row := range perDay {
		counts[row.Day] = row.Reviews
	}
	days := []dayJSON{}
	for i := 0; i < 7; i++ {
		day := weekStart.AddDate(0, 0, i).Format(time.DateOnly)
		days = append(days, dayJSON{day, counts[day]})
	}
	var reviewedThisWeek int64
	for _, row := range week {
		reviewedThisWeek += row.Reviews
	}
	type paceJSON struct {
		Reviewed          int64    `json:"reviewed"`            // reviews this week, by everyone
		Shipped           int64    `json:"shipped"`             // ships submitted this week
		SecondsApproved   int64    `json:"seconds_approved"`    // hours signed off this week
		Waiting           int64    `json:"waiting"`             // ships waiting for a first review now
		OldestWaitSeconds *int64   `json:"oldest_wait_seconds"` // how long the oldest has waited; null with none waiting
		MedianWaitSeconds *float64 `json:"median_wait_seconds"` // median wait for a first review, last 7 days; null with none
	}
	p := paceJSON{Reviewed: reviewedThisWeek, Shipped: pace.Shipped, SecondsApproved: pace.SecondsApproved, Waiting: pace.Waiting}
	if pace.Waiting > 0 {
		p.OldestWaitSeconds = &pace.OldestWaitSeconds
	}
	if pace.MedianWaitSeconds >= 0 {
		p.MedianWaitSeconds = &pace.MedianWaitSeconds
	}
	respond.JSON(w, http.StatusOK, struct {
		WeekStart time.Time    `json:"week_start"`
		WeekEnd   time.Time    `json:"week_end"`
		Today     string       `json:"today"` // YYYY-MM-DD, Eastern
		Days      []dayJSON    `json:"days"`  // reviews by everyone each day this week
		Week      []leaderJSON `json:"week"`
		AllTime   []leaderJSON `json:"all_time"`
		Pace      paceJSON     `json:"pace"`
	}{weekStart, weekEnd, now.In(reviewZone).Format(time.DateOnly), days, leadersFrom(week), leadersFrom(allTime), p})
}

// reviewPeriod reads ?from=YYYY-MM-DD&to=YYYY-MM-DD, both days included, in
// Eastern time. Without them it's this week so far. If it returns false, it
// has already responded.
func reviewPeriod(w http.ResponseWriter, r *http.Request) (from, to time.Time, ok bool) {
	q := r.URL.Query()
	if q.Get("from") == "" && q.Get("to") == "" {
		start, end := reviewWeek(time.Now())
		return start, end, true
	}
	from, err1 := time.ParseInLocation(time.DateOnly, q.Get("from"), reviewZone)
	last, err2 := time.ParseInLocation(time.DateOnly, q.Get("to"), reviewZone)
	if err1 != nil || err2 != nil {
		respond.Error(w, http.StatusUnprocessableEntity, "from and to have to be dates like 2026-11-09.")
		return from, to, false
	}
	if last.Before(from) {
		respond.Error(w, http.StatusUnprocessableEntity, "to has to be on or after from.")
		return from, to, false
	}
	return from, last.AddDate(0, 0, 1), true
}

type reviewerPayJSON struct {
	ID                   int64     `json:"id"`
	Name                 string    `json:"name"`
	Email                string    `json:"email,omitempty"`    // admins only
	SlackID              *string   `json:"slack_id,omitempty"` // admins only
	Reviews              int64     `json:"reviews"`
	FirstReviews         int64     `json:"first_reviews"`
	FirstApprovals       int64     `json:"first_approvals"`
	FirstRejections      int64     `json:"first_rejections"`
	FraudChecks          int64     `json:"fraud_checks"`
	HQDecisions          int64     `json:"hq_decisions"`
	LaterSentBack        int64     `json:"later_sent_back"`
	SecondsFirstReviewed int64     `json:"seconds_first_reviewed"`
	LastReviewAt         time.Time `json:"last_review_at"`
}

// adminReviewerStats is GET /admin/reviewer-stats?from=&to=[&format=csv]:
// how many reviews each person made in a period, by stage. This is what
// reviewers are paid by.
func (a *API) adminReviewerStats(w http.ResponseWriter, r *http.Request) {
	a.reviewerCounts(w, r, true)
}

// reviewReviewerStats is GET /review/reviewer-stats?from=&to=, the review
// page's Review stats tab: the same counts for every reviewer, but only
// admins get emails, Slack IDs and the CSV.
func (a *API) reviewReviewerStats(w http.ResponseWriter, r *http.Request) {
	a.reviewerCounts(w, r, auth.User(r.Context()).Role.IsAdmin())
}

func (a *API) reviewerCounts(w http.ResponseWriter, r *http.Request, admin bool) {
	from, to, ok := reviewPeriod(w, r)
	if !ok {
		return
	}
	rows, err := a.Queries.ReviewerStats(r.Context(), db.ReviewerStatsParams{Starts: from, Ends: to})
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	last := to.AddDate(0, 0, -1)
	if r.URL.Query().Get("format") == "csv" {
		if !admin {
			respond.Error(w, http.StatusForbidden, "Only admins can download everyone's counts.")
			return
		}
		out := startCSV(w, fmt.Sprintf("crossroads-reviewers-%s-to-%s.csv", from.Format(time.DateOnly), last.Format(time.DateOnly)))
		out.Write([]string{"Reviewer", "Email", "Slack ID", "Reviews", "First reviews", "First-review approvals",
			"First-review rejections", "Fraud checks", "HQ decisions", "First reviews later sent back",
			"Hours first-reviewed", "Last review (Eastern)"})
		for _, row := range rows {
			out.Write(cells(row.Name, row.Email, deref(row.SlackID), row.Reviews, row.FirstReviews, row.FirstApprovals,
				row.FirstRejections, row.FraudChecks, row.HqDecisions, row.LaterSentBack,
				strconv.FormatFloat(float64(row.SecondsFirstReviewed)/3600, 'f', 2, 64),
				row.LastReviewAt.In(reviewZone).Format("2006-01-02 15:04")))
		}
		out.Flush()
		return
	}
	people := []reviewerPayJSON{}
	var total int64
	for _, row := range rows {
		total += row.Reviews
		person := reviewerPayJSON{ID: row.ID, Name: row.Name,
			Reviews: row.Reviews, FirstReviews: row.FirstReviews, FirstApprovals: row.FirstApprovals,
			FirstRejections: row.FirstRejections, FraudChecks: row.FraudChecks, HQDecisions: row.HqDecisions,
			LaterSentBack: row.LaterSentBack, SecondsFirstReviewed: row.SecondsFirstReviewed, LastReviewAt: row.LastReviewAt}
		if admin {
			person.Email, person.SlackID = row.Email, row.SlackID
		}
		people = append(people, person)
	}
	respond.JSON(w, http.StatusOK, struct {
		From      string            `json:"from"` // the first day, Eastern
		To        string            `json:"to"`   // the last day, included
		Reviews   int64             `json:"reviews"`
		Reviewers []reviewerPayJSON `json:"reviewers"`
	}{from.Format(time.DateOnly), last.Format(time.DateOnly), total, people})
}

// adminReviewLog is GET /admin/reviews?from=&to=[&reviewer=ID][&format=csv]:
// every review in a period, oldest first: the line items behind the counts.
func (a *API) adminReviewLog(w http.ResponseWriter, r *http.Request) {
	a.reviewLog(w, r, nil)
}

// reviewReviewLog is GET /review/reviews?from=&to=[&reviewer=ID][&format=csv]
// for the Review stats tab: admins see anyone's reviews, everyone else only
// their own.
func (a *API) reviewReviewLog(w http.ResponseWriter, r *http.Request) {
	me := auth.User(r.Context())
	if me.Role.IsAdmin() {
		a.reviewLog(w, r, nil)
		return
	}
	if s := r.URL.Query().Get("reviewer"); s != "" && s != strconv.FormatInt(me.ID, 10) {
		respond.Error(w, http.StatusForbidden, "You can only list your own reviews.")
		return
	}
	a.reviewLog(w, r, &me.ID)
}

// reviewLog lists the reviews in a period: only's, when set, or ?reviewer='s.
func (a *API) reviewLog(w http.ResponseWriter, r *http.Request, only *int64) {
	from, to, ok := reviewPeriod(w, r)
	if !ok {
		return
	}
	params := db.ListReviewsParams{Starts: from, Ends: to, ReviewerID: only}
	if s := r.URL.Query().Get("reviewer"); s != "" && only == nil {
		id, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			respond.Error(w, http.StatusUnprocessableEntity, "reviewer has to be a user ID.")
			return
		}
		params.ReviewerID = &id
	}
	rows, err := a.Queries.ListReviews(r.Context(), params)
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	last := to.AddDate(0, 0, -1)
	if r.URL.Query().Get("format") == "csv" {
		out := startCSV(w, fmt.Sprintf("crossroads-reviews-%s-to-%s.csv", from.Format(time.DateOnly), last.Format(time.DateOnly)))
		out.Write([]string{"When (Eastern)", "Reviewer", "Reviewer ID", "Stage", "Decision", "Ship", "Project", "Maker",
			"Hours claimed", "Hours approved", "Later sent back"})
		for _, row := range rows {
			approved := ""
			if row.Seconds != nil {
				approved = strconv.FormatFloat(float64(*row.Seconds)/3600, 'f', 2, 64)
			}
			out.Write(cells(row.CreatedAt.In(reviewZone).Format("2006-01-02 15:04"), row.ReviewerName, row.ReviewerID,
				reviewStageLabel(row.Stage), decisionLabel(row.Kind), row.SubmissionID, row.ProjectName, row.MakerName,
				strconv.FormatFloat(float64(row.NewSeconds)/3600, 'f', 2, 64), approved, yesNo(row.LaterSentBack)))
		}
		out.Flush()
		return
	}
	type reviewJSON struct {
		ID            int64     `json:"id"`
		CreatedAt     time.Time `json:"created_at"`
		ReviewerID    int64     `json:"reviewer_id"`
		ReviewerName  string    `json:"reviewer_name"`
		Stage         string    `json:"stage"`
		Kind          string    `json:"kind"`
		SubmissionID  int64     `json:"submission_id"`
		ProjectName   string    `json:"project_name"`
		MakerName     string    `json:"maker_name"`
		NewSeconds    int32     `json:"new_seconds"`
		Seconds       *int32    `json:"seconds"`
		LaterSentBack bool      `json:"later_sent_back"`
	}
	out := []reviewJSON{}
	for _, row := range rows {
		out = append(out, reviewJSON{ID: row.ID, CreatedAt: row.CreatedAt, ReviewerID: row.ReviewerID, ReviewerName: row.ReviewerName,
			Stage: string(row.Stage), Kind: string(row.Kind), SubmissionID: row.SubmissionID, ProjectName: row.ProjectName,
			MakerName: row.MakerName, NewSeconds: row.NewSeconds, Seconds: row.Seconds, LaterSentBack: row.LaterSentBack})
	}
	respond.JSON(w, http.StatusOK, out)
}

func reviewStageLabel(stage db.SubmissionStatus) string {
	switch stage {
	case db.SubmissionStatusPending:
		return "First review"
	case db.SubmissionStatusFraudCheck:
		return "Fraud check"
	case db.SubmissionStatusAwaitingHq:
		return "HQ sign-off"
	}
	return string(stage)
}

func decisionLabel(kind db.ShipEventKind) string {
	switch kind {
	case db.ShipEventKindApproval:
		return "Approved"
	case db.ShipEventKindFraudPass:
		return "Passed"
	case db.ShipEventKindFinalApproval:
		return "Signed off"
	case db.ShipEventKindSentBack:
		return "Sent back"
	case db.ShipEventKindRejection:
		return "Rejected"
	}
	return string(kind)
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return ""
}

func startCSV(w http.ResponseWriter, filename string) *csv.Writer {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.Header().Set("Cache-Control", "no-store")
	return csv.NewWriter(w)
}

// cells turns values into CSV cells. Project and people's names are typed by
// participants, so a cell a spreadsheet would read as a formula (=, +, -, @,
// a tab or a carriage return first) gets a leading apostrophe.
func cells(values ...any) []string {
	out := make([]string, len(values))
	for i, v := range values {
		s := fmt.Sprint(v)
		if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
			s = "'" + s
		}
		out[i] = s
	}
	return out
}

// reviewInsights is GET /review/insights?from=&to= (Eastern days, this week
// without them), the review page's Review stats tab: what's waiting now,
// the decisions and rejection rates in the period, where the ships shipped
// in it got to, how long reviews take, and reviews each day.
func (a *API) reviewInsights(w http.ResponseWriter, r *http.Request) {
	from, to, ok := reviewPeriod(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	period := struct{ Starts, Ends time.Time }{from, to}
	waiting, err := a.Queries.WaitingByStage(ctx)
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	decisions, err := a.Queries.DecisionsByStage(ctx, db.DecisionsByStageParams(period))
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	funnel, err := a.Queries.ShipFunnel(ctx, db.ShipFunnelParams(period))
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	times, err := a.Queries.ReviewTimes(ctx, db.ReviewTimesParams(period))
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	perDay, err := a.Queries.ReviewsPerDay(ctx, db.ReviewsPerDayParams(period))
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}

	// Every stage, waiting or not, in the order a ship goes through them.
	type waitJSON struct {
		Stage             string   `json:"stage"` // review, fraud, hq: the queue tabs' keys
		Waiting           int64    `json:"waiting"`
		MedianWaitSeconds *float64 `json:"median_wait_seconds"`
		OldestID          *int64   `json:"oldest_id"`
		OldestProject     *string  `json:"oldest_project"`
		OldestWaitSeconds *int64   `json:"oldest_wait_seconds"`
	}
	stages := []waitJSON{}
	for _, s := range []struct{ status, key string }{{"pending", "review"}, {"fraud_check", "fraud"}, {"awaiting_hq", "hq"}} {
		out := waitJSON{Stage: s.key}
		for _, row := range waiting {
			if row.Stage == s.status {
				out = waitJSON{s.key, row.Waiting, &row.MedianWaitSeconds, &row.OldestID, &row.OldestProject, &row.OldestWaitSeconds}
			}
		}
		stages = append(stages, out)
	}

	type stageJSON struct {
		Stage    string `json:"stage"`
		Passed   int64  `json:"passed"` // approved, passed or signed off
		Rejected int64  `json:"rejected"`
		SentBack int64  `json:"sent_back"`
	}
	type dayJSON struct {
		Day     string `json:"day"`
		Reviews int64  `json:"reviews"`
	}
	counts := map[string]int64{}
	for _, row := range perDay {
		counts[row.Day] = row.Reviews
	}
	days := []dayJSON{}
	today := time.Now().In(reviewZone).Format(time.DateOnly)
	for d := from; d.Before(to); d = d.AddDate(0, 0, 1) {
		day := d.Format(time.DateOnly)
		if day > today {
			break
		}
		days = append(days, dayJSON{day, counts[day]})
	}
	// The same fields as db.ShipFunnelRow, so it converts.
	type funnelJSON struct {
		Shipped           int64 `json:"shipped"`
		FirstReviewed     int64 `json:"first_reviewed"`
		PassedFirstReview int64 `json:"passed_first_review"`
		FraudChecked      int64 `json:"fraud_checked"`
		ReachedHq         int64 `json:"reached_hq"`
		SignedOff         int64 `json:"signed_off"`
		Rejected          int64 `json:"rejected"`
		InReview          int64 `json:"in_review"`
	}
	median := func(seconds float64) *float64 {
		if seconds < 0 {
			return nil
		}
		return &seconds
	}

	respond.JSON(w, http.StatusOK, struct {
		From      string      `json:"from"`
		To        string      `json:"to"`
		Waiting   []waitJSON  `json:"waiting"`
		Decisions []stageJSON `json:"decisions"`
		Reviewers int64       `json:"reviewers"`
		// Hours HQ signed off in the period.
		SecondsSignedOff int64 `json:"seconds_signed_off"`
		// Ships shipped in the period, counted at every step they reached.
		Funnel                   funnelJSON `json:"funnel"`
		MedianFirstReviewSeconds *float64   `json:"median_first_review_seconds"`
		MedianSignOffSeconds     *float64   `json:"median_sign_off_seconds"`
		Days                     []dayJSON  `json:"days"`
	}{
		From: from.Format(time.DateOnly), To: to.AddDate(0, 0, -1).Format(time.DateOnly),
		Waiting: stages,
		Decisions: []stageJSON{
			{"review", decisions.FirstApprovals, decisions.FirstRejections, 0},
			{"fraud", decisions.FraudPasses, decisions.FraudRejections, decisions.FraudSentBack},
			{"hq", decisions.HqSignOffs, decisions.HqRejections, decisions.HqSentBack},
		},
		Reviewers: decisions.Reviewers, SecondsSignedOff: decisions.SecondsSignedOff,
		Funnel:                   funnelJSON(funnel),
		MedianFirstReviewSeconds: median(times.MedianFirstReviewSeconds),
		MedianSignOffSeconds:     median(times.MedianSignOffSeconds),
		Days:                     days,
	})
}
