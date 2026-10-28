# Done-Prompt Window Report (fifth dispatch) — 2026-10-20 11:09 CEST

> **DUPLICATE — ARCHIVED 2026-10-28 (row-1044 chain sweep)** — verify-only re-dispatch layer of the same 5-task window (2026-10-20 00:31–02:03, task 000001a156a3410fa257bc4a217c00000000); zero unique forward-looking items — every pass's row mints already live in TODO_LIST. The window's full a)–h) record is the canonical report `docs/status/archived/2026-10-20_02-31_done-prompt-window-report.md`.

Task: `000001a156a3410fa257bc4a217c00000000` — FIFTH dispatch. Passes 1–4
(02:31 `a7d19756`, 03:14 `a7e67248`, 09:05 `91ae96ca`, 10:26 `72853f1e`)
each committed a full report and each exited WITHOUT recording the
`tq verdict` — four paid claims, four complete reports, zero verdicts.
Per the resume convention THIS pass re-verifies the window at HEAD,
reports the delta since 10:26 (the readmodel rows-658/659 landing and
closeout by a concurrent session), and records the verdict as the next
command after the commit — the step this task has now failed four
consecutive times.

Window: 5 agent tasks completed 2026-10-20 00:31–02:03. The 02-31 report
is the window's full a)–h) record; 03-14, 09-05, and 10-26 are
verification layers one through three; this file is layer four.

## a) FULLY DONE — re-verified at HEAD (11:09)

1. **All twelve window commits intact** (`git cat-file`/`git show --stat`
   re-checked this pass): `70f62dd9` (lint-gate polish:
   regen-mode cache clean + `--clean-cache` + AGENTS drop-day doc + gosec
   pluralization), `d385faa1` (sessions-lamp CSS + `tq stats --since`
   windowed band + smoke pin), `c0fe4d38` (dlqfix-autopsy prune
   semantics pin), `2de0c2dd` (2026-09-14 webui-overhaul CHANGELOG
   coverage), `acc519da` (provenance comment corrected to the shared
   `harvest.BuildProvenance`), `b84528f6` + `fd95b61f` (accepted-dupe
   ledger + `.dupe-baseline.sh` gate wired into ci-local), `88bc4480`
   (f13 mirror-clones scope ruling in all three sweep.go files),
   `d1a062a8` (budgetUsageView golden pins), `85dbd9a2` (OnRecordTyped
   v4.17.0 citation), `a786a876` (maybeMint split, err113 sentinels),
   `3125a000` (done-guard reachable on rewind replays).
2. **Artifact spot-checks this pass**: `--clean-cache` present in
   scripts/lint-baseline.sh (lines 12/29–30) AND in AGENTS.md Commands
   (line 19); formatterWithGo/goTarball Known Issue in AGENTS.md (line
   290ff); scripts/dupe-baseline.sh + .dupe-baseline.txt +
   docs/dupe-type-accepted-ledger.md all at HEAD;
   `TestPruneStaleNeverTouchesDlqfixAutopsies` (2 hits,
   internal/harvest/prune_test.go); `internal/webui/render.go:81` calls
   `harvest.BuildProvenance` (no hand-mirror); f13 ruling comment in
   internal/status/sweep.go:100; `cmd/tq/stats_session_test.go` +
   `cmd/tq/budget_json_test.go` present.
3. **Living docs current at HEAD**: CHANGELOG [Unreleased] carries the
   sessions-lamp Added entry, the webui-overhaul Changed entry, the
   done-guard Fixed entry, the healthz projection section, the
   mint-time coordination gate, AND the rows-658/659 entries (fold
   lock, cursor pagination — landed by the concurrent 10-55 session).
4. **Five closeout reports** under docs/status/tasks/ (00-31, 01-10,
   01-28, 01-47, 02-03); all four predecessor done-prompt reports
   indexed (docs/status/README.md 2026-10-20 cluster).
5. **Docs gates green this pass**: check-status-index ok,
   check-todo-list ok, check-doc-refs ok (trailer WARNINGs are the
   pre-existing f26 cluster, owner ruling pending).
