// Package postgres is the networked PostgreSQL store driver. Since
// ADR-0019 S1 it is a THIN DRIVER over the queue/v4-backed adapter
// (internal/queue/postgresv4): the hand-rolled engine is retired and the
// upstream go-cqrs-lite queue engine owns the task/fact semantics, with
// token-fenced finalizes (upstream ADR-0134) replacing the legacy
// owner-string fence. The tq Store contract is unchanged for callers.
package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	v4 "github.com/larsartmann/go-taskqueue/internal/queue/postgresv4"
)

// Store is the concrete PostgreSQL store; it satisfies the sibling
// queue facade's Store interface through shared type aliases (ADR-0016).
type Store = v4.Store

// StoreOption configures optional Store behavior.
type StoreOption = v4.StoreOption

// Open connects to dsn (e.g. "postgres://user:pass@host:5432/db") and
// returns a ready store. The maxConns parameter is retained for call-site
// compatibility and ignored: the v4-backed store manages its own two
// handles (one engine write lane, one bounded companion pool).
func Open(ctx context.Context, dsn string, _ int32) (*Store, error) {
	return v4.Open(ctx, dsn)
}

// OpenWithPool wraps a caller-owned pool into a ready store, applying
// the schema on it. The caller keeps pool ownership; consumers with an
// existing pool pass THEIR pool in instead of opening a second one.
func OpenWithPool(ctx context.Context, pool *pgxpool.Pool, opts ...StoreOption) (*Store, error) {
	return v4.OpenWithPool(ctx, pool, opts...)
}
