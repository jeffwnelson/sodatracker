package backup

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/jeffwnelson/sodatracker/internal/domain"
	"github.com/jeffwnelson/sodatracker/internal/events"
	"github.com/jeffwnelson/sodatracker/internal/store/sqlite"
)

// fakeGitHub records Contents API uploads and fails while status is set.
type fakeGitHub struct {
	mu      sync.Mutex
	status  int
	body    string
	uploads map[string][]byte // path → decoded content
	auth    string
}

func (g *fakeGitHub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.auth = r.Header.Get("Authorization")
	if r.Method == http.MethodGet { // checkRepo
		if g.status != 0 {
			w.WriteHeader(g.status)
			io.WriteString(w, g.body)
			return
		}
		io.WriteString(w, `{"full_name":"x"}`)
		return
	}
	if g.status != 0 {
		w.WriteHeader(g.status)
		io.WriteString(w, g.body)
		return
	}
	var req struct{ Content string }
	json.NewDecoder(r.Body).Decode(&req)
	data, _ := base64.StdEncoding.DecodeString(req.Content)
	g.uploads[r.Method+" "+r.URL.Path] = data
	w.WriteHeader(http.StatusCreated)
	io.WriteString(w, `{}`)
}

func (g *fakeGitHub) set(status int, body string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.status, g.body = status, body
}

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func setup(t *testing.T, keep int) (*Manager, *sqlite.SQLiteStore, *fakeGitHub, *clock, Config) {
	t.Helper()
	dir := t.TempDir()
	st, err := sqlite.New(filepath.Join(dir, "sodatracker.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	if err := st.CreateProduct(context.Background(), &domain.Product{ID: "p1", Name: "Cola"}); err != nil {
		t.Fatal(err)
	}
	gh := &fakeGitHub{uploads: map[string][]byte{}}
	srv := httptest.NewServer(gh)
	t.Cleanup(srv.Close)
	ctx := context.Background()
	st.SetSetting(ctx, keyRepo, "owner/sodatracker-backups", false)
	st.SetSetting(ctx, keyToken, "secret-token", true)
	st.SetSetting(ctx, keyKeep, strconv.Itoa(keep), false)
	cfg := Config{Dir: filepath.Join(dir, "backups"), APIBase: srv.URL}
	m, err := New(ctx, cfg, st)
	if err != nil {
		t.Fatal(err)
	}
	c := &clock{t: time.Date(2026, 10, 2, 3, 0, 0, 0, time.UTC)}
	m.now = c.now
	return m, st, gh, c, cfg
}

func TestBackupSnapshotsAndPushes(t *testing.T) {
	ctx := context.Background()
	m, _, gh, _, cfg := setup(t, 30)

	s, err := m.BackupNow(ctx)
	if err != nil {
		t.Fatal(err)
	}
	name := "sodatracker-20261002T030000Z.db.gz"
	if len(s.Snapshots) != 1 || s.Snapshots[0].Name != name {
		t.Fatalf("snapshots = %+v", s.Snapshots)
	}
	if s.LastPushedFile != name || s.LastPushAt == nil || s.Stale || s.LastError != "" {
		t.Fatalf("status = %+v", s)
	}
	if gh.auth != "Bearer secret-token" {
		t.Errorf("auth header = %q", gh.auth)
	}
	pushed := gh.uploads["PUT /repos/owner/sodatracker-backups/contents/snapshots/2026/"+name]
	local, _ := os.ReadFile(filepath.Join(cfg.Dir, name))
	if len(pushed) == 0 || !bytes.Equal(pushed, local) {
		t.Fatalf("pushed %d bytes, local %d; uploads %v", len(pushed), len(local), keys(gh.uploads))
	}

	// The snapshot is a working database with the data in it.
	zr, err := gzip.NewReader(bytes.NewReader(local))
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(zr)
	restored := filepath.Join(t.TempDir(), "restored.db")
	os.WriteFile(restored, raw, 0o644)
	rst, err := sqlite.New(restored)
	if err != nil {
		t.Fatal(err)
	}
	defer rst.Close()
	if p, err := rst.GetProduct(ctx, "p1"); err != nil || p == nil || p.Name != "Cola" {
		t.Fatalf("restored product = %+v, %v", p, err)
	}

	// No stray temporary files.
	entries, _ := os.ReadDir(cfg.Dir)
	for _, e := range entries {
		if e.Name() != name && e.Name() != stateFile {
			t.Errorf("unexpected file %s", e.Name())
		}
	}
}

func TestBackupPushFailureRetriesAndPersists(t *testing.T) {
	ctx := context.Background()
	m, st, gh, c, cfg := setup(t, 30)

	gh.set(http.StatusUnauthorized, `{"message":"Bad credentials"}`)
	s, err := m.BackupNow(ctx)
	if err == nil {
		t.Fatal("want push error")
	}
	if !s.Stale || s.LastError == "" || s.LastPushAt != nil || len(s.Snapshots) != 1 {
		t.Fatalf("status = %+v", s)
	}
	if want := "push to owner/sodatracker-backups: GitHub 401: Bad credentials"; s.LastError != want {
		t.Errorf("LastError = %q, want %q", s.LastError, want)
	}

	// The failure survives a restart.
	m2, err := New(ctx, cfg, st)
	if err != nil {
		t.Fatal(err)
	}
	m2.now = c.now
	if !m2.Status().Stale {
		t.Fatal("restarted manager lost the failure")
	}

	// The next check, well before the next snapshot is due, retries the push.
	gh.set(0, "")
	c.t = c.t.Add(time.Hour)
	m2.tick(ctx)
	s = m2.Status()
	if s.Stale || s.LastPushedFile != s.Snapshots[0].Name || len(s.Snapshots) != 1 {
		t.Fatalf("after retry: %+v", s)
	}

	// A push older than StaleAfter is stale even without an error.
	c.t = c.t.Add(StaleAfter + time.Minute)
	m2.set.IntervalHours = 0 // keep tick from taking a fresh one
	if !m2.Status().Stale {
		t.Fatal("old push not reported stale")
	}
}

func TestBackupScheduleAndPrune(t *testing.T) {
	ctx := context.Background()
	m, _, gh, c, _ := setup(t, 3)

	m.tick(ctx) // nothing yet: due
	for i := 0; i < 24*5; i++ {
		c.t = c.t.Add(time.Hour)
		m.tick(ctx)
	}
	s := m.Status()
	// Five days of hourly checks take one snapshot a day, starting at hour 0.
	if got := len(gh.uploads); got != 6 {
		t.Errorf("pushed %d snapshots, want 6", got)
	}
	if len(s.Snapshots) != 3 {
		t.Fatalf("kept %d snapshots, want 3", len(s.Snapshots))
	}
	if s.Snapshots[0].Name != "sodatracker-20261007T030000Z.db.gz" {
		t.Errorf("newest = %s", s.Snapshots[0].Name)
	}
}

func TestBackupAlreadyPushedCountsAsSuccess(t *testing.T) {
	m, _, gh, _, _ := setup(t, 30)
	gh.set(http.StatusUnprocessableEntity, `{"message":"Invalid request.\n\n\"sha\" wasn't supplied."}`)
	if _, err := m.BackupNow(context.Background()); err != nil {
		t.Fatalf("422 for an existing file should count as pushed: %v", err)
	}
}

func TestBackupWithoutPush(t *testing.T) {
	m, _, gh, _, _ := setup(t, 30)
	m.set.Token = ""
	s, err := m.BackupNow(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if s.PushEnabled || s.Stale || len(s.Snapshots) != 1 || len(gh.uploads) != 0 {
		t.Fatalf("status = %+v, uploads %d", s, len(gh.uploads))
	}

	var buf bytes.Buffer
	if err := m.WriteSnapshot(context.Background(), &buf); err != nil {
		t.Fatal(err)
	}
	if _, err := gzip.NewReader(&buf); err != nil {
		t.Fatalf("download is not gzip: %v", err)
	}
	if n := len(m.Status().Snapshots); n != 1 {
		t.Errorf("download kept a snapshot: %d", n)
	}
}

func keys(m map[string][]byte) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestBackupConfigure(t *testing.T) {
	ctx := context.Background()
	m, st, gh, _, cfg := setup(t, 30)

	bad := []Update{
		{Repo: "not a repo", Keep: 30},
		{Repo: "o/r", IntervalHours: -1, Keep: 30},
		{Repo: "o/r", IntervalHours: 24, Keep: 0},
	}
	for _, u := range bad {
		var inv *InvalidError
		if _, err := m.Configure(ctx, u); !errors.As(err, &inv) {
			t.Errorf("Configure(%+v) = %v, want InvalidError", u, err)
		}
	}

	// GitHub refusing a new token rejects the update and keeps the old one.
	gh.set(http.StatusNotFound, `{"message":"Not Found"}`)
	tok := "wrong-token"
	_, err := m.Configure(ctx, Update{Repo: "owner/other", Token: &tok, IntervalHours: 24, Keep: 30})
	var inv *InvalidError
	if !errors.As(err, &inv) || err.Error() != "GitHub refused owner/other: GitHub 404: Not Found" {
		t.Fatalf("err = %v", err)
	}
	if got, _ := st.GetSetting(ctx, keyToken); got != "secret-token" {
		t.Fatalf("token after refused update = %q", got)
	}

	// Changing only the schedule keeps the token and doesn't call GitHub.
	s, err := m.Configure(ctx, Update{Repo: "owner/sodatracker-backups", IntervalHours: 12, Keep: 5})
	if err != nil {
		t.Fatal(err)
	}
	if !s.TokenSet || !s.PushEnabled || s.IntervalHours != 12 || s.Keep != 5 {
		t.Fatalf("status = %+v", s)
	}
	b, _ := json.Marshal(s)
	if bytes.Contains(b, []byte("secret-token")) {
		t.Fatalf("status leaks the token: %s", b)
	}

	// Settings survive a restart.
	m2, err := New(ctx, cfg, st)
	if err != nil {
		t.Fatal(err)
	}
	if s := m2.Status(); s.IntervalHours != 12 || s.Keep != 5 || !s.TokenSet {
		t.Fatalf("after restart: %+v", s)
	}

	// Clearing the token turns pushing off.
	empty := ""
	if s, err = m.Configure(ctx, Update{Repo: "owner/sodatracker-backups", Token: &empty, IntervalHours: 12, Keep: 5}); err != nil || s.PushEnabled || s.TokenSet {
		t.Fatalf("after clear: %+v, %v", s, err)
	}
}

func TestSnapshotLeavesOutToken(t *testing.T) {
	ctx := context.Background()
	m, _, _, _, _ := setup(t, 30)
	var buf bytes.Buffer
	if err := m.WriteSnapshot(ctx, &buf); err != nil {
		t.Fatal(err)
	}
	zr, _ := gzip.NewReader(&buf)
	raw, _ := io.ReadAll(zr)
	if bytes.Contains(raw, []byte("secret-token")) {
		t.Fatal("snapshot contains the token")
	}
	if !bytes.Contains(raw, []byte("owner/sodatracker-backups")) {
		t.Fatal("snapshot lost the non-secret settings")
	}
}

// recordingBus is a minimal EventBus that records published events.
type recordingBus struct {
	mu  sync.Mutex
	evs []events.Event
}

func (b *recordingBus) Publish(_ context.Context, evt events.Event) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.evs = append(b.evs, evt)
	return nil
}
func (b *recordingBus) Subscribe(events.EventType, events.Handler) func() { return func() {} }
func (b *recordingBus) Close() error                                      { return nil }

func (b *recordingBus) types() []events.EventType {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []events.EventType
	for _, e := range b.evs {
		out = append(out, e.Type)
	}
	return out
}

func TestBackupHealthEvents(t *testing.T) {
	ctx := context.Background()
	m, st, gh, c, cfg := setup(t, 30)
	bus := &recordingBus{}
	m.cfg.Bus = bus

	// Healthy run: nothing published.
	if _, err := m.BackupNow(ctx); err != nil {
		t.Fatal(err)
	}
	if got := bus.types(); len(got) != 0 {
		t.Fatalf("healthy backup published %v", got)
	}

	// Failing pushes alert once, however many retries fail.
	gh.set(http.StatusUnauthorized, `{"message":"Bad credentials"}`)
	c.t = c.t.Add(24 * time.Hour)
	m.tick(ctx)
	for i := 0; i < 3; i++ {
		c.t = c.t.Add(time.Hour)
		m.tick(ctx)
	}
	if got := bus.types(); len(got) != 1 || got[0] != events.EventBackupFailed {
		t.Fatalf("after failures: %v", got)
	}
	if e := bus.evs[0].Data["error"]; e != "push to owner/sodatracker-backups: GitHub 401: Bad credentials" {
		t.Errorf("error = %v", e)
	}

	// A restart while failing doesn't alert again.
	m2, err := New(ctx, cfg, st)
	if err != nil {
		t.Fatal(err)
	}
	m2.now, m2.cfg.Bus, m2.cfg.APIBase = c.now, bus, m.cfg.APIBase
	c.t = c.t.Add(time.Hour)
	m2.tick(ctx)
	if got := bus.types(); len(got) != 1 {
		t.Fatalf("restart re-alerted: %v", got)
	}

	// The retry that succeeds recovers.
	gh.set(0, "")
	c.t = c.t.Add(time.Hour)
	m2.tick(ctx)
	if got := bus.types(); len(got) != 2 || got[1] != events.EventBackupRecovered {
		t.Fatalf("after recovery: %v", got)
	}

	// With the schedule off, the hourly check alone notices a push going stale.
	m2.set.IntervalHours = 0
	c.t = c.t.Add(StaleAfter + time.Hour)
	m2.tick(ctx)
	if got := bus.types(); len(got) != 3 || got[2] != events.EventBackupFailed || bus.evs[2].Data["error"] != "" {
		t.Fatalf("stale by age: %v %v", got, bus.evs[len(bus.evs)-1].Data)
	}
}

// Snapshots are private, and a restart removes what an interrupted snapshot
// left behind (a raw copy still holding the secret settings).
func TestBackupFilesArePrivateAndLeftoversRemoved(t *testing.T) {
	ctx := context.Background()
	m, st, _, _, cfg := setup(t, 30)
	if _, err := m.BackupNow(ctx); err != nil {
		t.Fatal(err)
	}
	files, _ := m.list()
	if len(files) != 1 {
		t.Fatalf("got %d snapshots, want 1", len(files))
	}
	snap := filepath.Join(cfg.Dir, files[0].Name)
	for _, p := range []string{cfg.Dir, snap, filepath.Join(cfg.Dir, stateFile)} {
		info, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if perm := info.Mode().Perm(); perm&0o077 != 0 {
			t.Errorf("%s has mode %v, want no group/other access", filepath.Base(p), perm)
		}
	}

	leftovers := []string{".snapshot-123.db", files[0].Name + ".part"}
	for _, name := range leftovers {
		os.WriteFile(filepath.Join(cfg.Dir, name), []byte("x"), 0o644)
	}
	os.Chmod(snap, 0o644) // as written by an older version
	if _, err := New(ctx, cfg, st); err != nil {
		t.Fatal(err)
	}
	for _, name := range leftovers {
		if _, err := os.Stat(filepath.Join(cfg.Dir, name)); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s survived a restart", name)
		}
	}
	if info, _ := os.Stat(snap); info.Mode().Perm() != 0o600 {
		t.Errorf("old snapshot mode %v, want 0600", info.Mode().Perm())
	}
}
