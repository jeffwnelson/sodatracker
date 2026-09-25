package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jeffwnelson/sodatracker/internal/domain"
)

func TestAnalyticsEndpoints(t *testing.T) {
	router, st := setupWithStore(t)
	st.CreateMachine(context.Background(), &domain.Machine{ID: "m1", Name: "Test", SlotCount: 1})

	get := func(path string, authed bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if authed {
			req.Header.Set("X-API-Key", "test-key")
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}

	for _, kind := range []string{"consumption", "products", "inventory", "spend"} {
		base := "/api/v1/analytics/" + kind
		if w := get(base+"?machineId=m1", false); w.Code != http.StatusUnauthorized {
			t.Errorf("%s without auth: %d, want 401", kind, w.Code)
		}
		w := get(base+"?machineId=m1&tz=America/Chicago", true)
		if w.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", kind, w.Code, w.Body)
		}
		var body struct {
			Range struct{ Timezone string } `json:"range"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || body.Range.Timezone != "America/Chicago" {
			t.Errorf("%s range = %+v (%v)", kind, body.Range, err)
		}
		if w := get(base+"?machineId=missing", true); w.Code != http.StatusNotFound {
			t.Errorf("%s unknown machine: %d, want 404", kind, w.Code)
		}
	}

	for _, q := range []string{
		"",
		"?machineId=m1&tz=Mars/Olympus",
		"?machineId=m1&from=yesterday",
		"?machineId=m1&bucket=hour",
		"?machineId=m1&from=2026-03-02T00:00:00Z&to=2026-03-01T00:00:00Z",
	} {
		if w := get("/api/v1/analytics/consumption"+q, true); w.Code != http.StatusBadRequest {
			t.Errorf("query %q: %d, want 400", q, w.Code)
		}
	}
}
