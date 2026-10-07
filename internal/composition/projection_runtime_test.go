package composition

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
	"github.com/larsartmann/go-taskqueue/internal/readmodel"
)

// TestNewProjectionRuntime pins the composition-root factory for the S3
// fold surfaces: build, start, close is clean, and Close is a no-op-safe
// teardown (host stop+close, sidecar, model in LIFO order).
func TestNewProjectionRuntime(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "q.db")

	store, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	t.Cleanup(func() { _ = store.Close() })

	rt, err := NewProjectionRuntime(ctx, store, readmodel.PathFor(dbPath))
	if err != nil {
		t.Fatalf("NewProjectionRuntime: %v", err)
	}

	if rt.Model == nil || rt.Host == nil {
		t.Fatalf("runtime incomplete: model=%v host=%v", rt.Model, rt.Host)
	}

	if err := rt.Host.Start(ctx); err != nil {
		t.Fatalf("start host: %v", err)
	}

	if err := rt.Close(); err != nil {
		t.Fatalf("close runtime: %v", err)
	}

	if err := rt.Close(); err != nil {
		t.Errorf("second close = %v, want nil (idempotent teardown)", err)
	}
}
