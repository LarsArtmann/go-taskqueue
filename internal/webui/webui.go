// Package webui serves a read-only, live-updating web dashboard over the
// task queue: a pure projection consumer of the store's facts and task
// views (ADR-0003). One journal tailer fans out coalesced change
// notifications; each connected browser receives server-rendered HTML
// fragments over SSE and swaps them into the page by container id.
//
// The package cannot mutate the queue: the worst failure is a stale
// dashboard, never journal corruption.
//
//nolint:godoclint // templ-generated _templ.go files also carry package-adjacent comments; counted at package scope
package webui

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/queue"
)

// Default configuration values.
const (
	DefaultAddr      = "127.0.0.1:8090"
	DefaultPoll      = 500 * time.Millisecond
	DefaultHeartbeat = 15 * time.Second

	// BannerPrefix and BannerReadOnlySuffix define the serve startup
	// banner's exact shape — shared by the CLI print and the e2e/smoke
	// parsers, which nearly broke twice over string drift (00:21 e3).
	BannerPrefix         = "tq: dashboard on http://"
	BannerReadOnlySuffix = " (read-only)"
	BannerWritesSuffix   = " (writes ENABLED: cancel/rescue from the UI)"

	readHeaderTimeout = 5 * time.Second
	readTimeout       = 10 * time.Second
	idleTimeout       = 60 * time.Second
	shutdownTimeout   = 5 * time.Second
)

// Config controls the dashboard server.
type Config struct {
	// Addr is the TCP address to bind. Default 127.0.0.1:8090.
	Addr string
	// Poll is the journal tail interval. Default 500ms.
	Poll time.Duration
	// Heartbeat is the SSE keepalive interval. Default 15s.
	Heartbeat time.Duration
	// RequestLog enables per-request access logging (method, path, status,
	// duration) via slog at Info level. Off by default.
	RequestLog bool
	// DailyBudget mirrors the agent pool's --daily-budget so the dashboard
	// can show the spend card (tasks enqueued today vs cap). 0 hides the
	// card — the cap is an operator decision, not queue state.
	DailyBudget int
	// AuthToken, when set, requires every request (pages, API, SSE, static)
	// to present the token via an Authorization: Bearer header or a `token`
	// query parameter. Validate refuses non-loopback binds without it.
	AuthToken string
	// AllowWrites opts in to admin actions from the dashboard (cancel
	// pending/running tasks, rescue dead-lettered ones). ADR-0003's default
	// is a read-only projection; writes are a conscious operator decision
	// guarded by per-form CSRF tokens (withCSRF) and, on non-loopback
	// binds, by the auth token. Off by default.
	AllowWrites bool
}

func (c Config) withDefaults() Config {
	if c.Addr == "" {
		c.Addr = DefaultAddr
	}

	if c.Poll <= 0 {
		c.Poll = DefaultPoll
	}

	if c.Heartbeat <= 0 {
		c.Heartbeat = DefaultHeartbeat
	}

	return c
}

// Server is the read-only live dashboard. Create with New, then call Run.
type Server struct {
	store queue.Store
	hub   *Hub
	cfg   Config

	// writes throttles CSRF brute force on the two write routes.
	writes *writeRateLimiter

	httpServer *http.Server
}

// New creates a Server over the given store. The store is read-only from
// this package's perspective: only Get, List and Facts are called.
func New(store queue.Store, cfg Config) *Server {
	cfg = cfg.withDefaults()

	return &Server{
		store:  store,
		hub:    NewHub(),
		cfg:    cfg,
		writes: newWriteRateLimiter(),
	}
}

// routeBindings is the complete route table as data, so the read-only
// guardrail test can prove the dashboard registers no mutating handler
// (ADR-0003: the worst failure is a stale dashboard, never journal
// corruption). Adding a POST route fails TestRoutesAreReadOnly. The nil
// handler is the static file tree, wired in Handler.
func (s *Server) routeBindings() []struct {
	method  string
	pattern string
	handler func(http.ResponseWriter, *http.Request)
} {
	return []struct {
		method  string
		pattern string
		handler func(http.ResponseWriter, *http.Request)
	}{
		{"GET", "/{$}", s.handleIndex},
		{"GET", "/project/{name}", s.handleProject},
		{"GET", "/task/{id}", s.handleTaskDetail},
		{"GET", "/task/{id}/events", s.handleTaskEvents},
		{"GET", "/api/events", s.handleEvents},
		{"GET", "/api/stats", s.handleStats},
		{"GET", "/api/facts", s.handleFacts},
		{"GET", "/static/", nil}, // staticHandler, wired in Handler
	}
}

// writeBindings is the opt-in admin route table (Config.AllowWrites). It
// lives OUTSIDE routeBindings so the ADR-0003 read-only guardrail test
// keeps proving that the DEFAULT dashboard registers no mutating handler.
func (s *Server) writeBindings() []struct {
	method  string
	pattern string
	handler func(http.ResponseWriter, *http.Request)
} {
	return []struct {
		method  string
		pattern string
		handler func(http.ResponseWriter, *http.Request)
	}{
		{"POST", "/task/{id}/cancel", s.handleTaskCancelPOST},
		{"POST", "/task/{id}/rescue", s.handleTaskRescuePOST},
	}
}

