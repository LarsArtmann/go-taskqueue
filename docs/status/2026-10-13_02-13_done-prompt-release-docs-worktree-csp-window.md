# Done-Prompt Status Report — Release-docs hardening, worktree delivery, webui CSP window

**Date:** 2026-10-13 02:13 CEST
**Window:** 2026-10-12 01:57 → 2026-10-13 01:54 (13 dispatched tasks; 12 closeout reports present, ONE missing — see §b4/§g1)
**Method:** every listed closeout report read first; commits, TODO ticks, CHANGELOG entries, FEATURES/AGENTS rows, and the journal verified at HEAD (`0aca1703`). No code touched.

---

## a) FULLY DONE (verified, not just claimed)

1. **single_opener.md DESIGN→LANDED flip** (task …d654e9, commit `bcce882d`): status header at `internal/composition/single_opener.md:3` now cites `f6492a00` as landed evidence; TODO row ticked in the same commit; check-doc-refs green. Verified: header text present at HEAD.
2. **`release.sh --publish-steps-only`** (task …d719cb7): feature live in `scripts/release.sh` (entered via footer-less daemon sweep `56dffd7b`, tick commit `b84696d4`); TODO row ticked; closeout corrected twice by review-fix tickets (§d1/§d2). The flag itself works and is self-tested by nothing yet — doc pairing is the open half (§c1).
3. **check-release-docs.sh `--self-test` alarm pin** (task …da99c41, commits `9282b6de` + `26be367d`): the drift smoke re-runs itself against a mutated RELEASE.md copy and requires the exact doc-side miss; flatten-only negative control proven green; wired as a ci-local step. TODO row 77 ticked; CHANGELOG entry present under `[Unreleased]`.
4. **`poked demand-fill` log-line pin** (task …dbbec75, commit `b33d0937`): maximal-common-literal needle `poked demand-fill: https://proxy.golang.org/` pinned `need_in_both` (code: `scripts/lib/proxy-wait.sh:40`, doc: `docs/release/RELEASE.md:116`); negative control fired the exact new miss. TODO row 78 ticked; CHANGELOG entry present.
5. **Dogfood batch 3/3** (task …3646832, commits `4ee17193`, `81675a29`, `4fdc4dbf`): `need_in_both` flattened to captured `rc_script`/`rc_lib` with union semantics preserved (the first attempt broke lib-only-green and the smoke caught it pre-commit); `doc_flat` tr-snapshot built once; commit-task.sh gained `--body` + a `--self-test` pinning footer-as-LAST-line (live-exercised on its own commit); self-test hardened to exact-miss. TODO rows 82–84 ticked. Follow-up rows minted (ci-local wiring, body-ID refusal, union-green pin, exact-miss audit).
6. **Three review-fix tickets on the …d719cb7 lineage** (tasks …c513e3/…c5140b/…c51415, commits `5fff16c5`-adjacent `a6056967`, `be2530eb` line): §d5 footer-position correction (footer sat ABOVE the Crush attribution block on b84696d4/4acb02ff → queue derived ZERO commits for the task), §d6 addendum naming daemon sweep `56dffd7b` and the closed heal window (pushed), and the fabricated-cites cure (TODO_LIST "line 606" was blank; the 21-16 §43 "would have" conditional had been cited as "shipped"). All three verified in the report files at HEAD.
7. **Worktree-per-agent slice 1** (task …23f9a3c, commit `a9bef802`, healed across three footer-less daemon sweeps): executor `Worktrees` mode + `--worktrees` pool flag — `tq/<task-id>` branch from HEAD into `.tq-worktrees/<repo>/<id>`, committed-rails preflight, stale-branch refusal, AgentResult.WorktreeDir/Branch, parked close-outs keep the worktree, second-opinion clones never inherit it; facade parity green; 7 table tests + 1 real-git integration test green. TODO row 146 annotated, mint row 440 ticked, 5 slice rows minted.
8. **Worktree auto-merge batch** (task …251c305, commits `289628c3` auto-merge on review-approve, `5a75baba` orphan reaper, `0bf44d06` retry adopts the existing branch, `e6ad7dad` webui forensics render, `13f49cca` doctor `--hygiene` orphan count, `8f9710c4` `--project-exclusive` warning, `30b9ac78` AGENTS/FEATURES docs): TODO rows 462/463/466/467/468/469/470 all ticked with these commits as evidence. FEATURES.md agent-executor row and the AGENTS.md payload-contract bullet are current (verified at HEAD).
9. **Webui CSP `form-action 'self'`** (task …271c97c, commit `7e2746ed`): directive relaxed on dashboard routes only; `/health*` keeps `'none'` and its pin; route-table security test added; SECURITY.md + ADR-0003 amendment + CHANGELOG `[Unreleased]` entry landed; internal/webui module suite green. Verified: CHANGELOG entry and CSP row at HEAD match.
10. **Verify self-test pin bundle** (tasks …28418c0 + …9 1d427e, commits `01bd2e8e`, `a668b74a`, `88e78e00`, `5f67636e`, `f99e5478`, `8694ecc5`): KNOWN_FLAKY ↔ AGENTS.md Known Issues pin — bidirectional after the reverse gate caught the real `TestSelfManagingLoop` wording gap on its first run; in-situ genuine-red pin (shipped verify.sh under a failing go shim, rc=1, exactly one go call); O14 CHANGELOG policy codified in AGENTS.md; red-path spot-checks done and reverted clean. Verified: AGENTS.md Known Issues names both flaky tests; `scripts/check-verify.sh` carries both pins.

