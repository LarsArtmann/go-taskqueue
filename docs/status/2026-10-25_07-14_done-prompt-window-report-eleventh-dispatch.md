# Done-prompt window report — eleventh dispatch

**Report:** 2026-10-25 07:14 · **Window:** 14 tasks closed 2026-10-23 17:50 → 2026-10-25 06:11 · **Task-Queue-ID:** 000001a1718422a07ceb274ba1fa00000000

Method: all 12 available closeout reports read first; every §a claim below re-verified against the tree (artifacts present, gates re-run at HEAD) or against the cited commit. The two tasks without closeout reports are treated as findings (§d1), not work.

## a) FULLY DONE — verified

The window resolved into ten work streams. Receipts re-verified at HEAD unless noted.

1. **proxy-wait netns marker hardening** (`f4d1da08`, task 000001a15c2265…): `netns_is_empty()` now demands `/proc/self/net/dev` list exactly `lo` before `TQ_SMOKE_NETNS=1` is honored — an operator-exported marker on a non-empty namespace exits 1 loudly instead of silently voiding the structural-offline claim. Refusal path verified live in-session (host: 3 non-lo ifaces → exit 1); smoke 8/8 ok. `netns_is_empty` present ×3 in scripts/smoke/proxy-wait.sh at HEAD.
2. **Review-fix wave** (3 landed): row 777 canary tick (76ba0b8e), README index-row "rows 776-778 ticked" stale-claim corrected in place (ee6ac2bb), ci-local countable-gate self-tests reordered AHEAD of their real gates so the fleet convention is true of the wiring (1c7ce882). See §d1 for the two empty twins of this wave.
3. **Zero-deployment freeze-class sweep** (3f5e8dd1, task 000001a169b2b1…): the dispatched grep found exactly 2 hits, both the readmodel self-open seam (`projectionHomeCallerPragmas`), both env-honoring via `SyncPolicyFromEnv` and pinned by `TestProjectionHomeCallerPragmasHonorSyncEnv` (re-run green in-module). Classification-only outcome, row 823 ticked — honest zero-code-change result.
4. **Gosec micro-polish + fleet convention** (9660f35b, 46eb4dc8, bc5a347b/1dd616de; task 000001a16b0548…): all six gosec self-test micros (42 ok / 0 fail); **all four countable gates** (check-script-syntax, check-go-mods, check-dead-exports, check-guard-wiring) gained `--self-test` + negative-path batteries via env hooks; the fleet convention "countable gate summaries are PINNED" written into AGENTS.md and wired into ci-local (a real bug fixed on sight: dead-exports leaked rc=1 on its advisory path); session-close smoke coverage micros + the `scripts/smoke/COVERAGE.md` leg→assertion index.
5. **Webui swap-guard verification debt** (1dc53ed0, task 000001a16b348e…): Node test hook in `internal/webui/static/app.js`, 13-test zero-dep harness (`internal/webui/jstest/app_test.mjs`, incl. listener-drop, grace-window, data-state/focus/scroll re-apply invariants), `scripts/check-webui-js.sh` gate with self-test, contract doc `docs/webui-live-contract.md`. All present at HEAD. Three first-draft false-passing tests caught and re-pinned on landed markers.
6. **O14 CHANGELOG policy batch** (027aafc1, ad3e5363, c3977468; task 000001a16b4256…): AGENTS.md Conventions gained the full O14 bullet — **after discovering the batch's premise was false** (the cited "policy line row 159" never landed; 88e78e00 only ticked a TODO row, the AGENTS.md edit was daemon-folded 11 days ago). Mechanical O14 guard in check-release-docs.sh (+ self-test with exact-miss pin) fired a first live finding and moved a repo-internal row to `### Internal`; the mixed Changed row was split operator/internal.
7. **worktreeInFlight executor-side pin + OR-audit** (20fc83ab, 762e7a41; task 000001a16b764b…): `AgentExecutor.ParkCloseoutForTest` + `TestWorktreeInFlightCancelledParkedWithExecutor` pin the exact cancelled+parked shape the reaper died of (the store-side-only test passed `nil` for the executor — the gap is closed); the OR-over-store consumer audit (memo `docs/memos/2026-10-24_executor-store-or-audit.md`) verdict: `worktreeInFlight` was the ONLY OR-over-store liveness decision, every other consumer reads store status first. Both artifacts verified present; `ParkCloseoutForTest` ×2 in internal/executor/agent.go.
8. **CI-red triage batch 1** (558a3d47, d44a8a5a, 03c1f7a6; task 000001a16b9655…): webui-screenshots blank-capture guard (`check_png`, MIN_PNG_BYTES floor); **toolchain sweep to 1.27.3 in lockstep** (24 go.mod floors, 8 setup-go pins, flake tarball+hash; govulncheck "No vulnerabilities found" at 1.27.3; `check-go-mods` re-verified green 47 ok / 0 failed by this report); **CloseReadModel** — the windows `t.TempDir` unlinkat leak root-caused by reading close paths (no windows repro loop needed): `UseReadModel` armed 3 surfaces, tests closed none; fix extracts the production close order into `Server.CloseReadModel` with `t.Cleanup` in the four non-serving tests.
9. **check-ci hardening** (e94ed705, 1659a8f4, 25a35580; task 000001a16bb1cd…): `red_branch_relation()` extracted and hermetically pinned via a scratch git repo (same-tree / ancestor-predates / diverged); @tsv parse pinned end-to-end — **which caught a live field-shift bug**: a null run conclusion shifted every field under `IFS read`; jq now emits an `"unknown"` sentinel that correctly classifies red. Dead trailing echo dropped; two pairs of duplicate TODO rows ticked.
10. **CI-red triage batch 2** (e953a0e8/8e3b0378/fa867e99, 7742973b, e0532f80; task 000001a171641…): the CloseReadModel windows question answered YES from the existing CI run 40058586181 (Test step green at the fix's HEAD; `git merge-base --is-ancestor` verified) — the job's residual red was a NEW same-shape leak (`TestConvergeProjectionHome` rename sharing violation on windows), fixed with `renameWithTransientRetry` (go-retry, 3 attempts) in `internal/readmodel/schema.go` (×3 present at HEAD); `scripts/check-toolchain-pins.sh` shipped (floors == setup-go pins == flake ≥ floor; real run 10 ok / 0 failed, re-verified by this report) and wired into ci-local + ci.yml; `webui-screenshots.sh --self-test` pins check_png's FAIL lines verbatim and fixed a real `kill "${PID:-0}"` → process-group trap bug.

## b) PARTIALLY DONE

1. **Windows confirmation is substance-done but doctrine-unfinished**: the literal "trigger a focused rerun" was impossible (ci.yml has no `workflow_dispatch`; `gh run rerun` replays the same SHA and would go red on the new TestConvergeProjectionHome red). The substitution (cite existing run + fix the new red locally) was disclosed and correct, but the rename-retry fix is itself **windows-unverified** until the next master push (rowed).
2. **Standing gate reds at HEAD were flagged repeatedly but not cured** — three+ sessions documented them, none owns them: check-script-syntax "99 ok, 1 failed" (SC2034 check-guard-wiring.sh:44; SC2046 in check-consumer-tag.sh:132 same class), check-guard-wiring 2 orphaned scripts, check-todo-list rc=1 on its own advisories, check-fixture-attribution red (see §d3).
3. **Positive netns path remains inspection-only** (`unshare -n` EPERM everywhere this repo runs); row 802 stays BLOCKED on an owner ruling.
4. **O14 guard is a closed-set alarm** (3 signatures); the two un-judged 2026-10-20 Added rows (module-scaffolder Dependabot, dupe ledger) remain in operator sections pending the coverage-probe row.
5. **Focus-after-swap** shipped as a unit-harness assertion, not the browser livecheck the source report's wording implied (row 651's browser half still owns that).

