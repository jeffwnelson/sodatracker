package service

import (
	"context"
	"testing"
	"time"

	"github.com/jeffwnelson/sodatracker/internal/events"
)

func TestAgentHeartbeat(t *testing.T) {
	svc, _, bus := setupServiceWithBus(t)
	clock := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	svc.agents.now = func() time.Time { return clock }

	var online int
	bus.Subscribe(events.EventAgentOnline, func(context.Context, events.Event) error {
		online++
		return nil
	})
	ctx := context.Background()

	if s := svc.agents.status("m1"); s.Online || s.LastSeen != nil {
		t.Fatalf("never seen: %+v", s)
	}
	svc.RecordAgentHeartbeat(ctx, "m1")
	clock = clock.Add(30 * time.Second)
	svc.RecordAgentHeartbeat(ctx, "m1")
	if online != 1 {
		t.Fatalf("got %d online events for a steady agent, want 1", online)
	}
	if s := svc.agents.status("m1"); !s.Online {
		t.Fatalf("after beats: %+v", s)
	}

	clock = clock.Add(AgentOfflineAfter + time.Second)
	s := svc.agents.status("m1")
	if s.Online || s.LastSeen == nil || !s.LastSeen.Equal(clock.Add(-AgentOfflineAfter-time.Second)) {
		t.Fatalf("after silence: %+v", s)
	}
	svc.RecordAgentHeartbeat(ctx, "m1")
	if online != 2 {
		t.Fatalf("got %d online events, want 2 after coming back", online)
	}
}
