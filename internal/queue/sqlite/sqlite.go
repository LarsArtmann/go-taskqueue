// Package sqlite is the embedded SQLite store driver. Since ADR-0019 S1
// it is a THIN DRIVER over the queue/v4-backed adapter
// (internal/queue/sqlitev4): the hand-rolled engine is retired and the
// upstream go-cqrs-lite queue engine owns the task/fact semantics, with
// token-fenced finalizes (upstream ADR-0134) replacing the legacy
// owner-string fence. The tq Store contract is unchanged for callers.
//
// Open is also the BACKWARD AUTO-UPGRADE seam (ADR-0019 endgame P1): a
// pre-flip hand-rolled database is probed, snapshotted, converged in
// place, and projection-verified before the store serves — the deployed
// binary upgrades the production journal transparently on first open.
// Refuse with TQ_NO_AUTO_UPGRADE=1 (the replay CLI remains the manual
// path). Direct sqlitev4.Open callers bypass the shim deliberately.
package sqlite

import (
	"context"

	"github.com/larsartmann/go-taskqueue/internal/config"
	v4 "github.com/larsartmann/go-taskqueue/internal/queue/sqlitev4"
	"github.com/larsartmann/go-taskqueue/internal/queue/sqlitev4/migration"
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

// Open opens (creating if needed) the queue database at path. A legacy
// pre-flip database at path is auto-upgraded first (see the package doc).
func Open(path string, opts ...StoreOption) (*Store, error) {
	if _, err := migration.UpgradeIfNeeded(context.Background(), path); err != nil {
		return nil, err
	}

	return v4.Open(path, opts...)
}

// OpenWithDeployment opens the store from the resolved deployment struct
// (ADR-0022): the sync tier arrives resolved (no env read here —
// config.FromFlags is the ONE reader), and the legacy auto-upgrade still
// applies exactly as for Open.
func OpenWithDeployment(d config.Deployment, opts ...StoreOption) (*Store, error) {
	if err := migration.UpgradeIfNeeded(context.Background(), d.DBPath); err != nil {
		return nil, err
	}

	return v4.OpenWithDeployment(d, opts...)
}

// WithProjectExclusivity turns on store-level per-project serialization:
// ClaimDue will not claim a task whose project already has another running
// task — across ALL pools and processes sharing the same database file, not
// just within one pool. This is the per-repo guarantee for agent pools: two
// agents never work the same repo simultaneously. Every pool sharing the DB
// must opt in; pools that do not opt in ignore the guard. Tasks with an
// empty project are exempt (they are not tied to a repo).
var WithProjectExclusivity = v4.WithProjectExclusivity
