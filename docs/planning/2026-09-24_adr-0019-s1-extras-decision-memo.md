# ADR-0019 S1 decision memo: tq extras, fact-append path, replay design

Date: 2026-09-24
Task: 000001a0d09a (C10 in docs/planning/2026-09-22_23-49_go-cqrs-lite-platform-migration.md)
Status: DECIDED — closes the S1 open design questions (extras home, fact
append path, replay semantics) before any flip. Evidence base: the S1
spikes (`internal/queue/sqlitev4`, `internal/queue/postgresv4`,
`internal/queue/cqrsqlite`) plus the upstream `queue/v4` v4.0.0 source.

Scope: the tq-specific surfaces the upstream `Store[T]` contract
(queue/v4 v4.0.0 `store.go`) lacks — `RecordAnswer`/questions, the
`PriorityScores` cache, `CountFacts`/`FactsSince`/`LastFacts`,
`ProjectCounts` — plus where tq-only facts get written and how the
dogfood journal replays into an engine store.

## 1. Per-extra verdict: upstream-grown vs companion-table

Preference order per ADR-0019: upstream-grown first (the ratification-memo
pipeline is the channel); companion-table as the ship-anyway fallback.
The spikes already ship every extra as a same-DB companion surface over
the engine's tables, so each verdict below names the follow-up needed to
converge on "upstream-grown" later.

| tq extra | upstream surface today | verdict | rationale |
| --- | --- | --- | --- |
| `RecordAnswer` / questions (`task.question-*`, parked-via-NotBefore) | none — upstream has no question/park-and-resume concept | **Companion table, PERMANENT unless upstream ratifies a generic "task annotation/hold" surface.** File a ratification memo, low priority | This is tq's decision-question feature (2026-09-17), deeply tied to tq's `TQ_QUESTION_FILE` executor channel; upstream growing exactly this shape is unlikely. The companion table is small, same-DB, and read by tq only |
| `PriorityScores` cache (`priority_scores`, TTL + orphan eviction) | none — upstream has `UpdatePendingPriority` (the re-rank write) but no verdict cache | **Companion table, PERMANENT unless upstream wants score caching**; ratification memo optional | The cache is an tq AI-spend optimization (default OFF); its eviction semantics are tq policy (DefaultScoreTTL, orphan pruning at scorer completion). Upstream adopting a cache of AI verdicts is out of its abstraction. `UpdatePendingPriority` already carries the re-rank, so nothing is lost |
| `CountFacts(ftype, since)` | none as a count; `Facts` + `FactsForTask` + `HeadSeq` exist | **Prefer upstream-grown** — file a ratification memo FIRST (M4 channel): a filtered count/scan is a generic journal read every consumer of the watermarks/health pattern wants | Pure read pushdown over the engine's own facts table; upstream growth removes tq SQL against engine tables (the only place tq touches engine-schema SQL today). Fallback if the memo stalls: keep the companion read pushdown as shipped in the spikes |
| `FactsSince(ftype, since)` | same as above | **Prefer upstream-grown** — same memo as `CountFacts` | Identical rationale; budget + heartbeats + question expiry all read facts filtered by type+time |
| `LastFacts(limit)` | derivable: `HeadSeq` + `Facts(head-limit, limit)` | **Upstream-grown trivially; no memo needed** — derive from `HeadSeq`+`Facts` in the driver | Two existing upstream calls compose exactly; no new upstream semantics required. Drop the SQL pushdown in the final driver |
| `ProjectCounts` (per-project × status GROUP BY) | `StatusCounts` only (global) | **Companion read pushdown (over the engine's tasks table), PERMANENT unless upstream ratifies a project dimension** | Upstream `task.Task[T]` has no project field in the spec-donor contract — tq's project notion comes from the payload/repo, which upstream does not model. Ratification would require a schema change upstream; not worth the memo yet |

Net: two extras are upstream-growth candidates with one shared
ratification memo (`CountFacts`/`FactsSince` — one memo, both methods),
one composes from existing upstream calls, three stay companion
(questions, scores, ProjectCounts) because they encode tq domain upstream
does not model. The companion surfaces are read/extend-only over engine
tables plus ONE tq-owned table (`priority_scores`) — never redefining
engine schema, pinned by the spike structure.

## 2. tq-fact append path: upstream escape wins, no companion journal

**Verdict: the upstream escape — `FactTx.WithFacts` / `FactSink`
(queue/v4 `factsink.go`) — is the ONLY tq-fact append path. No companion
journal table.**

Upstream already grew the exact seam tq needs: `FactSink.Append` records
caller-authored facts into the engine's facts table, and `FactTx` makes
the append commit-or-roll-back with a surrounding transaction — the
same-tx-facts invariant (ADR-0001 lineage) enforced by the engine
contract instead of two hand-mirrored implementations. tq-specific fact
types (`session.*`, `question-*`, scorer/budget facts) ride the OPEN
`facts.FactType` string type as tq-side constants (S2 direction).

Rules carried over from today's contract:

- tq's `AppendFact` (non-task facts only: `session.opened`/`session.closed`
  and the `(*sqlite.Store).AppendFact` sanctioned surface) maps onto
  `FactTx.WithFacts` with a single append inside.
