# Done-Prompt Window: readmodel-integrity five-task pack (api pump host, TQ_SQLITE_SYNC self-open, doctor DLQ split-brain, citation sweep)

2026-10-12 00:39 CEST · one status reporter · branch master · window spanned 2026-10-11 00:31 → 03:42 CEST (task execution + closeouts); this report was dispatched ~21 h after the window closed (see §g3).

Window tasks (all five `completed` in the live journal `/mnt/pool/services/tq/tq.db`, facts seq 10901–11070, verified this pass):

| Task         | Dispatch                                              | TODO row     | Verdict                              |
| ------------ | ----------------------------------------------------- | ------------ | ------------------------------------ |
| `…7f145912…` | api pump managed-host adoption (row-35 re-dispatch)   | row 48 `[x]` | no-op closeout — landed pre-dispatch |
| `…815e4486…` | review-fix: index the unindexed 00-00 closeout report | (review-fix) | fixed: one README index row          |
| `…5a8e4586…` | verify readmodel self-open honors `TQ_SQLITE_SYNC`    | row 49 `[x]` | bug confirmed + fixed + pinned       |
| `…76062b53…` | doctor projection-DLQ path split-brain                | row 50 `[x]` | fixed, test un-papered               |
| `…88ceb2f8…` | stale `DLQPathFor(d.DBPath)` citation sweep (row 20)  | row 20 `[x]` | verified no-op                       |

SHA note: the dispatch prompts cite pre-rewrite commit SHAs (all three unreachable at HEAD after the heal — see §d2), so this report cites only SHAs verified reachable at HEAD (`git merge-base --is-ancestor`). The dangling-dispatch-payload class is §d2/§g1.

## a) FULLY DONE (every claim re-verified at HEAD this pass)

1. **Api pump rides the managed projection host** (task `…7f145912…`; the underlying row-35 decision landed 2026-10-11 ~00:43 via daemon folds, closeout `bd077fae`). Verified in code at HEAD: `internal/httpapi/httpapi.go:85-132` — `UseReadModel` builds `readmodel.NewProjectionHost` + best-effort `OpenDeadLetters` from the readmodel seams (no composition import; the row-34 self-open ruling stands), `ListenAndServe` drives `Host.Start/Stop/Close`, and the delete-the-file replay hatch is preserved via one serial `CatchUp` (`httpapi.go:116`). Pins green this pass: `TestUseReadModelArmsManagedHost` + replay test, `go test ./internal/httpapi/ -run TestUseReadModel` rc=0 (0.026 s). Shipped inside v0.3.4 (tag `7052cb73` contains `NewProjectionHost` in httpapi); CHANGELOG carries it ("`tq api` projection pump is crash-safe"). Closeout was a disciplined no-op: footer grep + ticked row checked BEFORE any work; fresh verification battery added (root build/vet, httpapi -race, doctor-red reproduction).
2. **Review-fix landed exactly as scoped** (task `…815e4486…`): the finding — the 2026-10-11 00-00 closeout report existed but had no `docs/status/README.md` index row (the gate's maxdepth-1 blind spot, now owned by its own row) — was fixed with ONE row insertion (reachable heir `dd91fdb7`; the prompt's original fix commit dangled in the rewrite), `check-status-index.sh` rc=0 before AND after; the row is present at HEAD (README line ~130). The addendum `61b1ab84` discloses that the closeout's files rode a mixed footer-less daemon fold (healing deliberately refused — foreign work product inside).
3. **`TQ_SQLITE_SYNC` self-open fallback fixed + pinned** (task `…5a8e4586…`; fix commit heir `50a51012`, reachable). Confirmed broken at source level first: `ProjectionHomeCallerPragmas` froze `SyncNormal` from a zero deployment, so stats/doctor/api self-opens silently ignored the operator's env override while the composed serve path honored it. Fix verified at HEAD: `internal/readmodel/model.go` `projectionHomeCallerPragmas()` resolves `config.SyncPolicyFromEnv()` (full/normal/off, broken value falls back to the default tier in the fallback path; `config.FromFlags` stays the ONE erroring reader). Pin test (4 table cases incl. garbage-env fallback) green in the module gate this pass: internal/readmodel `GOWORK=off` build+vet+test rc=0 (0.212 s). In the v0.3.4 tag. **Attribution healed post-window**: the code had been swept into pushed footer-less `29b36f68` while the footer commit carried only TODO_LIST — the 18:32 heal merged both into footered `50a51012`, closing the gap the 02-44 closeout recorded as unhealable.
4. **Doctor DLQ split-brain fixed; the standing cmd/tq red is gone** (task `…76062b53…`; fix heirs `51c52c93`/`d760ae9d`/`4ae62788`, all reachable). Verified at HEAD: `OpenDeadLetters` derives the sidecar through `dlqDSN` → `DLQPathFor(PathFor(d.DBPath))` (internal/readmodel/dlq.go:45-78) — ONE derivation matching `DLQPathFor`'s "beside a projection database" contract; `TestDoctorProjectionDLQ` un-papered and GREEN via the proper devmod gate this pass (`./scripts/test-cmd-tq.sh -run TestDoctorProjectionDLQ` rc=0, 5.1 s). The known-red module gate recorded by the 01-09 closeout §e2 no longer exists. The follow-up chain landed after the window: row 19's seam pin (`4c7641bd`, `TestOpenDeadLettersInputContract`), the doctor legacy-sidecar WARN (`eb581e87`, 2026-10-12), and the AGENTS.md daemon-sweep manual-fallback doc (`2e68f2a4`) — all ticked with evidence.
5. **Citation sweep: verified no-op with a full classification** (task `…88ceb2f8…`; heirs `c7550ab7` + `7f2a62e6`). Repo-wide grep for `DLQPathFor` across code + unchecked rows + living docs found zero stale-shaped citations (every match is historical by design); row 20 ticked with the classification. Its two minted guard rows (stale-citation check script; unexported `dlqPathForDeployment` reshape) remain open by design — correct outcomes, not incomplete work.
6. **Window journal context (verified, both neighbor repos included)**: the pool completed 21 tasks in the window span across go-taskqueue, CV, and SystemNix (my five + their review twins + a dlqfix autopsy + status tasks). Shape: 92 claims / 59 requeues / 21 completions / 14 failures / 1 dead-letter, 1 cancel — the cancel is `000001a127d5cd` (go-taskqueue agent), an env-streak death ("agent verify gate dead") cancelled 03:46 with its autopsy completed in-window: the DLQ loop worked as designed on live data.

