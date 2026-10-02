# Queue-Health Restoration — Phase 1 Root Causes Landed (M6–M9) + M10 In Flight

> **DUPLICATE — ARCHIVED 2026-10-02** Canonical report: `2026-10-01_17-05_queue-health-restoration-plan-closeout.md`. remaining M10-M21 executed by phase 2 and the 17-05 consolidating closeout


**Date:** 2026-10-01 11:37 CEST · **Session:** resumed 11:05 at M6/F20 per
the 11-01 close-out · **Plan:**
`docs/planning/2026-10-01_04-27_SUPERB-QUEUE-HEALTH-RESTORATION.md`
(rulings R1=park/cancel, R2=granted, R3=(a) N=3) · **Scope of this
report:** M6, M7, M8, M9 fully landed and gated; M10 ≈60% implemented
(uncommitted); M11–M21 + F70 untouched. Committed at `a0982d99`.

## a) Plan vs. done (checkable)

| Task                                     | Status             | Evidence                                                                                                                                                                                                                                                                                                                                                            |
| ---------------------------------------- | ------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| M6 scoped gofmt gate (F18–F24)           | **DONE**           | `be28c964` — `executor.ScopedGofmtStage` const wired into the `defaultVerify` mint (internal/executor/agent.go); e2e `TestDefaultVerifyScopedGofmtStage` (green with gitignored unformatted vendor/ present, RED on untracked non-ignored unformatted file, stays a real death); repo `.tq-verify` switched to the scoped idiom and smoke-probed green at repo root |
| M7 breaker design memo (F25–F27)         | **DONE**           | `0b33def0` — `docs/planning/2026-10-01_11-19_env-requeue-circuit-breaker.md`: call-site map w/ citations, counter-persistence ruling (in-memory), R3(a) N=3 design                                                                                                                                                                                                  |
| M8 env-requeue circuit breaker (F28–F34) | **DONE**           | `d220897b` — see §b for the root-cause find; e2e `TestEnvRequeueStreakBurnsAttempt` (3 gate refusals → attempt burned, `env-streak` code on the fact, heals afterwards), `TestEnvRequeueBreakerDisabled` (<0 disables), `TestStaysOnLadder` table                                                                                                                   |
| M9 requeue_class fact field (F35–F38)    | **DONE**           | `a0982d99` — `queue.RequeueEvidence.Class` + class consts ride every task.requeued fact; conform `Caps.RequeueClass` + `TestRequeueFactCarriesClass` verified **PASS (sqlitev4) / SKIP-divergence (cqrsqlite)** with `-run TestStoreConformance/TestRequeueFactCarriesClass -v`                                                                                     |
| M10 claims-per-task anomaly (F39–F42)    | **IN FLIGHT ≈60%** | DONE+uncommitted: `queue.ClaimAnomalyThreshold=20` + `queue.ClaimCount` helper (+ facade re-exports), webui `DashboardData.LoopSuspects` + `loopSuspects()` + `loadSnapshot` wiring. REMAINING: fragments.templ chip + `templ generate` regen, cmd/tq stats suspects section + JSON field, stats/webui tests, gates, atomic commit                                  |
| M11–M21, F70                             | NOT STARTED        | —                                                                                                                                                                                                                                                                                                                                                                   |

Gates re-run at each landing: executor/worker/queue(+companion, sqlitev4,
postgresv4, cqrsqlite) module gates `GOWORK=off build+vet+test`, cmd/tq
devmod gate, root `go build && go vet && go test ./... -race -count=1`
(**rc captured to file, 17/17 ok** at M9 close),
`check-facade-parity.sh` (7 facades OK), scoped-gofmt leg green.

## b) What the work ACTUALLY was (truth > plan)