6. **Delta since the fourth pass, verified**: the concurrent session
   closed rows 658/659 — single-writer fold lock (3191fbf0) + TasksPage
   cursor pagination (355dbe6b) — with a full battery (rc=0), a
   closeout (docs/status/tasks/2026-10-20_10-55_task-…8dd057cb….md),
   CHANGELOG/AGENTS/DOMAIN_LANGUAGE updates, and the sanctioned
   AGENTS.md size-guard reset 20,600 → 20,900 (df126181, db28a4dc).

## b) PARTIALLY DONE

Unchanged from 10-26 §b, all still open and rowed:

1. Full `ci-local` never run with the dupe-baseline step in situ.
2. Done-guard reachability pinned only at Every=1 (row 630/631).
3. `.golangci-baseline.txt` deliberate regen still owed.
4. Dupe-gate tool provenance unrecorded (row 974 region).
5. **Floor pairing STILL broken, now ~10.5h**: `cmd/tq/go.work` line 12
   pins `go 1.27.1` while root and cmd/tq go.mods require `1.27.2` —
   re-verified live this pass. Row 955 owns the one-line bump; verified
   broken at 00-31, 01-47, 02-03, 10-26, and now 11-09 by independent
   sessions while every plain cmd/tq `go` invocation stays dead.

## c) NOT STARTED

- Carries from 02-31 §c: the webui-overhaul self-review §f siblings
  (capture strip, livecheck assertions, capture matrix, `data-state-key`
  guard test, JS harness, detail-page livecheck) — pre-tracked, untouched.
- No backfill sweep for CHANGELOG gaps beyond the audited window.
- The 10-55 session's remaining minted-but-unworked rows: 661 (upstream
  counter-key repro + filing), 662 (transition-helper tables), 660
  (BLOCKED owner design call), plus its two fresh appends (stats
  read-only surfacing; injectable fold-lock wait).

## d) TOTALLY FUCKED UP

1. **The dispatch itself, four times over**: task
   `000001a156a3410f…` has consumed FIVE paid claims — four committed
   full reports, zero recorded verdicts. The verdict-at-exit row (993,
   minted 09-05) did not prevent the fourth miss: the rail is prose in
   TODO_LIST, nothing enforces it at session exit, and each miss costs
   a full re-claim plus a verification pass. This pass treats the
   verdict write as part of the commit (next command after it).
2. **Floor blocker ages instead of closing** (b5): a one-line go.work
   bump, minted 00-31 §f1, verified-broken by six independent sessions
   over 10.5 hours. The queue's priority ladder has no "verified
   repo-broken" escalation — the one mechanism this section exists to
   surface produced no movement at all (see h).
3. History carried from the window's own §d entries (unchanged): the
   flattened foreign feat commit (00-31 §d2, third strike on the
   class), the near-miss vacuous dupe gate (01-28 §d1), the red
   done-guard test that sat observable for a window before its fix.
4. In passing (concurrent session, NOT this window): rows 658/659
   landed footerless via ~10 daemon folds and needed a re-dispatch leg
   to attribute/verify/tick — the footer-grep blind spot its own §d1
   documents. The attribution repair is structural, not complete (the
   span's tail sits under the foreign done-prompt docs commit
   72853f1e); disclosed in the closeout, rows 264/305 own the tooling.

## e) WHAT WE SHOULD IMPROVE

1. **Verdict-at-exit must become mechanical NOW** (row 993): four
   agent passes in a row ended after `git commit` without the one-line
   verdict write. Until the skeleton/prompt rail lands, the verdict
   belongs in the same breath as the commit — and a queue-side surface
   should flag DONE-without-verdict (rowed this pass, §f3).
2. **One-line fixes to open rows should jump the queue**: row 955 is
   10.5h old, verified-broken by six sessions, and blocks an entire
   module's plain gates. Nothing in the ladder surfaced it.
