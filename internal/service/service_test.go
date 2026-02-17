package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/jeffwnelson/sodatracker/internal/domain"
	"github.com/jeffwnelson/sodatracker/internal/events/local"
	"github.com/jeffwnelson/sodatracker/internal/store/sqlite"
)

func setupService(t *testing.T) (*Service, *sqlite.SQLiteStore) {
	t.Helper()
	st, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	svc := New(st, "test-secret", nil)
	return svc, st
}

func setupServiceWithBus(t *testing.T) (*Service, *sqlite.SQLiteStore, *local.LocalBus) {
	t.Helper()
	st, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	bus := local.New()
	t.Cleanup(func() { bus.Close() })
	svc := New(st, "test-secret", bus)
	return svc, st, bus
}

func TestLogin(t *testing.T) {
	svc, st := setupService(t)
	ctx := context.Background()

	hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	u := &domain.User{ID: "u1", Username: "admin", PasswordHash: string(hash), Role: "admin"}
	if err := st.CreateUser(ctx, u); err != nil {
		t.Fatalf("create user: %v", err)
	}

	// Success
	token, user, err := svc.Login(ctx, "admin", "password123")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if token == "" {
		t.Error("expected non-empty token")
	}
	if user.Username != "admin" {
		t.Errorf("expected admin, got %s", user.Username)
	}

	// Bad password
	_, _, err = svc.Login(ctx, "admin", "wrong")
	if err == nil {
		t.Error("expected error for bad password")
	}

	// Non-existent user
	_, _, err = svc.Login(ctx, "nobody", "password123")
	if err == nil {
		t.Error("expected error for non-existent user")
	}
}

func TestGetDispenseStats(t *testing.T) {
	svc, st := setupService(t)
	ctx := context.Background()

	// Create products
	p1 := &domain.Product{ID: "p1", UPC: "111", Name: "Cola", CostPerCan: 0.50}
	p2 := &domain.Product{ID: "p2", UPC: "222", Name: "Sprite", CostPerCan: 0.45}
	st.CreateProduct(ctx, p1)
	st.CreateProduct(ctx, p2)

	// Create dispenses
	now := time.Now().UTC()
	dispenses := []domain.Dispense{
		{ID: "d1", MachineID: "m1", ProductID: "p1", SlotPosition: 1, SourceType: "gpio", Timestamp: now.Add(-1 * time.Hour)},
		{ID: "d2", MachineID: "m1", ProductID: "p1", SlotPosition: 1, SourceType: "gpio", Timestamp: now.Add(-1 * time.Hour)},
		{ID: "d3", MachineID: "m1", ProductID: "p2", SlotPosition: 2, SourceType: "gpio", Timestamp: now},
	}
	for _, d := range dispenses {
		st.CreateDispense(ctx, &d)
	}

	stats, err := svc.GetDispenseStats(ctx, "m1", now.Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if stats.TotalCount != 3 {
		t.Errorf("expected 3 total, got %d", stats.TotalCount)
	}
	expectedCost := 0.50*2 + 0.45
	if stats.TotalCost < expectedCost-0.01 || stats.TotalCost > expectedCost+0.01 {
		t.Errorf("expected cost ~%.2f, got %.2f", expectedCost, stats.TotalCost)
	}
	if len(stats.TopProducts) != 2 {
		t.Fatalf("expected 2 products, got %d", len(stats.TopProducts))
	}
	if stats.TopProducts[0].ProductName != "Cola" {
		t.Errorf("expected Cola first, got %s", stats.TopProducts[0].ProductName)
	}
	if stats.TopProducts[0].Count != 2 {
		t.Errorf("expected Cola count=2, got %d", stats.TopProducts[0].Count)
	}
}

func TestGetMachineCost(t *testing.T) {
	svc, st := setupService(t)
	ctx := context.Background()

	p := &domain.Product{ID: "p1", UPC: "111", Name: "Cola", CostPerCan: 0.50}
	st.CreateProduct(ctx, p)

	now := time.Now().UTC()
	for i := 0; i < 5; i++ {
		d := &domain.Dispense{
			ID: "d" + string(rune('a'+i)), MachineID: "m1", ProductID: "p1",
			SlotPosition: 1, SourceType: "gpio", Timestamp: now.Add(time.Duration(i) * time.Second),
		}
		st.CreateDispense(ctx, d)
	}

	cost, err := svc.GetMachineCost(ctx, "m1", now.Add(-1*time.Hour))
	if err != nil {
		t.Fatalf("cost: %v", err)
	}
	expected := 5 * 0.50
	if cost < expected-0.01 || cost > expected+0.01 {
		t.Errorf("expected cost=%.2f, got %.2f", expected, cost)
	}
}

func TestEvaluateStockAlert(t *testing.T) {
	svc, st, _ := setupServiceWithBus(t)
	ctx := context.Background()
	now := time.Now().UTC()

	// Setup: machine, product, slot, restock
	st.CreateMachine(ctx, &domain.Machine{ID: "m1", Name: "Test", SlotCount: 2})
	st.CreateProduct(ctx, &domain.Product{ID: "p1", UPC: "111", Name: "Cola"})
	st.SetSlots(ctx, "m1", []domain.Slot{
		{MachineID: "m1", Position: 1, ProductID: "p1", Capacity: 10, LowThreshold: 2},
	})
	st.CreateRestock(ctx, &domain.Restock{
		ID: "r1", MachineID: "m1",
		Slots:     []domain.RestockSlot{{Position: 1, Quantity: 10}},
		Timestamp: now,
	})

	// Dispense 8 times → stock=2 (at threshold)
	for i := 0; i < 8; i++ {
		st.CreateDispense(ctx, &domain.Dispense{
			ID: fmt.Sprintf("d%d", i), MachineID: "m1", ProductID: "p1",
			SlotPosition: 1, SourceType: "gpio", Timestamp: now.Add(time.Duration(i+1) * time.Second),
		})
	}

	// Evaluate: stock=2, threshold=2 → low_stock alert
	if err := svc.EvaluateStockAlert(ctx, "m1", 1); err != nil {
		t.Fatalf("evaluate: %v", err)
	}

	alerts, err := st.ListAlertEvents(ctx, "m1", true, 50)
	if err != nil {
		t.Fatalf("list alerts: %v", err)
	}
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}
	if alerts[0].AlertType != "low_stock" {
		t.Errorf("expected low_stock, got %s", alerts[0].AlertType)
	}

	// Evaluate again — should NOT create duplicate
	if err := svc.EvaluateStockAlert(ctx, "m1", 1); err != nil {
		t.Fatalf("evaluate again: %v", err)
	}
	alerts, _ = st.ListAlertEvents(ctx, "m1", true, 50)
	if len(alerts) != 1 {
		t.Errorf("expected still 1 alert (dedup), got %d", len(alerts))
	}

	// Dispense 2 more → stock=0 → out_of_stock
	for i := 8; i < 10; i++ {
		st.CreateDispense(ctx, &domain.Dispense{
			ID: fmt.Sprintf("d%d", i), MachineID: "m1", ProductID: "p1",
			SlotPosition: 1, SourceType: "gpio", Timestamp: now.Add(time.Duration(i+1) * time.Second),
		})
	}

	if err := svc.EvaluateStockAlert(ctx, "m1", 1); err != nil {
		t.Fatalf("evaluate empty: %v", err)
	}
	alerts, _ = st.ListAlertEvents(ctx, "m1", true, 50)
	if len(alerts) != 2 {
		t.Fatalf("expected 2 alerts (low+empty), got %d", len(alerts))
	}
}

