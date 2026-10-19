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
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/harvest"
	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
	"github.com/larsartmann/go-taskqueue/internal/watermark"
)

const (
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
	// Scanner attributes footer commits for the gate-artifact shipped-proof
	// check (TODO row 145). nil disables the auto-dismiss path entirely —
	// every dead agent task gets an autopsy as before. The pool wires
	// executor.GitLogScanner{}.
	Scanner executor.GitScanner
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
	// Skipped counts facts that could not yield work and are checkpointed
	// past (foreign payload shapes, dispositions already executed
	// elsewhere). A refused autopsy MINT or DISPOSITION is not counted
	// here: it aborts the sweep before the checkpoint so the death (or the
	// autopsy's Completed fact) redelivers.
	Skipped int
	// AutoDismissed counts dead tasks cancelled by the mechanical
	// gate-artifact dismissal (TODO row 145) — no autopsy was minted for
	// them.
	AutoDismissed int
	// EnvSkipped counts dead tasks whose death is machine-proven
	// environmental (env-streak breaker, verify-gate env/dead/slow classes):
	// no autopsy is minted because no in-repo fix exists — the task stays
	// DEAD for the operator (tq dlq) instead of burning a paid autopsy
	// turn that can only rule wontfix (2026-10-11 ceremony cut).
	EnvSkipped int
}

// Sweeper turns the journal's dead-letter and autopsy-completion facts into
// DLQ dispositions. It is safe for concurrent use (the cursor serializes
// sweeps; ticks and the --once drain watcher may call it from different
// goroutines).
//
// The struct shell deliberately stays per-sweeper (config fields and fact
// validation differ across the four sweepers); the shared pump is
// watermark.Cursor.Sweep.
//
// art-dupl:accept mirrored sweeper shell: config fields and fact validation
// differ per sweeper; the shared pump already lives in watermark.Cursor.
type Sweeper struct {
	store queue.Store
	cfg   SweeperConfig

	cur *watermark.Cursor
}

// NewSweeper returns a sweeper over store. The cursor resumes from the
// persisted checkpoint — deaths recorded while no sweeper was running are
// autopsied on the next start. A first run bootstraps at the journal head:
// dead letters that predate the sweeper are not replayed (same semantics as
// the review sweeper; rewind with
// `tq watermarks set dlqfix-sweeper SEQ`).
func NewSweeper(ctx context.Context, store queue.Store, cfg SweeperConfig) (*Sweeper, error) {
	cur, err := watermark.New(ctx, watermark.Config{
		Store:  store,
		Key:    ConsumerKey,
		Domain: "dlqfix sweep",
	})
	if err != nil {
		return nil, err
	}

	return &Sweeper{store: store, cfg: cfg, cur: cur}, nil
}

// Sweep consumes new facts since the last pass and enqueues autopsy (and
// executes disposition) work. Idempotent by dedup and transition guards:
// calling it twice never duplicates work — a replayed page (crash between
// consumption and checkpoint) re-hits dedup keys and idempotent store
// transitions instead of minting duplicates or double-disposing.
//
// A mint refusal — a store failure while minting an autopsy for a dead
// agent task — and a disposition refusal — a store failure while executing a
// completed autopsy's verdict — abort the sweep BEFORE the checkpoint: the
// fact redelivers on the next pass instead of being silently skipped forever
// (13-15 §e). Deterministic skips (foreign types, unusable payloads, missing
// records, already-executed transitions) stay checkpointed past: retrying
// them can never succeed and would wedge the cursor on one bad record.
func (s *Sweeper) Sweep(ctx context.Context) (SweepStats, error) {
	var stats SweepStats

	err := s.cur.Sweep(ctx, func(ctx context.Context, f journal.Fact) error {
		stats.Facts++

		return s.handleFact(ctx, f, &stats)
	})

	return stats, err
}

// handleFact reacts to one fact: dead-lettered agent tasks gain an autopsy,
// completed autopsy tasks gain their disposition. A mint or disposition
// refusal propagates (checkpoint held); deterministic skips never abort the
// sweep.
func (s *Sweeper) handleFact(ctx context.Context, fact journal.Fact, stats *SweepStats) error {
	if fact.Type == journal.DeadLettered {
		return s.mintAutopsy(ctx, fact, stats)
	}

	if fact.Type == journal.Completed {
		return s.dispose(ctx, fact, stats)
	}

	return nil
}

