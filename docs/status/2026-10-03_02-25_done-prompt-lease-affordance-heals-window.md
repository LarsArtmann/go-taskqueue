# Done-prompt status report — 2026-10-03 lease-affordance + heals window

Date: 2026-10-03 02:23 CEST · Scope: the five tasks listed in the dispatch
window (00-08 → 02-14 today), their commits, and the docs the pass owns.
All claims verified against git and the working tree at HEAD `8af9c70e`.

## a) FULLY DONE

1. **internal/status ULID test flake — fixed and adjudicated** (task
   `000001a0fea1…`). Fix commit `93ef8642` (process-monotonic `task.NewID`,
   internal/task/task.go:30, ordering pinned by `TestNewID`); two closeouts
   (00-08 fix window, 00-25 re-dispatch adjudication) with fresh gate
   evidence: internal/status `-race -count=50` rc=0, internal/task and root
   gates green. The 03-07 §d2/§f1 flake is dead and its adjudication stands.
2. **Stale committed binary untracked** (task `000001a0fec1d421…`).
   Commit `def4fccb` removed the 22 MB `cmd/tq/tq` binary (last touched
   2026-09-15, predating NOTBEFORE and `--count`), gitignored it, and
   ticked its TODO row; the 01-11 window verified at HEAD (`git ls-files
   cmd/tq/tq` empty) and found no other tracked stale binaries.
3. **heal-daemon-sweep review finding fixed + pinned** (task
   `000001a0fec1da6d…`, findings from the review of `000001a0fafea7ed…`).
   The pre-flight pushed/tag refusal rails were silent no-ops —
   `git rev-list --sort=reverse` is not a valid flag (usage error → empty
   list → rail never fired). Fixed in `d6545dff` (both rails now
   `rev-list | sort`), and `335cdfc4` added two self-test cases that
   genuinely exercise the tag rail (tag inside range → refuse; tag on base
   → heal succeeds; the new case FAILED pre-fix and passes post-fix).
   The window also fixed a real portability bug the new test surfaced
   (host `tag.gpgSign=true` broke the fixture) and healed its own
   daemon-sweep race via the sanctioned script.
4. **`tq tasks` STALE marker for lease-stale RUNNING rows** (task
   `000001a0fef42f6d…`, landed done-on-arrival via daemon commit `6298e1c3`
   + closeout `1a1d661b`). `statusCell` (cmd/tq/tasks.go:218-225) renders
   `STALE` when a RUNNING row's `LeaseExpires` is past — the 05-30 c3
   stale-lease hint now visible in the list view, pinned by
   `cmd/tq/liveness_test.go` ("stale-lease" case); cmd/tq module gate green.
5. **`tq tasks` lease countdown for healthy RUNNING rows** (task
   `000001a0ff18cd…`, commit `c7b3b764`). `statusCell` now renders
   `running 9m59s` (remaining lease, rounded to the second) for RUNNING
   rows with a valid lease; `TestStatusCell` rewritten to pin
   `running 10m0s` while keeping the STALE/no-lease/non-RUNNING cases;
   cmd/tq gate green twice; TODO row closed with a DONE note. Together
   with #4 the list view now explains both halves of lease liveness.
6. **Both prior done-prompt reports honored**: the 01-05 done-prompt's
   harvested rows were executed this window (heal rails, binary untrack,
   lease affordances), and the 2026-10-01 17-40 docs-health sweep's
   conventions (index-row write point, manifest rules) were followed.

## b) PARTIALLY DONE

1. **Heal self-test refusal-cause assertions**: the new tag-rail case
   asserts outcomes correctly, but the older dirty-worktree /
   pushed-commit / empty-range self-test cases still assert only rc≠0,
   not WHICH refusal fired — the exact masking that hid the dead rail
   (01-22 report §d1, §e1, §f1).
2. **`verify_heal` check #5 backup-ref half**: defense-in-depth scan of
   `refs/original/heal-daemon-sweep` not implemented (01-22 §b2/§f5);
   the working pre-flight refusal covers the old-commit case.
3. **TODO row 429 remainder**: heal self-test hex-validation
   (metacharacter/empty-id) and multi-branch backup-ref fixture cases
   still open (01-22 §b1).
4. **STATUS column alignment**: `running 9m59s` is 13 chars against the
   `%-10s` header — cosmetic drift noted in the 02-14 report §e1; a
   dedicated LEASE column is the proposed fix (queued, §f below).
5. **Host GOCACHE/toolchain hazards**: worked around everywhere
   (`GOCACHE=/tmp/go-build-cache`, corrupt toolchain removed and
   re-downloaded), but the root cause (host symlink → /nix/store, why the
   go1.27.1 extraction was truncated) remains unproven (00-25 §b1/§c1) —
   host-scoped, owner rows.

## c) NOT STARTED

1. **Red master CI run diagnosis** (standing TODO row 430 class): still
   unaddressed; three of this window's five runs deliberately skipped
   ci-local and used per-module gates instead (01-22 §c3, 02-02 §c3).
2. **Status-index archive sweep**: 172 live rows vs the 100 threshold —
   INDEX BLOAT WARNING printed again today (`check-status-index.sh`);
   no sweep this window (the standing ANNOTATE backlog row covers it).
3. **AGENTS.md host-cache hazard row**: asked for in two consecutive
   reports (00-08 §f13, 00-25 §f1), still not written; this pass adds it
   (see docs changes in this commit).
4. **Queue-side re-dispatch dedup gate**: the window saw a THIRD
   consecutive zero-diff re-dispatch of an already-closed row (01-11
   closed at 00:36, re-dispatched at 01:11; same pattern in 00-25 and
   02-02). Standing rows 148/179/199 already ask for variants of the
   gate; nothing built yet.