## c) NOT STARTED (window skipped; pre-existing backlog, verified open)

- Extract `scripts/smoke/lib.sh` (row 509) — still blocking the socket-wait fold row (889), correctly honored as BLOCKED.
- Freeze-class follow-ups 1062–1068 (self-open WARN, var-freeze, grep gate, literal expected list, sqlitev4 error pin, init-derivation sweep, AGENTS one-liner) — minted by the window, none dispatched yet.
- Webui follow-ups 1073–1075 (AGENTS conventions sweep, shellcheck the new gate, harness timer coverage) — minted, not dispatched.
- Tick-audit convention (1053) + mechanical tick-claim gate (1057) + tick-cite convention (1058) — **minted by this window's own reviewer findings and still unchecked** (see §d2: the class re-fired within the window).
- check-todo-list exit semantics (1087), TODO dedup pass (1088), DONE-annotation contradictions (1083), OR-over-store grep guard (1080), RecordAnswer pin (1081), hermetic repoDir (1079), displaced doc-comment (1082) — all minted this window, all open.
- v0.0.0 pseudo-pin refusal in check-go-mods (1072); test-cmd-tq on the 1.27.3 floor — landed by the sibling 06-34 batch and ticked.
- The 2026-10-20 done-prompt-chain archive sweep (row 1044) — untouched; index bloat worsened (§d6).

