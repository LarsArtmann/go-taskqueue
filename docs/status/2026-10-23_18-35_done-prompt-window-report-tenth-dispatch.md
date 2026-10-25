# Done-Prompt Window Report — 2026-10-23 18-35 CEST (tenth dispatch)

Task: 000001a16972732136dc80eb6e3a00000000 (same done-prompt re-dispatch;
the ninth pass at 18-09 already re-verified the window — this pass adds
footer-precise commit receipts the ninth missed, corrects its headline
error, and runs the docs-health delta). Window: eight tasks dispatched
2026-10-20 02:27 → 2026-10-21 03:37. Every §a claim re-verified at HEAD
56e62fbb from the closeout reports, `git log --format=%B` footer greps,
and the live tree. Docs-health delta folded in: ninth-dispatch §c
inline-corrected, index row added, 1 TODO row appended (new defect
class), no archive moves (dispatch-chain archive already rowed).

## a) FULLY DONE (verified, not claimed)

1. **art-dupl templ-directive issue FILED with a narrowed premise**
   (task 000001a156a33bbf…, closeout 02-27, commit f1cbb912 + siblings).
   The row's original premise was outdated: the accept regex matches
   anywhere in a line, so an HTML comment DOES suppress a templ-markup
   clone pair (empirical control pair: 1 group shown without, 0 with).
   The REAL residual — the suppression comment ships in rendered page
   output — filed as art-dupl #9; draft at
   `docs/drafts/2026-10-20_02-11_art-dupl-templ-directive-issue.md`.
   Config-pinning follow-ups landed: `scripts/art-dupl.json` +
   `check-mirror-clones.sh -c` invocation (41cd3a19, parity proven:
   27 groups / 3 shown both ways); mutation-verify of the
   `old_priority` wire-key pin (009ec7ff, FAIL transcript archived to
   `docs/status/assets/tq-000001a156a33bbf-evidence/` with SHA256SUMS);
   row 637 ticked. All receipts present at HEAD.
2. **Directive provenance pass + status-index bloat** (task
   000001a156ba0ad2…, closeout 02-56, commits 45f26cf3 + 10e4e86d).
   The 12-27 dupe-type verdicts ledger was re-read in full and proven
   DISJOINT from the window's 14 statement-clone reasons; all 42
   surviving window directives now cite the sweep report's §a3 ruling
   (first pass missed 18 indented directives; second pass
   count-verified). Index pruned 324→304 live rows; threshold
   re-derived at 4.2 rows/day and raised to 400 with the derivation
   pinned. Foreign break fixed en route: cmd/tq floor 1.27.1→1.27.2
   (dc0f8bbb). Rows 640+641 ticked.
3. **Metaengine Delta counter Stats surface** (task 000001a156ddd5ca…,
   closeout 04-21, commits 797f3938 + db241bc0 + b5d8cabf).
   `status_counts`/`project_status_counts` counter collections fold the
   same fact stream; `Model.Stats`/`StatusCounts`/`ProjectCounts` read
   them via `ExecuteTyped`; the GROUP-BY pushdown path DELETED incl.
   dead residue; transition-context prev-read keeps rescue/dismiss
   exact (`TestStatsParityLifecycle` unchanged-green); counter-version
   replay guard + `TestDurableCursorCountersExactAfterResume`.
   `/api/v1/healthz` projection section shipped with
   `TestHealthzCarriesProjectionSection`. Rows 655-657 ticked;
   CHANGELOG entries standing.
4. **Single-writer fold lock + cursor pagination** (task
   000001a1581c8dd0…, closeout 10-55, commits 3191fbf0 + 355dbe6b).
   Non-blocking flock on `<home>.fold-lock` with read-only fallback
   mount (serve/api/doctor wired; pinned by
   TestFoldLockElectsSingleFolderOverOneHome et al., all -race green);
   `Model.TasksPage` keyset cursor round-trip with malformed-cursor
   FAIL (TestTasksPageCursorRoundTrip). CHANGELOG/AGENTS/
   DOMAIN_LANGUAGE entries standing; rows 658/659 ticked.
5. **Canonical gate-receipt rule** (task 000001a1589024e8…, commits
   cce6e190 + acae4052, footer-verified this pass): AGENTS.md
   Conventions bullet — every "N ok / M fail" receipt is
   `grep -c '^ok:'` over a CAPTURED gate-output file — with the
   21,100 size-budget reset in BOTH twins (the script twin had lagged
   at 20,000; the reset also fixed that pre-existing twin-drift red).
6. **Dependabot parity at module creation** (task 000001a158b88f39…,
   commits 680d4387 + 34fe475c): `scripts/new-module.sh` appends the
   module's dependabot.yml gomod block (duplicate/missing-config
   refusals, scratch-repo smoke, wired into ci-local cheap gates);
   34fe475c adds the heal-tool dangling-cite report on rewritten SHAs.
