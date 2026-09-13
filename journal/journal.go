// Package journal is the public facade over the fact-journal module
// (ADR-0016): Fact types, the append-only Journal interface, and the
// in-memory implementation. The implementation stays internal; this
// package pins the names.
package journal

import internaljournal "github.com/larsartmann/go-taskqueue/internal/journal"

// Fact record and its type.
type (
	Fact     = internaljournal.Fact
	FactType = internaljournal.FactType
)

// Fact types.
const (
	Enqueued        = internaljournal.Enqueued
	Claimed         = internaljournal.Claimed
	Heartbeat       = internaljournal.Heartbeat
	Completed       = internaljournal.Completed
	Failed          = internaljournal.Failed
	DeadLettered    = internaljournal.DeadLettered
	Cancelled       = internaljournal.Cancelled
	CancelRequested = internaljournal.CancelRequested
	Released        = internaljournal.Released
	Requeued        = internaljournal.Requeued
	Orphaned        = internaljournal.Orphaned
	Reprioritized   = internaljournal.Reprioritized
	SessionOpened   = internaljournal.SessionOpened
	SessionClosed   = internaljournal.SessionClosed
)

// Journal boundary and the in-memory implementation.
type (
	Journal       = internaljournal.Journal
	MemoryJournal = internaljournal.MemoryJournal
)

var NewMemoryJournal = internaljournal.NewMemoryJournal
