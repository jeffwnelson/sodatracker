package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jeffwnelson/sodatracker/internal/backup"
	"github.com/jeffwnelson/sodatracker/internal/buildinfo"
	"github.com/jeffwnelson/sodatracker/internal/domain"
	"github.com/jeffwnelson/sodatracker/internal/export"
	"github.com/jeffwnelson/sodatracker/internal/service"
	"github.com/jeffwnelson/sodatracker/internal/slackcmd"
	"github.com/jeffwnelson/sodatracker/internal/store"
)

type handlers struct {
	store              store.Store
	svc                *service.Service
	jwtSecret          string
	slackSigningSecret string
	slackCmds          *slackcmd.Commands
	broker             *broker
	streamsDone        <-chan struct{}
	backups            *backup.Manager
}

// health godoc
// @Summary      Health check
// @Description  Returns API health status and the running release version
// @Tags         System
// @Produce      json
// @Success      200  {object}  HealthResponse
// @Router       /health [get]
func (h *handlers) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, HealthResponse{Status: "ok", Version: buildinfo.Version})
}

// --- Machines ---

// listMachines godoc
// @Summary      List all machines
// @Description  Returns all registered soda machines
// @Tags         Machines
// @Produce      json
// @Success      200  {array}   domain.Machine
// @Failure      500  {object}  ErrorResponse
// @Router       /machines [get]
func (h *handlers) listMachines(w http.ResponseWriter, r *http.Request) {
	machines, err := h.store.ListMachines(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if machines == nil {
		machines = []domain.Machine{}
	}
	writeJSON(w, http.StatusOK, machines)
}

// createMachine godoc
// @Summary      Create a machine
// @Description  Creates or updates a soda machine (upsert by ID)
// @Tags         Machines
// @Accept       json
// @Produce      json
// @Param        machine  body      domain.Machine  true  "Machine to create"
// @Success      201      {object}  domain.Machine
// @Failure      400      {object}  ErrorResponse
// @Failure      500      {object}  ErrorResponse
// @Security     ApiKeyAuth || BearerAuth
// @Router       /machines [post]
func (h *handlers) createMachine(w http.ResponseWriter, r *http.Request) {
	var m domain.Machine
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if m.Name == "" {
		writeErr(w, http.StatusBadRequest, "name is required")
		return
	}
	if m.SlotCount <= 0 {
		writeErr(w, http.StatusBadRequest, "slotCount must be positive")
		return
	}
	if m.ID == "" {
		m.ID = uuid.NewString()
	}
	if err := h.store.CreateMachine(r.Context(), &m); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, m)
}

// getMachineStatus godoc
// @Summary      Get machine stock status
// @Description  Returns computed stock status for every slot in a machine
// @Tags         Machines
// @Produce      json
// @Param        id   path      string  true  "Machine ID"
// @Success      200  {object}  domain.MachineStatus
// @Failure      404  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /machines/{id}/status [get]
func (h *handlers) getMachineStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	status, err := h.svc.GetMachineStatus(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if status == nil {
		writeErr(w, http.StatusNotFound, "machine not found")
		return
	}
	writeJSON(w, http.StatusOK, status)
}

// --- Slots ---

// setSlots godoc
// @Summary      Configure machine slots
// @Description  Destructive replace of all slots for a machine. Deletes existing slots and inserts the new set.
// @Tags         Machines
// @Accept       json
// @Produce      json
// @Param        id     path      string         true  "Machine ID"
// @Param        slots  body      []domain.Slot  true  "Slot configurations"
// @Success      200    {array}   domain.Slot
// @Failure      400    {object}  ErrorResponse
// @Failure      404    {object}  ErrorResponse
// @Failure      500    {object}  ErrorResponse
// @Security     ApiKeyAuth || BearerAuth
// @Router       /machines/{id}/slots [post]
func (h *handlers) setSlots(w http.ResponseWriter, r *http.Request) {
	machineID := chi.URLParam(r, "id")

	machine, err := h.store.GetMachine(r.Context(), machineID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if machine == nil {
		writeErr(w, http.StatusNotFound, "machine not found")
		return
	}

	var slots []domain.Slot
	if err := json.NewDecoder(r.Body).Decode(&slots); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	for i := range slots {
		slots[i].MachineID = machineID
	}
	if err := h.store.SetSlots(r.Context(), machineID, slots); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, slots)
}

// --- Dispenses ---

