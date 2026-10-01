package service

import (
	"context"
	"sync"
	"time"

	"github.com/jeffwnelson/sodatracker/internal/domain"
	"github.com/jeffwnelson/sodatracker/internal/events"
)

// AgentOfflineAfter is how long a machine's Pi agent can go without a
// heartbeat before it shows as offline. The agent beats every 30s, so this
// tolerates two missed beats.
const AgentOfflineAfter = 90 * time.Second

// agentTracker holds each machine's last heartbeat in memory. A restart
// forgets it, but the agent beats again within one interval.
type agentTracker struct {
	mu       sync.Mutex
	lastSeen map[string]time.Time
	now      func() time.Time
}

// beat records a heartbeat and reports whether the agent was offline (or
// never seen) before it.
func (t *agentTracker) beat(machineID string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := t.now()
	prev, seen := t.lastSeen[machineID]
	t.lastSeen[machineID] = now
	return !seen || now.Sub(prev) > AgentOfflineAfter
}

func (t *agentTracker) status(machineID string) domain.AgentStatus {
	t.mu.Lock()
	defer t.mu.Unlock()
	last, seen := t.lastSeen[machineID]
	if !seen {
		return domain.AgentStatus{}
	}
	last = last.UTC()
	return domain.AgentStatus{
		Online:   t.now().Sub(last) <= AgentOfflineAfter,
		LastSeen: &last,
	}
}

// RecordAgentHeartbeat notes that the machine's Pi agent is alive and
// publishes EventAgentOnline when it comes back from offline.
func (s *Service) RecordAgentHeartbeat(ctx context.Context, machineID string) {
	if !s.agents.beat(machineID) || s.bus == nil {
		return
	}
	s.bus.Publish(ctx, events.Event{
		Type:      events.EventAgentOnline,
		Timestamp: s.agents.now().UTC().Format(time.RFC3339),
		Data:      map[string]any{"machineId": machineID},
	})
}
