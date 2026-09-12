# Priority System Execution — Status Report (Phase 0–5 of 6)

**Session:** 2026-09-12 ~14:30–15:45 (post-G0 execution window)
**Plan:** `docs/planning/2026-09-12_14-19_PRIORITY-SYSTEM-SUPERB-PARETO-EXECUTION-PLAN.md`
**Spec:** ADR-0015 (`docs/adr/0015-priority-bands-importance-aging.md`)
**Reporter:** Crush (glm-5.3), executing the 40-task Pareto plan on the
owner's G0 "GET SHIT DONE" order.

Provenance tags: [v] = personally verified by gate/test run this session;
[c] = carried from the 14-03 research report; [a] = sub-agent claim, not
re-verified.

## a) FULLY DONE (verified green this session)

**Phase 0 — capture & verify (T01–T05)** [v]

- T01: every ATP claim spot-verified at source — weights sum exactly 1.0
  (`pkg/core/priority/score.go:71`), flagship DB-bridge stub confirmed
  (`next_task_issue_conversion.go:101`), cache is content-hash keyed with
  `DefaultCacheTTL = 24h` (`pkg/constants/defaults.go:120`), "dedupe" is
  go/ast code dedup. **ATP + project-meta LICENSEs are PROPRIETARY** —
  porting rules written into ADR-0015 §7 (rewrite-from-spec only).
  Correction folded in: `pkg/models/priority.go` DOES exist.
- T02: **ADR-0015 written and committed** — band ladder (backlog 0–99 /
  hot 100–149 / machine 150+), marker grammar (`— P[1-4]` → 90/70/50/30),
  precedence (hot > marker > AI > keyword > importance > flat), aging keyed
  on `created_at` with evidence, migration story, rejected alternatives,
  porting/licensing rules, risk register.
- T03: `docs/DOMAIN_LANGUAGE.md` gained the Prioritization section (8
  terms: Importance, Marker, Item score, Band, Working set, Aging, Unblock
  bump, Score cache).
- T04: coverage survey — 240/261 `~/projects` repos carry
  `.config/metadata.yaml`; ALL 6 pool repos have importance (SystemNix 90,
  CV 55, daemon 55, overview 55, tq 50, sdk 50). Pilot needs zero setup.
- T05: grammar-compat check — `— BLOCKED:` suffix convention and
  check-todo-list.sh unaffected; marker decision recorded in ADR-0015 §2.

**Phase 1 — aging, the 1% that delivers 51% (T06–T08)** [v]

- T06: sqlite `ClaimDue` orders by
  `priority + MIN(age_ms/86400000/PriorityAgingDaysPerPoint, PriorityAgingMaxBonus)`
  — constants (`3` days/point, cap `10`) defined ONCE in the queue
  contract (`internal/queue/queue.go`), referenced by both backends.
  Failing-test-first; flip + cap tests green. Aging is scheduling, not
  state: stored priority never changes.
- T07: postgres mirror (`LEAST`) + conformance subtests run against a REAL
  scratch postgres cluster (`initdb` on :55432, `TQ_TEST_POSTGRES`):
  full battery green.
- **Fixed a pre-existing MASTER CI failure** while there: the dismiss-dead
  conformance subtest leaked a pending task, breaking cooperative-cancel +
  heartbeat subtests on every run since that subtest landed (red at HEAD
  `c3f9779`, reproduced by reverting my changes). Root cause: equal-
  priority older leftover claims first. Fix: the subtest now cancels its
  rejected-dismiss leftover (`conformance_test.go`).
- T08: interaction tests — aging × NotBefore (gated stays gated), aging ×
  requeue (keyed on `created_at`, requeue never resets), per-window accrual
  (2 windows flip a 1-point gap, 3 outrank 2). Race clean.

**Phase 2 — enqueue composite (T10–T16)** [v]

