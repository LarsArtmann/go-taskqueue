// Package journal defines the append-only fact log that records everything
// that ever happened to tasks. All derived views (queue, DLQ, stats) are
// projections over these facts.
package journal

import (
	"context"
	"encoding/json/jsontext"
	"sync"
	"time"
)

// FactType enumerates the kinds of facts that can be recorded.
type FactType string

const (
	Enqueued     FactType = "task.enqueued"
	Claimed      FactType = "task.claimed"
	Heartbeat    FactType = "task.heartbeat"
	Completed    FactType = "task.completed"
	Failed       FactType = "task.failed"        // attempt failed, will retry
	DeadLettered FactType = "task.dead-lettered" // attempts exhausted
	Cancelled    FactType = "task.cancelled"
	// CancelRequested records an operator's request to stop a Running
	// task. The fact IS the flag: the executing worker observes it at its
	// next heartbeat, cancels the execution context, and records
	// task.cancelled; a crashed worker's expired lease finalizes the same
	// cancel at reclaim. No task-row column mirrors it.
	CancelRequested FactType = "task.cancel-requested"
	Released        FactType = "task.released" // lease expired, back to pending
	Requeued        FactType = "task.requeued" // preflight refusal, no attempt burned
	// Orphaned records that a Running task's lease expired and NO worker
	// reclaimed it (the worker died with the pool down). It is an
	// observation, not a state change: the task stays Running until a
	// ClaimDue reclaim (or a human) picks it up. Appended idempotently by
	// Store.MarkOrphaned, so `tq show` can explain a stranded task.
	Orphaned FactType = "task.orphaned"
	// SessionOpened / SessionClosed record the lifecycle of an INTERACTIVE
	// crush session (the session-close bridge, internal/session). They are
	// observations, not task state: TaskID carries the synthetic
	// "session:<id>" identity, never a real task row, so no enqueue/claim
	// machinery can ever pick them up. SessionClosed's detail names the
	// attributed commits and the minted review/status task IDs.
	SessionOpened FactType = "session.opened"
	SessionClosed FactType = "session.closed"
)

// Fact is one immutable observation about one task.
type Fact struct {
	Seq     int64          `json:"seq"`
	Time    time.Time      `json:"time"`
	TaskID  string         `json:"taskId"`
	Type    FactType       `json:"type"`
	Owner   string         `json:"owner,omitempty"`
	Attempt int            `json:"attempt,omitempty"`
	Error   string         `json:"error,omitempty"`
	Detail  jsontext.Value `json:"detail,omitempty"`
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

	var out []Fact

	for _, f := range m.facts {
		if f.Seq > after {
			out = append(out, f)
		}
	}

	return out, nil
}
