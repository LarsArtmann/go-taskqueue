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

// Migrate creates the companion tables on the shared database handle.
func Migrate(ctx context.Context, r Runner) error {
	if _, err := r.ExecContext(ctx, CompanionSchema); err != nil {
		return fmt.Errorf("companion: migrate priority_scores: %w", err)
	}

	return nil
}