func TestCreateRestockClearsAlerts(t *testing.T) {
	svc, st, _ := setupServiceWithBus(t)
	ctx := context.Background()
	now := time.Now().UTC()

	// Create an alert
	ae := &domain.AlertEvent{
		ID: "ae1", MachineID: "m1", SlotPosition: 1, ProductID: "p1",
		AlertType: "low_stock", CurrentStock: 2, Threshold: 3, Timestamp: now,
	}
	st.CreateAlertEvent(ctx, ae)

	// Restock should clear alerts
	r := &domain.Restock{
		ID: "r1", MachineID: "m1",
		Slots:     []domain.RestockSlot{{Position: 1, Quantity: 10}},
		Timestamp: now.Add(time.Second),
	}
	if err := svc.CreateRestock(ctx, r); err != nil {
		t.Fatalf("restock: %v", err)
	}

	alerts, _ := st.ListAlertEvents(ctx, "m1", true, 50)
	if len(alerts) != 0 {
		t.Errorf("expected 0 active alerts after restock, got %d", len(alerts))
	}
}

func TestGetStockPredictions(t *testing.T) {
	svc, st := setupService(t)
	ctx := context.Background()
	now := time.Now().UTC()

	st.CreateMachine(ctx, &domain.Machine{ID: "m1", Name: "Test", SlotCount: 2})
	st.CreateProduct(ctx, &domain.Product{ID: "p1", UPC: "111", Name: "Cola"})
	st.SetSlots(ctx, "m1", []domain.Slot{
		{MachineID: "m1", Position: 1, ProductID: "p1", Capacity: 10, LowThreshold: 2},
	})
	st.CreateRestock(ctx, &domain.Restock{
		ID: "r1", MachineID: "m1",
		Slots:     []domain.RestockSlot{{Position: 1, Quantity: 10}},
		Timestamp: now.Add(-7 * 24 * time.Hour), // 7 days ago
	})

	// 7 dispenses over 14 day lookback = 0.5/day → 10 stock / 0.5 = 20 days
	for i := 0; i < 7; i++ {
		st.CreateDispense(ctx, &domain.Dispense{
			ID: fmt.Sprintf("d%d", i), MachineID: "m1", ProductID: "p1",
			SlotPosition: 1, SourceType: "gpio",
			Timestamp: now.Add(-time.Duration(i) * 24 * time.Hour),
		})
	}

	preds, err := svc.GetStockPredictions(ctx, "m1", 14)
	if err != nil {
		t.Fatalf("predictions: %v", err)
	}
	if len(preds) != 1 {
		t.Fatalf("expected 1 prediction, got %d", len(preds))
	}

	p := preds[0]
	if p.ProductName != "Cola" {
		t.Errorf("expected Cola, got %s", p.ProductName)
	}
	// Stock = 10 - 7 = 3, rate = 7/14 = 0.5/day, days = 3/0.5 = 6
	if p.CurrentStock != 3 {
		t.Errorf("expected stock=3, got %d", p.CurrentStock)
	}
	if p.DispensesPerDay < 0.49 || p.DispensesPerDay > 0.51 {
		t.Errorf("expected ~0.5 dispenses/day, got %.2f", p.DispensesPerDay)
	}
	if p.EstimatedDaysLeft < 5.9 || p.EstimatedDaysLeft > 6.1 {
		t.Errorf("expected ~6 days left, got %.1f", p.EstimatedDaysLeft)
	}
	if p.PredictedStockout == nil {
		t.Error("expected predicted stockout time")
	}
}
