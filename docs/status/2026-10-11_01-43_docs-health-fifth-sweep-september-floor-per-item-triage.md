# Status Report — 2026-10-11 01:43 — Docs-Health Fifth Sweep: September-floor per-item triage — 10 archives, 99 strikes, harvest, living-docs truthing

> Interactive docs-health AUDIT (skill: HARVEST + VERIFY + ANNOTATE + BUILD)
> over the full live `2026-0*` corpus (glob = September and earlier: 116
> status reports + 6 planning docs + 2 feedback docs + 14 non-md artifacts
> LEAVE-carried). This is the dedicated per-item pass the archive
> conventions reserve for the KEEP-OPEN floor. Prior sweeps: 09-21, 09-30,
> 10-01, 10-02, 10-05 (fourth). All verdicts at this pass's HEAD (daemon
> HEAD ecc90dd0); the 2026-10-* reports (~150 live files) are outside this
> glob and remain the next sweep's corpus.

## a) FULLY DONE

1. **Full-corpus per-item triage via 9 date-bucket fan-outs** (B1
   09-08..13 ×14, B2 09-14 ×16, B3 09-15..17 ×13, B4 09-18..19 ×17,
   B5 09-20 ×15, B6 09-21..27 ×10, B7 09-28 ×10, B8 09-29..30 ×13,
   B9 planning/feedback ×8): every forward-looking item in every live
   file verdicted SHIPPED (file:line grep receipt) / ROUTED (live TODO
   row) / RULED (O-sheet, ADR, AGENTS contract) / NARRATIVE / OPEN, with
   duplicate-cluster checks (every same-ID canonical identified; the
   09-28 19-05/19-16 pair confirmed NOT duplicates — 19-16 filed #3982).
2. **10-file archive with manifest** (bulk → manifest per the 10-01 rule).
   Every archived file carries an inline disposition note + per-item
   strike pointers (annotate-status-items: emit-keys → --verify → apply,
   check-rows complete on all eight; 99 residue strikes total):
   - `2026-09-10_06-39_task-…899ed.md` → ARCHIVE-DONE (fullcore contract
     shipped main.go:59-60; dogfood evidence archived; rest routed/ruled).
   - `2026-09-15_02-30_post-deploy-rate-limit-retro.md` → ARCHIVE-DONE
     (both "Left open" items route to live rows).
   - `2026-09-17_03-04_task-…c8d5.md` → ARCHIVE-DONE (crush #3146 row
     carries command + dated 2026-10-03 re-check; session surfaces shipped).
   - `2026-09-17_03-26_task-…5d.md` → ARCHIVE-DONE (postgres AppendFact
     shipped + CHANGELOG; O5/O6 rule the tail).
   - `2026-09-20_06-17_task-…41a0.md` → ARCHIVE-DONE (dead-sha self-test
     shipped; case bundle routed; O16/O5 rule the rest).
   - `2026-09-20_08-02_task-…fadb.md` → ARCHIVE-DONE (module-loop
     consumers enumerator-fed + capture gate; ADR-0017/O5 rule the rest).
   - `2026-09-22_01-37_task-…80e2.md` → ARCHIVE-DONE (task.requeued
     shipped; ScopedGofmtStage shipped; forensics filters routed).
   - `2026-09-29_23-28_task-…0000.md` → ARCHIVE-DONE (archive-evidence
     shipped + smoke; probe/pointer/bullet landed; rulings routed/O14).
   - `docs/planning/2026-09-08_round5-m23-feature-designs.md` → EXECUTED
     (sketches-only; all five F121–F125 routed to ROADMAP seeds).
   - `docs/planning/2026-09-18_sail-research-provider-assessment.md` →
     EXECUTED (NOT ADOPTED dispositioned; parked at ROADMAP:182-188).
     Spot-verify exceeded the ≥2 quota: 2+ items per file re-verified
     first-hand at HEAD (deadline contract, dogfood archive, retro row,
     #3146 row, AppendFact, dead-sha self-test, capture gate, task.requeued,
     ScopedGofmtStage, smoke/archive-evidence, AGENTS bullet, README
     pointer, proxy fetch, ROADMAP seeds/disposition).
3. **Citations repointed in the same change** (check-doc-refs is the
   catcher): TODO row 475 (02-30 path + line), live reports
   2026-09-15_02-28 + 2026-10-01_06-26, ROADMAP ×2 (sail + round5-m23),
   CHANGELOG (round5-m23), plus 8 index rows → backticked `archived/…`.
4. **Living-docs truthing:** AGENTS.md STATUS line v0.3.2→**v0.3.3**
   (stale since the 10-07 publish); README quickstart "Until then, build
   from a clone" caveat REPLACED with the verified `go install
   …/cmd/tq@v0.3.3` line (proxy @v/v0.3.3.info fetched live this pass —
   Version v0.3.3, 2026-10-07T10:18:38Z); `check-agents-size.sh` budget
   twin synced to the test's sanctioned 19,000 B reset (RED at session
   start — the documented twin-drift class, test moved 10-10, script
   stayed 18,500).
5. **Harvest: 19 rows minted** in a dated sweep-5 section (help-text
   hardening, fullcore polish, verify_stage surfacing, webui shots
   ruling, canary parity, fixture-attribution gate, run-gate.sh,
   crush-doctor tail, done-guard residue, battery completeness,
   batched-harvest residue, derived-outcome residue, crush-ops residue,
   session polish, webui band tail, provenance dedup, gate-script polish,
   release re-cut rule BLOCKED, owner-policy bundle BLOCKED) + ROADMAP
   raw-ideas block (payload-file, env-file, Discord tail). Deduped by
   live-text grep: the cookie question was already at ROADMAP:306
   (skipped); canary facade half lives in the adjacent row.
6. **Count-first discipline held:** before any move — 116 status root
   files, 582 archived, 34 planning-archived, 286 index live rows; after
   — 108/590/36/279. Counter 576→590 (absorbing six uncounted moves from
   prior sweeps, recounted from the tree) + this sweep's narrative line.

## b) PARTIALLY DONE

1. **The MIXED floor (98 status files) stays live by convention** —
   their RESOLVED items remain unstruck (absence of a marker IS the open
   signal; strikes belong to the per-file retire pass). The per-item
   verdicts for those files were derived this pass but are
   session-transcript-resident, not transcribed into a durable doc —
   the durable record is the harvested OPEN clusters + the archived
   strikes (the ratified "routed = visible in TODO_LIST; the rest is
   report-resident" bar, now explicitly disclosed).
2. **cmd/tq `TestAgentsDocSizeGuard` twin unrun in-situ**: cmd/tq module
   setup fails at HEAD — `internal/config@v0.3.3` is tagged locally but
   unpushed, so the replace-free module cannot resolve it (documented
   foreign state, push authorization is owner Q). The script twin was
   green at my twin-sync, then went RED again (18,998 → 19,539 B) from a
   CONCURRENT window's fresh Known Issue (readmodel projection-home
   schema-drift outage note, +541 B, daemon-swept cec6e4cd) — foreign,
   load-bearing, left unpruned per the M20 doctrine (the adding window
   owns its budget accounting); the next window owes the prune/reset.
3. **Agent line-number drift**: sub-agent TODO_LIST cites drifted against
   the tree (rows moved); caught by the orchestrator spot-verify and
   re-verified by content grep — receipts are content-anchored, not
   line-anchored, in this report.
4. **CHANGELOG depth**: no sweep entry added (O14: internal-only sweeps
   get no row); the ~2,100-line historical audit remains unowned.

## c) NOT STARTED

1. The **2026-10-\* corpus** (~150 live files, growing ~15/day) — the
   next sweep's glob; the delta is now five days deep.
2. Standing owner rulings carried, not advanced: KEEP-OPEN terminal
   state (10-01 §g2), archive-eligibility gate (rows 218/417), TODO
   row-cap convention, strike-scope ratification.
3. `check-features-ci.sh` not run (FEATURES.md untouched this pass).

## d) TOTALLY FUCKED UP (honest defects)

1. **Attribution lost to the daemon again**: the entire sweep (moves,
   strikes, repoints, counter, gates fixes) landed in two footer-less
   `chore:` auto-commits (be0f64d9 ×8 files, ecc90dd0 ×26). No dispatch
   ID exists for an interactive session; heal-daemon-sweep needs one.
   This report is the marker the attribution audit reads.
2. **Session-start ritual skipped** — the chronic miss, re-hit (no
   `scripts/session-start.sh`, no CONTRIBUTING read, no master-CI probe).
3. **One 429-killed sub-agent launch** (B4, first try) — serial relaunch
   landed clean; the predefined degraded mode was not needed.
4. **Trusted agent line numbers once before verifying** — the B1 emit
   batch reused agent line numbers that had drifted; caught at
   emit-keys preview time (the tool's preview caught it), then full
   inventories were grep-derived per file. Tool atomicity made it free;
   the lesson stands: line numbers are claims, previews are evidence.

## e) WHAT WE SHOULD IMPROVE

1. Harvest receipts: content-anchored cites (row-text grep) beat
   line-anchored cites for any file another agent may have edited
   between read and write — make it the contract for TODO_LIST cites.
2. Persisting per-item verdicts for MIXED floors needs a cheap durable
   form (e.g. a per-bucket appendix file under docs/status/archived/ or
   accepting report-residency as the documented bar — decide once
   instead of re-litigating per sweep).
3. The size-twin drift keeps recurring because two constants exist;
   the check script could grep the test's `agentsDocMaxBytes` literal
   instead of carrying its own number (single source, zero drift).

## f) Next tasks

1. Next docs-health sweep: glob `2026-10-*` (~150 files) while the
   delta is small; the five 10-09/10-10 windows are fresh enough to
   triage cheaply.
2. The 19 minted rows enter the pool via harvest; highest-leverage
   three: battery completeness (check-ci + journal-drift legs),
   canary parity, verify_stage surfacing.
3. Re-run the root battery at final HEAD if concurrent windows land
   after this report's rc capture.

## g) QUESTIONS I CANNOT FIGURE OUT MYSELF

1. Should the per-item verdict inventory for MIXED floors become a
   durable artifact (per-sweep appendix file), or is report-residency
   (this report §b1) the ratified terminal state? Carried since 10-01
   §g2, now with a concrete form to rule on.
2. Push authorization for the local tag wave (internal/config/v0.3.3 et
   al.): until pushed, cmd/tq's module gate is structurally red and
   every window pays the diagnosis tax. Owner Q stands (05-47 §g).
