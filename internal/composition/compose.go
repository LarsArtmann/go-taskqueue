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

	_ "github.com/larsartmann/go-cqrs-lite/metaengine/sqliteengine/v4" // registers the "sqlite" driver (deployment choice)
	"github.com/larsartmann/go-cqrs-lite/system/v4"
	"github.com/larsartmann/go-taskqueue/internal/config"
	"github.com/larsartmann/go-taskqueue/internal/readmodel"
)

// DefaultEngineName is the projection-home engine's deployment name.
const DefaultEngineName = "projections"

// New composes the system root for one deployment. The sqlite queue
// database path decides the projection home (derived beside it) and the
// deployment's resolved sync tier drives the engine's pragmas — the
// config.Deployment is the ONE deployment description (ADR-0022
// deployment lane); the projection home itself stays sqlite-embedded.
func New(ctx context.Context, cfg config.Deployment) (*system.System, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("composition: deployment: %w", err)
	}

	if cfg.Driver != config.DriverSQLite {
		return nil, fmt.Errorf("composition: projection home is sqlite-embedded; driver %q not supported (postgres projection homes are future metaengine work)", cfg.Driver)
	}

	deployment := system.DeploymentConfig{
		Engines: map[string]system.EngineConfig{
			DefaultEngineName: {
				Driver: "sqlite",
				DSN:    readmodel.PathFor(cfg.DBPath),
				// The caller-pragmas the projection-home engine runs,
				// straight from the deployment — ONE pragma source
				// (single_opener.md): the sqliteengine factory
				// prepends journal_mode=WAL + busy_timeout=5000 to
				// these on every connection it builds.
				Pragmas: cfg.ProjectionPragmas(),
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
