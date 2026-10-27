# Done-Prompt Window Report — Twelfth Dispatch

**Report:** 2026-10-27 08-10 · done-prompt for the 21-task window (19 closeout reports; 2 tasks without one, see §b/§d)
**Window covered:** 2026-10-25 ~04:00 → 2026-10-26 ~03:30 (plus downstream ticks through 2026-10-27 HEAD)

## a) FULLY DONE

All commit hashes below verified reachable at HEAD (`git cat-file -t` pass, 2026-10-27 08-10). Every task with a closeout report carries its own gate receipts; this section summarizes and cross-checks, it does not re-derive.

**CI-red triage & tooling gates**

1. **UseReadModel caller audit** (`ee0cb980`): cmdServe now defers `rt.Close()` on pre-run failure paths — the never-closed ProjectionRuntime leak shape (task 000001a171842151, report 2026-10-25_06-34 §a1). ci-local pins `GOTOOLCHAIN` from `check-toolchain-pins.sh --print-floor` (→ 1.27.3, `d34b37d0`); run-URL annotations landed on rows 894/895/896 (`27cdd453`). test-cmd-tq.sh green; rows 894/895 ticked.
2. **Script-syntax red cured** (`3b6c5adc`): SC2046 word-split in check-consumer-tag.sh:132 + SC2034 `pos_rc` fixed; check-script-syntax 100 ok / 0 failed at the time (task 000001a171d1f8d5).
3. **Fold-lock SIGSEGV on second `tq serve` fixed** (`14cb413a`): nil-Host pump parks on ctx.Done instead of faulting; webui smoke 12 asserts green after 2 deterministic-red reproductions. Root cause of the row-904 windows red documented (`cdb5cd2c` — test fixture `openProbe` held its create-side sql.DB pool open through the rename).
4. **False tick-claim corrected** (`a7316b30`, task 000001a171d686f2): the 06-34 closeout's "row 896 ticked" claim annotated FALSE-AS-SHIPPED in the source report; row 896's real tick confirmed later. Report §a.3/§a.4 now truthful.
5. **ProjectionRuntime.Close idempotency doc contract** (`4c88ec84`, task 000001a171d6870a): "safe to call once" lie rewritten — fields nil'd, second Close is a relied-upon no-op (serve wiring). Composition module build/vet/test green; zero remaining "call once" claims by sibling grep. The unit test (row 1092) deliberately deferred — see TODO.
6. **TODO_LIST glued-row reviewer fix** (`efbc8710`, task 000001a17228efa2): two items sharing one line split; the css-drift row is machine-visible again; check-todo-list glued-checkbox structural check shipped later (row 1152, DONE).
7. **check-verify.sh pin-3/pin-6 audit** (`b84132c6` via heal `bbf02dbe`, task 000001a173b7428a): literal flaky-test-name greps (TestExactlyOnceUnderConcurrency/TestSelfManagingLoop) DROPPED as redundant — pin 6 hardened to grep inside the KNOWN_FLAKY array region, strictly stronger. Full self-test green + negative battery rc=1 as expected. Rows 909 + duplicate 200 ticked.
8. **Status-loop poller + push-timing guard** (task 000001a175f4de40): `wait_completed` poller fix for the status-loop flaky smoke; new `scripts/check-push-timing.sh` with `--self-test` (10 cases), pre-push hook installed, ci-local-wired (`9ff212d6`, `9539f606`, `e496b1c8`). Receipts: script-syntax 109/0, guard-wiring 75/0, gate-ordering 28/0.

**Security / header-matrix pin batch (4 tasks)**

9. **SECURITY.md response-header matrix pinned** (`517fd666`, task 000001a1755938f5): TestServeHeaderMatrix/TestAPIHeaderMatrix assert exact per-surface header sets; drift #1 fixed in CODE (all 7 `/health*` bindings through `withHealthHeaders`, internal/webui/webui.go:225-231), drift #2 fixed in DOC (SECURITY.md:146). Row 477 ticked.
10. **CSRF-lockout 429 row pinned for real** (`2e1f80ff`, task 000001a17566f4a5): 3 bad CSRF tokens driven to the 429; Retry-After + full header set asserted. Plus per-route `/health*` subtests (`a0be62aa`) and apiHeaders derived from the matrix inventory (`f7445ff2`). Rows 478-480 ticked.
11. **Conditional-header model + two-way doc pin + negative path** (`e7b4a3ed`, `cc658bbf`, `3a2d3477`, task 000001a17579443c): per-surface conditionally-present model (caught the real posture: healthz/readyz/startupz carry NO Cache-Control); TestSecurityDocMatrixMatchesGoMatrix two-way divergence verified by mutation; TestMatrixCheckNegativePath (4 subtests). Gate receipt 6 ok / 0 fail.
12. **`/health*` subtest routes derived from `healthBindings()`** (`fd7d6530`, task 000001a1758b93d1): hand-written 4-path slice replaced; new bindings auto-join. webui `-count=1 -race` green. Row 486 ticked.

