package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jeffwnelson/sodatracker/internal/events/local"
	"github.com/jeffwnelson/sodatracker/internal/service"
	"github.com/jeffwnelson/sodatracker/internal/store/sqlite"
)

const testSlackSecret = "test-signing-secret"

// signedSlackRequest builds a request signed the way Slack signs them.
func signedSlackRequest(body, secret string, ts time.Time) *http.Request {
	req := httptest.NewRequest("POST", "/api/v1/integrations/slack/commands", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	stamp := strconv.FormatInt(ts.Unix(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	fmt.Fprintf(mac, "v0:%s:%s", stamp, body)

	req.Header.Set("X-Slack-Request-Timestamp", stamp)
	req.Header.Set("X-Slack-Signature", "v0="+hex.EncodeToString(mac.Sum(nil)))
	return req
}

func slackRouter(t *testing.T) http.Handler {
	t.Helper()
	st, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	bus := local.New()
	t.Cleanup(func() { bus.Close() })
	svc := service.New(st, testJWTSecret, bus)
	return NewRouter(st, svc, RouterConfig{
		APIKey:             "test-key",
		JWTSecret:          testJWTSecret,
		SlackSigningSecret: testSlackSecret,
	})
}

func TestSlackCommandValidSignature(t *testing.T) {
	r := slackRouter(t)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, signedSlackRequest("command=%2Fsoda&text=help", testSlackSecret, time.Now()))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !strings.Contains(resp["text"], "/soda status") {
		t.Errorf("expected help text, got %q", resp["text"])
	}
}

func TestSlackCommandRejectsBadSignature(t *testing.T) {
	r := slackRouter(t)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, signedSlackRequest("command=%2Fsoda&text=status", "wrong-secret", time.Now()))

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for a signature from the wrong secret, got %d", w.Code)
	}
}

// A captured request must stop working once it ages out of the skew window,
// otherwise anyone who sees one can replay it forever.
func TestSlackCommandRejectsReplay(t *testing.T) {
	r := slackRouter(t)

	stale := time.Now().Add(-10 * time.Minute)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, signedSlackRequest("command=%2Fsoda&text=status", testSlackSecret, stale))

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for a 10-minute-old request, got %d", w.Code)
	}
}

func TestSlackCommandRejectsUnsigned(t *testing.T) {
	r := slackRouter(t)

	req := httptest.NewRequest("POST", "/api/v1/integrations/slack/commands",
		strings.NewReader("command=%2Fsoda&text=status"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for a request with no signature headers, got %d", w.Code)
	}
}

// With no signing secret the route must not exist at all — registering it
// would expose machine contents with nothing authenticating the caller.
func TestSlackCommandNotRegisteredWithoutSecret(t *testing.T) {
	r := setup(t)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, signedSlackRequest("command=%2Fsoda&text=status", testSlackSecret, time.Now()))

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 when no signing secret is configured, got %d", w.Code)
	}
}
