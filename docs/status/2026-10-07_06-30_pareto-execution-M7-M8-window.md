# Pareto Execution — M7 Wake Seam Complete, M8 Unblocked Slices, Vendor Gate Re-Anchored (Session Window 05:30–06:30)

- **Session:** 2026-10-07 05:30–06:30 CEST, continuation session (owner directive:
  keep executing the 26-task Pareto plan; prior windows M1–M5, M6 batch 1, M7 core)
- **Plan:** `docs/planning/2026-10-07_03-34_pareto-trunk-green-to-production.md`
- **Prior window report:** `2026-10-07_05-18_pareto-execution-M5-M7-window.md`
- **Session commits:** 10 authored (0e20b3e2 … 9a63ca69; the postgres parity
  work rides 63c9748b after a daemon re-commit of the same tree), plus foreign
  interleaves. 24 unpushed at report time; a concurrent actor pushed origin
  forward THROUGH 0e20b3e2 mid-window (the 05-18 §g1 push question is resolving
  itself externally — this session never pushed).

## a) FULLY DONE

1. **Owner Q3 answered autonomously — the vendor-sync gate was blind, now
   hash-anchored** (0e20b3e2). Proven live: a mutated `vendor/modules.txt`
   yields ZERO `git status --porcelain -- vendor/` lines (gitignored), so the
   M3 gate's root-vendor half was dead code in exactly the b886a677 class it
   was built for. Re-anchored to a content hash of the tree compared across
   the `go mod vendor` regeneration boundary; self-test grew case 4 (the
   blindness pin: drifted-ignored-vendor is invisible to git status yet moves
   the hash); full gate green over all 22 modules + root.
2. **M7 claim-wake seam COMPLETE** (fine tasks 39–48):
   - Worker wiring (264b4113): `Config.Wake` + `idleWait` — a four-way select
     (gap tick / wake / ctx / pool stop); wake RESETS the idle ladder. Pinned:
     parked-10s-gap re-claims <250ms CI-stable (observed ~10ms vs the 50ms
     design target) and a wake train holds gaps near the base interval.
     `sleepCtx` retired with its single caller.
   - Both tq pools armed (ff10a323): `tq worker` silently, `tq agent-pool`
     with the one armed stderr line. Concrete store (facade alias = the v4
     Waker), so the type-assert became a direct `Notify()` call.
   - postgresv4 parity (63c9748b): same buffered-1 non-blocking channel fired
     after enqueue / fail-with-retry / requeue / rescue-dead / record-answer.
     The three wake pins (fire, coalesce, requeue-fire) were verified LIVE
     against a scratch postgres cluster (unix-socket, trust auth, /tmp), and
     the FULL postgresv4 suite incl. conformance ran green on it (11.6s).
   - Dispatcher wake drain (06607444): `consumer.Config.Wake` + a select arm;
     end-to-end pin delivers within 2s against a 10s poll interval via the
     real sqlite store's own Notify. No production consumers yet (ADR-0009
     infrastructure) — the seam is ready for the composition root.
   - Docs (e7b5fefe): **ADR-0020** (side interface, buffered-1, polls stay
     fallback, process-local by design — the multi-process M7.48 semantics),
     DOMAIN_LANGUAGE **Claim-wake** + **Attempts** entries, CHANGELOG entry,
     M7 TODO row closed. Idle-IO evidence line added to TestPoolIdleBackoff:
     8 probes/500ms at a 10ms/80ms ladder where fixed cadence issues ~50 —
     the wake seam adds zero idle probes (it only stops OVER-waiting).