- **P1's exact cadence was a LADDER RESET BUG, not just a missing
  breaker.** `preflightReset` (worker.go:363-366 pre-fix) ran for ANY
  non-preflight outcome — including the gate requeue itself. Sequence:
  claim → gate fails → ladder count=1 → requeue → reclaim → **reset
  (gate error ≠ PreflightError)** → count=1 again → the gate class
  bounced at the base rung (2m ±20% ≈ 96–144 s) FOREVER and the 15 m
  cap was dead code for it. This matches the diagnosis's measured
  ~100–130 s cadence on the 164-claim CV loop exactly. M8's reset-rule
  rewrite (`staysOnLadder`) fixes escalation AND makes "consecutive"
  true; the breaker (N=3 → `store.Fail` burn with `ExpBackoff`
  escalation + `env-streak` reason code) bounds the class: worst case
  ≤2 free requeues between burned attempts, ≤9 claims on a 3-attempt
  task, then DLQ.
- **M6 went beyond the plan**: besides the mint, `StaleVerifyReasons`
  learned the unscoped-gofmt shape (predicate 4), so `tq doctor
  --hygiene` now content-flags any pinned gate still carrying the
  poisoned `test -z "$(gofmt -l .)"` stage — the exact P2 detector M11
  was scoped to be, at the pin level. M11's remaining delta (probe the
  live tree for gitignored unformatted files) is still worth landing.
- **cmd/tq is replace-free against proxy v0.3.0** (ADR-0017): the
  committed go.mod resolves `internal/*` from published tags, so
  NON-TEST cmd/tq code may only use v0.3.0-published symbols. Verified
  `List`/`FactsForTask`/`CountFacts` exist @v0.3.0 (module cache).
  Consequence: M10's threshold gets a canonical const in internal/queue
  (usable by webui via the root replace graph) + a local unexported
  twin in cmd/tq main.go, equivalence-pinned by a devmod-compiled test
  until the next release re-pins. My first M9 attempt referenced the
  new queue consts from cmd/tq/webui TEST files without importing the
  package — `undefined: queue` from vet, fixed with literals; lesson
  recorded in §d.
- **M10 overlap check (plan requirement)**: the parallel 07-30 session
  (report `2026-10-01_07-30_todo-sweep-…md`, commit `141c80f9`) landed
  doctor verify-pins verdict merge + hygiene summarize + parked
  surfaces + `tq tasks --verify-contains 'gofmt -l'` — NO
  claims-per-task work, NO gitignored-gofmt env probe. M10/M11 are
  clear to build as planned.

## c) Deviations from the plan

1. **F32 (conform-suite pin "env class never loops > N") was landed as
   a worker-level e2e pin** (`TestEnvRequeueStreakBurnsAttempt`) — the
   conform suite is store-scoped and the breaker is worker behavior;
   the loop bound is pinned where the behavior lives.
2. **F23's `go mod vendor` step was skipped deliberately** — vendor/
   stays trashed per M2; the root gate ran module-cache (17/17 ok).
3. **Daemon ferry recoveries ×3** (M6 amend, M8 fold, M9 fold of two
   ferries): auto-commit daemon swept uncommitted edits during long
   gates; each recovery verified content-identity and
   local-only-ness before `--amend`/`reset --soft`. The M10 WIP files
   (internal/queue/queue.go, queue/queue.go,
   internal/webui/render.go) are uncommitted as I write — the daemon
   will ferry them; next session folds (see §f item 2).

## d) Knowledge captured (what past-me didn't know)

- **cmd/tq module graph**: dev builds use the devmod replace shim
  (scripts/test-cmd-tq.sh); the committed replace-free go.mod is what
  the hermetic nix FOD resolves — new internal symbols must NOT be
  referenced from cmd/tq NON-TEST code until a release bumps the pins.
- **Parity guard checks symbol KIND** — a const re-exported as facade
  `var` fails (`KIND-MISMATCH`); consts go in facade const blocks.
- **Edit-tool whitespace-equivalent matches can mangle brace
  structure** — my conform-test insertion landed INSIDE the previous
  function (re-indented to "match style"). Anchor insertions at
  function boundaries or append at EOF; compile immediately after.
