package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/lockout"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

func newTestAPI(t *testing.T) (*Server, *sqlite.Store) {
	t.Helper()

	s, err := sqlite.Open(filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	t.Cleanup(func() { _ = s.Close() })

	srv, err := New(s, "secret-token", nil)
	if err != nil {
		t.Fatalf("new api: %v", err)
	}

	return srv, s
}

func TestTokenRequiredAtConstruction(t *testing.T) {
	if _, err := New(nil, "", nil); err == nil {
		t.Fatal("empty token must refuse to build the API (it writes)")
	}
}

func TestAuthMatrix(t *testing.T) {
	srv, _ := newTestAPI(t)
	h := srv.Handler()

	cases := []struct {
		name, header, query string
		want                int
	}{
		{"missing token", "", "", http.StatusUnauthorized},
		{"wrong token", "Bearer nope", "", http.StatusUnauthorized},
		{"bearer token", "Bearer secret-token", "", http.StatusOK},
		{"query token", "", "token=secret-token", http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			url := "/api/v1/healthz"
			if tc.query != "" {
				url += "?" + tc.query
			}

			req := httptest.NewRequest(http.MethodGet, url, nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}

			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d", rec.Code, tc.want)
			}
		})
	}
}

func TestEnqueueContract(t *testing.T) {
	srv, store := newTestAPI(t)
	h := srv.Handler()

	body := `{
		"project": "billing",
		"type": "sh",
		"payload": {"cmd": "echo hi"},
		"priority": 5,
		"maxAttempts": 2,
		"deps": [],
		"dedupKey": "api-1"
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer secret-token")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	var resp enqueueResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	got, err := store.Get(context.Background(), task.ID(resp.ID))
	if err != nil {
		t.Fatalf("stored task missing: %v", err)
	}

	if got.Project != "billing" || got.Type != "sh" || got.Priority != 5 || got.MaxAttempts != 2 {
		t.Fatalf("stored task = %+v", got)
	}

	if string(got.Payload) != `{"cmd": "echo hi"}` {
		t.Fatalf("payload passed through verbatim, got %s", got.Payload)
	}
}

func TestEnqueueValidation(t *testing.T) {
	srv, _ := newTestAPI(t)
	h := srv.Handler()

	cases := []struct {
		name, body string
	}{
		{"missing type", `{"payload":{}}`},
		{"missing payload", `{"type":"sh"}`},
		{"bad notBefore", `{"type":"sh","payload":{},"notBefore":"tomorrow"}`},
		{"not json", `nope`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks", bytes.NewReader([]byte(tc.body)))
			req.Header.Set("Authorization", "Bearer secret-token")

			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d (%s), want 400 with an actionable error", rec.Code, rec.Body.String())
			}

			var errDoc struct {
				Error string `json:"error"`
				Fix   string `json:"fix"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &errDoc); err != nil || errDoc.Error == "" || errDoc.Fix == "" {
				t.Fatalf("error doc = %s, want {error, fix}", rec.Body.String())
			}
		})
	}
}

func TestDedupKeyIdempotency(t *testing.T) {
	srv, store := newTestAPI(t)
	h := srv.Handler()

	post := func() string {
		body := `{"project":"p","type":"sh","payload":{},"dedupKey":"same"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer secret-token")

		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		var resp enqueueResponse

		_ = json.Unmarshal(rec.Body.Bytes(), &resp)

		return resp.ID
	}

	first, second := post(), post()
	if first == "" || first != second {
		t.Fatalf("dedup key not idempotent: %q vs %q", first, second)
	}

	// Exactly one task row exists.
	pending := task.Pending

	tasks, err := store.List(context.Background(), queue.Filter{Project: new("p"), Status: &pending})
	if err != nil || len(tasks) != 1 {
		t.Fatalf("tasks = %d (%v), want 1", len(tasks), err)
	}
}

//go:fix inline
func ptr(s string) *string { return new(s) }

// TestNosniffOnEveryResponse pins the 03-05 report f2 hardening: every
// response the API can emit — auth failures included — carries
// X-Content-Type-Options: nosniff, not just the happy paths.
func TestNosniffOnEveryResponse(t *testing.T) {
	srv, _ := newTestAPI(t)
	h := srv.Handler()

	cases := []struct {
		name, method, path string
		auth               bool
	}{
		{"auth failure", http.MethodGet, "/api/v1/healthz", false},
		{"healthz", http.MethodGet, "/api/v1/healthz", true},
		{"stats", http.MethodGet, "/api/v1/stats", true},
		{"validation error", http.MethodPost, "/api/v1/tasks", true},
		{"unknown route", http.MethodGet, "/api/v1/nope", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{}`))
			if tc.auth {
				req.Header.Set("Authorization", "Bearer secret-token")
			}

			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
				t.Fatalf("%s %s: X-Content-Type-Options = %q, want nosniff", tc.method, tc.path, got)
			}
		})
	}
}

