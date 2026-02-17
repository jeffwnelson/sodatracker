package piagent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
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

	log.Printf("Pi agent started: machine=%s, polling=%dms, %d pins",
		a.cfg.MachineID, a.cfg.PollMs, len(a.pins))

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
			log.Printf("error reading pin %d: %v", mp.pin.Number(), err)
			continue
		}
		// Detect button release: was low (pressed), now high (released)
		if !mp.lastHigh && high {
			log.Printf("dispense detected: slot=%d pin=%d", mp.slot, mp.pin.Number())
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
		log.Printf("failed to send dispense, caching: %v", err)
		if err := a.cache.Store(evt); err != nil {
			log.Printf("failed to cache event: %v", err)
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
		log.Printf("error reading pending events: %v", err)
		return
	}
	for _, evt := range events {
		if err := a.sendDispense(evt); err != nil {
			log.Printf("retry failed for event %d: %v", evt.ID, err)
			return // stop retrying on first failure
		}
		if err := a.cache.Remove(evt.ID); err != nil {
			log.Printf("error removing cached event %d: %v", evt.ID, err)
		} else {
			log.Printf("retried event %d successfully", evt.ID)
		}
	}
}
