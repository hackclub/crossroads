// Package slackclicks records clicks on the buttons of messages posted with
// the Slack bot (slackbot/ at the repo root). Slack tells the bot about every
// click over Socket Mode, and the bot posts it here with a secret the two
// share. It's mounted outside /api, which needs someone signed in, and the
// public ingress doesn't route to it: the bot calls the backend from inside
// the cluster.
package slackclicks

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/scherepi/crossroads-backend/db"
	"github.com/scherepi/crossroads-backend/respond"
)

type Handler struct {
	Queries *db.Queries
	Secret  string // the bot sends it as "Authorization: Bearer <secret>"
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok || subtle.ConstantTimeCompare([]byte(token), []byte(h.Secret)) != 1 {
		respond.Error(w, http.StatusUnauthorized, "That's not the Slack bot's secret.")
		return
	}
	var body struct {
		SlackID   string  `json:"slack_id"`
		Button    string  `json:"button"` // the button's action_id
		ChannelID *string `json:"channel_id"`
		MessageTS *string `json:"message_ts"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		respond.Error(w, http.StatusBadRequest, "Couldn't read that request: "+err.Error())
		return
	}
	if body.SlackID == "" || body.Button == "" {
		respond.Error(w, http.StatusUnprocessableEntity, "slack_id and button are needed.")
		return
	}
	if err := h.Queries.RecordSlackClick(r.Context(), db.RecordSlackClickParams{
		SlackID: body.SlackID, Button: body.Button, ChannelID: body.ChannelID, MessageTs: body.MessageTS,
	}); err != nil {
		respond.ServerError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
