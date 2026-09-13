// Package postgres is the public facade over the PostgreSQL store driver
// (ADR-0016): the networked backend for shared-machine pools. The
// implementation stays internal; this package pins the names.
package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	internalpostgres "github.com/larsartmann/go-taskqueue/internal/queue/postgres"
)

// Store is the concrete PostgreSQL store; it satisfies the sibling
// queue facade's Store interface through shared type aliases (ADR-0016).
type Store = internalpostgres.Store

var Open = internalpostgres.Open

// OpenWithPool wraps a caller-owned pool into a ready store, applying the
// schema on it. The caller keeps pool ownership; consumers with an
// existing pool pass THEIR pool in instead of opening a second one.
var OpenWithPool = internalpostgres.OpenWithPool

// compile-time proof the pool parameter type is the pgx/v5 pool consumers
// already hold.
var _ = func(p *pgxpool.Pool) error {
	_, err := OpenWithPool(context.Background(), p)
	return err
}
