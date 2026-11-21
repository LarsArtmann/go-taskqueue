# Done-prompt window report — 2026-11-21 02-21

Window: 16 tasks completed 2026-11-19 22:11 → 2026-11-20 05:51 CET. Dispatch-time dedup
check: docs/status/README.md index rows + window reports grepped for the window span and
all 16 task IDs — only per-task closeouts exist, NO same-window done-prompt report — so
this is a FULL pass, not a verification-only pass. All 15 closeout reports were read
first (the 16th task, `…f5fa2c7e`, has no closeout file; verified from git directly).

Verified at HEAD 2026-11-21 ~02:25 by grep/`ls` over the landed artifacts (receipts
inline per section). Journal reads: `tq facts -type task.reprioritized` → 0 facts
journal-wide; `task.dead-lettered` → 6 facts, all 2026-09-07/08, none in-window.

## a) FULLY DONE

All 16 window tasks landed their dispatched scope; every headline artifact re-verified
at HEAD this pass:

1. **io-surface write-non-atomic triage** (`…a1f5a045`, 22-11): all 4 ERROR sites made
   atomic temp+rename — `cmd/tq/bootstrap.go` installService ×2 + `cmd/tq/verdict.go`
   via the new `cmd/tq/atomicwrite.go` `writeFileAtomic` helper (verified: file exists,
   2+1 call sites), `sqlitev4` migration `restoreOnConvergeFailure` inline.
   `io-surface lint --enforce` rc=0; sqlitev4 + cmd/tq gates green; commits 5e8aaacd +
   af7adc81, both healed from footer-less daemon sweeps and footer-attributed.
2. **Webui budget meter reachability** (`…f5d0d122`, 23-11): `tq serve --daily-budget`
   flag wired into `webui.Config.DailyBudget` (previously dead config — no flag ever set
   it); meter a11y `role="meter"` + aria values in `internal/webui/fragments.templ`
   (verified: 1 hit); `budgetMeterPct` shared helper; CHANGELOG Added entry (feb2650c);
   commits f70468bc/feb2650c/5a6b9b2c.
3. **T24 erraudit sibling session** (`…f5e338ae`, 23-33): verified ALREADY-LANDED
   upstream — issues #12/#13/#14 closed 2026-11-03, master synced with origin/master;
   nothing owed; row ticked with the checkout evidence. Same session also landed (its
   batch): go-cqrs-lite corpus triage 709324f82 (195→53 KEEP/141 archive/1 POOL) +
   execution 0f0d8a74f (archived count 1,278→1,419); the check-verify family d5d2df6c
   (`--self-test` 3 red-path shapes, `--list`, ci-local pin, 179 ms timing); the
   stub-shebang gate 9d30192f whose FIRST live run caught two REAL exec-format-error
   stubs in the session-start smoke; full root-gate GREEN.
4. **tq-bash-guard commit-message FP** (`…f5fa2901`, 23-58): root-cause fix 010eb231 —
   the command-position smoke matcher strips quoted strings (awk `gsub` double + single,
   verified scripts/hooks/tq-bash-guard.sh:442-450) before the token scan, so a smoke
   path inside a `git commit -m` MESSAGE is text, only an executable smoke invocation
   denies; self-test 65 green incl. 2 FP-regression pins. Same session: footer-attribution
   default span scoped to the nearest foreign footer (354194b7, `nearest_foreign_footer`
   verified ×4); go-cqrs-lite POOL report harvested 7e83f4f1d (12 rows, 13/13 resolved)
   and its 300 broken archive links repointed 8828ea837 (check-doc-links 0 broken/1471);
   stub-shebang negative pins ed6511f4 (comment-opener exclusion + `<<-` dash fix).
5. **Reviewer fix — templ skew** (`…f5fa2c7e`, commit 58215143, no closeout report):
   `fragments_templ.go`/`layout_templ.go` regenerated with the PINNED templ v0.3.1100
   after a prior session had used the host's unpinned v0.3.1123 — verified at HEAD:
   both files carry `// templ: version: v0.3.1100`.
6. **Reviewer fix — false app.css claim** (`…f5fa2c9f`, 00-31): the 23-11 closeout's
   claim that app.css was committed corrected in place (no CSS change existed); heal
   51148de8 footer-attributed after the standard `--from` terminator refusal.