// TestAuthLockout pins the 03-05 report f3 decision: three failed bearer
// auths lock the client out of ALL routes for the window — a valid token
// during the lockout still gets 429 — and access returns once it expires.
func TestAuthLockout(t *testing.T) {
	srv, _ := newTestAPI(t)
	srv.strikes = lockout.New(lockout.Config{
		MaxHits:  authMaxHits,
		Lockout:  40 * time.Millisecond,
		IdleKeep: authIdleKeep,
		MaxKeys:  authMaxKeys,
	})
	h := srv.Handler()

	try := func(token string) int {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/healthz", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		return rec.Code
	}

	for i := range 3 {
		if code := try("wrong"); code != http.StatusUnauthorized {
			t.Fatalf("failed auth %d = %d, want 401", i+1, code)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/healthz", nil)
	req.Header.Set("Authorization", "Bearer secret-token")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("locked valid token = %d, want 429", rec.Code)
	}

	if rec.Header().Get("Retry-After") == "" {
		t.Fatal("429 must carry Retry-After")
	}

	time.Sleep(60 * time.Millisecond)

	if code := try("secret-token"); code != http.StatusOK {
		t.Fatalf("after lockout expiry = %d, want 200", code)
	}
}

// TestAuthLockoutIsPerClient: one client's lockout must not muzzle another.
func TestAuthLockoutIsPerClient(t *testing.T) {
	srv, _ := newTestAPI(t)
	srv.strikes = lockout.New(lockout.Config{
		MaxHits:  authMaxHits,
		Lockout:  time.Hour,
		IdleKeep: authIdleKeep,
		MaxKeys:  authMaxKeys,
	})
	h := srv.Handler()

	try := func(addr, token string) int {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/healthz", nil)
		req.RemoteAddr = addr
		req.Header.Set("Authorization", "Bearer "+token)

		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		return rec.Code
	}

	for i := range 3 {
		if code := try("10.0.0.1:1000", "wrong"); code != http.StatusUnauthorized {
			t.Fatalf("attacker failed auth %d = %d, want 401", i+1, code)
		}
	}

	if code := try("10.0.0.1:1000", "secret-token"); code != http.StatusTooManyRequests {
		t.Fatalf("locked client = %d, want 429", code)
	}

	if code := try("10.0.0.2:2000", "secret-token"); code != http.StatusOK {
		t.Fatalf("other client = %d, want 200 (lockout is per client IP)", code)
	}
}

// TestAuthStrikesResetOnSuccess mirrors the dashboard limiter: a successful
// auth clears the strikes, so scattered one-off failures never lock out a
// legitimate producer.
func TestAuthStrikesResetOnSuccess(t *testing.T) {
	srv, _ := newTestAPI(t)
	srv.strikes = lockout.New(lockout.Config{
		MaxHits:  authMaxHits,
		Lockout:  time.Hour,
		IdleKeep: authIdleKeep,
		MaxKeys:  authMaxKeys,
	})
	h := srv.Handler()

	try := func(token string) int {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/healthz", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		return rec.Code
	}

	try("wrong")
	try("wrong")

	if code := try("secret-token"); code != http.StatusOK {
		t.Fatalf("valid token with two strikes = %d, want 200", code)
	}

	try("wrong")
	try("wrong")

	if code := try("secret-token"); code != http.StatusOK {
		t.Fatalf("valid token after 2+2 strikes = %d, want 200 (a success resets the strikes)", code)
	}
}

// enqueueTask seeds one task through the store directly.
func enqueueTask(t *testing.T, store *sqlite.Store, project string) task.Task {
	t.Helper()

	tk, err := store.Enqueue(context.Background(), task.New{
		Type:    "sh",
		Project: project,
		Payload: json.RawMessage(`"echo hi"`),
	})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	return tk
}

func authorizedStats(t *testing.T, h http.Handler) map[string]int {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)
	req.Header.Set("Authorization", "Bearer secret-token")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("stats status = %d, body: %s", rec.Code, rec.Body)
	}

	var stats map[string]int
	if err := json.Unmarshal(rec.Body.Bytes(), &stats); err != nil {
		t.Fatalf("decode stats: %v", err)
	}

	return stats
}

