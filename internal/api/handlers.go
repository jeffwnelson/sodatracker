package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jeffwnelson/sodatracker/internal/domain"
	"github.com/jeffwnelson/sodatracker/internal/service"
	"github.com/jeffwnelson/sodatracker/internal/store"
)

type handlers struct {
	store     store.Store
	svc       *service.Service
	jwtSecret string
}

func (h *handlers) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- Machines ---

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
	if err := h.store.CreateRestock(r.Context(), &rs); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, rs)
}

// --- Products ---

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

func (h *handlers) createProduct(w http.ResponseWriter, r *http.Request) {
	var p domain.Product
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if p.UPC == "" {
		writeErr(w, http.StatusBadRequest, "upc is required")
		return
	}
	if p.Name == "" {
		writeErr(w, http.StatusBadRequest, "name is required")
		return
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

// --- Auth ---

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

// --- Helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
