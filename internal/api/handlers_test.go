package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/google/uuid"
	"github.com/jeffwnelson/sodatracker/internal/domain"
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
	svc := service.New(st, testJWTSecret)
	return NewRouter(st, svc, "test-key", testJWTSecret, "")
}

func setupWithStore(t *testing.T) (http.Handler, *sqlite.SQLiteStore) {
	t.Helper()
	st, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	svc := service.New(st, testJWTSecret)
	return NewRouter(st, svc, "test-key", testJWTSecret, ""), st
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
	body, _ := json.Marshal(domain.Product{UPC: "123456789012", Name: "Cola", CostPerCan: 0.50})
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
	body, _ := json.Marshal(domain.Product{UPC: "123456789012", Name: "Cola", CostPerCan: 0.50})
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