- **The scoped gofmt leg is the cheap first gate** — run
  `test -z "$(gofmt -l . | git check-ignore --stdin -v --non-matching
  | grep '^::')"` right after EVERY edit batch, not just at battery
  time (M9's queue.go needed a late `gofmt -w`).
- **Conform tests self-register** in `conformanceTests`
  (conform/suite.go) — a new conform test that is only DEFINED but not
  registered silently never runs; verify with
  `-run 'TestStoreConformance/<Name>' -v` (wrapper name is
  TestStoreConformance, not TestStoreSuite).

## e) Close-out of the day's window (proofs)

- `git log` this window: `be28c964` (M6) → `0b33def0` (M7 memo) →
  `d220897b` (M8) → `a0982d99` (M9); all gates green per §a; race
  battery rc=0 captured (`/tmp/m9-root.log`, 17 `^ok`, no failures).
- Live verification of the M9 pin: PASS (sqlitev4) / SKIP (cqrsqlite,
  S1 divergence) as designed.
- Live verification of M6: scoped leg green at repo root INCLUDING all
  M6–M9 edits (self-hosting: the repo's own .tq-verify runs the new
  idiom).

## f) Top-N follow-ups (ranked)

1. **Finish M10** (remaining steps in §a): templ chip + `templ
   generate` from REPO ROOT (binary at /run/current-system/sw/bin/templ;
   NO css change needed — marker class only, existing chips
   card-parked/card-sessions are unstyled hooks), cmd/tq stats
   suspects section + `claim_anomalies` JSON + threshold-twin pin
   test, webui chip render test (seed 21 claims via claim+requeue
   loop, assert chip), module gates, atomic commit (fold ferry).
2. **Fold the M10 WIP ferry** (3 files, §c3) into the M10 commit.
3. **M11** doctor env probe: `gofmt -l` ∩ non-ignored untracked/tracked
   files on the LIVE tree (the stale-pin half is already covered by M6's
   predicate 4 — cite that to avoid duplicate reporting).
4. **M12** dlqfix default-on memo + guard test (deploy still
   owner-gated, §g-1).
5. **M13** PapDashboard alert-path verification (176-dead window).
6. **M14** loop-detector smoke (fixture journal with a 30-claim task →
   red; healthy → green; wire into ci-local smoke list).
7. **M15/M16** CV dead census + budget-burn model (read-only
   python3-sqlite forensics, TQ_DB prod journal READ-ONLY).
8. **M17** CV cross-post + CV TODO row for their red gate.
9. **M18** archive evidence + final close-out (record the §c deviations
   and the F32 worker-pin decision there).
10. **M19 TODO harvest** — still gated on "M2+M6 verified" (now true:
    vendor/ absent AND mint scoped) — the grinder guard can lift after
    one green dispatched probe through the new gate.
11. **M20** ask park→resume e2e; **M21** AGENTS.md lessons (env-requeue
    class, gate hygiene, the §d items) within the 15,000 B budget;
    **F70** full `ci-local.sh` battery + M1 no-new-claims re-check.
12. Perf follow-up (non-blocking): the webui loop-suspects census is
    O(tasks) fact walks per snapshot render; if SSE churn shows on the
    live dashboard, add a claim-count column or a single GROUP BY
    store method (interface churn: 4 backends + conform + cqrsqlite
    divergence) instead.
13. `internal/harvest TestSelfManagingLoop` flake ("tick 2
    enqueued=[]", unreproduced 3/3 isolated) — still open from the
    prior session's §f; not touched this window.

## g) Blockers, questions, or decisions for the owner (STOPPED here)

Still the three unanswered questions from the 11-01 close-out, none
blocking M10–M21 code work:

1. **Deploy timing** for SystemNix `dlq-fix = "true"` (landed
   `c180144e`, needs `scripts/deploy.sh`; sudo/systemctl banned in
   agent shells) — until then 167 gofmt-class dead letters wait for
   autopsies.
2. **Looper task `000001a0f539…`** disposition (cancel vs let cycle).
3. **Push posture** — this repo is ~27 commits ahead of origin/master
   (never pushing without explicit request); CV, SystemNix,
   go-cqrs-lite also unpushed.

Waiting for direction before resuming the plan.
