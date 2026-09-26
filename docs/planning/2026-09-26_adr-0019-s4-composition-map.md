# ADR-0019 S4 composition map — `system/` DomainConfig vs the tq runtime

Date: 2026-09-26 · Status: ACTIVE (S4 pre-work; execution serialized behind
S2 + the S3 flip) · Feeds TODO row "ADR-0019 S4" in the go-cqrs-lite
platform-adoption section.

Purpose: the S4 row's composition half ("compose runtime (store,
executors, sweepers, bridges) via `system/` DomainConfig") needs a
component→role verdict per tq runtime piece BEFORE the execution window,
so the window applies a map instead of researching upstream. Every claim
below cites file:line verified 2026-09-26 against
`~/projects/go-cqrs-lite` (system@master, v4 family) and this repo at HEAD.

## 0. Ground truth that reshapes the row

- **The DELETE half already happened.** Commit f0643178 (2026-09-25 11:15,
  the S1 flip) removed `internal/queue/sqlite/sqlite.go` (-2136),
  `internal/queue/postgres/postgres.go` (-1816),
  `postgres/conformance_test.go` (-1470), `sqlite/store_test.go` (-3592),
  `postgres/postgres_test.go` (-377) and `postgres/mirror.go` (-96); both
  facade dirs are thin drivers over `internal/queue/{sqlitev4,postgresv4}`
  now (package docs: internal/queue/sqlite/sqlite.go:1-9,
  internal/queue/postgres/postgres.go:1-9). The 12 art-dupl mirror clone
  groups died there, not at S4. What remains of the DELETE clause is the
  "once proven in dogfood" residue: the owner-run cutover (the section's
  last row) plus the art-dupl `-t 4` recount as the dividend metric
  (item 49, report 2026-09-25_12-16).
- **S3 is in flight** (`internal/readmodel/` — metaengine Store
  collections, `metaengine.NewWatcher[TaskRow]` at
  internal/readmodel/model.go:120; webui/httpapi churn landing
  09-26 00:24..01:41). The serve half of S4 composition lands WITH S3,
  not after it.
- **S2 is not landed** (journal not unified on `facts.Fact` for consumers;
  only the flip-independent `AppendFact`-via-`FactTx` slice shipped,
  TODO row "ADR-0019 S2"). This is the hard serialization gate for S4 —
  see §3.

## 1. What `system/` actually offers

`system.New(DomainConfig, DeploymentConfig)` wires a CQRS composition
root: command/query dispatchers, an event journal over metaengine
engines, auto-wired projections on a `projectionhost.Host`, buses, and
lifecycle (`Start` via `ManageTimers`/host start, `GracefulClose` at
system/system.go:307, `Close` at system/system.go:265). The consumer
surface is `DomainConfig` (system/config_types.go:14): `Commands`,
`Queries`, `Timers`, `Projections`, `Evolutions`, `Events`,
`CheckpointStore` (config_types.go:96-101 — default: a
`system_checkpoints` Map collection on the deployment engine), plus
projection decoders. Runtime reads: `system.Get`/`Find`/`GetCount`
(system/runtime.go:20,110,159). Timers: `sys.TimerEngine()` +
`sys.ManageTimers(sched)` (system/timers.go:21,42-46) — started on
Start, stopped on GracefulClose/Close.

What `system/` does NOT offer: process supervision. There is no
actor/first-exit-cancels-group primitive, no signal handling. It is a
wiring + lifecycle root, not a supervisor.

## 2. Component→role map (the verdict table)

| tq runtime component (today)                                    | system/ role candidate                                  | Verdict |
| --------------------------------------------------------------- | ------------------------------------------------------- | ------- |
| serve read side: board/table/stats/httpapi reads (S3 flip pending) | Projections + `Watcher`/`ServeSSE` (readmodel already: model.go:53-64,120) | **1:1 — rides S3**, not a separate S4 task |
| readmodel watcher checkpoint                                    | `DomainConfig.CheckpointStore` / default `system_checkpoints` collection | **1:1** (rebuildable projection ⇒ disposable checkpoint home is correct) |
| sweeper TICKS (review/dlqfix/status/prioritize/depsweep cadence) | `DomainConfig.Timers` → `ManageTimers` (timers.go:42-46) | **1:1 for the tick machinery**; sweeper STATE stays in the queue `watermarks` table (internal/watermark) — see §4 open question a |
| worker claim loops (internal/worker `Pool.Start`)               | — (a lease-polling worker is not a decider, command, projection, or timer) | **no-fit** — stays a `runactor` actor |
| papdashboard bridge + answer poller (watermark consumers, cmd/tq/main.go:597-607,986-996) | `Bus`/`Publisher` consumers — only once tq facts are engine facts | **gated on S2** — until then they stay watermark actors |
| harvest/prune sweeps, once-drain, discovery-watch (cmd/tq/main.go:619,1455,1506) | — (process-local one-shots) | **no-fit** — `runactor` actors |
| process supervision: first-exit-cancels-group, LIFO teardown, `InterruptOn` 2nd-signal-exit-130 (internal/runactor/runactor.go:1-9) | — | **no-fit by design** — runactor survives INSIDE the composition root; system/ wraps it, never replaces it (ADR-0019 wording: "adopting the lifecycle/checkpoint machinery where it maps 1:1") |

