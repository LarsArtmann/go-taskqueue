# Nix-Job Cures (CSS Repin + Treefmt) + Tick-Claim Audit — Session Report

Created: 2026-10-25 17:30 CEST · Session: direct user request (no queue task · footer-less)
Scope: continue the SUPERB Pareto plan (`docs/planning/2026-10-25_15-40_SUPERB-PARETO-FLEET-GREENING.md`) — correction wave for the 16-57 report, fine tasks F10–F12, CI triage of the two red runs, and cure of both nix-job failure classes found there.

---

## a) FULLY DONE

1. **Row 871 premature-receipt CORRECTED (the 16-57 report's §d honesty flag).**
   `TODO_LIST.md:880` now states the true sequence: wiring exposed the smoke
   had ROTTED (first live run rc=1; four failure layers: in-run review
   auto-merge, watermark consuming completions without `--review`, plumbing
   merges never touching the main working dir, reap leaving empty per-repo
   parents) — REWRITTEN to pin the real contract, final rc=0
   (`/tmp/tq-gates/smoke-worktree10.log`); the earlier "(standalone rc=0)"
   note is explicitly flagged as a premature receipt written before any live
   run.
2. **CHANGELOG Internal bullet extended** with the smoke re-pin (rewrote
   `scripts/smoke/worktree.sh` to the evolved contract after the first live
   run proved the old shape rotted).
3. **All 10 check-tick-claims WARNs itemized.** The 16-57 session's
   `grep '^WARN'` missed because the literal line format is
   `TICK-CLAIM WARN:` — verified against `/tmp/tq-gates/tickclaims2.log`
   (318 ok / 10 WARN). The 10: rows 131, 128, 129 (2× reports 2026-09-19 and
   2026-10-04), 440 (2026-10-13), 834 (2026-10-15), 896 (2× 2026-10-25
   reports), 233-235 (README:165), 89 (README:901), 777 (the gate's own
   founding falsehood, header-documented).
4. **F10: row 440 re-rowed as genuinely open.** Verified the dlqfix webui
   surface has NOT landed (grep: 0 `dlqfix`/`DLQFix` refs in
   `internal/webui`); the 02-13 report's "mint row 440 ticked" proven a
   numbering-drift MISQUOTE — the mint row it described was 461, ticked at
   `74247e4a`. Row 440 (dlqfix autopsy webui surface) annotated accordingly.
5. **F11: 834/128/129/131 residue RESOLVED — every claimed work item landed.**
   Evidence per claim: strike-pin/lockout = `d8809930` (commit exists);
   auth-body shape = `6369ea75` (exists); gosec GOSEC_VERSION derivation =
   live at `ci.yml:403`; parked-surface CHANGELOG entry = live at
   `CHANGELOG.md:250`. The WARNs are premature-at-commit claims + row-number
   drift in a multi-agent repo; no row at HEAD needed ticking.
6. **F12: grep-audit for gate-blind tick-claim phrasings — one genuine false
   hit found and filed.** The gate regex covers "row(s) N[-M] ticked/`[x]`"
   with ≤2 intervening words; the audit probed number-after-verb, multi-slash
   (≥3 slash numbers), and 3+word-distance forms. Found 5 multi-slash claims
   (233/234/235, 236/237/238, 891/892/893, 894/895/896, 871/1087/1102) — ALL
   verified true-at-claim or landed-later (896 landed same day; 233/234/235
   all landed, today's rows 233/234/244). The one FALSE claim: archived
   `docs/status/archived/2026-09-29_04-15_task-…25678.md:32` claims "row 143
   was already DONE and ticked" while the row (Per-key map bound sweep) is
   `- [ ]` at BOTH the report commit `c7b54564` and HEAD (today's row 198).
   Filed as a new review-fix row (widen `claim_re`'s `{0,2}` word window;
   keep the multi-slash exclusion — row 1067 documents it as deliberate).
7. **CI triage of both red runs — each nix failure root-caused to ONE line.**
   Run 40205050267 (head `6f9b4ebc`): nix job failed ONLY on the
   byte-canonical app.css pin ("FAIL: committed app.css drifted from the
   tailwind rebuild"); all 7 other jobs success. Run 40207620242 (head
   `f4ff9b1f`, another agent's commit): nix failed ONLY on treefmt; all 7
   other jobs success. Receipt: `/tmp/tq-gates/ci-40205050267-failed-jobs.log`.
8. **Treefmt red CURED.** Single flagged file:
   `internal/harvest/verdicts_test.go` (stray blank line inside a test).
   Host gofmt (1.27.3) does NOT flag it — treefmt uses the nixpkgs go
   formatter with different blank-line handling (the flake's known
   toolchain split). `nix fmt internal/harvest/verdicts_test.go` fixed it
   (1 deletion); module test `TestRedispatch…` green (GOWORK=off);
   `nix build .#checks.x86_64-linux.treefmt` rc=0 re-verified.
9. **app.css drift CURED — the session's main find.** The committed
   `internal/webui/static/app.css` was UNMINIFIED (7,454 lines, 183,147 B)
   while `scripts/build-webui-css.sh` has emitted `--minify` output since
   2026-09-07 (rebuilt: 1 line, 145,582 B, same tailwindcss 4.3.3 binary —
   version mismatch ruled out by bisect; the script's exact entry file
   reproduces the full output when run from the repo). This is the SAME
   class the ci.yml "Byte-canonical app.css pin" step was built for after
   unminified css shipped to master twice on 2026-09-11 — it re-shipped via
   daemon sweeps (hooks bypassed) and sat through multiple CI runs.
   Regenerated via the canonical script, committed `633a80c6` (1 insertion,
   7,453 deletions), guard rc=0 ("app.css is in sync with the templates").
10. **Fleet-green posture at HEAD: every CI job except nix is green on both
    red runs, and BOTH nix causes are cured locally** (css rc=0, treefmt
    rc=0). Local master is 9 commits ahead of origin, headed by `633a80c6`;
    push + next CI run is all that remains to validate GREEN end-to-end.

## b) PARTIALLY DONE

1. **F3 (dated evidence file for the standing reds):** receipts exist but
   scattered (`/tmp/tq-gates/*.log` + this report); no single consolidated
   dated file as the plan demanded.
2. **F4 (classification rows):** rows 1109/1113 were annotated last session;
   TODAY's outcomes (css + treefmt cures) are NOT yet folded into them —
   the "todo-list advisory-exit" remainder (own row) is unchanged.
3. **Full ci-local receipt:** not executed this session; the worktree-smoke
   wiring still lacks an in-full-gate pass (standalone rc=0 + the two cured
   gates verified scoped only).
4. **Verification of the 16-57 report's CI premise:** the 16-57 §a3 claim
   "nix+test BOTH solely on TestAgentsDocSizeGuard" is now SUSPECT for the
   nix job (this session proved nix had the css cause at 50267; whether
   3714773's nix also failed on css was NOT re-verified — its full nix log
   was not re-pulled). Correction pointer appended to the 16-57 report.
5. **The 08-32 closeout's css "footgun" warning DISPROVEN — with a local
   receipt.** `docs/status/tasks/2026-10-25_08-32_task-…ea700000000.md`
   claimed the rebuild emits a "2-line empty sheet (NOT a stale commit —
   regen-committing would strip styles)". Decisive local test: checking out
   the OLD expanded css and running `scripts/check-webui-css.sh` FAILS it
   (rc=1), and the failing diff's `+` side shows the rebuild is a FULL
   minified sheet — every layer, utility, and tc-* class present, nothing
   stripped (receipt: `/tmp/tq-gates/css-guard-on-expanded.log`). The
   regenerated file committed as `633a80c6` is style-bearing; correction
   appendix appended to the 08-32 report (daemon-folded `ca45c587`).
5. **Fleet gate board (T10/F17/row 1118):** unbuilt — today re-demonstrated
   the need (both nix reds were hand-discovered, again).

## c) NOT STARTED

1. **F7** — check-todo-list exit-semantics rewrite + `--self-test` pin
   (owner-gated: structural-violation set ruling, 16-57 §g Q1).
2. **F8** — dangling-cite ruling memo (owner-gated: heal format + 769
   baseline, 16-57 §g Q2).
3. **F9** — heal wave 1 (100 oldest dead SHAs) — depends on F8.
4. **F13–F56 / T9–T27 / B1–B10** — plan backlog untouched this session.
5. **Row 1050** — five glued TODO lines.
6. **The newly filed tick-claim-regex row** — filed only.
7. **Owner rulings from 16-57 §g** — unanswered; re-asked below (§g).

## d) TOTALLY FUCKED UP

1. **The 16-57 session's CI triage was under-diagnosed — by me.** I
   attributed BOTH failed jobs (nix + test) to the AGENTS.md size guard
   without pulling the nix job's own log (`--log-failed` was available).
   The nix job had a SECOND, independent failure class (app.css) that
   stayed live for two more runs. Corrected here; the 16-57 file gets an
   appendix pointer, not a rewrite.
2. **The unminified app.css shipped and SAT.** The canonical guard existed
   in ci-local, the ci.yml twin step exists — yet an expanded 183 KB
   stylesheet rode daemon commits past both, because daemon sweeps bypass
   pre-commit hooks and the css was last committed inside one. The guard
   DETECTS but nothing HEALS; the artifact was red-in-CI for at least two
   runs (40205050267, and per §b4 possibly 40203714773) before anyone
   read the nix log.
3. **My own css-debug loop wasted ~6 tool calls on ghost theories.** The
   first `git diff --stat` already said "1 insertion(+), 7,453 deletions"
   — i.e. minify/format class — and I instead chased toolchain version,
   PATH order, and nix-sandbox resolution (including two broken /tmp
   script copies whose `cd "$(dirname "$0")/.."` silently aimed at the
   wrong root, and a sed'd trap line that never fired). The byte-count was
   the diagnosis; I looked at it last.
4. **Nothing was lost:** no data loss, no reverted foreign work, all cures
   committed (`633a80c6` css; treefmt fix daemon-folded; TODO/CHANGELOG
   edits folded into `9cba5c33`/`d322e735`/`c7b75dde`).

## e) WHAT WE SHOULD IMPROVE

1. **Attribute from the job LOG, never the job list.** "nix failed" without
   the failing step's lines produced last session's wrong premise;
   `gh run view <id> --log-failed | grep FAIL` is a 10-second habit.
2. **Read the byte-count before theorizing.** committed-vs-rebuilt asset
   size delta (183 KB vs 145 KB, 7,454 lines vs 1) is a complete diagnosis
   for minify/format drift; grep for the class before bisecting toolchains.
3. **Debug copies of path-sensitive scripts must run from their real
   directory** — `scripts/build-webui-css.sh` resolves the repo from `$0`;
   a /tmp copy breaks silently. Copy in place (`scripts/*.debug.sh`), never
   to /tmp.
4. **Grep receipt files for the LITERAL log token first** (`TICK-CLAIM
   WARN:` vs assumed `^WARN`) — sample one line before writing the filter.
5. **Row-number citations in reports rot within hours here** (three of
   today's seven WARN clusters were numbering drift). Extend the
   cite-the-tick-commit convention: aggregate claims should also cite the
   row's first few words, so drift is self-evidencing.
6. **The css guard should heal or the artifact should stop riding daemon
   sweeps**: either a regen-on-drift step (CI auto-commit) or an explicit
   "generated asset — regenerate, don't hand-edit" note where templaters
   touch it. Owner call (§g Q2).
7. **treefmt↔gofmt divergence keeps producing invisible-locally reds**
   (nixpkgs-go formatter vs toolchain gofmt): pin treefmt to the repo
   toolchain or document `nix fmt <file>` as the standing cure in AGENTS.md
   Known Issues.
8. **LSP noise unchanged:** 17 stale gopls "go.work lists go 1.27.2"
   false positives fired on every tool result (go.work is 1.27.3 on disk;
   toolchain-pins gate rc=0) — trust CLI gates only, as documented.

## f) FIFTY things to get done next (impact-ordered, owner-gated marked 🔒)

**Fleet-green closure (now):**
1. Push local master (9 commits, head `633a80c6`) and confirm the next CI
   run is GREEN end-to-end — both nix causes are cured but unproven in CI.
2. Re-pull run 40203714773's nix log; confirm/deny css drift failed it too;
   keep the 16-57 annotation accurate.
3. Run FULL ci-local locally — the in-gate receipt for the worktree-smoke
   wiring plus css/treefmt guards green in context.
4. Consolidate F3's dated evidence file (all red classes + receipts, one file).
5. Fold today's outcomes into rows 1109/1113 (four local reds green at HEAD;
   remainder = todo-list advisory posture) and row 1122.

**Gate semantics (owner-gated where marked):**
6. 🔒 F7: check-todo-list exit semantics (advisory rc=0) + `--self-test` pin.
7. Row 1137: structural glued-row violation pin (pairs with F7).
8. 🔒 F8: dangling-cite ruling memo (baseline vs heal, cost per option).
9. F9: heal wave 1 — 100 oldest dead SHAs via subject match + fork-record
   arrows (after F8).
10. Widen check-tick-claims `claim_re` word window ({0,2} → more) with a
    self-test of the 09-29 row-143 shape (filed row today).
11. F13/F14: review-fix zero-output guard (completion-without-output
    detector) + wire into ci-local.
12. F16: extend ci.yml with the 3 countable-gate self-tests.
13. F17/T10/row 1118: fleet gate board (`tq doctor` section first).
14. 🔒 css drift policy: heal-on-drift (auto-regen commit) vs detect-only.
15. T15: pin self-test-BEFORE-gate ordering in ci-local (dce6c12b class).
16. Row 948: golang-lint version single-pin (GOSEC_VERSION pattern).
17. Consider check-tick-claims hard-gate flip (advisory → rc=1) after the
    regex widening — 🔒 owner call.

**Docs health:**
18. F18/T11: ANNOTATE+ARCHIVE wave A (2026-10-20 done-prompt cluster,
    ≈200 index rows).
19. Row 1050: heal five glued TODO lines.
20. B1–B4: docs-health floors (FEATURES/ROADMAP/CHANGELOG/AGENTS freshness).
21. B5–B8: windows & process items (dispatch-env TQ_DB export, closeout
    policy, tick-claim citation extension per §e5).
22. B9: fleet/deploy + owner-gated leftovers.
23. B10: long tail + v0.4 re-row.
24. Harvest this report's §f into TODO_LIST/ROADMAP (docs-health HARVEST).
25. Annotate the 16-57 report §a3 with the corrected nix-job attribution
    (appendix pointer to this report).

**Webui:**
26. F21: webui smoke keeps TMP + dumps log on failure.
27. F22: webui smoke auth-wait FAIL with log tail.
28. F19: projectionPump nil-Host read-only mount pin (cmd/tq unit test).
29. F20: demoted-serve staleness contract (SSE + /api/stats).
30. 🔒 Row 440: dlqfix autopsy webui surface (needs the surface ruling).
31. css artifact: add "generated — regenerate via scripts/build-webui-css.sh,
    never hand-edit" header note where templaters will see it.

**Tooling hygiene:**
32. T16: heal UX improvements.
33. T18: retry graduation.
34. T19: COVERAGE.md + check-ci guards.
35. T20: commit-task echo.
36. T21: DLQPathFor cluster.
37. T22: fold-lock gitignore.
38. T23–T24: live-workstream items.
39. T25: cqrs-lint upstream.
40. T26: release hardening.
41. T27: freeze + window residue.
42. AGENTS.md Known Issues: css-minify class entry + `nix fmt` cure for
    treefmt/gofmt divergence + `TICK-CLAIM WARN:` literal for greps.
43. treefmt↔gofmt toolchain alignment (one formatter of record).
44. Re-run the four standing gates at HEAD for a fresh 4/4-green snapshot
    receipt post-css/treefmt.
45. Templ-generate idempotency check at HEAD (no other drifted generated
    artifacts — app.css was one; prove it was the only one).
46. `.tq-worktrees` empty per-repo parents: delete empty dirs in the reaper
    (residue from the L4 finding; small code change).
47. v0.3.5 candidate scan once CI green (O14: css fix is internal-only →
    `### Internal`, not operator notes).
48. Confirm CI's workflow_dispatch (F2) works with a scratch dispatch once
    green (dry-run never executed).
49. Check-test the 09-29 archived claim's sibling window (the same report
    cluster) for more 3+word-distance false claims once the regex widens.
50. 🔒 Owner rulings from §g below — unblocks 6, 8, 9, 14, 30, 17.

## g) Three questions I can NOT figure out myself

1. **Push authorization:** local master is 9 commits ahead (css fix
   `633a80c6`, treefmt fix, the greening batch). Both known nix causes are
   cured locally but CI-green is UNPROVEN until pushed. Push now, or hold
   for the fleet?
2. **CSS drift policy:** the guard detected but nothing healed — the
   expanded artifact sat through runs. Should a drifted generated asset
   auto-heal (CI/daemon commits the regeneration) or stay detect-only?
   (Determines item 14 and whether this class can ever re-ship.)
3. **Heal-format ruling (carried from 16-57 §g Q2, now sharper):** for
   dangling/false historical claims — the 09-29 report's false "row 143
   ticked" and the 769 dangling-cite baseline — do we annotate archived
   reports in place (end-of-file appendix, never rewrite), grandfather
   them behind a baseline row, or heal prose? One ruling covers F8+F9
   routing for ~100 items.

---
Gate receipts this session: `nix build .#checks.x86_64-linux.treefmt` rc=0;
`scripts/check-webui-css.sh` rc=0 on the committed minified css (post-
`633a80c6`) AND rc=1 on the pre-commit expanded css
(`/tmp/tq-gates/css-guard-on-expanded.log` — proves 50267's failure class
and that the cure is complete, not style-stripping); harvest module
`TestRedispatch…` rc=0; GH jobs: 40205050267 nix=css-drift, 40207620242
nix=treefmt, all 7 other jobs success on both
(`/tmp/tq-gates/ci-40205050267-failed-jobs.log`).
Verdict channel: no queue task attached; `TQ_RESULT_FILE` unset this
session — no `tq verdict` recorded (nothing to attribute queue-side).
