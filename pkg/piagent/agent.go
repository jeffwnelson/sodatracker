package piagent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/jeffwnelson/sodatracker/pkg/gpio"
)

type Agent struct {
	cfg      *Config
	provider gpio.Provider
	cache    *Cache
	client   *http.Client
	pins     []monitoredPin
}

type monitoredPin struct {
	pin      gpio.Pin
	slot     int
	lastHigh bool
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
	}
	for _, pc := range cfg.Pins {
		pin, err := provider.OpenInput(pc.Pin)
		if err != nil {
			return nil, fmt.Errorf("open pin %d: %w", pc.Pin, err)
		}
		a.pins = append(a.pins, monitoredPin{
			pin:      pin,
			slot:     pc.Slot,
			lastHigh: true, // assume not pressed initially
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

	slog.Info("Pi agent started", "machine", a.cfg.MachineID, "pollMs", a.cfg.PollMs, "pins", len(a.pins))

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
	for i := range a.pins {
		mp := &a.pins[i]
		high, err := mp.pin.Read()
		if err != nil {
			slog.Error("error reading pin", "pin", mp.pin.Number(), "error", err)
			continue
		}
		// Detect button release: was low (pressed), now high (released)
		if !mp.lastHigh && high {
			slog.Info("dispense detected", "slot", mp.slot, "pin", mp.pin.Number())
			a.handleDispense(mp.slot)
		}
		mp.lastHigh = high
	}
}

func (a *Agent) handleDispense(slot int) {
	evt := CachedEvent{
		MachineID: a.cfg.MachineID,
		Slot:      slot,
		Timestamp: time.Now().UTC(),
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
