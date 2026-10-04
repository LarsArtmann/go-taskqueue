// Package composition is the ADR-0019 S4 composition root (endgame P4):
// the ONE place tq's runtime meets go-cqrs-lite's system/ package. It
// builds system.New over a durable sqlite engine at the projection home
// (<db>.readmodel.db) with an EMPTY domain BY DESIGN — the queue engine's
// fact journal IS the journal since S2, so system's own event adapter
// stays unused (no second source of truth; the split-brain gate of the
// archived composition map §3).
//
// What the System owns here (the map's sanctioned shape):
//
//   - the projection-home engine (deployment decision, one DeploymentConfig
//     place: swap sqlite -> postgres by editing Engines, never domain code);
//   - close ordering for the surfaces that adopt it (GracefulClose after
//     the runactor actors stop — serve today, per §4c "adopt per surface");
//   - the growth seam: timers (§4a ruled: sweeper ticking STAYS runactor
//     until a genuinely timer-shaped feature lands), deciders, and
//     system.Lookup/metaengine query declarations attach to THIS root.
//
// What it deliberately does NOT own: the tq queue.Store (S1's thin-driver
// seam, injected as today), the worker claim loops, the sweepers, and the
// readmodel's fold projector (a tq-side projection over the QUEUE journal;
// its collections live on the same projection-home file through their own
// metaengine Plan).
package composition

import (
	"context"
	"fmt"

	"github.com/larsartmann/go-cqrs-lite/system/v4"
	_ "github.com/larsartmann/go-cqrs-lite/metaengine/sqliteengine/v4" // registers the "sqlite" driver (deployment choice)
	"github.com/larsartmann/go-taskqueue/internal/readmodel"
)

// DefaultEngineName is the projection-home engine's deployment name.
const DefaultEngineName = "projections"

// New composes the system root for one queue database. dbPath is the
// QUEUE database path (the projection home is derived beside it).
func New(ctx context.Context, dbPath string) (*system.System, error) {
	deployment := system.DeploymentConfig{
		Engines: map[string]system.EngineConfig{
			DefaultEngineName: {
				Driver: "sqlite",
				DSN:    readmodel.PathFor(dbPath),
				Pragmas: []string{
					"journal_mode=WAL",
					"busy_timeout(5000)",
				},
			},
		},
		Instances: []system.InstanceConfig{
			{Role: system.RoleProjections, Engine: DefaultEngineName},
		},
	}

	sys, err := system.New(ctx, system.DomainConfig{}, deployment)
	if err != nil {
		return nil, fmt.Errorf("composition: system root: %w", err)
	}

	return sys, nil
}
