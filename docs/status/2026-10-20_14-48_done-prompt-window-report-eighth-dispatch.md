# Done-Prompt Window Report — 2026-10-20 14:48 CEST (EIGHTH dispatch)

Task: 000001a156a3410fa257bc4a217c00000000 — the same five-task window
(00:31–02:03 closeouts) already covered by seven predecessor reports
(02:31, 03:14, 09:05, 10:26, 11:09, 11:43, 12:56). This pass re-verifies
every §a claim at HEAD, reports the delta since the SEVENTH dispatch
(12:56), and closes this dispatch's own docs-health gaps: the seventh
pass minted NO CHANGELOG entries, and two shipped user-visible changes
were still uncovered — both minted this pass.

## a) FULLY DONE (verified at HEAD, 14:48)

All five window tasks remain landed and standing:

1. **Lint-gate polish** (`70f62dd9`): `--clean-cache` present in
   `scripts/lint-baseline.sh` (usage line + flag parse, verified grep);
   formatterWithGo drop-day pairing in AGENTS.md Known Issues (line
   ~293). STILL uncovered in CHANGELOG — minted this pass (see e2).
2. **Sessions-lamp + `tq stats --since` band** (`d385faa1`):
   `card-sessions-lamp` entry standing in CHANGELOG [Unreleased] Added
   (line ~64); print pin + minified css in place.
3. **dlqfix-autopsy prune semantics** (`c0fe4d38`): pin + test standing.
4. **CHANGELOG coverage audit** (`2de0c2dd`): the 2026-09-14 webui
   overhaul entry standing in [Unreleased] Changed.
5. **Dupe-review batches** (`acc519da`, `b84528f6`, `fd95b61f`,
   `88bc4480`, `d1a062a8`, `85dbd9a2`, `a786a876`, `3125a000`):
   ledger + `scripts/dupe-baseline.sh` + `.dupe-baseline.txt` all
   present; `maybeMint` five-helper pipeline in place (6 hits for
   maybeMint/buildWindow in internal/status/sweep.go); done-guard fix
   standing with its CHANGELOG Fixed entry (line ~105). STILL uncovered
   in CHANGELOG: the dupe gate + ledger themselves — minted this pass.

## b) Delta since the SEVENTH dispatch (12:56 → 14:48)

Three footered commits from concurrent sessions, all judged from their
own diffs:

1. **`623b241b`** (docs): closes the root vendor-drift row — gate
   verified green at HEAD by its owner.
2. **`5217de78`** (fix, 13:49): **AnswerPoller bound drains
   oldest-first** — the MaxAnswersPerPoll take-max applied newest-first
   while the cursor skipped everything older, so a bound-truncated poll
   silently dropped the remainder of a mass-answer session; the pin test
   had encoded an unrealizable contract and sat deterministically RED
   for four days, blocking every full battery. Rewritten to row 504's
   oldest-first design; rows 323 + 504 ticked by the owner session.
   This is plausibly the unblocking of the long-red battery leg.
3. **`58e78faf`** (feat, 13:53): consumer-path proof for tagged facades
   in `release.sh --push` (check-consumer-tag.sh tightened).
Plus daemon chores (`8d6e3e35`, `73c012c7`).

CHANGELOG had NO entry for the AnswerPoller fix — a user-visible bug fix
(paid answer-loss class) — minted this pass under [Unreleased] Fixed.

## c) PARTIALLY DONE

