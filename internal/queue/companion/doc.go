// Package companion is the single home for the tq companion surfaces the
// ADR-0019 spike adapters (sqlitev4, postgresv4, cqrsqlite) currently
// mirror: the facts/watermark/priority-score/status/project reads,
// listWhere/List, ClaimDue with project exclusivity, Requeue, and the
// RecordAnswer/question cluster, all written against a placeholder-
// dialable Runner so one implementation serves every backend dialect.
//
// STATUS: scaffolded by the 2026-09-26 dedup window (owner ruling
// "accepts ≈ 0"); the extraction itself is designed in
// docs/planning/2026-09-26_companion-extraction-design.md and tracked in
// TODO_LIST. The mirrored adapters still own the code until that window
// lands; scripts/check-mirror-clones.sh (advisory until then) counts the
// remaining cross-backend clone groups.
package companion
