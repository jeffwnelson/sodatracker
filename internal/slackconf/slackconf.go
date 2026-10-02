// Package slackconf holds the Slack settings edited on the Settings page:
// the incoming webhook that alerts post to, and the app-level token that
// receives /soda over Socket Mode. Both are secret settings, so they are
// never returned by the API and are left out of backups. Changing either
// takes effect at once: the webhook is swapped in place and the Socket Mode
// connection is restarted.
package slackconf

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/jeffwnelson/sodatracker/internal/notify"
	"github.com/jeffwnelson/sodatracker/internal/slacksocket"
)

const (
	keyWebhook  = "slack.webhook_url"
	keyAppToken = "slack.app_token"
	// keySeeded records that the env vars were copied in once, so clearing
	// a value in the UI isn't undone by the env on the next restart.
	keySeeded = "slack.seeded_from_env"
)

// Store is the part of store.Store this package uses.
type Store interface {
	GetSetting(ctx context.Context, key string) (string, error)
	SetSetting(ctx context.Context, key, value string, secret bool) error
}

// Commands states for Status.
const (
	CommandsOff        = "off"
	CommandsConnecting = "connecting"
	CommandsConnected  = "connected"
	CommandsError      = "error"
)

// Status is what the Settings page shows. The secrets themselves are never
// included, only whether each is set.
type Status struct {
	WebhookSet  bool `json:"webhookSet"`
	AppTokenSet bool `json:"appTokenSet"`
	// Commands is the Socket Mode connection: off, connecting, connected
	// or error (with CommandsError).
	Commands      string `json:"commands" example:"connected"`
	CommandsError string `json:"commandsError,omitempty"`
}

// Update changes the settings. A nil field is left as is; "" clears it.
type Update struct {
	WebhookURL *string `json:"webhookUrl,omitempty"`
	AppToken   *string `json:"appToken,omitempty"`
}

// InvalidError is an Update that was refused.
type InvalidError struct{ msg string }

func (e *InvalidError) Error() string { return e.msg }

type Manager struct {
	store  Store
	parent context.Context
	handle slacksocket.Handler

	mu       sync.Mutex
	webhook  string
	client   *notify.SlackClient
	appToken string
	// The running Socket Mode client: stop cancels it, and gen ignores
	// state reports from one that has been replaced.
	stop     context.CancelFunc
	gen      int
	commands string
	cmdErr   string

	// runCommands holds a Socket Mode connection open until ctx ends,
	// reporting its state. Tests replace it so they don't dial Slack.
	runCommands func(ctx context.Context, token string, handle slacksocket.Handler, onState func(bool, error))
}

func runSocket(ctx context.Context, token string, handle slacksocket.Handler, onState func(bool, error)) {
	c := slacksocket.New(token, handle)
	c.OnState = onState
	c.Run(ctx)
}

// New loads the settings, copying SLACK_WEBHOOK_URL and SLACK_APP_TOKEN in
// the first time so an existing .env carries over, and starts Socket Mode
// if a token is set. ctx bounds the Socket Mode connection; handle answers
// slash commands.
func New(ctx context.Context, st Store, handle slacksocket.Handler, envWebhook, envAppToken string) (*Manager, error) {
	m := &Manager{store: st, parent: ctx, handle: handle, commands: CommandsOff, runCommands: runSocket}
	seeded, err := st.GetSetting(ctx, keySeeded)
	if err != nil {
		return nil, err
	}
	if seeded == "" {
		for _, kv := range [][2]string{{keyWebhook, envWebhook}, {keyAppToken, envAppToken}} {
			if kv[1] == "" {
				continue
			}
			if cur, err := st.GetSetting(ctx, kv[0]); err == nil && cur == "" {
				if err := st.SetSetting(ctx, kv[0], kv[1], true); err != nil {
					return nil, err
				}
				slog.Info("Slack setting copied from the environment", "key", kv[0])
			}
		}
		if err := st.SetSetting(ctx, keySeeded, "1", false); err != nil {
			return nil, err
		}
	}
	if m.webhook, err = st.GetSetting(ctx, keyWebhook); err != nil {
		return nil, err
	}
	if m.appToken, err = st.GetSetting(ctx, keyAppToken); err != nil {
		return nil, err
	}
	m.client = notify.NewSlack(m.webhook)

	m.mu.Lock()
	m.restartCommandsLocked()
	m.mu.Unlock()
	slog.Info("Slack configured", "notifications", m.webhook != "", "commands", m.appToken != "")
	return m, nil
}

