package service

import (
	"context"
	"time"

	"github.com/jeffwnelson/sodatracker/internal/domain"
	"github.com/jeffwnelson/sodatracker/internal/events"
)

// AgentOfflineAfter is how long a machine's Pi agent can go without a
// heartbeat before it counts as offline. The agent beats every 30s, so this
// tolerates two missed beats.
const AgentOfflineAfter = 90 * time.Second

// RecordAgentHeartbeat extends the agent's current session, or starts a new
// one if the last beat was too long ago, and publishes EventAgentOnline when
// it does. Sessions are stored, so uptime history and status survive a
// server restart.
func (s *Service) RecordAgentHeartbeat(ctx context.Context, machineID string) error {
	now := s.now().UTC()
	last, err := s.store.LatestAgentSession(ctx, machineID)
	if err != nil {
		return err
	}
	if last != nil && now.Sub(last.LastSeen) <= AgentOfflineAfter {
		return s.store.TouchAgentSession(ctx, last.ID, now)
	}
	if err := s.store.StartAgentSession(ctx, machineID, now); err != nil {
		return err
	}
	if s.bus != nil {
		s.bus.Publish(ctx, events.Event{
			Type:      events.EventAgentOnline,
			Timestamp: now.Format(time.RFC3339),
			Data:      map[string]any{"machineId": machineID},
		})
	}
	return nil
}

func (s *Service) agentStatus(ctx context.Context, machineID string) (domain.AgentStatus, error) {
	last, err := s.store.LatestAgentSession(ctx, machineID)
	if err != nil || last == nil {
		return domain.AgentStatus{}, err
	}
	seen := last.LastSeen.UTC()
	return domain.AgentStatus{
		Online:   s.now().Sub(seen) <= AgentOfflineAfter,
		LastSeen: &seen,
	}, nil
}
