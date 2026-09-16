// The health dashboard mounts github.com/larsartmann/go-health-dashboard
// under `tq serve` (2026-09-16 adoption): a live severity-grouped view of
// derived queue health at /health, plus the conventional JSON probes
// (/healthz, /readyz, /startupz). The checks are the store-backed subset of
// `tq doctor` — database, worker heartbeats, expired leases, DLQ depth —
// computed from the same queue.Store projection the dashboard already reads,
// never from process state.
package webui

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/larsartmann/go-datastar/static"
	health "github.com/larsartmann/go-health"
	dashboard "github.com/larsartmann/go-health-dashboard"
	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// Health route paths, mounted by Handler and asserted by the tests.
const (
	HealthDashboardPath = "/health"
	HealthSSEPath       = "/health/sse"
	HealthSDKPath       = "/health/datastar.js"
	HealthFaviconPath   = "/health/favicon.svg"
	HealthLivenessPath  = "/healthz"
	HealthReadinessPath = "/readyz"
	HealthStartupPath   = "/startupz"
)

const (
	// healthRefreshInterval is both the prober's evaluation cache TTL and
	// the dashboard's default SSE push cadence — one ladder.
	healthRefreshInterval = 5 * time.Second
	// healthHeartbeatWindow mirrors `tq doctor`'s worker-liveness window.
	healthHeartbeatWindow = 10 * time.Minute
	// healthEvalTimeout bounds one evaluation batch; a slow store must not
	// stall the SSE pusher goroutine beyond the refresh cadence.
	healthEvalTimeout = 5 * time.Second
)

// queueProber adapts the task queue's store-derived health onto
// dashboard.Prober: CachedResponse is a short-TTL cache over one evaluation
// batch, so N SSE subscribers and probe requests share O(1) store work per
// interval. Implements the liveness/readiness/startup probes with go-health's
// contract: liveness is always 200, readiness 503 on a failing critical
// check, startup 503 until the first successful database evaluation latches.
type queueProber struct {
	store     queue.Store
	startedAt time.Time

	mu        sync.Mutex
	resp      health.Response
	evalAt    time.Time
	evaluated bool
	startupOK bool
}

func newQueueProber(store queue.Store) *queueProber {
	return &queueProber{
		store:     store,
		startedAt: time.Now(),
		resp:      health.Response{Status: health.StatusWarn, Checks: map[string]health.Check{}},
	}
}

// CachedResponse returns the current health snapshot, re-evaluating when the
// cache is older than healthRefreshInterval.
func (p *queueProber) CachedResponse() health.Response {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	if !p.evaluated || now.Sub(p.evalAt) >= healthRefreshInterval {
		p.evaluate(now)
	}

	return p.resp
}

// RefreshInterval reports the evaluation cadence; the dashboard uses it as
// the default push interval.
func (p *queueProber) RefreshInterval() time.Duration { return healthRefreshInterval }

// LivenessHandler serves the JSON liveness probe — always 200: the process
// is up, which is all liveness may claim.
func (p *queueProber) LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeHealthJSON(w, http.StatusOK, health.Response{
			Status: health.StatusPass,
			Uptime: time.Since(p.startedAt).Round(time.Second).String(),
			Checks: map[string]health.Check{},
		})
	}
}

// ReadinessHandler serves the JSON readiness probe — 503 while any critical
// check (the database) is failing.
func (p *queueProber) ReadinessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		resp := p.CachedResponse()

		code := http.StatusOK
		if resp.Status == health.StatusFail {
			code = http.StatusServiceUnavailable
		}

		writeHealthJSON(w, code, resp)
	}
}

// StartupHandler serves the JSON startup probe — 503 until the first
// successful database evaluation latches, then readiness semantics.
func (p *queueProber) StartupHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		resp := p.CachedResponse()

		p.mu.Lock()
		latched := p.startupOK
		p.mu.Unlock()

		code := http.StatusOK
		if !latched {
			code = http.StatusServiceUnavailable
		}

		writeHealthJSON(w, code, resp)
	}
}

// evaluate runs one store-backed check batch (the doctor subset that needs
// nothing but the store).
func (p *queueProber) evaluate(now time.Time) {
	ctx, cancel := context.WithTimeout(context.Background(), healthEvalTimeout)
	defer cancel()

	checks := map[string]health.Check{}

	counts, err := p.store.StatusCounts(ctx)

	checks["database"] = mkCheck(statusOr(err == nil, health.StatusPass, health.StatusFail), errText(err))
	if err != nil {
		// Every check below reads the same store; one verdict beats five
		// identical failures.
		p.finishEval(now, checks)

		return
	}

	beats, err := p.store.CountFacts(ctx, journal.Heartbeat, now.Add(-healthHeartbeatWindow))

	workersErr := ""

	workersStatus := health.StatusPass

	switch {
	case err != nil:
		workersStatus, workersErr = health.StatusFail, errText(err)
	case beats == 0:
		workersStatus, workersErr = health.StatusWarn,
			"no worker heartbeat in "+healthHeartbeatWindow.String()+" — pool idle or down"
	}

	checks["workers"] = mkCheck(workersStatus, workersErr)

	stuck := countStuckRunning(ctx, p.store, now)
	checks["queue"] = mkCheck(
		statusOr(stuck == 0, health.StatusPass, health.StatusWarn),
		stuckNote(stuck))

	dead := counts[task.Dead]
	checks["dlq"] = mkCheck(
		statusOr(dead == 0, health.StatusPass, health.StatusWarn),
		dlqNote(dead))

	p.finishEval(now, checks)
}

