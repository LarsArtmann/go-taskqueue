# Done-Prompt Window Report — Thirteenth Dispatch

**Report:** 2026-10-27 08-49 · re-run of the twelfth dispatch's window (task 000001a17bfb2501 re-claimed)
**Window covered:** 2026-10-25 ~04:00 → 2026-10-26 ~03:30 (same 21-task window as `2026-10-27_08-10_done-prompt-window-report-twelfth-dispatch.md` — read together; this report verifies, updates, and does not re-derive)

## Why a thirteenth dispatch exists

The twelfth dispatch committed its report and TODO edits (3ce1609d, 08:17) but **never recorded its `tq verdict`** — `$TQ_RESULT_FILE` was empty at dispatch time (verified 2026-10-27 08-49). The queue re-claimed task 000001a17bfb2501 and paid this re-dispatch: a live, self-inflicted instance of the exact "report without verdict = half-closeout" class the twelfth's own close contract warns about. This dispatch records the verdict as its scripted final step.

## a) FULLY DONE

All 18 spot-checked window commits are reachable at HEAD (`git cat-file -t` pass over ee0cb980, 3b6c5adc, 14cb413a, 4c88ec84, efbc8710, b84132c6, 517fd666, 2e1f80ff, fd7d6530, 13230bbd, 3a6b0573, 7bfe4041, 9dc1571e, 5281772d, a471e1af, 49e4159e, ab0722d4, 03c1f7a6 — zero missing). Per-stream verification detail is the twelfth report §a1-§a19; nothing found contradicting it on re-check. Summary:

1. **UseReadModel caller audit** — pre-run `rt.Close()` defers in cmdServe (ee0cb980); GOTOOLCHAIN floor pinned in ci-local (d34b37d0).
2. **check-script-syntax red cured** (3b6c5adc) — SC2046/SC2034 in check-consumer-tag.sh.
3. **Fold-lock SIGSEGV on second `tq serve` fixed** (14cb413a); root cause of the windows red documented (cdb5cd2c).
4. **False row-896 tick corrected** (a7316b30) in the 06-34 source report.
5. **ProjectionRuntime.Close idempotency doc contract** (4c88ec84) — "call once" lie removed repo-wide.
6. **Glued TODO row split** (efbc8710) + glued-checkbox structural check landed.
7. **check-verify.sh pin-3 dropped as redundant** (b84132c6) — pin 6 strictly stronger.
8. **Status-loop poller + push-timing guard** (9ff212d6, 9539f606, e496b1c8) with --self-test.
   9-12. **SECURITY.md header-matrix pin batch** — matrix asserted in Go (517fd666), CSRF-lockout 429 row driven for real (2e1f80ff), conditional-header model + two-way doc pin + negative path (e7b4a3ed, cc658bbf, 3a2d3477), /health* routes derived from healthBindings() (fd7d6530). Real posture fix: all 7 /health* bindings through withHealthHeaders.
9. **Verify evidence stage queryable** (13230bbd, 672c01be): `tq dlq --evidence-stage`, `tq show` failure section, webui evidence= line. CHANGELOG Added row verified present in [Unreleased].
   14-15. **SecretHits silence pins** (3a6b0573 + 22-34 batch) and **shared SECRET EVIDENCE fixture** (7bfe4041) — redact_test.go and journal-drift.sh phase 5 consume one const.
10. **Mechanical archive-eligibility gate** (9dc1571e) with --self-test 9/9, ci-local-wired.
11. **Status-index prune + ratified digest ritual** (5281772d, 5a8b945f).
12. **Footer-last contract enforced** (a471e1af hook + 4810608c templates).
13. **Worktree smoke redesigned merge-tolerant, green** (49e4159e).
14. **Windows CI confirmed green** — run 40175514907 on master 4c88ec84 (readmodel rename retry ab0722d4 + CloseReadModel 03c1f7a6 riding); TODO row 926 ticked with the run citation.

Since the twelfth: the parked-surface provider batch (4eecffd0/390226e6, 1547e8c5/a732f257, 416ff2a0 one-fetch perf, fdb258b9 postgres conform recipe) landed 3/3 with rows ticked (00552b0c) and its closeout indexed (b3ad16fc) — outside this window, no action needed here.

## b) PARTIALLY DONE

(carry-over, all verified still open at HEAD)

