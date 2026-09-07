package webui

import (
	"context"
	"strconv"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/journal"
)

// tail is the single journal tailer: it polls Facts(after) every cfg.Poll
// and notifies the hub once per batch of new facts (burst coalescing).
// The watermark starts at the journal head so a freshly started server
// does not replay history — new clients get a full snapshot on connect
// anyway.
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

		facts, err := s.store.Facts(ctx, watermark)
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
	facts, err := s.store.Facts(ctx, 0)
	if err != nil {
		return 0, err
	}

	if len(facts) == 0 {
		return 0, nil
	}

	return facts[len(facts)-1].Seq, nil
}

// factsForTask returns all facts belonging to one task, in Seq order.
func (s *Server) factsForTask(ctx context.Context, id string) ([]journal.Fact, error) {
	facts, err := s.store.Facts(ctx, 0)
	if err != nil {
		return nil, err
	}

	var matched []journal.Fact

	for _, f := range facts {
		if f.TaskID == id {
			matched = append(matched, f)
		}
	}

	return matched, nil
}

func formatSeq(seq int64) string {
	return strconv.FormatInt(seq, 10)
}