// mkCheck builds one check result; go-health grades the overall response
// from the per-check statuses.
func mkCheck(st health.Status, errMsg string) health.Check {
	return health.Check{Status: st, Error: errMsg}
}

func (p *queueProber) finishEval(now time.Time, checks map[string]health.Check) {
	overall := health.StatusPass

	for _, c := range checks {
		switch c.Status {
		case health.StatusFail:
			overall = health.StatusFail
		case health.StatusWarn:
			if overall != health.StatusFail {
				overall = health.StatusWarn
			}
		}
	}

	p.resp = health.Response{
		Status:    overall,
		Uptime:    time.Since(p.startedAt).Round(time.Second).String(),
		Timestamp: now,
		Checks:    checks,
	}

	p.evalAt = now
	p.evaluated = true

	if checks["database"].Status == health.StatusPass {
		p.startupOK = true
	}
}

// countStuckRunning mirrors `tq doctor`'s expired-lease detection: Running
// tasks whose lease expired with no reclaim. A list error counts zero — the
// database check already reports store failures.
func countStuckRunning(ctx context.Context, store queue.Store, now time.Time) int {
	running := task.Running

	tasks, err := store.List(ctx, queue.Filter{Status: &running})
	if err != nil {
		return 0
	}

	stuck := 0

	for _, t := range tasks {
		if t.LeaseExpires != nil && t.LeaseExpires.Before(now) {
			stuck++
		}
	}

	return stuck
}

func stuckNote(stuck int) string {
	if stuck == 0 {
		return ""
	}

	return fmt.Sprintf(
		"%d running task(s) hold an EXPIRED lease and no worker reclaimed them — see tq doctor --mark-orphans",
		stuck,
	)
}

func dlqNote(dead int) string {
	if dead == 0 {
		return ""
	}

	return fmt.Sprintf("%d dead-lettered task(s) — inspect with tq dlq", dead)
}

func statusOr(ok bool, pass, fail health.Status) health.Status {
	if ok {
		return pass
	}

	return fail
}

func errText(err error) string {
	if err == nil {
		return ""
	}

	return err.Error()
}

func writeHealthJSON(w http.ResponseWriter, code int, resp health.Response) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.MarshalWrite(w, resp)
}

// newHealthDashboard builds the health dashboard wired to tq's surfaces:
// same-origin SDK and stylesheet (the CSP forbids CDNs), nonce extracted
// from the request context that withSecurityHeaders publishes.
func newHealthDashboard(pro dashboard.Prober) *dashboard.Dashboard {
	return dashboard.New(pro,
		dashboard.WithTitle("tq queue health"),
		dashboard.WithRoutes(dashboard.Routes{
			Dashboard:  HealthDashboardPath,
			SSE:        HealthSSEPath,
			Favicon:    HealthFaviconPath,
			Liveness:   HealthLivenessPath,
			Readiness:  HealthReadinessPath,
			Startup:    HealthStartupPath,
			Metrics:    "",
			Trend:      "",
			Export:     "",
			Introspect: "",
			DatastarJS: HealthSDKPath,
		}),
		dashboard.WithEmbeddedDatastarSDK(),
		dashboard.WithCSSPath("/static/app.css"),
		dashboard.WithNonceExtractor(func(r *http.Request) string { return ctxNonce(r.Context()) }),
	)
}

// withHealthHeaders adapts the shared security posture for the health
// dashboard's own routes. The Datastar SDK needs 'unsafe-eval', which
// securityHeaders deliberately never grants, so the CSP becomes the health
// dashboard's recommended policy COMPOSED with the task dashboard's
// embed/form hardening: the library's RecommendedCSP under-specifies both
// (no frame-ancestors/form-action at all, base-uri 'self'), so ours is
// stripped and re-appended stricter — CSP is first-occurrence-wins, a pure
// append would be ignored. X-Robots-Tag: noindex gives /health* the same
// crawler defense as the task pages' robots meta (the library head has no
// injection point). It runs INSIDE withSecurityHeaders, so only these
// headers are overridden — nosniff, X-Frame-Options and friends stay. The
// nonce is the same per-request one the security-headers middleware
// published, so the page's inline bootstrap script stays nonce-authorized.
func withHealthHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Robots-Tag", "noindex")
		if nonce := ctxNonce(r.Context()); nonce != "" {
			csp := strings.Replace(dashboard.RecommendedCSP(nonce),
				"base-uri 'self'", "base-uri 'none'", 1)
			csp += "; frame-ancestors 'none'; form-action 'none'"
			w.Header().Set("Content-Security-Policy", csp)
		}

		next.ServeHTTP(w, r)
	})
}

// serveDatastarSDK serves the embedded Datastar client bundle same-origin —
// script-src 'self' covers the load; the policy's 'unsafe-eval' covers the run.
func serveDatastarSDK(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = w.Write(static.Bytes())
}