## b) PARTIALLY DONE

1. **RELEASE.md still does not cite `--publish-steps-only`** — verified by grep (zero hits). The flag shipped without telling the operator the resume path exists; rows 79 (doc citation) and 80 (need_in_both pairing pin) are open. This was the whole point of the original ask.
2. **Worktree arc headline row 146 is still `[ ]`** — correct, because the smoke (row 465: two concurrent same-repo tasks on a scratch repo) never landed. Everything else in the arc is ticked.
3. **CSP smoke gap**: `scripts/smoke/webui.sh` does not assert `form-action 'self'` (unit-level pin only) and the full ci-local battery was not run for that change (module suite only) — the closeout's own §b admission, verified true.
4. **Task …251c305 (the auto-merge batch) has NO closeout report** — the only window task without one. Seven feature commits + seven TODO ticks landed, but no a)-g) self-review exists for the batch, and the docs-health sweep had to reconstruct its scope from the git log. Deviation from the closeout convention.
5. **check-verify.sh pin-numbering drift** (02-08 closeout §b): the header comment says pins 5/6, inline comments say Pin 6/7, closeouts say 6/7 — three schemes coexist in one file. Not fixed here (hard scope rule); reported as a TODO item.
6. **The …d719cb7 attribution debt is documented, not repaired**: `56dffd7b` is footer-less AND pushed — the queue's git cross-reference derives zero commits/files for that task forever. The §d6 addendum makes the lineage human-auditable only.
7. **Self-test wiring**: both new `--self-test` modes (check-release-docs, commit-task) exist but the ci-local wiring row is open — they run only on demand until then.

## c) NOT STARTED (window skipped; still open at HEAD)

- RELEASE.md `--publish-steps-only` citation + pairing pin (rows 79/80) — §b1.
- Worktree smoke on a scratch repo (row 465) — the last unticked worktree slice.
- B10 pool-half detectors wired into the agent-pool tick (row minted by the 00-39 done-prompt; detectors compile but main.go never constructs them).
- wmErr replay-guard loud-warn (httpapi.go:111-117 still silent).
- Live-open pragma probe; freeze-class config sweep; dangling-SHA sweep + the 289-citation dead-sha cure decision pack (owner-arbitrated).
- ADR-0019 S2 journal flip on upstream `facts.Fact`; upstream ratification memo (owner-gated).
- Postgres CLI store wiring; consumer ghost-package decision; three-interface decision (all owner-BLOCKED).
- SystemNix input flip + round-9 cutover (sudo, owner-run).
- Retire `--dep-sweep`: its O15 time gate (earliest 2026-10-13) is NOW — eligible for the owner's go.
- Dispatch prompting + batch-granularity redesign (owner go/no-go pending).
- check-report-cites.sh decision (deliberately not auto-minted by the 00-19 fix ticket; owner call).

## d) TOTALLY FUCKED UP

1. **Fabricated-by-shape citations in a landed closeout (twice).** The original …d719cb7 closeout (4acb02ff) cited "TODO_LIST line 606" (blank at that commit) and misread a conditional ("would have") sentence in the 21-16 report as recording the flag as shipped. Both took paid review rounds to cure (`a6056967`, plus the footer fix). Worst class of failure: evidence that reads as verification without being verification.
2. **Footer-above-attribution commits corrupted queue attribution.** b84696d4 and 4acb02ff placed `Task-Queue-ID:` above the Crush block, so `git interpret-trailers` sees no Task-Queue-ID and the queue derives ZERO commits for that task. Only this lineage was audited (00-12 §b2 explicitly skipped a census) — other window commits may share the shape.
3. **Permanent attribution loss on the actual feature commit**: `56dffd7b` (the `--publish-steps-only` implementation, 84 lines) landed as a footer-less daemon sweep and was pushed before any heal — unattributable forever, only recoverable by prose addendum.
4. **Daemon sweeps struck at least five times this window** (…23f9a3c ×3 healed, …271c97c ×1 healed manually after the script refused on a dirty tree, …3646832 ×1 unhealed `f203f8aa`, plus `56dffd7b` pushed). The heal-daemon-sweep span-terminator bug (row open) and the dirty-tree refusal both forced manual fallbacks.
5. **Commit-ordering discipline repeatedly violated**: running self-tests between edit and commit caused the f203f8aa sweep; the 01-54 ticket put two pins in one commit and needed a soft-reset rework; the 00-46 ticket's first commit had a non-last footer.
6. **No new code regressions found**: every touched module's tests were green in its own closeout, and no gate at HEAD is red from this window (the known check-dead-sha-refs red predates it and is rowed).

