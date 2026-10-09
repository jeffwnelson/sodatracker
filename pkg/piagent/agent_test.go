package piagent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/jeffwnelson/sodatracker/pkg/gpio"
)

func TestAgentReportsOneDispensePerBouncyVend(t *testing.T) {
	var mu sync.Mutex
	var got []dispensePayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p dispensePayload
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			t.Errorf("decode: %v", err)
		}
		mu.Lock()
		got = append(got, p)
		mu.Unlock()
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	cfg := &Config{
		APIURL:      srv.URL,
		MachineID:   "machine-1",
		PollMs:      20,
		MinClosedMs: 100,
		MinOpenMs:   500,
		DBPath:      filepath.Join(t.TempDir(), "cache.db"),
		Pins:        []PinConfig{{Pin: 22, Slot: 5}},
	}
	mock := gpio.NewMockProvider()
	a, err := New(cfg, mock)
	if err != nil {
		t.Fatal(err)
	}
	defer a.cache.Close()

	base := time.Date(2026, 9, 30, 13, 0, 0, 0, time.UTC)
	clock := base
	a.now = func() time.Time { return clock }
	step := func(ms int, high bool) {
		mock.SetPin(22, high)
		for end := clock.Add(time.Duration(ms) * time.Millisecond); clock.Before(end); clock = clock.Add(20 * time.Millisecond) {
			a.poll()
		}
	}

	step(1000, true)  // idle
	step(400, false)  // cam closes the contact
	step(100, true)   // bounce open
	step(1060, false) // closed again
	step(1000, true)  // vend over

	// Polling only caches; the sender delivers.
	if len(got) != 0 {
		t.Fatalf("poll sent %d dispenses itself", len(got))
	}
	a.drain(context.Background())

	if len(got) != 1 {
		t.Fatalf("got %d dispenses, want 1: %+v", len(got), got)
	}
	if got[0].SlotPosition != 5 || got[0].MachineID != "machine-1" || got[0].ID == "" {
		t.Errorf("unexpected payload %+v", got[0])
	}
	if want := base.Add(time.Second); !got[0].Timestamp.Equal(want) {
		t.Errorf("timestamp %v, want closure start %v", got[0].Timestamp, want)
	}
}

func TestAgentHeartbeats(t *testing.T) {
	beats := make(chan string, 10)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/agents/heartbeat" || r.Header.Get("X-API-Key") != "k" {
			t.Errorf("unexpected request %s key=%q", r.URL.Path, r.Header.Get("X-API-Key"))
		}
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		beats <- body["machineId"]
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	a, err := New(&Config{
		APIURL: srv.URL, APIKey: "k", MachineID: "machine-1",
		PollMs: 20, RetryMs: 60000, HeartbeatMs: 50,
		DBPath: filepath.Join(t.TempDir(), "cache.db"),
	}, gpio.NewMockProvider())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go a.Run(ctx)

	// One at startup, then one per interval.
	for i := 0; i < 3; i++ {
		select {
		case id := <-beats:
			if id != "machine-1" {
				t.Fatalf("heartbeat for %q", id)
			}
		case <-time.After(time.Second):
			t.Fatalf("got %d heartbeats, want 3", i)
		}
	}
}

// newTestAgent returns an agent that sends to handler, with no pins.
func newTestAgent(t *testing.T, handler http.HandlerFunc) *Agent {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	a, err := New(&Config{
		APIURL: srv.URL, MachineID: "machine-1",
		DBPath: filepath.Join(t.TempDir(), "cache.db"),
	}, gpio.NewMockProvider())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.cache.Close() })
	return a
}

func pendingCount(t *testing.T, a *Agent) int {
	t.Helper()
	evts, err := a.cache.Pending(sendBatch)
	if err != nil {
		t.Fatal(err)
	}
	return len(evts)
}

// A failed send keeps the dispense, and the resend carries the same ID so the
// server can tell it from a new can.
func TestAgentRetriesWithSameID(t *testing.T) {
	var ids []string
	fail := true
	a := newTestAgent(t, func(w http.ResponseWriter, r *http.Request) {
		var p dispensePayload
		json.NewDecoder(r.Body).Decode(&p)
		ids = append(ids, p.ID)
		if fail {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusCreated)
	})

	a.handleDispense(3, time.Now())
	a.drain(context.Background())
	if n := pendingCount(t, a); n != 1 {
		t.Fatalf("pending after failure = %d, want 1", n)
	}

	fail = false
	a.drain(context.Background())
	if n := pendingCount(t, a); n != 0 {
		t.Fatalf("pending after success = %d, want 0", n)
	}
	if len(ids) != 2 || ids[0] == "" || ids[0] != ids[1] {
		t.Fatalf("sent ids %q, want the same non-empty id twice", ids)
	}
}

// A dispense the API rejects is dropped instead of blocking the ones after it.
func TestAgentDropsRejectedDispense(t *testing.T) {
	var slots []int
	a := newTestAgent(t, func(w http.ResponseWriter, r *http.Request) {
		var p dispensePayload
		json.NewDecoder(r.Body).Decode(&p)
		slots = append(slots, p.SlotPosition)
		if p.SlotPosition == 1 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusCreated)
	})

	a.handleDispense(1, time.Now())
	a.handleDispense(2, time.Now())
	a.drain(context.Background())

	if n := pendingCount(t, a); n != 0 {
		t.Fatalf("pending = %d, want 0", n)
	}
	if len(slots) != 2 || slots[0] != 1 || slots[1] != 2 {
		t.Fatalf("sent slots %v, want [1 2] in order", slots)
	}
}

// Events cached by an older agent have no ID; they get one that sticks.
func TestCacheAssignsIDToLegacyEvents(t *testing.T) {
	a := newTestAgent(t, func(w http.ResponseWriter, r *http.Request) {})
	if _, err := a.cache.db.Exec(`INSERT INTO pending_events (payload, created_at) VALUES (?, ?)`,
		`{"ID":0,"MachineID":"machine-1","Slot":4,"Timestamp":"2026-10-01T12:00:00Z"}`, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := a.cache.db.Exec(`INSERT INTO pending_events (payload, created_at) VALUES ('not json', ?)`, time.Now()); err != nil {
		t.Fatal(err)
	}

	first, err := a.cache.Pending(sendBatch)
	if err != nil {
		t.Fatal(err)
	}
	second, err := a.cache.Pending(sendBatch)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("got %d then %d events, want 1 and 1 (bad row dropped)", len(first), len(second))
	}
	if first[0].EventID == "" || first[0].EventID != second[0].EventID {
		t.Fatalf("ids %q then %q, want one stable id", first[0].EventID, second[0].EventID)
	}
}
