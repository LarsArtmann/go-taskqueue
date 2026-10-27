# Done-Prompt Window Report — Sixteenth Dispatch

**Report:** 2026-10-27 10-57 · fifth re-run of the twelfth dispatch's window (task
000001a17bfb2501 re-claimed a FIFTH time; commit 4e1e3f35 = fifteenth, b3e32b05 =
fourteenth, abe06de7 = thirteenth, 3ce1609d = twelfth)
**Window covered:** 2026-10-25 ~04:00 → 2026-10-26 ~03:30 — the same 21-task window as
the twelfth through fifteenth dispatches. **Read together, do not re-derive:** the
twelfth report carries the per-stream verification and gate receipts for all 21 tasks,
the fifteenth re-verified the artifact set at 10:07 HEAD. This report verifies state at
10:57 HEAD, records the delta since the fifteenth, and closes in the order commit →
verdict → verify receipt.

## Why a sixteenth dispatch exists

The fifteenth (4e1e3f35, 10:19) committed its report, declared the verdict write the
final action — and this dispatch exists, which means the queue re-claimed the task
again. `$TQ_RESULT_FILE` was empty at this session's start (0 bytes, 10:46, by
allocation design). Whether the fifteenth's verdict write also failed is not verifiable
from this shell: `.tq/` carries no verdicts directory in-repo and no receipt surface is
readable here. The re-claim itself is the only evidence, and it is consistent with a
fifth consecutive half-closeout. This dispatch performs the commit and the `tq verdict`
in one closing sequence and verifies the receipt file is non-empty as the FINAL action —
the step now missed (or unverifiable) five times running.

## a) FULLY DONE

Cumulative accounting (twelfth §a1-19, re-verified by the thirteenth and fifteenth,
zero contradictions): **19 of 21 window tasks verified landed with gate receipts**;
1 paid round-trip repaired mid-window (false row-896 tick, a7316b30); 1 task
(000001a173ee1329, reviewer fix) has no closeout and no locatable commit (disposition
audit rowed: TODO row 1216; journal-blind per §h).

