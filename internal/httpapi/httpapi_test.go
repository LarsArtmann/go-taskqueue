package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-cqrs-lite/event/v4"
	errorfamily "github.com/larsartmann/go-error-family"
	"github.com/larsartmann/go-taskqueue/internal/config"
	"github.com/larsartmann/go-taskqueue/internal/incident"
	"github.com/larsartmann/go-taskqueue/internal/lockout"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
	"github.com/larsartmann/go-taskqueue/internal/readmodel"
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
		{"lowercase scheme", "bearer secret-token", "", http.StatusOK},
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

// TestRouteInventory pins the queue-metadata-only ruling (2026-09-28 00-55
// report §a): Handler() mounts EXACTLY the enqueue, stats, and health
// routes — no per-task read route, so derived outcomes (agent results,
// review/status verdicts, session usage) stay dashboard surfaces. A route
// landing without a conscious ruling + package-doc/FEATURES update fails
// here: the mounted paths reject every other method and the per-task read
// shapes stay unrouted. A brand-new path needs its own probe added to this
// table in the same change.
func TestRouteInventory(t *testing.T) {
	t.Parallel()

	srv, _ := newTestAPI(t)
	server := httptest.NewServer(srv.Handler())
	t.Cleanup(server.Close)

	id := "0001a2b3c4d5e6f7"

	cases := []struct {
		name, method, path, body string
		want                     int
	}{
		{
			"enqueue route serves",
			http.MethodPost,
			"/api/v1/tasks",
			`{"project":"p","type":"sh","payload":{}}`,
			http.StatusCreated,
		},
		{"stats route serves", http.MethodGet, "/api/v1/stats", "", http.StatusOK},
		{"healthz route serves", http.MethodGet, "/api/v1/healthz", "", http.StatusOK},

		{"tasks collection is POST-only", http.MethodGet, "/api/v1/tasks", "", http.StatusMethodNotAllowed},
		{"stats is GET-only", http.MethodPost, "/api/v1/stats", "{}", http.StatusMethodNotAllowed},
		{"healthz is GET-only", http.MethodPost, "/api/v1/healthz", "{}", http.StatusMethodNotAllowed},

		{"no per-task read", http.MethodGet, "/api/v1/tasks/" + id, "", http.StatusNotFound},
		{"no per-task write", http.MethodPost, "/api/v1/tasks/" + id, "{}", http.StatusNotFound},
		{"no per-task delete", http.MethodDelete, "/api/v1/tasks/" + id, "", http.StatusNotFound},
		{"no per-task result read", http.MethodGet, "/api/v1/tasks/" + id + "/result", "", http.StatusNotFound},
		{"no singular-task alias", http.MethodGet, "/api/v1/task/" + id, "", http.StatusNotFound},
		{"no tasks subtree", http.MethodGet, "/api/v1/tasks/", "", http.StatusNotFound},
	}

	for _, tc := range cases {
		req, err := http.NewRequestWithContext(t.Context(), tc.method, server.URL+tc.path, strings.NewReader(tc.body))
		if err != nil {
			t.Fatalf("%s: build request: %v", tc.name, err)
		}

		req.Header.Set("Authorization", "Bearer secret-token")

		resp, err := server.Client().Do(req)
		if err != nil {
			t.Fatalf("%s: %s %s: %v", tc.name, tc.method, tc.path, err)
		}

		_ = resp.Body.Close()

		if resp.StatusCode != tc.want {
			t.Errorf("%s: %s %s = %d, want %d", tc.name, tc.method, tc.path, resp.StatusCode, tc.want)
		}
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
	clock := time.Now()
	srv, _ := newTestAPI(t)
	srv.strikes = lockout.New(lockout.Config{
		MaxHits:  authMaxHits,
		Lockout:  40 * time.Millisecond,
		IdleKeep: authIdleKeep,
		MaxKeys:  authMaxKeys,
		Now:      func() time.Time { return clock },
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

	clock = clock.Add(50 * time.Millisecond)

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

// TestQueryTokenFailureStrikes pins the 01-21 report f8 ruling: the
// ?token= fallback is a full bearer peer — wrong query tokens count
// strikes exactly like wrong Authorization headers, and a correct query
// token still clears them.
func TestQueryTokenFailureStrikes(t *testing.T) {
	clock := time.Now()
	srv, _ := newTestAPI(t)
	srv.strikes = lockout.New(lockout.Config{
		MaxHits:  authMaxHits,
		Lockout:  40 * time.Millisecond,
		IdleKeep: authIdleKeep,
		MaxKeys:  authMaxKeys,
		Now:      func() time.Time { return clock },
	})
	h := srv.Handler()

	tryQuery := func(token string) int {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/healthz?token="+token, nil)

		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		return rec.Code
	}

	for i := range 3 {
		if code := tryQuery("wrong"); code != http.StatusUnauthorized {
			t.Fatalf("wrong query token %d = %d, want 401", i+1, code)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/healthz?token=secret-token", nil)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("correct query token under lockout = %d, want 429", rec.Code)
	}

	clock = clock.Add(50 * time.Millisecond)

	if code := tryQuery("secret-token"); code != http.StatusOK {
		t.Fatalf("after lockout expiry = %d, want 200", code)
	}

	tryQuery("wrong")
	tryQuery("wrong")

	if code := tryQuery("secret-token"); code != http.StatusOK {
		t.Fatalf("correct query token with two strikes = %d, want 200 (a success resets the strikes)", code)
	}
}

// TestLockoutCoversEnqueue pins the 01-21 report f9 all-routes ruling:
// the strike guard wraps the whole mux, so a locked client is refused on
// POST /api/v1/tasks too — not just the GET probes.
func TestLockoutCoversEnqueue(t *testing.T) {
	srv, _ := newTestAPI(t)
	srv.strikes = lockout.New(lockout.Config{
		MaxHits:  authMaxHits,
		Lockout:  time.Hour,
		IdleKeep: authIdleKeep,
		MaxKeys:  authMaxKeys,
	})
	h := srv.Handler()

	try := func(method, path, token string) int {
		req := httptest.NewRequest(method, path, strings.NewReader(`{"project":"p","type":"sh","payload":{}}`))
		req.Header.Set("Authorization", "Bearer "+token)

		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		return rec.Code
	}

	for i := range 3 {
		if code := try(http.MethodGet, "/api/v1/healthz", "wrong"); code != http.StatusUnauthorized {
			t.Fatalf("failed auth %d = %d, want 401", i+1, code)
		}
	}

	if code := try(http.MethodPost, "/api/v1/tasks", "secret-token"); code != http.StatusTooManyRequests {
		t.Fatalf("locked enqueue = %d, want 429", code)
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

	if err := srv.UseReadModel(
		context.Background(),
		config.SQLite(filepath.Join(t.TempDir(), "projection.db")),
	); err != nil {
		t.Fatalf("use read model: %v", err)
	}

	if srv.model == nil {
		t.Fatal("UseReadModel did not install the read model")
	}

	// No ListenAndServe here, so nothing else owns the projection: the
	// full close order (host → dlq → model) must run or the leaked DLQ
	// sidecar handle fails TempDir cleanup on windows (open-file unlink).
	t.Cleanup(srv.CloseReadModel)

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

	if err := srv.UseReadModel(
		context.Background(),
		config.SQLite(filepath.Join(t.TempDir(), "projection.db")),
	); err != nil {
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

// TestUseReadModelArmsManagedHost pins the pump decision (ADR-0019
// residual, docs/memos/2026-10-10_readmodel-open-seams.md): UseReadModel
// installs the managed projection host plus the best-effort DLQ sidecar
// at the deployment-derived path, and ListenAndServe starts/stops it
// cleanly. The raw Model.Run loop it replaces had neither protection —
// a poison fact wedged the cursor at repeated error logs forever.
func TestUseReadModelArmsManagedHost(t *testing.T) {
	srv, _ := newTestAPI(t)

	d := config.SQLite(filepath.Join(t.TempDir(), "queue.db"))

	if err := srv.UseReadModel(context.Background(), d); err != nil {
		t.Fatalf("use read model: %v", err)
	}

	if srv.host == nil {
		t.Fatal("UseReadModel did not arm the managed projection host")
	}

	if srv.dlq == nil {
		t.Fatal("UseReadModel did not open the DLQ sidecar")
	}

	if _, err := os.Stat(readmodel.DLQPathFor(readmodel.PathFor(d.DBPath))); err != nil {
		t.Fatalf("dlq sidecar file: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	served := make(chan error, 1)

	go func() { served <- srv.ListenAndServe(ctx, "127.0.0.1:0") }()

	cancel()

	if err := <-served; err != nil {
		t.Fatalf("ListenAndServe = %v, want clean shutdown", err)
	}
}

// TestUseReadModelYieldsToLockHolder pins the api surface's read-only
// mount: when another process owns the projection home's fold lock
// (serve runs), UseReadModel mounts a pure reader — no pump host, no DLQ
// sidecar — while GET /api/v1/stats keeps answering from the projection
// the lock holder converges.
func TestUseReadModelYieldsToLockHolder(t *testing.T) {
	srv, _ := newTestAPI(t)

	d := config.SQLite(filepath.Join(t.TempDir(), "queue.db"))

	lock, err := readmodel.AcquireFoldLock(readmodel.PathFor(d.DBPath), 0)
	if err != nil {
		t.Fatalf("hold the fold lock: %v", err)
	}

	t.Cleanup(func() { _ = lock.Release() })

	if err := srv.UseReadModel(context.Background(), d); err != nil {
		t.Fatalf("use read model: %v", err)
	}

	t.Cleanup(srv.CloseReadModel)

	if srv.host != nil {
		t.Fatal("read-only mount armed a pump host — a reader cannot fold")
	}

	if srv.dlq != nil {
		t.Fatal("read-only mount opened a DLQ sidecar — poison routing belongs to the folder")
	}

	if srv.model == nil || !srv.model.ReadOnly() {
		t.Fatal("UseReadModel did not mount the model read-only under the held lock")
	}

	stats := authorizedStats(t, srv.Handler())
	if stats == nil {
		t.Fatal("stats endpoint did not answer under the read-only mount")
	}

	// The sidecar must not exist: the reader never opened one.
	if _, err := os.Stat(readmodel.DLQPathFor(readmodel.PathFor(d.DBPath))); !os.IsNotExist(err) {
		t.Fatalf("dlq sidecar exists under the read-only mount: %v", err)
	}
}

// apiPoison refuses every event with the fold's corruption
// classification (non-retryable): a deterministic poison fact for the
// api surface's DLQ wire pin.
type apiPoison struct{}

func (apiPoison) Name() string { return "api-poison" }

func (apiPoison) EventTypes() []event.Type {
	return []event.Type{event.Type("task.enqueued")}
}

func (apiPoison) Handle(context.Context, event.Event) error {
	return errorfamily.NewCorruption("test.api-poison", "deliberate poison")
}

// TestUseReadModelDeadLettersPoisonThroughWiredStore pins the api
// surface's poison-DLQ WIRE end-to-end: UseReadModel must actually pass
// the sidecar into ProjectionHostOptions.DeadLetterStore. Dropping that
// one line silently downgrades a poison fact from dead-letter-and-
// converge to restart-budget-exhausted WorkerFailed while the arming
// test (non-nil host/dlq, clean start/stop) stays green. The pin folds a
// poison fact through the api-armed host, asserts a row lands in the
// sidecar at readmodel.DLQPathFor(readmodel.PathFor(d.DBPath)), and
// that GET /api/v1/stats keeps being served across the poison. The
// Count poll also watches the worker-death marker: on a dropped wire
// the poison cannot quarantine, the worker rides the restart budget
// (5 restarts, jittered 1s-16s backoffs) into WorkerFailed, and the
// marker names that exact failure long before the deadline — a bare
// timeout would only say "never dead-lettered".
func TestUseReadModelDeadLettersPoisonThroughWiredStore(t *testing.T) {
	srv, store := newTestAPI(t)
	enqueueTask(t, store, "billing")

	d := config.SQLite(filepath.Join(t.TempDir(), "queue.db"))

	if err := srv.UseReadModel(context.Background(), d); err != nil {
		t.Fatalf("use read model: %v", err)
	}

	if err := srv.host.Register(apiPoison{}); err != nil {
		t.Fatalf("register poison projection: %v", err)
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

	deadline := time.Now().Add(45 * time.Second)

	for {
		n, err := srv.dlq.Count(ctx)
		if err == nil && n > 0 {
			break
		}

		// Dropped-wire fast-fail: without the DeadLetterStore the poison
		// fact rides the restart budget into WorkerFailed, which stamps
		// the death marker beside the projection home. Failing on the
		// marker names the exact degradation; the 45s deadline merely
		// covers worst-case budget exhaustion (~31s of jittered backoffs).
		if marker, exists, _ := readmodel.ReadWorkerFailedMarker(d.DBPath); exists {
			cancel()

			<-served

			t.Fatalf(
				"poison fact hit the restart budget instead of the sidecar (worker %q failed at %s): the opts.DeadLetterStore wire is dropped",
				marker.Projection,
				marker.FailedAt.Format(time.RFC3339),
			)
		}

		if time.Now().After(deadline) {
			cancel()

			<-served

			t.Fatalf(
				"poison fact never dead-lettered (last count %d err %v); the DeadLetterStore wire is dropped or the restart budget ate the fact",
				n,
				err,
			)
		}

		time.Sleep(20 * time.Millisecond)
	}

	// The stats read stays served across the poison: the fold keeps
	// converging past the dead-lettered fact.
	url := "http://" + addr + "/api/v1/stats"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("build stats request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer secret-token")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		cancel()

		<-served

		t.Fatalf("stats after poison: %v", err)
	}

	var stats map[string]int

	err = json.NewDecoder(resp.Body).Decode(&stats)
	_ = resp.Body.Close()

	if err != nil || resp.StatusCode != http.StatusOK {
		cancel()

		<-served

		t.Fatalf("stats after poison = %d %v, want 200 with a body", resp.StatusCode, err)
	}

	cancel()

	if err := <-served; err != nil {
		t.Fatalf("ListenAndServe = %v, want clean shutdown", err)
	}

	// Independent file check: the row lives in the sidecar derived from
	// the deployment, not just in the in-memory handle.
	side, err := readmodel.OpenDeadLetters(context.Background(), d)
	if err != nil {
		t.Fatalf("reopen dlq sidecar: %v", err)
	}

	n, err := side.Count(context.Background())

	closeErr := side.Close()

	if err != nil || n == 0 {
		t.Fatalf("dlq file rows = %d err %v, want the poison fact persisted", n, err)
	}

	if closeErr != nil {
		t.Fatalf("close reopened sidecar: %v", closeErr)
	}
}

// TestUseReadModelReplaysDeletedProjectionHome pins the replay escape
// hatch under the managed host: deleting the projection file leaves the
// surviving checkpoint pointing past an empty home, so UseReadModel
// converges the home with one serial CatchUp before the host's own
// checkpoint load would resume PAST the missing rows.
func TestUseReadModelReplaysDeletedProjectionHome(t *testing.T) {
	srv, store := newTestAPI(t)

	enqueueTask(t, store, "billing")

	d := config.SQLite(filepath.Join(t.TempDir(), "queue.db"))

	// A previous fold: converge the projection home, seal the durable
	// cursor, then delete the file (the documented escape hatch).
	prev, err := readmodel.Open(context.Background(), readmodel.PathFor(d.DBPath), store, readmodel.WithDurableCursor())
	if err != nil {
		t.Fatalf("open previous model: %v", err)
	}

	if err := prev.CatchUp(context.Background()); err != nil {
		t.Fatalf("previous catch up: %v", err)
	}

	if err := prev.Close(); err != nil {
		t.Fatalf("close previous model: %v", err)
	}

	if err := os.Remove(readmodel.PathFor(d.DBPath)); err != nil {
		t.Fatalf("delete projection home: %v", err)
	}

	if err := srv.UseReadModel(context.Background(), d); err != nil {
		t.Fatalf("use read model: %v", err)
	}

	t.Cleanup(srv.CloseReadModel)

	stats := authorizedStats(t, srv.Handler())
	if stats["pending"] != 1 || stats["total"] != 1 {
		t.Fatalf("stats after replay = %v, want the deleted home's rows re-folded", stats)
	}
}

func TestReportErrorContract(t *testing.T) {
	srv, store := newTestAPI(t)

	// Not armed: the route must answer 503, not silently drop reports.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/errors",
		strings.NewReader(`{"project":"webapp","message":"boom"}`))
	req.Header.Set("Authorization", "Bearer secret-token")

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("unarmed status = %d: %s", rec.Code, rec.Body.String())
	}

	// Armed: a valid report journals one error.observed fact.
	srv.UseErrorRecorder(incident.NewRecorder(store))

	body := `{
		"project": "webapp",
		"kind": "client",
		"message": "TypeError: cannot read properties of undefined (reading 'map')",
		"stack": "at OrdersTable (main.a1b2c3.js:4:18771)",
		"release": "a1b2c3d",
		"route": "/orders",
		"traceUrl": "http://signoz.local/trace/abc",
		"breadcrumbs": ["navigate /orders", "click export"]
	}`

	req = httptest.NewRequest(http.MethodPost, "/api/v1/errors", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer secret-token")

	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	var res incident.RecordResult
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if !strings.HasPrefix(res.Incident, "incident:") {
		t.Fatalf("incident id: %q", res.Incident)
	}

	facts, err := store.Facts(context.Background(), 0, 10)
	if err != nil || len(facts) != 1 {
		t.Fatalf("facts: %v %d", err, len(facts))
	}

	if facts[0].Type != "error.observed" || facts[0].TaskID != res.Incident {
		t.Fatalf("fact: %s %s", facts[0].Type, facts[0].TaskID)
	}

	// Invalid reports answer 400 with a fix hint and journal nothing.
	req = httptest.NewRequest(http.MethodPost, "/api/v1/errors",
		strings.NewReader(`{"project":"webapp"}`))
	req.Header.Set("Authorization", "Bearer secret-token")

	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid status = %d: %s", rec.Code, rec.Body.String())
	}

	if !strings.Contains(rec.Body.String(), "fix") {
		t.Fatalf("error response must carry a fix hint: %s", rec.Body.String())
	}

	if facts, _ = store.Facts(context.Background(), 0, 10); len(facts) != 1 {
		t.Fatalf("invalid report journaled a fact: %d", len(facts))
	}
}

// TestHealthzCarriesProjectionSection pins the /health projection section
// (ADR-0019 S3 observability): with a mounted fold the body reports the
// cursor, journal head, lag, durable checkpoint state, and engine health;
// a store-backed server answers bare ok with no projection key.
func TestHealthzCarriesProjectionSection(t *testing.T) {
	t.Run("store-backed", func(t *testing.T) {
		srv, _ := newTestAPI(t)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/healthz", nil)
		req.Header.Set("Authorization", "Bearer secret-token")

		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("healthz = %d, want 200", rec.Code)
		}

		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode body: %v", err)
		}

		if _, ok := body["projection"]; ok {
			t.Errorf("store-backed healthz carries a projection section: %v", body)
		}
	})

	t.Run("mounted fold", func(t *testing.T) {
		srv, store := newTestAPI(t)

		if err := srv.UseReadModel(
			context.Background(),
			config.SQLite(filepath.Join(t.TempDir(), "projection.db")),
		); err != nil {
			t.Fatalf("use read model: %v", err)
		}

		t.Cleanup(srv.CloseReadModel)

		enqueueTask(t, store, "billing")

		if err := srv.model.CatchUp(context.Background()); err != nil {
			t.Fatalf("catch up: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/healthz", nil)
		req.Header.Set("Authorization", "Bearer secret-token")

		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)

		var body struct {
			Status     string `json:"status"`
			Projection struct {
				Cursor       int64 `json:"cursor"`
				Head         int64 `json:"head"`
				Lag          int64 `json:"lag"`
				Checkpointed bool  `json:"checkpointed"`
				Engines      []any `json:"engines"`
			} `json:"projection"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode body: %v", err)
		}

		if body.Status != "ok" {
			t.Errorf("status = %q, want ok", body.Status)
		}

		head, err := store.HeadSeq(context.Background())
		if err != nil {
			t.Fatalf("head seq: %v", err)
		}

		if body.Projection.Cursor != head || body.Projection.Head != head {
			t.Errorf("cursor/head = %d/%d, want %d (fold caught up)",
				body.Projection.Cursor, body.Projection.Head, head)
		}

		if body.Projection.Lag != 0 {
			t.Errorf("lag = %d, want 0", body.Projection.Lag)
		}

		// UseReadModel opens the model WithDurableCursor, so the
		// checkpoint exists once a batch was applied.
		if !body.Projection.Checkpointed {
			t.Error("checkpointed = false, want the durable checkpoint")
		}

		if len(body.Projection.Engines) == 0 {
			t.Error("engines empty, want the projection engine snapshot")
		}
	})
}