// createDispense godoc
// @Summary      Register a dispense event
// @Description  Records a soda dispense from the Pi agent. Auto-resolves productId from slot config if not provided.
// @Tags         Dispenses
// @Accept       json
// @Produce      json
// @Param        dispense  body      domain.Dispense  true  "Dispense event"
// @Success      201       {object}  domain.Dispense
// @Failure      400       {object}  ErrorResponse
// @Failure      401       {object}  ErrorResponse
// @Failure      500       {object}  ErrorResponse
// @Security     ApiKeyAuth
// @Router       /dispenses [post]
func (h *handlers) createDispense(w http.ResponseWriter, r *http.Request) {
	var d domain.Dispense
	if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if d.MachineID == "" {
		writeErr(w, http.StatusBadRequest, "machineId is required")
		return
	}
	if d.SlotPosition <= 0 {
		writeErr(w, http.StatusBadRequest, "slotPosition must be positive")
		return
	}
	if d.ID == "" {
		d.ID = uuid.NewString()
	}
	if d.Timestamp.IsZero() {
		d.Timestamp = time.Now().UTC()
	}
	if d.SourceType == "" {
		d.SourceType = "gpio"
	}

	// Look up the product for this slot if not provided
	if d.ProductID == "" {
		slots, err := h.store.GetSlots(r.Context(), d.MachineID)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		for _, sl := range slots {
			if sl.Position == d.SlotPosition {
				d.ProductID = sl.ProductID
				break
			}
		}
	}

	if err := h.svc.RegisterDispense(r.Context(), &d); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, d)
}

// listDispenses godoc
// @Summary      List dispense history
// @Description  Returns recent dispenses for a machine with optional time and limit filters
// @Tags         Dispenses
// @Produce      json
// @Param        id     path      string  true   "Machine ID"
// @Param        limit  query     int     false  "Max results (default 100)"
// @Param        since  query     string  false  "RFC3339 timestamp filter"
// @Success      200    {array}   domain.Dispense
// @Failure      500    {object}  ErrorResponse
// @Security     ApiKeyAuth || BearerAuth
// @Router       /machines/{id}/dispenses [get]
func (h *handlers) listDispenses(w http.ResponseWriter, r *http.Request) {
	machineID := chi.URLParam(r, "id")
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}
	since := time.Time{}
	if s := r.URL.Query().Get("since"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			since = t
		}
	}
	dispenses, err := h.svc.ListDispenses(r.Context(), machineID, since, limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if dispenses == nil {
		dispenses = []domain.Dispense{}
	}
	writeJSON(w, http.StatusOK, dispenses)
}

// --- Restocks ---

// createRestock godoc
// @Summary      Log a restock
// @Description  Records a restock event for a machine. Clears active alerts for restocked slots.
// @Tags         Restocks
// @Accept       json
// @Produce      json
// @Param        id       path      string          true  "Machine ID"
// @Param        restock  body      domain.Restock  true  "Restock data with slot quantities"
// @Success      201      {object}  domain.Restock
// @Failure      400      {object}  ErrorResponse
// @Failure      500      {object}  ErrorResponse
// @Security     ApiKeyAuth || BearerAuth
// @Router       /machines/{id}/restock [post]
func (h *handlers) createRestock(w http.ResponseWriter, r *http.Request) {
	machineID := chi.URLParam(r, "id")

	var rs domain.Restock
	if err := json.NewDecoder(r.Body).Decode(&rs); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	rs.MachineID = machineID
	if rs.ID == "" {
		rs.ID = uuid.NewString()
	}
	if rs.Timestamp.IsZero() {
		rs.Timestamp = time.Now().UTC()
	}
	if len(rs.Slots) == 0 {
		writeErr(w, http.StatusBadRequest, "slots are required")
		return
	}
	if err := h.svc.CreateRestock(r.Context(), &rs); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, rs)
}

// --- Products ---

