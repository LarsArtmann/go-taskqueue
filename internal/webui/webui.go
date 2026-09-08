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
	"errors"
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
	// AuthToken, when set, requires every request (pages, API, SSE, static)
	// to present the token via an Authorization: Bearer header or a `token`
	// query parameter. Validate refuses non-loopback binds without it.
	AuthToken string
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

	httpServer *http.Server
}

// New creates a Server over the given store. The store is read-only from
// this package's perspective: only Get, List and Facts are called.
func New(store queue.Store, cfg Config) *Server {
	cfg = cfg.withDefaults()

	return &Server{
		store: store,
		hub:   NewHub(),
		cfg:   cfg,
	}
}

// Handler returns the dashboard's HTTP routes:
//
//	GET /             dashboard page
//	GET /task/{id}    per-task detail page
//	GET /api/events   SSE stream (fragments + resume)
//	GET /api/stats    JSON status counts
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.HandleFunc("GET /task/{id}", s.handleTaskDetail)
	mux.HandleFunc("GET /api/events", s.handleEvents)
	mux.HandleFunc("GET /api/stats", s.handleStats)
	mux.Handle("GET /static/", http.StripPrefix("/static/", staticHandler()))

	var handler http.Handler = mux

	if s.cfg.AuthToken != "" {
		handler = withTokenAuth(s.cfg.AuthToken, handler)
	}

	// Request logging wraps auth so rejected requests are logged too.
	if s.cfg.RequestLog {
		return withRequestLog(handler)
	}

	return handler
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
