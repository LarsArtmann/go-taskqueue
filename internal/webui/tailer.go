package webui

import (
	"context"
	"strconv"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/readmodel"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// tail is the single journal tailer: it polls Facts(after) every cfg.Poll
// and notifies the hub once per batch of new facts (burst coalescing).
// The watermark starts at the journal head so a freshly started server
// does not replay history — new clients get a full snapshot on connect
// anyway. Each poll reads at most tailBatchLimit facts: the tailer only
// signals that something changed, so a burst longer than the cap skips
// middle facts without losing the notification.
func (s *Server) tail(ctx context.Context) error {
	watermark, err := s.journalHead(ctx)
	if err != nil {
		return err
	}

	ticker := time.NewTicker(s.cfg.Poll)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}

		facts, err := s.store.Facts(ctx, watermark, tailBatchLimit)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}

			continue
		}

		if len(facts) == 0 {
			continue
		}

		watermark = facts[len(facts)-1].Seq
		s.hub.Notify(watermark)
	}
}

// journalHead returns the current highest fact sequence (0 when empty).
func (s *Server) journalHead(ctx context.Context) (int64, error) {
	return s.store.HeadSeq(ctx)
}

// runReadModel is the ADR-0019 S3 live path: the injected pump (composed
// by the caller — webui never opens the projection) starts the managed
// projection host (ADR-0019 M08: restart budget, poison-fact DLQ,
// checkpoint batching) and replaces the hand tailer→hub fan-out — every
// folded ledger update wakes the hub (burst-coalesced, exactly the hand
// tailer's batch semantics) carrying the model's applied journal
// watermark, so SSE event ids keep their Last-Event-ID meaning. The pump
// owns the model's and the host's lifetime.
func (s *Server) runReadModel(ctx context.Context) error {
	s.model = s.cfg.Pump.Model()

	defer func() {
		s.model = nil
	}()

	return s.cfg.Pump.Run(ctx, s.hub.Notify)
}

// statusCounts reads the per-status counts through the shared
// readmodel.StatusCounts seam (model when the server runs on one, store
// otherwise). Callers zero-fill missing statuses themselves.
func (s *Server) statusCounts(ctx context.Context) (map[task.Status]int, error) {
	return readmodel.StatusCounts(ctx, s.model, s.store)
}

// factsForTask returns one task's facts, most recent last, bounded to the
// detail-page render budget.
func (s *Server) factsForTask(ctx context.Context, id string) ([]journal.Fact, error) {
	return s.store.FactsForTask(ctx, id, detailFactsLimit)
}

func formatSeq(seq int64) string {
	return strconv.FormatInt(seq, 10)
}
