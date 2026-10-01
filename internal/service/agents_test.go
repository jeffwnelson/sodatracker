package service

import (
	"context"
	"testing"
	"time"

	"github.com/jeffwnelson/sodatracker/internal/domain"
	"github.com/jeffwnelson/sodatracker/internal/events"
)

func status(t *testing.T, svc *Service) domain.AgentStatus {
	t.Helper()
	s, err := svc.agentStatus(context.Background(), "m1")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func beat(t *testing.T, svc *Service) {
	t.Helper()
	if err := svc.RecordAgentHeartbeat(context.Background(), "m1"); err != nil {
		t.Fatal(err)
	}
}

func TestAgentHeartbeat(t *testing.T) {
	svc, _, bus := setupServiceWithBus(t)
	clock := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return clock }

	var online int
	bus.Subscribe(events.EventAgentOnline, func(context.Context, events.Event) error {
		online++
		return nil
	})

	if s := status(t, svc); s.Online || s.LastSeen != nil {
		t.Fatalf("never seen: %+v", s)
	}
	beat(t, svc)
	clock = clock.Add(30 * time.Second)
	beat(t, svc)
	if online != 1 {
		t.Fatalf("got %d online events for a steady agent, want 1", online)
	}
	if s := status(t, svc); !s.Online {
		t.Fatalf("after beats: %+v", s)
	}

	clock = clock.Add(AgentOfflineAfter + time.Second)
	s := status(t, svc)
	if s.Online || s.LastSeen == nil || !s.LastSeen.Equal(clock.Add(-AgentOfflineAfter-time.Second)) {
		t.Fatalf("after silence: %+v", s)
	}
	beat(t, svc)
	if online != 2 {
		t.Fatalf("got %d online events, want 2 after coming back", online)
	}

	// A new Service over the same store (a server restart) still knows.
	if s, _ := New(svc.store, "", nil).agentStatus(context.Background(), "m1"); s.LastSeen == nil {
		t.Fatal("status lost across restart")
	}
}