- Task-lifecycle facts are NEVER written through the escape — the engine
  appends them inside its own operation transactions (the conformance
  suite pins the commit/rollback split, `conformance/facttx.go`).
- Mapping on read: upstream facts ↔ tq `journal.Fact` (type strings +
  Detail bytes) is already proven by the spikes' read paths
  (`scanFacts` in both adapters) — S2 formalizes it in
  `internal/journal/cqrs`.

The spike's `AppendFact` inserting into the `facts` table with tq's SQL
is the transitional shape; the final driver replaces that hand INSERT
with the `FactSink` handed out by `WithFacts`. What does NOT survive:
any idea of a second tq-owned journal table beside the engine's facts —
it would fork the seq space, break watermark monotonicity, and defeat
S2's one-journal goal.

## 3. Replay tool sketch (the migration's source of truth)

Design: **transition replay of the fact journal into a fresh engine
store** — facts are history AND state, so one ordered pass rebuilds both
(the ADR-0019 "facts-first replay is the migration story").

```
tq replay --from <old.db> --to <new.db> [--verify-only]
```

Pipeline (target: `scripts/migrate/replay/`, its own gate per the
module rules; C12/M056 own the implementation):

1. **Stream**: read the old journal in seq order (the existing store's
   `Facts` pagination, or the ADR-0014 cqrs adapter — same facts).
2. **Applier**: walk facts and re-apply transitions on the new engine
   store:
   - `task.enqueued` → `Enqueue` carrying the ORIGINAL task ID, dedup
     key, payload, priority, deps, attempts. ID preservation is the
     open risk (upstream `task.New` assigns IDs; the driver needs an
     ID-carrier constructor or the replay maps old→new IDs and rewrites
     every later fact's attribution — decide at C12, before any
     production rehearsal).
   - subsequent lifecycle facts → the matching store op
     (`Claimed`→ClaimDue-shaped claim record, `Completed`→`Complete`,
     `Failed`→`Fail`, `DeadLettered`→terminal, `Cancelled`→`Cancel`,
     requeue/NotBefore state) so the final projection matches without
     re-executing anything.
   - tq-specific facts (`session.*`, `question-*`, scores) → appended
     via the §2 `FactSink` escape with their Detail bytes verbatim;
     companion tables (`priority_scores`, questions state) rebuilt from
     the same facts.
3. **Equality gate** (`--verify-only` runs ONLY this against a completed
   replay): StatusCounts, per-task fact tails, DLQ contents, watermark
   positions, ProjectCounts — byte-comparable projections, the
   ADR-0019 definition-of-done bar. Any mismatch fails the replay; the
   cutover (owner-run, systemd TQ_DB swap) is gated on a green report
   from a COPY of the production journal (C23 rehearsal).
4. **Idempotence**: replaying is restartable — the applier records the
   consumed seq as a watermark in the new store; a re-run resumes, and
   the engine's dedup-keyed enqueue makes re-applied `enqueued` facts
   convergent.

Rejected alternative: live-tasks-only replay (enqueue current pending
tasks, import facts as inert history rows). It halves applier work but
forks history: terminal tasks' fact tails would live in a different
schema than the live journal, breaking `tq facts`/`tq audit --journal`
parity checks and the §3 equality gate's "per-task fact tails" clause.
Transition replay costs more up front and deletes an entire class of
post-cutover drift.

## 4. Follow-ups this memo feeds (already in the migration plan)

- One upstream ratification memo for `CountFacts`/`FactsSince` (the M4
  channel) — the only upstream-growth action this memo mandates.
- C12/M056: resolve the replay ID-preservation question and build
  `scripts/migrate/replay/`.
- The final S1 driver drops `LastFacts` SQL in favor of
  `HeadSeq`+`Facts` composition and replaces hand `AppendFact` INSERTs
  with `FactSink`.
