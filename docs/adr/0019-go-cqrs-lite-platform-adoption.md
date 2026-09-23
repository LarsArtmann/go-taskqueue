# ADR-0019: go-cqrs-lite is the queue, read-model, and composition platform

Date: 2026-09-22
Status: Accepted — owner ruling 2026-09-22 ("the whole idea of this project
was that it uses go-cqrs-lite system/ + metaengine/"). Supersedes the
"direct storage adoption rejected" stance (ADR-0001; ADR-0014 §Context)
for the queue stores and the read side. ADR-0014's journal adapter itself
stands unchanged.

## Context

Founding intent, restated by the owner 2026-09-22: this project exists to
put go-cqrs-lite to work — the queue runs on it, reads project through it,
the app composes through it. The 2026-09-13 assessment froze at "NOT
adopted" on three premises: (1) the upstream durable-work-queue assembly
was in progress and untagged, (2) swapping storage meant relocating claim
SQL onto a generic event store, (3) a live-journal migration had no
principled story. All three premises have since rotted out from under the
verdict:

- The upstream `queue/` family SHIPPED and the tags are PUSHED:
  `queue/v4.0.0`, `queue/sqlite/v4.0.0`, `queue/postgres/v4.0.0`,
  `queue/mysql/v4.0.0`, `claiming/v4.0.0` (verified via
  `git ls-remote --tags origin`, 2026-09-22). Its `Store[T]` contract is
  transcribed from THIS repo's contract — `queue/README.md` names tq the
  spec donor — covering lease claims with crash reclaim, dedup-keyed
  enqueue, retries with backoff, DLQ rescue/dismiss, `MarkOrphaned`,
  cooperative cancels, DAG-dependency gating at claim, bounded priority
  aging (`PriorityAgingDaysPerPoint`), the same-transaction fact journal,
  and watermarks — held identical by ONE shared conformance suite
  (`queue/conformance`) instead of tq's mirrored per-backend suites.
- Upstream finalizes are token-fenced (upstream ADR-0134): strictly safer
  than tq's owner-string finalizes — a lapsed claim cannot resurrect.
- The upstream v5 direction (ADR-0123) makes `metaengine` Store +
  `system` composition root the blessed surface; the v1 read-model tiers
  and `stack/` presets are removed in v5. tq piggy-backing v1 tiers now
  would adopt into deprecation.
- Facts-first design means the journal IS the migration source: a fresh
  engine store can be populated by replaying facts; the cutover needs no
  in-place schema surgery on the live DB.
- The hand-rolled mirror backends are the repo's largest duplication
  (art-dupl 2026-09-23: 12 clone groups are sqlite↔postgres mirror pairs,
  up from 7-of-9 when this ADR was written — recount before quoting) —
  kept alive only by the rejected-adoption verdict.

## Decision — staged adoption; every stage ships green on its own

**S1 — queue/v4 becomes the task store.** The hand-rolled engines
(`internal/queue/sqlite`, `internal/queue/postgres`) are replaced by thin
drivers over `queue/sqlite/v4` + `queue/postgres/v4`. tq's `queue.Store`
stays the in-repo boundary (the ADR-0016 facades keep compiling). The
tq-specific surfaces the upstream contract lacks — `RecordAnswer` /
questions, the `PriorityScores` cache, `CountFacts` / `FactsSince` /
`LastFacts`, `ProjectCounts` — land as a tq-side store extension over the
SAME database (companion tables), unless upstream grows them first (the
preferred outcome; upstream already runs the ratification-memo pipeline
for queue semantics). Workers move from owner-string finalizes to claim
tokens (theft detection moves INTO the store).

**S2 — one journal.** tq's fact vocabulary rides upstream `facts.Fact`
(`FactType` is an open string type): lifecycle facts come from the
engine's own transitions; tq-specific fact types (`session.*`,
`question-*`, scorer/budget facts) remain tq-side constants written via
the S1 extension's append path. `internal/journal/cqrs` (ADR-0014)
re-points at the unified journal. The facts-in-the-same-transaction
invariant is then enforced by the shared engine contract, not by two
hand-mirrored implementations.

**S3 — read models on metaengine.** `tq serve` views, `tq stats`, and
the machine API's read side become `metaengine` Store collections
(planned tables via `LayoutPlanApplier` / `BuildLayoutPlanFromType`) fed
from the journal; `Watcher[V]` / `ServeSSE` replaces the hand
journal-tailer→hub fan-out (ADR-0003 Phase D read path) for live
fragments. CSP / token-auth hardening carries over unchanged — no
unauthenticated oracles (ADR-0008), and the existing pins
(`TestRoutesAreReadOnly`, health-CSP tests) stay green.

**S4 — composition via system/.** Runtime composition (store, executors,
sweepers, bridges, consumers) is expressed as `system/` DomainConfig
wiring, adopting the lifecycle/checkpoint machinery where it maps 1:1
onto the run.Group plumbing. Once the flip is proven in dogfood, the
hand-rolled backends and their mirrored conformance suites are DELETED —
the mirror-clone mass dies with this stage.

**Data migration (gates production; part of S1's definition of done):**
the dogfood journal (`/mnt/pool/services/tq/tq.db`) is replayed from its
fact journal into a fresh engine store under a scratch path, verified by
projection equality (StatusCounts, per-task fact tails, DLQ contents,
watermark positions), then cutover swaps the systemd unit's `TQ_DB`.
Cutover is owner-run.

## Consequences

- The parity bar at every stage: `queue/conformance` + tq's own suites
  green. Facade modules grow the go-cqrs-lite require only at the
  backend/adapter layer — never `internal/task`/`internal/journal`
  (DAG purity, ADR-0014 §Decision 2).
- Upstream becomes a live dependency in the ordinary sense: queue-family
  tag waves are tracked like any other go.mod bump (tagged requires +
  relative replaces per the release gates; `check-go-mods.sh` enforces).
- The open upstream owner-gate — queue dep-validation semantics
  (M4 §f1, Reply A recommended: keep at-enqueue `ErrDanglingDep`) — does
  NOT block S1: tq adopts Reply A behavior and re-visits only if the
  owner rules otherwise.
- Gained: the mysql backend for free, store-level theft detection, one
  conformance suite instead of mirrored ones, deletion of the largest
  clone mass in the repo, and the v5-ready read/composition surface.
- Kept: tq's `Store` contract as the in-repo boundary, the facade
  modules, and every operational invariant — facts in the same tx
  (now engine-enforced), single serialized writer on sqlite, task
  execution context surviving pool shutdown.

Execution is tracked in `TODO_LIST.md` (section "go-cqrs-lite platform
adoption"); stages serialize S1→S4.
