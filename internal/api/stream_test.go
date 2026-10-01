package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jeffwnelson/sodatracker/internal/domain"
	"github.com/jeffwnelson/sodatracker/internal/events/local"
	"github.com/jeffwnelson/sodatracker/internal/service"
	"github.com/jeffwnelson/sodatracker/internal/store/sqlite"
)

// setupStream runs the router on a real listener, since an event stream
// needs one, and seeds machine-1 and the admin user.
func setupStream(t *testing.T) (*httptest.Server, chan struct{}) {
	t.Helper()
	st, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	bus := local.New()
	t.Cleanup(func() { bus.Close() })
	svc := service.New(st, testJWTSecret, bus)
	seedTestUser(t, st)
	if err := st.CreateMachine(context.Background(), &domain.Machine{ID: "machine-1", Name: "Test", SlotCount: 1}); err != nil {
		t.Fatalf("machine: %v", err)
	}
	done := make(chan struct{})
	srv := httptest.NewUnstartedServer(NewRouter(st, svc, RouterConfig{
		APIKey: "test-key", JWTSecret: testJWTSecret, Bus: bus, StreamsDone: done,
	}))
	// Like the real server's 15s WriteTimeout, which the stream must outlive.
	srv.Config.WriteTimeout = 100 * time.Millisecond
	srv.Start()
	t.Cleanup(srv.Close)
	return srv, done
}

func post(t *testing.T, url, body string) int {
	t.Helper()
	req, _ := http.NewRequest("POST", url, strings.NewReader(body))
	req.Header.Set("X-API-Key", "test-key")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

func TestEventStream(t *testing.T) {
	srv, done := setupStream(t)

	resp, err := http.Get(srv.URL + "/api/v1/events")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no auth: got %d, want 401", resp.StatusCode)
	}

	token := loginAndGetToken(t, srv.Config.Handler)
	req, _ := http.NewRequest("GET", srv.URL+"/api/v1/events", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("got %d %q", resp.StatusCode, resp.Header.Get("Content-Type"))
	}

	lines := make(chan string)
	go func() {
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			lines <- sc.Text()
		}
		close(lines)
	}()
	next := func() string {
		select {
		case l, ok := <-lines:
			if !ok {
				return "<closed>"
			}
			return l
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for the stream")
			return ""
		}
	}
	if l := next(); l != ": connected" {
		t.Fatalf("first line %q", l)
	}
	next() // blank line ending the comment

	time.Sleep(300 * time.Millisecond) // past the server's WriteTimeout

	if code := post(t, srv.URL+"/api/v1/dispenses", `{"machineId":"machine-1","slotPosition":1}`); code != http.StatusCreated {
		t.Fatalf("dispense: %d", code)
	}
	if l := next(); l != "event: dispense" {
		t.Fatalf("got %q, want the dispense event", l)
	}
	data := strings.TrimPrefix(next(), "data: ")
	var evt struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal([]byte(data), &evt); err != nil || evt.Data["machineId"] != "machine-1" {
		t.Fatalf("event data %q (%v)", data, err)
	}
	next()

	close(done) // server shutdown ends the stream
	if l := next(); l != "<closed>" {
		t.Fatalf("stream still open after shutdown, got %q", l)
	}
}

func TestAgentHeartbeat(t *testing.T) {
	srv, _ := setupStream(t)
	url := srv.URL + "/api/v1/agents/heartbeat"

	agent := func() domain.AgentStatus {
		resp, err := http.Get(srv.URL + "/api/v1/machines/machine-1/status")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var st domain.MachineStatus
		json.NewDecoder(resp.Body).Decode(&st)
		return st.Agent
	}

	if a := agent(); a.Online || a.LastSeen != nil {
		t.Fatalf("before any heartbeat: %+v", a)
	}
	if code := post(t, url, `{"machineId":"nope"}`); code != http.StatusNotFound {
		t.Errorf("unknown machine: %d, want 404", code)
	}
	if code := post(t, url, `{}`); code != http.StatusBadRequest {
		t.Errorf("no machineId: %d, want 400", code)
	}
	req, _ := http.NewRequest("POST", url, bytes.NewReader([]byte(`{"machineId":"machine-1"}`)))
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("no API key: %d, want 401", resp.StatusCode)
	}
	if code := post(t, url, `{"machineId":"machine-1"}`); code != http.StatusNoContent {
		t.Fatalf("heartbeat: %d, want 204", code)
	}
	if a := agent(); !a.Online || a.LastSeen == nil {
		t.Fatalf("after heartbeat: %+v", a)
	}
}