// listProducts godoc
// @Summary      List all products
// @Description  Returns all registered products
// @Tags         Products
// @Produce      json
// @Success      200  {array}   domain.Product
// @Failure      500  {object}  ErrorResponse
// @Security     ApiKeyAuth || BearerAuth
// @Router       /products [get]
func (h *handlers) listProducts(w http.ResponseWriter, r *http.Request) {
	products, err := h.store.ListProducts(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if products == nil {
		products = []domain.Product{}
	}
	writeJSON(w, http.StatusOK, products)
}

// createProduct godoc
// @Summary      Create a product
// @Description  Creates or updates a product (upsert by ID)
// @Tags         Products
// @Accept       json
// @Produce      json
// @Param        product  body      domain.Product  true  "Product to create"
// @Success      201      {object}  domain.Product
// @Failure      400      {object}  ErrorResponse
// @Failure      500      {object}  ErrorResponse
// @Security     ApiKeyAuth || BearerAuth
// @Router       /products [post]
func (h *handlers) createProduct(w http.ResponseWriter, r *http.Request) {
	var p domain.Product
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if p.Name == "" {
		writeErr(w, http.StatusBadRequest, "name is required")
		return
	}
	p.Color = strings.ToLower(p.Color)
	if p.Color != "" && !hexColor.MatchString(p.Color) {
		writeErr(w, http.StatusBadRequest, `color must be a "#rrggbb" hex value`)
		return
	}
	if p.SizeOz < 0 || p.CaffeineMg < 0 {
		writeErr(w, http.StatusBadRequest, "sizeOz and caffeineMg can't be negative")
		return
	}
	if p.SizeOz == 0 {
		p.SizeOz = 12
	}
	if p.ID == "" {
		p.ID = uuid.NewString()
	}
	if err := h.store.CreateProduct(r.Context(), &p); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

var hexColor = regexp.MustCompile(`^#[0-9a-f]{6}$`)

// deleteProduct godoc
// @Summary      Delete a product
// @Description  Deletes a product by ID. Fails with 409 if a slot still holds the product.
// @Tags         Products
// @Produce      json
// @Param        id   path      string  true  "Product ID"
// @Success      200  {object}  DeleteResponse
// @Failure      409  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Security     ApiKeyAuth || BearerAuth
// @Router       /products/{id} [delete]
func (h *handlers) deleteProduct(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.DeleteProduct(r.Context(), id); err != nil {
		var inUse *service.ProductInUseError
		if errors.As(err, &inUse) {
			writeErr(w, http.StatusConflict, err.Error())
			return
		}
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"deleted": id})
}

// deleteMachine godoc
// @Summary      Delete a machine
// @Description  Deletes a machine and all associated data (slots, dispenses, restocks, alerts)
// @Tags         Machines
// @Produce      json
// @Param        id   path      string  true  "Machine ID"
// @Success      200  {object}  DeleteResponse
// @Failure      500  {object}  ErrorResponse
// @Security     ApiKeyAuth || BearerAuth
// @Router       /machines/{id} [delete]
func (h *handlers) deleteMachine(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.DeleteMachine(r.Context(), id); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"deleted": id})
}

// --- Auth ---

// login godoc
// @Summary      Login
// @Description  Authenticates a user and returns a JWT token
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        credentials  body      LoginRequest   true  "Login credentials"
// @Success      200          {object}  LoginResponse
// @Failure      400          {object}  ErrorResponse
// @Failure      401          {object}  ErrorResponse
// @Router       /auth/login [post]
func (h *handlers) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Username == "" || req.Password == "" {
		writeErr(w, http.StatusBadRequest, "username and password are required")
		return
	}
	token, user, err := h.svc.Login(r.Context(), req.Username, req.Password)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"token": token,
		"user":  user,
	})
}

// --- Admin ---

// getAdminStats godoc
// @Summary      Get dispense statistics
// @Description  Returns aggregated dispense stats including time series, top products, and cost totals
// @Tags         Admin
// @Produce      json
// @Param        machineId  query     string  true   "Machine ID"
// @Param        since      query     string  false  "RFC3339 start time (default: 30 days ago)"
// @Success      200        {object}  service.DispenseStats
// @Failure      400        {object}  ErrorResponse
// @Failure      500        {object}  ErrorResponse
// @Security     ApiKeyAuth || BearerAuth
// @Router       /admin/stats [get]
func (h *handlers) getAdminStats(w http.ResponseWriter, r *http.Request) {
	machineID := r.URL.Query().Get("machineId")
	if machineID == "" {
		writeErr(w, http.StatusBadRequest, "machineId is required")
		return
	}
	since := time.Now().UTC().AddDate(0, 0, -30)
	if s := r.URL.Query().Get("since"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			since = t
		}
	}
	stats, err := h.svc.GetDispenseStats(r.Context(), machineID, since)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

