package sqlite

import (
	"context"
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

func TestProductCRUD(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	p := &domain.Product{ID: "p1", UPC: "012345678901", Name: "Cola"}
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
