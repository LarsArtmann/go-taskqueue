package readmodel

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/larsartmann/go-cqrs-lite/projectionhost/v4"
)

// DLQPathFor derives the poison-fact sidecar path beside a projection
// database: one DLQ per projection home, discovered by both the serve
// wiring (OpenDeadLetters) and the doctor's projection section.
func DLQPathFor(modelPath string) string {
	return modelPath + ".dlq.db"
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
// projection home at modelPath. The handle follows the queue's sqlite
// posture: WAL, busy timeout, MaxOpenConns(1).
func OpenDeadLetters(ctx context.Context, modelPath string) (*DeadLetters, error) {
	openCtx, cancel := context.WithTimeout(ctx, dlqOpenTimeout)
	defer cancel()

	db, err := sql.Open(
		"sqlite",
		fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)", DLQPathFor(modelPath)),
	)
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

// Store returns the platform DeadLetterStore for the projection host's
// DeadLetterStore option.
func (d *DeadLetters) Store() projectionhost.DeadLetterStore { return d.store }

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
