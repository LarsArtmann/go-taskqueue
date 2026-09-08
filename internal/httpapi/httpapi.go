// Package httpapi is the v0.2 production API for non-Go producers
// (ADR-0008): enqueue tasks and read queue stats over HTTP with token
// auth. Unlike the read-only dashboard (`tq serve`), this surface WRITES,
// so the token is mandatory on every bind (no loopback exemption: API
// servers are meant to be exposed to other machines).
package httpapi

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// Server is the write API over one store.
type Server struct {
	store queue.Store
	token string
	log   *slog.Logger
}

// New builds a Server. token must be non-empty: the API refuses to start
// without one (fail closed, unlike the dashboard's loopback exemption).
func New(store queue.Store, token string, log *slog.Logger) (*Server, error) {
	if token == "" {
		return nil, errors.New("httpapi: --auth-token is required (this surface writes)")
	}

	if log == nil {
		log = slog.Default()
	}

	return &Server{store: store, token: token, log: log}, nil
}

// Handler returns the routed, auth-guarded API handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/tasks", s.handleEnqueue)
	mux.HandleFunc("GET /api/v1/stats", s.handleStats)
	mux.HandleFunc("GET /api/v1/healthz", s.handleHealth)

	return s.guard(mux)
}

// guard enforces the bearer token on every route (constant-time compare).
// The token may also ride the query (?token=) for clients that cannot set
// headers — same contract as the dashboard stream.
func (s *Server) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		presented := bearerToken(r)

		if subtle.ConstantTimeCompare([]byte(presented), []byte(s.token)) != 1 {
			w.Header().Set("WWW-Authenticate", `Bearer realm="tq-api"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)

			return
		}

		w.Header().Set("Cache-Control", "no-store")

		next.ServeHTTP(w, r)
	})
}

func bearerToken(r *http.Request) string {
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}

	return r.URL.Query().Get("token")
}

// enqueueRequest is the wire contract for POST /api/v1/tasks. Payload is
// the executor-specific JSON document, passed through verbatim.
type enqueueRequest struct {
	Project     string          `json:"project"`
	Type        string          `json:"type"`
	Payload     json.RawMessage `json:"payload"`
	Priority    int             `json:"priority"`
	MaxAttempts int             `json:"maxAttempts"`
	NotBefore   string          `json:"notBefore,omitempty"` // RFC3339; empty = now
	Deps        []string        `json:"deps,omitempty"`
	DedupKey    string          `json:"dedupKey,omitempty"`
}

type enqueueResponse struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("httpapi: encode response", "err", err)
	}
}

func writeError(w http.ResponseWriter, status int, what, fix string) {
	writeJSON(w, status, map[string]string{"error": what, "fix": fix})
}

// handleEnqueue creates one task. Validation is explicit per field so a
// non-Go producer gets actionable errors, not a 500.
func (s *Server) handleEnqueue(w http.ResponseWriter, r *http.Request) {
	var req enqueueRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(
			w,
			http.StatusBadRequest,
			"body is not valid JSON: "+err.Error(),
			"send an enqueueRequest JSON document",
		)

		return
	}

	n := task.New{
		Project:     req.Project,
		Type:        req.Type,
		Payload:     req.Payload,
		Priority:    req.Priority,
		MaxAttempts: req.MaxAttempts,
		DedupKey:    req.DedupKey,
	}

	if req.Type == "" {
		writeError(w, http.StatusBadRequest, "type is required", "name the executor, e.g. \"sh\" or \"agent\"")

		return
	}

	if len(req.Payload) == 0 {
		writeError(w, http.StatusBadRequest, "payload is required", "send the executor's JSON payload document")

		return
	}

	if req.NotBefore != "" {
		when, err := time.Parse(time.RFC3339, req.NotBefore)
		if err != nil {
			writeError(w, http.StatusBadRequest, "notBefore must be RFC3339", "e.g. 2026-09-09T09:00:00Z")

			return
		}

		n.NotBefore = when
	}

	for _, d := range req.Deps {
		if d == "" {
			continue
		}

		n.Deps = append(n.Deps, task.ID(d))
	}

	t, err := queue.New(s.store).Enqueue(r.Context(), n)
	if err != nil {
		writeError(
			w,
			http.StatusInternalServerError,
			"enqueue failed: "+err.Error(),
			"retry; if it persists check the store",
		)

		return
	}

	writeJSON(w, http.StatusCreated, enqueueResponse{
		ID:        t.ID.String(),
		Status:    string(t.Status),
		CreatedAt: t.CreatedAt.UTC().Format(time.RFC3339),
	})
}

// handleStats reports the per-status counts.
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	counts, err := s.store.StatusCounts(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "stats failed: "+err.Error(), "retry")

		return
	}

	out := make(map[string]int, len(counts))
	total := 0

	for st, n := range counts {
		out[string(st)] = n
		total += n
	}

	out["total"] = total

	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ListenAndServe starts the API on addr (e.g. "127.0.0.1:8091") until ctx
// is cancelled.
func (s *Server) ListenAndServe(ctx context.Context, addr string) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_ = srv.Shutdown(shutdownCtx)
	}()

	s.log.Info("httpapi listening", "addr", addr)

	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}