// getMachineCost godoc
// @Summary      Get machine cost
// @Description  Returns the total cost of dispenses for a machine over a time period
// @Tags         Admin
// @Produce      json
// @Param        machineId  query     string  true   "Machine ID"
// @Param        since      query     string  false  "RFC3339 start time (default: 30 days ago)"
// @Success      200        {object}  CostResponse
// @Failure      400        {object}  ErrorResponse
// @Failure      500        {object}  ErrorResponse
// @Security     ApiKeyAuth || BearerAuth
// @Router       /admin/cost [get]
func (h *handlers) getMachineCost(w http.ResponseWriter, r *http.Request) {
	machineID := r.URL.Query().Get("machineId")
	if machineID == "" {
		writeErr(w, http.StatusBadRequest, "machineId is required")
		return
	}
	since := time.Now().UTC().AddDate(0, 0, -30)
	if s := r.URL.Query().Get("since"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			since = t
		}
	}
	cost, err := h.svc.GetMachineCost(r.Context(), machineID, since)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]float64{"cost": cost})
}

// listRestocks godoc
// @Summary      List restock history
// @Description  Returns recent restocks for a machine
// @Tags         Restocks
// @Produce      json
// @Param        id     path      string  true   "Machine ID"
// @Param        limit  query     int     false  "Max results (default 50)"
// @Success      200    {array}   domain.Restock
// @Failure      500    {object}  ErrorResponse
// @Security     ApiKeyAuth || BearerAuth
// @Router       /machines/{id}/restocks [get]
func (h *handlers) listRestocks(w http.ResponseWriter, r *http.Request) {
	machineID := chi.URLParam(r, "id")
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}
	restocks, err := h.svc.ListRestocks(r.Context(), machineID, limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if restocks == nil {
		restocks = []domain.Restock{}
	}
	writeJSON(w, http.StatusOK, restocks)
}

// --- Alerts ---

// listAlertConfigs godoc
// @Summary      List alert configs
// @Description  Returns all alert configurations for a machine
// @Tags         Alerts
// @Produce      json
// @Param        id   path      string  true  "Machine ID"
// @Success      200  {array}   domain.AlertConfig
// @Failure      500  {object}  ErrorResponse
// @Security     ApiKeyAuth || BearerAuth
// @Router       /machines/{id}/alerts [get]
func (h *handlers) listAlertConfigs(w http.ResponseWriter, r *http.Request) {
	machineID := chi.URLParam(r, "id")
	configs, err := h.svc.ListAlertConfigs(r.Context(), machineID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if configs == nil {
		configs = []domain.AlertConfig{}
	}
	writeJSON(w, http.StatusOK, configs)
}

// createAlertConfig godoc
// @Summary      Create alert config
// @Description  Creates an alert threshold config for a machine. SlotPosition 0 = machine-wide default. LowThreshold -1 = use slot's own threshold.
// @Tags         Alerts
// @Accept       json
// @Produce      json
// @Param        id      path      string              true  "Machine ID"
// @Param        config  body      domain.AlertConfig  true  "Alert configuration"
// @Success      201     {object}  domain.AlertConfig
// @Failure      400     {object}  ErrorResponse
// @Failure      500     {object}  ErrorResponse
// @Security     ApiKeyAuth || BearerAuth
// @Router       /machines/{id}/alerts [post]
func (h *handlers) createAlertConfig(w http.ResponseWriter, r *http.Request) {
	machineID := chi.URLParam(r, "id")
	var ac domain.AlertConfig
	if err := json.NewDecoder(r.Body).Decode(&ac); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	ac.MachineID = machineID
	if ac.ID == "" {
		ac.ID = uuid.NewString()
	}
	if err := h.svc.CreateAlertConfig(r.Context(), &ac); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, ac)
}

// updateAlertConfig godoc
// @Summary      Update alert config
// @Description  Updates an existing alert configuration
// @Tags         Alerts
// @Accept       json
// @Produce      json
// @Param        id      path      string              true  "Alert config ID"
// @Param        config  body      domain.AlertConfig  true  "Updated alert configuration"
// @Success      200     {object}  domain.AlertConfig
// @Failure      400     {object}  ErrorResponse
// @Failure      500     {object}  ErrorResponse
// @Security     ApiKeyAuth || BearerAuth
// @Router       /alerts/{id} [put]
func (h *handlers) updateAlertConfig(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var ac domain.AlertConfig
	if err := json.NewDecoder(r.Body).Decode(&ac); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	ac.ID = id
	if err := h.svc.UpdateAlertConfig(r.Context(), &ac); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, ac)
}

// deleteAlertConfig godoc
// @Summary      Delete alert config
// @Description  Deletes an alert configuration by ID
// @Tags         Alerts
// @Produce      json
// @Param        id   path      string  true  "Alert config ID"
// @Success      200  {object}  DeleteResponse
// @Failure      500  {object}  ErrorResponse
// @Security     ApiKeyAuth || BearerAuth
// @Router       /alerts/{id} [delete]
func (h *handlers) deleteAlertConfig(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.DeleteAlertConfig(r.Context(), id); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"deleted": id})
}