7. **Quiet-host ci-local task — THREE footer commits the ninth
   dispatch missed** (task 000001a158f55c1c…). Footer-precise
   `git log --grep 'Task-Queue-ID: …f55c1c…'` finds: 8b1efebd (the
   AnswerPoller oldest-first drain fix — removes the four-day red
   battery leg; rows 323+504 ticked), ed38bfb4 (root vendor-drift row
   closed, gate verified green), cb5ce139 (release.sh consumer-path
   proof refinement + RELEASE.md Phase 3 step). The ACTUAL deliverable
   — one full quiet ci-local run with the archived matrix log (TODO
   row 678) — is still UNCHECKED and no matrix-log artifact exists
   under docs/status/assets/, so the row stays open honestly; but the
   ninth dispatch's §c headline ("completed with NO commit/report/tick")
   is factually WRONG on the commit half and is inline-corrected in
   that report this pass.
8. **webui-shots ruling + canary single-sourcing + fixture-attribution
   gate** (task 000001a15c06ea39…, closeout 03-37, commits 7b3ada22 +
   3d17d769 + f5ac738c/3b7a8847/62f111c1 + deccc465): shots harness
   ruled productized at scripts/webui-screenshots.sh (NOT smoke/, with
   the nix-chromium recipe in the no-browser error); gosec canary list
   single-sourced via `for-each-module.sh --canaries` with
   empty-derivation hard-fail; `check-fixture-attribution.sh` shipped
   with self-test, 17 existing fixture heredocs annotated, wired into
   ci-local. 63-ok captured battery at deccc465. Rows 776-778 ticked.

## b) PARTIALLY DONE

1. **Row 678 (the ci-local run itself)** — see a7: enabler work landed
   (the known red leg fixed), the run + archived matrix log did not.
   The ninth dispatch correctly kept the row open; this pass corrects
   only its "no commit" claim.
2. **Non-idempotent-counter hazard** was shipped with the counters
   (04-21 closeout b1) and then CLOSED by the fold lock same window —
   residue: the upstream go-cqrs-lite counter-key parent-row finding
   (row 661) is repro'd in-repo but not filed upstream;
   transition-helper unit tests (row 662) unwritten.
3. **Provenance coverage**: the 42 window directives carry citations;
   ~18 non-window accept directives still carry local rationale only
   (no directive-class ledger index — rowed by the 02-56 §f5).
4. **Fixture-attribution enforces existence, not accuracy** (canary
   token layer rowed as row 1006); scan scope excludes smoke/ and
   printf-built fixtures.
5. **TasksPage has zero production consumers** — the webui row-rich
   ruling question is BLOCKED on the owner (ninth-dispatch §g1, rowed).
6. **Window verification batteries were scoped per session** (script
   gates / touched modules); full root `-race` + ci-local at a single
   post-window HEAD has not been run since the AnswerPoller fix —
   row 678's run would cover it.

## c) NOT STARTED (by this window; standing rows own them)

1. CI setup-go pin bump 1.27.1→1.27.2 (rowed in the ninth section).
2. Dispatch-chain archive-or-annotate sweep (seven 2026-10-20 dispatch
   reports; rowed, deliberately not executed here — each needs full
   inline ANNOTATE resolution before `git mv`).
3. art-dupl #9 upstream resolution tracking (rowed).
4. `tq stats --read-model` read-only/lag body line (row 663),
   injectable fold-lock wait (row 664), fold-lock chaos test, webui
   read-only banner (all rowed by the fifth dispatch).
