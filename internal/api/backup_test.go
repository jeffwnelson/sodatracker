package api

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jeffwnelson/sodatracker/internal/backup"
	"github.com/jeffwnelson/sodatracker/internal/service"
	"github.com/jeffwnelson/sodatracker/internal/store/sqlite"
)

func TestBackupEndpoints(t *testing.T) {
	dir := t.TempDir()
	st, err := sqlite.New(filepath.Join(dir, "sodatracker.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	backups, err := backup.New(context.Background(), backup.Config{Dir: filepath.Join(dir, "backups")}, st)
	if err != nil {
		t.Fatal(err)
	}
	svc := service.New(st, testJWTSecret, nil)
	r := NewRouter(st, svc, RouterConfig{APIKey: "test-key", JWTSecret: testJWTSecret, Backups: backups})

	do := func(method, path, body string, authed bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if authed {
			req.Header.Set("X-API-Key", "test-key")
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	for _, c := range []struct{ method, path string }{
		{"GET", "/api/v1/admin/backup"},
		{"POST", "/api/v1/admin/backup"},
		{"PUT", "/api/v1/admin/backup/config"},
		{"GET", "/api/v1/admin/backup/download"},
	} {
		if w := do(c.method, c.path, "{}", false); w.Code != http.StatusUnauthorized {
			t.Errorf("%s %s without auth: %d", c.method, c.path, w.Code)
		}
	}

	// Pushing off: Back up now only snapshots.
	w := do("POST", "/api/v1/admin/backup", "", true)
	var s backup.Status
	json.Unmarshal(w.Body.Bytes(), &s)
	if w.Code != http.StatusOK || len(s.Snapshots) != 1 || s.PushEnabled {
		t.Fatalf("backup now: %d %s", w.Code, w.Body)
	}

	if w := do("PUT", "/api/v1/admin/backup/config", `{"repo":"bad repo","intervalHours":24,"keep":30}`, true); w.Code != http.StatusBadRequest {
		t.Errorf("bad repo: %d %s", w.Code, w.Body)
	}
	w = do("PUT", "/api/v1/admin/backup/config", `{"repo":"","intervalHours":6,"keep":10}`, true)
	json.Unmarshal(w.Body.Bytes(), &s)
	if w.Code != http.StatusOK || s.IntervalHours != 6 || s.Keep != 10 {
		t.Fatalf("configure: %d %s", w.Code, w.Body)
	}

	w = do("GET", "/api/v1/admin/backup/download", "", true)
	if w.Code != http.StatusOK || w.Header().Get("Content-Type") != "application/gzip" ||
		!strings.Contains(w.Header().Get("Content-Disposition"), ".db.gz") {
		t.Fatalf("download: %d %v", w.Code, w.Header())
	}
	if _, err := gzip.NewReader(w.Body); err != nil {
		t.Fatalf("download not gzip: %v", err)
	}
}

func TestAlertSettingsEndpoints(t *testing.T) {
	r := setup(t)
	do := func(method, body string, authed bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/api/v1/admin/alert-settings", strings.NewReader(body))
		if authed {
			req.Header.Set("X-API-Key", "test-key")
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	if w := do("GET", "", false); w.Code != http.StatusUnauthorized {
		t.Errorf("GET without auth: %d", w.Code)
	}
	if w := do("PUT", `{"quietSlotCans":-1}`, true); w.Code != http.StatusBadRequest {
		t.Errorf("invalid PUT: %d %s", w.Code, w.Body)
	}
	w := do("PUT", `{"quietSlotCans":20,"quietMachineHours":0,"quietMachineWeekdaysOnly":false,"timezone":"UTC"}`, true)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"quietSlotCans":20`) {
		t.Fatalf("PUT: %d %s", w.Code, w.Body)
	}
	if w := do("GET", "", true); !strings.Contains(w.Body.String(), `"quietSlotCans":20`) {
		t.Errorf("GET after PUT: %s", w.Body)
	}
}
