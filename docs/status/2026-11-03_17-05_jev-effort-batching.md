# Session status — effort-based batch sizing + optional Jev (TypeSafe) estimates

Owner directive (verbatim): "I feel like we should batch based on estimated
effort; How could we integrate jev from TypeSafe AI to be smarter (optinal)?"
Owner session — no task ID, no Task-Queue-ID footer owed. Directly extends the
2026-11-03 section-batching session (same day, prior report
`2026-11-03_15-59_section-batching-change-self-review.md`).

## a) FULLY DONE

All committed on local master (never pushed; branch ~160 ahead of origin):

1. **Effort-aware batch fold** (`0900aefb`, internal/harvest/harvest.go):
   `sectionRuns(items, admissible, maxItems, budgetMinutes, effortOf)` — a run
   now stops at THREE walls: section boundary, context cap, and the summed
   member-effort budget. A single item whose estimate alone exceeds the budget
   still ships solo (every admissible item joins exactly one run). Derived
   budget (batchItems x DefaultBatchTimeoutMinutes) under FLAT estimates
   reproduces the count-cap grouping exactly — pinned by
   `TestSectionRunsFoldsByEffortBudget` (flat-equivalence case).
2. **Estimator seam** (`0900aefb`): `harvest.Config.EffortEstimator`
   (interface, once per repo over the admissible candidates) +
   `Config.BatchEffortMinutes` (0 = derive). Missing keys/non-positive values
   fall back to the flat 30m default — estimation trouble can never drop or
   stall work. Payload timeout = **max(count ceiling, effort sum)** — estimates
   can only RAISE timeouts; the count floor makes under-estimation
   non-truncating (the 2026-11-03 worst finding cannot regress:
   `TestBatchEffortNeverShrinksTimeout`). Payloads record per-member
   `effortMinutes` (additive, omitempty — OKF v0.2 §11) as the durable record
   for the later estimate-vs-outcome audit (`TestBatchSizesRunsByEffort`).
3. **Jev (TypeSafe) estimator** (`18a2862e`, moved `3c477fab` →
   internal/harvest/effort.go): one System One Score question per item over a
   5-level effort rubric (situations, not relative sizes); code owns the math —
   piecewise-linear rubric→minutes interpolation (10/25/60/150/240), 5-minute
   round-UP, clamp [10,240]. State = repo name + purpose
   (RepoPurpose, exported this session) + per-item text/section under stable
   `todos.<name>` paths; questions name their subject by path. Requests chunk
   at 20 items. Estimates cache in `<db>.effort.json` keyed by ITEM KEY (an
   edit re-keys → re-estimates) and stamped `jev:<model>` (a model swap
   invalidates — the prioritize scorerSource lesson); 30d load-prune. ANY API
   failure logs one WARN and degrades to flat for that chunk — an error is
   never converted into a fabricated "small" answer. Hermetic httptest
   coverage: request shape, cache-hit-no-re-ask, chunking, fallback,
   model swap, prune roundtrip (internal/harvest/effort_test.go).
4. **Pool wiring** (`25b1ac5a`, cmd/tq/agentpool.go): `--jev` (default OFF;
   missing key fails startup loudly), `--jev-model` ($TQ_JEV_MODEL),
   `--jev-api-key` ($TYPESAFE_API_KEY), `--effort-budget` minutes
   (0 = batch-items x 30m). Estimator cache = `<db>.effort.json`; startup
   banner prints mode + cache + budget + fallback. Unset `--task-timeout`
   ceiling now also covers the 240m estimate clamp and any explicit budget
   while `--jev` is on; an explicit pin stays authoritative
   (`TestParseAgentPoolOptionsJev`). `batchTimeoutPerItem` single-sourced from
   `harvest.DefaultBatchTimeoutMinutes` (was the 2nd of 3 copies; executor's
   own default remains — separate module/release cadence).
5. **Docs** (`495ed0ca`): AGENTS batched-harvest bullet (effort budget fold +
   `--jev` raise-only rule; file 25,395/25,400 B — under guard), FEATURES row
   77, DOMAIN_LANGUAGE Batch row + new **Effort estimate** term, CHANGELOG
   [Unreleased] Added row (default-off + flat-equivalence stated),
   problem-doc status paragraph. TODO_LIST row 506 DONE-note updated (both
   2026-11-03 batching extensions recorded).

## b) PARTIALLY DONE

- **Single-item path (BatchItems=1) does not consult estimates**: the payload
  timeout stays the ladder/30m default. Deliberate scope cut — the batched
  path is the default and the single path is the opt-out; wiring it is a
  5-line follow-up (§f).
- **Live TypeSafe probing not done**: the typesafe-ai skill requires owner
  permission + budget + data-disclosure before live API requests. The client
  is verified against httptest doubles only; rubric wording is
  first-principles, not tuned against live answers (§f/§g).

## c) NOT STARTED (session-relevant, deliberately)

- Estimate-quality measurement (outcome-join): `effortMinutes` is now in
  payloads, but no audit joins estimates to derived outcomes/durations yet —
  that is the standing outcome-join follow-up, now unblocked with data.
- `--repo-timeout` ladder x effort-sum interaction doc (a ladder floor above
  the count ceiling is already safe via the max() rule; only the FLAG HELP
  spelling is owed).

## d) TOTALLY FUCKED UP (caught + fixed, none irreversible)