- **task 000001a173ee1329** (reviewer fix): still NO closeout and NO locatable commit; disposition audit filed as TODO row 1211.
- **Archive bar is gate-only** — check-archive-eligibility.sh exists, nothing enforces it at `git mv` time (row 1158); the ~210-report strike pass not run (row 1188).
- **Evidence-stage follow-ups**: webui list filter, shared reader extraction, comma-set filter, non-dead-no-evidence pin, smoke case (rows 1172-1177).
- **Push-timing guard inspects only ref TIPs** (row 1193); poller fix empirical, not stress-proven (row 1194).
- **Doc-pin ceiling**: TestSecurityDocMatrixMatchesGoMatrix covers header-name sets, not cell prose (rows 1164/1165).
- **headermatrix_test.go lint debt**: ~41 advisory warnings + gocognit 53>25 (rows 1166/1167).

## c) NOT STARTED

- **Origin-CI validation of the whole wave**: 60+ commits still unpushed at HEAD (verified: master is local-only ahead); every cure remains local-proof until the owner authorizes the push (row 1215, BLOCKED).
- Dead-sha citation mass at HEAD (heal rows 385/386/45 open).
- First PROSPECTIVE monthly digest row (row 1186) and the resumed weekly ANNOTATE pass (row 1188).
- Reviewer-fix cross-reference protocol pin in AGENTS.md (twelfth §c, row 1122 lineage).

## d) TOTALLY FUCKED UP

- **This dispatch's own event**: the twelfth dispatch was a half-closeout — committed but no `tq verdict` → re-claim → this paid re-dispatch. The verdict recording is the close contract's final exit step; a committed report without it burns a full re-dispatch. Root cause: the close steps are prose, not tooling; nothing flags a same-day done-prompt report whose verdict was never recorded.
- **Inherited from the window (verified by the twelfth, still true)**: two paid round-trips from uncited tick claims (rows 896/942; tick-claims gate row 1151 unlanded); footer-less daemon folds making prompt-const work dedup-invisible (rows 1184/1192); heal-daemon-sweep over-heals footered commits (row 1196); the in-session write-revert race unexplained (row 1195).
- No code regressions found on re-check: every window commit carried gate receipts at its receipt time and HEAD builds on top.

## e) WHAT WE SHOULD IMPROVE

1. **Make the verdict step mechanical**: the done-prompt close contract should live in the dispatch template + docs/status/README.md skeleton as a copy-paste command (same pattern as the gate-receipts convention, row 1210), and ideally a session-end WARN when a `<ts>_done-prompt-*` report exists in the working tree without a verdict receipt. This dispatch exists because that step was optional in practice.
2. **Tick-claims commit-cited at closeout time** (row 1151) — unchanged, still the highest-leverage process fix.
3. **Premise-check before planning** (row 1209) — unchanged.
4. **Journal reachability** (row 1217, BLOCKED) — this dispatch re-hit the same wall: `tq facts -type task.reprioritized` returns 0 facts against the stale repo-side `./tasks.db` (facts end 2026-09-09), so §h remains structurally blind.
5. **Dedup guard worked as designed**: this dispatch dedup-checked every candidate against 592 open rows and appended only genuinely new work — the 2026-09-11 re-fire loop did not recur here.

## f) NEXT THINGS

The window's follow-ups are already densely rowed (1151-1222); do not re-row them. New items appended this pass (2, dedup-checked):

1. **Done-prompt verdict-receipt pin**: pin the close contract (report + TODO edits + `tq verdict` as the scripted FINAL exit step) into docs/status/README.md's a)-g) skeleton and the dispatch templates, so a committed-but-unverdicted done prompt is a visible convention violation rather than a silent paid re-dispatch (this event: twelfth dispatch 3ce1609d, verdict missing at 2026-10-27 08-49).
2. **Open-row triage via the ratified digest ritual**: 592 unchecked rows at 2026-10-27 08-49 (grep-counted), several provably landed per CHANGELOG [Unreleased] — run the first digest pass (row 1186's vehicle) as an evidence-backed tick/prune sweep so the backlog stops accruing landed work. (Distinct from row 1188, which is the 2026-09 ANNOTATE pass over reports, not TODO_LIST rows.)

## g) QUESTIONS FOR THE OWNER

Unchanged from the twelfth — all three are already filed as BLOCKED rows; do not re-add:

1. **Push authorization** for the 60+ unpushed commits (row 1215).
2. **SecretHits back-to-back policy** — boundary-aware matching vs documented limitation (row 1216).
3. **Production-journal reachability for done-prompt sessions** (row 1217) — this is also the fix that would un-blind §h.

## h) BAND DRIFT

**None recorded.** `tq facts -type task.reprioritized` returns 0 facts (re-run 2026-10-27 08-49), and no reprioritized fact of any kind is present in the journal reachable from this shell. Caveat unchanged from the twelfth: `./tasks.db` is stale (facts end 2026-09-09; TQ_DB unset in agent shells), so a move recorded only in the production journal would be invisible here — row 1217 is the standing remedy. Within what is verifiable: no priority changes need explaining for ADR-0015.
