package store

import (
	"context"
	"time"

	"github.com/jeffwnelson/sodatracker/internal/domain"
)

// Store defines the persistence interface for SodaTracker.
type Store interface {
	// Products
	CreateProduct(ctx context.Context, p *domain.Product) error
	GetProduct(ctx context.Context, id string) (*domain.Product, error)
	GetProductByUPC(ctx context.Context, upc string) (*domain.Product, error)
	ListProducts(ctx context.Context) ([]domain.Product, error)

	// Machines
	CreateMachine(ctx context.Context, m *domain.Machine) error
	GetMachine(ctx context.Context, id string) (*domain.Machine, error)
	ListMachines(ctx context.Context) ([]domain.Machine, error)

	// Slots
	SetSlots(ctx context.Context, machineID string, slots []domain.Slot) error
	GetSlots(ctx context.Context, machineID string) ([]domain.Slot, error)

	// Dispenses
	CreateDispense(ctx context.Context, d *domain.Dispense) error
	ListDispenses(ctx context.Context, machineID string, since time.Time, limit int) ([]domain.Dispense, error)
	CountDispensesSince(ctx context.Context, machineID string, slotPosition int, since time.Time) (int, error)

	// Restocks
	CreateRestock(ctx context.Context, r *domain.Restock) error
	GetLastRestock(ctx context.Context, machineID string, slotPosition int) (*domain.Restock, *domain.RestockSlot, error)
	ListRestocks(ctx context.Context, machineID string, limit int) ([]domain.Restock, error)

	// Users
	CreateUser(ctx context.Context, u *domain.User) error
	GetUserByUsername(ctx context.Context, username string) (*domain.User, error)

	// Lifecycle
	Close() error
}
