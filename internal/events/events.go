package events

import "context"

type EventType string

const (
	EventDispense     EventType = "dispense"
	EventLowStock     EventType = "low_stock"
	EventOutOfStock   EventType = "out_of_stock"
	EventStockCleared EventType = "stock_cleared"
	// EventAgentOnline fires when a machine's Pi agent heartbeats after
	// being offline (or for the first time since the server started).
	EventAgentOnline EventType = "agent_online"
	// EventBackupFailed fires when backups become stale: a snapshot or push
	// failed, or nothing has been pushed in backup.StaleAfter.
	// EventBackupRecovered fires when they stop being stale. Neither carries
	// a machineId.
	EventBackupFailed    EventType = "backup_failed"
	EventBackupRecovered EventType = "backup_recovered"
)

type Event struct {
	Type      EventType      `json:"type"`
	Timestamp string         `json:"timestamp"`
	Data      map[string]any `json:"data"`
}

type Handler func(ctx context.Context, evt Event) error

type EventBus interface {
	Publish(ctx context.Context, evt Event) error
	Subscribe(eventType EventType, handler Handler) (unsubscribe func())
	Close() error
}