5. Row 220 chromedp family, row 603 facade-module canaries (explicitly
   other rows' scope per the 03-37 closeout).

## d) TOTALLY FUCKED UP

Nothing new landed broken this pass (docs-only). Window-scope damage,
all dispositioned:

1. **The ninth dispatch's §c headline error** (the ci-local task "NO
   commit"): a footer grep against the WRONG precision (or skipped)
   hid three footer commits. Corrected inline in that report this
   pass — second specimen of the footer-grep blind-spot class (the
   10-55 closeout §d1 is the first; that one was footerless-landed
   work, this one footer-present-but-unmatched).
2. **The counter double-count hazard shipped before it was seen**
   (04-21 closeout d1) — closed by the fold lock, but the class
   (non-upsert fold kinds silently change concurrency requirements)
   is only guarded for counters; sets/logs ADTs unaudited.
3. **Edit-tool phantom successes** (02-27 closeout d1: two "Content
   replaced" reports with zero disk delta) and the **anchor-edit
   near-deletion of rows 658/659** (04-21 closeout d3) — both caught
   by verify-on-disk/adjacent-row greps; the habits are recorded in
   the closeouts but not yet mechanical.
4. **Daemon-sweep heals remain the expensive path** (five heal/amend
   maneuvers across the window's sessions; all recovered cleanly,
   fork records pasted).
5. **New defect found in passing (this pass)**: FIVE lines in
   TODO_LIST.md carry TWO glued `- [ ]` items on one line (missing
   newline — e.g. the SECURITY-matrix row at line ~468 ends with
   `…(22-37 report §f1/§b1)- [ ] Pin…`). Every glued second item is
   invisible to one-item-per-line harvest and to check-todo-list.
   Filed as a new TODO row this pass (de-glue sweep); NOT fixed here
   (ticking is the only edit allowed on existing items).

## e) WHAT WE SHOULD IMPROVE

1. **Footer-precise grep is the re-dispatch check**: `git log
   --format=%B --grep 'Task-Queue-ID: <id>'` (exact), plus the
   file-signature probe (`git log --oneline -5 -- <paths>`) the 10-55
   closeout asked for — both failure specimens are now receipted.
2. **TODO_LIST appends need a trailing-newline assertion**: the glued
   line class is born when an append lands without the separator
   newline; a check-todo-list leg (line matches `^- \[.\] .*$` exactly,
   flagging embedded `- [ ]`) kills it mechanically.
3. **Non-upsert fold kinds need a design gate**: "what happens when
   TWO pumps fold this?" belongs next to the metaengine adoption docs
   (the fold lock covers counters; the general contract is unstated).
4. **Change sessions should run the vendored-sync gate immediately
   after dep bumps** (the 10-55 §d2 45-minute latent red).
5. **One full quiet ci-local run per window family** (row 678) would
   have retired the whole "scoped gates by prose ruling" streak with
   one artifact — it is the cheapest standing gap.

## f) NEXT THINGS (dedup-checked against the live backlog; new rows

appended this pass are marked ★)

1. ★ De-glue the five double-checkbox TODO_LIST lines (and add the
   check-todo-list leg that flags embedded `- [ ]` on a single line).
2. Row 678: the quiet-host ci-local run + archived matrix log — the
   red leg is fixed, the run should now go green.
3. Row 661: go-cqrs-lite counter-key parent-row clean-room repro, then
   file upstream (verify-before-filing first).
4. Row 662: transitionDelta/projectTransitionDelta table tests.
5. Rows 663/664: `tq stats --read-model` read-only+lag line;
   injectable fold-lock wait.
6. CI setup-go pin bump to 1.27.2 (ninth section row).
7. Dispatch-chain archive-or-annotate sweep (ninth section row).
8. Row 1006: fixture-attribution v2 canary-token accuracy layer.
9. Row 220: chromedp browser-level smoke assertions.
10. Row 661-sibling: audit OTHER non-upsert fold surfaces (sets/logs
    ADTs) for the multi-folder hazard — one grep + one design
    paragraph (02-56…04-21 closeout §f8, already rowed family).
11. Directive-class → ruling-doc ledger table under docs/ (02-56 §f5).
12. Row 305/264: span-splitting attribution tooling for multi-item
    daemon folds (second specimen receipted).
13. Doctor check: completed tasks with no footer-linked commit and no
    verdict marker (needs owner go for default-on).
14. Fold-lock chaos test: kill -9 the lock holder, assert the next
    opener wins (fifth-dispatch row).
15. `.golangci-baseline.txt` deliberate regen (00-31 §f6 residue).

(15 listed; deeper backlog lives in TODO_LIST.md — nothing here
re-lists an existing row's work except as pointer.)

## g) QUESTIONS FOR THE OWNER

1. **Row 678 ownership**: should the quiet-host ci-local run be an
   agent dispatch (it is machine-runnable, ~30 min) or an owner-run
   artifact at push time? Three dispatch generations have now routed
   around it without executing it.
2. **Glued-TODO-line repair**: the five glued lines predate this
   window and editing existing items is append/tick-only — does the
   de-glue sweep get an explicit owner carve-out to split those lines
   (restoring the intended one-item-per-line form), or should each
   glued second item be re-minted as a fresh row and the glue annotated?
3. **TasksPage consumer ruling** (carried, BLOCKED row exists): do the
   webui row-rich views adopt readmodel pagination when
   `--read-model` is on, or does store-backed stay final?

## h) BAND DRIFT

`tq facts --type task.reprioritized` over the window timespan returns
**(0 facts)** — none recorded (tenth consecutive done-prompt census
with zero reprioritization facts; the ADR-0015 journal carries no
priority moves for this window).

## Verification receipts

- Footer greps (footer-precise, `git log --format=%B --grep`): all
  eight task IDs resolved to commits; the f55c1c triple
  (8b1efebd/ed38bfb4/cb5ce139) is this pass's delta over the ninth.
- Closeout reports read in full: docs/status/tasks/2026-10-20_{02-27,
  02-56,04-21,10-55}_task-_.md, 2026-10-21_03-37_task-_.md.
- CHANGELOG [Unreleased] covers the window's user-visible surfaces
  (fold lock, TasksPage, healthz section, dependabot emitter, heal
  dangling-cite, dupe ledger + gate, AnswerPoller fix) — no gaps
  minted this pass.
- AGENTS.md receipt-rule bullet standing (Conventions); size-budget
  twins at 21,100 both green at HEAD (guard green in the ninth's
  battery).
- Ninth-dispatch §c corrected inline this pass (see that file's
  correction note).
- TODO_LIST: 1 new row appended (glued-line de-glue), dedup-checked
  against all 532 unchecked rows.
