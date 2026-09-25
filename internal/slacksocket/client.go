// Package slacksocket receives Slack slash commands over Socket Mode.
//
// Slack normally delivers a slash command by POSTing to a public URL, which
// the Pi doesn't have: it's reachable only on the tailnet. In Socket Mode the
// app dials out to Slack over a WebSocket and Slack pushes commands down it,
// so nothing has to be exposed. The connection is authenticated by the
// app-level token, so there is no request signature to check.
//
// Protocol: POST apps.connections.open with the app token to get a wss URL,
// connect, and answer each envelope by sending back its envelope_id within
// three seconds. For a slash command the reply's payload is the message
// Slack shows. Slack rotates connections every few hours by sending a
// "disconnect" envelope; the client then reconnects.
package slacksocket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/coder/websocket"
)

// Handler answers one slash command. Its return value becomes the reply
// message and must marshal to a Slack message payload.
type Handler func(ctx context.Context, command, text string) any

// ackTimeout is how long a handler gets. Slack drops an envelope that isn't
// acknowledged within three seconds and retries it.
const ackTimeout = 2500 * time.Millisecond

const (
	minBackoff = time.Second
	maxBackoff = time.Minute
)

type Client struct {
	appToken string
	handle   Handler
	http     *http.Client
	apiBase  string // overridden in tests
}

func New(appToken string, handle Handler) *Client {
	return &Client{
		appToken: appToken,
		handle:   handle,
		http:     &http.Client{Timeout: 10 * time.Second},
		apiBase:  "https://slack.com/api",
	}
}

// Run holds a connection open until ctx is cancelled, reconnecting with
// exponential backoff after failures and immediately after Slack's routine
// disconnects.
func (c *Client) Run(ctx context.Context) {
	backoff := minBackoff
	for ctx.Err() == nil {
		connected, err := c.connectOnce(ctx)
		if ctx.Err() != nil {
			return
		}
		if connected {
			backoff = minBackoff
		}
		if err == nil {
			continue // routine rotation: reconnect straight away
		}
		slog.Warn("slack socket mode connection failed", "error", err, "retryIn", backoff.String())
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, maxBackoff)
	}
}

// envelope is every message Slack sends on the socket.
type envelope struct {
	Type       string          `json:"type"`
	EnvelopeID string          `json:"envelope_id"`
	Payload    json.RawMessage `json:"payload"`
	Reason     string          `json:"reason"`
}

type slashPayload struct {
	Command string `json:"command"`
	Text    string `json:"text"`
}

type ack struct {
	EnvelopeID string `json:"envelope_id"`
	Payload    any    `json:"payload,omitempty"`
}

// connectOnce serves one connection. connected reports whether Slack said
// hello, which resets the backoff. A nil error means Slack asked to
// reconnect.
func (c *Client) connectOnce(ctx context.Context) (connected bool, err error) {
	url, err := c.openConnection(ctx)
	if err != nil {
		return false, err
	}
	conn, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		return false, fmt.Errorf("dial: %w", err)
	}
	defer conn.CloseNow()
	conn.SetReadLimit(1 << 20)

	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return connected, fmt.Errorf("read: %w", err)
		}
		var env envelope
		if err := json.Unmarshal(data, &env); err != nil {
			slog.Warn("slack socket mode: undecodable message", "error", err)
			continue
		}

		switch env.Type {
		case "hello":
			connected = true
			slog.Info("slack socket mode connected")
		case "disconnect":
			slog.Info("slack socket mode reconnecting", "reason", env.Reason)
			conn.Close(websocket.StatusNormalClosure, "")
			return connected, nil
		case "slash_commands":
			reply := c.answer(ctx, env.Payload)
			if err := c.send(ctx, conn, ack{EnvelopeID: env.EnvelopeID, Payload: reply}); err != nil {
				return connected, err
			}
		default:
			// Nothing else is subscribed, but anything carrying an envelope
			// must still be acknowledged or Slack keeps retrying it.
			if env.EnvelopeID != "" {
				if err := c.send(ctx, conn, ack{EnvelopeID: env.EnvelopeID}); err != nil {
					return connected, err
				}
			}
		}
	}
}

func (c *Client) answer(ctx context.Context, raw json.RawMessage) any {
	var p slashPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		slog.Warn("slack socket mode: bad slash command payload", "error", err)
		return nil
	}
	hctx, cancel := context.WithTimeout(ctx, ackTimeout)
	defer cancel()
	slog.Info("slack command", "command", p.Command, "text", p.Text)
	return c.handle(hctx, p.Command, p.Text)
}

func (c *Client) send(ctx context.Context, conn *websocket.Conn, a ack) error {
	data, err := json.Marshal(a)
	if err != nil {
		return err
	}
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	return nil
}

// openConnection asks Slack for a fresh single-use WebSocket URL.
func (c *Client) openConnection(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiBase+"/apps.connections.open", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.appToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("apps.connections.open: %w", err)
	}
	defer resp.Body.Close()

	var out struct {
		OK    bool   `json:"ok"`
		URL   string `json:"url"`
		Error string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("apps.connections.open: decode: %w", err)
	}
	if !out.OK {
		// invalid_auth or not_allowed_token_type here means the token is
		// wrong or isn't an app-level (xapp-) token; retrying won't help,
		// but Run's backoff keeps the noise down.
		return "", fmt.Errorf("apps.connections.open: %s", out.Error)
	}
	if !strings.HasPrefix(out.URL, "wss://") && !strings.HasPrefix(out.URL, "ws://") {
		return "", errors.New("apps.connections.open: no websocket url in response")
	}
	return out.URL, nil
}
