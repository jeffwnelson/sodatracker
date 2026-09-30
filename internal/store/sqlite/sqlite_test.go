package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/jeffwnelson/sodatracker/internal/domain"
)

func testStore(t *testing.T) *SQLiteStore {
	t.Helper()
	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// A database created before products had a color column gains it on open, and
// existing rows read back with no color.
func TestProductColorMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	old, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`CREATE TABLE products (id TEXT PRIMARY KEY, upc TEXT DEFAULT '', name TEXT NOT NULL,
			cost_per_can REAL DEFAULT 0, image_url TEXT DEFAULT '', description TEXT DEFAULT '')`,
		`INSERT INTO products (id, name) VALUES ('p1', 'Diet Coke')`,
	} {
		if _, err := old.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	old.Close()

	s, err := New(path)
	if err != nil {
		t.Fatalf("open migrated store: %v", err)
	}
	defer s.Close()
	ctx := context.Background()
	got, err := s.GetProduct(ctx, "p1")
	if err != nil || got == nil || got.Name != "Diet Coke" || got.Color != "" {
		t.Fatalf("existing product after migration: %+v, %v", got, err)
	}
	got.Color = "#a7a9ac"
	if err := s.CreateProduct(ctx, got); err != nil {
		t.Fatal(err)
	}
	if got, _ = s.GetProduct(ctx, "p1"); got.Color != "#a7a9ac" {
		t.Errorf("color after update: %q", got.Color)
	}
}

