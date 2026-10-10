package sqlitev4

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/config"
)

// pragmaInt reads one integer pragma off the store's shared pool.
func pragmaInt(t *testing.T, s *Store, name string) int {
	t.Helper()

	var v int
	if err := s.db.QueryRow("PRAGMA " + name).Scan(&v); err != nil {
		t.Fatalf("PRAGMA %s: %v", name, err)
	}

	return v
}

// TestOpenPragmaPolicy pins the IO policy every steady-state connection
// carries: WAL journaling, checkpoint-only fsyncs (synchronous=NORMAL),
// and the single shared connection the engine and companion surfaces
// serialize on (no second writer pool handoffing the WAL lock).
func TestOpenPragmaPolicy(t *testing.T) {
	s, err := Open(t.TempDir() + "/pragma.db")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	t.Cleanup(func() { _ = s.Close() })

	if got := pragmaInt(t, s, "synchronous"); got != 1 { // 0=OFF 1=NORMAL 2=FULL
		t.Fatalf("synchronous = %d, want 1 (NORMAL)", got)
	}

	var mode string
	if err := s.db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatalf("journal_mode: %v", err)
	}

	if !strings.EqualFold(mode, "wal") {
		t.Fatalf("journal_mode = %q, want wal", mode)
	}

	if n := s.db.Stats().MaxOpenConnections; n != 1 {
		t.Fatalf("MaxOpenConnections = %d, want 1 (shared single connection)", n)
	}
}

// TestOpenPragmaPolicyEscapeHatch proves TQ_SQLITE_SYNC=full restores the
// per-commit fsync tier for operators who want strict durability back.
func TestOpenPragmaPolicyEscapeHatch(t *testing.T) {
	t.Setenv("TQ_SQLITE_SYNC", "full")

	s, err := Open(t.TempDir() + "/strict.db")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	t.Cleanup(func() { _ = s.Close() })

	if got := pragmaInt(t, s, "synchronous"); got != 2 {
		t.Fatalf("synchronous = %d, want 2 (FULL)", got)
	}
}

// TestOpenClaimIndexes pins the companion claim-path partial indexes on
// the shared database after Open: the project-exclusivity probe and the
// expired-lease reclaim must answer from the running set, not the whole
// history (see companion.CompanionIndexes).
func TestOpenClaimIndexes(t *testing.T) {
	s, err := Open(t.TempDir() + "/indexes.db")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	t.Cleanup(func() { _ = s.Close() })

	for _, name := range []string{"idx_tasks_project_running", "idx_tasks_lease_running"} {
		var n int
		if err := s.db.QueryRow(
			`SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = ?`, name,
		).Scan(&n); err != nil {
			t.Fatalf("read sqlite_master: %v", err)
		}

		if n != 1 {
			t.Fatalf("index %s missing after Open (found %d)", name, n)
		}
	}
}

// TestOpenPragmaPolicyInvalid refuses a bogus override at open with an
// actionable message instead of silently running a surprising tier.
func TestOpenPragmaPolicyInvalid(t *testing.T) {
	t.Setenv("TQ_SQLITE_SYNC", "sometimes")

	_, err := Open(t.TempDir() + "/bad.db")
	if err == nil || !strings.Contains(err.Error(), "TQ_SQLITE_SYNC") {
		t.Fatalf("err = %v, want TQ_SQLITE_SYNC validation error", err)
	}
}

// TestOpenWithDeploymentPragmaParity pins the F2 contract for the
// deployment constructor (ADR-0022): it opens the EXACT policy Open
// opens — same NORMAL tier, WAL, single shared connection — with the
// tier arriving resolved in the struct (SyncFull sticks), and the env
// NOT consulted here (config.FromFlags is the ONE TQ_SQLITE_SYNC
// reader; a stale env must not leak past a resolved struct).
func TestOpenWithDeploymentPragmaParity(t *testing.T) {
	t.Run("parity with Open", func(t *testing.T) {
		s, err := OpenWithDeployment(config.SQLite(filepath.Join(t.TempDir(), "q.db")))
		if err != nil {
			t.Fatalf("OpenWithDeployment: %v", err)
		}

		t.Cleanup(func() { _ = s.Close() })

		if got := pragmaInt(t, s, "synchronous"); got != 1 {
			t.Fatalf("synchronous = %d, want 1 (NORMAL)", got)
		}

		if n := s.db.Stats().MaxOpenConnections; n != 1 {
			t.Fatalf("MaxOpenConnections = %d, want 1 (shared single connection)", n)
		}
	})

	t.Run("struct tier wins", func(t *testing.T) {
		d := config.SQLite(filepath.Join(t.TempDir(), "q.db"))
		d.SyncPolicy = config.SyncFull

		s, err := OpenWithDeployment(d)
		if err != nil {
			t.Fatalf("OpenWithDeployment: %v", err)
		}

		t.Cleanup(func() { _ = s.Close() })

		if got := pragmaInt(t, s, "synchronous"); got != 2 {
			t.Fatalf("synchronous = %d, want 2 (FULL from the struct)", got)
		}
	})

	t.Run("env is not read here", func(t *testing.T) {
		t.Setenv("TQ_SQLITE_SYNC", "full")

		s, err := OpenWithDeployment(config.SQLite(filepath.Join(t.TempDir(), "q.db")))
		if err != nil {
			t.Fatalf("OpenWithDeployment: %v", err)
		}

		t.Cleanup(func() { _ = s.Close() })

		if got := pragmaInt(t, s, "synchronous"); got != 1 {
			t.Fatalf("synchronous = %d, want 1 (struct NORMAL; the env must not leak past a resolved deployment)", got)
		}
	})
}
