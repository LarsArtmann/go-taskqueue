# Done-Prompt Window Report (fourth dispatch) — 2026-10-20 10:26 CEST

> **DUPLICATE — ARCHIVED 2026-10-28 (row-1044 chain sweep)** — verify-only re-dispatch layer of the same 5-task window (2026-10-20 00:31–02:03, task 000001a156a3410fa257bc4a217c00000000); zero unique forward-looking items — every pass's row mints already live in TODO_LIST. The window's full a)–h) record is the canonical report `docs/status/archived/2026-10-20_02-31_done-prompt-window-report.md`.

Task: `000001a156a3410fa257bc4a217c00000000` — FOURTH dispatch. Pass 1
(02:31, `a7d19756`), pass 2 (03:14, `a7e67248`), and pass 3 (09:05,
`91ae96ca`) each committed a full report; none recorded the `tq verdict`,
so the queue re-claimed the task three times. Per the resume convention
(row 921's pattern) THIS pass re-verifies the predecessor reports at HEAD,
reports the delta since 09:05, and records the verdict before exiting —
the step this task has now failed three consecutive times.

Window: 5 agent tasks completed 2026-10-20 00:31–02:03. The 02-31 report
is the window's full a)–h) record; 03-14 and 09-05 are verification
layers one and two; this file is layer three.

## a) FULLY DONE — re-verified at HEAD (10:26)

Every §a claim of the 02-31 report re-checked this pass; all standing:

1. **Window commits intact** (all twelve `git cat-file -t OK`):
   `70f62dd9` (lint-gate polish), `d385faa1` (sessions lamp +
   `tq stats --since` band), `c0fe4d38` (dlqfix prune-semantics pin),
   `2de0c2dd` (webui-overhaul CHANGELOG coverage), `acc519da`
   (provenance comment fix), `b84528f6` + `fd95b61f` (dupe ledger +
   baseline gate), `88bc4480` (mirror-clones ruling), `d1a062a8`
   (budget golden pins), `85dbd9a2` (OnRecordTyped citation),
   `a786a876` (status lint pass), `3125a000` (done-guard fix). No
   reverts since.
2. **Artifact spot-checks this pass**: `scripts/lint-baseline.sh`
   carries `--clean-cache` (lines 12/29/30) + regen-mode cache clean;
   `scripts/dupe-baseline.sh`, `.dupe-baseline.txt`,
   `docs/dupe-type-accepted-ledger.md` at HEAD;
   `TestPruneStaleNeverTouchesDlqfixAutopsies`
   (internal/harvest/prune_test.go:430) and
   `TestSweepDoneGuardIsKnownNotRefusal`
   (internal/status/sweep_test.go:928) in place;
   `cmd/tq/stats_session_test.go` + `cmd/tq/budget_json_test.go`
   present.
3. **Living docs current at HEAD**: CHANGELOG [Unreleased] carries the
   sessions-lamp Added entry, the webui-overhaul Changed entry, the
   done-guard Fixed entry, AND pass 3's counter-Stats mints (healthz
   projection section; Delta-counter stats reads); FEATURES.md readmodel
   row carries the corrected Delta-counter rationale; AGENTS.md carries
   the `--clean-cache` Commands line.
4. **Five closeout reports** present under `docs/status/tasks/`
   (00-31, 01-10, 01-28, 01-47, 02-03), indexed where the index
   requires; all three predecessor done-prompt reports indexed
   (docs/status/README.md lines 124–127).
5. **Docs gates green this pass**: check-status-index ok,
   check-todo-list ok, check-doc-refs ok (trailer WARNINGs are the
   pre-existing f26 cluster class, owner ruling pending).

## b) PARTIALLY DONE

Unchanged from 09-05 §b, all still open and rowed:

1. Full `ci-local` never run with the dupe-baseline step in situ.
2. Done-guard reachability pinned only at Every=1 (rows 630/631).
3. `.golangci-baseline.txt` deliberate regen still owed.
4. Dupe-gate tool provenance unrecorded (row 974).
5. **Floor pairing STILL broken at HEAD, now 7.5h**: `cmd/tq/go.work`
   pins `go 1.27.1` while cmd/tq, root, readmodel, and composition
   go.mods all require `1.27.2`. Re-verified LIVE this pass: plain
   `cd cmd/tq && go list ./...` fails with "module … requires go >=
   1.27.2, but go.work lists go 1.27.1". Row 955 owns the fix; the
   concurrent go.mod/go.sum churn since 09:05 (see d4) did not touch
   go.work.

## c) NOT STARTED

Unchanged from 02-31 §c: the webui-overhaul self-review §f siblings,
the remaining 2026-10-05 16-02 rows, changelog backfill sweep beyond
the audited window. No new skipped work in this window's task set.

## d) TOTALLY FUCKED UP

1. **The dispatch itself, three times over**: task
   000001a156a3410f… has now consumed FOUR paid claims — three
   committed full reports (02:31, 03:14, 09:05) with zero recorded
   verdicts between them. The verdict-at-exit row minted at 09:05 is
   the right fix; this fourth claim is the third re-payment of the
   same omission. Each miss costs a full re-claim plus a verification
   pass.