7. **Reviewer fix — over-cap aria pin** (`…f5fa2cbc`, 00-38): spent>cap case pinned in
   `TestBudgetCardRendersFromSnapshot` — raw `aria-valuenow="9"` beside clamped
   `tq-meter-fill-100` + over tone (verified: test line 1241); the lying
   `budgetMeterPct` doc comment ("share the pct") corrected to state the deliberate
   divergence; commit b1c7b42e.
8. **Reviewer fix — missing BLOCKED suffix** (`…f62396fc`, 00-42): the unfinished
   T33/T34 row now carries `— BLOCKED: remaining rows are owner-gated or separate asks`
   (verified at TODO_LIST; commit 874f993e) — the loop-closing contract holds for it.
9. **Reviewer fix — load-flaky timing pin** (`…f6239758`, 00-47): check-verify
   `--self-test` timing budget raised 1000→3000 ms (cold host measured 1226 ms) with
   the WHY recorded in-script; budget raised over retry so genuine slowdown drift still
   reds (verified: "3000" ×5 in scripts/check-verify.sh; commit 9df85a65).
10. **Pareto-residue residue batch** (`…f63a34ef`, 01-14, 5 items): stub-shebang
    `#!/bin/sh -e` policy pinned LEGAL (one interpreter arg; `is_sh_shebang` classifier,
    66931e9d); root-gate tees full output to a dated /var/tmp receipt (ec350779, full
    run 21 ok/0 FAIL); session-start smoke PASS(3) against scratch TQ_DB (ee98189c);
    check-tick-claims over the 11-19 ticks: ZERO batch WARNs, 369 ok/35 pre-existing;
    pareto-residue section header annotated with the landed SHAs (7e0bb6f2).
11. **Pareto-residue batch 2** (`…f65ed39d`, 02-08, 5 items): DONE-adjacent-marker
    tripwire ruled NO-WARN (5/5 live hits genuinely open — shape-alone would be 100%
    FP; three quiet fixtures, 22572bd9); flags-form misplaced negative fixture
    (1ecd0516); root-gate receipt rotation, keep=20 (4216ebaa); ci-local receipt parity
    via shared `scripts/lib/receipt.sh` (3ecfabb4, verified); tick-claims baseline
    ratchet pinned at 35 warns, ratchet-down-only, with the ONE real finding
    (Adoptions-graph false "ticked" claims) resurfaced on its row (1b220e88).
