package companion

import (
	"context"
	"fmt"
)

// CompanionSchema is the tq-side table set the upstream engines do not
// carry (the tasks/facts/deps/watermarks tables come from the engines'
// own migrate — same-DB companion surfaces read and extend them but
// never redefine them). BIGINT has INTEGER affinity in SQLite, so the
// one DDL serves both engine families.
const CompanionSchema = `
CREATE TABLE IF NOT EXISTS priority_scores (
	item_key       TEXT PRIMARY KEY, -- the harvest dedup key (repo + item text)
	score          INTEGER NOT NULL, -- 0-100
	effort_minutes INTEGER NOT NULL, -- estimated agent effort
	source         TEXT NOT NULL,    -- scorer identity, e.g. "ai:<model>"
	reasoning      TEXT NOT NULL,    -- one-line why
	tokens         INTEGER NOT NULL, -- what the verdict cost
	scored_at      BIGINT NOT NULL   -- unix millis
);
`

// CompanionIndexes extends the engine-owned tasks table with the claim
// path's hot probes (an index is an extension, not a redefinition). Both
// are PARTIAL over the running set only — a handful of rows — so they
// stay tiny no matter how deep the terminal history grows:
//
//   - idx_tasks_project_running serves ClaimDue's project-exclusivity
//     NOT EXISTS: with the engine's full idx_tasks_project the probe
//     walks every historical row of the candidate's project (thousands
//     after a harvest-heavy life); the partial index answers from the
//     running set alone.
//   - idx_tasks_lease_running serves the expired-lease reclaim arm
//     (status='running' AND lease_expires <= ?) as a direct range probe
//     instead of a scan of all running rows filtered row-by-row.
//
// SQLite and PostgreSQL accept identical partial-index DDL, so the one
// const serves both dialects. The indexed columns predate the index
// (they are engine-schema columns), keeping the "indexes only after the
// column exists" migration rule satisfied by construction.
const CompanionIndexes = `
CREATE INDEX IF NOT EXISTS idx_tasks_project_running ON tasks(project) WHERE status = 'running';
CREATE INDEX IF NOT EXISTS idx_tasks_lease_running ON tasks(lease_expires) WHERE status = 'running';
`

// Migrate creates the companion tables on the shared database handle.
func Migrate(ctx context.Context, r Runner) error {
	if _, err := r.ExecContext(ctx, CompanionSchema); err != nil {
		return fmt.Errorf("companion: migrate priority_scores: %w", err)
	}

	if _, err := r.ExecContext(ctx, CompanionIndexes); err != nil {
		return fmt.Errorf("companion: migrate claim indexes: %w", err)
	}

	return nil
}