// mintAutopsy mints the one autopsy task for a dead AGENT task. The type
// scope IS the loop guard: dlqfix, sh, review and status dead tasks are
// never autopsied, so an autopsy can never spawn another autopsy.
//
// A store failure under the mint (record read, autopsy enqueue) is a
// REFUSAL: it returns an error so the sweep holds its checkpoint and the
// death redelivers, warned once per pass. The done-guard refusal (a dedup
// key whose autopsy already completed) is NOT a failure — it counts
// known, like every dedup hit.
func (s *Sweeper) mintAutopsy(ctx context.Context, fact journal.Fact, stats *SweepStats) error {
	t, err := s.store.Get(ctx, task.ID(fact.TaskID))
	if err != nil {
		return s.refuseMint(fact.TaskID, "read dead record", err)
	}

	if t.Type != executor.TaskTypeAgent {
		stats.Skipped++

		return nil
	}

	var agentPayload executor.AgentPayload

	if err := json.Unmarshal(t.Payload, &agentPayload); err != nil || agentPayload.Repo == "" ||
		agentPayload.Prompt == "" {
		stats.Skipped++

		return nil
	}

	if s.autoDismissGateArtifact(ctx, t, agentPayload.Repo, stats) {
		return nil
	}

	// Environmental deaths never get an autopsy: the env-streak breaker
	// and the verify-gate environmental classes are machine-PROVEN
	// unfixable from inside the repo, so the paid turn could only ever
	// return wontfix (2026-10-10 production: ~7 futile cycles/day). The
	// task stays dead and operator-visible in `tq dlq`.
	if evidence := s.lastFailureEvidence(ctx, t.ID); executor.IsEnvironmentalDeath(t.LastError) ||
		executor.IsEnvironmentalDeath(evidence.Tail) {
		stats.EnvSkipped++

		s.log("dlqfix: autopsy refused — environmental death",
			"dead", t.ID.String(), "repo", agentPayload.Repo, "last_error", firstLine(t.LastError))

		return nil
	}

	payload := executor.DLQFixPayload{
		Repo:                      agentPayload.Repo,
		DeadTask:                  t.ID.String(),
		DeadType:                  t.Type,
		Work:                      agentPayload.Prompt,
		Failure:                   s.lastFailureEvidence(ctx, t.ID),
		LastError:                 t.LastError,
		Attempts:                  t.Attempts,
		ReclaimsWithUnknownEffect: s.reclaimsWithUnknownEffect(ctx, t.ID),
		Model:                     s.cfg.Model,
		Yolo:                      agentPayload.Yolo,
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		stats.Skipped++

		return nil
	}

	got, err := s.store.Enqueue(ctx, task.New{
		Type:     executor.TaskTypeDLQFix,
		Project:  t.Project,
		Priority: t.Priority,
		Payload:  raw,
		DedupKey: DedupKey(t.ID),
	})
	if errors.Is(err, queue.ErrTaskDone) {
		// The done-guard (Queue wrapper): the dedup key matched a
		// COMPLETED autopsy — the one autopsy for this death already ran.
		// Known, not refused: the checkpoint moves past.
		stats.FixesKnown++

		return nil
	}

	if err != nil {
		return s.refuseMint(t.ID.String(), "enqueue autopsy", err)
	}

	if got.Attempts == 0 && got.Status == task.Pending {
		stats.FixesEnqueued++

		s.log("dlqfix: enqueued autopsy", "task", got.ID.String(), "dead", t.ID.String(), "repo", agentPayload.Repo)
	} else {
		stats.FixesKnown++
	}

	return nil
}

// refuseMint reports a mint refusal and returns the error that holds the
// sweep's checkpoint: a refused mint must not advance past the death, or
// the autopsy never happens without an operator watermark rewind.
func (s *Sweeper) refuseMint(dead, why string, err error) error {
	s.warn("dlqfix: autopsy mint refused — checkpoint held",
		"dead", dead, "why", why, "err", err)

	return fmt.Errorf("dlqfix: autopsy mint refused for dead %s (%s): %w", dead, why, err)
}

