package webui

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// failingStore fails every store read: the database-check-degraded path.
type failingStore struct {
	queue.Store
}

func (f *failingStore) StatusCounts(_ context.Context) (map[task.Status]int, error) {
	return nil, errStoreDown
}

var errStoreDown = errors.New("store down")

func getBody(t *testing.T, url string) (int, http.Header, string) {
	t.Helper()

	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}

	defer resp.Body.Close()

	var body strings.Builder
	if _, err := io.Copy(&body, resp.Body); err != nil {
		t.Fatalf("read body: %v", err)
	}

	return resp.StatusCode, resp.Header, body.String()
}

func TestHealthDashboardPage(t *testing.T) {
	t.Parallel()

	s := New(newTestStore(t), Config{})
	server := httptest.NewServer(s.Handler())
	t.Cleanup(server.Close)

	code, header, body := getBody(t, server.URL+HealthDashboardPath)

	if code != http.StatusOK {
		t.Fatalf("GET /health = %d, want 200", code)
	}

	csp := header.Get("Content-Security-Policy")
	for _, want := range []string{"default-src 'self'", "'unsafe-eval'", "'nonce-"} {
		if !strings.Contains(csp, want) {
			t.Errorf("health CSP missing %s, got %q", want, csp)
		}
	}

	// The library's RecommendedCSP under-specifies the embed/form posture
	// the task dashboard enforces; the override must compose it in, and its
	// laxer base-uri 'self' must be GONE (CSP is first-occurrence-wins, so
	// an append-instead-of-replace regression would leave 'self' winning).
	for _, want := range []string{"base-uri 'none'", "frame-ancestors 'none'", "form-action 'none'"} {
		if !strings.Contains(csp, want) {
			t.Errorf("health CSP missing composed %s, got %q", want, csp)
		}
	}
	if strings.Contains(csp, "base-uri 'self'") {
		t.Errorf("health CSP kept the library's lax base-uri, got %q", csp)
	}
	if header.Get("X-Robots-Tag") != "noindex" {
		t.Errorf("health page missing X-Robots-Tag noindex, got %q", header.Get("X-Robots-Tag"))
	}

	if strings.Contains(csp, "default-src 'none'") {
		t.Errorf("health CSP must be the dashboard policy, got %q", csp)
	}

	for _, want := range []string{"tq queue health", HealthSDKPath, "/static/app.css"} {
		if !strings.Contains(body, want) {
			t.Errorf("health page missing %q", want)
		}
	}
}

func TestHealthTaskDashboardCSPUnchanged(t *testing.T) {
	t.Parallel()

	s := New(newTestStore(t), Config{})
	server := httptest.NewServer(s.Handler())
	t.Cleanup(server.Close)

	_, header, _ := getBody(t, server.URL+"/api/stats")

	if csp := header.Get("Content-Security-Policy"); !strings.Contains(csp, "default-src 'none'") {
		t.Errorf("task dashboard CSP drifted, got %q", csp)
	}
}

func TestHealthProbesHealthyStore(t *testing.T) {
	t.Parallel()

	s := New(newTestStore(t), Config{})
	server := httptest.NewServer(s.Handler())
	t.Cleanup(server.Close)

	// The empty store is warn-overall (no worker heartbeats) but readiness
	// only gates on fail — a quiet queue is not a down queue.
	for path, wantCode := range map[string]int{
		HealthLivenessPath:  http.StatusOK,
		HealthReadinessPath: http.StatusOK,
		HealthStartupPath:   http.StatusOK,
	} {
		code, _, body := getBody(t, server.URL+path)

		if code != wantCode {
			t.Fatalf("GET %s = %d, want %d (body %s)", path, code, wantCode, body)
		}
	}

	_, _, body := getBody(t, server.URL+HealthReadinessPath)

	var resp struct {
		Status string `json:"status"`
		Checks map[string]struct {
			Status string `json:"status"`
			Error  string `json:"error"`
		} `json:"checks"`
	}
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("decode readiness body: %v (%s)", err, body)
	}

	if resp.Status != "warn" {
		t.Errorf("overall = %q, want warn (idle queue)", resp.Status)
	}

	if db := resp.Checks["database"]; db.Status != "pass" {
		t.Errorf("database check = %+v, want pass", db)
	}

	if w := resp.Checks["workers"]; w.Status != "warn" {
		t.Errorf("workers check = %+v, want warn on empty journal", w)
	}

	if _, _, liveness := getBody(t, server.URL+HealthLivenessPath); !strings.Contains(liveness, `"status":"pass"`) {
		t.Errorf("liveness body %q, want status pass", liveness)
	}
}

func TestHealthProbesStoreFailure(t *testing.T) {
	t.Parallel()

	s := New(&failingStore{}, Config{})
	server := httptest.NewServer(s.Handler())
	t.Cleanup(server.Close)

	if code, _, body := getBody(t, server.URL+HealthReadinessPath); code != http.StatusServiceUnavailable {
		t.Fatalf("readiness = %d, want 503 (body %s)", code, body)
	}

	if code, _, _ := getBody(t, server.URL+HealthStartupPath); code != http.StatusServiceUnavailable {
		t.Fatalf("startup = %d, want 503 while database never latched", code)
	}

	if code, _, _ := getBody(t, server.URL+HealthLivenessPath); code != http.StatusOK {
		t.Fatalf("liveness = %d, want 200 always", code)
	}
}

func TestHealthRoutesBehindAuth(t *testing.T) {
	t.Parallel()

	s := New(newTestStore(t), Config{AuthToken: "sekrit"})
	server := httptest.NewServer(s.Handler())
	t.Cleanup(server.Close)

	for _, path := range []string{HealthDashboardPath, HealthReadinessPath, HealthLivenessPath} {
		code, _, _ := getBody(t, server.URL+path)

		if code != http.StatusUnauthorized {
			t.Errorf("GET %s unauthenticated = %d, want 401 (no unauthenticated oracle)", path, code)
		}

		code, _, _ = getBody(t, server.URL+path+"?token=sekrit")

		if code != http.StatusOK {
			t.Errorf("GET %s with token = %d, want 200", path, code)
		}
	}
}

func TestHealthStaticAssets(t *testing.T) {
	t.Parallel()

	s := New(newTestStore(t), Config{})
	server := httptest.NewServer(s.Handler())
	t.Cleanup(server.Close)

	code, header, body := getBody(t, server.URL+HealthSDKPath)

	if code != http.StatusOK {
		t.Fatalf("GET %s = %d", HealthSDKPath, code)
	}

	if ct := header.Get("Content-Type"); !strings.Contains(ct, "text/javascript") {
		t.Errorf("SDK content-type = %q", ct)
	}

	if len(body) == 0 {
		t.Fatal("empty SDK bundle")
	}

	code, header, _ = getBody(t, server.URL+HealthFaviconPath)

	if code != http.StatusOK {
		t.Fatalf("GET %s = %d", HealthFaviconPath, code)
	}

	if ct := header.Get("Content-Type"); !strings.Contains(ct, "image/svg") {
		t.Errorf("favicon content-type = %q", ct)
	}

	// The pusher is not started outside Run; the SSE route must still be
	// mounted (anything but 404 proves registration).
	if code, _, _ := getBody(t, server.URL+HealthSSEPath); code == http.StatusNotFound {
		t.Fatalf("GET %s = 404, route not mounted", HealthSSEPath)
	}
}
