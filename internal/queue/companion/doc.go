// Package companion is the single home for the tq companion surfaces the
// ADR-0019 spike adapters (sqlitev4, postgresv4, cqrsqlite) currently
// mirror: the facts/watermark/priority-score/status/project reads,
// listWhere/List, ClaimDue with project exclusivity, Requeue, and the
// RecordAnswer/question cluster, all written against a placeholder-
// dialable Runner so one implementation serves every backend dialect.
//
// STATUS: extracted by the 2026-09-26 companion window (owner ruling
// "accepts ≈ 0"): the sqlitev4, postgresv4, and cqrsqlite adapters
// delegate their companion surfaces here — the design in
// docs/planning/2026-09-26_companion-extraction-design.md is LANDED for
// production code (art-dupl: zero cross-backend adapter groups).
// The three mirrored conformance suites are the last mirrors; their
// consolidation into companion/conform is tracked in TODO_LIST and
// ratcheted by scripts/check-mirror-clones.sh (strict: baseline
// scripts/mirror-baseline.txt fails on any new cross-backend group).
package companion