12. **Root-gate battery re-run** (`…f6837d10`, 02-28): full root-gate.sh GREEN at HEAD,
    21 ok / 0 FAIL counted over the CAPTURED receipt (the `^ok:` convention-pattern
    mismatch vs go-test's `ok  ` shape documented as a finding); row 1559 ticked
    (04eb7a73).
13. **Durable-evidence + tick-claims + triage batch** (`…f6b18889`, 04-31, 5 items):
    22 dead `/tmp/` DONE-note cites re-anchored to re-runnable commands, the one
    surviving receipt archived with SHA256SUMS to
    `docs/status/assets/2026-11-20_tick-claims-live/` (verified: dir + SHA256SUMS
    exist); `check-tick-claims --receipt` (b570ddce); STALE-BATCH-HEADER advisory rail
    (d75f0620, verified in gate); stub-shebang flags edge settled by a LIVE exec probe —
    `#!/bin/sh -e -x` FAILS fork/exec on this host, classifier tightened to ONE flag
    cluster (9d4859a3); T21 webui SSE cluster triage (~80 POOL / 33 KEEP / obsolete,
    4 clean-obsolete rows ticked, e7ba0eaa).
14. **df preflight + digest sweep** (`…f7087efd`, 05-01): ci-local refuses the battery
    when /tmp free < 2 GiB (`TQ_TMP_FLOOR_KB`, verified in scripts/ci-local.sh ×4) —
    kills the ENOSPC wall class (6b8f3c42); the 2026-11 monthly ANNOTATE+ARCHIVE digest
    ritual executed: **19 reports archived** (12 DUPLICATE dispositions + 7
    fully-resolved 2026-10-02), index rows repointed, `digest 2026-11` row accrued,
    archive counter 616→635, live rows 427→408.
15. **Dedup-receipt pins** (`…f723ac6c`, 05-27): AGENTS §Conventions
    "greps-with-receipts" dedup procedure pinned (fb2d76eb); DEDUP-CLAIM-NO-RECEIPT
    advisory in check-todo-list.sh (7279cfbc, `--self-test` 22/22, verified); citation
    check resolved 9519eb06 as a rebase-dangling byte-identical twin of 57b76be6.
16. **Done-prompt residue batch** (`…f73a8d97`, 05-51, 5 items): budget-meter unified
    table test `TestBudgetMeterUnifiesPctFillTone` (3261bcb7, verified ×2); serve
    flag→Config seam `newServeFlagSet()`/`config()` + `TestServeFlagParseWiresConfig`
    (f0ca9d29); templ header gate `scripts/check-templ-header.sh` + ci-local wiring +
    AGENTS rule (8e0f0906, verified ×3); live budget-meter smoke on a scratch TQ_DB
    (ee24ed87, verified); `webui.Config` dead-field audit → the NEXT dead field
    `Config.Metrics` cured with `--health-metrics` + reflection parity test
    `TestServeConfigFieldParity` (21c002bb/62169f1b). AGENTS size guard back green.

Post-window residue (ticked after the window closed, noticed while verifying): the
serveOpts.config(version) purity fix + the serve flag-list pin row landed d73ad758/
b6332620 (rows 2014-class).

## b) PARTIALLY DONE

1. **go-cqrs-lite `2026-10-23_17-20` POOL report**: harvest complete (13/13 boxes
   resolved) but the `git mv` into its `archived/` is deliberately the NEXT pass's job
   (harvest-before-archive); its live-report count stays one above target.
2. **T33/T34 residue** (TODO_LIST 1715): BLOCKED suffix landed (reviewer fix); the rows
   themselves remain un-drained — overwhelmingly owner-gated (crush #3146, CI gosec
   ruling, spend gates, push-gated ref drops).
3. **T21 obsolete half**: 4 of ~10 provisional-obsolete rows ticked; the fuzz.yml,
   TQ_HOOK_ADVISORY_LOG, and hook-fp-triage rows still owe their one-command premise
   re-checks (deliberately not tick-on-faith).
4. **ci-local receipt parity**: verified at wiring level (5-second capture), not by a
   full real battery run — rowed (TODO_LIST 2049).
5. **Status-index bloat**: 409 live rows vs the 400 threshold even AFTER the 19-report
   digest sweep (this report's own index row adds one). The ritual row landed; December
   accrual must target the un-struck §f middle (~215 files), not just duplicates.
6. **AGENTS.md headroom**: 41 B after this pass's fleet-convention line extension
   (check-stub-shebang + check-tick-claims added to the pinned-gate list; it was at
   80 B, was 7 B mid-window). Every future AGENTS edit needs a prune-first reflex.
7. **erraudit upstream**: #11/#10/#3/#2 remain open — their premises are unverified the
   same way #12-#14 turned out stale.
8. **11 dangling SHA cites** in foreign 2026-11-19/20 reports (daemon-sweep mid-window
   SHAs the later heals orphaned) — rowed this pass, not yet cured.
9. **check-tick-claims baseline 35** lives as an in-script env default, not a committed
   pin file — the rail is self-tested, the number is not diff-reviewable (02-08 §b).

## c) NOT STARTED

Nothing dispatched in this window was skipped. Standing backlog the window did not
reach (and this pass rows or confirms rowed):

- INFO-level write-non-atomic sweep (4 sites: ensureTQVerify ×2, ensureCrushConfig,
  cmdAsk marker) — **rowed this pass** (was report-only since 22-11).
- `--health-metrics` live smoke leg — **rowed this pass** (parity/unit-pinned only;
  the exact dead-config class DailyBudget had).
- Budget-meter zero-spend + cap=0 rendered-fragment cases — **rowed this pass**.
- check-verify `--self-test` positive control (vacuous-pass guard) — **rowed this
  pass**.
- Dead-sha census re-run over the window's 11 new dangling cites — **rowed this pass**.
- Receipt-header self-identification (HEAD sha + dirty/clean; FLAKE-RETRY count line) —
  **rowed this pass**.
- December digest accrual targeting the un-struck middle (~215 files) — standing row.
- STALE-BATCH-HEADER quiet/summary mode (64 multi-line warns per run) — rowed 1743.
- Receipt-less dedup-header backfill (76 headers) + receipt-shape FP triage — rows
  1897/1898.
- The ~100-commit unpushed master window — owner-gated (blocked row refreshed this pass).

## d) TOTALLY FUCKED UP

No regressions at HEAD: root-gate GREEN receipts twice in-window (23-31 full battery;
02-28 re-run 21 ok/0 FAIL), status-index + TODO_LIST gates green at every landing, and
this pass's re-verification found every headline artifact present. The damage class of
this window is process cost, not broken state:

1. **The templ-skew incident chain** — a session regenerated `*_templ.go` with the
   host's unpinned templ v0.3.1123, breaking the pinned-CLI contract and dragging an
   unrelated diff into `layout_templ.go`; one paid reviewer rejection + one restore
   commit (58215143); then the cure (check-templ-header gate + AGENTS rule). A full
   regen-diff-freshness failure that took three separate pieces of work to close.
2. **Four paid reviewer rejections in one window** (app.css false claim, missing aria
   pin, missing BLOCKED suffix, load-flaky 1000 ms timing pin) — all five fix sessions
   landed cleanly, but three of the four findings are report/doc-drift classes a gate
   could catch (see e2).
3. **Daemon-sweep heal tax: ~15 heal/fork-record round-trips across the window** —
   every session lost the edit→commit race at least once (05-51 paid 4; 04-31 4 sweeps;
   02-08 an 8-commit re-fork; 22-11 twice; 23-11 twice; 01-14; 05-27; 00-31; 00-38).
   The heal tooling worked every time, but the <60s daemon vs LSP-paused edit windows
   is now the dominant recurring cost of paid work here.
4. **`lsp_replace_symbol` truncated `cmdServe` mid-function** (05-51 d1) — dropped the
   function tail, broke the build; recovered from `git show HEAD:`. A whole-symbol
   replace on a LARGE function is a tool hazard: use exact-block edits or split first.
5. **AGENTS pruned under dispatch pressure** (05-27 d1): 5 edit rounds to discover the
   file was 523 B over, then 4 doctrine bullets pruned; a bad multiedit leg nearly
   deleted the rebuild-lineage line. The file sat at 7 B headroom until 05-51 and this
   pass refit it to 41 B.
6. **The /tmp-evidence class kept recurring within hours**: 01-14's receipts went to
   /tmp, 04-31's sweep then re-anchored 22 dead /tmp cites — the convention (cite
   durable classes or archive at capture) exists but was applied late.
7. **Assertion-first gate design, again** (23-33 d2): check-stub-shebang needed 3
   drafts (41 false violations, then 0 checked) before the corpus-first draft — the
   observe-first convention has now been re-learned in at least three windows.

## e) WHAT WE SHOULD IMPROVE

1. **Daemon-race economics**: 15+ heals/window. Concrete: (a) escalate the
   commit-before-gate hook from WARN to DENY on the second in-session offense (verify
   hook-session-state feasibility first); (b) make stage+commit-in-one-message the
   explicit default in tq-edit-advice for Go files (row 1774 exists — wire the advice);
   (c) teach commit-task.sh to fold a exactly-mine contiguous footer-less sweep
   automatically instead of failing on the missing file (23-33 f39).
2. **Reviewer-proofing the doc surface** — 3 of 4 window findings were drift classes:
   (a) `check-committed-file-claims.sh`: a file cited as "committed" in a status report
   must appear in the cited commit's `--stat` (00-31 e1); (b) DONE-row narratives that
   quote a pin CONSTANT get a "since raised" note convention so constants are not
   "restored" from fossilized rows (00-47 e1, row 329 still narrates 1000 ms);
   (c) machine-consumed rows must carry their own state (the T33/T34 lesson is now
   encoded — keep it that way).
3. **Self-test asymmetry reflex**: every ACCEPT fixture ships its paired REJECT fixture
   in the same change (01-14 e4) — the flags-positive landed without the
   flags-misplaced negative and burned a whole follow-up task.
4. **Corpus dump before checker draft**: grep the corpus shape BEFORE writing a gate
   (23-33 f50) — would have saved 2 of 3 check-stub-shebang drafts.
5. **AGENTS budget**: 41 B headroom — prune-first as a reflex (a tq-edit-advice row for
   AGENTS.md edits naming check-agents-size BEFORE composing), or grow the budget
   consciously (owner row 677 owns the policy; it is now acute).
6. **Smoke single-scene pattern**: a shared scratch-serve helper in
   `scripts/smoke/_lib.sh` (boot/wait/probe/teardown) so new serve legs cost only their
   asserts (05-51 e; rowed 2013).
7. **Timing pins share no helper**: every `--self-test` re-rolls EPOCHREALTIME/awk
   inline — a `scripts/lib/timing-pin.sh` (measure + budget + pinned message) ends the
   class, including the measure-under-the-right-interpreter hazard (00-47 d1 measured
   0 ms under mvdan/sh before re-measuring under bash).
8. **Advisory-noise budget**: 140 DEDUP-CLAIM + 64 STALE-BATCH-HEADER + 35 tick-claims
   warns per gate run — the rails are right, the volume trains ignoring; the backfill
   and triage rows (1897/1898/1743) need one sweep each.
9. **Receipt pattern mismatch**: the §Conventions count pattern `grep -c '^ok:'` counts
   0 on go-test's `ok  <pkg>` receipts — reconcile the convention text or the gate
   output (02-28 e1; rowed 1539-class).
10. **Living-doc reconciliation is still manual**: this pass found the CHANGELOG
    metrics entry claiming "the cmd/tq flag lands when a pool opts in" for a flag that
    had LANDED two days prior — a cheap docs-gates freshness-style check over
    [Unreleased] entries vs the flag surface would catch the class.

## f) UP TO 50 NEXT THINGS

Minted to TODO_LIST this pass (6 open rows + 3 blocked rows, all dedup-checked by
domain-noun greps — receipts in the TODO_LIST section header):

1. Sweep the 4 INFO-level write-non-atomic sites (fix or claims-`ephemeral` per site).
2. Re-run the dead-sha census over the window's 11 new dangling cites (fork records or
   dated baselines).
