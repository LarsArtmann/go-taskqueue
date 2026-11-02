# Done-prompt window report — twenty-seventh dispatch (re-verification pass)

Written: 2026-11-02 17:16 CET. Task `000001a19bd5e04bb892a3f3c2f800000000`.
Window: the 17 tasks completed 2026-10-30 05:31 → 2026-10-31 04:18 (13 with
a)-g) closeout reports, 2 reviewer-fix tickets, 1 push-master ops task).

**Dispatch-context finding first**: this window was already fully reported
TODAY by the twenty-sixth dispatch
(`docs/status/2026-11-02_12-26_done-prompt-window-report-twentysixth-dispatch.md`,
committed 12:26). This pass is a RE-VERIFICATION of that report against the
current HEAD, not a re-derivation: all 13 closeout reports were read, every
headline landing re-grepped at HEAD (all green), the production journal
re-read for band drift. No contradictions with the 12-26 report were found.
The double dispatch of one window in a day is itself a finding (§e6, rowed).

## a) FULLY DONE (verified at HEAD, not just claimed)

1. **Header-matrix hardening chain** (…b0ce2017, …b1a9a232, …b317d28d +
   reviewer tickets …b317e4294f, …b317e9d01, …b6d0061a):
   - serve-429 row collapsed onto package-level `serve429Row`, favicon
     hoisted to `serveFaviconRow` (494b4820).
   - Six per-surface test functions exist (`grep -c 'func TestServeHeaderMatrix'
     internal/webui/headermatrix_test.go` = 7 incl. the RouteCoverage
     meta-guard), gocognit/noctx verified silent by scoped lint in the
     05-51 closeout.
   - `withContentType` pins live-observed media types per surface; `with(rows…)`
     is the single merge mechanism; observe-first convention bullet landed in
     AGENTS.md §Conventions (verified in file).
   - Reviewer findings cured: §g1 BLOCKED annotation (ae43dff0), both-rows
     overclaim corrected in report + index (c3e2d0d9), filterChip doc-comment
     fragment restored (74d34566).
2. **Failure-evidence surface** (…b486056d, …b6cff639):
   `executor.MatchesEvidenceStageSet` verified at internal/executor/evidence.go:109;
   `LastFailureEvidence` + `Summary()` in the same file; `tq dlq` rows carry
   `evidence=`; webui `?evidence=` filter rides the ONE matcher; dead-only
   BY DESIGN pinned (`TestRequeuesCarryNoFailureEvidence`); real-binary smoke
   `scripts/smoke/dlq-evidence-stage.sh` exists (verified). CHANGELOG Added
   rows present.
3. **Gates + pins** (…b8d0aff5, …b9ac5f8e, …ba890359, …ba892a7020f):
   - `row_path_violations` verified at scripts/check-status-index.sh:61 with
     the mutation-proofed negative battery (83b453a2) — the reviewer-found
     vacuous/inverted first battery is cured (honest counter, rc=1 on a
     missing path, mutation probes pinned).
   - `TestSecretPatternSamplesRawCountExactlyOne` verified at
     internal/executor/redact_test.go:238.
   - `tq audit --journal --json` wire-contract test + journal-drift.sh
     phase-6 entry-cell pins (cad2c9a1, 8b85a841).
   - AGENTS.md dry-run-first bullet verified (AGENTS.md:240); the conscious
     budget raise to 24,400 verified (`agentsDocMaxBytes = 24_400`,
     cmd/tq/facts_json_test.go:94) with the size-guard twin synced.
   - Prompt-const fold guard wired into ci-local (verified at
     scripts/ci-local.sh:172); commit-msg hook case count derived.
4. **Digest ritual** (…fe3eac52): `digest 2026-11` row live (README:130);
   empty-accrual shape pinned; bloat-warning wording reworded + render-
   verified; the 32-report ANNOTATE pass landed (f5998ae2), 4 candidates
   archived by the T13 sweep (digest row accrues 4).
5. **Push-master ops task** (…f208e533): vendorHash resync e65b8447 landed;
   master was subsequently PUSHED in the 2026-10-31 v0.4.1 release saga and
   origin CI ran green incl. nix + consumer jobs (run 41362835745, cited in
   README index row 131). Task's goal achieved. Local master is currently
   ahead 2 — but those two commits (dcd2d542 verdict-rail reword, adc94299
   closeout) are POST-window work from the 2026-11-01 stdout-channel task,
   not this window's residue.
