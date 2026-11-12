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
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/larsartmann/go-datastar"
	"github.com/larsartmann/go-datastar/static"
	health "github.com/larsartmann/go-health"
	dashboard "github.com/larsartmann/go-health-dashboard"
	"github.com/larsartmann/go-sse"
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
	HealthTrendPath     = "/health/trend"
	HealthExportPath    = "/health/export"
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
	// healthProjectionLagTolerance is the fold-lag head gap the projection
	// check tolerates before warning: the tail polls at DefaultPoll and
	// the fold applies within a batch, so only a wedged fold lags wide.
	healthProjectionLagTolerance = 100
	// healthTrendSamples is the trend ring's depth: at the 5s refresh
	// cadence 72 samples ≈ 6 minutes of sparkline — enough to see a flap
	// cycle without unbounded history (fleet review f14: restart amnesia
	// was the gap).
	healthTrendSamples = 72
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
	version   string

	// projectionCursor reports the read model's applied journal watermark
	// when the server folds on one (mounted), and ok=false otherwise —
	// the projection check is emitted only for mounted folds. Nil means
	// the server runs store-backed reads (no check).
	projectionCursor func(ctx context.Context) (cursor int64, mounted bool)

	// foldFollower mirrors Server.foldFollower (a demoted, read-only
	// mount): when set, the projection check becomes the follower
	// staleness contract regardless of the cursor.
	foldFollower *atomic.Bool

	// liveSSE reports the SSE hub's lifecycle snapshot when the server
	// wired one (nil = no check): subscriber/buffer visibility for the
	// /health JSON (fleet review f18 — a closed hub means the live
	// dashboard is dead while the process looks healthy).
	liveSSE func() *sse.BroadcasterHealth

	mu        sync.Mutex
	resp      health.Response
	evalAt    time.Time
	evaluated bool
	startupOK bool
}

func newQueueProber(store queue.Store, version string) *queueProber {
	return &queueProber{
		store:     store,
		startedAt: time.Now(),
		version:   version,
		resp:      health.Response{Status: health.StatusWarn, Version: version, Checks: map[string]health.Check{}},
	}
}

// CachedResponse returns the current health snapshot, re-evaluating when the
// cache is older than healthRefreshInterval.
func (p *queueProber) CachedResponse() health.Response {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	if !p.evaluated || now.Sub(p.evalAt) >= healthRefreshInterval {
		p.evaluateSafely(now)
	}

	return p.resp
}

// evaluateSafely runs one evaluation batch, converting a panicking check
// into a failed response instead of letting it kill the caller: the SSE
// pusher goroutine rides this path on every refresh, and an unrecovered
// panic there takes all of `tq serve` down with it.
func (p *queueProber) evaluateSafely(now time.Time) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("webui: health evaluation panicked — serving degraded response",
				"panic", r, "stack", string(debug.Stack()))
			p.finishEval(now, now, map[string]health.Check{
				"prober": mkCheck(health.StatusFail, fmt.Sprintf("health evaluation panicked: %v", r), 0),
			})
		}
	}()

	p.evaluate(now)
}

// RefreshInterval reports the evaluation cadence; the dashboard uses it as
// the default push interval.
func (p *queueProber) RefreshInterval() time.Duration { return healthRefreshInterval }

// LivenessHandler serves the JSON liveness probe — always 200: the process
// is up, which is all liveness may claim.
func (p *queueProber) LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeHealthJSON(w, http.StatusOK, health.Response{
			Status:  health.StatusPass,
			Version: p.version,
			Uptime:  time.Since(p.startedAt).Round(time.Second).String(),
			Checks:  map[string]health.Check{},
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
	batchStart := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), healthEvalTimeout)
	defer cancel()

	checks := map[string]health.Check{}

	dbStart := time.Now()
	counts, err := p.store.StatusCounts(ctx)

	checks["database"] = mkCheck(statusOr(err == nil, health.StatusPass, health.StatusFail), errText(err), time.Since(dbStart))
	if err != nil {
		// Every check below reads the same store; one verdict beats five
		// identical failures.
		p.finishEval(now, batchStart, checks)

		return
	}

	workersStart := time.Now()
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

	checks["workers"] = mkCheck(workersStatus, workersErr, time.Since(workersStart))

	queueStart := time.Now()
	stuck := queue.CountStuckRunning(ctx, p.store, now)
	checks["queue"] = mkCheck(
		statusOr(stuck == 0, health.StatusPass, health.StatusWarn),
		stuckNote(stuck), time.Since(queueStart))

	dead := counts[task.Dead]
	checks["dlq"] = mkCheck(
		statusOr(dead == 0, health.StatusPass, health.StatusWarn),
		dlqNote(dead), 0)

	if p.projectionCursor != nil {
		projStart := time.Now()
		cursor, mounted := p.projectionCursor(ctx)
		if mounted {
			projDur := time.Since(projStart)
			head, headErr := p.store.HeadSeq(ctx)

			switch {
			case headErr != nil:
				checks["projection"] = mkCheck(health.StatusFail, errText(headErr), projDur)
			case head-cursor > healthProjectionLagTolerance:
				checks["projection"] = mkCheck(health.StatusWarn, fmt.Sprintf(
					"fold %d fact(s) behind the journal head (#%d of #%d) — the fold is wedged or restarting",
					head-cursor, cursor, head), projDur)
			default:
				checks["projection"] = mkCheck(health.StatusPass, fmt.Sprintf("at #%d (head #%d)", cursor, head), projDur)
			}
		}
	}

	if p.foldFollower != nil && p.foldFollower.Load() {
		checks["projection"] = mkCheck(
			health.StatusWarn,
			"fold: follower — read-only projection mount (the fold lock is held by another serve): this dashboard's model is frozen at mount and its SSE stream will not fire; trust the lock-holding dashboard, or restart this serve once the lock frees",
			0,
		)
	}

	if p.liveSSE != nil {
		sseStart := time.Now()
		if hub := p.liveSSE(); hub != nil {
			sseDur := time.Since(sseStart)
			switch {
			case hub.Closed:
				checks["live-sse"] = mkCheck(
					health.StatusFail,
					"SSE hub closed — the live dashboard is dead; restart the serve",
					sseDur,
				)
			case hub.Draining:
				checks["live-sse"] = mkCheck(health.StatusWarn, "SSE hub draining (shutdown in progress)", sseDur)
			default:
				checks["live-sse"] = mkCheck(
					health.StatusPass,
					fmt.Sprintf("%d live subscriber(s), buffer %d event(s)", hub.SubscriberCount, hub.BufferSize),
					sseDur,
				)
			}
		}
	}

	p.finishEval(now, batchStart, checks)
}

