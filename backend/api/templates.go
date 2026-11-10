package api

// Feedback templates: snippets reviewers drop into feedback. Everyone who
// reviews shares them and can change them.

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/scherepi/crossroads-backend/auth"
	"github.com/scherepi/crossroads-backend/db"
	"github.com/scherepi/crossroads-backend/respond"
)

type feedbackTemplateJSON struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	CreatedBy *int64    `json:"created_by"`
	UpdatedAt time.Time `json:"updated_at"`
}

func toFeedbackTemplateJSON(t db.FeedbackTemplate) feedbackTemplateJSON {
	return feedbackTemplateJSON{ID: t.ID, Title: t.Title, Body: t.Body, CreatedBy: t.CreatedBy, UpdatedAt: t.UpdatedAt}
}

type templateFields struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

func (f *templateFields) problem() string {
	f.Title, f.Body = strings.TrimSpace(f.Title), strings.TrimSpace(f.Body)
	switch {
	case f.Title == "" || f.Body == "":
		return "A template needs a title and some text."
	case len(f.Title) > 100:
		return "Keep the title under 100 characters."
	case len(f.Body) > 5000:
		return "Keep the text under 5,000 characters."
	}
	return ""
}

func (a *API) listFeedbackTemplates(w http.ResponseWriter, r *http.Request) {
	templates, err := a.Queries.ListFeedbackTemplates(r.Context())
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	out := []feedbackTemplateJSON{}
	for _, t := range templates {
		out = append(out, toFeedbackTemplateJSON(t))
	}
	respond.JSON(w, http.StatusOK, out)
}

func (a *API) createFeedbackTemplate(w http.ResponseWriter, r *http.Request) {
	var body templateFields
	if !decode(w, r, &body) {
		return
	}
	if p := body.problem(); p != "" {
		respond.Error(w, http.StatusUnprocessableEntity, p)
		return
	}
	t, err := a.Queries.CreateFeedbackTemplate(r.Context(), db.CreateFeedbackTemplateParams{
		Title: body.Title, Body: body.Body, CreatedBy: &auth.User(r.Context()).ID,
	})
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toFeedbackTemplateJSON(t))
}

func (a *API) updateFeedbackTemplate(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "templateID")
	if !ok {
		return
	}
	var body templateFields
	if !decode(w, r, &body) {
		return
	}
	if p := body.problem(); p != "" {
		respond.Error(w, http.StatusUnprocessableEntity, p)
		return
	}
	t, err := a.Queries.UpdateFeedbackTemplate(r.Context(), db.UpdateFeedbackTemplateParams{ID: id, Title: body.Title, Body: body.Body})
	if errors.Is(err, pgx.ErrNoRows) {
		respond.Error(w, http.StatusNotFound, "That template doesn't exist.")
		return
	}
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	respond.JSON(w, http.StatusOK, toFeedbackTemplateJSON(t))
}

func (a *API) deleteFeedbackTemplate(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "templateID")
	if !ok {
		return
	}
	n, err := a.Queries.DeleteFeedbackTemplate(r.Context(), id)
	if err != nil {
		respond.ServerError(w, r, err)
		return
	}
	if n == 0 {
		respond.Error(w, http.StatusNotFound, "That template doesn't exist.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
