package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/jeffwnelson/sodatracker/internal/auth"
	"github.com/jeffwnelson/sodatracker/internal/events"
)

// streamedEvents are the bus events forwarded to dashboards. The stock and
// agent events carry a machineId, and the browser refetches that machine's
// data when one arrives, so the stream never has to carry the data itself.
// Backup events carry none; the browser refetches backup status on those.
var streamedEvents = []events.EventType{
	events.EventDispense,
	events.EventLowStock,
	events.EventOutOfStock,
	events.EventStockCleared,
	events.EventAgentOnline,
	events.EventBackupFailed,
	events.EventBackupRecovered,
}

// streamKeepalive is short enough that proxies don't close an idle stream.
const streamKeepalive = 20 * time.Second

// broker fans bus events out to the open event streams. The bus calls
// subscribers synchronously from the request that published, so publish
// never blocks: a client whose buffer is full misses the event, and its next
// event or the dashboard's slow poll catches it up.
type broker struct {
	mu      sync.Mutex
	clients map[chan events.Event]struct{}
}

func newBroker(bus events.EventBus) *broker {
	b := &broker{clients: map[chan events.Event]struct{}{}}
	for _, t := range streamedEvents {
		bus.Subscribe(t, b.publish)
	}
	return b
}

func (b *broker) publish(_ context.Context, evt events.Event) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.clients {
		select {
		case ch <- evt:
		default:
		}
	}
	return nil
}

func (b *broker) add() chan events.Event {
	ch := make(chan events.Event, 16)
	b.mu.Lock()
	b.clients[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *broker) remove(ch chan events.Event) {
	b.mu.Lock()
	delete(b.clients, ch)
	b.mu.Unlock()
}

// streamEvents godoc
// @Summary      Live event stream
// @Description  Server-Sent Events stream of dispense, stock alert, restock and agent events. Each event's data is the bus event JSON, including machineId. A JWT stream ends when the token expires.
// @Tags         System
// @Produce      text/event-stream
// @Success      200
// @Failure      401  {object}  ErrorResponse
// @Security     ApiKeyAuth
// @Security     BearerAuth
// @Router       /events [get]
func (h *handlers) streamEvents(w http.ResponseWriter, r *http.Request) {
	rc := http.NewResponseController(w)
	// The server's WriteTimeout would cut the stream off after 15s.
	if err := rc.SetWriteDeadline(time.Time{}); err != nil {
		slog.Debug("event stream: cannot clear write deadline", "error", err)
	}

	// A JWT is checked once, at connect, so end the stream when it expires;
	// the browser reconnects and gets a 401 if it hasn't logged in again.
	var expired <-chan time.Time
	if c := auth.ClaimsFromContext(r.Context()); c != nil && c.ExpiresAt != nil {
		t := time.NewTimer(time.Until(c.ExpiresAt.Time))
		defer t.Stop()
		expired = t.C
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	ch := h.broker.add()
	defer h.broker.remove(ch)

	keepalive := time.NewTicker(streamKeepalive)
	defer keepalive.Stop()

	send := func(chunk string) bool {
		if _, err := fmt.Fprint(w, chunk); err != nil {
			return false
		}
		return rc.Flush() == nil
	}
	if !send(": connected\n\n") {
		return
	}
	for {
		select {
		case <-r.Context().Done():
			return
		case <-h.streamsDone:
			return
		case <-expired:
			return
		case <-keepalive.C:
			if !send(": keepalive\n\n") {
				return
			}
		case evt := <-ch:
			data, err := json.Marshal(evt)
			if err != nil {
				continue
			}
			if !send(fmt.Sprintf("event: %s\ndata: %s\n\n", evt.Type, data)) {
				return
			}
		}
	}
}

// agentHeartbeat godoc
// @Summary      Pi agent heartbeat
// @Description  The Pi agent calls this every 30s so dashboards can show when it goes offline.
// @Tags         Dispenses
// @Accept       json
// @Param        heartbeat  body  HeartbeatRequest  true  "Heartbeat"
// @Success      204
// @Failure      400  {object}  ErrorResponse
// @Failure      401  {object}  ErrorResponse
// @Failure      404  {object}  ErrorResponse
// @Security     ApiKeyAuth
// @Router       /agents/heartbeat [post]
func (h *handlers) agentHeartbeat(w http.ResponseWriter, r *http.Request) {
	var req HeartbeatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.MachineID == "" {
		writeErr(w, http.StatusBadRequest, "machineId is required")
		return
	}
	m, err := h.store.GetMachine(r.Context(), req.MachineID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if m == nil {
		writeErr(w, http.StatusNotFound, "machine not found")
		return
	}
	if err := h.svc.RecordAgentHeartbeat(r.Context(), req.MachineID); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
