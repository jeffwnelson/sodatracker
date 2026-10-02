package slackconf

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/jeffwnelson/sodatracker/internal/events"
	"github.com/jeffwnelson/sodatracker/internal/events/local"
	"github.com/jeffwnelson/sodatracker/internal/notify"
	"github.com/jeffwnelson/sodatracker/internal/slacksocket"
	"github.com/jeffwnelson/sodatracker/internal/store/sqlite"
)

func newStore(t *testing.T) *sqlite.SQLiteStore {
	t.Helper()
	st, err := sqlite.New(filepath.Join(t.TempDir(), "s.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

// fakeSocket records which tokens were run and lets the test report state.
type fakeSocket struct {
	mu      sync.Mutex
	tokens  []string
	states  []func(bool, error)
	running int
}

func (f *fakeSocket) run(ctx context.Context, token string, _ slacksocket.Handler, onState func(bool, error)) {
	f.mu.Lock()
	f.tokens = append(f.tokens, token)
	f.states = append(f.states, onState)
	f.running++
	f.mu.Unlock()
	<-ctx.Done()
	f.mu.Lock()
	f.running--
	f.mu.Unlock()
}

func (f *fakeSocket) snapshot() ([]string, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.tokens...), f.running
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	for i := 0; i < 200; i++ {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not met")
}

func TestSeedsFromEnvOnce(t *testing.T) {
	ctx := context.Background()
	st := newStore(t)
	m, err := New(ctx, st, nil, "https://hooks.slack.com/services/T/B/x", "")
	if err != nil {
		t.Fatal(err)
	}
	if s := m.Status(); !s.WebhookSet || s.AppTokenSet || s.Commands != CommandsOff {
		t.Fatalf("status = %+v", s)
	}
	// Cleared in the UI, the env doesn't bring it back on the next start.
	empty := ""
	if _, err := m.Configure(ctx, Update{WebhookURL: &empty}); err != nil {
		t.Fatal(err)
	}
	m2, err := New(ctx, st, nil, "https://hooks.slack.com/services/T/B/x", "")
	if err != nil {
		t.Fatal(err)
	}
	if m2.Status().WebhookSet {
		t.Fatal("env re-seeded a cleared webhook")
	}
}

func TestConfigureValidates(t *testing.T) {
	m, err := New(context.Background(), newStore(t), nil, "", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range []Update{
		{WebhookURL: ptr("http://hooks.slack.com/services/x")},
		{WebhookURL: ptr("https://example.com/hook")},
		{AppToken: ptr("xoxb-bot-token")},
	} {
		var inv *InvalidError
		if _, err := m.Configure(context.Background(), u); !errors.As(err, &inv) {
			t.Errorf("Configure(%v) = %v, want InvalidError", u, err)
		}
	}
	var inv *InvalidError
	if err := m.SendTest(context.Background()); !errors.As(err, &inv) {
		t.Errorf("SendTest without a webhook = %v", err)
	}
}

// A new webhook applies to the next alert without a restart.
func TestWebhookSwapsLive(t *testing.T) {
	ctx := context.Background()
	got := make(chan string, 4)
	hook := func(name string) *httptest.Server {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var m notify.Message
			json.NewDecoder(r.Body).Decode(&m)
			got <- name + ": " + m.Text
		}))
		t.Cleanup(srv.Close)
		return srv
	}
	a, b := hook("a"), hook("b")

	m, err := New(ctx, newStore(t), nil, "", "")
	if err != nil {
		t.Fatal(err)
	}
	// Tests can't use hooks.slack.com, so set the client directly the way
	// Configure would after validating.
	m.mu.Lock()
	m.webhook, m.client = a.URL, notify.NewSlack(a.URL)
	m.mu.Unlock()

	bus := local.New()
	t.Cleanup(func() { bus.Close() })
	notify.RegisterSlackSource(bus, nil, m.Client)
	bus.Publish(ctx, events.Event{Type: events.EventBackupRecovered, Data: map[string]any{}})
	if v := <-got; v != "a: Backups recovered" {
		t.Fatalf("first alert went to %q", v)
	}

	m.mu.Lock()
	m.webhook, m.client = b.URL, notify.NewSlack(b.URL)
	m.mu.Unlock()
	bus.Publish(ctx, events.Event{Type: events.EventBackupRecovered, Data: map[string]any{}})
	if v := <-got; v != "b: Backups recovered" {
		t.Fatalf("second alert went to %q", v)
	}
	if err := m.SendTest(ctx); err != nil {
		t.Fatal(err)
	}
	if v := <-got; v[:2] != "b:" {
		t.Fatalf("test message went to %q", v)
	}
}

// Changing the token restarts Socket Mode on the new one; clearing stops it.
func TestAppTokenRestartsCommands(t *testing.T) {
	ctx := context.Background()
	m, err := New(ctx, newStore(t), nil, "", "")
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeSocket{}
	m.runCommands = f.run

	s, err := m.Configure(ctx, Update{AppToken: ptr("xapp-one")})
	if err != nil || s.Commands != CommandsConnecting || !s.AppTokenSet {
		t.Fatalf("status = %+v, %v", s, err)
	}
	waitFor(t, func() bool { _, n := f.snapshot(); return n == 1 })
	f.states[0](true, nil)
	if s := m.Status(); s.Commands != CommandsConnected {
		t.Fatalf("after hello: %+v", s)
	}

	m.Configure(ctx, Update{AppToken: ptr("xapp-two")})
	waitFor(t, func() bool { tokens, n := f.snapshot(); return n == 1 && len(tokens) == 2 })
	// The replaced client's late report is ignored.
	f.states[0](false, errors.New("closed"))
	f.states[1](false, errors.New("invalid_auth"))
	if s := m.Status(); s.Commands != CommandsError || s.CommandsError != "invalid_auth" {
		t.Fatalf("after bad token: %+v", s)
	}

	m.Configure(ctx, Update{AppToken: ptr("")})
	waitFor(t, func() bool { _, n := f.snapshot(); return n == 0 })
	if s := m.Status(); s.Commands != CommandsOff || s.AppTokenSet {
		t.Fatalf("after clearing: %+v", s)
	}
}

func ptr(s string) *string { return &s }
