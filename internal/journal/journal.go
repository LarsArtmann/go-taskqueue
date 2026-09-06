// Package journal defines the append-only fact log that records everything
// that ever happened to tasks. All derived views (queue, DLQ, stats) are
// projections over these facts.
package journal

import (
	"context"
	"encoding/json"
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
	Released     FactType = "task.released" // lease expired, back to pending
	Requeued     FactType = "task.requeued" // preflight refusal, no attempt burned
)

// Fact is one immutable observation about one task.
type Fact struct {
	Seq     int64           `json:"seq"`
	Time    time.Time       `json:"time"`
	TaskID  string          `json:"taskId"`
	Type    FactType        `json:"type"`
	Owner   string          `json:"owner,omitempty"`
	Attempt int             `json:"attempt,omitempty"`
	Error   string          `json:"error,omitempty"`
	Detail  json.RawMessage `json:"detail,omitempty"`
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
