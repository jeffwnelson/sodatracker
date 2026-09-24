// Package notify delivers SodaTracker events to outside chat systems.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// SlackClient posts messages to a Slack incoming webhook.
type SlackClient struct {
	webhookURL string
	http       *http.Client
}

// NewSlack returns a client for the given incoming-webhook URL. A blank URL
// yields a nil client, which Post treats as a no-op so callers can stay
// unconditional.
func NewSlack(webhookURL string) *SlackClient {
	if webhookURL == "" {
		return nil
	}
	return &SlackClient{
		webhookURL: webhookURL,
		http:       &http.Client{Timeout: 10 * time.Second},
	}
}

// Message is the subset of Slack's payload format we use. Blocks render the
// rich layout; Text is the notification fallback and is what shows in the
// sidebar and on mobile push, so it is always set.
type Message struct {
	Text   string  `json:"text"`
	Blocks []Block `json:"blocks,omitempty"`
}

type Block struct {
	Type     string       `json:"type"`
	Text     *TextObject  `json:"text,omitempty"`
	Fields   []TextObject `json:"fields,omitempty"`
	Elements []TextObject `json:"elements,omitempty"`
}

type TextObject struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func mrkdwn(format string, args ...any) *TextObject {
	return &TextObject{Type: "mrkdwn", Text: fmt.Sprintf(format, args...)}
}

// Post delivers a message. A nil client is a no-op so Slack stays optional.
func (c *SlackClient) Post(ctx context.Context, m Message) error {
	if c == nil {
		return nil
	}
	body, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshal slack message: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build slack request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("post to slack: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Slack returns the reason in the body ("invalid_payload", "no_service").
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("slack returned %d: %s", resp.StatusCode, bytes.TrimSpace(detail))
	}
	return nil
}
