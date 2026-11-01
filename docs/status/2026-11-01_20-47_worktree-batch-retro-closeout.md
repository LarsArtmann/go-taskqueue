# Worktree auto-merge batch — retroactive closeout (the window's missing a)-g) report)

Session 2026-11-01 ~20:47 (retrospective). This report closes the debt
flagged by docs/status/2026-10-13_02-13_done-prompt-release-docs-worktree-csp-window.md
§b4: the 2026-10-13 12:26 worktree batch was the ONLY window task set
without an a)-g) closeout report. The work itself ran 2026-10-13
(undispatched window; the auto-commit daemon absorbed the files).
Everything below was verified per-commit BEFORE any claim — per the
row's own instruction.

## a) FULLY DONE (verified at 2026-11-01 HEAD, not from memory)

- **All seven batch commits exist and their features are live at HEAD:**
  - `1d202130` feat: worktree auto-merge — `Sweeper.mergeOnApprove`
    (internal/review/sweep.go:208): an approve verdict on a review whose
    payload names a `tq/` delivery branch merges it into base with git
    plumbing (no checkout, base drift tolerated — daemon commits master
    mid-task) and reaps the checkout + branch. The O11 ruling's
    verify-gate-green half is STRUCTURAL, not checked: a red gate never
    completes the task, so approve is the last unmet condition (doc
    comment states this). A conflicting merge refuses untouched — the
    branch stays the delivery for a human. Pinned by
    internal/review/sweep_test.go.
  - `80a88804` feat: worktree orphan reaper — pool-tick sweep of dead
    `tq/*` remnants (`git worktree list --porcelain` + branch prefix +
    prune), parked-closeout worktrees survive via in-flight detection.
    internal/executor/worktreereap.go + worktreereap_test.go (67 lines).
  - `01d91bd2` fix: retry attempts ADOPT the existing `tq/<id>` branch
    (the suspected collision-class bug from the 00-46 report §g1:
    attempt 2 re-entering Execute used to Permanent-refuse on the
    existing branch — adoption was ruled over removal). Pinned by
    worktree_test.go.
  - `a0833318` feat: webui agent-run card renders worktree delivery
    forensics (WorktreeDir/Branch where session/cost render;
    fragments.templ + generated fragment go).
  - `6430ee11` feat: `tq doctor --hygiene` counts orphan `tq/*`
    remnant(s) (cmd/tq/doctor.go:969 today) — pre-reaper visibility.
  - `271335e1` feat: pool-start warning when `--worktrees` combines
    with `--project-exclusive` (exclusivity neuters worktrees'
    intra-repo parallelism; warn, don't refuse — agentpool.go:887
    today).
  - `edb9c44c` docs: AGENTS.md payload-contracts `--worktrees` bullet
    (AGENTS.md:126 today) + agent-executor FEATURES row.
- **All slice rows ticked** (old numbering 462/463/466-470 = today's
  TODO_LIST lines 553-561, all `- [x]`), and the UMBRELLA row 178 —
  stale-open with every slice done — is ticked in the same change as
  this report (tick-audit rule: landed + unchecked = paid round-trip).
- **Feature survives at HEAD**: every file named above was re-grepped
  at 2026-11-01 HEAD; none was refactored away by the three weeks of
  subsequent waves.

## b) PARTIALLY DONE

1. **The report is 19 days late** — the window itself never wrote one.
   This report reconstructs from commits + current source, which is
   weaker than a same-day report written with the run's own receipts
   in hand (no battery log, no live gate rc from 2026-10-13 survives;
   the claims rest on today's per-commit verification instead).
2. **The verify-green structural argument was re-derived, not
   re-witnessed**: today's sweep.go:200-207 doc comment carries the
   reasoning; the original window's own words are gone with its
   missing report.

## c) NOT STARTED

- None for this batch. (Follow-on worktree items — smoke redesign,
  dirty-heal tolerance — were separately rowed and landed in later
  windows: row 556's 2026-10-26 smoke re-run, row 184's 2026-10-31
  heal fix.)

## d) TOTALLY FUCKED UP

1. **The window shipped seven features and no report.** Whatever the
   cause (session death mid-closeout is the standing guess), the debt
   survived three docs-health harvests because the row pointed at
   line numbers (462-470) that drifted as TODO_LIST grew — the 02-13
   report §b4 flag kept naming positions that no longer resolved to
   the worktree rows. A pointer by CONTENT (commit shas) would not
   have drifted.
2. **The umbrella row stayed open with every slice closed** for the
   same 19 days — the tick-audit rule exists precisely for this
   class and was not run against row 178 by any of the intervening
   sessions (including the harvests that ticked the slices).

## e) WHAT WE SHOULD IMPROVE

1. Closeout reports reference WORK BY COMMIT SHA, never by TODO_LIST
   line number — lines drift, shas do not.
2. An umbrella row whose slices are all `[x]` is a docs-health WARN
   candidate: "children all done, parent open" is mechanically
   detectable at harvest time.
3. A window that lands features owes its report BEFORE the next
   window starts on the same files; the 02-13 window noticed the debt
   but wrote §b4 instead of the report itself (flagging is not
   closing).

## f) NEXT THINGS

1. None owed by this batch. The standing worktree adjacent items
   (ROADMAP open question 7 — forensic retention window for reaped
   worktrees — and the `--worktrees` dogfooding decision) are already
   rowed elsewhere.
2. Consider the umbrella-orphan detector from §e2 as a cheap
   docs-health check (not rowed yet; small).

## g) QUESTIONS FOR THE OWNER

- None. Factual retro; no decision was encountered that this session
  could not make from the recorded rulings (O11, adoption-over-removal).

---
*Indexed in docs/status/README.md (top chronological cluster).
Retro-verification receipts: `git show <sha> --stat` over all seven
commits + HEAD greps recorded in §a (sweep.go:208, worktreereap.go,
doctor.go:969, agentpool.go:887, AGENTS.md:126).*
