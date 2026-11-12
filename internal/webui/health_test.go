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
	"time"
	"unicode/utf8"

	health "github.com/larsartmann/go-health"
	"github.com/larsartmann/go-sse"
	"github.com/larsartmann/go-taskqueue/internal/journal"
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

func TestHealthVersionStamp(t *testing.T) {
	t.Parallel()

	// The configured version must reach every surface the health mount
	// serves: the dashboard's Version stat card (the 2026-09-16 live gap
	// rendered "unknown" forever because the hand-rolled prober never
	// copied Config.Version into its responses) and all three JSON
	// probes, which dashboard/textfile consumers read instead of the HTML.
	s := New(newTestStore(t), Config{Version: "9.9.9-test"})
	server := httptest.NewServer(s.Handler())
	t.Cleanup(server.Close)

	_, _, page := getBody(t, server.URL+HealthDashboardPath)

	if !strings.Contains(page, "9.9.9-test") {
		t.Errorf("health page missing configured version (stat card renders 'unknown')")
	}

	for _, path := range []string{HealthLivenessPath, HealthReadinessPath, HealthStartupPath} {
		_, _, probe := getBody(t, server.URL+path)

		if !strings.Contains(probe, `"version":"9.9.9-test"`) {
			t.Errorf("%s body missing version stamp, got %s", path, probe)
		}
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

// utf8PoisonStore carries a store error string with raw invalid UTF-8
// bytes: the truncated-body class T08 pins (pareto plan, 2026-11-12).
type utf8PoisonStore struct {
	failingStore
}

func (p *utf8PoisonStore) StatusCounts(_ context.Context) (map[task.Status]int, error) {
	return nil, errors.New("store busy: \xff\xfe corrupted")
}

func TestHealthReadinessPoisonedUTF8StoreError(t *testing.T) {
	t.Parallel()

	s := New(&utf8PoisonStore{}, Config{})
	server := httptest.NewServer(s.Handler())
	t.Cleanup(server.Close)

	code, header, body := getBody(t, server.URL+HealthReadinessPath)

	if code != http.StatusServiceUnavailable {
		t.Fatalf("readiness = %d, want 503", code)
	}

	if got := header.Get("Cache-Control"); got != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", got)
	}

	if !json.Valid([]byte(body)) {
		t.Errorf("readiness body is not valid JSON: %q", body)
	}

	if !utf8.ValidString(body) {
		t.Errorf("readiness body is not valid UTF-8: %q", body)
	}

	if strings.Contains(body, "\xff") {
		t.Errorf("readiness body carries the raw invalid byte 0xff: %q", body)
	}

	if !strings.Contains(body, "store busy") {
		t.Errorf("coercion dropped the error text, got %q", body)
	}
}

// recordingWriter records every Write chunk even when the write fails,
// so the marshal-fallback path can be pinned directly (pareto T08).
type recordingWriter struct {
	header http.Header
	chunks []string
}

func (w *recordingWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}

	return w.header
}

func (w *recordingWriter) WriteHeader(int) {}

func (w *recordingWriter) Write(p []byte) (int, error) {
	w.chunks = append(w.chunks, string(p))

	return 0, errors.New("write failed")
}