## b) PARTIALLY DONE

1. **The window's minted follow-up chain is half-landed.** Done post-window: OpenDeadLetters seam pin (row 19), doctor legacy-sidecar WARN (row 23), AGENTS.md daemon-fallback doc (row 25). Still open at HEAD, verified: the stale-citation guard script (TODO_LIST line 22), the `dlqPathForDeployment` API reshape (line 24), the single-opener seam for stats/doctor (line 46), the api poison-DLQ WIRE pin (line 51), the `single_opener.md` DESIGN→LANDED flip (line 52 — the file still reads `Status: DESIGN` at HEAD), the pragma-union equality pin (line 53), the ADR-0016 S4 addendum (line 54), the invalid-env WARN (line 673), and the vendor-sync gate instability decision (line 679).
2. **Task 3's queue↔code attribution was split at window close** (code in pushed footer-less sweep, footer on a docs-only commit) — closed later by the 18:32 heal, but only because a full msg-filter heal pass happened to run; the structural daemon-race gap remains (§e2/§d3).
3. **Task 5's second dispatch** (03:40) existed only to close the daemon-race loop (heal refused on non-contiguity → amend fallback). The work was complete; the dispatch spent a turn on git hygiene. Documented since in AGENTS.md Known Issues.

## c) NOT STARTED (backlog the window skipped — verified open at HEAD)

- The single-opener arc that this window's fixes left behind: stats/doctor still self-open (TODO_LIST line 46), the poison-DLQ wire is unpinned end-to-end (line 51), the pragma-union pin is prose-only (line 53), `single_opener.md` still says DESIGN (line 52), the ADR-0016 addendum is unwritten (line 54).
- The window's own process mints never landed: stale-citation guard script (line 22), `dlqPathForDeployment` reshape (line 24), invalid-`TQ_SQLITE_SYNC` warn (line 673), vendor-sync gate instability decision (line 679), re-anchor protocol doc (line 662).
- The standing owner-blocked set was correctly not touched: SystemNix deploy + input flip (rows 21/110/157), `--allow-writes` + DLQ rescue (row 92), dispatch-prompting redesign go/no-go (row 39), S2 journal unification (row 43), upstream filings (rows 44/55).

## d) TOTALLY FUCKED UP

