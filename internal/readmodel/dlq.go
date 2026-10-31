package readmodel

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/larsartmann/go-cqrs-lite/projectionhost/v4"
	"github.com/larsartmann/go-taskqueue/internal/config"
)

// DLQPathFor derives the poison-fact sidecar path beside a projection
// database: one DLQ per projection home, discovered by both the serve
// wiring (OpenDeadLetters) and the doctor's projection section.
func DLQPathFor(modelPath string) string {
	return modelPath + ".dlq.db"
}

// dlqPathForDeployment is the ONE sidecar-path derivation for a
// deployment's projection home: the sidecar lives beside the PROJECTION
// db (a queue-home deployment describes the QUEUE db — PathFor maps
// it). Every internal caller routes through here, and external callers
// reach for the named exported shapes (DeploymentDLQPath /
// LegacyDLQPath), so a raw DLQPathFor(PathFor(...)) or queue-home
// composition cannot reappear at a call site (TODO row 56).
func dlqPathForDeployment(d config.Deployment) string {
	return DLQPathFor(PathFor(d.DBPath))
}

// DeploymentDLQPath is the deployment's sidecar path — the shape serve
// opens (OpenDeadLetters) and doctor inspects. Row 56's one-derivation
// seam, exported for the doctor surface.
func DeploymentDLQPath(d config.Deployment) string {
	return dlqPathForDeployment(d)
}

// LegacyDLQPath is the PRE-FIX sidecar shape (beside the QUEUE db) that
// pre-2026-10-11 homes wrote. DETECTION ONLY — never open this shape:
// doctor surfaces it so a stale legacy sidecar cannot hide behind the
// corrected one (TODO row 58).
func LegacyDLQPath(dbPath string) string {
	return DLQPathFor(dbPath)
}

// dlqOpenTimeout bounds the sidecar's schema bootstrap; the DLQ is a
// diagnostic convenience and must never wedge serve startup on a stuck
// sqlite lock the busy timeout already bounded.
const dlqOpenTimeout = 5 * time.Second

// DeadLetters is the projection host's poison-fact sidecar over one
// sqlite file beside the projection database (DLQPathFor). Serve opens
// it for the fold host's DeadLetterStore; doctor surfaces read counts
// and recent entries from the same file. Open creates the file and the
// dead-letter table; Close releases the handle.
type DeadLetters struct {
	store *projectionhost.SQLiteDeadLetterStore
	db    *sql.DB
}

// OpenDeadLetters opens (creating if needed) the DLQ sidecar for the
// projection home described by d (ADR-0022: the deployment is the ONE
// pragma source — the sidecar follows the same sync tier as both
// homes). The handle follows the queue's sqlite posture: WAL, busy
// timeout, MaxOpenConns(1).
func OpenDeadLetters(ctx context.Context, d config.Deployment) (*DeadLetters, error) {
	openCtx, cancel := context.WithTimeout(ctx, dlqOpenTimeout)
	defer cancel()

	dsn := dlqDSN(d)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("readmodel: open dlq sidecar: %w", err)
	}

	db.SetMaxOpenConns(1)

	store, err := projectionhost.NewSQLiteDeadLetterStore(openCtx, db)
	if err != nil {
		_ = db.Close()

		return nil, fmt.Errorf("readmodel: bootstrap dlq sidecar: %w", err)
	}

	return &DeadLetters{store: store, db: db}, nil
}

// dlqDSN is the ONE sidecar-address derivation: the projection-beside
// path (DLQPathFor over PathFor — a queue-home deployment describes the
// QUEUE db, the sidecar lives beside the PROJECTION db) opened at the
// deployment's DLQ pragma tier (ADR-0022: no local pragma literals).
func dlqDSN(d config.Deployment) string {
	// Relaxed-fsync posture like both homes: the sidecar is diagnostic
	// (poison-fact forensics), never authoritative — losing a tail entry
	// to a power cut costs one replay, not data. The tier is the
	// deployment's, not a local literal.
	parts := make([]string, 0, len(d.DLQPragmas()))
	for _, pragma := range d.DLQPragmas() {
		parts = append(parts, "_pragma="+pragma)
	}

	return "file:" + dlqPathForDeployment(d) + "?" + strings.Join(parts, "&")
}

// Store returns the platform DLQ for the projection host's
// DeadLetterStore option; the concrete store satisfies the interface.
func (d *DeadLetters) Store() *projectionhost.SQLiteDeadLetterStore { return d.store }

// Count reports the stored poison facts across every projection.
func (d *DeadLetters) Count(ctx context.Context) (int64, error) {
	count, err := d.store.Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("readmodel: count dlq: %w", err)
	}

	return count, nil
}

// Recent returns up to limit dead-letter entries, newest failure first.
func (d *DeadLetters) Recent(ctx context.Context, limit int) ([]projectionhost.DeadLetterEntry, error) {
	entries, err := d.store.ListPaged(ctx, "", 0, limit)
	if err != nil {
		return nil, fmt.Errorf("readmodel: list dlq: %w", err)
	}

	return entries, nil
}

// Close releases the sidecar's handle.
func (d *DeadLetters) Close() error {
	if err := d.db.Close(); err != nil {
		return fmt.Errorf("readmodel: close dlq sidecar: %w", err)
	}

	return nil
}
