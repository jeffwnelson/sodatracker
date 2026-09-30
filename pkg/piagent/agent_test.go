package piagent

import (
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

	if len(got) != 1 {
		t.Fatalf("got %d dispenses, want 1: %+v", len(got), got)
	}
	if got[0].SlotPosition != 5 || got[0].MachineID != "machine-1" {
		t.Errorf("unexpected payload %+v", got[0])
	}
	if want := base.Add(time.Second); !got[0].Timestamp.Equal(want) {
		t.Errorf("timestamp %v, want closure start %v", got[0].Timestamp, want)
	}
}
