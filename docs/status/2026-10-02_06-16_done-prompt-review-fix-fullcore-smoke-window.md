# Done-prompt status report — review-fix / fullcore / smoke-forensics window

**Written:** 2026-10-02 06:16 CEST · **Window:** 2026-09-26 06:05 → 2026-10-01 06:39 (five tasks)
**Tasks closed this window:**

| Task | Work commit(s) | Close-out report |
| --- | --- | --- |
| 000001a0dbccd985bf0aa3ec0803c57e05b7 (review fix: dead-SHA baseline) | `fa1c03bd` | docs/status/2026-09-26_06-05_task-000001a0dbccd985bf0aa3ec0803c57e05b7.md |
| 000001a0dbccd9be35b494256bf3f8ffa87d (review fix: gate misattribution) | `a1bbd1d6` | docs/status/2026-09-26_06-21_task-000001a0dbccd9be35b494256bf3f8ffa87d.md |
| 000001a0f58abc7cddceaed8497b00000000 (README rot guard + `--model` retirement) | `7f78e21f`, `dc43c582`, `2f56ee19` | docs/status/2026-10-01_06-12_task-000001a0f58abc7cddceaed8497b00000000.md |
| 000001a0f5af5d2409a380f2fe9c00000000 (fullcore fatal-path close-before-exit) | `9a28605b` | docs/status/2026-10-01_06-26_task-000001a0f5af5d2409a380f2fe9c00000000.md |
| 000001a0f5bd17ec1703d395419b00000000 (smoke forensics batch) | `f1e03759` | docs/status/2026-10-01_06-35_task-000001a0f5bd17ec1703d395419b00000000.md |

## a) FULLY DONE (verified this pass)

1. **Dead-SHA gate review fix** (06-05 report): 13 dead-SHA cites across three
   rewrite-orphaned tokens (`fd2176c5`, `eb48b416`, `a51585e1`) baselined with
   patch-id/diff forensics per token; `check-dead-sha-refs.sh` rc=0 ("110
   baselined, 0 new") + `--self-test` green. Verified: baseline rows present in
   scripts/dead-sha-baseline.txt; the gate is green at HEAD today.
2. **Gate-misattribution review fix** (06-21 report): the 05-02 battery item now
   names `check-dead-sha-refs.sh` as the real SHA gate (the misattributed
   `check-doc-refs.sh` corrected in place); all four doc gates + root
   build/vet/test -race rc=0 re-run at close. Verified: the corrected battery
   text is in the cited report; the gate-attribution-guard residue was rowed
   (TODO_LIST "Review-fix window follow-ups").
3. **README-as-contract rot guard + `agent-pool --model` retirement** (10-01
   06-12 report): `scripts/smoke/readme-install.sh` exists, wired into ci-local
   next to bootstrap-install, negative-verified against a rotted README; the
   `--model` option is refused at parse (CLI + pool-config line) with
   remediation pointing at the managed `.crushrc` block; README.md:221
   documents the retirement. DONE-on-arrival at the dispatch — verified this
   pass by file presence + the report's green smoke run.
4. **Fullcore fatal-path close-before-exit** (10-01 06-26 report): example
   `examples/fullcore` converted to run()-returns-error; deferred
   `store.Close()`/`cancel()` run on every fatal path; the LIFO
   close-under-live-workers race found live in the first build was fixed with
   a `stop()` seam; binary-proven deadline (exit 1, exact message) + success
   (4/4) paths; `smoke/fullcore.sh` PASS.
5. **Smoke forensics batch** (10-01 06-35 report): `scripts/smoke/multi-repo.sh`
   keeps the workdir + prints its path on ANY failure (40-line pool-log tail,
   up from 8), asserts `task.requeued == 0` via the real `tq facts` renderer
   (requeued=0 printed live against the release binary); AGENTS.md smoke list
   verified pre-done for multi-repo + papdashboard-e2e; TODO row closed with
   smoke numbers (6/6 completed, 0 dead, 0 requeued).

## b) PARTIALLY DONE

1. **Fullcore postgres fatal path**: code-reviewed, compiles, never executed —
   no local postgres in that window (mirrors 06-39 §c1). Evidence is sqlite-only.
2. **Fullcore stop-seam ordering**: pinned only implicitly (deadline-path smoke
   eyeballs output; no grep-assertion on "database is closed" absence; no unit
   test pins the LIFO ordering).
3. **Keep-on-fail scope**: multi-repo.sh only; bootstrap-install.sh and
   journal-drift.sh (the other SMOKE_KEEP users) still delete their workdirs on
   failure. The keep-on-fail logic is now three hand-rolled per-script variants.
4. **Dead-SHA follow-ups**: the 06-05 report's six §f items (baseline
   `--baseline-add` mode, `TQ_DEAD_SHA_VERBOSE=1` INFO line in ci-local,
   baseline-file header sentence for the exact-match semantics, amend-appends
   codify, closeout-battery codify, reviewer template) remain UNHARVESTED
   (verified by the 06-21 §c3 grep; still absent from TODO_LIST this pass —
   rowed below). The amend-residue class (§d1 there: every sanctioned amend
   regenerates an uncitable dead SHA) is unchanged.
