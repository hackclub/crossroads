package main

import (
	"cmp"
	"context"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/scherepi/crossroads-backend/activity"
	"github.com/scherepi/crossroads-backend/airtable"
	"github.com/scherepi/crossroads-backend/airtablesync"
	"github.com/scherepi/crossroads-backend/api"
	"github.com/scherepi/crossroads-backend/audit"
	"github.com/scherepi/crossroads-backend/auth"
	"github.com/scherepi/crossroads-backend/db"
	"github.com/scherepi/crossroads-backend/devgithub"
	"github.com/scherepi/crossroads-backend/devhackatime"
	"github.com/scherepi/crossroads-backend/github"
	"github.com/scherepi/crossroads-backend/hackatime"
	"github.com/scherepi/crossroads-backend/hca"
	"github.com/scherepi/crossroads-backend/linkcheck"
	"github.com/scherepi/crossroads-backend/slackclicks"
	"github.com/scherepi/crossroads-backend/unified"
	"github.com/scherepi/crossroads-backend/uploads"
)

// Only Hackatime time from here on counts toward Crossroads: the day the RSVP
// page first went up.
var hoursStart = time.Date(2026, time.October, 30, 0, 0, 0, 0, time.UTC)

// The home page's community goal: 500 hours together by 11:59 PM Eastern on
// November 18 (EST, UTC-5). It was 200 hours by November 15 until Nov 11.
var communityGoal = api.CommunityGoal{
	Seconds: 500 * 3600,
	Ends:    time.Date(2026, time.November, 18, 23, 59, 0, 0, time.FixedZone("EST", -5*3600)),
}

// Superadmins are admins who can also read the audit log. It's exactly these
// Slack IDs, set at startup and every sign-in, and nothing in the app can
// change it: adding someone takes a commit here.
var superadmins = []string{
	"U083W5CCFDG", // Joaquin
	"U078ZJHUKFW", // Max
	"U08RVF1BAN4", // Barnav
}

// The launch sale: from launch on November 6 until the community goal closes,
// 11:59 PM Eastern on November 18. It first ran for a week, to Nov 13.
var saleEnd = communityGoal.Ends

