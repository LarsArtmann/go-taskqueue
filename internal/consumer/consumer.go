// Package consumer is the journal-as-bus push seam (ADR-0009): one paging
// reader over queue.Store bounded reads fans every fact out to exact
// subscribers, in Seq order, at-least-once.
//
// Delivery contract (ADR-0009 D1/D2): exact consumers are NEVER skipped —
// a handler error pauses that subscriber's drain until its next tick
// (backpressure), while other subscribers continue. Subscribers own their
// persistence (the watermarks table); this dispatcher's cursors are
// per-process and resume from whatever `since` the subscriber passes.
// Signal consumers (the webui hub) stay on the hub — the dispatcher serves
// the exact class only.
package consumer

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/journal"
)

// DefaultPollInterval is how often the dispatcher pages the journal when
// no notify hook exists (the v1 wake strategy; a notify-after-commit hook
// is the documented v2 path and must fire outside the mutation
// transaction — ADR-0009 D4).
const DefaultPollInterval = time.Second

// DefaultPageSize bounds one Facts page per subscriber drain.
const DefaultPageSize = 500

// Source is the pull seam the dispatcher pages; queue.Store satisfies it.
type Source interface {
	Facts(ctx context.Context, after int64, limit int) ([]journal.Fact, error)
	HeadSeq(ctx context.Context) (int64, error)
}

// Handler receives facts in Seq order, at-least-once. An error pauses this
// subscriber's drain (the fact is re-delivered on the next tick); it never
// blocks the other subscribers and never makes the dispatcher skip.
type Handler func(ctx context.Context, f journal.Fact) error

// Dispatcher fans journal facts out to registered subscribers.
type Dispatcher struct {
	src      Source
	poll     time.Duration
	pageSize int
	log      *slog.Logger

	mu   sync.Mutex
	subs []*subscriber
}

type subscriber struct {
	name    string
	handler Handler

	mu     sync.Mutex
	cursor int64 // last delivered seq (advances only after the handler accepts)
}

func (s *subscriber) advance(seq int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cursor = seq
}

func (s *subscriber) current() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.cursor
}

// Config controls a Dispatcher. Zero values select the defaults.
type Config struct {
	// PollInterval is the paging cadence. Default 1s.
	PollInterval time.Duration
	// PageSize bounds one Facts page per drain. Default 500.
	PageSize int
	// Logger receives drain diagnostics. Default slog.Default().
	Logger *slog.Logger
}

// New builds a Dispatcher over src. Call Subscribe before Run.
func New(src Source, cfg Config) *Dispatcher {
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = DefaultPollInterval
	}

	if cfg.PageSize <= 0 {
		cfg.PageSize = DefaultPageSize
	}

	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	return &Dispatcher{src: src, poll: cfg.PollInterval, pageSize: cfg.PageSize, log: cfg.Logger}
}

// Subscribe registers an exact consumer delivering facts with Seq >
// since. It returns the subscriber's unsubscribe function. Subscribing
// while Run is active is safe.
func (d *Dispatcher) Subscribe(name string, since int64, handler Handler) (unsubscribe func()) {
	d.mu.Lock()
	defer d.mu.Unlock()

	sub := &subscriber{name: name, handler: handler, cursor: since}
	d.subs = append(d.subs, sub)

	return func() {
		d.mu.Lock()
		defer d.mu.Unlock()

		for i, s := range d.subs {
			if s != sub {
				continue
			}

			d.subs = append(d.subs[:i], d.subs[i+1:]...)

			break
		}
	}
}

// Run pages the journal for every subscriber until ctx is cancelled. Each
// tick drains each subscriber independently in bounded pages: a handler
// error stops that subscriber's drain (the failing fact redelivers next
// tick) and the others continue — per-class backpressure, ADR-0009 D2.
// Lag is logged once a minute per subscriber that is behind the head.
func (d *Dispatcher) Run(ctx context.Context) error {
	ticker := time.NewTicker(d.poll)
	defer ticker.Stop()

	var lastLagLog time.Time

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			d.tick(ctx)

			if time.Since(lastLagLog) >= time.Minute {
				lastLagLog = time.Now()

				if lag, err := d.Lag(ctx); err == nil {
					for name, l := range lag {
						if l > 0 {
							d.log.Info("consumer dispatcher lag", "subscriber", name, "lag", l)
						}
					}
				}
			}
		}
	}
}

func (d *Dispatcher) tick(ctx context.Context) {
	for _, sub := range d.snapshotSubs() {
		d.drain(ctx, sub)
	}
}

// drain delivers every currently available fact to one subscriber. The
// cursor advances past a fact only after its handler accepts it.
func (d *Dispatcher) drain(ctx context.Context, sub *subscriber) {
	for {
		facts, err := d.src.Facts(ctx, sub.current(), d.pageSize)
		if err != nil {
			if ctx.Err() != nil {
				return
			}

			d.log.Error("consumer dispatcher read failed", "subscriber", sub.name, "err", err)

			return
		}

		for _, fact := range facts {
			if err := sub.handler(ctx, fact); err != nil {
				if ctx.Err() != nil {
					return
				}

				d.log.Error("consumer dispatcher handler failed; will retry",
					"subscriber", sub.name, "seq", fact.Seq, "type", fact.Type, "err", err)

				return
			}

			sub.advance(fact.Seq)
		}

		if len(facts) < d.pageSize {
			return
		}
	}
}

// Lag reports per-subscriber lag (HeadSeq − cursor, floored at 0): how far
// behind the journal head each subscriber sits — the ops surface of
// ADR-0009 D4.
func (d *Dispatcher) Lag(ctx context.Context) (map[string]int64, error) {
	head, err := d.src.HeadSeq(ctx)
	if err != nil {
		return nil, fmt.Errorf("consumer dispatcher: read head: %w", err)
	}

	out := make(map[string]int64)

	for _, sub := range d.snapshotSubs() {
		lag := max(head-sub.current(), 0)

		out[sub.name] = lag
	}

	return out, nil
}

// Cursor reports one subscriber's current delivery cursor (0 = registered
// but nothing delivered yet). ok=false when the name is unknown.
func (d *Dispatcher) Cursor(name string) (int64, bool) {
	for _, sub := range d.snapshotSubs() {
		if sub.name == name {
			return sub.current(), true
		}
	}

	return 0, false
}

func (d *Dispatcher) snapshotSubs() []*subscriber {
	d.mu.Lock()
	defer d.mu.Unlock()

	return append([]*subscriber(nil), d.subs...)
}