**Failure-evidence + redaction batch**

13. **Verify evidence stage queryable** (`13230bbd` + dedup `672c01be`, task 000001a1759de318): `tq dlq --evidence-stage <class>`, `tq show` `failure` JSON section, webui detail `evidence=` line, worker verify_stage pin. CHANGELOG Added row present (verified in CHANGELOG.md [Unreleased]); AGENTS.md/FEATURES.md updated. Receipts: cmd/tq 191/0, worker 42/0.
14. **SecretHits silence pins** (`3a6b0573`, task 000001a175b033d5): `SecretHits(RedactMarker) == 0` and `SecretHits("") == 0` as table rows (TestPostRedactionSilence). Plus the order-alignment/one-marker-per-sample and same-line-count pins (task 000001a173ce0af0: `2caa702a`, `0e411a57`, `e43db212`, `14582f27`; rows 923/924/925 ticked).
15. **Shared SECRET EVIDENCE fixture** (`7bfe4041`, tasks 000001a175d4d2eb + 000001a175e28e90): one `fakeAuditProbe` const consumed by redact_test.go AND journal-drift.sh phase 5; TestAuditProbeSharedFixture pin; 6/6 smoke phases PASS. The re-dispatch (…e28e90) was a paid no-op caused by a false tick-claim (see §d) — it verified the landed work and ticked row 942 (`2cc00033`).

**Docs & archive machinery**

16. **Mechanical archive-eligibility gate** (`9dc1571e`, task 000001a1754b7de36): `scripts/check-archive-eligibility.sh` refuses `git mv` of candidates with unresolved items; `--self-test` 9/9; wired into ci-local (scripts/ci-local.sh:634). Rows 296+549 ticked.
17. **Status-index prune + digest ritual** (`5281772d`, task 000001a175b033d5 batch item 3): 9 stale re-routed rows pruned (live 334→325; premise was stale — threshold already 400); monthly digest row ratified as the standing ritual (`5a8b945f`).
18. **Footer-last contract enforced** (`a471e1af` hook + `4810608c` templates, task 000001a175c28382 batch): commit-msg guard checks footer-as-last-non-empty-line; dispatch templates pinned (one missing template fixed); 8 footer rows ticked. This report's own commit rides the guard.
19. **Worktree smoke premise fixed** (task 000001a175f4de40 item 1): scripts/smoke/worktree.sh redesigned merge-tolerant (reviews now run mid-drain), re-run green (`49e4159e`).

**Windows CI confirmation** (task 000001a172245af9, no closeout — verified via TODO row 926): full test-windows job CONFIRMED GREEN via run 40175514907 (master 4c88ec84, readmodel rename retry ab0722d4 + CloseReadModel 03c1f7a6 both riding). Row 926 ticked with the run citation.

## b) PARTIALLY DONE

- **task 000001a173ee1329** (reviewer fix, "one finding") has NO closeout report and NO locatable commit: `git log --all --grep` on the id is empty and the repo-side journal (`tasks.db`) holds nothing past 2026-09-09. Either its fix landed under a footer this shell cannot attribute, or it died unrecorded. Unverifiable from here — filed as a TODO audit item.
- **Archive bar is gate-only**: `check-archive-eligibility.sh` exists but nothing mechanically enforces it at `git mv` time (00-05 §b1, row 1158); row 502's forward-only strike scope and the ~210-report strike pass not run.
- **Evidence-stage follow-ups**: webui list views lack the filter, cross-module decode still duplicated, gate-env/gate-dead/gate-slow remain error-TEXT aliases (01-40 §b, rows 1172-1177).
- **Push-timing guard inspects only ref TIPs** — fresh footer-less chores mid-history pass (03-22 §b1, row 1193). Poller fix empirical, not stress-proven (row 1194).
- **Doc pin ceiling**: TestSecurityDocMatrixMatchesGoMatrix covers header-name sets, not cell prose (row 1164); SECURITY.md Cache-Control prose trails the conditional model (row 1165).
- **~273-report ANNOTATE pass** deliberately deferred to the ratified digest ritual (row 1188) — not started this window.
- **headermatrix_test.go carries ~41 advisory lint warnings** + gocognit 53>25 (rows 1166/1167) — landed functional, not lint-clean.

## c) NOT STARTED