// Handler returns the dashboard's HTTP routes:
//
//	GET /                  dashboard page
//	GET /project/{name}    dashboard pinned to one project
//	GET /task/{id}         per-task detail page
//	GET /task/{id}/events  per-task SSE stream (detail fragments + resume)
//	GET /api/events        SSE stream (dashboard fragments + resume)
//	GET /api/stats         JSON status counts
//	GET /api/facts         JSON journal cursor (?after=SEQ&limit=N)
//
// With Config.AllowWrites the admin routes are added on top:
//
//	POST /task/{id}/cancel  withdraw a pending task / request a running stop
//	POST /task/{id}/rescue  re-queue a dead-lettered task (fresh attempts)
//
// Every response carries strict security headers (securityHeaders); the
// default dashboard is read-only by construction, writes are opt-in and
// CSRF-guarded (withCSRF).
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	for _, route := range s.routeBindings() {
		if route.handler == nil {
			mux.Handle(route.method+" "+route.pattern, http.StripPrefix("/static/", staticHandler()))

			continue
		}

		mux.HandleFunc(route.method+" "+route.pattern, route.handler)
	}

	// The CSRF wrapper is innermost: page GETs issue/refresh the token
	// cookie and publish it to the render chain; write POSTs verify it.
	handler := withCSRFIssue(http.Handler(mux))

	if s.cfg.AllowWrites {
		for _, route := range s.writeBindings() {
			// Rate limit OUTSIDE CSRF: three failed tokens lock the client
			// out of the write routes entirely for a minute, so the token
			// cannot be brute-forced request-by-request.
			mux.Handle(route.method+" "+route.pattern,
				s.writes.wrap(withCSRF(http.HandlerFunc(route.handler))))
		}
	}

	if s.cfg.AuthToken != "" {
		handler = withTokenAuth(s.cfg.AuthToken, handler)
	}

	// Security headers wrap everything — including auth rejections — so
	// even the 401 page is CSP-covered.
	handler = withSecurityHeaders(handler)

	// Request logging wraps auth so rejected requests are logged too.
	if s.cfg.RequestLog {
		return withRequestLog(handler)
	}

	return handler
}

// securityHeaders is the strict default for a dashboard that renders
// untrusted task payloads: nothing remote, nothing framed, scripts only
// same-origin files plus the per-request nonce (templ-components' theme
// bootstrap and toggle ship small inline <script>s that a bare
// script-src 'self' would otherwise block). All assets are same-origin
// files under /static; SSE is same-origin.
func securityHeaders(w http.ResponseWriter, nonce string) {
	h := w.Header()
	h.Set("Content-Security-Policy",
		"default-src 'none'; style-src 'self'; script-src 'self' 'nonce-"+nonce+"'; img-src 'self' data:; "+
			"font-src 'self'; connect-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'none'")
	// The dashboard never needs device capabilities; deny them outright so
	// any future embed/iframe drift cannot grant a rendered payload access.
	h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("X-Frame-Options", "DENY")
}

// nonceCtxKey is the context key under which withSecurityHeaders publishes
// the per-request nonce for the templ components (layout.Base's theme
// bootstrap script, ThemeToggle's inline script).
type nonceCtxKey struct{}

// newNonce generates a fresh 128-bit hex nonce for one request's CSP.
func newNonce() (string, error) {
	var buf [16]byte

	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("read random nonce: %w", err)
	}

	return hex.EncodeToString(buf[:]), nil
}

// ctxNonce returns the per-request CSP nonce the security-headers middleware
// generated, or "" outside that chain (tests, direct renders). With an empty
// nonce the components render without a nonce attribute — and a page served
// without the middleware's CSP would not need one.
func ctxNonce(ctx context.Context) string {
	nonce, _ := ctx.Value(nonceCtxKey{}).(string)

	return nonce
}

// withSecurityHeaders generates a fresh nonce per request, applies
// securityHeaders to every response (including errors produced by inner
// handlers), and publishes the nonce to the render chain via the request
// context.
func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nonce, err := newNonce()
		if err != nil {
			slog.Error("webui: nonce generation failed", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)

			return
		}

		securityHeaders(w, nonce)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), nonceCtxKey{}, nonce)))
	})
}

// Run starts the journal tailer and serves until ctx is cancelled or the
// listener fails. It always shuts the HTTP server down gracefully. It
// refuses to start on a non-loopback bind without a token (Validate).
func (s *Server) Run(ctx context.Context) error {
	if err := s.cfg.Validate(); err != nil {
		return err
	}

	tailerDone := make(chan struct{})

	go func() {
		defer close(tailerDone)

		if err := s.tail(ctx); err != nil && !errors.Is(err, context.Canceled) {
			slog.Error("webui: journal tailer failed", "err", err)
		}
	}()

	s.httpServer = &http.Server{
		Addr:              s.cfg.Addr,
		Handler:           s.Handler(),
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      0, // SSE streams write indefinitely by design
		IdleTimeout:       idleTimeout,
	}

	serveErr := make(chan error, 1)

	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}

		close(serveErr)
	}()

	var runErr error

	select {
	case runErr = <-serveErr:
		if runErr != nil {
			slog.Error("webui: server failed", "err", runErr)
		}
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()

	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		slog.Error("webui: shutdown", "err", err)
	}

	<-tailerDone

	if err := s.hub.Shutdown(shutdownCtx); err != nil {
		slog.Error("webui: hub shutdown", "err", err)
	}

	return runErr
}
