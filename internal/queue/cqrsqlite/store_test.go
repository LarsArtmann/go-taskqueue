package cqrsqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/queue/companion"
	"github.com/larsartmann/go-taskqueue/internal/queue/companion/conform"
)

// TestStoreConformance runs the shared companion conformance suite
// against the cqrsqlite spike backend (fresh temp-file database per
// test). The S1 divergences ride the capability knobs: no store-level
// project exclusivity and no Requeue resume_closeout evidence key
// upstream yet — those tests skip with their divergence notes.
func TestStoreConformance(t *testing.T) {
	conform.StoreSuite(t, conform.Suite{
		FreshDSN: func(t *testing.T) string {
			t.Helper()

			return filepath.Join(t.TempDir(), "q.db")
		},
		OpenOn: func(t *testing.T, dsn string, opts ...companion.StoreOption) conform.Store {
			t.Helper()

			s, err := Open(dsn)
			if err != nil {
				t.Fatalf("Open: %v", err)
			}

			t.Cleanup(func() { _ = s.Close() })

			return s
		},
		Exec: func(ctx context.Context, s conform.Store, query string, args ...any) (sql.Result, error) {
			return s.(*Store).db.ExecContext(ctx, query, args...)
		},
		Begin: func(ctx context.Context, s conform.Store) (*sql.Tx, error) {
			return s.(*Store).db.BeginTx(ctx, nil)
		},
		Dialect: companion.SQLite,
		Caps: conform.Caps{
			Exclusivity:      false,
			ResumeCloseout:   false,
			LegacyMigration:  true,
			Archive:          false,
			EnqueuedSnapshot: false,
		},
	})
}
