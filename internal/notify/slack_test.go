package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jeffwnelson/sodatracker/internal/domain"
	"github.com/jeffwnelson/sodatracker/internal/events"
	"github.com/jeffwnelson/sodatracker/internal/events/local"
	"github.com/jeffwnelson/sodatracker/internal/store/sqlite"
)

// fakeSlack stands in for the incoming webhook and records what it receives.
func fakeSlack(t *testing.T, got chan<- Message) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var m Message
		if err := json.Unmarshal(body, &m); err != nil {
			t.Errorf("slack received invalid JSON: %v", err)
		}
		got <- m
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestLowStockEventPostsToSlack(t *testing.T) {
	received := make(chan Message, 1)
	srv := fakeSlack(t, received)

	st, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	ctx := context.Background()
	st.CreateMachine(ctx, &domain.Machine{ID: "m1", Name: "Break Room", SlotCount: 5})
	st.CreateProduct(ctx, &domain.Product{ID: "p1", Name: "Diet Coke"})

	bus := local.New()
	t.Cleanup(func() { bus.Close() })
	RegisterSlack(bus, st, NewSlack(srv.URL))

	bus.Publish(ctx, events.Event{
		Type: events.EventLowStock,
		Data: map[string]any{
			"machineId": "m1", "productId": "p1",
			"slotPosition": 3, "currentStock": 2, "threshold": 3,
		},
	})

	select {
	case m := <-received:
		// The fallback text is what shows in push notifications, so it must
		// name the product and machine on its own.
		if !strings.Contains(m.Text, "Diet Coke") || !strings.Contains(m.Text, "Break Room") {
			t.Errorf("expected product and machine in fallback text, got %q", m.Text)
		}
		if len(m.Blocks) == 0 {
			t.Error("expected rich blocks alongside the fallback text")
		}
	default:
		t.Fatal("no message reached Slack")
	}
}

// A Slack outage must not break dispense handling.
func TestSlackFailureDoesNotBreakPublish(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no_service", http.StatusGone)
	}))
	t.Cleanup(srv.Close)

	st, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	bus := local.New()
	t.Cleanup(func() { bus.Close() })
	RegisterSlack(bus, st, NewSlack(srv.URL))

	err = bus.Publish(context.Background(), events.Event{
		Type: events.EventOutOfStock,
		Data: map[string]any{"machineId": "m1", "slotPosition": 1},
	})
	if err != nil {
		t.Errorf("a failing Slack webhook must not fail the event publish, got %v", err)
	}
}

// An unset webhook URL must leave the whole integration inert.
func TestNilClientIsNoOp(t *testing.T) {
	if c := NewSlack(""); c != nil {
		t.Fatal("expected nil client for a blank webhook URL")
	}
	if err := NewSlack("").Post(context.Background(), Message{Text: "x"}); err != nil {
		t.Errorf("posting on a nil client should be a no-op, got %v", err)
	}
}

func TestBackupEventsPostToSlack(t *testing.T) {
	received := make(chan Message, 2)
	srv := fakeSlack(t, received)
	bus := local.New()
	t.Cleanup(func() { bus.Close() })
	RegisterSlack(bus, nil, NewSlack(srv.URL))

	ctx := context.Background()
	bus.Publish(ctx, events.Event{Type: events.EventBackupFailed, Data: map[string]any{"error": "push to o/r: GitHub 401: Bad credentials"}})
	bus.Publish(ctx, events.Event{Type: events.EventBackupRecovered, Data: map[string]any{}})

	for _, want := range []string{"Backups failing: push to o/r: GitHub 401: Bad credentials", "Backups recovered"} {
		select {
		case m := <-received:
			if m.Text != want {
				t.Errorf("text = %q, want %q", m.Text, want)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("no Slack message for %q", want)
		}
	}
}