// mkCheck builds one check result; go-health grades the overall response
// from the per-check statuses. dur feeds the dashboard's duration column
// (DurationNanos; omitted from the JSON when zero = unknown).
func mkCheck(st health.Status, errMsg string, dur time.Duration) health.Check {
	return health.Check{Status: st, Error: errMsg, DurationNanos: dur.Nanoseconds()}
}

func (p *queueProber) finishEval(now, batchStart time.Time, checks map[string]health.Check) {
	overall := health.StatusPass

	for name, c := range checks {
		switch c.Status {
		case health.StatusFail:
			overall = health.StatusFail
		case health.StatusWarn:
			if overall != health.StatusFail {
				overall = health.StatusWarn
			}
		}

		// StatusSince stamps the transition time: carried from the previous
		// response while the check holds its status, reset to now on every
		// transition (and on first observation) so the dashboard's "since"
		// column is probe-observed truth, not render time.
		if prev, ok := p.resp.Checks[name]; ok && prev.Status == c.Status && !prev.StatusSince.IsZero() {
			c.StatusSince = prev.StatusSince
		} else {
			c.StatusSince = now
		}

		checks[name] = c
	}

	p.resp = health.Response{
		Status:         overall,
		Version:        p.version,
		Uptime:         time.Since(p.startedAt).Round(time.Second).String(),
		Timestamp:      now,
		TotalLatencyMs: time.Since(batchStart).Milliseconds(),
		Checks:         checks,
	}

	p.evalAt = now
	p.evaluated = true

	if checks["database"].Status == health.StatusPass {
		p.startupOK = true
	}
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
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(code)

	// Coerce before marshal: a store error string with invalid UTF-8 would
	// otherwise truncate or poison the JSON body the dashboard parses
	// (pareto T08, 2026-11-12).
	if err := json.MarshalWrite(w, health.CoerceValidUTF8(resp)); err != nil {
		slog.Error("webui: health response marshal failed — serving static fallback body",
			"err", err)
		_, _ = fmt.Fprint(w, `{"status":"fail","error":"health marshal failed"}`)
	}
}

// newHealthDashboard builds the health dashboard wired to tq's surfaces:
// same-origin SDK and stylesheet (the CSP forbids CDNs), nonce extracted
// from the request context that withSecurityHeaders publishes. webhookURL,
// when non-empty (Config.AlertURL), is the loopback health-relay target:
// the dashboard POSTs a snapshot there on every overall status transition,
// and the relay translates it into PapDashboard alerts.
func newHealthDashboard(pro dashboard.Prober, webhookURL string) *dashboard.Dashboard {
	opts := []dashboard.Option{
		dashboard.WithTitle("tq queue health"),
		dashboard.WithRoutes(dashboard.Routes{
			Dashboard:  HealthDashboardPath,
			SSE:        HealthSSEPath,
			Favicon:    HealthFaviconPath,
			Liveness:   HealthLivenessPath,
			Readiness:  HealthReadinessPath,
			Startup:    HealthStartupPath,
			Metrics:    "",
			Trend:      HealthTrendPath,
			Export:     HealthExportPath,
			Introspect: "",
			DatastarJS: healthSDKSrc,
		}),
		dashboard.WithEmbeddedDatastarSDK(),
		dashboard.WithCSSPath("/static/app.css"),
		dashboard.WithTrend(healthTrendSamples),
		dashboard.WithNonceExtractor(func(r *http.Request) string { return ctxNonce(r.Context()) }),
	}

	if webhookURL != "" {
		opts = append(opts, dashboard.WithWebhook(webhookURL))
	}

	return dashboard.New(pro, opts...)
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

// healthSDKSrc is the versioned SDK URL the health dashboard loads. The
// handler pins Cache-Control max-age=86400, so a browser skips ETag
// revalidation for up to 24h; the v query turns each static.Version bump
// into a fresh cache key and collapses that stale window to zero.
var healthSDKSrc = HealthSDKPath + "?v=" + static.Version

// healthSDKScript serves the embedded Datastar client bundle same-origin —
// script-src 'self' covers the load; the policy's 'unsafe-eval' covers the
// run. datastar.ScriptHandler adds the ETag, conditional-GET, and HEAD
// semantics the hand-rolled version lacked.
var healthSDKScript = datastar.ScriptHandler()
