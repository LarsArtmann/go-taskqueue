package readmodel

import (
	"testing"
	"time"
)

// TestWorkerFailureMarkerRecorderStampsSidecar pins the metrics-observer
// delegation: the lifecycle no-op callbacks stamp nothing, and
// WorkerFailed writes exactly the shape the exported read returns.
func TestWorkerFailureMarkerRecorderStampsSidecar(t *testing.T) {
	t.Parallel()

	dbPath := t.TempDir() + "/queue.db"
	recorder := newWorkerFailureMarker(PathFor(dbPath))

	recorder.EventProcessed(CursorConsumer, "task.enqueued", time.Second)
	recorder.EventErrored(CursorConsumer, "task.enqueued")
	recorder.EventDeadLettered(CursorConsumer, "task.enqueued")
	recorder.WorkerRestarted(CursorConsumer)
	recorder.CheckpointAdvanced(CursorConsumer, time.Second)

	if _, exists, err := ReadWorkerFailedMarker(dbPath); err != nil {
		t.Fatalf("read after no-op callbacks: %v", err)
	} else if exists {
		t.Fatal("a no-op callback stamped the marker")
	}

	recorder.WorkerFailed(CursorConsumer)

	got, exists, err := ReadWorkerFailedMarker(dbPath)
	if err != nil {
		t.Fatalf("read after WorkerFailed: %v", err)
	}

	if !exists {
		t.Fatal("marker missing after WorkerFailed")
	}

	if got.Projection != CursorConsumer {
		t.Fatalf("projection = %q, want %q", got.Projection, CursorConsumer)
	}

	if age := time.Since(got.FailedAt); age < 0 || age > time.Minute {
		t.Fatalf("failedAt age = %v, want within a minute", age)
	}
}
