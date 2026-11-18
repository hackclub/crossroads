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
	"github.com/scherepi/crossroads-backend/github"
	"github.com/scherepi/crossroads-backend/hackatime"
	"github.com/scherepi/crossroads-backend/hca"
	"github.com/scherepi/crossroads-backend/linkcheck"
	"github.com/scherepi/crossroads-backend/respond"
	"github.com/scherepi/crossroads-backend/slack"
	"github.com/scherepi/crossroads-backend/unified"
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
	// The token scripts/prereview.mjs sends to /api/prereviews (prereview.go).
	// Empty turns those off; reviewers can still use the script signed in.
	PrereviewToken string
	// For the review checks (checks.go). Any of them can be nil, and the
	// checks that need it say they couldn't run.
	GitHub  *github.Client
	Links   linkcheck.Checker
	Unified unified.Lookup // reads the Unified YSWS DB; nil when no token is set
	// Announcements (announcements.go) post to AnnounceChannel with the Slack
	// app's bot. A nil Slack still lets superadmins write drafts.
	Slack           *slack.Client
	AnnounceChannel string
}

func (a *API) Routes() chi.Router {
	r := chi.NewRouter()
	// Sidekick signs its requests with a shared secret instead of a session.
	if a.SidekickSecret != "" {
		r.Post("/sidekick", a.sidekick)
	}
	if a.PrereviewToken != "" {
		r.With(a.prereviewToken).Get("/prereviews", a.prereviewBatch)
		r.With(a.prereviewToken).Post("/prereviews", a.savePrereviews)
	}
	r.Group(func(r chi.Router) { a.signedInRoutes(r) })
	return r
}

func (a *API) signedInRoutes(r chi.Router) {
	r.Use(auth.RequireUser)
	r.Get("/me", a.me)
	r.Patch("/me", a.updateMe)
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
	r.Post("/special-stamps/claims", a.claimSpecialStamp)
	r.Post("/special-stamps/{stampID}/placement", a.placeStamp)
	r.Post("/special-stamps/{stampID}/seen", a.seeSpecialStamp)
	r.Get("/shop/items", a.listShopItems)
	r.Get("/emporium/items", a.listEmporiumItems)
	r.Post("/shop/items/{itemID}/orders", a.buy)
	r.Get("/orders", a.listOrders)
	r.Route("/review", func(r chi.Router) {
		r.Use(auth.RequireRole(db.UserRoleReviewer, db.UserRoleAdmin))
		r.Get("/queue", a.reviewQueue)
		r.Get("/stats", a.reviewStats)
		r.Post("/next", a.claimNext)
		r.Route("/submissions/{submissionID}", func(r chi.Router) {
			r.Get("/", a.getSubmissionForReview)
			r.Post("/", a.reviewSubmission)
			r.Post("/claim", a.claimSubmission)
			r.Delete("/claim", a.releaseClaim)
			r.Post("/skip", a.skipSubmission)
			r.Post("/comments", a.commentOnSubmission)
			r.Post("/reopen", a.reopenSubmission)
		})
		r.Get("/past", a.pastReviews)
		r.Get("/prereviews/prompt", a.prereviewPrompt)
		r.Get("/prereviews/batch", a.prereviewBatch)
		r.Post("/prereviews", a.savePrereviews)
		r.Put("/projects/{projectID}/resubmit-block", a.setResubmitBlock)
		r.Patch("/projects/{projectID}", a.reviewerEditProject)
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
		r.Get("/reviewer-stats", a.adminReviewerStats)
		r.Get("/reviews", a.adminReviewLog)
		r.Patch("/review-settings", a.adminUpdateReviewSettings)
		r.Post("/users/{userID}/boxes", a.adminGrantBox)
		r.Post("/users/{userID}/special-stamps", a.adminGrantSpecialStamp)
		r.Post("/users/{userID}/stamp-adjustments", a.adminAdjustStamps)
		r.Get("/role-invites", a.adminListRoleInvites)
		r.Put("/roles-by-email", a.adminSetRoleByEmail)
		r.Delete("/role-invites/{inviteID}", a.adminDeleteRoleInvite)
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
	r.Route("/superadmin", func(r chi.Router) {
		r.Use(auth.RequireRole(db.UserRoleSuperadmin))
		r.Get("/audit-log", a.superadminAuditLog)
		r.Get("/announcements", a.listAnnouncements)
		r.Post("/announcements", a.createAnnouncement)
		r.Post("/announcements/test", a.testAnnouncement)
		r.Route("/announcements/{announcementID}", func(r chi.Router) {
			r.Get("/", a.getAnnouncement)
			r.Patch("/", a.updateAnnouncement)
			r.Delete("/", a.deleteAnnouncement)
			r.Post("/post", a.postAnnouncement)
		})
	})
}

// me is auth.Me plus the stamps paid per approved hour, so prices can be
// shown in hours.
func (a *API) me(w http.ResponseWriter, r *http.Request) {
	user := auth.User(r.Context())
	// Whether the review pages show AI pre-reviews (prereview.go).
	canPrereview := false
	if p := permissionsOf(user); p.Review || p.FraudReview || p.HQReview {
		settings, err := a.Queries.GetReviewSettings(r.Context())
		if err != nil {
			respond.ServerError(w, r, err)
			return
		}
		canPrereview = canSeePrereviews(user, settings)
	}
	respond.JSON(w, http.StatusOK, struct {
		auth.MeJSON
		StampsPerHour int  `json:"stamps_per_hour"`
		CanPrereview  bool `json:"can_prereview"`
	}{auth.ToMeJSON(user), a.StampsPerHour, canPrereview})
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
