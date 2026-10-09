package piagent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jeffwnelson/sodatracker/internal/buildinfo"
	"github.com/jeffwnelson/sodatracker/pkg/gpio"
)

type Agent struct {
	cfg      *Config
	provider gpio.Provider
	cache    *Cache
	client   *http.Client
	pins     []monitoredPin
	now      func() time.Time
	// wake nudges the sender when poll has cached a new dispense.
	wake chan struct{}
}

// sendBatch is how many cached dispenses the sender reads at a time.
const sendBatch = 50

type monitoredPin struct {
	pin      gpio.Pin
	slot     int
	debounce *debouncer
}

type dispensePayload struct {
	ID           string    `json:"id"`
	MachineID    string    `json:"machineId"`
	SlotPosition int       `json:"slotPosition"`
	SourceType   string    `json:"sourceType"`
	Timestamp    time.Time `json:"timestamp"`
}

func New(cfg *Config, provider gpio.Provider) (*Agent, error) {
	cache, err := NewCache(cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("init cache: %w", err)
	}
	a := &Agent{
		cfg:      cfg,
		provider: provider,
		cache:    cache,
		client:   &http.Client{Timeout: 10 * time.Second},
		now:      time.Now,
		wake:     make(chan struct{}, 1),
	}
	minClosed := time.Duration(cfg.MinClosedMs) * time.Millisecond
	minOpen := time.Duration(cfg.MinOpenMs) * time.Millisecond
	for _, pc := range cfg.Pins {
		pin, err := provider.OpenInput(pc.Pin)
		if err != nil {
			return nil, fmt.Errorf("open pin %d: %w", pc.Pin, err)
		}
		a.pins = append(a.pins, monitoredPin{
			pin:      pin,
			slot:     pc.Slot,
			debounce: newDebouncer(minClosed, minOpen),
		})
	}
	return a, nil
}

func (a *Agent) Run(ctx context.Context) error {
	pollInterval := time.Duration(a.cfg.PollMs) * time.Millisecond
	retryInterval := time.Duration(a.cfg.RetryMs) * time.Millisecond

	pollTicker := time.NewTicker(pollInterval)
	defer pollTicker.Stop()

	// Heartbeats and dispense delivery run on their own goroutines so a slow
	// server never stalls GPIO polling, which would miss vends.
	go a.heartbeatLoop(ctx, time.Duration(a.cfg.HeartbeatMs)*time.Millisecond)
	sent := make(chan struct{})
	go func() {
		defer close(sent)
		a.sendLoop(ctx, retryInterval)
	}()

	slog.Info("Pi agent started", "version", buildinfo.Version, "machine", a.cfg.MachineID, "pollMs", a.cfg.PollMs,
		"minClosedMs", a.cfg.MinClosedMs, "minOpenMs", a.cfg.MinOpenMs, "pins", len(a.pins))

	for {
		select {
		case <-ctx.Done():
			<-sent
			a.cache.Close()
			a.provider.Close()
			return nil
		case <-pollTicker.C:
			a.poll()
		}
	}
}

func (a *Agent) poll() {
	now := a.now()
	for i := range a.pins {
		mp := &a.pins[i]
		high, err := mp.pin.Read()
		if err != nil {
			slog.Error("error reading pin", "pin", mp.pin.Number(), "error", err)
			continue
		}
		// A vend closes the contact (LOW) and reports once it has stayed
		// open again; see debouncer for the timing rules.
		if vend, closedAt := mp.debounce.update(high, now); vend {
			slog.Info("dispense detected", "slot", mp.slot, "pin", mp.pin.Number(),
				"closedMs", now.Sub(closedAt).Milliseconds())
			a.handleDispense(mp.slot, closedAt)
		}
	}
}

