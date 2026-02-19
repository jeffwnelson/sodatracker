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
	DeleteProduct(ctx context.Context, id string) error

	// Machines
	CreateMachine(ctx context.Context, m *domain.Machine) error
	GetMachine(ctx context.Context, id string) (*domain.Machine, error)
	ListMachines(ctx context.Context) ([]domain.Machine, error)
	DeleteMachine(ctx context.Context, id string) error

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

	// Alert Configs
	CreateAlertConfig(ctx context.Context, ac *domain.AlertConfig) error
	ListAlertConfigs(ctx context.Context, machineID string) ([]domain.AlertConfig, error)
	UpdateAlertConfig(ctx context.Context, ac *domain.AlertConfig) error
	DeleteAlertConfig(ctx context.Context, id string) error

	// Alert Events
	CreateAlertEvent(ctx context.Context, ae *domain.AlertEvent) error
	ListAlertEvents(ctx context.Context, machineID string, activeOnly bool, limit int) ([]domain.AlertEvent, error)
	ClearAlertEvents(ctx context.Context, machineID string, slotPosition int) error
	GetActiveAlert(ctx context.Context, machineID string, slotPosition int, alertType string) (*domain.AlertEvent, error)

	// Lifecycle
	Close() error
}
