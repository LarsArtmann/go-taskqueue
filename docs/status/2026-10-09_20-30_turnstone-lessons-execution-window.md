# Turnstone-lessons execution window (T1–T25)

**Window:** 2026-10-09 ~19:35–20:30 (+0200), interactive owner window (non-queue; no `Task-Queue-ID`).
**Base HEAD at window start:** `795d4da7` (the SUPERB plan commit). **HEAD at report:** `8ec3e75d`.
**Driver:** execute the Pareto plan `docs/planning/2026-10-09_19-32_SUPERB-turnstone-lessons-execution-plan.md` (T1–T25 / F1–F48), one verifiable step at a time.

---

## a) FULLY DONE (verified at HEAD, not just claimed)

1. **T1 — effect-disposition design memo.** `docs/planning/2026-10-09_effect-disposition-design.md`: chose Option A (`EffectStatus` on the existing `task.released` detail), proved the reclaim observer cannot soundly assert `none`, so the path stamps `unknown`. Decision made autonomously under the owner's "keep going" authorization.
2. **T2 — vocabulary.** `journal.EffectStatus` (`committed|none|unknown|partial|rolled_back`) + `journal.ReleasedDetail` in `internal/journal/journal.go`; re-exported via the `journal/` facade. `check-facade-parity` green (7 facades).
3. **T3 — the real fix.** `internal/queue/companion/claims.go`: both the reclaim branch (`:165`) and the cancel branch (`:136`) now stamp `releasedDetail(EffectUnknown, …)`. Additive fact detail; no change to claim ordering/lease math.
4. **T4 — SIGKILL mid-effect.** Covered by T3's cancel-path stamp (`cancelled-mid-run`); the plain lease-expiry crash is the `lease-expiry` stamp. No separate code path exists (the worker's death IS lease expiry).
5. **T5 — surfacing.** `tq facts` human line gains `[effect=…]` (`cmd/tq/main.go`); the web UI timeline renders `effect <e> (<reason>)` (`internal/webui/components.go`). Tests: `TestFormatFactSurfacesEffectDisposition`, `TestDetailFactsSurfacesEffectDisposition`.
6. **T6 — DLQ autopsy.** `DLQFixPayload.ReclaimsWithUnknownEffect` populated by the sweeper scanning the dead task's `task.released` facts; `dlqFixPrompt` warns "prior attempt may have partially landed". Tests: `TestDLQFixPromptWarnsOnUnresolvedEffect`, `TestSweeperAutopsyCarriesReclaimDisposition`.
7. **T7 — tests.** conform suite test `TestReclaimRecordsUnknownEffectDisposition` (registered in `conformanceTests`, runs for every backend), plus `TestLeaseLostMidExecution` worker assertion. `go test` green per module.
8. **T8 — chaos e2e.** `TestChaosKillWorkerMidRun` now SIGKILLs a real worker process and asserts the reclaim records `unknown`. PASS (1.5 s).
9. **T9 — narrowing rule as code.** `executor.MergeFindings` (`internal/executor/finding.go`) = `max(deterministic, learned)` + union-of-flags; `finding_test.go` proves a learned "benign" finding cannot lower a deterministic one (incl. `SecretHits`). Exported helper; additive.
10. **T10 + T12 — security doc pins.** `SECURITY.md`: the narrowing section now names `executor.MergeFindings` + the test; a new "Reads are not free" paragraph names the bridge fetches as untrusted data. `scripts/check-security-invariants.sh` (with `--self-test`) gates both the prose and the helper; wired into `ci-local.sh` (guard-wiring green: 55 scripts, 0 orphaned).
11. **T11 — budget serialization.** `internal/e2e/budget_serialization_test.go`: two handles over one DB agree on `SpentToday` after concurrent enqueues. Audit conclusion: the budget is a COUNT over enqueued facts (single-writer serialized), so no read-then-write overdraw is possible by construction.
12. **T14 — model-keyed score cache.** `internal/prioritize/sweep.go`: `scorerSource()` suffixes the model; a cached verdict whose Source differs is stale and re-joins the batch. Test `TestModelChangeRejoinsTheBatch`.
13. **T15 — cross-version audit.** No separate compare path exists; the cache is the only cross-version surface and T14 closes it. Recorded, not re-implemented.
14. **T19/T20/T22/T23/T24/T25 — docs/hygiene.** `docs/research/README.md` index; turnstone citations pinned to `@ad95c3c`; M24 corroboration noted on the child-ceiling row; `TestExactlyOnceUnderConcurrency` documented as the state-ablation falsifier; `docs/references/research-note-template.md`; ROADMAP scheduled-self-audit row.
15. **Guards green:** `check-doc-refs`, `check-features-roadmap`, `check-todo-list`, `check-status-index`, `check-security-invariants`, `check-facade-parity`, `check-guard-wiring`, `check-gomod-vendor-sync`.