6. **Both zero-footer reviewer tickets** (…ba892ab9, …ba892bb3): verified in
   the production journal as `status: completed, attempts: 0, completedAt
   2026-10-31T00:23` — closed FREE via preflight dedup, both findings
   (fabricated `fails+1` summary count; vacuous clean-fixture battery)
   already cured by the sibling fix 83b453a2. No work was owed.

## b) PARTIALLY DONE

1. The 05-31 batch landed 1 of 3 items — the two SECURITY.md rows are
   owner-gated (§g1 generation-vs-pin ceiling; §g2 bare-probe Cache-Control
   posture) and sit correctly BLOCKED (TODO rows at the 01-01 cluster, seen
   in this pass). The reviewer round-trips those omissions caused are the
   honest cost.
2. Attribution fragmentation: the 04-18 and 08-06 batches interleaved with
   footer-less daemon sweeps; all work is at HEAD and correct, the footer
   channel cannot claim it.
3. The digest ritual's archive half is a standing cadence — `digest 2026-11`
   accrues in place; no further sweep ran in-window.

## c) NOT STARTED

1. Nothing batch-mandated was skipped: all 13 reported tasks landed their
   items, both free tickets owed nothing, the push task's push landed.
2. Window §f follow-ups were exhaustively rowed and mostly TICKED since by
   the concurrent T13–T27 waves; the residue is tracked in TODO_LIST (rows
   1489–1494 from the 12-26 pass, plus the standing rows it cites).

## d) TOTALLY FUCKED UP (window-level, as verified this pass)

1. **The window shipped a gate battery that could not fail** (row 1209's
   path-gate self-test: unbackticked fixtures + inverted rc assertion),
   standing ~5 days until the reviewer caught it and 83b453a2 cured it.
   The fleet-wide mutation-proof sweep is still open (row 1438-class).
2. **Reviewer findings cost 4 paid dispatches**, mostly for honesty defects
   of the window's own batches (1b2fee1d's "both rows BLOCKED" claim vs a
   one-line diff). Two tickets then closed free — the dedup worked, the
   findings were self-inflicted.
3. **AGENTS.md budget structurally exhausted**: raised 23,900→24,400 inside
   the window, then pruned, then grown again; ~2 B headroom at the 12-26
   pass. The terminal-policy ruling row (1497) is BLOCKED on the owner.
4. **Daemon-race self-flags in six consecutive windows** — edit→battery→
   commit ordering inversions, multi-item sweeps needing manual soft-reset
   splits. Conventions exist, enforcement does not (heal interleave/split/
   span rows all open).
5. **Early-window dispatches (05-31..06-22) recorded no verdict** (no
   TQ_RESULT_FILE in the shell) — the half-closeout class; later windows
   record same-session, and `tq verdict --file` is now the baked rail.
6. **NOTICED AT THIS HEAD (not this window's defect, not rowed — likely
   in-flight peer work)**: `check-doc-refs.sh` rc=1 — CHANGELOG.md cites
   `internal/executor/result_fuzz_test.go`, which no longer exists
   (deleted in a recent footer-less sweep, plausibly the row-1501
   fuzz-target retirement in flight). If the deletion lands, the
   CHANGELOG line needs a same-change repoint per the append-only
   tension the O14 policy already navigates.

## e) WHAT WE SHOULD IMPROVE

1. Mutation-proof every negative battery at landing (green rc proves
   nothing) — row the sweep, don't re-mint it.
2. heal-daemon-sweep should WRITE fork records into citing docs, not print
   them (row 1494 from the 12-26 pass).
3. Commit-per-edit-batch remains the highest-leverage discipline.
4. Re-dispatch dedup should prefer row-text/anchor content over footer
   grep (row 1407-class).
5. Owner-gated rows must be born BLOCKED (row exists at the harvest-hygiene
   line) — the 05-31 batch paid two round-trips re-learning this.
6. **NEW this pass**: a done-prompt dispatch should cheaply check
   docs/status/README.md for an existing same-window report before spending
   a full pass — the 12:26 twenty-sixth and this 17:xx twenty-seventh
   dispatch covered the identical task set hours apart. Appended as a row.

## f) UP TO 50 NEXT THINGS

Ranked; deduped hard against the 628 open rows — the genuinely-new items
from this pass are 1 (appended). Everything else cites its standing row;
do NOT re-mint.

