# ADR-0006: Journal compaction — hot-cold split for the facts log

- Status: Accepted (design; first prototype shipped)
- Date: 2026-09-08
- Deciders: owner + agent session (round-5 plan, ideas I6/I7 / task M17)
- Relates to: ADR-0001 (facts-first core), round-6 watermarks work

## Context

Every state change appends an immutable fact; the `tasks` table is a
projection of the journal's CURRENT state. After weeks of dogfooding the
journal grows without bound (the dogfood DB crossed 141 facts in two days;
a two-pool setup multiplies that). Nothing in the operating path needs the
full history: claims read `tasks`, the dashboard reads bounded fact tails,
bridges read forward from a watermark. The journal's tail weight is pure
audit value — valuable, but not at hot-query cost.

Two naive fixes are rejected:

- **DELETE old facts** — the journal's immutability is the trust contract
  (`tq show` must always be able to answer "what happened"). Silent
  deletion breaks audit and any future replay.
- **Move the whole DB / VACACUM-style rewrite** — compaction must be
  online (the pool may keep running) and incremental.

## Decision

Compaction is a **hot-cold split at task granularity**: facts of tasks
that are TERMINAL (completed / dead / cancelled) move into `facts_archive`
(a sibling table in the same DB). Nothing is ever deleted.

Invariants (enforced by `SQLiteStore.ArchiveFactsBefore`, the prototype):

1. **Task-granular, never seq-granular**: a task's facts move together or
   not at all. Splitting a task's trail across hot/cold would make
   `tq show` a two-source merge for active stories.
2. **Terminal-only**: a task qualifies only when its status is terminal
   AND its highest fact seq is below the caller's cutoff. Pending,
   running, delayed, and rescued (pending-again) tasks keep their facts
   hot — the reclaim/heartbeat/cancel paths read them.
3. **Projections never read the archive**: `tasks` is untouched; `Facts()`
   serves the hot table only. The dashboard, `tq tail`, and bridges are
   watermark consumers — a cold prefix is invisible to them by design.
4. **Watermark bookkeeping**: each run records the highest archived seq
   in `journal_meta`, so tooling can state "history complete above seq N;
   below N, terminal-task history is in the archive".

Command sketch (F89, implementation follows demand):

```
tq journal compact --before SEQ --min-age 720h [--dry-run]
  # moves facts of terminal tasks whose max seq < min(SEQ, now-minAge)
  # prints: tasks archived, facts moved, new watermark
```

`--min-age` guards against racing a task that just terminated while a
bridge watermark still points below its facts.

## Alternatives considered

- **Separate archive FILE (attach + copy)**: better cold-storage isolation
  but breaks the single-file deploy story (the project's headline). A
  later escalation can move `facts_archive` rows out in a second step
  without changing these semantics.
- **Snapshot/rollup rows ("task summarized at seq N")**: replaces detail
  with a summary — loses `tq show` fidelity; the archive keeps it.
- **Doing nothing**: fine until it isn't; the dogfood pool makes "isn't"
  a moving target.

## Consequences

- `Facts(ctx, after, limit)` with `after` below the watermark returns a
  hot-only prefix: replay-from-zero tooling must consult the watermark
  first (recorded in `journal_meta`, surfaced by `tq journal compact`).
- The archive table needs the same `(task_id, seq)` index; compaction is
  one transaction (INSERT…SELECT + DELETE) under the single serialized
  writer, so it cannot race a claim.
- `tq show <id>` for an archived task becomes a two-table read; the
  prototype keeps `Facts` hot-only and leaves the union view to the
  command's implementation.
