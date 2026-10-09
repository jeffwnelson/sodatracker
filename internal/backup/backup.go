// Package backup snapshots the database on a schedule, keeps the last few
// snapshots on local disk, and pushes each one to a private GitHub repo, so a
// dead SD card doesn't take the data with it.
package backup

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jeffwnelson/sodatracker/internal/events"
)

// StaleAfter is how old the last successful push can get, with pushing on,
// before Status reports the backup as stale.
const StaleAfter = 36 * time.Hour

// checkEvery is how often Run looks for a snapshot that is due or a push to
// retry.
const checkEvery = time.Hour

const (
	filePrefix = "sodatracker-"
	fileSuffix = ".db.gz"
	fileStamp  = "20060102T150405Z"
	stateFile  = "state.json"
)

// Setting keys. The token is a secret setting, so snapshots leave it out.
const (
	keyRepo     = "backup.github_repo"
	keyToken    = "backup.github_token"
	keyInterval = "backup.interval_hours"
	keyKeep     = "backup.keep"
)

const (
	defaultIntervalHours = 24
	defaultKeep          = 30
	maxIntervalHours     = 24 * 30
	maxKeep              = 365
)

var repoPattern = regexp.MustCompile(`^[A-Za-z0-9-]+/[A-Za-z0-9._-]+$`)

// Store is the part of store.Store backups use.
type Store interface {
	// Snapshot writes a consistent copy of the database, without secret
	// settings, to dest.
	Snapshot(ctx context.Context, dest string) error
	GetSetting(ctx context.Context, key string) (string, error)
	SetSetting(ctx context.Context, key, value string, secret bool) error
}

type Config struct {
	// Dir holds the local snapshots and state.json.
	Dir string
	// APIBase is the GitHub API root. Empty means https://api.github.com.
	APIBase string
	// Bus, when set, gets EventBackupFailed when backups become stale and
	// EventBackupRecovered when they stop being stale.
	Bus events.EventBus
}

// settings are edited from the Settings page and stored in the database.
type settings struct {
	// Repo ("owner/name") and Token turn pushing on. Both are required.
	Repo  string
	Token string
	// IntervalHours is the time between scheduled snapshots. Zero turns the
	// schedule off; BackupNow still works.
	IntervalHours int
	// Keep is how many local snapshots to keep.
	Keep int
}

// Update changes the settings. Token nil leaves it as is; "" clears it.
type Update struct {
	Repo          string  `json:"repo" example:"jeffwnelson/sodatracker-backups"`
	Token         *string `json:"token,omitempty"`
	IntervalHours int     `json:"intervalHours" example:"24"`
	Keep          int     `json:"keep" example:"30"`
}

// InvalidError is an Update that was refused: bad values, or GitHub
// wouldn't accept the repo and token.
type InvalidError struct{ msg string }

func (e *InvalidError) Error() string { return e.msg }

func invalid(format string, args ...any) error {
	return &InvalidError{msg: fmt.Sprintf(format, args...)}
}

// File is one local snapshot.
type File struct {
	Name      string    `json:"name" example:"sodatracker-20261002T030000Z.db.gz"`
	Size      int64     `json:"size" example:"24576"`
	CreatedAt time.Time `json:"createdAt"`
}

// Status is what the Settings page shows.
type Status struct {
	PushEnabled bool   `json:"pushEnabled"`
	Repo        string `json:"repo" example:"jeffwnelson/sodatracker-backups"`
	// TokenSet says whether a token is stored. The token itself is never
	// returned.
	TokenSet       bool       `json:"tokenSet"`
	IntervalHours  int        `json:"intervalHours" example:"24"`
	Keep           int        `json:"keep" example:"30"`
	LastPushAt     *time.Time `json:"lastPushAt"`
	LastPushedFile string     `json:"lastPushedFile,omitempty"`
	LastError      string     `json:"lastError,omitempty"`
	LastErrorAt    *time.Time `json:"lastErrorAt,omitempty"`
	// Stale means the last snapshot or push failed, or the last push is
	// older than StaleAfter.
	Stale bool `json:"stale"`
	// Snapshots are the local snapshots, newest first.
	Snapshots []File `json:"snapshots"`
}

