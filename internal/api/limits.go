package api

import (
	"net/http"
	"strconv"
	"sync"
	"time"
)

// maxBodyBytes caps every request body. The largest real ones (slot and
// restock lists) are a few KB, and an unbounded body could exhaust the Pi's
// memory.
const maxBodyBytes = 1 << 20

func limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
		next.ServeHTTP(w, r)
	})
}

// Login attempts allowed in a burst, and how fast they come back.
const (
	loginBurst  = 10
	loginRefill = 6 * time.Second
)

// loginLimiter is a token bucket for POST /auth/login. Each attempt costs a
// bcrypt comparison, a few hundred ms of CPU on a Pi 3, so it limits both
// password guessing and load. A successful login gives its token back.
//
// It is one bucket for all clients: behind `tailscale serve` every request
// comes from loopback, and X-Forwarded-For can be forged, so there is no
// trustworthy per-client key.
type loginLimiter struct {
	mu     sync.Mutex
	tokens float64
	last   time.Time
	now    func() time.Time
}

// loginClock is the limiter's clock; tests freeze it.
var loginClock = time.Now

func newLoginLimiter() *loginLimiter {
	return &loginLimiter{tokens: loginBurst, now: loginClock}
}

// take spends a token, or reports how long until one is available.
func (l *loginLimiter) take() (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if !l.last.IsZero() {
		l.tokens = min(loginBurst, l.tokens+float64(now.Sub(l.last))/float64(loginRefill))
	}
	l.last = now
	if l.tokens < 1 {
		return false, time.Duration((1 - l.tokens) * float64(loginRefill))
	}
	l.tokens--
	return true, 0
}

func (l *loginLimiter) refund() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.tokens = min(loginBurst, l.tokens+1)
}

// writeTooMany answers a rate-limited request.
func writeTooMany(w http.ResponseWriter, wait time.Duration) {
	w.Header().Set("Retry-After", strconv.Itoa(int(wait.Seconds())+1))
	writeErr(w, http.StatusTooManyRequests, "too many login attempts; try again shortly")
}