5. Nothing else new: the remaining backlog (ADR-0019 S1–S4, postgres
   wiring, consumer ghost-package decision, pilot gates) is unchanged and
   mostly owner-blocked; not re-listed here.

## d) TOTALLY FUCKED UP

1. **The dead-rail bug class (review-caught, then fixed properly)**: the
   original heal-script rails shipped an invalid `rev-list` flag, printed
   refusals that could never fire, and a self-test that "passed" by
   accident (a different refusal fired first). Root causes: no red
   fixture before declaring a rail real; assertions didn't name the
   refusal. Fixed at `d6545dff`/`335cdfc4`, but the refusal-naming
   tightening across the remaining rails is still open (§b1).
2. **Three wasted windows on done work**: the queue re-dispatched
   already-closed items three times in one night (00-25, 01-11, 02-02).
   Each window correctly self-detected and produced verification-only
   closeouts (no duplicate commits — footers stayed unique), but the paid
   turns were pure overhead. Root cause is queue-side (mint-time
   done-check, rows 148/179); the per-agent step-zero
   `git log --grep <id>` ritual is the current mitigation.
3. **Daemon-sweep commit races hit two windows**: the 01-22 window had to
   soft-reset a footer-less daemon fold of its own work; the 02-14 window
   lost its first commit to the daemon and needed a scripted heal. Both
   recovered correctly with sanctioned procedures, but this is now the
   dominant per-window failure cost (and it recurred after rows 142/174/
   200 already track it).
4. **Host environment burned three gate cycles across two windows**
   (ENOSPC on the store-symlinked GOCACHE, then the corrupt toolchain
   presenting as "package X is not in std") — 00-25 §d1: the lesson from
   the first incident was read and then not applied. Documented in
   AGENTS.md Known Issues by this pass; the host symlink fix itself
   remains an owner row.
5. Nothing broken at HEAD: full root build+vet+test gates green per the
   window's reports; no dead-lettered tasks from this window's IDs; no
   regressions introduced in shipped behavior.

## e) WHAT WE SHOULD IMPROVE

1. **Self-test assertions must name the refusal** (grep stderr for the
   specific "refusing:"/"FAIL:" line) — kills the masked-rail class at
   authoring time; apply it to every remaining heal self-test case.
2. **Red-fixture-first for shell rails**, same TDD rule as Go — the
   dead-rail bug shipped only because no fixture made the rail fire.
3. **Step zero on every dispatch**: `git log --grep <Task-Queue-ID>` +
   TODO row state check. Cheap, now demonstrated three times; the
   systemic fix is the queue-side mint-time done-check (rows 148/179).
4. **Host-env probe as gate step zero** on this host (`df; go env GOCACHE
   GOROOT; ls "$GOROOT/src/unsafe"`); now documented in AGENTS.md.
5. **Lease affordance placement**: consider a dedicated LEASE column in
   `tq tasks` instead of overloading STATUS (02-14 §e1), and check the
   webui task table for lease-state parity (02-14 §f4).
6. **DONE-row note consolidation**: rows 114/115 in TODO_LIST.md now both
   carry long DONE prose for adjacent lease-affordance work — the
   one-note-plus-pointer convention wants a merge pass (02-14 §e3).

## f) UP TO 50 NEXT THINGS (top 10; the long tail lives in TODO_LIST)

1. Tighten heal self-test refusal-cause assertions (grep stderr per rail).
2. Wire `scripts/heal-daemon-sweep.sh --self-test` into ci-local (row 428).
3. Add hex-validation + multi-branch self-test fixture cases (row 429).
4. Diagnose the red master CI run blocking ci-local (row 430).
5. Queue-side mint-time done-check: suppress dispatch when the task ID
   has a terminal fact or a footer-bearing closeout commit (rows 148/179;
   three incidents this window).
6. Daemon short-delay/lockfile or footer-first fold rule (rows 142/174/
   200; two incidents this window).
7. AGENTS.md host-cache hazard row — DONE by this pass (tick it after
   this commit lands; item kept here for the audit trail).
8. Dedicated LEASE column in `tq tasks` (+ `--sort lease` / lease-soonest
   filter) to fix the STATUS-column width drift.
9. Webui task-table lease-state parity check (does `tq serve` show lease
   state at all? If not, port the countdown/STALE affordance).
10. Status-index archive sweep: 172 live rows, threshold 100 (existing
    ANNOTATE row 134; a monthly digest row is the alternative).

11–50: covered by the live TODO_LIST backlog (ADR-0019 S1–S4 flip
sequence, postgres CLI wiring, consumer ghost-package ruling, priority
pilot gates, secrets/depsweep/audit residues). Padding this list to 50
would duplicate TODO_LIST without adding value.

## g) QUESTIONS ONLY THE OWNER CAN ANSWER

1. **Lease affordance terminal state**: is `running 9m59s` inside the
   STATUS cell acceptable, or do you want a dedicated LEASE column
   (touches the header row and every column-grepping consumer)?
2. **Re-dispatch policy**: should the queue mint-time suppress tasks
   whose ID already has a terminal fact / footer-bearing closeout commit
   (three zero-diff windows this night), or is per-agent step-zero
   checking the accepted mechanism?
3. **Refusal-cause assertion norm**: should ALL heal self-test cases
   assert the SPECIFIC refusal reason (retroactively reopening the older
   cases), and should verify_heal check #5 also scan the backup-ref side?

## h) BAND DRIFT

`task.reprioritized` facts in the journal: **none recorded** — the fact
census at 02:25 CEST (all time, production journal) holds zero
reprioritized facts of any source (marker/ai/unblock/importance). The
window's claim order therefore came entirely from stored priority + the
ADR-0015 aging ladder; no priority moves require accountability notes.
