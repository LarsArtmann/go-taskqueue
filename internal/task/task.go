// Package task defines the core Task record and its lifecycle.
package task

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

// ID identifies a task. Opaque, unique, roughly time-sortable.
type ID string

// NewID returns a new unique task ID (timestamp prefix + random suffix).
func NewID() ID {
	var b [10]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(fmt.Sprintf("task: crypto/rand failed: %v", err))
	}

	return ID(fmt.Sprintf("%016x", time.Now().UnixMilli()) + hex.EncodeToString(b[:]))
}

// String returns the raw ID.
func (id ID) String() string { return string(id) }

// Task is the unit of work: what to run, for which project, under which constraints.
type Task struct {
	ID           ID              `json:"id"`
	Project      string          `json:"project,omitempty"`
	Type         string          `json:"type"`
	Payload      json.RawMessage `json:"payload,omitempty"`
	Deps         []ID            `json:"deps,omitempty"`
	Priority     int             `json:"priority,omitempty"`
	Attempts     int             `json:"attempts"`
	MaxAttempts  int             `json:"maxAttempts"`
	NotBefore    time.Time       `json:"notBefore"`
	Status       Status          `json:"status"`
	LeaseOwner   string          `json:"leaseOwner,omitempty"`
	LeaseExpires *time.Time      `json:"leaseExpires,omitempty"`
	LastError    string          `json:"lastError,omitempty"`
	CreatedAt    time.Time       `json:"createdAt"`
	UpdatedAt    time.Time       `json:"updatedAt"`
	CompletedAt  *time.Time      `json:"completedAt,omitempty"`
}

// New is a task template for enqueueing. ID, Attempts, Status and timestamps
// are assigned by the store; everything else is caller-supplied.
type New struct {
	Project     string
	Type        string
	Payload     json.RawMessage
	Deps        []ID
	Priority    int
	MaxAttempts int
	NotBefore   time.Time
	// DedupKey, when set, makes Enqueue idempotent: if a task with the same
	// key already exists, that task is returned unchanged and no duplicate is
	// created. Use a stable derivation (e.g. hash of project + source + title)
	// so repeated harvest runs converge instead of re-enqueueing.
	DedupKey string
}

// DefaultMaxAttempts is used when New.MaxAttempts is zero.
const DefaultMaxAttempts = 3

// Normalize applies defaults to a template.
func (n New) Normalize() New {
	if n.MaxAttempts <= 0 {
		n.MaxAttempts = DefaultMaxAttempts
	}

	return n
}
