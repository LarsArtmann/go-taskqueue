package composition

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	sqliteengine "github.com/larsartmann/go-cqrs-lite/metaengine/sqliteengine/v4"
	"github.com/larsartmann/go-taskqueue/internal/config"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/readmodel"
)

// ProjectionRuntime bundles the ADR-0019 S3 fold surfaces one serve run
// owns: the model, the poison-fact sidecar (nil when it could not be
// opened — warn-and-continue), and the managed projection host. Close
// tears them down in LIFO order (host stop+close, sidecar, model), the
// ordering the serve wiring previously hand-rolled with stacked defers.
type ProjectionRuntime struct {
	Model *readmodel.Model
	Host  *readmodel.ProjectionHost

	dlq *readmodel.DeadLetters
}

// projectionFoldLockWait bounds the runtime's fold-lock wait at startup:
// long enough to ride out a transient holder (a stats/doctor catchup is
// sub-second, a delete-file replay a few more), short enough that a
// genuinely owned home demotes serve to a reader without hanging it.
const projectionFoldLockWait = 5 * time.Second

// NewProjectionRuntime composes the projection runtime for one
// deployment's projection home (derived beside cfg.DBPath) over the
// queue journal (src): elect the single folder through the projection
// home's fold lock (a bounded wait rides out transient holders; a lost
// election mounts the runtime READ-ONLY — model without host), then for
// the folder: open the ONE tq-owned engine for the projection home (the
// deployment's resolved pragma union — the single-opener seam, the
// composition root is the only tq constructor call site), open the model
// over it with a durable cursor, best-effort the DLQ sidecar beside it,
// and build the managed host wired with the sidecar. Starting stays the
// caller's job (Host.Start) so the runactor actor keeps its own
// lifecycle; on a build error nothing is left open.
func NewProjectionRuntime(ctx context.Context, src queue.Store, cfg config.Deployment) (*ProjectionRuntime, error) {
	modelPath := readmodel.PathFor(cfg.DBPath)

	lock, err := readmodel.AcquireFoldLock(modelPath, projectionFoldLockWait)
	if err != nil && !errors.Is(err, readmodel.ErrFoldLocked) {
		return nil, fmt.Errorf("composition: acquire fold lock: %w", err)
	}

	folder := err == nil

	if folder {
		if err := readmodel.ConvergeProjectionHome(ctx, modelPath); err != nil {
			_ = lock.Release()

			return nil, fmt.Errorf("composition: converge projection home: %w", err)
		}
	} else {
		slog.Warn(
			"composition: projection fold lock held by another folder — mounting the runtime read-only (no fold, no host)",
			"home",
			modelPath,
			"lock",
			readmodel.FoldLockPath(modelPath),
		)
	}

	eng, err := sqliteengine.NewSQLiteEngineFromDSN(modelPath, cfg.ProjectionPragmas()...)
	if err != nil {
		_ = lock.Release()

		return nil, fmt.Errorf("composition: open projection-home engine: %w", err)
	}

	opts := []readmodel.Option{readmodel.WithDurableCursor(), readmodel.WithEngine(eng)}
	if folder {
		opts = append(opts, readmodel.WithFoldLock(lock))
	} else {
		opts = append(opts, readmodel.WithReadOnly())
	}

	m, err := readmodel.Open(ctx, modelPath, src, opts...)
	if err != nil {
		_ = eng.Close()
		_ = lock.Release()

		return nil, fmt.Errorf("composition: open read model: %w", err)
	}

	runtime := &ProjectionRuntime{Model: m}

	if !folder {
		// Read-only mount: the lock holder folds into the same home and
		// this runtime serves its reads — no host (a read-only model
		// cannot fold, so a pump here would only re-checkpoint the
		// shared watermark) and no DLQ sidecar (poison routing belongs
		// to the folder).
		return runtime, nil
	}

	cleanup := func() { _ = m.Close() }
	defer func() {
		if cleanup != nil {
			cleanup()
		}
	}()

	dlq, err := readmodel.OpenDeadLetters(ctx, cfg)
	if err != nil {
		slog.Warn("readmodel: dlq sidecar unavailable; poison facts restart the fold", "err", err)
	} else {
		runtime.dlq = dlq
	}

	hostOpts := readmodel.ProjectionHostOptions{}
	if runtime.dlq != nil {
		hostOpts.DeadLetterStore = runtime.dlq.Store()
	}

	host, err := readmodel.NewProjectionHost(src, m, hostOpts)
	if err != nil {
		if runtime.dlq != nil {
			_ = runtime.dlq.Close()
		}

		return nil, fmt.Errorf("composition: build projection host: %w", err)
	}

	runtime.Host = host
	cleanup = nil

	return runtime, nil
}

// Close releases the runtime in reverse construction order: host stop and
// close, then the DLQ sidecar, then the model. Idempotent: each field is
// nil'd after release, so a second Close is a no-op (the serve wiring
// relies on this — the pump's deferred Close runs after cmdServe's).
func (r *ProjectionRuntime) Close() error {
	var errs []error

	if r.Host != nil {
		_ = r.Host.Stop()
		_ = r.Host.Close()

		r.Host = nil
	}

	if r.dlq != nil {
		if err := r.dlq.Close(); err != nil {
			errs = append(errs, err)
		}

		r.dlq = nil
	}

	if r.Model != nil {
		if err := r.Model.Close(); err != nil {
			errs = append(errs, err)
		}

		r.Model = nil
	}

	if len(errs) > 0 {
		return fmt.Errorf("composition: close projection runtime: %w", errs[0])
	}

	return nil
}