// state survives restarts in Dir/state.json.
type state struct {
	LastPushAt     *time.Time `json:"lastPushAt,omitempty"`
	LastPushedFile string     `json:"lastPushedFile,omitempty"`
	LastError      string     `json:"lastError,omitempty"`
	LastErrorAt    *time.Time `json:"lastErrorAt,omitempty"`
	// Alerting records that backup_failed was published and
	// backup_recovered hasn't been yet, so a restart doesn't repeat either.
	Alerting bool `json:"alerting,omitempty"`
}

type Manager struct {
	cfg    Config
	store  Store
	client *http.Client
	now    func() time.Time

	mu  sync.Mutex // serializes snapshots, pushes, set, st and pending
	set settings
	st  state
	// pending events are published by unlock, after the lock is released,
	// because the bus runs subscribers (a Slack post) synchronously.
	pending []events.Event
}

// unlock releases mu, then publishes the events queued while it was held.
func (m *Manager) unlock() {
	evs := m.pending
	m.pending = nil
	m.mu.Unlock()
	if m.cfg.Bus == nil {
		return
	}
	for _, evt := range evs {
		if err := m.cfg.Bus.Publish(context.Background(), evt); err != nil {
			slog.Warn("publish backup event", "type", evt.Type, "error", err)
		}
	}
}

// evaluateLocked queues an event when backups go from healthy to stale or
// back. Only the transitions are published, so a push that keeps failing
// every hour alerts once.
func (m *Manager) evaluateLocked() {
	stale := m.staleLocked()
	if stale == m.st.Alerting {
		return
	}
	m.st.Alerting = stale
	m.save()
	evt := events.Event{
		Type:      events.EventBackupRecovered,
		Timestamp: m.now().UTC().Format(time.RFC3339),
		Data:      map[string]any{"repo": m.set.Repo},
	}
	if stale {
		evt.Type = events.EventBackupFailed
		evt.Data["error"] = m.st.LastError
	}
	if m.st.LastPushAt != nil {
		evt.Data["lastPushAt"] = m.st.LastPushAt.Format(time.RFC3339)
	}
	slog.Info("backup health changed", "stale", stale, "error", m.st.LastError)
	m.pending = append(m.pending, evt)
}

func New(ctx context.Context, cfg Config, st Store) (*Manager, error) {
	if cfg.APIBase == "" {
		cfg.APIBase = "https://api.github.com"
	}
	if err := os.MkdirAll(cfg.Dir, 0o700); err != nil {
		return nil, fmt.Errorf("backup dir: %w", err)
	}
	tidyDir(cfg.Dir)
	m := &Manager{
		cfg:    cfg,
		store:  st,
		client: &http.Client{Timeout: 60 * time.Second},
		now:    time.Now,
	}
	m.loadSettings(ctx)
	b, err := os.ReadFile(filepath.Join(cfg.Dir, stateFile))
	switch {
	case err == nil:
		if err := json.Unmarshal(b, &m.st); err != nil {
			slog.Warn("ignoring unreadable backup state", "error", err)
		}
	case !errors.Is(err, os.ErrNotExist):
		return nil, fmt.Errorf("backup state: %w", err)
	}
	return m, nil
}

// tidyDir makes the backup directory and its snapshots private (they hold the
// admin's password hash) and removes what an interrupted snapshot left: a
// raw .snapshot-*.db still has the secret settings, scrubbed only after it
// is written, and nothing else would ever delete it.
func tidyDir(dir string) {
	os.Chmod(dir, 0o700)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		name, path := e.Name(), filepath.Join(dir, e.Name())
		switch {
		case strings.HasPrefix(name, ".snapshot-") || strings.HasSuffix(name, ".part"):
			if err := os.Remove(path); err != nil {
				slog.Warn("remove leftover backup file", "file", name, "error", err)
			} else {
				slog.Info("removed leftover backup file", "file", name)
			}
		case !e.IsDir():
			os.Chmod(path, 0o600)
		}
	}
}

