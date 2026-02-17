package local

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/jeffwnelson/sodatracker/internal/events"
)

func TestPublishSubscribe(t *testing.T) {
	bus := New()
	defer bus.Close()

	var called atomic.Int32
	bus.Subscribe(events.EventDispense, func(ctx context.Context, evt events.Event) error {
		called.Add(1)
		return nil
	})

	if err := bus.Publish(context.Background(), events.Event{Type: events.EventDispense}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if called.Load() != 1 {
		t.Errorf("expected 1 call, got %d", called.Load())
	}
}

func TestMultipleSubscribers(t *testing.T) {
	bus := New()
	defer bus.Close()

	var count atomic.Int32
	bus.Subscribe(events.EventDispense, func(ctx context.Context, evt events.Event) error {
		count.Add(1)
		return nil
	})
	bus.Subscribe(events.EventDispense, func(ctx context.Context, evt events.Event) error {
		count.Add(1)
		return nil
	})

	bus.Publish(context.Background(), events.Event{Type: events.EventDispense})
	if count.Load() != 2 {
		t.Errorf("expected 2 calls, got %d", count.Load())
	}
}

func TestUnsubscribe(t *testing.T) {
	bus := New()
	defer bus.Close()

	var count atomic.Int32
	unsub := bus.Subscribe(events.EventDispense, func(ctx context.Context, evt events.Event) error {
		count.Add(1)
		return nil
	})

	bus.Publish(context.Background(), events.Event{Type: events.EventDispense})
	if count.Load() != 1 {
		t.Errorf("expected 1 call, got %d", count.Load())
	}

	unsub()
	bus.Publish(context.Background(), events.Event{Type: events.EventDispense})
	if count.Load() != 1 {
		t.Errorf("expected still 1 call after unsub, got %d", count.Load())
	}
}

func TestPublishNoSubscribers(t *testing.T) {
	bus := New()
	defer bus.Close()

	// Should not panic or error
	if err := bus.Publish(context.Background(), events.Event{Type: events.EventDispense}); err != nil {
		t.Fatalf("publish with no subscribers: %v", err)
	}
}

func TestDifferentEventTypes(t *testing.T) {
	bus := New()
	defer bus.Close()

	var dispenseCount, lowStockCount atomic.Int32
	bus.Subscribe(events.EventDispense, func(ctx context.Context, evt events.Event) error {
		dispenseCount.Add(1)
		return nil
	})
	bus.Subscribe(events.EventLowStock, func(ctx context.Context, evt events.Event) error {
		lowStockCount.Add(1)
		return nil
	})

	bus.Publish(context.Background(), events.Event{Type: events.EventDispense})
	bus.Publish(context.Background(), events.Event{Type: events.EventLowStock})

	if dispenseCount.Load() != 1 {
		t.Errorf("dispense: expected 1, got %d", dispenseCount.Load())
	}
	if lowStockCount.Load() != 1 {
		t.Errorf("low_stock: expected 1, got %d", lowStockCount.Load())
	}
}