2. **Floor-pairing blocker ages instead of closing** (see b5): row 955
   is a one-line go.work bump, minted 00-31 §d2/§f1, still open at
   10:26 while every plain cmd/tq `go` invocation stays dead and
   `scripts/test-cmd-tq.sh` remains shim-only for anyone without
   manual dev.mod regeneration.
3. History carried from 02-31/03-14 §d (unchanged): the flattened
   foreign feat commit (00-31 §d2), the near-miss vacuous dupe gate
   (01-28 §d1), the Every>1 done-guard gap (row 630).
4. In passing (NOT this window's work, mid-flight at verify time): a
   concurrent agent landed ~10 daemon-swept commits 09:38–10:15 —
   readmodel foldlock files (`foldlock.go`, `foldlock_unix.go`,
   `foldlock_windows.go`), model/projection-runtime/httpapi/doctor
   changes, tests, and composition+readmodel go.mod/go.sum churn. This
   is row 658's territory (single-writer fold guard, minted by the
   04-21 counter-Stats closeout). Uncommitted-by-nobody, unjudged by
   this pass; its own session owns the closeout, CHANGELOG, and gates.

## e) WHAT WE SHOULD IMPROVE

1. **Verdict-at-exit must become mechanical NOW** (row exists; the
   fourth claim is its complete evidence file): three agent passes in
   a row ended after `git commit` without running the one-line verdict
   write. Until the skeleton/prompt rail lands, every done-prompt
   session should treat `tq verdict` as part of the commit itself —
   same session, next command after the commit, not an afterthought.
2. **One-line fixes to open rows should jump the queue**: row 955 is
   seven hours old, verified-broken twice by independent sessions, and
   blocks an entire module's plain gates for everyone. Nothing about
   the queue's priority ladder surfaced it (see h) — a "verified
   repo-broken" flag on a row would.
3. Everything else carries from 02-31 §e / 03-14 §e / 09-05 §e
   (commit-first discipline, self-tests for silent-success gates,
   CHANGELOG-at-landing, FEATURES rationale verify passes).

## f) NEXT THINGS

**Zero new rows minted this pass.** The dedup check held again: every
candidate this verification surfaced is already an open row —
verdict-at-exit skeleton step + prompt rail (09-05 appends),
go.work bump (955), devmod floor-parity guard (645), CI pin (647),
Every>1 done-guard (630/631), dupe-baseline --self-test + provenance +
full ci-local + nolint ruling + growth attribution (974–978 region),
tick-sweep, golden pins, heal Known-Issues clause, §b sweep, dedup
pass, and the four BLOCKED owner rulings. Appending reworded
duplicates would re-fire the 2026-09-11 loop. The foldlock work in
flight (d4) is its own session's to row.

Highest-leverage open work at HEAD (pre-rowed, ordered): row 955
go.work bump, row 658 single-writer fold guard (in flight),
full ci-local pass with the dupe gate in situ, dupe-baseline
--self-test, row 645 devmod parity guard, row 647 CI pin.

## g) QUESTIONS (owner-only; all riding existing BLOCKED/open rows)

1. Every>1 done-guard semantics at rewind replays — riding row 630
   (open; owner-decided either way).
2. `errInFlight` sentinel-as-control-flow vs `(time.Time, bool)`
   return — riding the BLOCKED row (Done-prompt window appends).
3. Is `branching-flow` installed by nix on CI runners, or
   dev-host-only — riding row 992 (BLOCKED on runner inventory).

## h) BAND DRIFT

`tq facts --type task.reprioritized` → **0 facts** (fourth consecutive
census, re-run 10:26 against the production journal; the journal holds
no reprioritized fact type at all, matching the 02:31, 03:14, and
09:05 censuses). Nothing to account for under ADR-0015; all observed
claim-order movement across the window is stored priority + ADR-0015
aging. Notably, the repo-broken floor blocker (row 955) has produced
no priority movement at all — the one change this section exists to
catch would have been exactly this kind of jump.

## Appendix — docs-health pass notes (10:26)

1. **No archive moves**: nothing in this window's blast radius is
   fully-resolved-and-unreferenced — the three predecessor done-prompt
   reports and the five closeouts carry open §f items by design
   (KEEP-OPEN rule); this report continues them.
2. **Living docs verified standing at HEAD** (spot-checked this pass,
   listed in a3): CHANGELOG, FEATURES, AGENTS.md all carry the entries
   passes 1–3 minted; no new user-visible surface landed in THIS pass
   to mint (the readmodel foldlock churn is a concurrent session's
   mid-flight work — its closeout owns CHANGELOG).
3. **TODO_LIST.md**: 0 appends, 0 ticks, 0 rewords this pass (dedup
   held; open unchecked count 525 at pass start, consistent with the
   09:05 ledger of 524 + 2 − 1). This report is indexed in
   docs/status/README.md (top chronological cluster).
4. **README.md / ROADMAP.md**: no surface claims affected by this
   window or this pass (verified by the same greps as 09-05; nothing
   user-visible shipped since).
