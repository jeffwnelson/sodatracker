package store

import (
	"context"
	"errors"
	"time"

	"github.com/jeffwnelson/sodatracker/internal/domain"
)

// ErrDuplicate is returned by CreateDispense when a dispense with the same ID
// is already stored, which is how a retried request is recognized.
var ErrDuplicate = errors.New("duplicate")

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
	// CreateDispense stores d, or returns ErrDuplicate if its ID exists.
	CreateDispense(ctx context.Context, d *domain.Dispense) error
	ListDispenses(ctx context.Context, machineID string, since time.Time, limit int) ([]domain.Dispense, error)
	CountDispensesSince(ctx context.Context, machineID string, slotPosition int, since time.Time) (int, error)
	// ListDispensesBetween returns dispenses in [from, to), oldest first. A
	// zero to means no upper bound.
	ListDispensesBetween(ctx context.Context, machineID string, from, to time.Time) ([]domain.Dispense, error)
	// FirstActivity is the machine's earliest dispense or restock, or the
	// zero time if it has neither.
	FirstActivity(ctx context.Context, machineID string) (time.Time, error)

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

	// Agent sessions (heartbeat history), oldest first
	LatestAgentSession(ctx context.Context, machineID string) (*domain.AgentSession, error)
	StartAgentSession(ctx context.Context, machineID string, at time.Time) error
	TouchAgentSession(ctx context.Context, id int64, at time.Time) error
	ListAgentSessions(ctx context.Context, machineID string) ([]domain.AgentSession, error)

	// Admin
	ResetAllData(ctx context.Context) error
	// Snapshot writes a consistent copy of the whole database to dest, which
	// must not exist, leaving out secret settings. Safe while other
	// connections read and write.
	Snapshot(ctx context.Context, dest string) error

	// Settings: app configuration edited from the UI. Secret ones are left
	// out of snapshots. Reset All Data keeps them.
	GetSetting(ctx context.Context, key string) (string, error)
	SetSetting(ctx context.Context, key, value string, secret bool) error

	// Lifecycle
	Close() error
}
