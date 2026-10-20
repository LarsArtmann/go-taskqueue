package readmodel

import (
	"context"

	metaengine "github.com/larsartmann/go-cqrs-lite/metaengine/v4"
)

// EngineHealthRow is one projection engine's health snapshot — the tq-side
// JSON shape over metaengine's EngineHealth (no upstream types on the wire).
type EngineHealthRow struct {
	Name                string `json:"name"`
	State               string `json:"state"`
	ConsecutiveFailures int    `json:"consecutiveFailures,omitempty"`
	LastError           string `json:"lastError,omitempty"`
}

// ProjectionHealth is the token-gated /health projection section: the
// fold's cursor lag against the journal head, the durable checkpoint
// state, and the projection engines' health (ADR-0019 S3 observability —
// the same surfaces `tq doctor`'s projection section reads, live).
type ProjectionHealth struct {
	// Cursor is the applied journal watermark (JournalCursor).
	Cursor int64 `json:"cursor"`
	// Head is the journal's highest seq at read time.
	Head int64 `json:"head"`
	// Lag is Head - Cursor; 0 means the fold is at the head.
	Lag int64 `json:"lag"`
	// Checkpointed reports whether a durable cursor checkpoint exists in
	// the queue db's watermarks table.
	Checkpointed bool `json:"checkpointed"`
	// Checkpoint is the persisted checkpoint seq (0 when none).
	Checkpoint int64 `json:"checkpointSeq,omitempty"`
	// Engines is the projection store's per-engine health snapshot.
	Engines []EngineHealthRow `json:"engines"`
}

// ProjectionHealth reads the live projection surfaces. Head errors surface
// as lag -1 with the error — a health endpoint must answer, not fail.
func (m *Model) ProjectionHealth(ctx context.Context) ProjectionHealth {
	report := ProjectionHealth{Cursor: m.JournalCursor()}

	head, err := m.src.HeadSeq(ctx)
	if err != nil {
		report.Lag = -1
	} else {
		report.Head = head
		report.Lag = head - report.Cursor
	}

	if seq, exists, err := m.src.Watermark(ctx, CursorConsumer); err == nil && exists {
		report.Checkpointed = true
		report.Checkpoint = seq
	}

	// Health records are lazy (created on first classified failure), so
	// the engine list comes from the stats snapshot — one row per planned
	// engine — with the health record overlaid when one exists.
	engineHealth := m.store.HealthSnapshot()

	engineStats := m.store.GetEngineStats(ctx)
	report.Engines = make([]EngineHealthRow, 0, len(engineStats))

	for _, engine := range engineStats {
		row := EngineHealthRow{
			Name:  engine.Name,
			State: string(metaengine.EngineActive),
		}

		if health, ok := engineHealth[engine.Name]; ok {
			row.State = string(health.State)
			row.ConsecutiveFailures = health.ConsecutiveFailures
			row.LastError = health.LastError
		}

		report.Engines = append(report.Engines, row)
	}

	return report
}
