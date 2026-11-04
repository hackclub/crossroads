package main

import (
	"cmp"
	"context"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/scherepi/crossroads-backend/airtable"
	"github.com/scherepi/crossroads-backend/airtablesync"
	"github.com/scherepi/crossroads-backend/api"
	"github.com/scherepi/crossroads-backend/auth"
	"github.com/scherepi/crossroads-backend/db"
	"github.com/scherepi/crossroads-backend/hackatime"
	"github.com/scherepi/crossroads-backend/hca"
)

// Only Hackatime time from here on counts toward Crossroads.
var hoursStart = time.Date(2026, time.November, 3, 0, 0, 0, 0, time.UTC)

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

	clientID, clientSecret, redirectURL := os.Getenv("HCA_CLIENT_ID"), os.Getenv("HCA_CLIENT_SECRET"), os.Getenv("HCA_REDIRECT_URL")
	if clientID == "" || clientSecret == "" || redirectURL == "" {
		log.Fatal("HCA_CLIENT_ID, HCA_CLIENT_SECRET and HCA_REDIRECT_URL have to be set")
	}
	hcaClient := hca.New(hca.Config{ClientID: clientID, ClientSecret: clientSecret, RedirectURL: redirectURL})
	a := &auth.Auth{
		Queries:       db.New(pool),
		HCA:           hcaClient,
		SecureCookies: strings.HasPrefix(redirectURL, "https://"),
	}

	stampsPerHour, err := strconv.Atoi(os.Getenv("STAMPS_PER_HOUR"))
	if err != nil || stampsPerHour <= 0 {
		log.Fatal("STAMPS_PER_HOUR has to be set to a whole number above 0")
	}
	apiRoutes := &api.API{
		Pool:          pool,
		Queries:       db.New(pool),
		Hackatime:     hackatime.New(hackatime.Config{Start: hoursStart}),
		HCA:           hcaClient,
		StampsPerHour: stampsPerHour,
	}

	// Off unless there's a token, so a dev server doesn't write test data into
	// the real base.
	if token := os.Getenv("AIRTABLE_TOKEN"); token != "" {
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
	r.Use(middleware.Logger, middleware.Recoverer, a.LoadSession)
	r.Mount("/auth", a.Routes())
	r.Mount("/api", apiRoutes.Routes())

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			http.Error(w, "database unreachable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	addr := ":" + cmp.Or(os.Getenv("PORT"), "8080")
	log.Printf("listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}
