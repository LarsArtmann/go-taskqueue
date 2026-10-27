package postgresv4

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"os"
	"strings"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/queue/companion"
	"github.com/larsartmann/go-taskqueue/internal/queue/companion/conform"
)

// testDSN returns the TQ_TEST_POSTGRES dsn scoped to a fresh per-test
// schema (the sqlite spike used a per-test temp file; postgres gets one
// schema per test so the parallel battery cannot see each other's rows).
// The schema is dropped (cascade) after the store handle is closed.
func testDSN(t *testing.T) string {
	t.Helper()

	base := os.Getenv("TQ_TEST_POSTGRES")
	if base == "" {
		t.Skip("TQ_TEST_POSTGRES unset — postgresv4 spike suite needs a database")
	}

	var seed [8]byte

	if _, err := rand.Read(seed[:]); err != nil {
		t.Fatalf("schema seed: %v", err)
	}

	schema := "tqtest_" + hex.EncodeToString(seed[:])

	admin, err := sql.Open("pgx", base)
	if err != nil {
		t.Fatalf("admin open: %v", err)
	}

	if _, err := admin.ExecContext(context.Background(), "CREATE SCHEMA "+schema); err != nil {
		_ = admin.Close()

		t.Fatalf("create schema %s: %v", schema, err)
	}

	t.Cleanup(func() {
		_, _ = admin.ExecContext(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
		_ = admin.Close()
	})

	if strings.Contains(base, "?") {
		return base + "&search_path=" + schema
	}

	return base + "?search_path=" + schema
}

// TestStoreConformance runs the shared companion conformance suite
// against the postgresv4 spike backend (per-test schema under
// TQ_TEST_POSTGRES; the suite skips itself when the variable is unset —
// CI's env-gated postgres job runs it).
//
// Postgres-proven 2026-10-27: the full suite (incl.
// TestRequeueFactCarriesProvider) passes on a real engine. Local
// recipe (peer-auth-free scratch cluster):
//
//	initdb -D /tmp/tqpg -U tq --auth=trust -E UTF8
//	pg_ctl -D /tmp/tqpg -o "-p 55432 -k /tmp/tqpg -c listen_addresses=''" start
//	createdb -h /tmp/tqpg -p 55432 -U tq tqtest
//	TQ_TEST_POSTGRES='postgres://tq@/tqtest?host=/tmp/tqpg&port=55432' \
//	  (cd internal/queue/postgresv4 && GOWORK=off go test ./... -count=1)
func TestStoreConformance(t *testing.T) {
	conform.StoreSuite(t, conform.Suite{
		FreshDSN: testDSN,
		OpenOn: func(t *testing.T, dsn string, opts ...companion.StoreOption) conform.Store {
			t.Helper()

			s, err := Open(context.Background(), dsn, opts...)
			if err != nil {
				t.Fatalf("Open: %v", err)
			}

			t.Cleanup(func() { _ = s.Close() })

			return s
		},
		Exec: func(ctx context.Context, s conform.Store, query string, args ...any) (sql.Result, error) {
			return s.(*Store).cr.ExecContext(ctx, companion.Postgres(query), args...)
		},
		Begin: func(ctx context.Context, s conform.Store) (*sql.Tx, error) {
			return s.(*Store).db.BeginTx(ctx, nil)
		},
		Dialect: companion.Postgres,
		Caps: conform.Caps{
			Exclusivity:      true,
			ResumeCloseout:   true,
			RequeueClass:     true,
			LegacyMigration:  false,
			Archive:          false,
			EnqueuedSnapshot: false,
		},
	})
}