3. `--health-metrics` live smoke leg (200 + Prometheus-shaped body on scratch TQ_DB).
4. Receipt headers self-identify (HEAD sha + dirty/clean; FLAKE-RETRY count line).
5. Budget-meter zero-spend + cap=0 rendered-fragment cases.
6. check-verify `--self-test` positive control (unmutated tree rc=0).

Blocked rows (§g below): 7. over-cap aria ruling; 8. push-timing ruling; 9.
STALE-BATCH-HEADER drain strategy + N ruling.

Already rowed by the window's own closeouts (the pool's next food, not re-listed as
new): df-preflight `--self-test` negative battery (2042); full ci-local run through the
df preflight (2049); archived/README manifest for the 19 files (2044); per-field parity
test (2006); under-cap smoke leg (2007); scratch-serve helper (2013); aria-valuetext
evaluation (2016); TQ_SERVE_DAILY_BUDGET env parity (2003, BLOCKED); templ-header
exactly-one invariant (2010); unbalanced-quote guard pin (2020); STALE-BATCH-HEADER
quiet mode (1743); dedup-receipt backfill (1897) + shape triage (1898); stale-header
annotation sweep + DUPLICATE-canonical convention + df-guard extension to root-gate/
nightly/smoke libs + archived-row presence check + digest-row semantics (tail cluster);
the December un-struck-middle digest accrual (standing row).