// Client is the current webhook client, nil without a webhook. The notifier
// calls it for every message, so a new webhook applies at once.
func (m *Manager) Client() *notify.SlackClient {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.client
}

func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	return Status{
		WebhookSet:    m.webhook != "",
		AppTokenSet:   m.appToken != "",
		Commands:      m.commands,
		CommandsError: m.cmdErr,
	}
}

// Configure validates and stores u, then applies it. An *InvalidError means
// it was refused.
func (m *Manager) Configure(ctx context.Context, u Update) (Status, error) {
	var webhook, token *string
	if u.WebhookURL != nil {
		v := strings.TrimSpace(*u.WebhookURL)
		if v != "" {
			p, err := url.Parse(v)
			if err != nil || p.Scheme != "https" || p.Host != "hooks.slack.com" {
				return Status{}, &InvalidError{"the webhook URL should start with https://hooks.slack.com/"}
			}
		}
		webhook = &v
	}
	if u.AppToken != nil {
		v := strings.TrimSpace(*u.AppToken)
		if v != "" && !strings.HasPrefix(v, "xapp-") {
			return Status{}, &InvalidError{"the app token should start with xapp- (an app-level token, not a bot xoxb- token)"}
		}
		token = &v
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if webhook != nil {
		if err := m.store.SetSetting(ctx, keyWebhook, *webhook, true); err != nil {
			return Status{}, err
		}
		m.webhook = *webhook
		m.client = notify.NewSlack(*webhook)
	}
	if token != nil && *token != m.appToken {
		if err := m.store.SetSetting(ctx, keyAppToken, *token, true); err != nil {
			return Status{}, err
		}
		m.appToken = *token
		m.restartCommandsLocked()
	}
	slog.Info("Slack settings changed", "notifications", m.webhook != "", "commands", m.appToken != "")
	return Status{
		WebhookSet:    m.webhook != "",
		AppTokenSet:   m.appToken != "",
		Commands:      m.commands,
		CommandsError: m.cmdErr,
	}, nil
}

// SendTest posts a test message to the webhook.
func (m *Manager) SendTest(ctx context.Context) error {
	c := m.Client()
	if c == nil {
		return &InvalidError{"no webhook URL is set"}
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	err := c.Post(ctx, notify.Message{
		Text: "SodaTracker can post here. Stock, backup and not-registering alerts will arrive in this channel.",
	})
	if err != nil {
		return fmt.Errorf("Slack didn't accept the test message: %w", err)
	}
	return nil
}

// restartCommandsLocked stops any Socket Mode client and starts one for
// the current token.
func (m *Manager) restartCommandsLocked() {
	if m.stop != nil {
		m.stop()
		m.stop = nil
	}
	m.gen++
	m.cmdErr = ""
	if m.appToken == "" {
		m.commands = CommandsOff
		return
	}
	m.commands = CommandsConnecting
	ctx, cancel := context.WithCancel(m.parent)
	m.stop = cancel
	gen := m.gen
	onState := func(connected bool, err error) {
		m.mu.Lock()
		defer m.mu.Unlock()
		if gen != m.gen {
			return // a replaced client reporting its shutdown
		}
		if connected {
			m.commands, m.cmdErr = CommandsConnected, ""
		} else {
			m.commands, m.cmdErr = CommandsError, err.Error()
		}
	}
	go m.runCommands(ctx, m.appToken, m.handle, onState)
}

// Stop ends the Socket Mode connection.
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.stop != nil {
		m.stop()
		m.stop = nil
	}
	m.gen++
}