5. **The `--model` refusal has no pinned test surfaced in the 06-12 report** —
   the 06-12 §f3 smoke/test assertion candidate is still open (rowed below).

## c) NOT STARTED (skipped by this window; carried rows unchanged)

1. Master CI state re-check (red at `e5c386e1` per the 06-21 report; not
   re-verified here — docs-only pass).
2. ci-local end-to-end soak over the ~121-commit unpushed lineage (standing row).
3. ADR-0019 S1 flip + S2/S3/S4 serialization, dogfood cutover, and the whole
   owner-blocked decision set — untouched, correctly BLOCKED.
4. Baseline shrink policy for the `fd2176c5` row (reachable rebuild twin) —
   no owning ruling, untouched.

## d) TOTALLY FUCKED UP

1. **The dead-SHA residue class regenerates on every sanctioned amend** (06-05
   §d1): the window's own footer-attaching amend manufactured a new unreachable
   commit (`a483fdbe→fa1c03bd`), green only because nothing cites it. Three
   windows in this span hit daemon-fold/amend heals; the queue↔git attribution
   tax is now the dominant per-window overhead (see also 06-26 §a6 triple-sweep
   heal, 06-35 §d1 footer-above-attribution miss caught by
   `interpret-trailers` post-check).
2. **The 05-02 battery lie (root cause of task 2)**: a verify window whose
   entire purpose was claim accuracy cited an rc from a script that measures
   doc PATHS for a dead-SHA-gate claim. Fixed at the cite site; the CLASS
   (gate-identity by nickname) has only a filed advisory row, no enforcement.
3. **A teardown-order bug shipped inside a teardown-order fix** (06-26 §d1):
   the first fullcore conversion closed the store under live workers. Caught by
   binary execution, fixed in-window, disclosed — but the LIFO-order desk-check
   should have predicted it.
4. **Commit-ordering discipline lost twice more** (06-21 §d2 instance 7; 06-26
   §d2 three-commits-where-one): the daemon folded work footer-less while
   minutes-long batteries ran pre-commit. The edit→commit→battery convention
   now in AGENTS.md is the remedy; these windows predate its codification.
5. Nothing destructive: no pushes, no reverts of others' work, no config edits
   in any of the five windows.

## e) WHAT WE SHOULD IMPROVE

1. **Shared smoke helper** (`scripts/smoke/lib.sh`): WORK setup, EXIT trap,
   FAILED/keep-on-fail flag, prebuilt-binary branch, `tq version` print —
   collapses the SMOKE_KEEP triplication before a fourth variant lands
   (06-35 §e1).
2. **Assert cleanliness, not just exit codes**: the fullcore smoke should grep
   its deadline-path output for `sql: database is closed` absence (06-26 §e1);
   multi-repo's new zero-requeue assert is the right pattern — extend it.
3. **Desk-check defer/exit structure before building** whenever a change alters
   teardown shape (06-26 §e1) — execution-proof is necessary, not sufficient.
4. **Harvest §f lists mechanically**: the 06-05 follow-ups sat unharvested for
   six days and needed a second window's grep to prove absence. The done-prompt
   (this report) is the right harvest point; keep it.
5. **Battery gate-identity convention** (TODO row filed 06-21): cite gates by
   wired identity (script path + ci-local step name); the advisory guard row
   should land before the next misattribution.

## f) NEXT THINGS (appended to TODO_LIST, dedup-checked)

See TODO_LIST.md section "Done-prompt harvest (2026-10-02)": shared smoke lib,
keep-on-fail for the two remaining smokes, fullcore clean-output assertion,
postgres fatal-path execution, readme-install version pin, `--model` refusal
pin, the 06-05 dead-SHA follow-up harvest. Items already covered by open rows
(TQ_BIN rot-guard, gate-attribution guard, footer-placement rulings) were
skipped, not re-added.

## g) QUESTIONS FOR THE OWNER

1. Should `scripts/smoke/readme-install.sh` also run in the nightly
   fuzz/scratch schedule, or is ci-local placement sufficient coverage?
   (06-12 report §g3)
2. Task 000001a0f5af5d2409a380f2fe9c's item said "closeout turn skipped when
   the process exits on a fatal path" — the fix addressed the fullcore
   EXAMPLE's exit shape. Was queue-side closeout bookkeeping ALSO in scope, or
   is the item fully closed at the example level? (06-26 report §b2)

## h) BAND DRIFT (ADR-0015 accountability)

`grep -i reprioritized` over `tq facts` for the journal returns NO
`task.reprioritized` facts — none recorded in or around this window. All five
tasks ran at their harvest-time priority; no marker/AI/importance moves to
explain. Priority order in the window is fully attributable to the aging
ladder + stored priorities (ADR-0015), no discretionary moves.