3. **M8 unblocked slices** (the 50-51 fix itself stays owner-gated, see §g):
   - Gap reproduced + pinned (f87e4c80): a commit whose footer sits above the
     trailing attribution block attributes NOTHING (git `%(trailers)` parses
     only the final paragraph) while the identical footer-last commit in the
     same repo attributes fine — characterization pin
     `TestGitLogScannerFinalParagraphGapCharacterized`, loudly pointing at the
     owner-gated fix row.
   - Derivation-blind census (f87e4c80): `scripts/census-derivation-blind.sh`
     (rerunnable). Lifetime v0.1.0..HEAD: **283 of 1031 footer commits
     invisible (27%)**, 1710 no-footer; the unpushed range at measurement time
     was CLEAN (6/6 visible).
   - `tq show --commits` honesty (148f0255, closes rows 240/282/242): AMBIGUOUS
     now reserved for genuine conflicts (commits whose footer block names a
     FOREIGN task ID — detected + listed); count>1 reads as the work+close-out
     norm; every folded_here daemon commit renders its changed files
     (diff-tree with `--root` so root commits show) making the 552a1383-class
     false-positive fold visible at a glance; flag help documents verdict
     classes + folded_here + working flag order. Windows-root-cause found the
     hard way: `git diff-tree` without `--root` shows nothing for root commits.
4. **M6 archive batch 2, file 1 of 4** (c9a31765 + 9a63ca69): the 2026-09-15
   00-12 journal-drift report archives with ALL 53 forward items resolved
   inline (annotate-status-items.py, verify-first, 53/53). The one code gap it
   left open was MINTED rather than routed: `TestJournalDriftNoDriftAfter-
   FailThenComplete` (fail → reclaim → complete through the real store:
   attempts=1 stored AND replayed — the DOMAIN_LANGUAGE Attempts semantics
   pinned). Item 22 routed (forensics row), 12/46 subsumed (upstream-enrich /
   fold-marker rows), §g1–3 answered in-file. Manifest + index updated;
   check-rows + status-index + doc-refs green.
5. **AGENTS budget battle won without an owner reset** (e7b5fefe): concurrent
   agents pushed the file to 16,982 (over budget!) then 17,023 with my edits;
   compressed the templ-components Known-Issue bullet (every fact kept) back
   to 16,879/16,900. A later foreign trim sits at 16,858 now. The Q2
   sixth-reset-vs-prune question stays open but did not block this window.
6. **Gates cited at their commit points:** worker module build+vet+race ✓;
   cmd/tq shim gate ✓ ×4 (incl. windows cross-compile); postgresv4 full suite
   live ✓; consumer build+vet+race ✓; root vendor+build+vet ✓ (mid-window);
   doc gates (agents-size, doc-refs, todo-list, status-index, dead-sha-refs,
   script-syntax, vendor-sync real+self-test) ✓. Root race suite NOT re-run
   end-of-window (owed, see §b).

## b) PARTIALLY DONE

1. **M6 batch 2 files 2–4 NOT archived** (02-30 / 02-28 / 01-01). The 02-30
   retro was mid-flight when the status demand arrived: the missed 7-day 429
   checkpoint's tasks-table half ran (see §d5 for the finding) but the
   facts-side classification died on a schema guess; zero annotations written.
2. **Final battery owed:** no full quiet-tree `ci-local.sh` run, no end-of-window
   root race suite, and the executor module got only the single-test run after
   the characterization append (full module gate owed).
3. **CHANGELOG entries for the M8 trio** (repro+census, tq-show verdicts) not
   yet written — the wake-seam entry landed; the M8 commits are documented in
   their messages + this report only.
4. **M8.55 doctor attribution WARN** (row 323): not started; the census script
   now covers the data half for it.
5. **Row 442 (7-day checkpoint)**: half-executed — see §d5; closure needs the
   facts query fixed (one PRAGMA away) + the classification rerun.

## c) NOT STARTED

1. M8.50–51 scanner fix + e2e shape — **owner-blocked** (row 153/246: %B
   fallback vs footer-last harness rule; the repro pin + census data now sit
   under the ruling).
2. M6 batch 3 (09-16..09-25 reports, per row 102).
3. M9 (worktree-per-agent slice 1) — next plan milestone; nothing in M8's
   blocked half gates it.
4. M10–M26 per plan sequencing.

## d) TOTALLY FUCKED UP

1. **AGENTS.md anchor fights:** three foreign edits landed INSIDE my edit loop
   (16,886 → 17,023 → 16,949); two python replacements died on stale anchors
   before switching to line-range replacement. On a known-hot file I should
   re-read immediately before every anchor edit.
