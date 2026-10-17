package companion

import (
	"context"
	"time"

	utask "github.com/larsartmann/go-cqrs-lite/queue/v4/task"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// EngineEnqueuer is the slice of the upstream engine the shared Enqueue
// path needs (both backend engines are instantiated at T=[]byte).
type EngineEnqueuer interface {
	Enqueue(ctx context.Context, n utask.New[[]byte]) (utask.Task[[]byte], error)
}

// Enqueue routes a tq task.New through the upstream engine and re-reads
// the created task so the returned record carries every tq field
// (DedupKey included — the upstream task struct does not surface it).
// The engine owns the write; this is the ONE shared routing surface for
// both backend adapters (dedup ruling 2026-09-26: one home, no
// cross-backend mirrors). wake is the adapter's fireWake (may be nil).
func Enqueue(ctx context.Context, engine EngineEnqueuer, cr Runner, wake func(), n task.New) (task.Task, error) {
	if n.Type == "" {
		return task.Task{}, queue.ErrEmptyType
	}

	deps := make([]utask.ID, 0, len(n.Deps))
	for _, d := range n.Deps {
		deps = append(deps, utask.ID(d.String()))
	}

	notBefore := n.NotBefore
	if notBefore.IsZero() {
		notBefore = time.UnixMilli(0)
	}

	created, err := engine.Enqueue(ctx, utask.New[[]byte]{
		Project:     n.Project,
		Type:        n.Type,
		Payload:     []byte(n.Payload),
		Deps:        deps,
		Priority:    n.Priority,
		MaxAttempts: n.MaxAttempts,
		NotBefore:   notBefore,
		DedupKey:    n.DedupKey,
	})
	if err != nil {
		return task.Task{}, MapErr(err)
	}

	if wake != nil {
		wake()
	}

	return Get(ctx, cr, task.ID(created.ID.String()))
}