func TestHealthWriteJSONMarshalFallback(t *testing.T) {
	t.Parallel()

	w := &recordingWriter{}
	writeHealthJSON(w, http.StatusServiceUnavailable, health.Response{
		Status: health.StatusFail,
		Checks: map[string]health.Check{},
	})

	if len(w.chunks) == 0 {
		t.Fatal("writeHealthJSON wrote nothing")
	}

	fallback := w.chunks[len(w.chunks)-1]

	if fallback != `{"status":"fail","error":"health marshal failed"}` {
		t.Errorf("fallback body = %q, want the static fail body", fallback)
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

// headStore pins the journal head the projection check lags against.
type headStore struct {
	queue.Store

	head int64
}

func (h *headStore) HeadSeq(context.Context) (int64, error) { return h.head, nil }

// TestHealthProjectionCheck pins the fold-lag check: emitted only for a
// mounted read model, pass within the tolerance, warn when the fold
// lags the journal head wide (the wedged-fold signature).
func TestHealthProjectionCheck(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	// Not mounted: the visible off-row with the enable recipe (pareto
	// T17, 2026-11-12) — never a verdict (off grades with pass), so the
	// prober's other checks run and readiness stays green.
	s := New(store, Config{})

	resp := s.prober.CachedResponse()

	proj, mounted := resp.Checks["projection"]
	if !mounted {
		t.Fatal("unmounted serve emits no projection row — the surface is invisible")
	}

	if proj.Status != health.StatusOff {
		t.Fatalf("unmounted projection = %+v, want off", proj)
	}

	if !strings.Contains(proj.Error, "--read-model") {
		t.Errorf("off row lacks the enable recipe: %q", proj.Error)
	}

	// Mounted at head: pass.
	p := newQueueProber(&headStore{Store: store, head: 120}, "test")
	p.projectionCursor = func(context.Context) (int64, bool) { return 118, true }

	resp = p.CachedResponse()

	proj = resp.Checks["projection"]
	if proj.Status != health.StatusPass {
		t.Errorf("at-head projection = %+v, want pass", proj)
	}

	// Mounted and wedged: warn past the tolerance.
	p = newQueueProber(&headStore{Store: store, head: 1000}, "test")
	p.projectionCursor = func(context.Context) (int64, bool) { return 118, true }

	resp = p.CachedResponse()

	proj = resp.Checks["projection"]
	if proj.Status != health.StatusWarn {
		t.Errorf("wedged projection = %+v, want warn", proj)
	}
}

// TestHealthFoldFollowerCheck pins the demoted-serve staleness contract:
// when the server lost the fold-lock election (MarkFoldFollower), the
// projection check becomes the follower warning — the mounted model never
// folds again, so an operator must see the frozen-at-mount surface before
// trusting the dashboard (TODO row: eleventh dispatch §e/§f7).
func TestHealthFoldFollowerCheck(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	s := New(store, Config{})

	resp := s.prober.CachedResponse()
	if _, mounted := resp.Checks["projection"]; mounted {
		t.Fatalf("non-follower projection check emitted: %+v", resp.Checks["projection"])
	}

	s.MarkFoldFollower()

	resp = s.prober.CachedResponse()

	proj := resp.Checks["projection"]
	if proj.Status != health.StatusWarn {
		t.Errorf("follower projection = %+v, want warn", proj)
	}

	if !strings.Contains(proj.Error, "fold: follower") {
		t.Errorf("follower detail = %q, want the fold: follower contract", proj.Error)
	}

	if resp.Status != health.StatusWarn {
		t.Errorf("overall = %q, want warn while demoted", resp.Status)
	}
}

// panickingStore detonates on the first read: the evaluate-recover path.
type panickingStore struct {
	queue.Store
}

func (p *panickingStore) StatusCounts(context.Context) (map[task.Status]int, error) {
	panic("store driver exploded")
}

// TestHealthEvaluationPanicRecovers pins the crash-recover contract: a
// panicking check must degrade the response (fail + a prober check naming
// the panic), never take the SSE pusher — and with it all of `tq serve` —
// down (libs fleet review f11/f12: a pusher panic killed serve).
func TestHealthEvaluationPanicRecovers(t *testing.T) {
	t.Parallel()

	s := New(&panickingStore{Store: newTestStore(t)}, Config{})
	server := httptest.NewServer(s.Handler())
	t.Cleanup(server.Close)

	code, _, body := getBody(t, server.URL+HealthReadinessPath)
	if code != http.StatusServiceUnavailable {
		t.Fatalf("GET readiness after panic = %d, want 503", code)
	}

	for _, want := range []string{"health evaluation panicked", "store driver exploded"} {
		if !strings.Contains(body, want) {
			t.Errorf("degraded body lacks %q: %s", want, body)
		}
	}

	// The process-equivalent: the same server keeps answering after the
	// panic — the prober survived to serve its degraded snapshot.
	if code, _, _ = getBody(t, server.URL+HealthLivenessPath); code != http.StatusOK {
		t.Fatalf("GET liveness after panic = %d, want 200", code)
	}
}

// TestHealthStatusSinceAndLatency pins the probe-observed metadata the
// dashboard renders: StatusSince carries across evaluations while a check
// holds its status, and TotalLatencyMs reflects the batch (fleet review
// f11: dashboard rows rendered empty without them).
func TestHealthStatusSinceAndLatency(t *testing.T) {
	t.Parallel()

	p := newQueueProber(newTestStore(t), "test")

	first := p.CachedResponse()
	if first.TotalLatencyMs < 0 {
		t.Errorf("TotalLatencyMs = %d, want >= 0", first.TotalLatencyMs)
	}

	dbFirst, ok := first.Checks["database"]
	if !ok {
		t.Fatal("database check missing")
	}

	if dbFirst.StatusSince.IsZero() {
		t.Error("first observation must stamp StatusSince")
	}

	p.mu.Lock()
	p.evalAt = time.Now().Add(-2 * healthRefreshInterval) // force refresh
	p.mu.Unlock()

	second := p.CachedResponse()
	if !second.Checks["database"].StatusSince.Equal(dbFirst.StatusSince) {
		t.Errorf("StatusSince drifted while the status held: %v -> %v",
			dbFirst.StatusSince, second.Checks["database"].StatusSince)
	}
}

// TestHealthLiveSSECheck pins the hub-visibility check (fleet review f18):
// pass with the live subscriber census, fail when the hub is closed — a
// dead live dashboard under a healthy process is exactly the silent
// failure the /health JSON exists to catch.
func TestHealthLiveSSECheck(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	p := newQueueProber(store, "test")
	p.liveSSE = func() *sse.BroadcasterHealth {
		return &sse.BroadcasterHealth{SubscriberCount: 2, BufferSize: 64}
	}

	resp := p.CachedResponse()
	if got := resp.Checks["live-sse"]; got.Status != health.StatusPass {
		t.Errorf("live-sse = %+v, want pass", got)
	}

	p = newQueueProber(store, "test")
	p.liveSSE = func() *sse.BroadcasterHealth {
		return &sse.BroadcasterHealth{Closed: true}
	}

	resp = p.CachedResponse()
	if got := resp.Checks["live-sse"]; got.Status != health.StatusFail {
		t.Errorf("closed-hub live-sse = %+v, want fail", got)
	}
}

// wedgedWorkersStore wedges the worker-heartbeat probe on a channel and
// ignores its context: the abandonment shape T16 pins (pareto plan,
// 2026-11-12) needs a probe that ignores its deadline on purpose.
type wedgedWorkersStore struct {
	queue.Store
	release chan struct{}
}

func (w *wedgedWorkersStore) CountFacts(_ context.Context, _ journal.FactType, _ time.Time) (int64, error) {
	<-w.release

	return 0, errors.New("unreachable: probe was abandoned, not completed")
}

// TestHealthSlowCheckAbandonedAtBatchDeadline pins the per-check
// abandonment (pareto T16): a workers probe wedged past the batch
// deadline fails closed with the abandonment message while the batch
// still completes — database stays pass, every remaining check lands in
// the snapshot, nothing stalls past the deadline.
func TestHealthSlowCheckAbandonedAtBatchDeadline(t *testing.T) {
	release := make(chan struct{})
	s := &wedgedWorkersStore{Store: newTestStore(t), release: release}
	t.Cleanup(func() { close(release) })

	prev := healthEvalTimeout
	healthEvalTimeout = 50 * time.Millisecond
	t.Cleanup(func() { healthEvalTimeout = prev })

	p := newQueueProber(s, "abandon-test")
	p.evaluateSafely(time.Now())

	p.mu.Lock()
	resp := p.resp
	p.mu.Unlock()

	db, ok := resp.Checks["database"]
	if !ok || db.Status != health.StatusPass {
		t.Fatalf("database check = %+v, want present pass (nothing wedged there)", db)
	}

	workers, ok := resp.Checks["workers"]
	if !ok || workers.Status != health.StatusFail {
		t.Fatalf("workers check = %+v, want present fail (the wedged probe)", workers)
	}

	if !strings.Contains(workers.Error, "did not finish before the batch deadline") {
		t.Errorf("workers error = %q, want the abandonment message", workers.Error)
	}

	for _, name := range []string{"queue", "dlq"} {
		if _, ok := resp.Checks[name]; !ok {
			t.Errorf("%s check missing — the batch did not complete past the abandoned workers probe", name)
		}
	}
}
