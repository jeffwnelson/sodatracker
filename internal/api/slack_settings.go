package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jeffwnelson/sodatracker/internal/slackconf"
)

// getSlackSettings godoc
// @Summary      Slack settings
// @Description  Whether the alert webhook and the Socket Mode app token are set, and the /soda connection's state. The secrets are never returned.
// @Tags         Admin
// @Produce      json
// @Success      200  {object}  slackconf.Status
// @Security     ApiKeyAuth || BearerAuth
// @Router       /admin/slack [get]
func (h *handlers) getSlackSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.slack.Status())
}

// updateSlackSettings godoc
// @Summary      Update Slack settings
// @Description  Sets the incoming webhook URL (alerts) and the app-level token (/soda over Socket Mode). Omit a field to keep it, or send "" to clear it. Changes apply at once.
// @Tags         Admin
// @Accept       json
// @Produce      json
// @Param        body  body      slackconf.Update  true  "Slack settings"
// @Success      200   {object}  slackconf.Status
// @Failure      400   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Security     ApiKeyAuth || BearerAuth
// @Router       /admin/slack [put]
func (h *handlers) updateSlackSettings(w http.ResponseWriter, r *http.Request) {
	var u slackconf.Update
	if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	status, err := h.slack.Configure(r.Context(), u)
	var invalid *slackconf.InvalidError
	switch {
	case errors.As(err, &invalid):
		writeErr(w, http.StatusBadRequest, err.Error())
	case err != nil:
		writeErr(w, http.StatusInternalServerError, err.Error())
	default:
		writeJSON(w, http.StatusOK, status)
	}
}

// testSlack godoc
// @Summary      Send a Slack test message
// @Description  Posts a test message to the configured webhook
// @Tags         Admin
// @Produce      json
// @Success      200  {object}  slackconf.Status
// @Failure      400  {object}  ErrorResponse
// @Failure      502  {object}  ErrorResponse
// @Security     ApiKeyAuth || BearerAuth
// @Router       /admin/slack/test [post]
func (h *handlers) testSlack(w http.ResponseWriter, r *http.Request) {
	err := h.slack.SendTest(r.Context())
	var invalid *slackconf.InvalidError
	switch {
	case errors.As(err, &invalid):
		writeErr(w, http.StatusBadRequest, err.Error())
	case err != nil:
		writeErr(w, http.StatusBadGateway, err.Error())
	default:
		writeJSON(w, http.StatusOK, h.slack.Status())
	}
}
