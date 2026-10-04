package composition

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestNewBuildsSystemRootOverProjectionHome pins the S4 composition:
// system.New succeeds with the empty domain (no second journal — the
// queue engine journal IS the journal since S2), the projection-home
// engine lands beside the queue db, and GracefulClose is clean and
// repeatable across a reopen.
func stat(path string) (os.FileInfo, error) { return os.Stat(path) }

func TestNewBuildsSystemRootOverProjectionHome(t *testing.T) {
	ctx := context.Background()

	dbPath := filepath.Join(t.TempDir(), "tq.db")

	sys, err := New(ctx, dbPath)
	if err != nil {
		t.Fatalf("compose: %v", err)
	}

	if err := sys.GracefulClose(ctx); err != nil {
		t.Fatalf("graceful close: %v", err)
	}

	// The projection-home file exists beside the queue db.
	if _, err := stat(filepath.Join(filepath.Dir(dbPath), "tq.db.readmodel.db")); err != nil {
		t.Fatalf("projection home missing: %v", err)
	}

	// Reopen the same home (serve/api/stats processes share it).
	again, err := New(ctx, dbPath)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}

	if err := again.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}