1. **New-root-package import broke the release floor** (`3c477fab` fix):
   internal/effort as its own package + cmd/tq importing it failed
   `check-gomod-vendor-sync.sh` — cmd/tq pins PUBLISHED root v0.4.1 (tagged
   2026-10-31) which cannot contain a package created 2026-11-03; the gate's
   `GOWORK=off go mod tidy` resolves the pin, not the workspace. Fixed by
   folding the estimator into internal/harvest (a floor package; symbol-only
   additions ride the documented cmd/tq facade-floor exception). Gate green
   after. LESSON: a NEW root package imported by cmd/tq requires a root
   release first — until then, new cmd/tq-reachable code goes into existing
   floor packages.
2. **Ran the root race battery concurrently with a vendor-regenerating gate**
   (both moved to background): the gate's `go mod vendor` tore the vendor tree
   under the running battery (`stdouttrace/config.go: no such file`). Battery
   re-run sequentially: green. LESSON: `go mod vendor` (any gate that runs it)
   and root builds/tests are mutually exclusive — sequence them.
3. Draft-stage defects caught before commit: `body_reader` placeholder,
   MinutesFromScore `+1` rounding bug, redundant cache-prune branch, a
   struct-field rename I made by accident (`donePreflight`) — all reverted
   pre-commit; two test bugs (stub's model pin too strict; roundtrip test
   overwrote its own fixture) fixed on first red.

## e) WHAT WE SHOULD IMPROVE (judgment, this session's evidence)

- The AGENTS size guard forced concept-preserving compression of the batching
  bullet AGAIN (25,416 → 25,395 B by trimming emphasis, not content). The
  standing owner-gated row to re-tighten the budget to ~24,000 gets more
  urgent every session; this session spent ~10 minutes of gate iteration on
  19 bytes of headroom.
- Effort semantics live in THREE flag surfaces (--batch-items count cap,
  --effort-budget minutes, --task-timeout ceiling) with derivation rules
  between them. Understandable but not obvious; a `tq agent-pool --dry-run`
  style banner line showing the effective triple per start (banner shows
  budget+timeout now, count cap is in the batching line) is the cheap cure.
- The rubric minutes ladder (10/25/60/150/240) is judgment, not measurement.
  Once `effortMinutes` rows accumulate against real durations, re-fit the
  ladder from data instead of feel (anti prompt-gymnastics: fit once from
  outcomes, not per failing case).

## f) NEXT (session-derived, priority order)

1. Owner answers the §g questions; if Jev goes live: probe the live API with
   representative + hard + negative TODO items (skill's test protocol), keep
   payloads, re-fit nothing until outcome data exists.
2. Wire estimates into the single-item path timeout (max(ladder, estimate)).
3. Outcome-join audit v0: for completed batches with `effortMinutes`, compare
   sum(estimate) vs wall-clock + session usage from the calibration join keys
   — first real calibration curve of the rubric.
4. Spell the `--repo-timeout` x effort-sum interaction into the flag help.
5. Batch-member count + effort sum into calibration rows (rides 2's join).
6. e2e pin: a pool-loop test with `--jev` against an httptest Jev (currently
   unit + harvest-level only).
7. Consider `internal/effort` extraction AGAIN at the next root release (the
   floor constraint disappears once a tag contains internal/harvest/effort.go;
   extraction then becomes a pure architecture call).
8. AGENTS budget re-tighten (standing owner-gated row — repeated §e evidence).
9. Redaction smoke run (last session's §f item; still unclaimed).

## g) QUESTIONS (cannot figure out myself)

1. **Go live with Jev?** `--jev` needs `TYPESAFE_API_KEY` and sends item
   texts + repo name/purpose to api.typesafe.ai (disclosed in flag help +
   this report). Skill requires explicit owner permission + experiment budget
   for live probing. Say the word and I probe + report estimate quality on
   real backlog items.
2. **Budget model for effort mode** (carried from last session, sharper now):
   a 240m monster and a 10m twirl each count as ONE task against
   `--daily-budget`. Effort data makes per-effort budgeting possible (cap
   MINUTES/day, not tasks) — ruling needed before implementing.
3. **Rubric ladder** (10/25/60/150/240): gut-fit. Alternative anchors: the
   executor's own 30m default as level 2, or repo-timeout ladder medians. No
   data to arbitrate yet (outcome-join is §f3); flag if you have priors.

## Verification (evidence, this session)

- Root: `go build ./...` OK, `go vet ./...` OK, `go test ./... -count=1 -race`
  ALL packages green (re-run after the fold; the 5 non-ok lines are "[no test
  files]" info lines, zero FAIL).
- cmd/tq: `./scripts/test-cmd-tq.sh` green x3 runs (incl. windows
  cross-compile; 14.5s/21.9s/28.8s).
- Gates: facade parity 7/7; agents-size 25,395/25,400 OK; doc-refs ok;
  release-docs ok; gomod-vendor-sync ok (root vendor + all module tidies +
  cqrs parity) — green only AFTER the floor fix; go-mods 53/53; erraudit 26
  modules clean; gofmt scoped clean (one pre-commit fix).
- Smoke: `scripts/smoke/multi-repo.sh` (scratch `TQ_DB=/tmp/tq-smoke-effort.db`)
  MULTI-REPO SMOKE OK (enqueued=6 completed=6 dead=0) — the pinned
  `--batch-items 1` path untouched by the fold, as predicted.
- New pins: 3 harvest batch tests + 8 effort tests + 1 cmd/tq flag test, all
  green in the above batteries.