Net: S4 is NOT a big-bang rewrite of the pool into deciders/commands. It
is (a) the S3 flip carrying the read side, (b) sweeper ticks moved onto
`ManageTimers` where that is strictly the same semantics, (c) everything
else staying runactor actors under a composition root that owns engines,
checkpoints, and close ordering.

## 3. The S2 gate — why composition-before-S2 is decorative

`system.New` stands up ITS OWN event journal over the deployment engines
(system/system.go:111 `System` struct + engineSlice). tq's fact journal
only becomes that journal at S2 (consumers re-pointed onto
`facts.Fact`). Composing today would mean two journals — the system's
metaengine journal beside the queue's fact journal — the exact split
brain ADR-0019 exists to kill. The readmodel's separate sqlite file is
fine (it is a disposable fold projection, model.go doc comment), but the
composition ROOT must not mint a second source of truth. Therefore S4
execution is serialized: S1 flip (landed) → S2 consumer re-pointing → S3
flip → S4.

## 4. Open questions the S4 execution window must settle (defaults included)

a. **TimerStore engine home.** `ManageTimers` needs `sys.TimerEngine()` —
   a metaengine engine. The projection DB is disposable/rebuildable, so
   durable sweeper-cadence state must not live there. Default: either the
   deployment declares a durable engine for timer/checkpoint collections
   (`DeploymentConfig` engine set), or sweeper ticking stays in runactor
   and `Timers` is adopted only for genuinely timer-shaped new features.
   Decide against the post-S2 journal shape; do not pre-build.
b. **Watcher checkpoint home.** `system_checkpoints` collection (default)
   vs the queue `watermarks` table. Default: `system_checkpoints` — the
   readmodel projection is rebuildable, checkpoint loss costs a rebuild,
   and reusing the queue watermarks table would couple a disposable
   projection to queue-store durability.
c. **GracefulClose vs runactor teardown ordering.** Where the S3-flipped
   serve process (cmd/tq/main.go:3020 http actor) already runs inside a
   system-rooted composition, `GracefulClose` should own the close
   ordering; the agent-pool keeps runactor's first-exit + interrupt
   semantics. Adopt per surface, not globally.
d. **Require/module home for `system/v4`.** The composition wiring lives
   in cmd/tq (the runactor groups), which is its OWN replace-free module
   (ADR-0017), while ADR-0011 names the root module the app layer. Either
   extract a root-module internal composition package (root carries the
   require) or cmd/tq carries it (hand-pinned indirects convention
   applies). See §5; decide at execution time.

## 5. Module/facade impact (bookkeeping for the window)

- OPEN QUESTION where the `system/v4` require lands: the composition
  wiring lives in cmd/tq (runactor groups, cmd/tq/main.go:586-637,
  968-996, 1455-1539, 3020), which is its OWN replace-free module
  (ADR-0017), while ADR-0011 names the root module the app layer. Either
  the composition is extracted into a root-module internal package (root
  go.mod carries the require) or cmd/tq carries it (its hand-pinned
  indirects convention applies). Upstream tag verified 2026-09-26:
  `system/v4.9.0` is the latest system tag. Facades are unaffected
  either way (ADR-0016's backend-layer-only rule).
- After the go.mod change, the gates that bite depend on the module
  chosen: root → `go mod vendor` (vendor-resolved internal imports,
  AGENTS.md vendor hazard) + nix vendorHash fast gate; cmd/tq → the
  devmod shim's FOD. `scripts/check-go-mods.sh` covers both.
- No new module: `scripts/new-module.sh` is not needed for S4.

## 6. Execution sketch for the S4 window (after S2 + S3 flip land)

1. Re-read this doc against the then-HEAD (S2/S3 will have moved the
   ground); re-verify every §1/§2 citation.
2. Land the serve half exactly as the S3 flip shaped it (§2 row 1-2); no
   separate composition work if S3 already rooted it.
3. Agent-pool: wrap pool construction in `system.New` with an empty-ish
   `DomainConfig` (no Commands/Queries), `Timers` per §4a's ruling, and
   `runactor` actors handed their contexts as today; `GracefulClose`
   ordering per §4c. The tq `queue.Store` stays injected — S4 does NOT
   move store selection into DeploymentConfig (that is S1's thin-driver
   seam and the owner-run cutover's business).
4. Gates: root build/vet/test -race, all-module loop, webui + httpapi
   smokes, `TestRoutesAreReadOnly` + health-CSP pins, check-go-mods +
   vendorHash fast gate, lint-baseline `--check` (clean cache).
5. Art-dupl `-t 4` recount (the deletion-dividend metric, item 49 of
   report 2026-09-25_12-16) and the ADR-0019/FEATURES/CHANGELOG doc
   pass; then the row's residue is only the owner-run dogfood cutover.
