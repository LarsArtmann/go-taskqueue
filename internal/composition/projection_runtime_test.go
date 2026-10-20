package composition

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/config"
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

	rt, err := NewProjectionRuntime(ctx, store, config.SQLite(dbPath))
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

// TestNewProjectionRuntimeYieldsToLockHolder pins the read-only runtime:
// when the projection home's fold lock is held elsewhere (a stats/api
// folder, a second serve), the runtime mounts a reader — model without
// host, no DLQ sidecar — and releasing the lock lets the next runtime
// win the folder role back.
func TestNewProjectionRuntimeYieldsToLockHolder(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "q.db")

	store, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	t.Cleanup(func() { _ = store.Close() })

	modelPath := readmodel.PathFor(dbPath)

	lock, err := readmodel.AcquireFoldLock(modelPath, 0)
	if err != nil {
		t.Fatalf("hold the fold lock: %v", err)
	}

	rt, err := NewProjectionRuntime(ctx, store, config.SQLite(dbPath))
	if err != nil {
		t.Fatalf("NewProjectionRuntime under a held lock: %v", err)
	}

	if rt.Model == nil || !rt.Model.ReadOnly() {
		t.Fatal("runtime did not mount read-only under the held lock")
	}

	if rt.Host != nil {
		t.Fatal("read-only runtime built a host — a reader cannot fold")
	}

	if err := rt.Close(); err != nil {
		t.Fatalf("close read-only runtime: %v", err)
	}

	if err := lock.Release(); err != nil {
		t.Fatalf("release lock: %v", err)
	}

	folder, err := NewProjectionRuntime(ctx, store, config.SQLite(dbPath))
	if err != nil {
		t.Fatalf("NewProjectionRuntime after release: %v", err)
	}

	t.Cleanup(func() { _ = folder.Close() })

	if folder.Model == nil || folder.Model.ReadOnly() {
		t.Fatal("runtime after release must win the folder role")
	}

	if folder.Host == nil {
		t.Fatal("folder runtime did not build the managed host")
	}

	if err := folder.Host.Start(ctx); err != nil {
		t.Fatalf("start host: %v", err)
	}

	if err := folder.Close(); err != nil {
		t.Fatalf("close folder runtime: %v", err)
	}
}
