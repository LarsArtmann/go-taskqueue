package webui

import (
	"context"
	"fmt"
	"log/slog"
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

// runReadModel is the ADR-0019 S3 live path: it opens the projection at
// cfg.ReadModelPath, folds the journal into it through the managed
// projection host (ADR-0019 M08: restart budget, poison-fact DLQ,
// checkpoint batching), and replaces the hand tailer→hub fan-out — every
// folded ledger update wakes the hub (burst-coalesced, exactly the hand
// tailer's batch semantics) carrying the model's applied journal
// watermark, so SSE event ids keep their Last-Event-ID meaning. Run owns
// the model's and the host's lifetime.
func (s *Server) runReadModel(ctx context.Context) error {
	m, err := readmodel.Open(ctx, s.cfg.ReadModelPath, s.store, readmodel.WithDurableCursor())
	if err != nil {
		return fmt.Errorf("open read model: %w", err)
	}

	s.model = m

	defer func() {
		s.model = nil

		_ = m.Close()
	}()

	// The DLQ sidecar catches poison facts beside the projection db; a
	// sidecar failure is warn-and-continue — the fold runs without a
	// DLQ rather than refusing to serve. The close defer registers
	// before the host's, so Stop runs first (LIFO).
	dlq, err := readmodel.OpenDeadLetters(ctx, s.cfg.ReadModelPath)
	if err != nil {
		slog.Warn("readmodel: dlq sidecar unavailable; poison facts restart the fold", "err", err)
	}

	hostOpts := readmodel.ProjectionHostOptions{}
	if dlq != nil {
		hostOpts.DeadLetterStore = dlq.Store()

		defer func() { _ = dlq.Close() }()
	}

	host, err := readmodel.NewProjectionHost(s.store, m, hostOpts)
	if err != nil {
		return fmt.Errorf("build projection host: %w", err)
	}

	if err := host.Start(ctx); err != nil {
		return fmt.Errorf("start projection host: %w", err)
	}

	defer func() { _ = host.Close() }()
	defer func() { _ = host.Stop() }()

	updates := m.WatchSeq(ctx)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case _, ok := <-updates:
			if !ok {
				return nil
			}

		drain:
			for {
				select {
				case _, ok := <-updates:
					if !ok {
						break drain
					}
				default:
					break drain
				}
			}

			s.hub.Notify(m.JournalCursor())
		}
	}
}

// statusCounts reads the per-status counts from the read model when the
// server runs on one, from the store otherwise. Callers zero-fill missing
// statuses themselves.
func (s *Server) statusCounts(ctx context.Context) (map[task.Status]int, error) {
	if s.model != nil {
		counts, err := s.model.StatusCounts(ctx)
		if err != nil {
			return nil, err
		}

		return task.StatusCountsMap(counts), nil
	}

	return s.store.StatusCounts(ctx)
}

// factsForTask returns one task's facts, most recent last, bounded to the
// detail-page render budget.
func (s *Server) factsForTask(ctx context.Context, id string) ([]journal.Fact, error) {
	return s.store.FactsForTask(ctx, id, detailFactsLimit)
}

func formatSeq(seq int64) string {
	return strconv.FormatInt(seq, 10)
}
