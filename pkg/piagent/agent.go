package piagent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

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
}

type monitoredPin struct {
	pin      gpio.Pin
	slot     int
	debounce *debouncer
}

type dispensePayload struct {
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

	retryTicker := time.NewTicker(retryInterval)
	defer retryTicker.Stop()

	// Heartbeats run on their own goroutine so a slow server never stalls
	// GPIO polling.
	go a.heartbeatLoop(ctx, time.Duration(a.cfg.HeartbeatMs)*time.Millisecond)

	slog.Info("Pi agent started", "version", buildinfo.Version, "machine", a.cfg.MachineID, "pollMs", a.cfg.PollMs,
		"minClosedMs", a.cfg.MinClosedMs, "minOpenMs", a.cfg.MinOpenMs, "pins", len(a.pins))

	for {
		select {
		case <-ctx.Done():
			a.cache.Close()
			a.provider.Close()
			return nil
		case <-pollTicker.C:
			a.poll()
		case <-retryTicker.C:
			a.retryPending()
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

func (a *Agent) handleDispense(slot int, at time.Time) {
	evt := CachedEvent{
		MachineID: a.cfg.MachineID,
		Slot:      slot,
		Timestamp: at.UTC(),
	}
	if err := a.sendDispense(evt); err != nil {
		slog.Warn("failed to send dispense, caching", "slot", slot, "error", err)
		if err := a.cache.Store(evt); err != nil {
			slog.Error("failed to cache event", "slot", slot, "error", err)
		}
	}
}

func (a *Agent) sendDispense(evt CachedEvent) error {
	payload := dispensePayload{
		MachineID:    evt.MachineID,
		SlotPosition: evt.Slot,
		SourceType:   "gpio",
		Timestamp:    evt.Timestamp,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", a.cfg.APIURL+"/api/v1/dispenses", bytes.NewReader(body))
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

	if resp.StatusCode >= 300 {
		return fmt.Errorf("API returned %d", resp.StatusCode)
	}
	return nil
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
	if resp.StatusCode >= 300 {
		return fmt.Errorf("API returned %d", resp.StatusCode)
	}
	return nil
}

func (a *Agent) retryPending() {
	events, err := a.cache.Pending()
	if err != nil {
		slog.Error("error reading pending events", "error", err)
		return
	}
	for _, evt := range events {
		if err := a.sendDispense(evt); err != nil {
			slog.Warn("retry failed", "eventID", evt.ID, "error", err)
			return // stop retrying on first failure
		}
		if err := a.cache.Remove(evt.ID); err != nil {
			slog.Error("error removing cached event", "eventID", evt.ID, "error", err)
		} else {
			slog.Info("retried event successfully", "eventID", evt.ID)
		}
	}
}