Higher-value report-only residue worth a future pass: guard hook warning when staged
files span >1 module (22-11 f4); check-stub-shebang `--json` output mode (23-33 f24);
`heal-daemon-sweep --message` flag (22-11 g3); commit-before-gate WARN→DENY escalation
feasibility spike (04-31 e5); `check-committed-file-claims.sh` gate (00-31 e1);
timing-pin shared lib (00-47 e2); DONE-note constant-fossil note convention (00-47 e1);
per-report content verdicts for the go-cqrs-lite 53 KEEP window (23-33 e1); erraudit
#11/#10/#3/#2 premise sweep (23-33 f31); un-struck-middle corpus triage method reuse
(05-01 f41); webui a11y sweep for other raw-vs-derived aria divergences (00-38 f4);
second meter component registry note (00-38 f10); CHANGELOG release-cut pass over the
new Added entries (23-11 f10); serve banner mentioning the budget meter when set
(23-11); `writeFileAtomic` promotion to shared seams on a third cmd-side site (22-11
f7); index-bloat root cause (auto-digest trigger, 00-31 f4); env-neutral
`newServeFlagSet` variant against t.Setenv noise (05-51 e); check-templ-header missing
go.mod line message + header-count pin (2010 covers the count half); root-gate receipt
rotation already landed (4216ebaa) — verify the /var/tmp accumulation policy holds
after a month; keep-alive: none.

