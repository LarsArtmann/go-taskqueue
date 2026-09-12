// Package dlqfix closes the loop around dead-lettered agent work: every
// dead-lettered agent task gets ONE autopsy by a second agent, and the
// autopsy's verdict is executed mechanically — "fixed" rescues the original
// with a fresh attempt budget, "wontfix" dismisses it with the recorded
// reason. The agent only contributes the diagnosis; everything downstream
// (disposition, dedup, loop guarding) is mechanical.
//
// Loop safety is structural: autopsies are only ever minted for dead
// "agent" tasks (a dead autopsy can never mint another autopsy), each dead
// task is autopsied at most once (the dlqfix:<task-id> dedup key, forever),
// and every mint pass runs under the pool's budget guard, which counts
// autopsy tasks like every other enqueue.
package dlqfix

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"log/slog"
	"slices"
	"sync"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

const (
	// defaultPageSize bounds one Facts page per sweep iteration.
	defaultPageSize = 500

	// ConsumerKey is the sweeper's identity in the watermarks table: the
	// persisted cursor shared by every sweeper over the same database, so
	// tasks that dead-lettered while no sweeper was running still get
	// their autopsy when the next pool starts.
	ConsumerKey = "dlqfix-sweeper"

	// DismissedBySweeper names the sweeper in a dismissed task's
	// task.cancelled fact detail (operators dismiss as "operator").
	DismissedBySweeper = "dlqfix-sweeper"
)

// SweeperConfig controls one Sweeper.
type SweeperConfig struct {
	// Model, when set, is written into every autopsy payload. Empty (the
	// default) leaves model choice to each repo's crush config.
	Model string
	// Log receives one summary line per mint/disposition. nil logs nothing.
	Log *slog.Logger
	// PageSize bounds one fact-stream page; 0 selects the default.
	PageSize int
}

// SweepStats summarizes one sweep pass.
type SweepStats struct {
	// Facts is the number of new facts consumed.
	Facts int
	// FixesEnqueued counts fresh autopsy tasks this sweep created.
	FixesEnqueued int
	// FixesKnown counts dead tasks whose autopsy task already existed
	// (dedup hit — the autopsy ran, or is pending, elsewhere).
	FixesKnown int
	// Rescued counts dead tasks re-queued by a "fixed" verdict.
	Rescued int
	// Dismissed counts dead tasks cancelled by a "wontfix" verdict.
	Dismissed int
	// Skipped counts facts that could not yield work (vanished records,
	// foreign payload shapes, dispositions already executed elsewhere).
	Skipped int
}

// Sweeper turns the journal's dead-letter and autopsy-completion facts into
// DLQ dispositions. It is safe for concurrent use (a mutex serializes
// sweeps; ticks and the --once drain watcher may call it from different
// goroutines).
type Sweeper struct {
	store queue.Store
	cfg   SweeperConfig

	mu        sync.Mutex
	watermark int64
	persisted int64 // last checkpoint written to the watermarks table
}

// NewSweeper returns a sweeper over store. The cursor resumes from the
// persisted checkpoint — deaths recorded while no sweeper was running are
// autopsied on the next start. A first run bootstraps at the journal head:
// dead letters that predate the sweeper are not replayed (same semantics as
// the review sweeper; rewind with
// `tq watermarks set dlqfix-sweeper SEQ`).
func NewSweeper(ctx context.Context, store queue.Store, cfg SweeperConfig) (*Sweeper, error) {
	if cfg.PageSize <= 0 {
		cfg.PageSize = defaultPageSize
	}

	persisted, exists, err := store.Watermark(ctx, ConsumerKey)
	if err != nil {
		return nil, fmt.Errorf("dlqfix sweep: read watermark: %w", err)
	}

	// seq 0 with a row is a real cursor ("bootstrapped on an empty journal,
	// consumed nothing yet"): resume from it, do not jump to head.
	if exists {
		return &Sweeper{store: store, cfg: cfg, watermark: persisted, persisted: persisted}, nil
	}

	head, err := store.HeadSeq(ctx)
	if err != nil {
		return nil, fmt.Errorf("dlqfix sweep: read journal head: %w", err)
	}

	// First run: eagerly persist the head (even 0) so a crash before the
	// first sweep still resumes exactly here (and never replays history).
	if err := store.SaveWatermark(ctx, ConsumerKey, head); err != nil {
		return nil, fmt.Errorf("dlqfix sweep: persist bootstrap watermark: %w", err)
	}

	return &Sweeper{store: store, cfg: cfg, watermark: head, persisted: head}, nil
}