## d) TOTALLY FUCKED UP

1. **Two paid review-fix tasks completed with ZERO attributable output**: 000001a1699ba3833830c158802300000000 and 000001a16b500bc6780f54cd988a00000000 — no commit in the entire history carries either footer, no closeout report exists, and neither is resolvable via `tq show` from this session. Each is the dispatch-twin (IDs seconds apart) of a review-fix ticket that DID land the fix (76ba0b8e / 1c7ce882). Pattern match: row 1025's "DONE agent tasks that never recorded a verdict" class (four claims, 2026-10-20) — now six, and review-fix tickets are affected too. Every gate is blind to a completed task that committed nothing.
2. **The landed-but-unticked class re-fired inside the window that was billed for it.** Reviewer finding #1 (18-40) was exactly this; by this report's sweep, the fix stream and siblings left rows 865/907 (injection point, landed 20fc83ab), 910 (OR audit, landed 762e7a41), 896 (run URLs, landed 27cdd453 — and the 06-34 closeout CLAIMS "row 896 ticked" while the checkbox never flipped), 870/1042/1070 (superseded by the 1.27.3 sweep, d44a8a5a), and 900 (landed post-window by e8af721f) all unchecked. Eight landed-but-unticked rows in one window — the tick-audit convention that prevents this (row 1053) is itself still unchecked. All eight are ticked by this pass with citations.
3. **Master is multi-red on repo-tooling gates and has been all window**: check-guard-wiring (2 orphans), check-script-syntax (1 failed), check-fixture-attribution (7 unattributed heredocs: check-gosec.sh:159/168/178 — flagged by the 02-09 session and never fixed — plus check-go-mods.sh:34 and three fresh ones in check-poll-loudness.sh from the post-window e8af721f, which **landed a new gate while the attribution gate was red**), check-todo-list (rc=1 on advisories). Nothing pushed, but every closeout's "repo green" is per-gate, never fleet-level, and the red-tolerant self-tests are green by design while the underlying reds persist.
4. **Daemon races dominated the window's cost**: ≥14 footer-less sweeps needed heal maneuvers across the sessions (01-50: 1 · 02-09: 3 · 02-31: 1 · 03-00: 1 · 03-20: 1 · 04-03: 2 · 06-11: 4, five fork-records · sibling 06-34: 2 via soft-reset fallback). AGENTS.md's edit→commit→battery rail was violated by name in at least three closeouts ("I read it and still ran the battery first"). Minutes before this report, ANOTHER unattributed 5-file Go sweep landed (3810c13c, 06:58) belonging to no window task — the attribution hole is not closing.
5. **A closeout claimed a tick that did not happen** (06-34: "row 896 ticked" — the annotation commit edited neighboring rows in place but never flipped 896's checkbox). This is row 1058's exact premise (aggregate/closure claims must cite the tick commit) — second live instance, now with a false-claim-in-report variant.
6. **Index bloat crossed every threshold**: 939 live `|` rows in docs/status/README.md vs the 400-row bloat point (the README's own math) — the archive sweep (row 1044) and the digest-row decision have been pending for five days while ~4 rows/day accrue.
7. Near-miss caught in-session (06-11 §d1): a root `go mod tidy` pruned 202 go.sum lines including the golangci-lint plugin block; caught via `git status` and restored. A blind commit would have shipped lint tooling churn inside a windows-fix commit.
8. **check-dead-sha-refs is corpus-wide RED at HEAD and being ignored**: this pass ran it in full for the first time in the window — 772 dangling cites across docs/status/ + TODO_LIST.md (row 30 recorded 108 at the 2026-10-12 rebase era; row 943 owns the window-8 residue; the count has grown ~7× while every session's battery ran the gate and moved on — the permanently-red-gate-trains-everyone-to-ignore-rc class of §d3, now at corpus scale). This pass fork-recorded the four window-cited shas it could verify by subject+reachability (d653361a→4e26f5bc, e0532f80→33ee8725, 011d2116→e73b0099 in the 06-11 report; 583a7b54→ee0cb980 in the 06-34 report); the remaining ~768 belong to rows 30/943's cure, not this dispatch.

## e) WHAT WE SHOULD IMPROVE

1. **Close the empty-output hole (§d1)**: extend row 1025's doctor/audit surface to review-fix tickets, and rule whether paired dispatches (twin IDs seconds apart for one finding) should dedupe at mint time — two free-looking paid turns per finding is the worst cost shape in the window.
2. **Land the window's own process fixes first**: 1053 (tick-audit), 1057 (tick-claim gate), 1058 (tick-cite convention) were minted by reviewer findings THIS window and would have caught all eight §d2 misses mechanically.
3. **Fleet-level green needs an owner**: a `ci-local --summary`/doctor "gate board" listing countable-gate red/green at HEAD would stop every session from re-discovering the same four reds by hand (§d3), and would make "repo left green" claims checkable instead of per-gate.
4. **Commit-first discipline needs a mechanical trigger** (pre-battery dirty-tree guard, rowed) — prose has now failed in at least five named sessions; each daemon heal is a paid round-trip plus fork-record obligations.
5. **Fixture attribution must be a landing requirement, not a follow-up**: a new gate (check-poll-loudness) landed with 3 unattributed heredocs on a day the attribution gate was already red for 3 older ones — the fleet convention should say a new countable gate lands with its `pins:` attributions the same commit.
6. **Windows confirmation doctrine**: either add `workflow_dispatch:` to ci.yml (one line, makes "trigger a rerun" executable forever) or ban the phrasing from dispatch templates — the current shape sent the 06-11 session down three dead ends it then had to disclose.
7. **One fix for check-gosec's fixture attributions and the two shellcheck warnings is ~10 minutes total** and has been re-discovered by three consecutive sessions — batch them into a single "green the fleet" dispatch instead of another round of flagging.
8. In passing (post-window): the sibling 06-34 session's two `git reset --soft` fallbacks orphaned commit shas — harmless today (nothing cited them, verified) but row 262's "heals re-run dead-sha-refs" needs to cover the soft-reset path too, not just heal-daemon-sweep.

## f) UP TO 50 NEXT THINGS (top 10 appended to TODO_LIST, dedup-checked; the rest live here)

1. Attribute the 7 unattributed fixture heredocs (check-gosec ×3, check-go-mods ×1, check-poll-loudness ×3) → green check-fixture-attribution. **[FILED]**
2. Add `workflow_dispatch:` to ci.yml. **[FILED]**
3. CI parity for the three new countable gates' self-tests (webui-js, toolchain-pins, poll-loudness): wire into ci.yml or rule local-only. **[FILED]**
4. `scripts/smoke/COVERAGE.md` freshness gate (fail on missing contract line). **[FILED]**
5. Queue-side guard for empty-output review-fix twins + paired-dispatch dedup ruling (extends row 1025). **[FILED]**
6. gitignore the `<db>.readmodel.db.fold-lock` sidecar family (untracked dirt on every readmodel-open in a checkout; reported per scope, not fixed). **[FILED]**
7. Index bloat: run the row-1044 ANNOTATE+ARCHIVE sweep over the 2026-10-20 done-prompt chain; 939 live rows vs 400 threshold. **[FILED]**
8. Fleet gate board (ci-local `--summary` or doctor section) surfacing countable-gate red/green at HEAD. **[FILED]**
9. Graduate `renameWithTransientRetry` to a shared home if row 906's os.Rename audit confirms the class at workerfailed.go:81 / depbump.go:586. **[FILED]**
10. check-ci self-test `command -v git || WARN+skip` environment guard. **[FILED]**

11. Green-the-fleet batch: rows 903 (SC2046+SC2034) + 871/1071 (wire-or-delete orphans) + item 1 + 1087 (todo-list exit semantics) in ONE dispatch — four reds, one session.
12. Land 1053/1057/1058 (the window's own process-fix trio) before any further batch dispatches.
13. Freeze-class follow-ups 1062–1068 (self-open WARN first — closes the composed-vs-self-open error asymmetry).
14. Webui follow-ups 1073–1075 + harness flushPendingSwap/two-id coverage + stub-vs-jsdom ruling.
15. Worktree follow-ups 1079–1082 (hermetic repoDir, OR-guard, RecordAnswer pin, doc-comment repair).
16. Socket-wait fold after row 509 lands (BLOCKED, correctly).
17. Row 1088 TODO dedup pass (887/898-style pairs were caught twice this window; more likely exist).
18. Row 1083: cure the two DONE-annotation-unchecked contradictions (501, 957).
19. AGENTS.md Known Issues one-liners: windows rename delayed-release class (next to ETXTBSY) + the `kill 0` trap class (next to session-shell hazards) — byte-budget permitting (660 B headroom today).
20. CHANGELOG: judge the 2 un-judged 2026-10-20 Added rows + O14 signature growth policy (rowed; owner call).
    21–50. Remainder of the minted-but-undispatched residue this window left: 1037/1038 (netns self-test + diagnostic), 1072 (v0.0.0 pins), quiet-tree smoke runs (902), footer-template fix (901), poll-loop grep guard — landed post-window (ticked this pass), record the 06-11 fork-records in the dead-sha baseline if the gate requires paste-in, and the standing owner-gated rows (802 netns CI leg, worktree-batch closeout report, footer census) unchanged.

## g) QUESTIONS FOR THE OWNER (up to 3)

1. **Empty review-fix twins**: were 000001a1699ba383… and 000001a16b500bc6… intentional duplicate dispatches (twin IDs seconds apart for one finding each), and should the queue void/dismiss the twin automatically when its sibling records a verdict? If accidental, the dispatch template is minting a paid no-op per review finding.
2. **Fleet-red posture**: are the four standing tooling-gate reds at master (§d3) accepted as advisory, or should the next dispatch be a single "green the fleet" batch (shellcheck ×2, orphan guards, fixture attributions, check-todo-list exit semantics)? Every session currently pays a re-triage tax on them.
3. **Index-bloat ritual**: digest rows (the README's proposed `| digest 2026-10 | …` format) or a standing monthly ANNOTATE+ARCHIVE sweep — which becomes the cure for 939 live rows, and should row 1044's chain be the first sweep?

## h) BAND DRIFT — task.reprioritized facts

**None recorded.** `tq facts --type task.reprioritized` returns 0 facts, and the journal this session can read holds no reprioritized fact type at all (189 facts: enqueued/claimed/completed/requeued/failed/dead-lettered/cancelled only). Scope note: this session's shell inherits NO production `TQ_DB` (the repo-local `tasks.db` is an idle scratch from 2026-10-17 whose journal head is seq 189 — it predates this window), so journal visibility here cannot cover the production queue that dispatched this window; ADR-0015 accountability for this window is therefore "no reprioritizations visible from any reachable journal," not a positive assertion that none occurred.

## Verification receipts (this report)

- Gates re-run at HEAD (2026-10-25 ~07:05): check-go-mods 47 ok / 0 failed · check-toolchain-pins 10 ok / 0 failed · check-release-docs ok · check-script-syntax 99 ok / 1 failed (SC2034) · check-guard-wiring 64 wired / 2 orphaned · check-fixture-attribution FAIL (7 unattributed) · check-todo-list rc=1 (advisories).
- Artifacts verified present: netns_is_empty ×3, jstest harness, check-webui-js.sh, check-toolchain-pins.sh, docs/webui-live-contract.md, scripts/smoke/COVERAGE.md, OR-audit memo, renameWithTransientRetry ×3, ParkCloseoutForTest ×2, TQ_SYNTAX_FILES hook in ci-local.
- Window commits verified by footer/content: f4d1da08, 76ba0b8e, 93b03c8e, ee6ac2bb, 4eabdf13, 3f5e8dd1, 9660f35b, 46eb4dc8, bc5a347b, 1dd616de, 1dc53ed0, 027aafc1, ad3e5363, 1ec1bab5, c3977468, 74b457b2, 20fc83ab, 762e7a41, 558a3d47, d44a8a5a, 03c1f7a6, e94ed705, 1659a8f4, 25a35580, a45136af, e953a0e8, 8e3b0378, fa867e99, 7742973b, e0532f80, 011d2116, d653361a, cb88c2d8.
- TODO ticks this pass (8, each citation in place): 865, 870, 896, 900, 907, 910, 1042, 1070.
