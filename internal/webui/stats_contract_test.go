package webui

import (
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/httpapi"
)

// TestStatsSurfacesAgree pins the stats split-brain shut (23:47 f24): the
// dashboard's GET /api/stats and the write API's GET /api/v1/stats must
// publish the SAME payload over one store — identical keys and counts,
// every known status present (zeros included), equal totals. The two
// handlers once computed the shape independently (dashboard: full
// snapshot projection keyed by HTML badge labels; API: raw GROUP BY rows
// with zero-count statuses omitted), so the wire could drift silently in
// three directions. Any divergence — new status missed by either side,
// label rename, total computed differently — now fails here.
func TestStatsSurfacesAgree(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	ctx := context.Background()

	cancelled := enqueue(t, s, "sh", "demo")
	enqueue(t, s, "sh", "demo")
	enqueue(t, s, "sh", "demo")
	enqueue(t, s, "sh", "demo")

	if err := s.Cancel(ctx, cancelled.ID, "contract test"); err != nil {
		t.Fatalf("cancel pending: %v", err)
	}

	claimed, err := s.ClaimDue(ctx, "w1", time.Minute)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}

	if err := s.Complete(ctx, claimed.ID, "w1", json.RawMessage(`"ok"`)); err != nil {
		t.Fatalf("complete: %v", err)
	}

	if _, err := s.ClaimDue(ctx, "w1", time.Minute); err != nil {
		t.Fatalf("claim 2: %v", err)
	}

	srv := New(s, Config{})
	apiServer, err := httpapi.New(s, "sekrit", nil)
	if err != nil {
		t.Fatalf("httpapi.New: %v", err)
	}

	dashStats := getJSONStats(t, srv.Handler(), "/api/stats", "")
	apiStats := getJSONStats(t, apiServer.Handler(), "/api/v1/stats", "sekrit")

	if !maps.Equal(dashStats, apiStats) {
		t.Errorf("stats surfaces diverged:\ndashboard /api/stats    = %v\nwrite API /api/v1/stats = %v", dashStats, apiStats)
	}

	for _, st := range allStatuses {
		if _, ok := apiStats[string(st)]; !ok {
			t.Errorf("stats payload omits status %q: zeros must stay present so producers see a stable key set", st)
		}
	}

	// The route namespaces stay disjoint: /api/v1/* belongs to the write
	// API alone, the unversioned /api/* space to the dashboard.
	for _, route := range srv.routeBindings() {
		if strings.HasPrefix(route.pattern, "/api/v1") {
			t.Errorf("dashboard claims the write API's versioned namespace: %s %s", route.method, route.pattern)
		}
	}

	rec := httptest.NewRecorder()
	apiServer.Handler().ServeHTTP(rec, authedRequest(http.MethodGet, "/api/stats", "sekrit"))

	if rec.Code != http.StatusNotFound {
		t.Errorf("write API serves unversioned /api/stats: status = %d, want 404", rec.Code)
	}
}

// getJSONStats GETs a stats endpoint and decodes the payload.
func getJSONStats(t *testing.T, h http.Handler, path, token string) map[string]int {
	t.Helper()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, authedRequest(http.MethodGet, path, token))

	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s: status = %d, body = %s", path, rec.Code, rec.Body.String())
	}

	var stats map[string]int
	if err := json.Unmarshal(rec.Body.Bytes(), &stats); err != nil {
		t.Fatalf("GET %s: decode stats: %v", path, err)
	}

	return stats
}

// authedRequest builds a GET with the bearer token set (empty token = no
// header).
func authedRequest(method, path, token string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	return req
}