// Sweep consumes new facts since the last pass and enqueues autopsy (and
// executes disposition) work. Idempotent by dedup and transition guards:
// calling it twice never duplicates work — a replayed page (crash between
// consumption and checkpoint) re-hits dedup keys and idempotent store
// transitions instead of minting duplicates or double-disposing. The cursor
// checkpoints after each page; a failed checkpoint stops the sweep — facts
// are never consumed past an unpersisted cursor.
func (s *Sweeper) Sweep(ctx context.Context) (SweepStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var stats SweepStats

	// A pending checkpoint gates sweeping (same rule as the bridge's
	// drain): retry it before consuming anything new.
	if s.watermark > s.persisted {
		if err := s.store.SaveWatermark(ctx, ConsumerKey, s.watermark); err != nil {
			return stats, fmt.Errorf("dlqfix sweep: checkpoint %d: %w", s.watermark, err)
		}

		s.persisted = s.watermark
	}

	for {
		facts, err := s.store.Facts(ctx, s.watermark, s.cfg.PageSize)
		if err != nil {
			return stats, fmt.Errorf("dlqfix sweep: read facts after %d: %w", s.watermark, err)
		}

		for _, f := range facts {
			s.watermark = f.Seq
			stats.Facts++

			s.handleFact(ctx, f, &stats)
		}

		// Page end: checkpoint AFTER the last consumed fact — never before,
		// or a crash would silently skip the page's deaths and verdicts.
		if len(facts) > 0 {
			if err := s.store.SaveWatermark(ctx, ConsumerKey, s.watermark); err != nil {
				return stats, fmt.Errorf("dlqfix sweep: checkpoint %d: %w", s.watermark, err)
			}

			s.persisted = s.watermark
		}

		if len(facts) < s.cfg.PageSize {
			return stats, nil
		}
	}
}

// handleFact reacts to one fact: dead-lettered agent tasks gain an autopsy,
// completed autopsy tasks gain their disposition.
func (s *Sweeper) handleFact(ctx context.Context, fact journal.Fact, stats *SweepStats) {
	if fact.Type == journal.DeadLettered {
		s.mintAutopsy(ctx, fact, stats)
	}

	if fact.Type == journal.Completed {
		s.dispose(ctx, fact, stats)
	}
}

// mintAutopsy mints the one autopsy task for a dead AGENT task. The type
// scope IS the loop guard: dlqfix, sh, review and status dead tasks are
// never autopsied, so an autopsy can never spawn another autopsy.
func (s *Sweeper) mintAutopsy(ctx context.Context, fact journal.Fact, stats *SweepStats) {
	t, err := s.store.Get(ctx, task.ID(fact.TaskID))
	if err != nil {
		stats.Skipped++

		return
	}

	if t.Type != executor.TaskTypeAgent {
		stats.Skipped++

		return
	}

	var agentPayload executor.AgentPayload

	if err := json.Unmarshal(t.Payload, &agentPayload); err != nil || agentPayload.Repo == "" ||
		agentPayload.Prompt == "" {
		stats.Skipped++

		return
	}

	payload := executor.DLQFixPayload{
		Repo:      agentPayload.Repo,
		DeadTask:  t.ID.String(),
		DeadType:  t.Type,
		Work:      agentPayload.Prompt,
		Failure:   s.lastFailureEvidence(ctx, t.ID),
		LastError: t.LastError,
		Attempts:  t.Attempts,
		Model:     s.cfg.Model,
		Yolo:      agentPayload.Yolo,
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		stats.Skipped++

		return
	}

	got, err := s.store.Enqueue(ctx, task.New{
		Type:     executor.TaskTypeDLQFix,
		Project:  t.Project,
		Priority: t.Priority,
		Payload:  raw,
		DedupKey: DedupKey(t.ID),
	})
	if err != nil {
		stats.Skipped++

		return
	}

	if got.Attempts == 0 && got.Status == task.Pending {
		stats.FixesEnqueued++

		s.log("dlqfix: enqueued autopsy", "task", got.ID.String(), "dead", t.ID.String(), "repo", agentPayload.Repo)
	} else {
		stats.FixesKnown++
	}
}