func (m *Manager) loadSettings(ctx context.Context) {
	get := func(key string) string {
		v, err := m.store.GetSetting(ctx, key)
		if err != nil {
			slog.Warn("read backup setting", "key", key, "error", err)
		}
		return v
	}
	atoi := func(key string, fallback int) int {
		if n, err := strconv.Atoi(get(key)); err == nil {
			return n
		}
		return fallback
	}
	m.set = settings{
		Repo:          get(keyRepo),
		Token:         get(keyToken),
		IntervalHours: atoi(keyInterval, defaultIntervalHours),
		Keep:          atoi(keyKeep, defaultKeep),
	}
}

// Configure validates and stores new settings. A new repo or token is
// checked against GitHub first, so a bad one fails here rather than at the
// next push. An *InvalidError means the update was refused.
func (m *Manager) Configure(ctx context.Context, u Update) (Status, error) {
	u.Repo = strings.TrimSpace(u.Repo)
	if u.Repo != "" && !repoPattern.MatchString(u.Repo) {
		return Status{}, invalid("repo must look like owner/name")
	}
	if u.IntervalHours < 0 || u.IntervalHours > maxIntervalHours {
		return Status{}, invalid("intervalHours must be 0 (off) to %d", maxIntervalHours)
	}
	if u.Keep < 1 || u.Keep > maxKeep {
		return Status{}, invalid("keep must be 1 to %d", maxKeep)
	}

	m.mu.Lock()
	prev := m.set
	m.mu.Unlock()
	next := settings{Repo: u.Repo, Token: prev.Token, IntervalHours: u.IntervalHours, Keep: u.Keep}
	if u.Token != nil {
		next.Token = strings.TrimSpace(*u.Token)
	}
	changed := next.Repo != prev.Repo || next.Token != prev.Token
	// Checked without the lock, since it waits on GitHub.
	if changed && next.Repo != "" && next.Token != "" {
		if err := m.checkRepo(ctx, next); err != nil {
			return Status{}, invalid("GitHub refused %s: %v", next.Repo, err)
		}
	}

	m.mu.Lock()
	defer m.unlock()

	for _, kv := range []struct {
		key, value string
		secret     bool
	}{
		{keyRepo, next.Repo, false},
		{keyToken, next.Token, true},
		{keyInterval, strconv.Itoa(next.IntervalHours), false},
		{keyKeep, strconv.Itoa(next.Keep), false},
	} {
		if err := m.store.SetSetting(ctx, kv.key, kv.value, kv.secret); err != nil {
			return Status{}, err
		}
	}
	m.set = next
	if changed {
		// The old error was about the old repo or token, and nothing has
		// gone to the new pair yet. Push the newest snapshot (or take the
		// first one) now rather than at the next hourly check.
		m.st.LastError, m.st.LastErrorAt, m.st.LastPushedFile = "", nil, ""
		m.save()
		if m.pushEnabled() {
			go m.tick(context.WithoutCancel(ctx))
		}
	}
	if err := m.prune(); err != nil {
		slog.Warn("prune backups", "error", err)
	}
	slog.Info("backup settings changed", "repo", next.Repo, "push", m.pushEnabled(),
		"intervalHours", next.IntervalHours, "keep", next.Keep)
	m.evaluateLocked()
	return m.statusLocked(), nil
}

func (m *Manager) pushEnabled() bool { return m.set.Repo != "" && m.set.Token != "" }

// Run takes a snapshot whenever one is due and retries a failed push, until
// ctx ends. It checks once at startup, so a Pi that was off overnight
// catches up when it boots.
func (m *Manager) Run(ctx context.Context) {
	m.tick(ctx)
	t := time.NewTicker(checkEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			m.tick(ctx)
		}
	}
}

func (m *Manager) tick(ctx context.Context) {
	m.mu.Lock()
	defer m.unlock()
	// Last, after any snapshot or push, and also on quiet hours: that's
	// what notices a push growing older than StaleAfter.
	defer m.evaluateLocked()
	files, err := m.list()
	if err != nil {
		slog.Error("list backups", "error", err)
		return
	}
	// Half a check of slack keeps the nightly snapshot from drifting an hour
	// later each day.
	interval := time.Duration(m.set.IntervalHours) * time.Hour
	due := interval > 0 &&
		(len(files) == 0 || m.now().Sub(files[0].CreatedAt) >= interval-checkEvery/2)
	if due {
		if err := m.backupLocked(ctx); err != nil {
			slog.Error("backup failed", "error", err)
		}
		return
	}
	if !m.pushEnabled() || len(files) == 0 {
		return
	}
	if files[0].Name != m.st.LastPushedFile {
		if err := m.push(ctx, files[0]); err != nil {
			slog.Error("backup push retry failed", "file", files[0].Name, "error", err)
		}
	}
}