## b) PARTIALLY DONE

- **T13 — child-task authority/budget ceiling.** Design-gated, NOT implemented. Wrote `docs/planning/2026-10-09_child-authority-ceiling-design.md` (no parent link on a task today; the rule needs a hot-path schema decision) and marked the TODO row `— BLOCKED: owner ruling on the parent-grant model`. Per guardrail 1, paused rather than guessed.
- **T21 — AGENTS.md reclaim-gap known-issue.** Skipped: `check-agents-size` is ALREADY over budget (foreign/concurrent-agent red, ~19011 B vs 18500), so no bullet could be added without a prune. The rule ("only if bytes freed") forbids it.
- **T16–T18 — verdict→outcome calibration dataset.** NOT started (see §c). This is the one 20%-tier item with real feature weight; it needs a schema + migration + writer + query surface, which the window did not have budget to land safely after the critical path.

## c) NOT STARTED

- **T16** calibration-table schema + migration; **T17** writer from the verdict channel; **T18** query surface (`tq calibrate` / `tq audit --verdicts`). TODO row 611 stands, unclaimed.
- **T20 optional link-rot check** (the pin landed; an automated rot check did not).

## d) TOTALLY FUCKED UP

- **Two self-inflicted edit slips**, both caught and fixed in the same step: (1) editing `formatFact`'s anonymous struct once removed a `d.Class != ""` guard, merging two branches — corrected immediately; (2) a `SECURITY.md` insert briefly deleted the "Security at the agent boundary runs in BOTH directions." sentence (a sloppy `old_string`) — restored within one edit. No lasting damage; both are the exact "edit with too little context" class the rules warn about.
- **First `check-security-invariants.sh --self-test` passed a mutation it should have failed** (the `root` was resolved at script top, so the temp override was invisible). Fixed by resolving the root inside `run_gate`; re-ran and both drift mutations now fail as intended.
- **Foreign tree churn:** the auto-commit daemon swept my core changes into several `chore: auto-commit` commits before I could stage them; the root `vendor/` was stale against a committed go.mod bump (untracked, so `go mod vendor` was safe and unblocked all root gates). Neither was mine.

## e) WHAT WE SHOULD IMPROVE

- **Land calibration (T16–T18) as its own window.** It is the last turnstone lesson with feature weight; a focused schema+writer+query pass with its own verify battery is the right shape, not a bolt-on at the tail.
- **A `check-security-invariants` self-test that actually mutates a copy is the pattern the other guards already use** — worth replicating where guard semantics are only pinned by the green-path run.
- **The `journal.EffectStatus` `none` value is defined but unproduced.** If a genuine never-dispatched release path appears, wire it; until then the enum honestly reflects tq's observation limits.
- **Root `vendor/` staleness** keeps biting (a committed go.mod bump without `go mod vendor`). A pre-commit `check-gomod-vendor-sync` already exists but runs late in ci-local; consider moving it earlier.

## f) UP TO 50 NEXT THINGS

1. T16: calibration-table schema + migration (verdict × outcome) + legacy-DB ALTER test.
2. T17: writer from the verdict channel (`executor` registered results) + derived outcomes.
3. T18: `tq calibrate` / `tq audit --verdicts` query + snapshot.
4. T13: owner ruling, then fold the child ceiling into M24.
5. T20: link-rot check for the pinned turnstone SHA.
6. T21: prune AGENTS.md to free bytes, then add the reclaim-gap known-issue.
7. Decide whether `journal.EffectStatus.EffectNone` gets a producer.
8. Consider a `TestMergeFindings` fuzz/property seed.

## g) QUESTIONS (only the owner can answer)

1. **Calibration (T16–T18):** proceed next window as a standalone feature pass?
2. **Child ceiling (T13):** which axis first — autonomy (`Yolo`), priority, or paid budget — and explicit `ParentID` vs derived lineage?
3. **AGENTS.md budget:** the file is over the 18500 guard; prune (and by how much) or consciously reset the twin budgets?
4. **Turnstone pin:** is `ad95c3c` (observed main 2026-10-09) an acceptable permanent pin, or should the note track a release tag (e.g. `v1.9.0a1`)?

## h) BAND DRIFT

`./tasks.db` band facts: not measured this window (interactive, non-queue; `$TQ_DB` unset — no claim/requeue traffic generated by this window). No drift attributable to this work.