## e) WHAT WE SHOULD IMPROVE

1. **Cite-check as a distinct pre-commit step**: re-read every file:line cite at staging time; quote verbatim any "X records Y" cite; prefer process cites (`git log -S`, commit SHAs) over position cites (line numbers) which die on the next edit. Two review rounds bought this lesson; codify it.
2. **Footer-last is mechanically enforceable — always use `scripts/commit-task.sh`**, and extend it to refuse a body containing its own `Task-Queue-ID:` line (row minted). The d5 failure class is one wrapper invocation away from extinct.
3. **Closeouts are mandatory per batch, even when the queue splits a feature across tasks**: the …251c305 gap (§b4) meant no self-review of seven feature commits. If the queue does not auto-mint one, the done-prompt window must (minted as a TODO item, §f3).
4. **Daemon attribution remains the top recurring tax** (5 strikes/window). The C12.4 GitLogScanner body-fallback landed, but footer-less sweeps still erase attribution when pushed. The standing fix is cultural: commit FIRST, run batteries second.
5. **Self-tests nobody runs are documentation**: wire the new pins into ci-local (row minted) — the 03-19 precedent already proved the five-step wiring pattern.
6. **Pin numbering needs a single authority** when pins are inserted mid-list (append-only numbering or name-based references instead of ordinals).
7. **No-op dispatches still owe the full look-around** — the RELEASE.md doc gap was caught only during the closeout's dedup pass, not during the work pass.

## f) UP TO 50 NEXT THINGS (top of the ranked list; items 1–3 minted into TODO_LIST this pass, the rest are already rowed — dedup-checked against 432 open rows)

