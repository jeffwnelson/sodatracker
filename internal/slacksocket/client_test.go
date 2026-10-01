package slacksocket

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// fakeSlack serves apps.connections.open and a WebSocket that runs script
// against each connection.
func fakeSlack(t *testing.T, token string, script func(ctx context.Context, conn *websocket.Conn)) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var dials atomic.Int32
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/api/apps.connections.open", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+token {
			json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "invalid_auth"})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "url": "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"})
	})
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		dials.Add(1)
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Errorf("accept: %v", err)
			return
		}
		defer conn.CloseNow()
		script(r.Context(), conn)
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, &dials
}

func writeJSON(t *testing.T, ctx context.Context, conn *websocket.Conn, v any) {
	t.Helper()
	data, _ := json.Marshal(v)
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		t.Errorf("write: %v", err)
	}
}

func TestSlashCommandIsAnsweredInTheAck(t *testing.T) {
	acks := make(chan map[string]any, 1)
	srv, _ := fakeSlack(t, "xapp-test", func(ctx context.Context, conn *websocket.Conn) {
		writeJSON(t, ctx, conn, map[string]any{"type": "hello"})
		writeJSON(t, ctx, conn, map[string]any{
			"type":        "slash_commands",
			"envelope_id": "env-1",
			"payload":     map[string]any{"command": "/soda", "text": "top"},
		})
		_, data, err := conn.Read(ctx)
		if err != nil {
			t.Errorf("read ack: %v", err)
			return
		}
		var a map[string]any
		json.Unmarshal(data, &a)
		acks <- a
		<-ctx.Done()
	})

	var gotCommand, gotText string
	c := New("xapp-test", func(ctx context.Context, command, text string) any {
		gotCommand, gotText = command, text
		return map[string]string{"response_type": "in_channel", "text": "Coca-Cola — 3"}
	})
	c.apiBase = srv.URL + "/api"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Run(ctx)

	select {
	case a := <-acks:
		if a["envelope_id"] != "env-1" {
			t.Errorf("ack envelope_id = %v, want env-1", a["envelope_id"])
		}
		payload, _ := a["payload"].(map[string]any)
		if payload["text"] != "Coca-Cola — 3" || payload["response_type"] != "in_channel" {
			t.Errorf("ack payload = %v", a["payload"])
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no ack received")
	}
	if gotCommand != "/soda" || gotText != "top" {
		t.Errorf("handler got %q %q, want /soda top", gotCommand, gotText)
	}
}

// Slack rotates connections with a "disconnect" envelope; the client must
// come straight back rather than wait out a backoff.
func TestReconnectsAfterDisconnect(t *testing.T) {
	srv, dials := fakeSlack(t, "xapp-test", func(ctx context.Context, conn *websocket.Conn) {
		// The client drops the connection as soon as it reads "disconnect"
		// and keeps redialling after the test returns, so a failed write
		// here is expected rather than a test failure.
		for _, msg := range []map[string]any{
			{"type": "hello"},
			{"type": "disconnect", "reason": "refresh_requested"},
		} {
			data, _ := json.Marshal(msg)
			if conn.Write(ctx, websocket.MessageText, data) != nil {
				return
			}
		}
		conn.Read(ctx) // wait for the client to close
	})

	c := New("xapp-test", func(context.Context, string, string) any { return nil })
	c.apiBase = srv.URL + "/api"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Run(ctx)

	deadline := time.Now().Add(minBackoff / 2)
	for dials.Load() < 3 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if n := dials.Load(); n < 3 {
		t.Errorf("dialled %d times within %s, want at least 3 immediate reconnects", n, minBackoff/2)
	}
}

// Non-command envelopes still need an ack or Slack redelivers them.
func TestOtherEnvelopesAreAcknowledged(t *testing.T) {
	acks := make(chan map[string]any, 1)
	srv, _ := fakeSlack(t, "xapp-test", func(ctx context.Context, conn *websocket.Conn) {
		writeJSON(t, ctx, conn, map[string]any{"type": "events_api", "envelope_id": "env-9", "payload": map[string]any{}})
		_, data, err := conn.Read(ctx)
		if err != nil {
			return
		}
		var a map[string]any
		json.Unmarshal(data, &a)
		acks <- a
		<-ctx.Done()
	})

	called := false
	c := New("xapp-test", func(context.Context, string, string) any { called = true; return nil })
	c.apiBase = srv.URL + "/api"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Run(ctx)

	select {
	case a := <-acks:
		if a["envelope_id"] != "env-9" {
			t.Errorf("ack = %v, want envelope_id env-9", a)
		}
		if _, ok := a["payload"]; ok {
			t.Errorf("ack for a non-command carried a payload: %v", a)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no ack received")
	}
	if called {
		t.Error("handler was called for a non-command envelope")
	}
}

func TestBadTokenIsReported(t *testing.T) {
	srv, dials := fakeSlack(t, "xapp-right", func(context.Context, *websocket.Conn) {})
	c := New("xapp-wrong", nil)
	c.apiBase = srv.URL + "/api"

	connected, err := c.connectOnce(context.Background())
	if connected || err == nil || !strings.Contains(err.Error(), "invalid_auth") {
		t.Errorf("connectOnce = %v, %v; want an invalid_auth error", connected, err)
	}
	if dials.Load() != 0 {
		t.Error("dialled the socket despite a rejected token")
	}
}
