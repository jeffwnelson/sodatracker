package events

import "context"

type EventType string

const (
	EventDispense     EventType = "dispense"
	EventLowStock     EventType = "low_stock"
	EventOutOfStock   EventType = "out_of_stock"
	EventStockCleared EventType = "stock_cleared"
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