1. **Cure the check-verify.sh pin-numbering drift** — header says 5/6, inline comments say Pin 6/7, the 01-54/02-08 closeouts say 6/7; one commit, pick ONE scheme (name-based preferred), cite 2026-10-13_02-08 §b. (minted)
2. **Footer-shape census over the window's commits** — `git log --format=%B 2026-10-12..HEAD | git interpret-trailers --parse` audit for the footer-above-attribution class (only b84696d4/4acb02ff verified; 00-12 §b2 skipped the census); record findings in a scratch doc, then decide heal-vs-grandfather per O16. (minted)
3. **Write the missing a)-g) closeout for the worktree auto-merge batch** (task 000001a13251c305eb46f0c37e5300000000) — verify each of the seven commits' evidence first (289628c3, 5a75baba, 0bf44d06, e6ad7dad, 13f49cca, 8f9710c4, 30b9ac78). (minted)
4. RELEASE.md: cite `--publish-steps-only` as the sanctioned resume path (row 79).
5. check-release-docs.sh: `need_in_both`-pin `--publish-steps-only` (row 80).
6. Worktree smoke on a scratch repo, two concurrent tasks, no orphan branches (row 465) — the arc's last slice, then tick row 146.
7. Wire check-release-docs.sh + commit-task.sh self-tests into ci-local.sh (minted row, 00-29).
8. commit-task.sh: refuse `--body` containing `Task-Queue-ID:`; pin the refusal (minted row, 00-29).
9. check-release-docs.sh self-test union-GREEN case (minted row, 00-29).
10. Exact-miss audit across all --self-test-bearing gates (minted row, 00-29).
11. Cure the check-dead-sha-refs red: 289 dangling citations + owner arbitration pack (minted row, 00-15).
12. Dangling-SHA sweep with merge-base check, pinned in check-doc-refs (minted row, 03-11).
13. B10 pool-half detector wiring in main.go (minted row, 00-39).
14. wmErr replay-guard warn (minted row, 00-39).
15. Live-open pragma probe (minted row, 00-39).
16. Freeze-class config-derivation sweep (minted row, 00-39).
17. gomod-vendor-sync instability ruling (minted row, 03-11).
18. Regenerate lint baseline post-readmodel fix (minted row, 01-00).
19. schema_test.go noctx/errcheck nits (owner-gated policy, minted row).
20. Postgres projection-home seam decision (owner-gated, minted row).
21. heal-daemon-sweep.sh span-terminator bug + --from UX (two minted rows, 00-04/01-41).
22. Legacy-DLQ: `LegacyDLQPathFor` extraction, both-sidecars surfacing, opt-in migrate flag (three minted rows).
23. Shared poison-projection fixture readmodel↔httpapi (minted row, 02-05).
24. Durable mutation-pin policy ruling (owner, minted row).
25. Queue-side heal of dispatch-prompt SHAs (owner, minted row).
26. `task.reprioritized` dead-machinery ruling (owner, minted row).
27. Done-prompt pacing ruling (~21h lag; this very report is another datapoint) (owner, minted row).
28. Retire `--dep-sweep` — O15 gate date reached 2026-10-13; ask for the go.
29. ADR-0019 S2 journal flip (unblocked since S1 shipped).
30. Upstream ratification memo to go-cqrs-lite (owner go).
31. Dispatch prompting + batching redesign go/no-go (MAJOR OPEN PROBLEMS doc).
32. Single-opener seam for stats/doctor (row 46); pragma-union pin (row 54); ADR-0016 S4 addendum (row 55); upstream system-seam ask (row 56, owner).
33. dlqPathForDeployment API reshape (minted row, 03-23).
34. Push master for green CI (owner push button — the window added ~25 unpushed commits).
35. SystemNix input flip + cutover (owner sudo).
36. Postgres CLI store wiring; consumer ghost decision; three-interface decision (owner).
37. check-gosec.sh self-test micro-polish set (minted row, sixth sweep).
38. Sibling-gate summary pins + negative-path battery (minted row, sixth sweep).
39. Session-close smoke coverage micros (minted row, sixth sweep).
40. Webui swap-guard verification debt (minted row, sixth sweep).
41. Provision local TQ_TEST_POSTGRES (owner, minted row).
42. tq doctor polish backlog triage (minted row, sixth sweep).
43. commit-task.sh `--amend` mode (row 667).
44. commit-task.sh `--body` multi-paragraph scope ruling (00-29 §g3).
45. f203f8aa heal ruling (00-29 §g1, unpushed window still open at report time — verify before acting).
46. Generalize exact-miss assertion pattern to other self-test scripts (00-29 §f13 — folded into item 10's row).
47. `doc_flat` snapshot for companion-pin loops if RELEASE.md grows (00-29 §f14, low).
48. Scripts-header self-test inventory comment (00-29 §f15, low).
49. AGENTS.md "X records Y cites must quote the sentence" convention line (00-19 §f4, owner home call).
50. Re-sweep v0.3.4 release-wave reports for conditionals cited as shipped elsewhere (00-19 §f5, low).

## g) QUESTIONS ONLY THE OWNER CAN ANSWER

1. **Closeout policy for queue-split batches**: task …251c305 landed seven feature commits with zero closeout report (the other 12 window tasks all have one). Should the queue mint a closeout per task even when a batch's tasks are mechanically split, or is the done-prompt window expected to backfill (as minted in §f3)?
2. **Is `check-report-cites.sh` wanted at all?** The fabricated-cites failure cost two review rounds; a cite-checker is grep-shaped and false-positive-prone. If yes: which reports, how many cites per spot-check?
3. **Which journal is canonical for done-prompt band-drift reads?** `TQ_DB` was unset in this session's shell, so `tq facts` read `./tasks.db` (397 facts, zero `task.reprioritized` all-time). AGENTS.md says shells inherit `TQ_DB` (production) — if the done-prompt dispatch is supposed to read the production journal, the dispatch env needs it set explicitly.

## h) BAND DRIFT (ADR-0015 accountability)

`tq facts -type task.reprioritized` over the live journal returns **zero facts** — none recorded for this window (or any window: the reprioritize machinery has never fired in this journal). No marker, AI, unblock, or importance moves to account for. This continues the standing pattern flagged in TODO_LIST (the "is `task.reprioritized` dead code in practice" owner question): priority movement this window happened via dispatch order and TODO_LIST row placement, not via recorded fact.

---

**TODO_LIST writes this pass:** 3 new task items (§f1–3) + 3 BLOCKED question items (§g1–3), dedup-checked against 432 open rows; zero ticks (all window-completed rows were already ticked by their own tasks). **Archives:** none moved — the 2026-10-12 sixth sweep already archived 23 files; every remaining live report still carries open items or is a window-adjacent closeout under `tasks/` (index-exempt). Living docs verified current: CHANGELOG (`[Unreleased]` covers the CSP relax, both release-docs pins, worktree docs via FEATURES/AGENTS), FEATURES agent-executor row (2026-10-13), AGENTS.md Known Issues (both flaky tests named, O14 policy present).
