# B1: Single Projection-Home Opener — Design Note

Status: DESIGN (implementation: plan task B2, 2026-10-09)
Plan: docs/planning/2026-10-09_20-23_CONFIG-SYSTEM-ALL-IN-BRIDGE.md (B1→B2→B3→B4)
Review source: docs/architecture-understanding/2026-10-09_20-10_config-vs-system-composition-root.md (P1a)

## Problem

One tq serve run opens the projection-home SQLite file
(`<db>.readmodel.db`) through TWO independent constructors:

1. `composition.New` declares it to `system.New` via
   `DeploymentConfig.Engines["projections"]` (compose.go:41-55); system
   builds its own `*sql.DB` from Driver+DSN+Pragmas at
   `createEngineFromDriver` (pinned vendor system/v4 constructor.go).
2. `readmodel.Open` opens a SECOND `*sql.DB` on the same file via
   `sqliteengine.NewSQLiteEngineFromDSN(path, "synchronous=NORMAL",
   "cache_size=-32768")` (readmodel/model.go:130-131).

Two connections to one WAL file is legal SQLite, but it is a config
split-brain: the two connections run DIFFERENT pragma sets, and the
plan's original goal ("exactly one constructor call reachable from
serve") cannot be met by exporting an engine from the root.

## The pragma union (effective, per connection)

| Pragma               | system-declared connection   | readmodel connection        |
| -------------------- | ---------------------------- | --------------------------- |
| journal_mode=WAL     | yes (explicit)               | yes (upstream default)      |
| busy_timeout         | 5000 (explicit)              | 5000 (upstream default)     |
| synchronous          | SQLite default (FULL) — GAP  | NORMAL (explicit)           |
| cache_size           | SQLite default — GAP         | -32768 (explicit)           |
| MaxOpenConns         | 1 (system policy)            | 1 (upstream policy)         |

Upstream fact (verified against the PINNED vendor source
vendor/github.com/larsartmann/go-cqrs-lite/metaengine/sqliteengine/v4@v4.5.2
dsl.go:91-140): `NewSQLiteEngineFromDSNWith` ALWAYS prepends
`journal_mode=WAL` + `busy_timeout=5000` before caller pragmas and sets
`MaxOpenConns(1)`. So the readmodel connection already carries the full
union; the DIVERGENT connection is the system-declared one.

## Why NOT system-owned (the plan's original B2.2)

Plan micro B2.2 asked to "export the projection-home engine from the
composition root". Not implementable against pinned system/v4@v4.11.0
(verified against vendor source, not the ahead-of-pin local checkout):

- No `Engine(name string)` accessor exists on `*System` — the full
  method set (system.go, introspection.go, introspection_extended.go,
  shutdown.go, timers.go, scream_store.go) exposes only `EngineNames()`,
  `MetaEngine()`, `TimerEngine()`, `EventStore()`, and friends.
- `EngineConfig` is data only (Driver/DSN/Pragmas/Priority/
  MaterializedViews, config_types.go:198-218); system builds engines
  internally from the registry — there is no instance-injection point.
- Dropping the engine from `DeploymentConfig` degrades the S4 root to
  the in-memory event-store fallback with an advisory scream
  (constructor.go ~181-197) — worse than today's decorative double-open.

## Decision: the review's P1a INVERSION

> "or, if the engine accessor is awkward, invert: readmodel owns it and
> the root references it" (review §P1a, line 162)

B2 implements the inversion, not the export:

1. readmodel exports ONE shared pragma-literal source (the union list)
   and gains `WithEngine(eng)` (nil-default keeps the current open —
   same Option pattern as WithPoll/WithBatch/WithDurableCursor,
   model.go:81-111).
2. composition's `DeploymentConfig` references the exported pragma
   source (one literal in the repo), and `NewProjectionRuntime` accepts
   the tq-owned engine (build it once, hand it to readmodel via
   WithEngine).
3. The duplicate pragma literal in readmodel/model.go:130-131 dies; the
   duplicate in compose.go:46-49 dies with it.

Net effect after B2: ONE tq-owned constructor call for the projection
home per serve run; the system-declared connection keeps running the
identical union (its DSN-only Driver path applies the upstream
WAL/busy_timeout defaults; the pragma literals converge via the shared
source). The literal "exactly one constructor call reachable from
serve" gate becomes "exactly one tq-owned opener + ONE pragma source";
the system-side connection is upstream surface until the seam below
lands. Owner question Q1 parked on this rewording (accept-recommend;
re-ask at the B4 gate).

## Invariants untouched

- Single serialized writer per FILE: after B2 the projection home has
  two writers (tq engine + system connection) exactly as today — both
  MaxOpenConns(1), both WAL+busy_timeout 5000. The tq queue journal DB
  keeps its ONE shared `*sql.DB` (MaxOpenConns(1)) — not touched by B2.
- Facts in the same tx as state: unchanged.
- Task contexts survive pool shutdown: unchanged.
- RowsAffected() re-checks: unchanged.

## Pin caveat + post-v5 tightening

Claims above are pinned to system/v4@v4.11.0 and sqliteengine/v4@v4.5.2
(vendor/modules.txt). The upstream seam ask (docs/memos/2026-10-09_
upstream-seam.md) requests `Engine(name)` / instance injection; when it
lands (post-v5), B2's tq-owned opener collapses into a root-exported
engine and the gate tightens to the original literal wording.
