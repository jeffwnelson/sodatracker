package api

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/google/uuid"
	"github.com/jeffwnelson/sodatracker/internal/domain"
	"github.com/jeffwnelson/sodatracker/internal/events"
	"github.com/jeffwnelson/sodatracker/internal/events/local"
	"github.com/jeffwnelson/sodatracker/internal/service"
	"github.com/jeffwnelson/sodatracker/internal/store/sqlite"
)

const testJWTSecret = "test-jwt-secret"

func setup(t *testing.T) http.Handler {
	t.Helper()
	st, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	bus := local.New()
	t.Cleanup(func() { bus.Close() })
	svc := service.New(st, testJWTSecret, bus)
	wireDispenseSubscriber(bus, svc)
	return NewRouter(st, svc, "test-key", testJWTSecret, "")
}

func setupWithStore(t *testing.T) (http.Handler, *sqlite.SQLiteStore) {
	t.Helper()
	st, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	bus := local.New()
	t.Cleanup(func() { bus.Close() })
	svc := service.New(st, testJWTSecret, bus)
	wireDispenseSubscriber(bus, svc)
	return NewRouter(st, svc, "test-key", testJWTSecret, ""), st
}

func wireDispenseSubscriber(bus *local.LocalBus, svc *service.Service) {
	bus.Subscribe(events.EventDispense, func(ctx context.Context, evt events.Event) error {
		machineID, _ := evt.Data["machineId"].(string)
		slotPos := 0
		switch v := evt.Data["slotPosition"].(type) {
		case int:
			slotPos = v
		case float64:
			slotPos = int(v)
		}
		if machineID != "" && slotPos > 0 {
			return svc.EvaluateStockAlert(ctx, machineID, slotPos)
		}
		return nil
	})
}

func seedTestUser(t *testing.T, st *sqlite.SQLiteStore) {
	t.Helper()
	hash, _ := bcrypt.GenerateFromPassword([]byte("test-password"), bcrypt.DefaultCost)
	u := &domain.User{ID: uuid.NewString(), Username: "admin", PasswordHash: string(hash), Role: "admin"}
	if err := st.CreateUser(context.Background(), u); err != nil {
		t.Fatalf("seed user: %v", err)
	}
}

func loginAndGetToken(t *testing.T, r http.Handler) string {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"username": "admin", "password": "test-password"})
	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("login: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	return resp["token"].(string)
}

