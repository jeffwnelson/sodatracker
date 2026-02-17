package local

import (
	"context"
	"log"
	"sync"

	"github.com/jeffwnelson/sodatracker/internal/events"
)

type subscriber struct {
	id      int
	handler events.Handler
}

type LocalBus struct {
	mu          sync.RWMutex
	subscribers map[events.EventType][]subscriber
	nextID      int
	closed      bool
}

func New() *LocalBus {
	return &LocalBus{
		subscribers: make(map[events.EventType][]subscriber),
	}
}

func (b *LocalBus) Publish(ctx context.Context, evt events.Event) error {
	b.mu.RLock()
	subs := b.subscribers[evt.Type]
	// Copy slice under read lock to avoid holding lock during handler calls
	copied := make([]subscriber, len(subs))
	copy(copied, subs)
	b.mu.RUnlock()

	for _, s := range copied {
		if err := s.handler(ctx, evt); err != nil {
			log.Printf("event handler error for %s: %v", evt.Type, err)
		}
	}
	return nil
}

func (b *LocalBus) Subscribe(eventType events.EventType, handler events.Handler) func() {
	b.mu.Lock()
	defer b.mu.Unlock()

	id := b.nextID
	b.nextID++
	b.subscribers[eventType] = append(b.subscribers[eventType], subscriber{id: id, handler: handler})

	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		subs := b.subscribers[eventType]
		for i, s := range subs {
			if s.id == id {
				b.subscribers[eventType] = append(subs[:i], subs[i+1:]...)
				return
			}
		}
	}
}

func (b *LocalBus) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	b.subscribers = make(map[events.EventType][]subscriber)
	return nil
}
