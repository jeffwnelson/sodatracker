package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jeffwnelson/sodatracker/internal/service"
)

// getAlertSettings godoc
// @Summary      Quiet alert settings
// @Description  When a slot, or the whole machine, registering nothing raises an alert
// @Tags         Alerts
// @Produce      json
// @Success      200  {object}  service.QuietSettings
// @Failure      500  {object}  ErrorResponse
// @Security     ApiKeyAuth || BearerAuth
// @Router       /admin/alert-settings [get]
func (h *handlers) getAlertSettings(w http.ResponseWriter, r *http.Request) {
	q, err := h.svc.GetQuietSettings(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, q)
}

// updateAlertSettings godoc
// @Summary      Update quiet alert settings
// @Description  quietSlotCans: alert when a stocked slot registers nothing while the other slots dispense this many cans (0 = off). quietMachineHours: alert when no slot registers anything for this many hours while the Pi agent is online (0 = off). quietMachineWeekdaysOnly counts only Monday–Friday hours in timezone.
// @Tags         Alerts
// @Accept       json
// @Produce      json
// @Param        body  body      service.QuietSettings  true  "Settings"
// @Success      200   {object}  service.QuietSettings
// @Failure      400   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Security     ApiKeyAuth || BearerAuth
// @Router       /admin/alert-settings [put]
func (h *handlers) updateAlertSettings(w http.ResponseWriter, r *http.Request) {
	var q service.QuietSettings
	if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	q, err := h.svc.SetQuietSettings(r.Context(), q)
	var invalid *service.ErrInvalidSettings
	switch {
	case errors.As(err, &invalid):
		writeErr(w, http.StatusBadRequest, err.Error())
	case err != nil:
		writeErr(w, http.StatusInternalServerError, err.Error())
	default:
		writeJSON(w, http.StatusOK, q)
	}
}
