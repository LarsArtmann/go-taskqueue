package composition

import (
	"context"
	"fmt"
	"log/slog"

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

// NewProjectionRuntime composes the projection runtime for one
// deployment's projection home (derived beside cfg.DBPath) over the
// queue journal (src): open the ONE tq-owned engine for the projection
// home (the deployment's resolved pragma union — the single-opener
// seam, the composition root is the only tq constructor call site),
// open the model over it with a durable cursor, best-effort the DLQ
// sidecar beside it, and build the managed host wired with the
// sidecar. Starting stays the caller's job (Host.Start) so the
// runactor actor keeps its own lifecycle; on a build error nothing is
// left open.
func NewProjectionRuntime(ctx context.Context, src queue.Store, cfg config.Deployment) (*ProjectionRuntime, error) {
	modelPath := readmodel.PathFor(cfg.DBPath)

	eng, err := sqliteengine.NewSQLiteEngineFromDSN(modelPath, cfg.ProjectionPragmas()...)
	if err != nil {
		return nil, fmt.Errorf("composition: open projection-home engine: %w", err)
	}

	m, err := readmodel.Open(ctx, modelPath, src,
		readmodel.WithDurableCursor(), readmodel.WithEngine(eng))
	if err != nil {
		_ = eng.Close()

		return nil, fmt.Errorf("composition: open read model: %w", err)
	}

	runtime := &ProjectionRuntime{Model: m}

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
// close, then the DLQ sidecar, then the model. Safe to call once; the
// serve wiring defers it exactly where its stacked defers used to live.
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