func TestProductCRUD(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	p := &domain.Product{ID: "p1", UPC: "012345678901", Name: "Cola", Description: "Classic cola", Color: "#e4002b"}
	if err := s.CreateProduct(ctx, p); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := s.GetProduct(ctx, "p1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "Cola" {
		t.Errorf("expected Cola, got %s", got.Name)
	}
	if got.Color != "#e4002b" {
		t.Errorf("expected color #e4002b, got %q", got.Color)
	}

	got, err = s.GetProductByUPC(ctx, "012345678901")
	if err != nil {
		t.Fatalf("getByUPC: %v", err)
	}
	if got.ID != "p1" {
		t.Errorf("expected p1, got %s", got.ID)
	}

	products, err := s.ListProducts(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(products) != 1 {
		t.Errorf("expected 1 product, got %d", len(products))
	}

	// Delete
	if err := s.DeleteProduct(ctx, "p1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	products, err = s.ListProducts(ctx)
	if err != nil {
		t.Fatalf("list after delete: %v", err)
	}
	if len(products) != 0 {
		t.Errorf("expected 0 products after delete, got %d", len(products))
	}
}

func TestMachineSlotsAndStatus(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	m := &domain.Machine{ID: "m1", Name: "Break Room", SlotCount: 3}
	if err := s.CreateMachine(ctx, m); err != nil {
		t.Fatalf("create machine: %v", err)
	}

	slots := []domain.Slot{
		{MachineID: "m1", Position: 1, ProductID: "p1", Capacity: 10, LowThreshold: 2},
		{MachineID: "m1", Position: 2, ProductID: "p2", Capacity: 10, LowThreshold: 2},
	}
	if err := s.SetSlots(ctx, "m1", slots); err != nil {
		t.Fatalf("set slots: %v", err)
	}

	got, err := s.GetSlots(ctx, "m1")
	if err != nil {
		t.Fatalf("get slots: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 slots, got %d", len(got))
	}

	// Add related data for cascade delete test
	now := time.Now().UTC()
	s.CreateRestock(ctx, &domain.Restock{
		ID: "r1", MachineID: "m1",
		Slots:     []domain.RestockSlot{{Position: 1, Quantity: 10}},
		Timestamp: now,
	})
	s.CreateDispense(ctx, &domain.Dispense{
		ID: "d1", MachineID: "m1", ProductID: "p1", SlotPosition: 1,
		SourceType: "gpio", Timestamp: now,
	})
	s.CreateAlertConfig(ctx, &domain.AlertConfig{
		ID: "ac1", MachineID: "m1", SlotPosition: 1, LowThreshold: 3, Enabled: true,
	})
	s.CreateAlertEvent(ctx, &domain.AlertEvent{
		ID: "ae1", MachineID: "m1", SlotPosition: 1, AlertType: "low_stock",
		CurrentStock: 2, Threshold: 3, Timestamp: now,
	})

	// Delete machine — should cascade
	if err := s.DeleteMachine(ctx, "m1"); err != nil {
		t.Fatalf("delete machine: %v", err)
	}
	machines, err := s.ListMachines(ctx)
	if err != nil {
		t.Fatalf("list machines after delete: %v", err)
	}
	if len(machines) != 0 {
		t.Errorf("expected 0 machines after delete, got %d", len(machines))
	}
	slotsAfter, _ := s.GetSlots(ctx, "m1")
	if len(slotsAfter) != 0 {
		t.Errorf("expected 0 slots after delete, got %d", len(slotsAfter))
	}
	dispenses, _ := s.ListDispenses(ctx, "m1", time.Time{}, 100)
	if len(dispenses) != 0 {
		t.Errorf("expected 0 dispenses after delete, got %d", len(dispenses))
	}
	restocks, _ := s.ListRestocks(ctx, "m1", 100)
	if len(restocks) != 0 {
		t.Errorf("expected 0 restocks after delete, got %d", len(restocks))
	}
	alertConfigs, _ := s.ListAlertConfigs(ctx, "m1")
	if len(alertConfigs) != 0 {
		t.Errorf("expected 0 alert configs after delete, got %d", len(alertConfigs))
	}
	alertEvents, _ := s.ListAlertEvents(ctx, "m1", false, 100)
	if len(alertEvents) != 0 {
		t.Errorf("expected 0 alert events after delete, got %d", len(alertEvents))
	}
}

func TestUserCRUD(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	u := &domain.User{ID: "u1", Username: "admin", PasswordHash: "hash123", Role: "admin"}
	if err := s.CreateUser(ctx, u); err != nil {
		t.Fatalf("create user: %v", err)
	}

	got, err := s.GetUserByUsername(ctx, "admin")
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if got == nil {
		t.Fatal("expected user to exist")
	}
	if got.ID != "u1" {
		t.Errorf("expected id=u1, got %s", got.ID)
	}
	if got.PasswordHash != "hash123" {
		t.Errorf("expected hash=hash123, got %s", got.PasswordHash)
	}

	// Non-existent user
	got, err = s.GetUserByUsername(ctx, "nobody")
	if err != nil {
		t.Fatalf("get non-existent: %v", err)
	}
	if got != nil {
		t.Error("expected nil for non-existent user")
	}
}

func TestAlertConfigCRUD(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// Create
	ac := &domain.AlertConfig{ID: "ac1", MachineID: "m1", SlotPosition: 1, LowThreshold: 3, Enabled: true}
	if err := s.CreateAlertConfig(ctx, ac); err != nil {
		t.Fatalf("create: %v", err)
	}

	// List
	configs, err := s.ListAlertConfigs(ctx, "m1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(configs) != 1 {
		t.Fatalf("expected 1 config, got %d", len(configs))
	}
	if configs[0].LowThreshold != 3 || !configs[0].Enabled {
		t.Errorf("unexpected config: %+v", configs[0])
	}

	// Update
	ac.LowThreshold = 5
	ac.Enabled = false
	if err := s.UpdateAlertConfig(ctx, ac); err != nil {
		t.Fatalf("update: %v", err)
	}
	configs, _ = s.ListAlertConfigs(ctx, "m1")
	if configs[0].LowThreshold != 5 || configs[0].Enabled {
		t.Errorf("update failed: %+v", configs[0])
	}

	// Delete
	if err := s.DeleteAlertConfig(ctx, "ac1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	configs, _ = s.ListAlertConfigs(ctx, "m1")
	if len(configs) != 0 {
		t.Errorf("expected 0 configs after delete, got %d", len(configs))
	}
}

func TestAlertEventCRUD(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	// Create alert event
	ae := &domain.AlertEvent{
		ID: "ae1", MachineID: "m1", SlotPosition: 1, ProductID: "p1",
		AlertType: "low_stock", CurrentStock: 2, Threshold: 3,
		Cleared: false, Timestamp: now,
	}
	if err := s.CreateAlertEvent(ctx, ae); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Get active alert
	got, err := s.GetActiveAlert(ctx, "m1", 1, "low_stock")
	if err != nil {
		t.Fatalf("get active: %v", err)
	}
	if got == nil {
		t.Fatal("expected active alert")
	}
	if got.ID != "ae1" {
		t.Errorf("expected ae1, got %s", got.ID)
	}

	// No active alert for different type
	got, err = s.GetActiveAlert(ctx, "m1", 1, "out_of_stock")
	if err != nil {
		t.Fatalf("get active (other type): %v", err)
	}
	if got != nil {
		t.Error("expected nil for different alert type")
	}

	// List active only
	events, err := s.ListAlertEvents(ctx, "m1", true, 50)
	if err != nil {
		t.Fatalf("list active: %v", err)
	}
	if len(events) != 1 {
		t.Errorf("expected 1 active event, got %d", len(events))
	}

	// Clear
	if err := s.ClearAlertEvents(ctx, "m1", 1); err != nil {
		t.Fatalf("clear: %v", err)
	}

	// Verify cleared
	events, _ = s.ListAlertEvents(ctx, "m1", true, 50)
	if len(events) != 0 {
		t.Errorf("expected 0 active events after clear, got %d", len(events))
	}

	// All events still exist
	events, _ = s.ListAlertEvents(ctx, "m1", false, 50)
	if len(events) != 1 {
		t.Errorf("expected 1 total event, got %d", len(events))
	}
	if !events[0].Cleared {
		t.Error("expected event to be cleared")
	}
}

func TestDispenseAndRestock(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	// Restock slot 1 with 10 items
	r := &domain.Restock{
		ID:        "r1",
		MachineID: "m1",
		Slots:     []domain.RestockSlot{{Position: 1, Quantity: 10}},
		Timestamp: now,
	}
	if err := s.CreateRestock(ctx, r); err != nil {
		t.Fatalf("create restock: %v", err)
	}

	// Dispense 3 times
	for i := 0; i < 3; i++ {
		d := &domain.Dispense{
			ID:           "d" + string(rune('1'+i)),
			MachineID:    "m1",
			ProductID:    "p1",
			SlotPosition: 1,
			SourceType:   "gpio",
			Timestamp:    now.Add(time.Duration(i+1) * time.Second),
		}
		if err := s.CreateDispense(ctx, d); err != nil {
			t.Fatalf("create dispense %d: %v", i, err)
		}
	}

	// Count dispenses since restock
	count, err := s.CountDispensesSince(ctx, "m1", 1, now)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 dispenses, got %d", count)
	}

	// Get last restock
	rs, rsl, err := s.GetLastRestock(ctx, "m1", 1)
	if err != nil {
		t.Fatalf("get last restock: %v", err)
	}
	if rs == nil || rsl == nil {
		t.Fatal("expected restock to exist")
	}
	if rsl.Quantity != 10 {
		t.Errorf("expected quantity 10, got %d", rsl.Quantity)
	}
}