// refuseDisposition reports a disposition refusal and returns the error that
// holds the sweep's checkpoint: a refused disposition must not advance past
// the autopsy's Completed fact, or the verdict's rescue/dismissal never
// happens without an operator watermark rewind (sibling of refuseMint).
func (s *Sweeper) refuseDisposition(autopsy, why string, err error) error {
	s.warn("dlqfix: disposition refused — checkpoint held",
		"autopsy", autopsy, "why", why, "err", err)

	return fmt.Errorf("dlqfix: disposition refused for autopsy %s (%s): %w", autopsy, why, err)
}

// autoDismissGateArtifact executes the one mechanical dismissal of TODO row
// 145: a dead agent task whose death is the unscoped-gofmt verify artifact
// (executor.IsGateArtifactDeath) AND whose work demonstrably shipped — at
// least one footer commit carrying the task's Task-Queue-ID plus a TODO_LIST
// item that is ticked or gone — never needs an autopsy diagnosing a gate
// that lied. The verdict channel's wontfix-without-summary rule does not
// apply (no agent verdict is parsed); the sweeper records its own summary.
// Conservative by construction: every unmet condition — no scanner, artifact
// mismatch, scan error, zero footer commits, non-harvest task, item still
// open, unreadable TODO_LIST — falls through and the normal autopsy is
// minted. Idempotent on cursor rewind: a non-Dead record fails the
// dismissal and keeps the dedup-keyed mint path intact.
func (s *Sweeper) autoDismissGateArtifact(ctx context.Context, t task.Task, repo string, stats *SweepStats) bool {
	if s.cfg.Scanner == nil || t.Status != task.Dead {
		return false
	}

	if !executor.IsGateArtifactDeath(s.lastFailureEvidence(ctx, t.ID), t.LastError) {
		return false
	}

	commits, err := s.cfg.Scanner.CommitsByTrailer(ctx, repo, executor.TaskTrailer, t.ID.String())
	if err != nil {
		s.warn("dlqfix: footer scan failed, autopsy falls through", "dead", t.ID.String(), "repo", repo, "err", err)

		return false
	}

	if len(commits) == 0 {
		return false
	}

	shipment, ok := todoShipment(t, repo)
	if !ok {
		return false
	}

	summary := fmt.Sprintf(
		"gate-artifact auto-dismiss [%s]: verify died on the unscoped gofmt artifact, work shipped (%d footer commit(s), TODO item %s)",
		executor.VerifyGateEnvCode,
		len(commits),
		shipment,
	)

	if err := s.store.DismissDead(ctx, t.ID, summary, DismissedBySweeper); err != nil {
		s.warn("dlqfix: auto-dismiss skipped", "dead", t.ID.String(), "err", err)

		return false
	}

	stats.AutoDismissed++

	s.log("dlqfix: auto-dismissed gate-artifact death", "dead", t.ID.String(), "repo", repo, "shipment", shipment)

	return true
}