// TestStatsReadModelSeam pins the ADR-0019 S3 stats seam: with
// UseReadModel the /api/v1/stats read comes from the projection's planned
// table (zero-filled over task.AllStatuses, store-identical), not from the
// queue store.
func TestStatsReadModelSeam(t *testing.T) {
	srv, store := newTestAPI(t)

	if err := srv.UseReadModel(filepath.Join(t.TempDir(), "projection.db")); err != nil {
		t.Fatalf("use read model: %v", err)
	}

	if srv.model == nil {
		t.Fatal("UseReadModel did not install the read model")
	}

	enqueueTask(t, store, "billing")

	h := srv.Handler()

	if stats := authorizedStats(t, h); stats["total"] != 0 {
		t.Fatalf("stats before catch-up = %v, want the empty projection", stats)
	}

	if err := srv.model.CatchUp(context.Background()); err != nil {
		t.Fatalf("catch up: %v", err)
	}

	stats := authorizedStats(t, h)

	if stats["pending"] != 1 || stats["total"] != 1 {
		t.Fatalf("stats = %v, want pending=1 total=1", stats)
	}

	if _, ok := stats["cancelled"]; !ok {
		t.Errorf("stats missing the zero-filled cancelled key: %v", stats)
	}

	storeCounts, err := store.StatusCounts(context.Background())
	if err != nil {
		t.Fatalf("store counts: %v", err)
	}

	if stats[string(task.Pending)] != storeCounts[task.Pending] {
		t.Errorf("model %v vs store %v diverge", stats, storeCounts)
	}
}

// TestListenAndServePumpsReadModel pins pump ownership: the serve window
// tails the journal into the projection, so a fact appended after startup
// shows up in the stats without any manual catch-up.
func TestListenAndServePumpsReadModel(t *testing.T) {
	srv, store := newTestAPI(t)

	if err := srv.UseReadModel(filepath.Join(t.TempDir(), "projection.db")); err != nil {
		t.Fatalf("use read model: %v", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}

	addr := ln.Addr().String()

	if err := ln.Close(); err != nil {
		t.Fatalf("close port probe: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	served := make(chan error, 1)

	go func() { served <- srv.ListenAndServe(ctx, addr) }()

	enqueueTask(t, store, "billing")

	url := "http://" + addr + "/api/v1/stats"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer secret-token")

	deadline := time.Now().Add(5 * time.Second)

	for {
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			var stats map[string]int

			err = json.NewDecoder(resp.Body).Decode(&stats)
			_ = resp.Body.Close()

			if err == nil && stats["total"] == 1 {
				break
			}
		}

		if time.Now().After(deadline) {
			cancel()
			t.Fatalf("stats never reflected the pump (last err: %v)", err)
		}

		time.Sleep(20 * time.Millisecond)

		req, err = http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			t.Fatalf("rebuild request: %v", err)
		}

		req.Header.Set("Authorization", "Bearer secret-token")
	}

	cancel()

	if err := <-served; err != nil {
		t.Errorf("ListenAndServe = %v, want clean shutdown", err)
	}
}