1. **Phantom repeats kept burning paid turns through the whole window, despite the queue-side dedup rows being ticked.** Row 35 re-dispatched ~2 minutes after its footer-carrying landing (01-09 §e1); the doctor split-brain task earned TWO closeouts (03-00 and 03-11) plus this window's dispatch; the citation sweep got a second dispatch the same hour (03:40). Rows 190/562 (COMPLETED short-circuit, mint-time done-check) are ticked DONE, yet the 01-09 closeout's strongest-datapoint-yet question — is the dispatcher minting from a stale snapshot with no post-mint COMPLETED check? — remains unanswered from where agents sit. Every repeat is a full paid window on landed work.
2. **Dispatch payloads carried SHAs that a same-day heal invalidated.** Three of the five dispatch prompts name pre-rewrite commit SHAs that the 18:32 heal rewrite dangled. Any agent trusting the prompt SHA over `git log --grep 'Task-Queue-ID'` pays a resolve cost or, worse, concludes the work vanished. The docs-side cure (check-dead-sha-refs addendum, TODO_LIST line 683) cannot reach queue payloads — only the owner can fix or stop the practice (§g1).
3. **Daemon attribution races hit 3 of 5 windows** (code swept into pushed footer-less `29b36f68`; closeout files folded into a mixed footer-less fold — healing refused; sweep closeout folded into a non-contiguous footer-less fold — amend fallback proven). The amend fallback is now documented (AGENTS.md, `2e68f2a4`), but the root cause — commit-after-verify ordering — recurred in-window despite AGENTS.md mandating stage+commit first (the 02-44 closeout's own §d1/§e1).
4. **The env-streak breaker burned real attempts on concurrent-agent tree churn**: journal shows task `…815e4486…` requeued 6× on preflight "repo has uncommitted changes" and lost 2 of its attempts to the 3-streak rule before completing. Working as designed, but under `project-exclusive` + a 5-minute pool interval + constant daemon commits, the design spends attempts on the co-tenancy it was meant to punish.
5. **check-dead-sha-refs is RED at HEAD** (289 accreted dangling citations; TODO_LIST line 683; ci-local's dead-sha step is a hard step). Not caused by this window, but this window's closeout-era SHAs are part of the accretion, and every new docs file written under the red gate must self-police reachability (this report cites only `merge-base`-verified SHAs).
6. **One closeout cited recorded verification instead of fresh runs** (01-09 §d1, self-caught and repaired in-report). The pattern matters because two later windows repeated it before the 03-11 pass codified the green-check-first remedy — still only convention, not enforced.

## e) WHAT WE SHOULD IMPROVE

1. **Make the re-dispatch/status-repeat suppression real.** The window is the cleanest dataset yet: 5 dispatched tasks → 8 paid windows (2 no-op re-dispatches, 1 repeat closeout pair, 1 same-hour second dispatch). Queue-side fix, not agent discipline (§g1/g3).
2. **Stop embedding commit SHAs in dispatch payloads.** Task text + closeout report path identify work durably; SHAs rot under any heal. If a SHA is kept, prompts should mark it "context, verify with `git log --grep Task-Queue-ID`".
3. **Codify "green-check before no-op close" in the closeout prompt.** This pass re-ran all three pin suites in under 6 seconds total; the 03-11 closeout's remedy is cheap enough to be a contract line, not a habit.
4. **Commit-first under the daemon needs mechanical help, not reminders** — two of five windows still lost the race after re-reading the rule. Options already rowed: daemon deferral is infeasible owner-side (02-44 §g2); the remaining lever is dispatch-side tooling (a commit-task.sh-first template) or accepting the heal path as normal (it is now documented + has the amend fallback).
5. **Band-drift accountability has an input problem**: `task.reprioritized` has NEVER fired in the live journal (~11.3k facts, zero reprioritize facts) while FEATURES row 84 advertises marker/keyword/unblock bumps. Either the machinery is dead code on the dogfood pool (prioritize owner-gated OFF, row 124) or the fact emission is broken — one probe would answer (§g2).
6. **cmd/tq test invocation**: three raw-`go test` failures in-window before reaching `scripts/test-cmd-tq.sh` (03-11 §d2). AGENTS.md documents the gate; dispatch prompts for cmd/tq tasks could name it in the verify line.
7. **Docs-health mechanics** (carried, owned rows): index bloat warning still active (live rows ≫ 100; rows 532/148), row-format drift in the index table continues (row 583), and the two legacy-root 09-15 closeout reports remain the oldest unannotated live files for the next archive sweep.

## f) UP TO 50 NEXT THINGS (report-only; minted subset marked)

1. Pin the api pump's poison-DLQ WIRE end-to-end — already queued (TODO_LIST line 51): one test folding a non-retryable poison fact through the api-armed host, asserting a row lands in the corrected sidecar while stats stays served. The highest-value unpinned seam this window's task 1 left.
2. Route stats/doctor through the single-opener seam (line 46) — the last self-open call sites; also unlocks the deferred serve call-site-count pin.
3. Stale-citation guard script in the check-* family (line 22) — makes task 5's manual sweep permanently unnecessary.
4. Unexported `dlqPathForDeployment` so the wrong shape cannot compile (line 24) — the structural root-cause fix behind tasks 4+5.
5. **MINTED**: wire B10's pool-half alert detectors (see Appends) — explicit written-but-unwired partial from the adjacent 23-58 window with NO backlog row at HEAD until this pass.
6. **MINTED**: loud-warn on the api pump replay guard's wmErr skip (see Appends; found in the 01-09 closeout §e5, still silent at HEAD).
7. **MINTED**: live-open `PRAGMA synchronous` probe (see Appends) — closes the computation→application gap task 3's pin leaves.
8. **MINTED**: zero-deployment freeze-class sweep (see Appends) — task 3's bug class has siblings (`config.SQLite("")/Postgres("")` init-time derivations).
9. Pragma-union equality pin (line 53) — task 3 kept both sides delegating to the same builder; the pin is green-as-written but unlanded.
10. Flip `single_opener.md` to LANDED (line 52) — one-line docs task, verified still DESIGN at HEAD.
11. Fold the S4 facade ruling into ADR-0016 (line 54).
12. Invalid-`TQ_SQLITE_SYNC` stderr/doctor warn (line 673) — operator typos are still invisible in stats/doctor/api.
13. Vendor-sync gate instability decision (line 679) — every clean-checkout gate run still produces an 18-file heal.
14. Cure the check-dead-sha-refs red (line 683) — the pre-push gate is red at that step for everyone; the cure pack is fully specified.
15. Cure-check companion: after 14 lands, re-verify this window's closeout citations resolve to their heirs (heal fork-records cover them).
16. Re-anchor protocol doc (line 662) — the review-fix window's protocol, one fold from lost lore.
17. heal-daemon-sweep `--from` UX (line 28) — second occurrence of the eyeball-git-log dance recorded 2026-10-12.
18. SystemNix input flip + deploy (rows 21/110/157, owner) — production still runs v0.3.3 while master holds the whole window's + wave's fixes; the deployed pool cannot exercise any of it.
19. `--allow-writes` on deployed tq-serve + the DLQ rescue (row 92, owner) — the production host visibly carries BOTH legacy (`tq.db.dlq.db`) and corrected (`tq.db.readmodel.db.dlq.db`) sidecars (verified on disk this pass): the legacy poison facts are invisible until the shipped WARN runs on a current binary or the both-exist surface (line 27) lands.
20. Doctor: surface the legacy path when BOTH sidecars exist (line 27) — exact live state on the production host today.
21. Release v0.3.5 when the [Unreleased] set (worker-death surface, stale-items, legacy-sidecar WARN) justifies a cut — owner timing (row 35's wave shows the path).
22. Dispatch-prompting redesign go/no-go (line 39 + docs/problems/major-prompting-and-batching.md) — this window's repeat-burn data (§d1) is one more datapoint for the memo.
23. S2 journal unification on upstream `facts.Fact` (line 43, unblocked).
24. Upstream ratification + system-seam filings (lines 44/55, drafts exist).
25. Retire `--dep-sweep` after the Dependabot week (line 38, time-gated earliest 2026-10-13).
26. Lint nits accepted at the v0.3.4 seal (line 37) — readmodel ctx/close hygiene.
27. Post-33: fold the memo's "pending ruling" wording + serve call-site-count pin (01-09 §f7-8) — born-stale until 2 lands.
28. Row-note accretion trim (rows 32-35 DONE notes ≥1k chars) — rides the weekly docs-health ANNOTATE cadence.
29. Warning-fatigue remedy for check-status-index (02-07 closeout mint, line-owned by 84/532) — every run ends in the same known-class warnings.
30. check-status-index advisory tasks/-presence mode (line-owned row 605) — would have caught the exact daemon-fold class task 2 fixed.
31. Index the 10 remaining unindexed legacy closeouts (row 604 class).
32. f26 canonical-ID ruling (line-owned row 84/97 cluster, owner).
33. queue-side: repeat status-demand suppression (03-11 §f25; subsumed by §g3 here).
34. Env-streak economics review: preflight "uncommitted changes" requeues under co-tenancy cost attempts without agent fault (§d4) — consider excluding daemon-swept-pending states from the streak or widening the streak window; owner posture call first.
35. Replay-guard posture ruling (01-09 §g3): warn vs accept — item 6 implements the warn side; ruling decides whether it lands.
36. `tq verdict` fallback path: dispatch shells without `tq` on PATH write $TQ_RESULT_FILE by hand — consider shipping the binary path in the dispatch payload (minor; observed this pass).
37. Status-report index rows for done-prompts: consider a fixed "window span" prefix cell so pacing lag (§g3) is visible in the index itself (cosmetic; row 583's format doc would own it).
38. Band-drift probe: emit one synthetic `task.reprioritized` on a scratch journal test to prove the ADR-0015 fact pipeline compiles end-to-end even while unused in production (tiny, test-only).
39. Finish the B10 bridge-half test matrix if the pool-half wiring (item 5) surfaces more aggregates (rides item 5).
40. Archive sweep resumption (row 148): the three 09-15-class legacy-root reports + then 09-16..09-25 — bounded ANNOTATE passes, not this window's scope.

(41-50 deliberately not listed: everything else noticed in passing is already queued under the rows cited above; padding would mint duplicate dispatches — the exact failure the backlog's dedup contract exists to prevent.)

## g) QUESTIONS I CANNOT FIGURE OUT MYSELF

1. **Dangling dispatch payloads after heals**: the queue's task texts embedded commit SHAs that the 2026-10-11 18:32 heal rewrite invalidated (§d2). Can queue-side task payloads be healed owner-only (as the 03-11 closeout assumed), or should dispatch generation stop embedding SHAs entirely? The docs-side cure (line 683) cannot reach queue payloads.
2. **Is `task.reprioritized` expected to be live?** Zero such facts exist all-time in the production journal while the ADR-0015 machinery is advertised as FULLY_FUNCTIONAL (FEATURES row 84) and prioritize stays owner-gated OFF (row 124). Dead-code-in-practice, or a pipeline that should fire on marker bumps?
3. **Done/status prompt pacing**: this window closed 03:42 and its done-prompt dispatched 00:23 the next day (~21 h). Is that `status-every=5` pacing working as intended, or is the done-prompt queue backed up? Determines whether 21-h-stale window reports are still worth a paid turn.

## h) BAND DRIFT (ADR-0015 accountability)

**None recorded.** The live journal (`/mnt/pool/services/tq/tq.db`, queried directly this pass) contains zero `task.reprioritized` facts across its entire history (~11.3k facts, facts table scanned for the type), so there are no priority moves in the window span (2026-10-11 00:30–04:30 CEST) or anywhere else to explain. All five window tasks sat at stored priority 0 (default backlog band); claim order for them was arrival + aging only. The drift machinery (markers, keyword bumps, unblock bumps, the `--prioritize` AI scorer) exists and is documented (FEATURES row 84) but has never emitted a fact on this pool — see §e5/§g2 for the accountability gap that follows from a permanently-silent audit channel.

## Appends to TODO_LIST.md (this closeout — minted 4, blocked-questions 3; dedup-checked against all 404 unchecked rows at HEAD)

See the `## Done-prompt follow-up (2026-10-12 00-39…)` section at the TODO_LIST tail: B10 pool-half wiring (HARVEST of the 23-58 report's explicit partial — verified unqueued), replay-guard wmErr warn, live-open pragma probe, zero-deployment freeze-class sweep, plus the three §g questions as BLOCKED rows. Nothing ticked this pass: every row this window's evidence proves done (19/20/23/25/48/49/50) was already `[x]` with citations at HEAD.

_Archival note (docs-health): no root-level 2026-* report qualified for archival this pass — the recent window reports (B9/B10, v0.3.4 wave/seal) each carry explicit open items, and the older corpus is owned by the row-148 sweep cadence. Living docs reconciled instead: CHANGELOG gained the legacy-sidecar WARN + the split-brain backfill entry; FEATURES gained the legacy-sidecar doctor cell + the api-pump managed-host clause; README/ROADMAP/AGENTS verified current (README's `TQ_SQLITE_SYNC` claim at line ~257 became TRUE with this window's fix)._