// handleDispense writes the dispense to the cache, which the sender drains.
// Caching first means a crash or power cut between detecting and sending a
// vend can't lose it, and the poll loop never waits on the network.
func (a *Agent) handleDispense(slot int, at time.Time) {
	evt := CachedEvent{
		EventID:   uuid.NewString(),
		MachineID: a.cfg.MachineID,
		Slot:      slot,
		Timestamp: at.UTC(),
	}
	if err := a.cache.Store(evt); err != nil {
		// The cache is unusable; try once directly rather than drop the can.
		slog.Error("failed to cache dispense, sending directly", "slot", slot, "eventID", evt.EventID, "error", err)
		go func() {
			if err := a.sendDispense(context.Background(), evt); err != nil {
				slog.Error("dispense lost", "slot", slot, "eventID", evt.EventID, "error", err)
			}
		}()
		return
	}
	select {
	case a.wake <- struct{}{}:
	default: // a wake is already pending
	}
}

// sendLoop delivers cached dispenses when poll caches one and on every retry
// interval, until ctx ends.
func (a *Agent) sendLoop(ctx context.Context, retryInterval time.Duration) {
	t := time.NewTicker(retryInterval)
	defer t.Stop()
	a.drain(ctx) // anything left from before a restart
	for {
		select {
		case <-ctx.Done():
			return
		case <-a.wake:
		case <-t.C:
		}
		a.drain(ctx)
	}
}

// drain sends cached dispenses oldest first and removes each one the API
// accepts. It stops at the first transient failure so order is kept, and the
// next wake or tick resumes from there. A dispense the API rejects outright
// can never succeed, so it is logged and dropped rather than block the rest.
func (a *Agent) drain(ctx context.Context) {
	for ctx.Err() == nil {
		events, err := a.cache.Pending(sendBatch)
		if err != nil {
			slog.Error("error reading pending events", "error", err)
			return
		}
		for _, evt := range events {
			err := a.sendDispense(ctx, evt)
			var rejected *rejectedError
			switch {
			case err == nil:
			case errors.As(err, &rejected):
				slog.Error("API rejected dispense, dropping it", "eventID", evt.EventID,
					"slot", evt.Slot, "timestamp", evt.Timestamp, "error", err)
			default:
				slog.Warn("failed to send dispense, will retry", "eventID", evt.EventID, "slot", evt.Slot, "error", err)
				return
			}
			if err := a.cache.Remove(evt.ID); err != nil {
				slog.Error("error removing cached event", "eventID", evt.EventID, "error", err)
				return
			}
		}
		if len(events) < sendBatch {
			return
		}
	}
}

// rejectedError is a 4xx response that retrying won't fix.
type rejectedError struct{ status int }

func (e *rejectedError) Error() string { return fmt.Sprintf("API returned %d", e.status) }

func (a *Agent) sendDispense(ctx context.Context, evt CachedEvent) error {
	payload := dispensePayload{
		ID:           evt.EventID,
		MachineID:    evt.MachineID,
		SlotPosition: evt.Slot,
		SourceType:   "gpio",
		Timestamp:    evt.Timestamp,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", a.cfg.APIURL+"/api/v1/dispenses", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", a.cfg.APIKey)

	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body) // lets the connection be reused

	switch code := resp.StatusCode; {
	case code < 300:
		return nil
	// Bad credentials, timeouts and rate limits can clear up; keep those.
	case code >= 400 && code < 500 && code != http.StatusUnauthorized &&
		code != http.StatusForbidden && code != http.StatusRequestTimeout &&
		code != http.StatusTooManyRequests:
		return &rejectedError{status: code}
	default:
		return fmt.Errorf("API returned %d", code)
	}
}

// heartbeatLoop tells the server the agent is alive, at once and then every
// interval, so dashboards can flag a dead agent instead of showing no dispenses.
func (a *Agent) heartbeatLoop(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		if err := a.sendHeartbeat(ctx); err != nil {
			slog.Warn("heartbeat failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (a *Agent) sendHeartbeat(ctx context.Context) error {
	body, _ := json.Marshal(map[string]string{"machineId": a.cfg.MachineID})
	req, err := http.NewRequestWithContext(ctx, "POST", a.cfg.APIURL+"/api/v1/agents/heartbeat", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", a.cfg.APIKey)
	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("API returned %d", resp.StatusCode)
	}
	return nil
}