- T10: `runRepo` extracted into `surveyRepo` / `itemDenial` /
  `admitItem` (+ later `occupancyDenial`) — behavior-identical (one
  subtle pacing case caught in self-review: dedup-race returns still count
  against one-per-run), gocognit findings zero.
- T11: `harvest.ReadImportance` — strict-subset YAML reader (top-level
  `importance:` only, quoted scalars tolerated, 0–100 enforced, malformed
  ⇒ repo skip reason, absent ⇒ 50). Unit + fuzz green (15s campaign).
- T12: `SplitMarker` + **strip-before-hash in `ParseRepoAll`** — Item.Text,
  Item.Key and the payload all carry marker-free text; `MarkerLevel`
  rides the Item. Hash-stability test pins: marker edits NEVER fork tasks,
  real rewords still re-arm. Fuzz found and fixed a real edge (marker-only
  line stripped to empty) plus tightened the progress invariant. 15s
  campaign green.
- T13: `KeywordBump` (security/CVE +30, critical/urgent/asap +25,
  production/breaking/outage +20, strongest wins, no stacking) —
  rewritten from spec, ATP credited in prose only.
- T14: `internal/queue/bands.go` — Band type, boundaries, `BandOf`,
  `ClampBacklog`, both-direction tests, and an ADR-invariant guard test
  (aging cap < marker gap 20).
- T15: harvest wiring — `Config.UseImportance`, importance read once per
  repo in surveyRepo (malformed ⇒ all items skipped with reason),
  `enqueue` resolves via `ResolvePriority` (hot > marker > cached AI score
  > importance+keyword > flat). Legacy mode bit-identical (pinned by
  test). E2E verified with a scratch repo + scratch DB: `— P1` ⇒ priority
  90, stripped text in prompt/payload.
- T16: flags — `--priority-from importance` on BOTH `tq harvest` and `tq
  agent-pool` (validated, default empty = legacy).

**Phase 3 — mutable priority (T17–T22)** [v]

- T17: `journal.Reprioritized` fact type + `queue.ReprioritizeEvidence`
  {old, new, source, reason}. No consumer switches needed (all fact
  switches ignore unknowns by design).
- T18: `Store.UpdatePendingPriority` on the CONTRACT + both backends —
  PENDING-only (`ErrInvalidTransition` otherwise), same-value ⇒ no fact
  (idempotent), `RowsAffected` re-check, evidence fact in the SAME tx.
- T19: mirrored conformance subtest (live cluster green) +
  `harvest.RepriMutable` band protection (hot/machine never rewritten by
  sweeps) with both-direction tests.
- T20: **`tq reprioritize`** CLI — repo scan, dry-run, value-idempotent,
  JSON mode. E2E: editing `— P1` → `— P4: cooled` moved the pending task
  90→30 with exactly one journal fact; second run = 0 changes.
- T21: agent-pool pre-actor sweep (prune-stale slot, default ON,
  `--reprioritize=false` to skip) — idempotency proven by the sweep tests
  (second run asserts zero changes). Budget note: this sweep is pure
  local SQL, no agent spend, so no budget gate needed (the plan's
  budget-guard concern applies to the T27 AI scorer, which IS gated).
- T22: cqa bridge migrated 80 → **150** (machine band) + test updated.
  **Live-journal migration is a NO-OP**: read-only enumeration of
  `/mnt/pool/services/tq/tq.db` (2026-09-12 ~15:35) = 113 tasks, ALL
  priority 0, zero pending, zero 80s anywhere. Nothing to migrate; the
  code change covers future mints.

**Phase 4 — working set (T23)** [v]