// todoShipment reports the TODO_LIST shipment state of a harvested dead
// task: "ticked" when its item is [x], "absent" when the item text is gone
// (the done-and-deleted convention — the successor consumed the row).
// ok=false when the task is not a single-item harvest (no todo: dedup key;
// batches stay a human surface), its repo has no readable TODO_LIST, or the
// item is STILL OPEN in the file — none of which proves shipment.
func todoShipment(t task.Task, repo string) (string, bool) {
	if !strings.HasPrefix(t.DedupKey, "todo:") {
		return "", false
	}

	items, err := harvest.ParseRepo(repo, harvest.DefaultTodoFile)
	if err != nil {
		return "", false
	}

	for _, item := range items {
		if item.Key != t.DedupKey {
			continue
		}

		if item.Done {
			return "ticked", true
		}

		return "", false
	}

	return "absent", true
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

// reclaimsWithUnknownEffect counts the dead task's task.released facts whose
// effect disposition is unknown — a crash mid-effect. The autopsy is warned
// when > 0 so it treats the repo's git state as possibly partial rather than
// a clean slate (turnstone: "crashes aren't finishes").
func (s *Sweeper) reclaimsWithUnknownEffect(ctx context.Context, id task.ID) int {
	trail, err := s.store.FactsForTask(ctx, id.String(), 0)
	if err != nil {
		return 0
	}

	count := 0

	for _, f := range trail {
		if f.Type != journal.Released || len(f.Detail) == 0 {
			continue
		}

		var detail journal.ReleasedDetail
		if json.Unmarshal(f.Detail, &detail) == nil && detail.Effect == journal.EffectUnknown {
			count++
		}
	}

	return count
}

// dispose executes the mechanical disposition of one completed autopsy:
// "fixed" rescues the dead task with its original attempt budget, "wontfix"
// dismisses it with the autopsy's summary as the recorded reason.
//
// A store failure under the disposition (autopsy read, dead-record read,
// rescue, dismissal) is a REFUSAL: it returns an error so the sweep holds
// its checkpoint and the Completed fact redelivers, warned once per pass.
// Permanent store outcomes (missing record, already-executed transition —
// the idempotent replay class) and unparsable payloads stay checkpointed
// past: they cannot succeed on retry.
func (s *Sweeper) dispose(ctx context.Context, fact journal.Fact, stats *SweepStats) error {
	t, err := s.store.Get(ctx, task.ID(fact.TaskID))
	if err != nil {
		if watermark.PermanentOutcome(err) {
			stats.Skipped++

			return nil
		}

		return s.refuseDisposition(fact.TaskID, "read autopsy record", err)
	}

	if t.Type != executor.TaskTypeDLQFix {
		return nil // someone else's completion — not a skip, just not ours
	}

	deadID, result, ok := s.autopsyOutcome(t, fact, stats)
	if !ok {
		return nil
	}

	switch result.Verdict {
	case executor.VerdictFixed:
		return s.rescue(ctx, deadID, t.ID, result, stats)
	case executor.VerdictWontFix:
		return s.dismiss(ctx, deadID, t.ID, result, stats)
	default:
		stats.Skipped++
	}

	return nil
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

// rescue re-queues the dead task with its original attempt budget. A
// transient store failure refuses (checkpoint held); a permanently decided
// outcome (record gone, already moved elsewhere) skips past.
func (s *Sweeper) rescue(
	ctx context.Context,
	dead, autopsy task.ID,
	result executor.DLQFixResult,
	stats *SweepStats,
) error {
	deadRec, err := s.store.Get(ctx, dead)
	if err != nil {
		if watermark.PermanentOutcome(err) {
			stats.Skipped++

			return nil
		}

		return s.refuseDisposition(autopsy.String(), "read dead record", err)
	}

	if err := s.store.RescueDead(ctx, dead, deadRec.MaxAttempts); err != nil {
		if watermark.PermanentOutcome(err) {
			// Already rescued (or otherwise moved) is fine — dedup and
			// transition guards make the disposition idempotent.
			s.warn("dlqfix: rescue skipped", "dead", dead.String(), "autopsy", autopsy.String(), "err", err)

			stats.Skipped++

			return nil
		}

		return s.refuseDisposition(autopsy.String(), "rescue dead task", err)
	}

	stats.Rescued++

	s.log("dlqfix: rescued", "dead", dead.String(), "autopsy", autopsy.String(), "summary", result.Summary)

	return nil
}

// dismiss cancels the dead task, recording the autopsy's summary as reason.
// A transient store failure refuses (checkpoint held); a permanently decided
// outcome (already dismissed or moved elsewhere) skips past.
func (s *Sweeper) dismiss(
	ctx context.Context,
	dead, autopsy task.ID,
	result executor.DLQFixResult,
	stats *SweepStats,
) error {
	if err := s.store.DismissDead(ctx, dead, result.Summary, DismissedBySweeper); err != nil {
		if watermark.PermanentOutcome(err) {
			s.warn("dlqfix: dismiss skipped", "dead", dead.String(), "autopsy", autopsy.String(), "err", err)

			stats.Skipped++

			return nil
		}

		return s.refuseDisposition(autopsy.String(), "dismiss dead task", err)
	}

	stats.Dismissed++

	s.log("dlqfix: dismissed", "dead", dead.String(), "autopsy", autopsy.String(), "reason", result.Summary)

	return nil
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

// firstLine keeps the env-refusal log line single-line: LastError can be
// a multi-line evidence dump.
func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}

	return s
}

// DedupKey is the dedup identity of the autopsy of one dead task: exactly
// one autopsy per dead task, forever. A rescued task that dies again does
// NOT get a second autopsy — the second death is deliberately a human
// surface (the first verdict and the fresh failure evidence are both in the
// journal).
// DedupPrefix is the dedup-identity namespace of autopsy tasks; audit
// surfaces key off it to find them without stringly coupling.
const DedupPrefix = "dlqfix:"

func DedupKey(dead task.ID) string {
	return DedupPrefix + dead.String()
}