// BackupNow takes a snapshot, prunes old ones, and pushes the new one when
// pushing is on. The returned status reflects the outcome either way.
func (m *Manager) BackupNow(ctx context.Context) (Status, error) {
	m.mu.Lock()
	defer m.unlock()
	err := m.backupLocked(ctx)
	m.evaluateLocked()
	return m.statusLocked(), err
}

func (m *Manager) backupLocked(ctx context.Context) error {
	f, err := m.snapshotLocked(ctx)
	if err != nil {
		m.fail(fmt.Errorf("snapshot: %w", err))
		return err
	}
	slog.Info("backup snapshot taken", "file", f.Name, "size", f.Size)
	if err := m.prune(); err != nil {
		slog.Warn("prune backups", "error", err)
	}
	if !m.pushEnabled() {
		m.st.LastError, m.st.LastErrorAt = "", nil
		m.save()
		return nil
	}
	return m.push(ctx, f)
}

// WriteSnapshot writes a fresh gzipped snapshot to w without keeping it.
// The lock covers only taking the snapshot, not streaming it to what may be
// a slow browser, so status checks and the hourly tick aren't held up.
func (m *Manager) WriteSnapshot(ctx context.Context, w io.Writer) error {
	m.mu.Lock()
	raw, err := m.rawSnapshot(ctx)
	m.mu.Unlock()
	if err != nil {
		return err
	}
	defer os.Remove(raw)
	return gzipFile(raw, w)
}

// FileName is the name a snapshot taken at t gets.
func FileName(t time.Time) string {
	return filePrefix + t.UTC().Format(fileStamp) + fileSuffix
}

func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.statusLocked()
}

func (m *Manager) statusLocked() Status {
	files, err := m.list()
	if err != nil {
		slog.Error("list backups", "error", err)
	}
	if files == nil {
		files = []File{}
	}
	s := Status{
		PushEnabled:    m.pushEnabled(),
		Repo:           m.set.Repo,
		TokenSet:       m.set.Token != "",
		IntervalHours:  m.set.IntervalHours,
		Keep:           m.set.Keep,
		LastPushAt:     m.st.LastPushAt,
		LastPushedFile: m.st.LastPushedFile,
		LastError:      m.st.LastError,
		LastErrorAt:    m.st.LastErrorAt,
		Snapshots:      files,
	}
	s.Stale = m.staleLocked()
	return s
}

func (m *Manager) staleLocked() bool {
	return m.st.LastError != "" ||
		(m.pushEnabled() && m.st.LastPushAt != nil && m.now().Sub(*m.st.LastPushAt) > StaleAfter)
}

// rawSnapshot writes an uncompressed snapshot to a temporary file in Dir and
// returns its path.
func (m *Manager) rawSnapshot(ctx context.Context) (string, error) {
	raw := filepath.Join(m.cfg.Dir, fmt.Sprintf(".snapshot-%d.db", m.now().UnixNano()))
	if err := m.store.Snapshot(ctx, raw); err != nil {
		os.Remove(raw)
		return "", err
	}
	return raw, nil
}

func (m *Manager) snapshotLocked(ctx context.Context) (File, error) {
	raw, err := m.rawSnapshot(ctx)
	if err != nil {
		return File{}, err
	}
	defer os.Remove(raw)

	at := m.now().UTC().Truncate(time.Second)
	name := FileName(at)
	final := filepath.Join(m.cfg.Dir, name)
	part := final + ".part"
	out, err := os.OpenFile(part, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return File{}, err
	}
	err = gzipFile(raw, out)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(part, final)
	}
	if err != nil {
		os.Remove(part)
		return File{}, err
	}
	info, err := os.Stat(final)
	if err != nil {
		return File{}, err
	}
	return File{Name: name, Size: info.Size(), CreatedAt: at}, nil
}

func gzipFile(src string, w io.Writer) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	zw := gzip.NewWriter(w)
	if _, err := io.Copy(zw, in); err != nil {
		return err
	}
	return zw.Close()
}