- T23: `Config.MaxPendingPerRepo` (0 = off = legacy). Design correction
  mid-implementation: the legacy any-pending-is-busy rule SHADOWED the
  cap, so the knob (when set) REPLACES the legacy rule — running always
  denies (one agent executing per repo), pending fills to cap,
  completion frees the slot. Tests: admit-below-cap, deny-at-cap with
  reason, un-starve after completion. CLI flags on both harvest and
  agent-pool. (T24's docs/perf-assertion pieces folded into T37.)

**Phase 5 — cache + scorer contract (T25–T26)** [v]

- T25: `priority_scores` table in BOTH backends (schema + upsert/read
  via new contract methods `SavePriorityScore`/`PriorityScore`), live
  conformance subtest green, and the harvest enqueue now feeds cached
  scores into the resolver (`AIScore`). Key derivation shared by
  construction: the cache is keyed by the SAME `harvest.ItemKey` the
  queue dedup uses.
- T26: `internal/executor/prioritize.go` — `TaskTypePrioritize`,
  `PrioritizePayload` (repo + batch items + autonomy knobs),
  `PrioritizeResult`, closeout-free executor on the review pattern,
  batch-scoring prompt with rubric + strict `TQ_RESULT` contract, and
  `ParsePrioritizeResult` (every item exactly once, 0–100 range, no
  unknowns/duplicates). Contract + prompt tests green in the executor
  module. Registered in `registerAgentExecutors` (carry parity with
  review/status/dlqfix).

## b) PARTIALLY DONE

- **T09** (aging docs): DOMAIN_LANGUAGE entry done; the AGENTS.md
  paragraph + webui effective-rank hint deliberately folded into T32/T37
  (one template pass, one AGENTS pass).
- **T24** (working-set docs + perf assertion): admission knob DONE;
  docs + the structural perf note folded into T37 (the sweep is
  O(repos × (parse + one filtered List)) — bounded by the working set,
  not the warehouse).

## c) NOT STARTED

- **T27** prioritize sweeper: mint per-repo batch scorer tasks (dedup
  `prioritize:<repo>:<key-hash>`), apply verdicts to the score cache +
  as repri facts, budget-gated. (Its executor, payload and contract —
  the hard parts — already exist from T26.)
- **T28** AI cost measurement + cadence recommendation (needs a real
  scorer run; T40-gated in practice).
- **T29** `tq show` score provenance section.
- **T30** unblock bump (dependents priority bump when a dep completes).
- **T31** effort-aware claims under low budget (G2-gated, default flat).
- **T32** webui bands (badge, importance column, filters; templ +
  webui-css regen; carries T09's hint).
- **T33** starvation alarm → PapDashboard pattern.
- **T34** score feedback loop (review verdicts × score source
  calibration report).
- **T35** paused-repo rule (importance=none ⇒ no auto-admission) — note
  the reader today treats missing/0-file importance as default 50;
  the `none(0)` explicit pause rule is a small T23-style denial.
- **T36** status-report prompt band-drift section + session-close
  parity.
- **T37** docs sweep (CHANGELOG, FEATURES, README, SECURITY, AGENTS.md
  conventions + payload contracts; carries T09/T24).
- **T38** lint-baseline `--check` + full ci-local. My NEW-code lint
  findings were fixed on the spot (gci/wsl/golines via --fix + manual
  mnd/varnamelen/err113/cyclop cleanups; priority.go lints clean); the
  full 3-minute baseline gate re-run is still pending.
- **T39** minting the G0-approved follow-ups into TODO_LIST.md (only
  after T02 — done — but deliberately held until execution pauses so
  the pool doesn't start spending on mid-flight work).
- **T40** dogfood pilot: SystemNix flag proposal + first-window watch.

## d) TOTALLY FUCKED UP (this session) — caught, fixed, lessons

1. **Two destructive edit-tool mishaps**: an edit intended to INSERT
   before `TestNotBeforeDelays` instead replaced its body with the
   signature; the same pattern hit sqlite `ListWatermarks` (rows query
   deleted). Both caught immediately by build/grep and repaired. Lesson
   applied mid-session: for insertions use append/python-anchored
   replacement, never a "signature-only" old_string; view before edit.
2. **Admission-control design flaw caught red-handed by my own test**:
   the first MaxPendingPerRepo implementation was DEAD CODE — the
   legacy `busy` rule (any pending) shadowed the cap at every denial.
   Redesigned as `occupancyDenial` (knob replaces legacy rule; running
   always denies; pending fills to cap). The test that exposed it is
   now the pinned spec.
3. **Fuzz did its job twice**: marker-only lines (`" —P1"`) stripped to
   empty text (fixed: marker-only = not a marker), and my initial
   "no second marker" invariant was wrong for degenerate multi-marker
   lines (replaced with a strict-prefix progress invariant).
4. **A wrong claim I made earlier this session, corrected**: I reported
   the lint baseline "missing 9 modules". Wrong — those packages
   (harvest, bridge, …) are ROOT-module packages; the baseline's `root`
   row covers them. The baseline is complete; the gate failure was
   purely my new findings (since fixed).
5. **Master CI was already red at HEAD when I started** (both
   `test-postgres` and `test`): the postgres battery bug I fixed in a),
   and a release-gate failure because `internal/journal/cqrs/v0.2.0`
   exists as a SIGNED LOCAL TAG but was never pushed while root go.mod
   requires v0.2.0. Local release-gates smoke is green (tag exists
   locally); CI cannot be. NOT MINE TO PUSH — owner action required
   (see §g).

## e) IMPROVEMENTS (delivered beyond the letter of the plan)

- Pre-existing master breakage fixed (dismiss-dead conformance leak)
  — CI's test-postgres goes green on the next push.
- Aging constants defined ONCE in the contract package — zero backend
  drift possible, plus a guard test pinning cap < marker gap.
- `Item.MarkerLevel` is `omitempty` — all legacy JSON consumers and the
  drift-golden fixtures stay byte-stable.
- The scratch-postgres conformance harness (initdb on :55432 +
  TQ_TEST_POSTGRES) made LIVE dual-backend verification possible
  mid-session instead of CI-only. Cluster still running for T38's
  final gates (kill: `pg_ctl -D /tmp/tq-pg-* stop`).

## f) The next 50 things, in execution order

1. T27 sweeper: `internal/prioritize` — cursor (head-bootstrapped),
   per-repo batch minting with dedup `prioritize:<repo>:<hash>`, verdict
   application (SavePriorityScore + UpdatePendingPriority via
   RepriMutable), budget gate class, tests.
2. T27 wiring: `--prioritize-every` style pool flag; AGENTS payload-
   contract entry.
3. T29: `tq show` provenance (score source, scored_at, reasoning from
   the cache + last repri fact) + test.
4. T30: unblock bump — deps.task_id index check (EXPLAIN QUERY PLAN),
   dependents-count query, bump at completion time in the same tx as
   task.completed? (No — completion is worker-side; bump via a fact-
   consumer or at repri sweep; design note needed) + tests.
5. T32: webui band badge + effective-rank hint + `--band` filter;
   `templ generate` + `nix run .#webui-css` + adoption-table rows if
   components used.
6. T33: starvation alarm (oldest pending below band threshold) →
   NotifyDeadPool pattern.
7. T35: paused-repo rule (importance=none(0) ⇒ admission denial) + test.
8. T36: status prompt band-drift section + session-close band summary.
9. T31: effort-aware claims (G2 default: off — only implement the
   plumbing behind a flag; document).
10. T34: score feedback loop — tag review verdicts with score source,
    calibration report (`tq` read-side join + report).
11. T37 docs sweep: AGENTS.md (priority contracts: markers, bands,
    aging, repri, admission, score cache, `tq reprioritize`), README
    feature list, FEATURES.md rows, CHANGELOG entry (all phases),
    SECURITY.md (no new surfaces), DOMAIN_LANGUAGE cross-refs, harvest
    docs example for `--priority-from`.
12. T24 leftover: working-set docs section + repri-scan perf note.
13. T38: `scripts/lint-baseline.sh --check` full run; fix or
    policy-owned regen; then FULL `./scripts/ci-local.sh` (CI_CHECK
    state depends on §g answer).
14. T39: mint G0-approved follow-ups into TODO_LIST.md (unchecked,
    machine-parseable, agent-executable; check-todo-list gate).
15. T40: SystemNix proposal (aging is unconditional — nothing to enable
    there; enable `--priority-from importance`, `--reprioritize`
    already default-on, propose `--max-pending-per-repo` value ~3-5,
    same-session 120 recommendation), then first-window watch report.
16. Owner: push `internal/journal/cqrs/v0.2.0` + master (CI red fix).
17. Band-migration verify step for the live journal at pilot time
    (re-enumerate; expect still no 80s).
18. Nightly fuzz will exercise SplitMarker/ReadImportance seeds — watch
    the first nightly after merge.
19. Post-pilot: calibrate keyword table + marker ladder from real claim
    order (the review-driven calibration from T34 feeds this).
20. Post-pilot: revisit G1 (2000/50) — admission cap sizing from
    observed drain rate.
21-50. The out-of-scope list from the plan stays out (per-repo budget
    scaling, cross-project dedup, cross-repo dep inference, project-meta
    tags in formula) unless the owner re-opens them; the remaining
    ~30 slots are the L0 micro-steps of items 1-15 above, each ≤12 min,
    fully enumerated in the plan's §5.

## g) Up to 3 questions I cannot figure out myself

1. **Push authorization**: master CI is red at HEAD partly because
   `internal/journal/cqrs/v0.2.0` (signed local tag, cut by an earlier
   release flow) is unpushed while root go.mod requires it. The daemon
   never pushes and pushes are owner-run. Order me to push
   (`git push origin master internal/journal/cqrs/v0.2.0`), or push it
   yourself — until then `test`/`test-postgres` CI jobs stay red
   regardless of local state.
2. **Dogfood pilot timing (T40)**: enable `--priority-from importance`
   on the SystemNix pool NOW (aging + markers + repri sweep are already
   live in code once deployed) and watch the first window, or wait
   until T27–T38 land so the AI pass ships with the pilot? Deploying
   now gets real claim-order data sooner; deploying once means one
   owner-run input flip instead of two.
3. **G2/G3 flip or confirm** (plan defaults are encoded and silent
   acceptance means they stand): budget stays global-flat and importance
   is ordering-only (G2); the scorer reuses each repo's `.crushrc` model
   (GLM-5.3-Flash today) with cost measured before any cadence (G3).
   Both are money decisions — say the word if either should flip.

## Verification appendix (claims carry citations)

- Root: `go build ./... && go vet ./...` green 15:43 [v];
  `go test ./cmd/... -count=1` green (post MarkerLevel omitempty) [v].
- harvest module: full suite + race green 15:43 [v]; fuzz campaigns
  (SplitMarker, ReadImportance) 15s each PASS [v].
- executor module: full suite green 15:43 [v] (prioritize included).
- queue/sqlite: full suite green [v] (aging flip/cap/interactions,
  UpdatePendingPriority battery, PriorityScore roundtrip).
- queue/postgres: FULL battery green against live scratch cluster
  (`host=/tmp port=55432`) [v] — includes the fixed pre-existing
  failures + aging + repri + score-cache subtests.
- queue contract module: green [v].
- bridge (cqa 150): green [v].
- E2E CLI proofs: marker→90 harvest; reprioritize 90→30 with exactly 1
  fact; dry-run mutates nothing; scratch DBs only (`TQ_DB` to /tmp) [v].
- Live journal enumeration (read-only): 113 tasks, all priority 0,
  zero pending [v].
- NOT yet run: full ci-local (T38), lint-baseline --check full pass,
  nix build, webui smokes (webui untouched so far).
