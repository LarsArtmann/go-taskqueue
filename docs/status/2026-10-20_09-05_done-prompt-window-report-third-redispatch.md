# Done-Prompt Window Report (third re-dispatch) — 2026-10-20 09:05 CEST

Task: `000001a156a3410fa257bc4a217c00000000` — THIRD dispatch. Pass 1
(02:31, `a7d19756`) wrote the full window report but recorded no verdict;
pass 2 (03:14, `a7e67248`) re-verified at HEAD and ALSO failed to record
the verdict (its commit message says "record verdict"; the queue re-claimed
the task anyway). Per the resume convention (row 921's pattern) THIS pass
re-verifies the predecessor reports at HEAD, reports the delta since 03:14,
and its sole non-negotiable exit step is the `tq verdict` recording.

Window: 5 agent tasks completed 2026-10-20 00:31–02:03 (the docs-health
fourth-sweep tail + the dupe-review follow-up batches). The 02-31 report
remains the window's full a)–h) record; 03-14 is the first verification
layer; this file is the second.

## a) FULLY DONE — re-verified at HEAD (09:05)

Every §a claim of the 02-31 report re-checked this pass; all standing:

1. **Window commits intact**: `70f62dd9` (lint-gate polish),
   `d385faa1` (sessions lamp + `tq stats --since` band), `c0fe4d38`
   (dlqfix prune-semantics pin), `2de0c2dd` (webui-overhaul CHANGELOG
   coverage), `acc519da` (provenance comment fix), `b84528f6` +
   `fd95b61f` (dupe ledger + baseline gate), `88bc4480` (mirror-clones
   ruling), `d1a062a8` (budget golden pins), `85dbd9a2` (OnRecordTyped
   citation), `a786a876` (status lint pass), `3125a000` (done-guard
   fix) — all present in `git log`, no reverts since.
2. **Survival spot-checks this pass** (files the later windows touched):
   - f19's OnRecordTyped citation SURVIVED the counter-Stats window's
     `internal/readmodel/events.go` rewrite — the wire-typed fold
     comment stands (events.go:23, now load-bearing for the new
     `status_counts`/`project_status_counts` folds).
   - `scripts/lint-baseline.sh` carries `--clean-cache` (lines 12, 29)
     and the regen-mode cache clean.
   - CHANGELOG [Unreleased] carries all three window-era entries
     (sessions lamp Added, webui-overhaul Changed, done-guard Fixed).
   - `docs/dupe-type-accepted-ledger.md`, `scripts/dupe-baseline.sh`,
     `.dupe-baseline.txt` at HEAD; `TestPruneStaleNeverTouchesDlqfixAutopsies`
     and `TestSweepDoneGuardIsKnownNotRefusal` in place.
3. **TODO rows the window minted are present** (rows 955–967, 970–981
   region: floor pairing, day-suffix durations, stats --json, heal
   --pick, baseline regen, AGENTS --clean-cache line, webui-css loud
   fallback, gosec N==1 stub, nowband windowing, smoke band assert,
   changelog-audit follow-ups, dupe-baseline follow-ups, tick-sweep,
   golden pins, heal Known-Issues clause, §b sweep, dedup pass, plus
   the BLOCKED rulings); the 02-03 batch's rows 630/631 present. One
   row CLOSED this pass (see appendix): the AGENTS.md --clean-cache
   Commands line.
4. **Docs gates green this pass**: check-status-index, check-todo-list,
   check-doc-refs all ok (trailer WARNINGs are the pre-existing f26
   cluster class, owner ruling pending — not this window's).

## b) PARTIALLY DONE

Unchanged from 03-14 §b, all still open and rowed:

1. Full `ci-local` never run with the dupe-baseline step in situ.
2. Done-guard reachability pinned only at Every=1 (rows 630/631).
3. `.golangci-baseline.txt` deliberate regen still owed.
4. Dupe-gate tool provenance unrecorded (row 974).
5. NEW this pass: the floor-pairing fix itself is HALF-landed —
   `dc0f8bbb` (02:47) raised cmd/tq/go.mod to 1.27.2 but left
   `cmd/tq/go.work` at `go 1.27.1`, so the blocker is now STRICTLY
   WORSE: BOTH workspace members require 1.27.2 while the workspace
   directive pins 1.27.1. Verified live at HEAD: plain
   `cd cmd/tq && go list ./...` fails, AND the documented workaround
   class (`GOTOOLCHAIN=go1.27.2 …`) ALSO fails — the go.work directive
   check precedes toolchain selection ("module . listed in go.work file
   requires go >= 1.27.2, but go.work lists go 1.27.1"). Row 955 owns
   the fix; the remedy is now pinned to the go.work bump (or `go work
   use`), not a toolchain env override.

## c) NOT STARTED

Unchanged from 02-31 §c: the webui-overhaul self-review §f siblings,
the remaining 2026-10-05 16-02 rows, changelog backfill sweep beyond
the audited window. No new skipped work in this window's task set.

## d) TOTALLY FUCKED UP

1. **The dispatch itself, twice**: task 000001a156a3410f… has now
   consumed THREE paid claims. Two passes committed full reports
   (02:31, 03:14) and neither recorded `tq verdict` — the exact miss
   class 03-14 §d1 named, then struck a second time four hours later.
   Each omission costs a full re-claim plus a verification pass.
   Rowed this pass: make verdict-at-exit a numbered step of the
   closeout skeleton + done-prompt rail.
2. **Floor-pairing half-fix made plain-go worse** (see b5): the 02:47
   bump touched go.mod files only; `cmd/tq/go.work` — the file the
   error message names — still pins 1.27.1 at HEAD, 6+ hours after
   row 955 was minted, and the GOTOOLCHAIN workaround no longer
   applies. `scripts/test-cmd-tq.sh` remains dead for anyone without
   manual dev.mod regeneration.
3. History carried from 02-31 §d (unchanged): the flattened foreign
   feat commit (00-31 §d2) and the near-miss vacuous dupe gate.
4. In passing (LATER window, noticed while verifying): the counter-Stats
   window's own closeout (04-21) found a regression-shaped hazard —
   non-idempotent Delta counters double-count under the blessed
   multi-process projection-home sharing (row 658, not fixed; operating
   rule: one folding process per home). Not this window's work; listed
   because it is the highest-value open risk at HEAD.

## e) WHAT WE SHOULD IMPROVE

1. **Verdict-at-exit must be mechanical** (rowed): the closeout
   skeleton should carry it as a numbered exit step, and the done-prompt
   prompt template should repeat it as the literal last instruction.
   Two consecutive agent passes forgetting the same step is a contract
   defect, not an agent defect.
2. **Floor moves should land as ONE commit** across go.work + go.mods
   (+ CI pin, row 647): today the pairing is enforced by three open
   rows (955 go.work, 647 CI pin, 645 devmod parity guard) that were
   all minted because one commit moved one file. The 645 guard makes
   the next occurrence loud; landing all three together closes the
   class.
3. **CHANGELOG-at-landing, again**: within hours of this window
   closing the webui coverage gap, the counter-Stats window landed two
   user-visible surfaces (Delta-counter stats reads, /healthz
   projection section) with no [Unreleased] entries — the same
   deferral shape. This pass mints both entries (appendix); the
   generalization row (changelog-audit script) stays open.
4. **FEATURES rationales need VERIFY passes too, not just statuses**:
   the readmodel row still claimed "a stateless event-counter projection
   provably diverges" AFTER the code adopted exactly that design. A
   status-only freshness check would never catch a claim-to-code
   inversion; this pass corrected the row (appendix).
5. Everything else carries from 02-31 §e / 03-14 §e (commit-first
   discipline, self-tests for silent-success gates, in-module lint
   scoping, runnable host-env runbook).

## f) NEXT THINGS

The five closeouts' cap-10 batches, the 02-31 pass's 7 appends, and
the later windows' mints (rows 645/647/658/661/662, f4c8a748's pair)
already cover nearly every candidate this verification surfaced. Two
genuinely new rows minted this pass (dedup-checked by live-text grep):

1. Verdict-at-exit as a mechanical closeout-skeleton step + done-prompt
   rail (the three-claim evidence above).
2. branching-flow CI availability confirmation + step-comment note
   (BLOCKED on owner runner inventory; 01-28 §g2 was never rowed).

Highest-leverage open work at HEAD (pre-rowed, ordered): row 955
go.work bump (unblocks cmd/tq gates for everyone), row 658 single-writer
fold guard (regression-shaped hazard), row 647 CI pin, row 645 devmod
parity guard, full ci-local pass (row 975), dupe-baseline --self-test
(row 973).

## g) QUESTIONS (owner-only)

1. Is `branching-flow` installed by nix on CI runners, or dev-host-only?
   (Minted as the BLOCKED row above; decides whether the dupe gate is
   CI-real or dev-only and what the ci-local step comment must say.)
2. Done-guard semantics at Every>1 rewind replays: probe the dedup key
   directly vs accept the silent no-op and scope the pin to Every=1 —
   riding row 630 (open; not BLOCKED-tagged, treat as owner-decided
   either way).
3. `errInFlight` sentinel-as-control-flow vs `(time.Time, bool)` return,
   before row 503's sibling sweepers copy the pattern — riding the
   BLOCKED row (Done-prompt window appends section).

## h) BAND DRIFT

`tq facts --type task.reprioritized` → **0 facts** (re-run 09:05; the
journal holds no reprioritized fact type at all, matching the 02:31 and
03:14 censuses). Nothing to account for under ADR-0015; all observed
claim-order movement is stored priority + ADR-0015 aging.

## Appendix — docs-health pass notes (09:05)

Living-doc corrections made THIS pass (all claims verified against code
before writing):

1. **CHANGELOG.md** — minted the counter-Stats window's two missing
   [Unreleased] entries (user-visible, landed, verified): Added
   `/api/v1/healthz` projection section (`b5d8cabf`;
   `TestHealthzCarriesProjectionSection`, httpapi_test.go:1008) and
   Changed readmodel stats onto Delta counters (`797f3938`+`db241bc0`;
   collections at queries.go:29-30, `ExecuteTyped` reads at stats.go,
   parity test green per closeout). Append-only respected.
2. **FEATURES.md** — corrected the readmodel row's now-false rationale
   ("SQL GROUP BY pushdowns … event-counter projection provably
   diverges" → the shipped Delta-counter design with apply-time
   prev-read exactness) and extended the durable-cursor row with the
   counter-version replay guard. Status labels unchanged (both
   FULLY_FUNCTIONAL, verified).
3. **AGENTS.md** — added the one-line `--clean-cache` Commands-block
   entry (the open row's exact ask); TICKED that row in TODO_LIST with
   this report as evidence. AGENTS.md growth: one line, deliberate.
4. **TODO_LIST.md** — 1 tick (above), 2 appends (§f), 0 rewords, 0
   deletions. Open unchecked count at pass start: 524.
5. **No archive moves**: nothing in this window's blast radius is
   fully-resolved-and-unreferenced (the window's closeouts and both
   predecessor done-prompt reports carry open §f items by design);
   index bloat was pruned separately (10e4e86d, threshold re-derived
   to 400). This report is indexed in docs/status/README.md.
6. **README.md / ROADMAP.md**: no surface claims affected by this
   window or the passing corrections (the lamp/band and counter reads
   sit under existing rows; verified by grep).