2. **Spec-builder self-bugs:** my dup-assertion crashed twice (bare item
   numbers collide across §f/§g restarting lists — section tags were the fix
   I should have used first), and `--emit-keys` with an out-of-range line
   number wrote a FAIL line into the key file that then crashed the builder.
   Three wasted round trips, all my own tooling discipline.
3. **Schema guess on the production journal:** queried `facts.ts` without a
   PRAGMA — `no such column` mid-checkpoint. Read-only was honored; the stall
   cost the 02-30 archive.
4. **Commit-hash confusion:** the postgres parity work I committed as 53420f83
   exists as 63c9748b (identical tree, daemon re-commit) — caught while
   writing THIS report, not when it happened. Harmless here (work intact,
   attributed content identical) but worth knowing the daemon re-hashed it.
5. **A parked discovery with teeth:** the DLQ holds **325 dead tasks, every
   one created pre-flip (< 2026-09-12)** — the 02-30 retro tracked 8. That is
   not a bug I introduced; it is a surface I stumbled into mid-query and did
   NOT investigate (correctly — not my lane this window) but it must reach
   the owner (§g context).

## e) WHAT WE SHOULD IMPROVE

1. Re-read hot files immediately before anchor edits; prefer line-range or
   marker-based replacement when a file is under concurrent edit.
2. PRAGMA table_info before first query on any unfamiliar schema.
3. Section-tagged spec keys from the start in multi-list files.
4. Full module gate immediately after ANY test-file append (the executor
   single-test run was lazy).
5. CHANGELOG entry per feature commit, not batched for later.
6. What held WELL: commit-at-compile-clean discipline meant zero daemon
   steals this window; verify-before-write on the annotator caught every
   spec problem before any file mutation.

## f) UP TO 25 THINGS TO GET DONE NEXT (ranked)

1. Finish 02-30: PRAGMA the facts schema, run the 429 classification, verdict
   both open items, archive (M6 batch 2 file 2).
2. Annotate + archive 09-15_02-28 and 09-15_01-01 (batch 2 complete; row 102).
3. Full quiet-tree `ci-local.sh` + root race suite (the standing formalities).
4. Executor module full gate (owed from the characterization-test append).
5. CHANGELOG entries for the M8 trio.
6. Row 442 checkpoint closure with the working tasks-side queries.
7. M8.55: `tq doctor` attribution WARN over the census classes (row 323).
8. M6 batch 3 (09-16..09-20) per plan fine task 32.
9. M9.56–63: worktree-per-agent slice 1 (mint rows from 01-43 §f11–20).
10. M10 budget O2 core (token/cost cap gate, fine tasks 64–72).
11. Dead-DLQ surface: owner decision on the 325 pre-flip dead (rescue /
    cancel / `--dlq-fix` sweep) — do not act unilaterally.
12. Push decision for the 24 unpushed (owner; external pushes continue).
13. AGENTS budget Q2 ruling (sixth reset vs prune discipline; now 16,858).
14. Dispatcher wake wiring at a composition root when a consumer appears.
15. Census → `tq audit`/doctor integration polish (ids + task mapping).

## g) TOP QUESTION I CANNOT FIGURE OUT MYSELF

**The scanner fix direction (row 153/246, now evidence-loaded):** git
`%(trailers)` parses ONLY the final paragraph, so a footer above the trailing
attribution block is invisible — lifetime 283/1031 footer commits (27%),
unpushed range clean. Option A: `%B` fallback in GitLogScanner (attributes
every verbatim footer, but sacrifices the "quoting someone else's footer is
never attributed" guarantee at gitscan.go:141). Option B: footer-last shape
rule on the harness/template (preserves the guarantee; touches the harness
commit contract and leaves 283 historical commits needing a heal-or-accept
decision). This gates M8.50–51 and the e2e fixture pin. Fresh inputs for the
ruling: the characterization pin + `scripts/census-derivation-blind.sh`.

_(Owner-facing facts, not questions: 325 pre-flip dead tasks in the
production DLQ per the read-only query; 24 unpushed commits; origin advanced
through 0e20b3e2 by an external actor.)_
