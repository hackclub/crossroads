// Package devhackatime is a stand-in for Hackatime for local development, so
// the whole submit → review → stamps flow can be tried without real tracked
// time. It's only mounted when DEV_MODE=1 (see main.go).
//
// Time lives in memory and is lost when the server restarts. That's harmless:
// the backend never pays out more than Hackatime reports, so after a restart a
// project just has nothing new to submit until more time is added.
package devhackatime

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/scherepi/crossroads-backend/auth"
	"github.com/scherepi/crossroads-backend/respond"
)

// Everyone starts with these, so there's something to link right away.
var starter = map[string]int{
	"crossroads-game": 14 * 3600,
	"personal-site":   6 * 3600,
	"discord-bot":     2*3600 + 30*60,
}

type Fake struct {
	mu   sync.Mutex
	time map[string]map[string]int // slack ID → project → seconds
}

func New() *Fake {
	return &Fake{time: map[string]map[string]int{}}
}

func (f *Fake) projects(slackID string) map[string]int {
	p, ok := f.time[slackID]
	if !ok {
		p = map[string]int{}
		for name, s := range starter {
			p[name] = s
		}
		f.time[slackID] = p
	}
	return p
}

// Routes are mounted at /dev/hackatime.
func (f *Fake) Routes() chi.Router {
	r := chi.NewRouter()
	// What the real hackatime client calls (BaseURL is pointed here).
	r.Get("/api/v1/users/{slackID}/stats", f.stats)
	r.Get("/api/v1/users/{slackID}/heartbeats/spans", f.spans)
	// For the frontend's dev panel. Acts on the signed-in person.
	r.Get("/mine", f.mine)
	r.Post("/mine", f.add)
	return r
}

func (f *Fake) stats(w http.ResponseWriter, r *http.Request) {
	slackID := chi.URLParam(r, "slackID")
	var filter []string
	if v := r.URL.Query().Get("filter_by_project"); v != "" {
		filter = strings.Split(v, ",")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	type project struct {
		Name         string  `json:"name"`
		TotalSeconds float64 `json:"total_seconds"`
	}
	list := []project{}
	total := 0
	for name, s := range f.projects(slackID) {
		if filter != nil && !slices.Contains(filter, name) {
			continue
		}
		list = append(list, project{name, float64(s)})
		total += s
	}
	slices.SortFunc(list, func(a, b project) int { return strings.Compare(a.Name, b.Name) })
	var body struct {
		Data struct {
			TotalSeconds float64   `json:"total_seconds"`
			Projects     []project `json:"projects"`
		} `json:"data"`
		TrustFactor struct {
			TrustLevel string `json:"trust_level"`
		} `json:"trust_factor"`
	}
	body.Data.TotalSeconds = float64(total)
	body.Data.Projects = list
	body.TrustFactor.TrustLevel = "green"
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(body)
}

// spans has no record of when the time happened, so each project's time is
// one span that ends now. Only filter_by_project is honored.
func (f *Fake) spans(w http.ResponseWriter, r *http.Request) {
	slackID := chi.URLParam(r, "slackID")
	filter := strings.Split(r.URL.Query().Get("filter_by_project"), ",")
	f.mu.Lock()
	defer f.mu.Unlock()
	type span struct {
		StartTime float64 `json:"start_time"`
		EndTime   float64 `json:"end_time"`
		Duration  float64 `json:"duration"`
	}
	now := float64(time.Now().Unix())
	out := []span{}
	for name, s := range f.projects(slackID) {
		if slices.Contains(filter, name) {
			out = append(out, span{now - float64(s), now, float64(s)})
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string][]span{"spans": out})
}

type mineProject struct {
	Name    string `json:"name"`
	Seconds int    `json:"seconds"`
}

func (f *Fake) mine(w http.ResponseWriter, r *http.Request) {
	user := auth.User(r.Context())
	if user == nil || user.SlackID == nil {
		respond.Error(w, http.StatusUnauthorized, "Sign in first.")
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []mineProject{}
	for name, s := range f.projects(*user.SlackID) {
		out = append(out, mineProject{name, s})
	}
	slices.SortFunc(out, func(a, b mineProject) int { return strings.Compare(a.Name, b.Name) })
	respond.JSON(w, http.StatusOK, out)
}

// add takes {"name": "my-project", "seconds": 3600} and adds that much time to
// the signed-in person's Hackatime project, creating it if it's new.
func (f *Fake) add(w http.ResponseWriter, r *http.Request) {
	user := auth.User(r.Context())
	if user == nil || user.SlackID == nil {
		respond.Error(w, http.StatusUnauthorized, "Sign in first.")
		return
	}
	var body mineProject
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.Error(w, http.StatusBadRequest, "Send {\"name\", \"seconds\"}.")
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" || strings.Contains(body.Name, ",") || body.Seconds <= 0 || body.Seconds > 200*3600 {
		respond.Error(w, http.StatusUnprocessableEntity, "Give a project name without commas and between 1 second and 200 hours.")
		return
	}
	f.mu.Lock()
	f.projects(*user.SlackID)[body.Name] += body.Seconds
	f.mu.Unlock()
	f.mine(w, r)
}