## g) UP TO 3 QUESTIONS

1. **Over-cap `aria-valuenow`**: keep RAW spend (HTML meter allows now > max;
   informative) or clamp to `aria-valuemax` (stricter WCAG/AOM role=meter reading)?
   The 00-38 pin enforces the shipped raw behavior either way; only an owner ruling
   flips it. (Rowed BLOCKED.)
2. **Push timing**: the master window is now ~100 commits ahead of origin/master
   (27 at the 00-31 fix, grown since). Queue rules bar agents from pushing; every cure
   stays local-only until origin CI runs it. Push at the next batch boundary?
   (Rowed BLOCKED.)
3. **STALE-BATCH-HEADER policy**: drain the 64 standing hits by per-header SHA
   archaeology (citable) or bulk no-work/accepted-residue marker (hours cheaper,
   permanently weaker tick-claims)? And is the default age N=7 right, or should
   in-flight batches stay quiet until N=14? (Rowed BLOCKED.)

## h) BAND DRIFT

**None recorded.** `tq facts -type task.reprioritized` returns 0 facts journal-wide —
no priority moved in this window or any prior one (fourth consecutive window with zero
reprioritized facts; the 11-19 window report recorded the same). Companion journal
read: `task.dead-lettered` holds 6 facts, all 2026-09-07/08 — no dead letters in this
window, so no DLQ-priority churn either. The ADR-0015 accountability question is
therefore empty for this span: claim order moved only via stored priority + aging.

## Docs-health pass notes (this pass)

- Read all 15 window closeouts first; verified headline landings at HEAD by grep/ls
  (receipts inline in §a). No same-window report existed; full pass performed.
- **CHANGELOG**: cured a stale `[Unreleased]` claim — the Prometheus-metrics entry said
  "the cmd/tq flag lands when a pool opts in"; the flag (`tq serve --health-metrics`)
  LANDED in this window (21c002bb/6caaebdc), entry corrected to the shipped state.
  Internal tooling rows (df preflight, receipt lib, templ-header gate, guard FP fix,
  STALE-BATCH-HEADER/DEDUP rails) stay un-listed per O14 (repo-internal, not
  operator-visible).
- **AGENTS.md**: extended the countable-gate fleet-convention list with
  check-stub-shebang + check-tick-claims (both now carry `--self-test` and ci-local
  wiring; they were countable gates missing from the named list, 23-33 f19). Size
  guard green at 27559/27600 B.
- **FEATURES.md**: budget-visibility row updated — the dashboard meter is now
  reachable via `tq serve --daily-budget` with `role="meter"` a11y (verifiably shipped
  this window).
- **README.md**: no stale claims found (serve examples unchanged; no flag-list prose to
  drift). **ROADMAP.md**: no routed ideas from this window (all residue is bounded
  work, routed to TODO_LIST).
- **Archives**: none this pass — the ratified monthly digest ritual ran YESTERDAY
  (task …f7087efd: 19 archived, counter 635, digest 2026-11 row accrued); the next
  accrual is December's, targeting the un-struck middle. check-status-index green.
- TODO_LIST: 9 rows appended (6 open + 3 BLOCKED), 0 ticks owed (the window's own
  closeouts ticked their rows), all dedup-checked with inline grep receipts in the
  section header.