// list returns the local snapshots, newest first.
func (m *Manager) list() ([]File, error) {
	entries, err := os.ReadDir(m.cfg.Dir)
	if err != nil {
		return nil, err
	}
	var files []File
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, filePrefix) || !strings.HasSuffix(name, fileSuffix) {
			continue
		}
		at, err := time.Parse(fileStamp, strings.TrimSuffix(strings.TrimPrefix(name, filePrefix), fileSuffix))
		if err != nil {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, File{Name: name, Size: info.Size(), CreatedAt: at})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].CreatedAt.After(files[j].CreatedAt) })
	return files, nil
}

func (m *Manager) prune() error {
	files, err := m.list()
	if err != nil {
		return err
	}
	for _, f := range files[min(max(m.set.Keep, 1), len(files)):] {
		if err := os.Remove(filepath.Join(m.cfg.Dir, f.Name)); err != nil {
			return err
		}
	}
	return nil
}

// push uploads f to snapshots/<year>/<name> in the repo through the
// Contents API, which needs no git binary and no clone on the Pi.
func (m *Manager) push(ctx context.Context, f File) error {
	err := m.upload(ctx, f)
	if err != nil {
		m.fail(fmt.Errorf("push to %s: %w", m.set.Repo, err))
		return err
	}
	now := m.now().UTC()
	m.st.LastPushAt, m.st.LastPushedFile = &now, f.Name
	m.st.LastError, m.st.LastErrorAt = "", nil
	m.save()
	slog.Info("backup pushed", "repo", m.set.Repo, "file", f.Name)
	return nil
}

func (m *Manager) upload(ctx context.Context, f File) error {
	data, err := os.ReadFile(filepath.Join(m.cfg.Dir, f.Name))
	if err != nil {
		return err
	}
	body, err := json.Marshal(map[string]string{
		"message": "backup: " + f.Name,
		"content": base64.StdEncoding.EncodeToString(data),
	})
	if err != nil {
		return err
	}
	resp, err := m.github(ctx, m.set, http.MethodPut,
		fmt.Sprintf("/contents/snapshots/%d/%s", f.CreatedAt.Year(), f.Name), body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
		return nil
	}
	msg := githubMessage(resp)
	if resp.StatusCode == http.StatusUnprocessableEntity && strings.Contains(msg, `"sha"`) {
		// The file is already there: an earlier push landed but state.json
		// didn't record it. Snapshot names are unique, so it's this one.
		return nil
	}
	return githubError(resp.StatusCode, msg)
}

// checkRepo confirms the token can see the repo. A fine-grained token
// scoped to one repo can read its metadata, so a 404 here means a wrong
// name or a token without access to it.
func (m *Manager) checkRepo(ctx context.Context, s settings) error {
	resp, err := m.github(ctx, s, http.MethodGet, "", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return nil
	}
	return githubError(resp.StatusCode, githubMessage(resp))
}

// github calls /repos/<repo><path> with the token.
func (m *Manager) github(ctx context.Context, s settings, method, path string, body []byte) (*http.Response, error) {
	url := strings.TrimRight(m.cfg.APIBase, "/") + "/repos/" + s.Repo + path
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, rd)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+s.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return m.client.Do(req)
}

func githubMessage(resp *http.Response) string {
	var ghErr struct {
		Message string `json:"message"`
	}
	json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&ghErr)
	return ghErr.Message
}

func githubError(status int, msg string) error {
	if msg != "" {
		return fmt.Errorf("GitHub %d: %s", status, msg)
	}
	return fmt.Errorf("GitHub %d", status)
}

func (m *Manager) fail(err error) {
	now := m.now().UTC()
	m.st.LastError, m.st.LastErrorAt = err.Error(), &now
	m.save()
}

func (m *Manager) save() {
	b, err := json.MarshalIndent(m.st, "", "  ")
	if err != nil {
		return
	}
	path := filepath.Join(m.cfg.Dir, stateFile)
	if err := os.WriteFile(path+".tmp", b, 0o600); err == nil {
		err = os.Rename(path+".tmp", path)
	}
	if err != nil {
		slog.Warn("save backup state", "error", err)
	}
}