func TestHealth(t *testing.T) {
	r := setup(t)
	req := httptest.NewRequest("GET", "/api/v1/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestCreateAndListMachines(t *testing.T) {
	r := setup(t)

	// Create
	body, _ := json.Marshal(domain.Machine{Name: "Test Machine", SlotCount: 5})
	req := httptest.NewRequest("POST", "/api/v1/machines", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var created domain.Machine
	json.NewDecoder(w.Body).Decode(&created)
	if created.ID == "" {
		t.Error("expected machine ID to be set")
	}

	// List
	req = httptest.NewRequest("GET", "/api/v1/machines", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d", w.Code)
	}

	var machines []domain.Machine
	json.NewDecoder(w.Body).Decode(&machines)
	if len(machines) != 1 {
		t.Errorf("expected 1 machine, got %d", len(machines))
	}
}

func TestDispenseRequiresAPIKey(t *testing.T) {
	r := setup(t)
	body := []byte(`{"machineId":"m1","slotPosition":1}`)
	req := httptest.NewRequest("POST", "/api/v1/dispenses", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without API key, got %d", w.Code)
	}
}

func TestLoginSuccess(t *testing.T) {
	r, st := setupWithStore(t)
	seedTestUser(t, st)

	body, _ := json.Marshal(map[string]string{"username": "admin", "password": "test-password"})
	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["token"] == nil || resp["token"].(string) == "" {
		t.Error("expected token in response")
	}
}

func TestLoginBadPassword(t *testing.T) {
	r, st := setupWithStore(t)
	seedTestUser(t, st)

	body, _ := json.Marshal(map[string]string{"username": "admin", "password": "wrong"})
	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAdminStatsRequiresAuth(t *testing.T) {
	r := setup(t)
	req := httptest.NewRequest("GET", "/api/v1/admin/stats?machineId=m1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestFullWorkflow(t *testing.T) {
	r := setup(t)

	// 1. Create product
	body, _ := json.Marshal(domain.Product{UPC: "123456789012", Name: "Cola", Description: "Classic cola", CostPerCan: 0.50})
	req := httptest.NewRequest("POST", "/api/v1/products", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create product: %d %s", w.Code, w.Body.String())
	}
	var product domain.Product
	json.NewDecoder(w.Body).Decode(&product)

	// 2. Create machine
	body, _ = json.Marshal(domain.Machine{Name: "Break Room", SlotCount: 5})
	req = httptest.NewRequest("POST", "/api/v1/machines", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create machine: %d", w.Code)
	}
	var machine domain.Machine
	json.NewDecoder(w.Body).Decode(&machine)

	// 3. Configure slots
	slots := []domain.Slot{
		{Position: 1, ProductID: product.ID, Capacity: 10, LowThreshold: 2},
	}
	body, _ = json.Marshal(slots)
	req = httptest.NewRequest("POST", "/api/v1/machines/"+machine.ID+"/slots", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("set slots: %d", w.Code)
	}

	// 4. Restock
	restock := map[string]any{
		"slots": []map[string]any{{"position": 1, "quantity": 10}},
	}
	body, _ = json.Marshal(restock)
	req = httptest.NewRequest("POST", "/api/v1/machines/"+machine.ID+"/restock", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("restock: %d %s", w.Code, w.Body.String())
	}

	// 5. Dispense
	dispense := map[string]any{"machineId": machine.ID, "slotPosition": 1}
	body, _ = json.Marshal(dispense)
	req = httptest.NewRequest("POST", "/api/v1/dispenses", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("dispense: %d %s", w.Code, w.Body.String())
	}

	// 6. Check status — stock should be 9
	req = httptest.NewRequest("GET", "/api/v1/machines/"+machine.ID+"/status", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	var status domain.MachineStatus
	json.NewDecoder(w.Body).Decode(&status)
	if len(status.Slots) != 1 {
		t.Fatalf("expected 1 slot, got %d", len(status.Slots))
	}
	if status.Slots[0].CurrentStock != 9 {
		t.Errorf("expected stock=9, got %d", status.Slots[0].CurrentStock)
	}
	if status.Slots[0].Status != "ok" {
		t.Errorf("expected status=ok, got %s", status.Slots[0].Status)
	}
}

func TestFullWorkflowWithJWT(t *testing.T) {
	r, st := setupWithStore(t)
	seedTestUser(t, st)
	token := loginAndGetToken(t, r)

	// 1. Create product with JWT
	body, _ := json.Marshal(domain.Product{UPC: "123456789012", Name: "Cola", Description: "Classic cola", CostPerCan: 0.50})
	req := httptest.NewRequest("POST", "/api/v1/products", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create product: %d %s", w.Code, w.Body.String())
	}
	var product domain.Product
	json.NewDecoder(w.Body).Decode(&product)

	// 2. Create machine with JWT
	body, _ = json.Marshal(domain.Machine{Name: "Break Room", SlotCount: 5})
	req = httptest.NewRequest("POST", "/api/v1/machines", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create machine: %d", w.Code)
	}
	var machine domain.Machine
	json.NewDecoder(w.Body).Decode(&machine)

	// 3. Set slots with JWT
	slots := []domain.Slot{
		{Position: 1, ProductID: product.ID, Capacity: 10, LowThreshold: 2},
	}
	body, _ = json.Marshal(slots)
	req = httptest.NewRequest("POST", "/api/v1/machines/"+machine.ID+"/slots", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("set slots: %d", w.Code)
	}

	// 4. Restock with JWT
	restock := map[string]any{"slots": []map[string]any{{"position": 1, "quantity": 10}}}
	body, _ = json.Marshal(restock)
	req = httptest.NewRequest("POST", "/api/v1/machines/"+machine.ID+"/restock", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("restock: %d %s", w.Code, w.Body.String())
	}

	// 5. Get admin stats with JWT
	req = httptest.NewRequest("GET", "/api/v1/admin/stats?machineId="+machine.ID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("admin stats: %d %s", w.Code, w.Body.String())
	}

	// 6. Get machine cost with JWT
	req = httptest.NewRequest("GET", "/api/v1/admin/cost?machineId="+machine.ID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("admin cost: %d %s", w.Code, w.Body.String())
	}

	// 7. List restocks with JWT
	req = httptest.NewRequest("GET", "/api/v1/machines/"+machine.ID+"/restocks", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list restocks: %d %s", w.Code, w.Body.String())
	}
	var restocks []domain.Restock
	json.NewDecoder(w.Body).Decode(&restocks)
	if len(restocks) != 1 {
		t.Errorf("expected 1 restock, got %d", len(restocks))
	}
}

func TestAlertConfigCRUD(t *testing.T) {
	r := setup(t)

	// Create machine first
	body, _ := json.Marshal(domain.Machine{ID: "m1", Name: "Test", SlotCount: 3})
	req := httptest.NewRequest("POST", "/api/v1/machines", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create machine: %d", w.Code)
	}

	// Create alert config
	acBody := map[string]any{"slotPosition": 1, "lowThreshold": 3, "enabled": true}
	body, _ = json.Marshal(acBody)
	req = httptest.NewRequest("POST", "/api/v1/machines/m1/alerts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create alert config: %d %s", w.Code, w.Body.String())
	}
	var created domain.AlertConfig
	json.NewDecoder(w.Body).Decode(&created)
	if created.ID == "" {
		t.Error("expected ID")
	}

	// List
	req = httptest.NewRequest("GET", "/api/v1/machines/m1/alerts", nil)
	req.Header.Set("X-API-Key", "test-key")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list: %d", w.Code)
	}
	var configs []domain.AlertConfig
	json.NewDecoder(w.Body).Decode(&configs)
	if len(configs) != 1 {
		t.Fatalf("expected 1, got %d", len(configs))
	}

	// Update
	updateBody := map[string]any{"lowThreshold": 5, "enabled": false}
	body, _ = json.Marshal(updateBody)
	req = httptest.NewRequest("PUT", "/api/v1/alerts/"+created.ID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("update: %d %s", w.Code, w.Body.String())
	}

	// Delete
	req = httptest.NewRequest("DELETE", "/api/v1/alerts/"+created.ID, nil)
	req.Header.Set("X-API-Key", "test-key")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("delete: %d", w.Code)
	}

	// Verify deleted
	req = httptest.NewRequest("GET", "/api/v1/machines/m1/alerts", nil)
	req.Header.Set("X-API-Key", "test-key")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	json.NewDecoder(w.Body).Decode(&configs)
	if len(configs) != 0 {
		t.Errorf("expected 0 after delete, got %d", len(configs))
	}
}

func TestPredictionsEndpoint(t *testing.T) {
	r := setup(t)

	// Create product + machine + slots + restock
	body, _ := json.Marshal(domain.Product{ID: "p1", UPC: "111", Name: "Cola", Description: "Classic cola"})
	req := httptest.NewRequest("POST", "/api/v1/products", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	body, _ = json.Marshal(domain.Machine{ID: "m1", Name: "Test", SlotCount: 3})
	req = httptest.NewRequest("POST", "/api/v1/machines", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	body, _ = json.Marshal([]domain.Slot{{Position: 1, ProductID: "p1", Capacity: 10, LowThreshold: 2}})
	req = httptest.NewRequest("POST", "/api/v1/machines/m1/slots", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	body, _ = json.Marshal(map[string]any{"slots": []map[string]any{{"position": 1, "quantity": 10}}})
	req = httptest.NewRequest("POST", "/api/v1/machines/m1/restock", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Get predictions
	req = httptest.NewRequest("GET", "/api/v1/machines/m1/predictions", nil)
	req.Header.Set("X-API-Key", "test-key")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("predictions: %d %s", w.Code, w.Body.String())
	}
	var preds []domain.StockPrediction
	json.NewDecoder(w.Body).Decode(&preds)
	if len(preds) != 1 {
		t.Fatalf("expected 1 prediction, got %d", len(preds))
	}
	if preds[0].CurrentStock != 10 {
		t.Errorf("expected stock=10, got %d", preds[0].CurrentStock)
	}
}

func TestFullWorkflowWithAlerts(t *testing.T) {
	r := setup(t)

	// 1. Create product
	body, _ := json.Marshal(domain.Product{ID: "p1", UPC: "111", Name: "Cola", Description: "Classic cola"})
	req := httptest.NewRequest("POST", "/api/v1/products", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("product: %d", w.Code)
	}

	// 2. Create machine
	body, _ = json.Marshal(domain.Machine{ID: "m1", Name: "Test", SlotCount: 3})
	req = httptest.NewRequest("POST", "/api/v1/machines", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 3. Set slots (lowThreshold=2)
	body, _ = json.Marshal([]domain.Slot{{Position: 1, ProductID: "p1", Capacity: 10, LowThreshold: 2}})
	req = httptest.NewRequest("POST", "/api/v1/machines/m1/slots", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 4. Restock with 5
	body, _ = json.Marshal(map[string]any{"slots": []map[string]any{{"position": 1, "quantity": 5}}})
	req = httptest.NewRequest("POST", "/api/v1/machines/m1/restock", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("restock: %d %s", w.Code, w.Body.String())
	}

	// 5. Dispense 3 times → stock=2 (at threshold) → triggers low_stock alert
	for i := 0; i < 3; i++ {
		body, _ = json.Marshal(map[string]any{"machineId": "m1", "slotPosition": 1})
		req = httptest.NewRequest("POST", "/api/v1/dispenses", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Key", "test-key")
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("dispense %d: %d %s", i, w.Code, w.Body.String())
		}
	}

	// 6. Check alert events — should have low_stock
	req = httptest.NewRequest("GET", "/api/v1/machines/m1/alert-events?active=true", nil)
	req.Header.Set("X-API-Key", "test-key")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("alert events: %d", w.Code)
	}
	var alertEvents []domain.AlertEvent
	json.NewDecoder(w.Body).Decode(&alertEvents)
	if len(alertEvents) != 1 {
		t.Fatalf("expected 1 active alert, got %d", len(alertEvents))
	}
	if alertEvents[0].AlertType != "low_stock" {
		t.Errorf("expected low_stock, got %s", alertEvents[0].AlertType)
	}

	// 7. Restock clears alerts
	body, _ = json.Marshal(map[string]any{"slots": []map[string]any{{"position": 1, "quantity": 10}}})
	req = httptest.NewRequest("POST", "/api/v1/machines/m1/restock", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("restock2: %d", w.Code)
	}

	// 8. Verify alerts cleared
	req = httptest.NewRequest("GET", "/api/v1/machines/m1/alert-events?active=true", nil)
	req.Header.Set("X-API-Key", "test-key")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	json.NewDecoder(w.Body).Decode(&alertEvents)
	if len(alertEvents) != 0 {
		t.Errorf("expected 0 active alerts after restock, got %d", len(alertEvents))
	}
}

// setupExportData creates a machine, product, slot, restock, and dispenses for export tests.
// Returns the machine ID.
func setupExportData(t *testing.T, r http.Handler, st *sqlite.SQLiteStore) string {
	t.Helper()

	// Create product
	body, _ := json.Marshal(domain.Product{ID: "p1", UPC: "111222333444", Name: "Cola", Description: "Classic cola", CostPerCan: 0.50})
	req := httptest.NewRequest("POST", "/api/v1/products", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create product: %d %s", w.Code, w.Body.String())
	}

	// Create machine
	body, _ = json.Marshal(domain.Machine{ID: "m1", Name: "Break Room", SlotCount: 5})
	req = httptest.NewRequest("POST", "/api/v1/machines", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create machine: %d", w.Code)
	}

	// Set slots
	body, _ = json.Marshal([]domain.Slot{{Position: 1, ProductID: "p1", Capacity: 10, LowThreshold: 2}})
	req = httptest.NewRequest("POST", "/api/v1/machines/m1/slots", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Restock
	body, _ = json.Marshal(map[string]any{"slots": []map[string]any{{"position": 1, "quantity": 10}}})
	req = httptest.NewRequest("POST", "/api/v1/machines/m1/restock", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Dispense twice
	for i := 0; i < 2; i++ {
		body, _ = json.Marshal(map[string]any{"machineId": "m1", "slotPosition": 1})
		req = httptest.NewRequest("POST", "/api/v1/dispenses", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Key", "test-key")
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("dispense %d: %d", i, w.Code)
		}
	}

	return "m1"
}

func TestExportDispensesCSV(t *testing.T) {
	r, st := setupWithStore(t)
	seedTestUser(t, st)
	token := loginAndGetToken(t, r)
	machineID := setupExportData(t, r, st)

	req := httptest.NewRequest("GET", "/api/v1/reports/dispenses?machineId="+machineID+"&format=csv", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/csv" {
		t.Errorf("expected Content-Type text/csv, got %s", ct)
	}
	if cd := w.Header().Get("Content-Disposition"); !strings.Contains(cd, "attachment") {
		t.Errorf("expected Content-Disposition attachment, got %s", cd)
	}

	reader := csv.NewReader(strings.NewReader(w.Body.String()))
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("parse CSV: %v", err)
	}
	// Header + 2 dispenses
	if len(records) != 3 {
		t.Fatalf("expected 3 CSV rows, got %d", len(records))
	}
	if records[1][3] != "Cola" {
		t.Errorf("expected Cola, got %s", records[1][3])
	}
}

func TestExportDispensesPDF(t *testing.T) {
	r, st := setupWithStore(t)
	seedTestUser(t, st)
	token := loginAndGetToken(t, r)
	machineID := setupExportData(t, r, st)

	req := httptest.NewRequest("GET", "/api/v1/reports/dispenses?machineId="+machineID+"&format=pdf", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/pdf" {
		t.Errorf("expected Content-Type application/pdf, got %s", ct)
	}
	if !strings.HasPrefix(w.Body.String(), "%PDF-") {
		t.Error("expected PDF header in response body")
	}
}

func TestDeleteProduct(t *testing.T) {
	r := setup(t)

	// Create product
	body, _ := json.Marshal(domain.Product{Name: "Cola", UPC: "111"})
	req := httptest.NewRequest("POST", "/api/v1/products", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var product domain.Product
	json.NewDecoder(w.Body).Decode(&product)

	// Delete without auth should fail
	req = httptest.NewRequest("DELETE", "/api/v1/products/"+product.ID, nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without auth, got %d", w.Code)
	}

	// Delete with auth
	req = httptest.NewRequest("DELETE", "/api/v1/products/"+product.ID, nil)
	req.Header.Set("X-API-Key", "test-key")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", w.Code, w.Body.String())
	}
	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["deleted"] != product.ID {
		t.Errorf("expected deleted=%s, got %s", product.ID, resp["deleted"])
	}

	// Verify gone
	req = httptest.NewRequest("GET", "/api/v1/products", nil)
	req.Header.Set("X-API-Key", "test-key")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var products []domain.Product
	json.NewDecoder(w.Body).Decode(&products)
	if len(products) != 0 {
		t.Errorf("expected 0 products after delete, got %d", len(products))
	}
}

func TestDeleteMachine(t *testing.T) {
	r := setup(t)

	// Create product + machine + slots + restock + dispense
	body, _ := json.Marshal(domain.Product{ID: "p1", Name: "Cola"})
	req := httptest.NewRequest("POST", "/api/v1/products", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	body, _ = json.Marshal(domain.Machine{Name: "Test", SlotCount: 3})
	req = httptest.NewRequest("POST", "/api/v1/machines", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create machine: %d", w.Code)
	}
	var machine domain.Machine
	json.NewDecoder(w.Body).Decode(&machine)

	// Set slots
	body, _ = json.Marshal([]domain.Slot{{Position: 1, ProductID: "p1", Capacity: 10, LowThreshold: 2}})
	req = httptest.NewRequest("POST", "/api/v1/machines/"+machine.ID+"/slots", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Restock
	body, _ = json.Marshal(map[string]any{"slots": []map[string]any{{"position": 1, "quantity": 10}}})
	req = httptest.NewRequest("POST", "/api/v1/machines/"+machine.ID+"/restock", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Dispense
	body, _ = json.Marshal(map[string]any{"machineId": machine.ID, "slotPosition": 1})
	req = httptest.NewRequest("POST", "/api/v1/dispenses", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Delete without auth should fail
	req = httptest.NewRequest("DELETE", "/api/v1/machines/"+machine.ID, nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without auth, got %d", w.Code)
	}

	// Delete with auth
	req = httptest.NewRequest("DELETE", "/api/v1/machines/"+machine.ID, nil)
	req.Header.Set("X-API-Key", "test-key")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", w.Code, w.Body.String())
	}
	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["deleted"] != machine.ID {
		t.Errorf("expected deleted=%s, got %s", machine.ID, resp["deleted"])
	}

	// Verify machine gone from list
	req = httptest.NewRequest("GET", "/api/v1/machines", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var machines []domain.Machine
	json.NewDecoder(w.Body).Decode(&machines)
	if len(machines) != 0 {
		t.Errorf("expected 0 machines after delete, got %d", len(machines))
	}

	// Verify status returns 404
	req = httptest.NewRequest("GET", "/api/v1/machines/"+machine.ID+"/status", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for deleted machine status, got %d", w.Code)
	}
}

func TestUpdateUserTheme(t *testing.T) {
	r, st := setupWithStore(t)
	seedTestUser(t, st)
	token := loginAndGetToken(t, r)

	// Update theme to dark
	body, _ := json.Marshal(map[string]string{"theme": "dark"})
	req := httptest.NewRequest("PUT", "/api/v1/users/me/theme", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["theme"] != "dark" {
		t.Errorf("expected theme=dark, got %s", resp["theme"])
	}

	// Verify login returns updated theme
	loginBody, _ := json.Marshal(map[string]string{"username": "admin", "password": "test-password"})
	req = httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(loginBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("login: expected 200, got %d", w.Code)
	}
	var loginResp map[string]any
	json.NewDecoder(w.Body).Decode(&loginResp)
	user := loginResp["user"].(map[string]any)
	if user["theme"] != "dark" {
		t.Errorf("expected login to return theme=dark, got %v", user["theme"])
	}
}

func TestUpdateUserThemeInvalid(t *testing.T) {
	r, st := setupWithStore(t)
	seedTestUser(t, st)
	token := loginAndGetToken(t, r)

	body, _ := json.Marshal(map[string]string{"theme": "neon"})
	req := httptest.NewRequest("PUT", "/api/v1/users/me/theme", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid theme, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateUserThemeRequiresJWT(t *testing.T) {
	r := setup(t)

	body, _ := json.Marshal(map[string]string{"theme": "dark"})
	req := httptest.NewRequest("PUT", "/api/v1/users/me/theme", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without JWT, got %d", w.Code)
	}

	// API key should NOT work (JWT-only endpoint)
	req = httptest.NewRequest("PUT", "/api/v1/users/me/theme", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 with API key only, got %d", w.Code)
	}
}

func TestExportRequiresAuth(t *testing.T) {
	r := setup(t)
	req := httptest.NewRequest("GET", "/api/v1/reports/dispenses?machineId=m1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without auth, got %d", w.Code)
	}
}
