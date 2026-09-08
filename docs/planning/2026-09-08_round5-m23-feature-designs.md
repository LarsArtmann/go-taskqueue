# Round-5 M23 — Feature Design Pack (F121–F125)

**Date:** 2026-09-08. Designs for the v0.3 feature set; each names the
mechanism, the facts-first encoding, and the open question that must be
answered before implementation. Nothing here is built yet — these are the
sketches the plan asked for, recorded where the next session can pick
them up.

## F121 — Cron / recurring tasks (D83)

**Mechanism**: a `cron(spec)` table row materializes the NEXT occurrence
as an ordinary task with `DedupKey = "cron:<id>:<bucket>"` — time-bucketed
dedup keys make materialization idempotent: a re-run tick that recomputes
a missed bucket gets the stored task back, never a duplicate. A pool tick
calls `MaterializeDueCrons(now)`: for each due spec, enqueue with the
bucket key. No scheduler daemon; the pool IS the scheduler (same pattern
as harvest).

**Encoding**: `task.enqueued` fact detail gains `"cron": "<spec>"`;
spec storage is a `crons` table (id, spec, task template JSON, enabled).
**Open question**: missed-bucket policy — catch up all missed buckets
(after downtime) or only the newest? Harvest's answer (newest only,
dedup suppresses the rest) is the precedent to copy.

## F122 — Per-repo daily budgets

**Mechanism**: `budget.Guard` already projects global spend from
`CountFacts(task.Enqueued, since)`; per-repo needs the same count grouped
by task project: one query `SELECT project, COUNT(*) FROM facts f JOIN
tasks t ON t.id = f.task_id WHERE f.type='task.enqueued' AND f.time >= ?
GROUP BY project`. Guard check becomes per-repo: repo R's tick is refused
when R's count ≥ R's cap (`--repo-budget name=N,...` ladder, mirroring
`--repo-timeout`). Facts stay the source of truth — no counters table,
no drift.

**Open question**: do REVIEW and FIX tasks (minted by the sweeper) count
against the reviewed repo's budget? Today every enqueue counts globally;
per-repo must decide, and the answer should be yes (they spend real
money in that repo).

## F123 — Retry-policy table (D99) + cross-repo DAG templates (D97)

**Retry tables**: `task.New` gains optional `Retry RetryPolicy
{On []string-class, Backoff string, MaxAttempts int}`; the worker's Fail
path consults the task's policy instead of the global ExpBackoff.
Encoding: policy rides in the task row (JSON column) — it is per-task
input, not journal state. Permanent/transient classification already
flows through error classes; the policy maps class → backoff.

**DAG templates**: a template is a named set of task.New + dep edges in a
file (`templates/*.json`); `tq enqueue --template nightly-tests`
materializes the graph with dedup keys `tpl:<name>:<item>:<bucket>`.
Edges are the existing deps gate — templates are sugar over enqueue, not
a new engine. **Open question**: partial materialization on failure
(some items enqueued, template invalid midway) — validate the whole
template BEFORE enqueueing anything.

## F124 — Session chains (D94) + per-project concurrency

**Session chains**: agent payloads already carry `Session`; a chain is a
task whose payload sets `Session: <previous task's session>` — the
harvester's fix-task minting already does this shape manually. Design:
`task.New.ChainOf task.ID` — on completion, the store appends
`task.completed` detail `{"chainNext": ...}`? Simpler and honest: chains
stay a PAYLOAD convention (repo + session id continuity), documented,
with `tq show` linking tasks sharing a session id. No schema.

**Per-project concurrency**: `--project-concurrency name=N` — a store
claim-side gate like project exclusivity but counting: candidate WHERE
NOT EXISTS (SELECT ... running tasks of same project >= N). The
exclusivity option is the N=1 special case; generalize
`WithProjectExclusivity` to `WithProjectConcurrency(map[string]int, default int)`.

## F125 — Completion webhooks + /metrics sketch

**Webhooks**: a bridge, not a queue feature: `tq worker --webhook-url U`
posts `task.completed`/`task.dead-lettered` envelopes (same shape as the
papdashboard bridge, minus alert semantics). Reuse the bridge watermark
pattern; idempotency key = fact seq. The papdashboard bridge is the
template — factor its tailing loop into a shared `internal/bridge/tailer`
when the second consumer lands (rule of three says wait for the third).

**/metrics**: `tq serve --metrics` adds `GET /metrics` (Prometheus text
format, token-gated like everything else): counters from StatusCounts +
HeadSeq + fact-rate from CountFacts windows. Read-only projection of
existing queries — no new state.

---

**Cross-cutting rule for all of the above**: every feature must answer
"which fact records this?" before "which table stores this?" — the
journal stays the source of truth (ADR-0001); tables are indexes.
