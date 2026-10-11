package readmodel_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
	"github.com/larsartmann/go-taskqueue/internal/readmodel"
)

// TestWorkerFailedMarkerRoundTrip pins the sidecar contract: absent
// reads healthy, write-then-read preserves both fields, the file lands
// at the documented path beside the projection db, and no temp file
// leaks from the atomic publish.
func TestWorkerFailedMarkerRoundTrip(t *testing.T) {
	t.Parallel()

	dbPath := t.TempDir() + "/queue.db"

	if _, exists, err := readmodel.ReadWorkerFailedMarker(dbPath); err != nil {
		t.Fatalf("read absent marker: %v", err)
	} else if exists {
		t.Fatal("marker exists before any write")
	}

	stamp := readmodel.WorkerFailedMarker{
		Projection: "readmodel",
		FailedAt:   time.Now().UTC().Truncate(time.Second),
	}

	if err := readmodel.WriteWorkerFailedMarker(dbPath, stamp); err != nil {
		t.Fatalf("write marker: %v", err)
	}

	if _, err := os.Stat(readmodel.WorkerFailedMarkerPath(dbPath)); err != nil {
		t.Fatalf("marker file: %v", err)
	}

	if _, err := os.Stat(readmodel.WorkerFailedMarkerPath(dbPath) + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("temp file leaked: %v", err)
	}

	got, exists, err := readmodel.ReadWorkerFailedMarker(dbPath)
	if err != nil {
		t.Fatalf("read marker: %v", err)
	}

	if !exists {
		t.Fatal("marker missing after write")
	}

	if got.Projection != stamp.Projection || !got.FailedAt.Equal(stamp.FailedAt) {
		t.Fatalf("round trip = %+v, want %+v", got, stamp)
	}
}

// TestProjectionHostClearsWorkerDeathMarker pins the lifecycle half of
// the worker-death surface: a standing marker means the fold died and
// nobody restarted it, so a fresh host (a fresh restart budget) must
// clear it at construction.
func TestProjectionHostClearsWorkerDeathMarker(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	dbPath := t.TempDir() + "/queue.db"

	store, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	t.Cleanup(func() { _ = store.Close() })

	stamp := readmodel.WorkerFailedMarker{Projection: "readmodel", FailedAt: time.Now().UTC()}
	if err := readmodel.WriteWorkerFailedMarker(dbPath, stamp); err != nil {
		t.Fatalf("stage stale marker: %v", err)
	}

	m, err := readmodel.Open(ctx, readmodel.PathFor(dbPath), store)
	if err != nil {
		t.Fatalf("open model: %v", err)
	}

	t.Cleanup(func() { _ = m.Close() })

	host, err := readmodel.NewProjectionHost(store, m, readmodel.ProjectionHostOptions{})
	if err != nil {
		t.Fatalf("build host: %v", err)
	}

	t.Cleanup(func() { _ = host.Close() })

	if _, exists, err := readmodel.ReadWorkerFailedMarker(dbPath); err != nil {
		t.Fatalf("read marker after host build: %v", err)
	} else if exists {
		t.Fatal("worker-death marker survived a fresh host build")
	}
}
