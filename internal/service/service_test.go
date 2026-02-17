package service

import (
	"context"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/jeffwnelson/sodatracker/internal/domain"
	"github.com/jeffwnelson/sodatracker/internal/store/sqlite"
)

func setupService(t *testing.T) (*Service, *sqlite.SQLiteStore) {
	t.Helper()
	st, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	svc := New(st, "test-secret")
	return svc, st
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
