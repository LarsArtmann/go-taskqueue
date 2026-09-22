// Package watermark holds the durable journal cursor shared by the
// journal-driven sweepers (review, dlqfix, status, prioritize): one row per
// consumer in the queue store's watermarks table, paged Facts reads, and a
// checkpoint AFTER each consumed page — never before, or a crash would
// silently skip the page's facts. A first use bootstraps at the journal
// head (eagerly persisted), so facts that predate a sweeper are not
// replayed.
package watermark

import (
	"context"
	"fmt"
	"sync"

	"github.com/larsartmann/go-taskqueue/internal/journal"
)

// DefaultPageSize bounds one Facts page per sweep iteration.
const DefaultPageSize = 500

// Source is the pull seam the cursor pages; queue.Store satisfies it.
type Source interface {
	Facts(ctx context.Context, after int64, limit int) ([]journal.Fact, error)
	HeadSeq(ctx context.Context) (int64, error)
	Watermark(ctx context.Context, consumer string) (int64, bool, error)
	SaveWatermark(ctx context.Context, consumer string, seq int64) error
}

// Handler reacts to one consumed fact. An error aborts the sweep before
// the page's checkpoint, so the fact redelivers on the next sweep
// (at-least-once).
type Handler func(ctx context.Context, f journal.Fact) error

// Config builds one Cursor.
type Config struct {
	// Store pages the facts and owns the watermarks row.
	Store Source
	// Key is the cursor's identity in the watermarks table (the sweeper's
	// ConsumerKey), shared by every sweeper over the same database.
	Key string
	// Domain names the owner in error messages ("review sweep"), keeping
	// the wrapped errors grep-identical across the sweepers.
	Domain string
	// PageSize bounds one Facts page; 0 selects DefaultPageSize.
	PageSize int
}

// Cursor is a durable, page-checkpointing journal cursor. It is safe for
// concurrent use: sweeps serialize on an internal mutex, so ticks and the
// --once drain watcher may call Sweep from different goroutines.
type Cursor struct {
	src      Source
	key      string
	domain   string
	pageSize int

	mu        sync.Mutex
	watermark int64
	persisted int64 // last checkpoint written to the watermarks table
}

// New returns a cursor resuming from the persisted checkpoint — facts
// recorded while no sweeper was running are consumed on the next start. A
// first run bootstraps at the journal head and eagerly persists it (even
// 0), so a crash before the first sweep still resumes exactly here and
// never replays history. Rewind deliberately with
// `tq watermarks set <key> SEQ` — replay idempotency is the sweeper's
// concern (dedup keys and transition guards), not the cursor's.
func New(ctx context.Context, cfg Config) (*Cursor, error) {
	if cfg.PageSize <= 0 {
		cfg.PageSize = DefaultPageSize
	}

	persisted, exists, err := cfg.Store.Watermark(ctx, cfg.Key)
	if err != nil {
		return nil, fmt.Errorf("%s: read watermark: %w", cfg.Domain, err)
	}

	// seq 0 with a row is a real cursor ("bootstrapped on an empty journal,
	// consumed nothing yet"): resume from it, do not jump to head.
	if exists {
		return newCursor(cfg, persisted, persisted), nil
	}

	head, err := cfg.Store.HeadSeq(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: read journal head: %w", cfg.Domain, err)
	}

	if err := cfg.Store.SaveWatermark(ctx, cfg.Key, head); err != nil {
		return nil, fmt.Errorf("%s: persist bootstrap watermark: %w", cfg.Domain, err)
	}

	return newCursor(cfg, head, head), nil
}

func newCursor(cfg Config, watermark, persisted int64) *Cursor {
	return &Cursor{
		src:       cfg.Store,
		key:       cfg.Key,
		domain:    cfg.Domain,
		pageSize:  cfg.PageSize,
		watermark: watermark,
		persisted: persisted,
	}
}

// Sweep consumes new facts since the last pass, handing each to handle in
// Seq order. Idempotent by the sweeper's dedup and transition guards: a
// replayed page (crash between consumption and checkpoint) re-hits dedup
// keys instead of duplicating work. The cursor checkpoints after each
// page; a failed checkpoint (or handler) stops the sweep — facts are never
// consumed past an unpersisted cursor.
func (c *Cursor) Sweep(ctx context.Context, handle Handler) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// A pending checkpoint gates sweeping (same rule as the bridge's
	// drain): retry it before consuming anything new.
	if c.watermark > c.persisted {
		if err := c.src.SaveWatermark(ctx, c.key, c.watermark); err != nil {
			return fmt.Errorf("%s: checkpoint %d: %w", c.domain, c.watermark, err)
		}

		c.persisted = c.watermark
	}

	for {
		facts, err := c.src.Facts(ctx, c.watermark, c.pageSize)
		if err != nil {
			return fmt.Errorf("%s: read facts after %d: %w", c.domain, c.watermark, err)
		}

		for _, f := range facts {
			c.watermark = f.Seq

			if err := handle(ctx, f); err != nil {
				return err
			}
		}

		// Page end: checkpoint AFTER the last consumed fact — never
		// before, or a crash would silently skip the page's facts.
		if len(facts) > 0 {
			if err := c.src.SaveWatermark(ctx, c.key, c.watermark); err != nil {
				return fmt.Errorf("%s: checkpoint %d: %w", c.domain, c.watermark, err)
			}

			c.persisted = c.watermark
		}

		if len(facts) < c.pageSize {
			return nil
		}
	}
}