1. Same standing partials as pass seven: no window session ran the full
   `ci-local` battery; per-module scopes only. The 5217de78 fix removes
   the known red leg — the next full pass is now unblocked evidence-wise
   (row 977's in-situ dupe-gate run still open).
2. The floor pairing (root go.mod 1.27.2 vs `cmd/tq/go.work` 1.27.1,
   row 957/935) is STILL unresolved at 14:48 — ~14h old, now the
   ninth-dispatch-verifying blocker if nothing lands.

## d) NOT STARTED (by this window)

Nothing new: the window's own five tasks shipped in full. All backlog
asks ride existing rows (dupe-baseline --self-test 975, tool provenance
976, in-situ ci-local 977, Every>1 rows 630/631, tick-sweep 983, dedup
987, verdict-at-exit 993).

## e) TOTALLY FUCKED UP

1. **Eight paid dispatches for one done-prompt.** Passes 1–6 committed
   full reports and recorded ZERO verdicts (row 993's class); the
   seventh finally recorded one; this is pass eight, re-verifying the
   same five tasks for the eighth time. The mint-time gate (row 993's
   companion, shipped 2026-10-20) covers re-MINT, but the queue-side
   "done-prompt task whose verdict/receipt is absent" surface (row 1000)
   is still unbuilt — until it is, this loop keeps costing full claims.
2. Nothing new by THIS pass's hand; no reverts, no daemon conflicts, no
   foreign damage observed in the delta commits.

## f) WHAT WE SHOULD IMPROVE

1. Land row 1000 (doctor/audit surface for DONE tasks that never
   recorded a verdict) — it is the mechanical end of the eight-dispatch
   loop that this pass is the latest instance of.
2. CHANGELOG-at-ship: 5217de78 landed a user-visible fix with no
   changelog row; three passes of done-prompt verification greps never
   caught it either (this pass's grep did). The coverage-audit script
   (row 969) should include recent footered fix commits, not just
   status-report §a items.
3. The same-floor pairing discipline: every blocker row (957) aged
   another 2h across this pass with no movement while nine verification
   passes each re-measured it. One owner ruling unblocks it.

## g) NEXT (dedup-held: 0 new TODO rows this pass)

Every candidate is already rowed — 963–1001 cover the window's family;
the delta's asks (none) mint nothing. The highest-leverage open work at
HEAD, ordered: 957 floor pairing → 977 full ci-local with the dupe gate
in situ (now unblocked by the 5217de78 battery fix) → 975 dupe-baseline
--self-test → 1000 verdict-absence doctor surface → 983 tick-sweep (522
open unchecked at this pass's start) → 987 dedup pass.

## h) QUESTIONS (owner-only; all riding existing rows — zero new)

1. Floor pairing: does the 1.27.2 root bump stand (raise
   cmd/tq/go.work + CI setup-go together), or revert at root? — row 957.
2. Every>1 done-guard semantics at rewind replays — row 630.
3. `errInFlight` sentinel-as-control-flow vs `(time.Time, bool)` — the
   BLOCKED row from the 02-31 appends.

## i) BAND DRIFT

`tq facts --type task.reprioritized` → `(0 facts)` (rc=0, run 14:47
against the production journal; EIGHTH consecutive census — the journal
holds no reprioritized fact type at all, matching all seven prior
censuses). Nothing to account for under ADR-0015; all observed
claim-order movement is stored priority + aging. The standing irony now
has nine data points: the repo-broken floor blocker (row 957) has
produced zero priority movement across every census while consuming a
verification pass per dispatch.

## Appendix — docs-health pass notes (14:48)

1. **CHANGELOG**: minted three [Unreleased] entries this pass —
   AnswerPoller oldest-first drain (Fixed), the accepted-dupe-group
   ledger + dupe-baseline growth gate (Added), lint-baseline
   regen-always-cleans + `--check --clean-cache` (Changed). Append-only
   respected; no existing rows touched.
2. **Status index**: this report indexed on creation (top of the
   2026-10-20 cluster; ONE index-row write point respected).
3. **No archive moves**: the five closeouts and seven predecessor
   done-prompt reports all carry open §f items by design (KEEP-OPEN);
   no 2026-10-20 file qualifies for archived/.
4. **TODO_LIST.md**: 0 rows appended (dedup held — eighth pass), 0
   ticks, 0 rewords; 522 open unchecked / 155 checked at pass start.
5. **README / ROADMAP / FEATURES**: greps for the delta's nouns
   (AnswerPoller, vendor-drift, consumer-path) against the living docs →
   only pre-existing accurate rows; no edits needed.
6. **Gates this pass**: check-status-index, check-todo-list,
   check-doc-refs on the final tree before commit.
7. **Verdict**: `tq verdict` runs in the SAME shell as the commit,
   followed by a `test -s` receipt — the seventh pass's practice,
   continued.