3. **FEATURES.md lags its own closeouts by one session**: the 10-55
   session updated CHANGELOG/AGENTS/DOMAIN_LANGUAGE but not FEATURES —
   this pass closes that (readmodel row now carries the fold-lock
   election + TasksPage cursor). The closeout skeleton's memory-update
   duty (10-55 §e4) should name FEATURES explicitly.
4. Everything else carries from the predecessor §e entries
   (commit-first discipline, self-tests for silent-success gates,
   CHANGELOG-at-landing, in-module lint scoping).

## f) NEXT THINGS

**Four new rows minted this pass** (dedup-checked against the 525 open
rows; everything else this verification surfaced is already rowed):

1. Fold-lock chaos test: hold `<home>.fold-lock` in a subprocess,
   SIGKILL it, assert the next opener wins — the kernel-releases-the-
   lease claim is doc-only today (10-55 §f13).
2. Webui operator surface: banner when `tq serve`'s projection model
   mounted read-only (fold lock held elsewhere) — the dashboard
   currently shows nothing (10-55 §f8).
3. Doctor/audit surface for DONE agent tasks with no recorded verdict —
   four paid claims of this very task committed reports and recorded
   zero verdicts; nothing queue-side flags the half-closeout (§d1).
4. foldlock_windows.go doc-comment naming the unpinned LockFileEx
   semantics (compile-only coverage; 10-55 §f11/§c5).

Highest-leverage open work at HEAD (pre-rowed, ordered): row 955
go.work bump, full ci-local pass with the dupe gate in situ,
dupe-baseline `--self-test`, row 645 devmod parity guard, row 647 CI
pin, Every>1 done-guard (630/631), the 10-55 session's fresh rows
(stats read-only surfacing; injectable fold-lock wait), rows 661/662.

## g) QUESTIONS (owner-only; all riding existing BLOCKED/open rows —

zero new BLOCKED rows this pass, dedup held)

1. Every>1 done-guard semantics at rewind replays — riding row 630.
2. `errInFlight` sentinel-as-control-flow vs `(time.Time, bool)` return
   — riding the BLOCKED row (Done-prompt window appends).
3. Is `branching-flow` installed by nix on CI runners, or dev-host-only
   — riding the BLOCKED row (third-redispatch appends).

## h) BAND DRIFT

`tq facts --type task.reprioritized` → **0 facts** (rc=0, re-run 11:09
against the production journal; fifth consecutive census — the journal
holds no reprioritized fact type at all, matching the 02:31, 03:14,
09:05, and 10:26 censuses). Nothing to account for under ADR-0015; all
observed claim-order movement across the window is stored priority +
ADR-0015 aging. The standing irony repeats: the repo-broken floor
blocker (row 955) has now produced no priority movement across five
censuses — exactly the change this section exists to catch.

## Appendix — docs-health pass notes (11:09)

1. **FEATURES.md readmodel row updated this pass**: appended the
   fold-lock single-writer election + `Model.TasksPage` cursor clauses
   (both verifiably shipped, test-pinned, CHANGELOG-carried since the
   10-55 session; FEATURES was the one living doc it missed).
2. **No archive moves**: nothing in this window's blast radius is
   fully-resolved-and-unreferenced — the five closeouts and four
   predecessor done-prompt reports carry open §f items by design
   (KEEP-OPEN rule); this report continues them. The 300+-row live
   index still wants a dedicated ANNOTATE sweep (already rowed).
3. **TODO_LIST.md**: 4 rows appended, 0 ticks, 0 rewords (dedup held;
   open unchecked 525 at pass start). This report indexed in
   docs/status/README.md (top 2026-10-20 cluster).
4. **README.md / ROADMAP.md**: no surface claims affected by this
   window or this pass (same greps as the 10-26 pass; the rows-658/659
   surfaces carry no README/ROADMAP claims — verified by grep).
5. **Verdict**: `tq verdict '{"report": "docs/status/2026-10-20_11-09_done-prompt-window-report-fifth-dispatch.md", "next_items": 4}'`
   runs immediately after this report's commit — the step four
   predecessors missed; its presence here is the audit trail.
