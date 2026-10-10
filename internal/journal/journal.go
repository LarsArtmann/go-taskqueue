// Package journal defines the append-only fact log that records everything
// that ever happened to tasks. All derived views (queue, DLQ, stats) are
// projections over these facts.
//
// S2 (ADR-0019): the fact vocabulary RIDES the upstream engine's
// go-cqrs-lite queue/v4/facts vocabulary — FactType and Fact are TYPE
// ALIASES of facts.FactType/facts.Fact (the lifecycle spellings are
// byte-identical, pinned in internal/queue/companion's vocabulary test),
// and lifecycle facts come from the engine's own same-transaction
// transitions. Tq-only families (heartbeat, session.*, question-*,
// error/incident) stay tq-side constants of the aliased type, written
// through the engine's FactTx sink; tq invariants on lifecycle facts
// (which transitions may emit them, e.g. ADR-0015 §5 on Reprioritized)
// live at their construction sites. Fact reads come from the ENGINE's
// queue.Store contract, mapped at the companion seam — never mirrored SQL.
package journal

import (
	"context"
	"sync"
	"time"

	"github.com/larsartmann/go-cqrs-lite/queue/v4/facts"
)

// FactType enumerates the kinds of facts that can be recorded; it IS the
// engine's facts.FactType (ADR-0019 S2), so tq constants and upstream
// constants are interchangeable by construction.
type FactType = facts.FactType

// Fact is one immutable observation about one task; it IS the engine's
// facts.Fact (ADR-0019 S2). Tq-only detail shapes ride the same Detail
// bytes.
type Fact = facts.Fact

const (
	Heartbeat FactType = "task.heartbeat"
	// SessionOpened / SessionClosed record the lifecycle of an INTERACTIVE
	// crush session (the session-close bridge, internal/session). They are
	// observations, not task state: TaskID carries the synthetic
	// "session:<id>" identity, never a real task row, so no enqueue/claim
	// machinery can ever pick them up. SessionClosed's detail names the
	// attributed commits and the minted review/status task IDs.
	SessionOpened FactType = "session.opened"
	SessionClosed FactType = "session.closed"
	// QuestionAsked records an agent's request for a human decision while
	// its task is parked (PapDashboard questions). TaskID is the PARKED
	// task; detail carries the question contract (ref, type, text,
	// options, repo, expiry). The task stays PENDING — the park is a
	// requeue without attempt burn — until RecordAnswer unblocks it.
	QuestionAsked FactType = "task.question-asked"
	// QuestionAnswered records the owner's decision arriving from
	// PapDashboard. Written by Store.RecordAnswer in the SAME transaction
	// that injects the answer into the parked task's payload and clears
	// its NotBefore, so "answered but still parked" is unrepresentable.
	// Idempotent per question ref: a replayed pickup appends nothing.
	QuestionAnswered FactType = "task.question-answered"
	// ErrorObserved records one error report from a production surface
	// (web-client beacon or server panic/5xx middleware): an observation,
	// not task state — the session:* precedent. TaskID carries the
	// synthetic "incident:<fingerprint>" identity, never a real task row,
	// so no enqueue/claim machinery can ever pick it up. Detail carries
	// the full redacted Report (message, stack, release, route, trace
	// link); the incident policy (internal/incident) folds these facts
	// into incidents and mints agent fix tasks as reactions (ADR-0021).
	ErrorObserved FactType = "error.observed"
	// IncidentTaskMinted records that the incident policy enqueued a fix
	// task for one incident: TaskID is the incident identity, detail names
	// the minted task (taskID, sourceSeq, regression, priority). Written
	// after Enqueue succeeds; replayed deliveries converge on the task's
	// dedup key instead of minting duplicates. The task's own lifecycle
	// facts (task.completed, task.dead-lettered) close or fail the
	// incident in the fold — never a mirrored column.
	IncidentTaskMinted FactType = "incident.task-minted"
)

// EffectStatus is the disposition of a run's side effect, recorded on the
// task.released detail when a leased Running task is reclaimed. It exists so
// a crash mid-effect is distinguishable from a clean re-run: turnstone's rule
// is "crashes aren't finishes" and "unknown is not none". The observer that
// reclaims cannot prove the effect never ran, so it records unknown, never a
// false none (a learned/derived label may not lower a certainty).
type EffectStatus string

const (
	// EffectCommitted: the effect is observed to have landed.
	EffectCommitted EffectStatus = "committed"
	// EffectNone: the effect provably never executed. Reserved: no tq
	// path can prove this today (the reclaiming observer sees no
	// dispatch-started marker), so nothing stamps it yet.
	EffectNone EffectStatus = "none"
	// EffectUnknown: the run crashed mid-effect; it may or may not have
	// landed. What the reclaim path records.
	EffectUnknown EffectStatus = "unknown"
	// EffectPartial: some steps of a multi-step effect landed.
	EffectPartial EffectStatus = "partial"
	// EffectRolledBack: the effect was compensated.
	EffectRolledBack EffectStatus = "rolled_back"
)

// ReleasedDetail is the task.released fact detail: the prior attempt's effect
// disposition, so a reclaim is distinguishable from a clean first run.
type ReleasedDetail struct {
	Effect EffectStatus `json:"effect"`
	Reason string       `json:"reason,omitempty"` // lease-expiry | cancelled-mid-run
}

// Journal is the persistence boundary for facts.
type Journal interface {
	// Append records a fact. Implementations must assign Seq and Time when zero.
	Append(ctx context.Context, f Fact) (Fact, error)
	// All returns every fact in Seq order.
	All(ctx context.Context) ([]Fact, error)
	// Since returns facts with Seq strictly greater than after, in Seq order.
	Since(ctx context.Context, after int64) ([]Fact, error)
}

// MemoryJournal is an in-memory Journal for tests and ephemeral queues.
type MemoryJournal struct {
	mu    sync.Mutex
	facts []Fact
}

// NewMemoryJournal returns an empty in-memory journal.
func NewMemoryJournal() *MemoryJournal { return &MemoryJournal{} }

// Append records the fact, assigning Seq and Time when zero.
func (m *MemoryJournal) Append(_ context.Context, f Fact) (Fact, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if f.Seq == 0 {
		f.Seq = int64(len(m.facts) + 1)
	}

	if f.Time.IsZero() {
		f.Time = time.Now()
	}

	m.facts = append(m.facts, f)

	return f, nil
}

// All returns every fact in Seq order.
func (m *MemoryJournal) All(_ context.Context) ([]Fact, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	out := make([]Fact, len(m.facts))
	copy(out, m.facts)

	return out, nil
}

// Since returns facts with Seq strictly greater than after.
func (m *MemoryJournal) Since(_ context.Context, after int64) ([]Fact, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	return AfterSeq(m.facts, after, 0), nil
}

// AfterSeq filters facts to Seq strictly greater than after, in order,
// bounded to the most recent limit when > 0 — the cursor semantics shared
// by slice-backed sources and test fakes.
func AfterSeq(facts []Fact, after int64, limit int) []Fact {
	var out []Fact

	for _, f := range facts {
		if f.Seq > after {
			out = append(out, f)
		}
	}

	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}

	return out
}
