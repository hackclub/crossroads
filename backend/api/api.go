// Package api is the JSON API the frontend uses, mounted at /api. Everything
// in it needs someone signed in.
package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/scherepi/crossroads-backend/auth"
	"github.com/scherepi/crossroads-backend/db"
	"github.com/scherepi/crossroads-backend/hackatime"
	"github.com/scherepi/crossroads-backend/hca"
	"github.com/scherepi/crossroads-backend/respond"
	"github.com/scherepi/crossroads-backend/uploads"
)

type API struct {
	Pool          *pgxpool.Pool
	Queries       *db.Queries
	Hackatime     *hackatime.Client
	HCA           *hca.Client    // for checking verification before purchases
	StampsPerHour int            // paid for each approved hour
	Uploads       *uploads.Store // project screenshots; nil turns uploading off
	HoursStart    time.Time      // when Hackatime time starts counting; activity before it isn't shown
	CommunityGoal CommunityGoal  // the home page's "Repair Bridge" goal
	SaleEnd       time.Time      // until then the shop is on sale (sale.go); zero for no sale
	// The secret Sidekick sends to POST /api/sidekick (sidekick.go). Empty
	// turns the endpoint off.
	SidekickSecret string
	PublicURL      string // the site, for links Sidekick shows reviewers
}

func (a *API) Routes() chi.Router {
	r := chi.NewRouter()
	// Sidekick signs its requests with a shared secret instead of a session.
	if a.SidekickSecret != "" {
		r.Post("/sidekick", a.sidekick)
	}
	r.Group(func(r chi.Router) { a.signedInRoutes(r) })
	return r
}

func (a *API) signedInRoutes(r chi.Router) {
	r.Use(auth.RequireUser)
	r.Get("/me", a.me)
	r.Get("/hackatime/projects", a.listHackatimeProjects)
	r.Get("/community", a.community)
	if a.Uploads != nil {
		r.Post("/uploads", a.Uploads.Upload)
	}
	r.Route("/projects", func(r chi.Router) {
		r.Get("/", a.listProjects)
		r.Post("/", a.createProject)
		r.Route("/{projectID}", func(r chi.Router) {
			r.Get("/", a.getProject)
			r.Patch("/", a.updateProject)
			r.Delete("/", a.deleteProject)
			r.Post("/submissions", a.submit)
		})
	})
	r.Get("/boxes", a.listBoxes)
	r.Post("/boxes/{boxID}/open", a.openBox)
	r.Get("/box-types", a.listBoxTypes)
	r.Get("/special-stamps", a.listSpecialStamps)
	r.Post("/special-stamps/{stampID}/placement", a.placeStamp)
	r.Get("/shop/items", a.listShopItems)
	r.Post("/shop/items/{itemID}/orders", a.buy)
	r.Get("/orders", a.listOrders)
	r.Route("/review", func(r chi.Router) {
		r.Use(auth.RequireRole(db.UserRoleReviewer, db.UserRoleAdmin))
		r.Get("/queue", a.reviewQueue)
		r.Post("/next", a.claimNext)
		r.Route("/submissions/{submissionID}", func(r chi.Router) {
			r.Get("/", a.getSubmissionForReview)
			r.Post("/", a.reviewSubmission)
			r.Post("/claim", a.claimSubmission)
			r.Delete("/claim", a.releaseClaim)
			r.Post("/skip", a.skipSubmission)
			r.Post("/comments", a.commentOnSubmission)
		})
		r.Put("/users/{userID}/note", a.setReviewerNote)
		r.Get("/templates", a.listFeedbackTemplates)
		r.Post("/templates", a.createFeedbackTemplate)
		r.Patch("/templates/{templateID}", a.updateFeedbackTemplate)
		r.Delete("/templates/{templateID}", a.deleteFeedbackTemplate)
	})
	r.Route("/admin", func(r chi.Router) {
		r.Use(auth.RequireRole(db.UserRoleAdmin))
		r.Get("/activity", a.adminActivity)
		r.Get("/users", a.adminListUsers)
		r.Patch("/users/{userID}", a.adminUpdateUser)
		r.Get("/review-settings", a.adminGetReviewSettings)
		r.Patch("/review-settings", a.adminUpdateReviewSettings)
		r.Post("/users/{userID}/boxes", a.adminGrantBox)
		r.Post("/users/{userID}/special-stamps", a.adminGrantSpecialStamp)
		r.Post("/users/{userID}/stamp-adjustments", a.adminAdjustStamps)
		r.Get("/special-stamp-types", a.adminListSpecialStampTypes)
		r.Post("/special-stamp-types", a.adminCreateSpecialStampType)
		r.Patch("/special-stamp-types/{typeID}", a.adminUpdateSpecialStampType)
		r.Get("/box-types", a.adminListBoxTypes)
		r.Post("/box-types", a.adminCreateBoxType)
		r.Patch("/box-types/{boxTypeID}", a.adminUpdateBoxType)
		r.Put("/box-types/{boxTypeID}/drops", a.adminSetBoxDrops)
		r.Get("/milestones", a.adminListMilestones)
		r.Post("/milestones", a.adminCreateMilestone)
		r.Get("/shop-items", a.adminListShopItems)
		r.Post("/shop-items", a.adminCreateShopItem)
		r.Patch("/shop-items/{itemID}", a.adminUpdateShopItem)
		r.Get("/orders", a.adminListOrders)
		r.Patch("/orders/{orderID}", a.adminUpdateOrder)
	})
}

// me is auth.Me plus the stamps paid per approved hour, so prices can be
// shown in hours.
func (a *API) me(w http.ResponseWriter, r *http.Request) {
	respond.JSON(w, http.StatusOK, struct {
		auth.MeJSON
		StampsPerHour int `json:"stamps_per_hour"`
	}{auth.ToMeJSON(auth.User(r.Context())), a.StampsPerHour})
}

// decode reads a JSON request body into v and rejects fields v doesn't have.
// An empty body is the same as {}. If it returns false, it has already
// responded.
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(v); err != nil && !errors.Is(err, io.EOF) {
		respond.Error(w, http.StatusBadRequest, "Couldn't read that request: "+err.Error())
		return false
	}
	return true
}

// idParam reads an ID from the URL. If it returns false, it has already
// responded.
func idParam(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, name), 10, 64)
	if err != nil {
		respond.Error(w, http.StatusNotFound, "That doesn't exist.")
		return 0, false
	}
	return id, true
}