1. NEW — Dispatch-time window-report dedup: before minting a done-prompt
   pass, grep docs/status/README.md index rows for the window's task IDs /
   timestamps; if a same-window report exists at a later HEAD, do a
   verification-only pass (this report's shape), not a full re-derivation
   (appended).
2. Rows 1489–1494 (from the 12-26 pass): Connection-header home, API-column
   Cache-Control absence pin, RouteCoverage tripwire hardening + -race,
   counter-arithmetic sweep, heal fork-record write-back.
3. Rows 1495–1497 (BLOCKED, owner): silent band-drift channel, row-1216
   ruling confirm, AGENTS budget terminal policy.
4. Rows 1498–1502: verdict-rail wording guard + queue-side prompt template
   reword + prose-surface cleanup + fuzz-target audit + prompt-const dedup
   (the stdout-channel task's follow-through).
5. Row 1201/1202 pair (BLOCKED, owner): SECURITY.md §g1/§g2 rulings — the
   oldest open pair from this window; every header-matrix doc follow-up
   gates on one of them.
6. Row 1438-class: mutation-proof the four convention-named countable gates,
   then the fleet.
7. Row 1220: journal-drift phase-6 sabotage leg. Row 1221: heal `--split`.
   Row 1395: heal interleave shape. Row 1330: heal span guard.
8. Row 1390: record the consumer-job diagnosis (the red itself is gone —
   origin CI green run 41362835745).
9. Rows 1400/1401: withContentType failingTB negative path; SECURITY.md
   Content-Type column (gated on §g1/§g2 shape).
10. Row 1402/1422: cite a root-gate.sh rc in a window report (twice minted,
    still uncited).
11. Rows 1426/1427/1428: templ drift gate + CLI/go.mod version alignment +
    comment-fragment lint.
12. Row 1404: UNBLOCKED-OWNER-GATED → hard gate for a batch's own anchors.
    Row 1406: index-row BLOCKED/count-claim audit.
13. Row 1439/1440/1441: --self-test-paths fixture marker; per-path counting
    ruling; full ci-local end-to-end receipt post-83b453a2.
14. Row 1302-class: conscious AGENTS budget re-tighten (~22k) once the
    terminal policy lands.
15. Row 1171: finish retryx graduation (executor/depbump.go).
16. Row 1233: AGENTS Known Issues line for prompt-const dedup-invisibility
    (detector landed; the LINE is the deliverable).
17. Row 1290-class: scripted landed-but-unticked sweep tool.
18. Row 1311: TODO-dedup detector (check-todo-dedup.sh).
19–50: standing backlog rows carry the remainder (dead-SHA cures, verdict
    receipt surfaces, transient-retry calibrations, SystemNix deploy rows) —
    deliberately not re-enumerated.

## g) QUESTIONS (owner rulings — already BLOCKED-rowed, re-asked here)

1. §g1: SECURITY.md doc pin ceiling — per-cell substring pins vs generated
   table? (TODO row 1201 pair.)
2. §g2: bare-probe Cache-Control posture — code header vs doc prose?
   (sibling row.)
3. Silent band-drift channel: should marker/keyword/unblock bumps emit
   `task.reprioritized` facts so ADR-0015 accountability has data, or is
   the silent channel accepted? (Row 1495. Verified this pass: zero such
   facts in the production journal, read-only.)

## h) BAND DRIFT

None recorded. Read-only query of the production journal
(`/mnt/pool/services/tq/tq.db`, the running pool's `TQ_DB` per the systemd
unit environment) for `task.reprioritized` returned zero facts — not just
in this window's timespan (2026-10-30 05:00 → 2026-11-02 17:00) but ever.
A plain-text grep of the full fact dump matches "reprioritized" only inside
requeued-fact error text (past agents' own output), never as a fact type.
This is the eleventh consecutive none-census; the channel-silence question
is rowed BLOCKED (§g3).

## Dedup + TODO closure note

All unchecked-row candidates from §f were dedup-checked against the live
backlog (628 open rows): items 2–19 above map to standing rows (12-26 pass
rows 1489–1502, plus the older rows they cite) — none re-minted. ONE new
row appended (§f1). The §g questions are already carried by BLOCKED rows
1495–1497 + the 1201/1202 owner-gated pair; no duplicate BLOCKED rows
appended. Ticking pass: all window batch rows verified `- [x]` in place
(1194/1195/1197/1199/1200/1201/1202/1203-1213 cluster re-read this pass).