Fresh spot-verifications at 10:57 HEAD (this dispatch's own checks):

- `TestServeHeaderMatrix` present (internal/webui/headermatrix_test.go) — the
  SECURITY.md response-header matrix pin (task 000001a1755938f5).
- `cmd/tq/evidence_test.go` present — the `--evidence-stage` surface
  (task 000001a1759de318).
- `fakeAuditProbe` const present in internal/executor/redact_test.go with the
  grep-extraction contract (tasks 000001a175d4d2eb / 000001a175e28e90).
- `scripts/check-archive-eligibility.sh`, `check-poll-loudness.sh`,
  `check-push-timing.sh` all present (tasks 000001a1754b7de3, 000001a171967255,
  000001a175f4de40 legs).
- `cmd/tq/go.work:12` reads `go 1.27.3` and `scripts/check-toolchain-pins.sh` pins
  every tracked go.work `go` directive to the floor (TQ_TOOLCHAIN_EXTRA_GOWORKS hook)
  — **NEW finding this pass: TODO row 1009's work landed but the row sat UNCHECKED**
  (its duplicate row 1148 IS ticked) — ticked this pass with evidence.
- Windows-green confirmation row verified ticked with run 40175514907
  (task 000001a172245af9, commit a4c91fb4) — unchanged from the fifteenth's check.
- Doc gates green pre-write: `check-status-index.sh` ok (full; the pre-existing f26
  TRAILER WARNINGs are the known cluster), `check-todo-list.sh` ok (4 advisory warns,
  0 structural).

**NEW SINCE THE FIFTEENTH (the actual delta of this pass):**

1. **Env-streak foreign-dirt batch landed and closed** (concurrent agent, outside this
   window): a0a343eb parks dirty-tree preflights instead of burning them via the
   3-streak env-streak breaker (`executor.ErrDirtyTree` typed marker, escalating ladder
   cap 15 min, no attempt burn; BEHAVIOR CHANGE flagged in CHANGELOG), closeout
   6a1bcc9d (task 000001a17ca0bd7c, TODO row 974) indexed, +5 deduped follow-up rows.
   **Not pushed** — origin/master is still e2042b28 (verified via `git ls-remote`),
   so master now carries 3 unpushed commits past the red-validated push HEAD.
2. **Row 1009 ticked** (landed-but-unticked cure, see §a spot-checks).
3. **Origin CI state re-checked**: latest origin run is still 40476763162 = FAILURE
   (test-windows module-isolation + linux windows-cross-compile); no green run has
   validated e2042b28 yet, so the cure row stays open and the next push will mix the
   CI-cure with the env-streak behavior change.

## b) PARTIALLY DONE

All carry-over, re-verified or cited from the fifteenth, still open at 10:57:

- **task 000001a173ee1329** (reviewer fix): no closeout, no commit under any grep;
  unauditable while the production journal is unreachable (row 1216).
- **Origin-CI red at e2042b28** (run 40476763162) — uncured; logs admin-gated; now
  with 3 more unpushed commits stacked on top (§a1), so the eventual validating run
  mixes cure + behavior change.
- **Archive bar is gate-only** — nothing enforces `check-archive-eligibility.sh` at
  `git mv` time (row 1158, doubly dead: unreachable anchor SHA + mirror landed);
  ~210-report strike pass not run (row 1188).
- **Evidence-stage follow-ups** (webui list filter, shared reader extraction,
  comma-set filter, non-dead-no-evidence pin, smoke case — rows 1172-1177).
- **Push-timing guard inspects only ref TIPs** (row 1193); status-loop poller fix
  empirical, not stress-proven (row 1194).
- **Doc-pin ceiling**: TestSecurityDocMatrixMatchesGoMatrix covers header-name sets,
  not cell prose (rows 1164/1165); headermatrix_test.go lint debt (rows 1166/1167).

## c) NOT STARTED

- **First digest-ritual pass over open TODO rows** (rows 1186/1224): open rows grew
  599 → **605** between 10:07 and 10:57 (+6 in under an hour; 5 of them from the
  env-streak closeout's own follow-up appends). Accretion still outpaces triage.
- **Verdict-receipt ENFORCEMENT gate** (fourteenth §f1 row) — NOT landed; this
  re-dispatch is the fifth datapoint.
- **Dead-SHA citation mass at HEAD** (~801 cites; row 1148 ruling + execution row).
- **Reviewer-fix cross-reference protocol pin** in AGENTS.md (row 1122 family).
- **Weekly ANNOTATE pass over live reports** — superseded by the ratified monthly
  digest ritual (2026-10-26 02-22 batch); this dispatch deliberately performs NO
  corpus archive/annotate sweep (the ritual owns it; mass archive from a done prompt
  would duplicate the class row 935's ritual branch retired).

## d) TOTALLY FUCKED UP

1. **The half-closeout is now a five-instance class on ONE task.** Twelfth (3ce1609d),
   thirteenth (abe06de7), fourteenth (b3e32b05) each left `$TQ_RESULT_FILE` empty with
   proof in-repo; the fifteenth declared a verified receipt and the task was
   re-claimed anyway (this dispatch). Six paid turns on one closeout contract whose
   hazard text names the exact failure. Nothing tool-enforces the verdict write; until
   the enforcement gate lands, every done prompt should be ASSUMED to drop it.
2. **Landed-but-unticked rows persist even when the row tells the truth about itself.**
   Row 1009 carried a 2026-10-25 annotation naming the exact landing conditions
   (go.work → 1.27.3, not 1.27.2), the work landed (go.work:12 + toolchain gate
   go.work leg), and the row sat unchecked for two days while its DUPLICATE (row 1148)
   was ticked. Two rows, one work item, asymmetric tick state — the exact noise the
   digest ritual (row 1224) and the landed-but-unticked sweep exist to kill. Found
   this pass only because the dispatch went looking; the fleet needs the mechanical
   sweep more than another manual catch.
3. **Backlog accretion beats triage, accelerating**: 592 → 596 → 599 → 605 open rows
   across four dispatches in one morning. Each closeout mints 3-6 follow-ups; zero
   digest passes have run. The queue is paying for bookkeeping that compounds.
4. **The origin red is aging unvalidated**: e2042b28 pushed 09:53 yesterday-today,
   failed CI in 3m21s, and master has since accrued 3 more unpushed commits — every
   hour widens the diff the eventual validating run must bisect if the red turns out
   to be real. (No local regression evidence: root gates green at HEAD per the
   env-streak closeout's battery.)
5. **Inherited from the window (verified by predecessors, unchanged)**: two paid
   round-trips from uncited tick claims (rows 896/942; tick-claims gate row 1151
   unlanded); footer-less daemon folds making prompt work dedup-invisible (rows
   1184/1192); heal-daemon-sweep over-heals footered commits (row 1196); the
   in-session write-revert race unexplained (row 1195).

## e) WHAT WE SHOULD IMPROVE

1. **Enforce the verdict, don't document it** (fifth datapoint): WARN/FAIL when a
   same-day `docs/status/<ts>_done-prompt-*.md` names a Task-Queue-ID with no
   `.tq/verdicts/<id>` receipt — AND make the receipt surface agent-readable, so a
   re-dispatch can verify its predecessor's claim in one `ls` instead of inferring
   from the re-claim itself. Today neither half of that loop is observable from an
   agent shell.
2. **Run the landed-but-unticked sweep as the digest ritual's first leg** (row 1009 is
   its second confirmed catch after row 897): grep every open row's named
   symbols/paths at HEAD, tick with evidence, feed the residue to the digest pass.
   One mechanical sweep would have caught 1009 two days ago.
3. **Close duplicate-row pairs wholesale in the digest pass**: when two rows name one
   work item (1009/1148), the digest verdict must land on both in the same commit —
   the asymmetric state is what made 1009 invisible (a reader ticking 1148 believes
   the item closed).
4. **Pre-push gate receipt for wave pushes** (standing, fifteenth §e2): an 84-commit
   push with no green local receipt bought a 3-minute origin red; the next push adds
   the env-streak BEHAVIOR CHANGE to an unvalidated base. Owner ruling rowed BLOCKED.
5. **Journal reachability for done-prompt sessions** (row 1217) — fifth consecutive
   re-hit; `tq facts` against the stale repo-side `./tasks.db` (189 facts, newest
   2026-09-09) keeps §h structurally blind every single time.

## f) NEXT THINGS

The backlog (605 open rows) already densely covers the window's follow-ups; every
candidate this pass surfaced mapped to an existing row on dedup-check — same
conclusion as the fifteenth's §f. Changes this pass:

- **Ticked**: row 1009 (evidence in §a) — evidence-backed tick, not an append.
- **Appended (1 new + 1 BLOCKED question row)**:
  1. **Verdict-receipt + re-claim loop visibility** (§g1, filed BLOCKED): rule whether
     a re-dispatched done prompt may close from verified predecessor reports via a
     delta-only report + no-op verdict after N verified runs, or must always
     re-derive; and where the verdict receipt surface lives so agents can verify it.

Everything else is already rowed: origin-CI red cure, digest-ritual first pass,
verdict-receipt enforcement gate, dead-SHA execution, unreachable-anchor audit,
push-policy (BLOCKED), journal reachability (BLOCKED), evidence-stage follow-ups,
archive-bar enforcement, poller stress, doc-pin ceiling.

## g) QUESTIONS FOR THE OWNER

1. **Re-dispatch closure economics** (NEW — filed as the BLOCKED row above): the same
   done-prompt task has now consumed six paid turns; four produced reports verifying
   the same 19/21 landing state, and each successor re-verified its predecessors
   rather than re-deriving. Is the verified-predecessor + delta-only shape (fifteenth,
   sixteenth) the sanctioned steady state — i.e., may a future re-claim close with a
   thin delta report and a no-op-style verdict once N predecessors verify clean — or
   is full re-derivation required every time? (Queue-side landed-dedup, asked once
   before for work re-dispatches in the 2026-10-26 02-48 closeout §g2, never ruled.)
2. **Push policy** — unchanged, already rowed BLOCKED (fifteenth §g1); do not re-add.
3. **Production-journal reachability** — unchanged, already rowed BLOCKED (row 1217);
   it is also the fix that un-blinds §h and closes the 000001a173ee1329 audit.

## h) BAND DRIFT

**None recorded.** `tq facts -type task.reprioritized` returns 0 facts (re-run
2026-10-27 10:52 against the store reachable from this shell; 189 facts total, newest
2026-09-09 — the repo-side store is stale and `TQ_DB` is unset in agent shells, so a
move recorded only in the production journal would be invisible here — row 1217 is the
remedy). Within what is verifiable: no priority changes need explaining for ADR-0015.

## Receipts (this dispatch)

- Pre-write gates: `check-status-index.sh` ok (full); `check-todo-list.sh` ok
  (4 advisory, 0 structural).
- Verifications behind §a: file presence greps at 10:57 HEAD; `git ls-remote origin
  master` → e2042b28; `gh run list` → latest origin run 40476763162 (failure);
  `cmd/tq/go.work:12` + `scripts/check-toolchain-pins.sh` go.work leg read directly;
  TODO row 1009/1148 states read at the file.
- Verdict: `tq verdict` executed immediately after the commit lands; receipt file
  verified non-empty as the FINAL action.
- TODO_LIST deltas this pass: 1 evidence-backed tick (row 1009), +1 BLOCKED append;
  band-drift query receipt in §h.
- Daemon-sweep heal (the race hit THIS dispatch too): the auto-commit daemon folded
  the report file + row-1009 tick into footer-less 14d54f08 between edit and commit;
  healed via `heal-daemon-sweep.sh --from 14d54f08^ --retitle …` (HEAL OK, backup ref
  dropped). Fork-records: `14d54f08→77f680d7` (sweep, footered + retitled),
  `43b9a22a→c907c4dd` (index-row + append commit, preserved verbatim). Nothing cited
  the pre-heal SHAs; post-heal citation check clean.
