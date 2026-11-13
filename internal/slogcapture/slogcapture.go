// Package slogcapture provides the slog test double the sweeper suites
// share: a handler that records every record so tests can pin the warning
// a refusal emits without asserting on log plumbing.
package slogcapture

import (
	"context"
	"log/slog"
	"slices"
	"sync"
)

// Handler records the records it receives.
type Handler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *Handler) Enabled(context.Context, slog.Level) bool { return true }

func (h *Handler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.records = append(h.records, r)

	return nil
}

func (h *Handler) WithAttrs([]slog.Attr) slog.Handler { return h }

func (h *Handler) WithGroup(string) slog.Handler { return h }

// Records returns a copy of the captured records.
func (h *Handler) Records() []slog.Record {
	h.mu.Lock()
	defer h.mu.Unlock()

	return slices.Clone(h.records)
}