func main() {
	ctx := context.Background()

	// An empty URL would fall back to Postgres' defaults (localhost:5432), which
	// could be some other project's database.
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL has to be set")
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		log.Fatalf("connecting to postgres: %v", err)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool); err != nil {
		log.Fatalf("migrating: %v", err)
	}
	if err := db.New(pool).SyncSuperadmins(ctx, superadmins); err != nil {
		log.Fatalf("setting superadmins: %v", err)
	}

	clientID, clientSecret, redirectURL := os.Getenv("HCA_CLIENT_ID"), os.Getenv("HCA_CLIENT_SECRET"), os.Getenv("HCA_REDIRECT_URL")
	if clientID == "" || clientSecret == "" || redirectURL == "" {
		log.Fatal("HCA_CLIENT_ID, HCA_CLIENT_SECRET and HCA_REDIRECT_URL have to be set")
	}
	hcaClient := hca.New(hca.Config{ClientID: clientID, ClientSecret: clientSecret, RedirectURL: redirectURL})
	a := &auth.Auth{
		Queries:       db.New(pool),
		HCA:           hcaClient,
		SecureCookies: strings.HasPrefix(redirectURL, "https://"),
		Superadmins:   superadmins,
	}

	// DEV_MODE=1 adds a sign-in that skips Hack Club Auth and a fake Hackatime,
	// for trying the whole flow locally. Anyone could sign in as anyone, so it
	// refuses to run anywhere but a developer's machine.
	addr := ":" + cmp.Or(os.Getenv("PORT"), "8080")
	devMode := os.Getenv("DEV_MODE") == "1"
	hackatimeConfig := hackatime.Config{Start: hoursStart}
	if devMode {
		if !runsLocally(redirectURL) {
			log.Fatal("DEV_MODE only runs locally, outside Kubernetes and with an http://localhost HCA_REDIRECT_URL; it lets anyone sign in as anyone")
		}
		hackatimeConfig.BaseURL = "http://localhost" + addr + "/dev/hackatime"
		log.Print("DEV_MODE is on: /dev/login signs in without Hack Club Auth, and Hackatime is faked")
	}

	stampsPerHour, err := strconv.Atoi(os.Getenv("STAMPS_PER_HOUR"))
	if err != nil || stampsPerHour <= 0 {
		log.Fatal("STAMPS_PER_HOUR has to be set to a whole number above 0")
	}
	// Uploaded screenshots are stored on disk and served at /uploads/. Their
	// URLs are absolute (project links have to be), built from PUBLIC_URL,
	// which defaults to the site the sign-in redirect points at.
	publicURL := os.Getenv("PUBLIC_URL")
	if publicURL == "" {
		u, err := url.Parse(redirectURL)
		if err != nil || u.Host == "" {
			log.Fatal("HCA_REDIRECT_URL has to be a full URL, or set PUBLIC_URL")
		}
		publicURL = u.Scheme + "://" + u.Host
	}
	uploadStore := &uploads.Store{
		Dir:     cmp.Or(os.Getenv("UPLOADS_DIR"), "data/uploads"),
		BaseURL: strings.TrimSuffix(publicURL, "/") + "/uploads/",
	}

	// Sidekick is off in dev mode: dev data isn't for real reviewers.
	sidekickSecret := os.Getenv("SIDEKICK_SECRET")
	if devMode && sidekickSecret != "" {
		log.Print("DEV_MODE is on, so /api/sidekick is off even though SIDEKICK_SECRET is set")
		sidekickSecret = ""
	}
	// GitHub stats and link checks for reviewers. In dev mode GitHub is faked,
	// and so are links on the example projects' made-up hosts.
	githubConfig := github.Config{Token: os.Getenv("GITHUB_TOKEN"), ProxyKey: os.Getenv("GH_PROXY_API_KEY")}
	var links linkcheck.Checker = linkcheck.New()
	// Searching the Unified YSWS DB for projects another program paid for.
	// Read-only; dev mode never touches it.
	var unifiedLookup unified.Lookup
	if devMode {
		githubConfig = github.Config{BaseURL: "http://localhost" + addr + "/dev/github"}
		links = linkcheck.Dev{Real: links}
		unifiedLookup = unified.Dev{}
	} else if token := os.Getenv("UNIFIED_AIRTABLE_TOKEN"); token != "" {
		unifiedLookup = unified.NewAirtable(token)
	} else {
		log.Print("UNIFIED_AIRTABLE_TOKEN isn't set, so reviews don't check the Unified YSWS DB for double-dipping")
	}
	apiRoutes := &api.API{
		Pool:          pool,
		Queries:       db.New(pool),
		Hackatime:     hackatime.New(hackatimeConfig),
		HCA:           hcaClient,
		StampsPerHour: stampsPerHour,
		Uploads:       uploadStore,
		HoursStart:    hoursStart,
		CommunityGoal: communityGoal,
		SaleEnd:       saleEnd,
		// Off unless set. Sidekick is Hack Club's review platform; see
		// api/sidekick.go and planning/REVIEW_FLOW.md.
		SidekickSecret: sidekickSecret,
		PublicURL:      publicURL,
		GitHub:         github.New(githubConfig),
		Links:          links,
		Unified:        unifiedLookup,
		// For example https://telescreen.hackclub.com/.../{slack_id}
		TelescreenProfileURL: os.Getenv("TELESCREEN_PROFILE_URL"),
	}

	// Price locks are refunded if the tickets sell out before they're used.
	go apiRoutes.RunPriceLockRefunds(ctx, 10*time.Minute)

	// Each project's Hackatime time per day, for the admin activity page.
	poller := &activity.Poller{Pool: pool, Queries: db.New(pool), Hackatime: apiRoutes.Hackatime, Start: hoursStart}
	go poller.Run(ctx, 30*time.Minute)

	// Off unless there's a token, so a dev server doesn't write test data into
	// the real base. Never in dev mode either: the sync ticks "Submit to
	// Unified YSWS", and test ships must never reach the Unified YSWS DB.
	if token := os.Getenv("AIRTABLE_TOKEN"); devMode && token != "" {
		log.Print("DEV_MODE is on, so the Airtable sync is off even though AIRTABLE_TOKEN is set: nothing goes to Airtable or the Unified YSWS DB")
	} else if token != "" {
		baseID := os.Getenv("AIRTABLE_BASE_ID")
		if baseID == "" {
			log.Fatal("AIRTABLE_BASE_ID has to be set along with AIRTABLE_TOKEN")
		}
		syncer := &airtablesync.Syncer{
			Queries:    db.New(pool),
			Airtable:   airtable.New(airtable.Config{Token: token, BaseID: baseID}),
			HoursStart: hoursStart,
		}
		go syncer.Run(ctx, 30*time.Second)
	} else {
		log.Print("AIRTABLE_TOKEN isn't set, so nothing is being copied to Airtable")
	}

	r := chi.NewRouter()
	// Which days each person uses the platform, for the admin activity page.
	visits := &activity.Visits{Queries: db.New(pool)}
	r.Use(middleware.Logger, middleware.Recoverer, a.LoadSession, visits.Record)
	// Every write to /auth and /api goes in the audit log (package audit).
	// The Slack bot's clicks have their own table, and /dev is only local.
	requests := &audit.Requests{Queries: db.New(pool)}
	r.Group(func(r chi.Router) {
		r.Use(requests.Record)
		r.Mount("/auth", a.Routes())
		r.Mount("/api", apiRoutes.Routes())
	})
	r.Handle("/uploads/*", http.StripPrefix("/uploads", uploadStore.Files()))
	// Clicks on the Slack bot's buttons (slackbot/). Off unless the secret it
	// shares with the bot is set.
	if secret := os.Getenv("SLACK_CLICKS_SECRET"); secret != "" {
		r.Method("POST", "/slack/clicks", &slackclicks.Handler{Queries: db.New(pool), Secret: secret})
	}
	if devMode {
		r.Get("/dev/login", a.DevLogin)
		r.Mount("/dev/github", (&devgithub.Fake{Created: hoursStart.AddDate(0, 0, -30), HoursStart: hoursStart}).Routes())
		r.Mount("/dev/hackatime", devhackatime.New(cmp.Or(os.Getenv("DEV_HACKATIME_FILE"), "data/devhackatime.json")).Routes())
		r.With(auth.RequireUser).Post("/dev/box", apiRoutes.DevGrantBox)
	}

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			http.Error(w, "database unreachable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	log.Printf("listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}

// runsLocally reports whether this looks like a developer's machine: sign-in
// redirects to plain http on a loopback address, and it isn't in a Kubernetes
// pod (Orchard runs everything in one).
func runsLocally(redirectURL string) bool {
	u, err := url.Parse(redirectURL)
	if err != nil || u.Scheme != "http" || os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		return false
	}
	if strings.EqualFold(u.Hostname(), "localhost") {
		return true
	}
	ip := net.ParseIP(u.Hostname())
	return ip != nil && ip.IsLoopback()
}