// listAlertEvents godoc
// @Summary      List fired alert events
// @Description  Returns alert events for a machine with optional active-only filter
// @Tags         Alerts
// @Produce      json
// @Param        id      path      string  true   "Machine ID"
// @Param        active  query     bool    false  "Only return active (uncleared) alerts"
// @Param        limit   query     int     false  "Max results (default 50)"
// @Success      200     {array}   domain.AlertEvent
// @Failure      500     {object}  ErrorResponse
// @Security     ApiKeyAuth || BearerAuth
// @Router       /machines/{id}/alert-events [get]
func (h *handlers) listAlertEvents(w http.ResponseWriter, r *http.Request) {
	machineID := chi.URLParam(r, "id")
	activeOnly := r.URL.Query().Get("active") == "true"
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}
	events, err := h.svc.ListAlertEvents(r.Context(), machineID, activeOnly, limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if events == nil {
		events = []domain.AlertEvent{}
	}
	writeJSON(w, http.StatusOK, events)
}

// getStockPredictions godoc
// @Summary      Get stock-out predictions
// @Description  Computes estimated days until stock-out for each slot based on recent dispense rate
// @Tags         Predictions
// @Produce      json
// @Param        id            path      string  true   "Machine ID"
// @Param        lookbackDays  query     int     false  "Days of dispense history to analyze (default 14)"
// @Success      200           {array}   domain.StockPrediction
// @Failure      500           {object}  ErrorResponse
// @Security     ApiKeyAuth || BearerAuth
// @Router       /machines/{id}/predictions [get]
func (h *handlers) getStockPredictions(w http.ResponseWriter, r *http.Request) {
	machineID := chi.URLParam(r, "id")
	lookbackDays := 14
	if l := r.URL.Query().Get("lookbackDays"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			lookbackDays = v
		}
	}
	predictions, err := h.svc.GetStockPredictions(r.Context(), machineID, lookbackDays)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if predictions == nil {
		predictions = []domain.StockPrediction{}
	}
	writeJSON(w, http.StatusOK, predictions)
}

// --- Reports ---

// exportDispenses godoc
// @Summary      Export dispense report
// @Description  Exports dispense history as CSV or PDF
// @Tags         Reports
// @Produce      text/csv,application/pdf
// @Param        machineId  query     string  true   "Machine ID"
// @Param        format     query     string  false  "Export format: csv or pdf (default: csv)"  Enums(csv, pdf)
// @Param        since      query     string  false  "RFC3339 start time filter"
// @Success      200        {file}    file
// @Failure      400        {object}  ErrorResponse
// @Failure      500        {object}  ErrorResponse
// @Security     ApiKeyAuth || BearerAuth
// @Router       /reports/dispenses [get]
func (h *handlers) exportDispenses(w http.ResponseWriter, r *http.Request) {
	machineID := r.URL.Query().Get("machineId")
	if machineID == "" {
		writeErr(w, http.StatusBadRequest, "machineId is required")
		return
	}
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "csv"
	}
	if format != "csv" && format != "pdf" {
		writeErr(w, http.StatusBadRequest, "format must be csv or pdf")
		return
	}

	since := time.Time{}
	if s := r.URL.Query().Get("since"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			since = t
		}
	}

	dispenses, err := h.svc.ListDispenses(r.Context(), machineID, since, 10000)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	products, err := h.store.ListProducts(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	productMap := make(map[string]domain.Product, len(products))
	for _, p := range products {
		productMap[p.ID] = p
	}

	machine, err := h.store.GetMachine(r.Context(), machineID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	machineName := machineID
	if machine != nil {
		machineName = machine.Name
	}

	switch format {
	case "csv":
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="dispenses-%s.csv"`, machineID))
		if err := export.WriteDispensesCSV(w, dispenses, productMap); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
		}
	case "pdf":
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="dispenses-%s.pdf"`, machineID))
		if err := export.WriteDispensesPDF(w, dispenses, productMap, machineName); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
		}
	}
}

// --- Admin Data Management ---

// resetAllData godoc
// @Summary      Reset all data
// @Description  Deletes all data from the database and re-seeds the admin user
// @Tags         Admin
// @Produce      json
// @Success      200  {object}  object{status=string}
// @Failure      401  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Security     BearerAuth
// @Router       /admin/reset [delete]
func (h *handlers) resetAllData(w http.ResponseWriter, r *http.Request) {
	if err := h.store.ResetAllData(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- User Preferences ---

// --- Helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