- Full ci-local + origin-CI validation of the whole wave: 60+ commits unpushed; every cure (fold-lock fix, css regen, four new gates) is a local-only proof until the owner authorizes the push (03-22 §g1, row 1137 update).
- Dead-sha citation debt: `check-dead-sha-refs.sh` rc=1 mass at HEAD (~hundreds of sites per the 00-35 §e5 observation) — heal tooling rows (385/386/45) open, risk of the red normalizing.
- check-status-index TRAILER WARNING fatigue (f26 cluster) — warning-fatigue remedy row 818 open.
- Retroactive archive alarm (row 1159), live enforcement hook (row 1158) — both unstarted.
- The reviewer-fix protocol row 1122 (grep every cross-reference before addressing a finding) — convention proposed, not yet pinned in AGENTS.md.

## d) TOTALLY FUCKED UP

- **Paid re-dispatch of landed work**: the shared-SECRET-EVIDENCE re-dispatch (task 000001a175e28e90) existed only because the 02-34 closeout claimed "TODO row ticked" but ticked row 939 instead of 942 (its own §d1 admits it). Same class as the row-896 false tick corrected this window (task 000001a171d686f2). The tick-claims gate (row 1151) remains unlanded.
- **Footer-less daemon folds pre-landing work invisibly**: the footer-position batch's item 3 was landed by fold `3810c13c` before the agent's commit — dedup could not see it, nearly double-shipping (02-22 §d2). Prompt-content changes are dedup-invisible when daemon-folded; rows 1184/1192 open.
- **The stale-premise red**: the check-script-syntax red (task 000001a171d1f8d5) and the css unminified drift (misdiagnosed first as an "empty sheet", corrected + fixed as `633a80c6`) both persisted because nobody ran the gate before triage; the 07-28 session's AGENTS-size premise was likewise 380 B stale (§d4 of its closeout). The verify-window premise-check line (row 1209) is the open fix.
- **Daemon-sweep combat cost**: at least 4 windows report multi-retry battles with the auto-commit daemon (edit→commit ordering violations, `--from` heal misfires ×3 in one session, in-session write-revert race with ~8 retries, 4 commits with `--no-verify`). Root cause of the write-revert race unknown (row 1195).
- **`heal-daemon-sweep.sh` over-heals**: folded intentional footered commits under generic subjects (03-22 §b4, row 1196) — the healing tool damages the history it cleans.
- **task 000001a173ee1329 disposition unknown** (see §b) — a possibly-dead paid reviewer-fix with zero paper trail.
- No regressions in shipped code were found: every window commit is test/gate-backed at its receipt time, and HEAD (including the 10-27 parked-provider batch) built its receipts on top.

## e) WHAT WE SHOULD IMPROVE

1. **Tick-claims must be commit-cited at closeout time** — two paid round-trips this window trace to uncited "row N ticked" claims (row 1151's exact thesis). Land the gate.
2. **Premise-check before planning** — run the gate a batch item's premise cites before writing code (row 1209); three windows paid for stale premises.
3. **Journal reachability for done-prompt sessions** — this report could not read the production journal or two tasks' dispositions from the repo shell (`tasks.db` stale since 09-09, no TQ_DB exported). Band-drift audits (§h) are structurally blind without it.
4. **Dry-run-first for one-off sweep tooling** (row 1183) — the 334-row deletion near-miss was caught only by the gate.
5. **Guard liveness probes** — the pre-push hook can vanish silently (row 1197); a session-start WARN exists for skills, add the same for hooks.

## f) NEXT THINGS

See the 10 items appended to TODO_LIST.md this pass (dedup-checked against 586 open rows; several near-duplicates were skipped in favor of existing rows 1151-1209).

## g) QUESTIONS FOR THE OWNER

1. May the 60+ unpushed commits (fold-lock fix, css regen, four new gates, windows-green evidence) be pushed to validate the cures in origin CI? Everything is local-proof only until then.
2. Should `SecretHits` count two back-to-back distinct secrets as 2 (boundary-aware matching), or is the one-location greedy-charset limitation accepted and merely documented (row 1156 documents; the policy is unruled)?
3. Is the production task journal intentionally unreachable from agent shells (done-prompt included), or should done-prompt sessions run with TQ_DB pointed at it so dispositions like 000001a173ee1329 and band-drift facts (§h) are auditable?

## h) BAND DRIFT

**None recorded.** `tq facts -type task.reprioritized` returns 0 facts journal-wide, and the full journal (189 facts) contains no reprioritized fact of any kind in or out of this window's timespan. Caveat: the journal reachable from this shell (`./tasks.db`) is stale (facts end 2026-09-09; the window's production store was not addressable — see §g3), so a move recorded only in the production journal would be invisible here. Within what is verifiable: no priority changes need explaining for ADR-0015.
