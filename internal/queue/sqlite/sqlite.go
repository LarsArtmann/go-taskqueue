// Package sqlite is the embedded SQLite store driver. Since ADR-0019 S1
// it is a THIN DRIVER over the queue/v4-backed adapter
// (internal/queue/sqlitev4): the hand-rolled engine is retired and the
// upstream go-cqrs-lite queue engine owns the task/fact semantics, with
// token-fenced finalizes (upstream ADR-0134) replacing the legacy
// owner-string fence. The tq Store contract is unchanged for callers.
package sqlite

import (
	v4 "github.com/larsartmann/go-taskqueue/internal/queue/sqlitev4"
)

type (
	// Store is the durable store; one queue per database file.
	Store = v4.Store
	// StoreOption configures optional Store behavior.
	StoreOption = v4.StoreOption
	// ArchiveStats reports hot vs archived fact counts and the compaction
	// watermark (highest archived seq; -1 when nothing was archived yet).
	ArchiveStats = v4.ArchiveStats
)

// Open opens (creating if needed) the queue database at path.
func Open(path string, opts ...StoreOption) (*Store, error) {
	return v4.Open(path, opts...)
}

// WithProjectExclusivity turns on store-level per-project serialization:
// ClaimDue will not claim a task whose project already has another running
// task — across ALL pools and processes sharing the same database file, not
// just within one pool. This is the per-repo guarantee for agent pools: two
// agents never work the same repo simultaneously. Every pool sharing the DB
// must opt in; pools that do not opt in ignore the guard. Tasks with an
// empty project are exempt (they are not tied to a repo).
var WithProjectExclusivity = v4.WithProjectExclusivity
