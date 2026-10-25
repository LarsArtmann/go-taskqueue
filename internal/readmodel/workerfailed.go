package readmodel

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"time"

	"github.com/larsartmann/go-cqrs-lite/projectionhost/v4"
)

// Durable worker-death marker (B9.3): a fold worker that exhausts its
// restart budget stops consuming — and the platform reports lag 0 for a
// failed worker, so the cursor-vs-head doctor checks cannot see the
// death (the multi-day silent-outage class). The host's metrics
// WorkerFailed callback stamps this sidecar beside the projection db;
// every new host clears it (a fresh host carries a fresh restart
// budget), so a standing marker means the fold died and nobody
// restarted it. doctor reads it; nothing else writes it.

const workerFailedMarkerSuffix = ".worker-failed"

// WorkerFailedMarkerPath derives the marker sidecar's path for a queue
// database: beside the projection db PathFor stamps.
func WorkerFailedMarkerPath(dbPath string) string {
	return PathFor(dbPath) + workerFailedMarkerSuffix
}

// WorkerFailedMarker is the sidecar's JSON shape: which projection's
// worker died and when (last failure wins — one terminal marker, no
// history).
type WorkerFailedMarker struct {
	Projection string    `json:"projection"`
	FailedAt   time.Time `json:"failedAt"`
}

// WriteWorkerFailedMarker stamps the marker for a queue database
// (atomic temp+rename: the write itself happens on a crash path). This
// is the exported write point for tests and smokes; the live host
// writes through the same single function via its recorder.
func WriteWorkerFailedMarker(dbPath string, marker WorkerFailedMarker) error {
	return writeWorkerFailedMarkerAt(WorkerFailedMarkerPath(dbPath), marker)
}

// ReadWorkerFailedMarker reads the marker for a queue database; exists
// is false when no marker stands (the healthy case).
func ReadWorkerFailedMarker(dbPath string) (marker WorkerFailedMarker, exists bool, err error) {
	raw, err := os.ReadFile(WorkerFailedMarkerPath(dbPath))
	if errors.Is(err, fs.ErrNotExist) {
		return WorkerFailedMarker{}, false, nil
	}

	if err != nil {
		return WorkerFailedMarker{}, false, fmt.Errorf("readmodel: read worker-death marker: %w", err)
	}

	if err := json.Unmarshal(raw, &marker); err != nil {
		return WorkerFailedMarker{}, false, fmt.Errorf("readmodel: decode worker-death marker: %w", err)
	}

	return marker, true, nil
}

// writeWorkerFailedMarkerAt is the one write point both the exported
// helper and the recorder share.
func writeWorkerFailedMarkerAt(path string, marker WorkerFailedMarker) error {
	raw, err := json.Marshal(marker)
	if err != nil {
		return fmt.Errorf("readmodel: encode worker-death marker: %w", err)
	}

	tmp := path + ".tmp"

	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return fmt.Errorf("readmodel: write worker-death marker: %w", err)
	}

	if err := renameWithTransientRetry(tmp, path); err != nil {
		_ = os.Remove(tmp)

		return fmt.Errorf("readmodel: publish worker-death marker: %w", err)
	}

	return nil
}

// workerFailureMarker adapts the sidecar to the platform's metrics
// observer: every lifecycle callback is a no-op except WorkerFailed,
// which logs and stamps the marker. Implementations must not block —
// the writes are two tiny file ops.
type workerFailureMarker struct {
	path string
}

// newWorkerFailureMarker builds the recorder for a projection-home
// path (NOT the queue db path: the host knows the model's own file).
func newWorkerFailureMarker(modelPath string) *workerFailureMarker {
	return &workerFailureMarker{path: modelPath + workerFailedMarkerSuffix}
}

func (w *workerFailureMarker) EventProcessed(string, string, time.Duration) {}
func (w *workerFailureMarker) EventErrored(string, string)                  {}
func (w *workerFailureMarker) EventDeadLettered(string, string)             {}
func (w *workerFailureMarker) WorkerRestarted(string)                       {}
func (w *workerFailureMarker) CheckpointAdvanced(string, time.Duration)     {}

// WorkerFailed is terminal: the worker will not restart without a new
// host. Log loud (serve's console carries it) and stamp the sidecar
// doctor reads; a failed marker write must not panic the host.
func (w *workerFailureMarker) WorkerFailed(projection string) {
	marker := WorkerFailedMarker{Projection: projection, FailedAt: time.Now().UTC()}

	if err := writeWorkerFailedMarkerAt(w.path, marker); err != nil {
		slog.Error("readmodel: worker-death marker write failed", "path", w.path, "err", err)
	}

	slog.Warn("readmodel: projection worker exhausted its restart budget and stopped consuming; restart required",
		"projection", projection,
		"marker", w.path,
	)
}

// compile-time pin: the recorder satisfies the platform observer.
var _ projectionhost.MetricsRecorder = (*workerFailureMarker)(nil)