// lastFailureEvidence pulls the FailureEvidence off the dead task's LAST
// task.failed fact (the why lives there, not on the dead-lettered fact).
// Best-effort: an absent or unparsable detail leaves the evidence zero and
// the autopsy still runs on the prompt + last error.
func (s *Sweeper) lastFailureEvidence(ctx context.Context, id task.ID) executor.FailureEvidence {
	trail, err := s.store.FactsForTask(ctx, id.String(), 0)
	if err != nil {
		return executor.FailureEvidence{}
	}

	for _, trailFact := range slices.Backward(trail) {
		if trailFact.Type != journal.Failed {
			continue
		}

		var evidence executor.FailureEvidence

		if json.Unmarshal(trailFact.Detail, &evidence) == nil {
			return evidence
		}

		break
	}

	return executor.FailureEvidence{}
}

// dispose executes the mechanical disposition of one completed autopsy:
// "fixed" rescues the dead task with its original attempt budget, "wontfix"
// dismisses it with the autopsy's summary as the recorded reason.
func (s *Sweeper) dispose(ctx context.Context, fact journal.Fact, stats *SweepStats) {
	t, err := s.store.Get(ctx, task.ID(fact.TaskID))
	if err != nil {
		stats.Skipped++

		return
	}

	if t.Type != executor.TaskTypeDLQFix {
		return // someone else's completion — not a skip, just not ours
	}

	deadID, result, ok := s.autopsyOutcome(t, fact, stats)
	if !ok {
		return
	}

	switch result.Verdict {
	case executor.VerdictFixed:
		s.rescue(ctx, deadID, t.ID, result, stats)
	case executor.VerdictWontFix:
		s.dismiss(ctx, deadID, t.ID, result, stats)
	default:
		stats.Skipped++
	}
}

// autopsyOutcome decodes the verdict and the disposition target from a
// completed autopsy task. ok=false (with the skip counted) when either side
// is missing or unparsable.
func (s *Sweeper) autopsyOutcome(
	t task.Task,
	fact journal.Fact,
	stats *SweepStats,
) (task.ID, executor.DLQFixResult, bool) {
	var result executor.DLQFixResult

	if err := json.Unmarshal(fact.Detail, &result); err != nil || result.Verdict == "" {
		stats.Skipped++

		return "", result, false
	}

	var payload executor.DLQFixPayload

	if err := json.Unmarshal(t.Payload, &payload); err != nil || payload.DeadTask == "" {
		stats.Skipped++

		return "", result, false
	}

	return task.ID(payload.DeadTask), result, true
}

// rescue re-queues the dead task with its original attempt budget.
func (s *Sweeper) rescue(ctx context.Context, dead, autopsy task.ID, result executor.DLQFixResult, stats *SweepStats) {
	deadRec, err := s.store.Get(ctx, dead)
	if err != nil {
		stats.Skipped++

		return
	}

	if err := s.store.RescueDead(ctx, dead, deadRec.MaxAttempts); err != nil {
		// Already rescued (or otherwise moved) is fine — dedup and
		// transition guards make the disposition idempotent.
		s.warn("dlqfix: rescue skipped", "dead", dead.String(), "autopsy", autopsy.String(), "err", err)

		stats.Skipped++

		return
	}

	stats.Rescued++

	s.log("dlqfix: rescued", "dead", dead.String(), "autopsy", autopsy.String(), "summary", result.Summary)
}

// dismiss cancels the dead task, recording the autopsy's summary as reason.
func (s *Sweeper) dismiss(ctx context.Context, dead, autopsy task.ID, result executor.DLQFixResult, stats *SweepStats) {
	if err := s.store.DismissDead(ctx, dead, result.Summary, DismissedBySweeper); err != nil {
		s.warn("dlqfix: dismiss skipped", "dead", dead.String(), "autopsy", autopsy.String(), "err", err)

		stats.Skipped++

		return
	}

	stats.Dismissed++

	s.log("dlqfix: dismissed", "dead", dead.String(), "autopsy", autopsy.String(), "reason", result.Summary)
}

func (s *Sweeper) log(msg string, args ...any) {
	if s.cfg.Log != nil {
		s.cfg.Log.Info(msg, args...)
	}
}

func (s *Sweeper) warn(msg string, args ...any) {
	if s.cfg.Log != nil {
		s.cfg.Log.Warn(msg, args...)
	}
}

// DedupKey is the dedup identity of the autopsy of one dead task: exactly
// one autopsy per dead task, forever. A rescued task that dies again does
// NOT get a second autopsy — the second death is deliberately a human
// surface (the first verdict and the fresh failure evidence are both in the
// journal).
func DedupKey(dead task.ID) string {
	return "dlqfix:" + dead.String()
}
