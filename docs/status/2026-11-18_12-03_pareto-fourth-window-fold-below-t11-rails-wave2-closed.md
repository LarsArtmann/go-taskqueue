# Pareto 4th window — fold-below + both daemon shapes, T11 commit-task rails, wave-2 closed, CI cure staged

- **Date:** 2026-11-18 12:03 CET (window: ~11:15–12:05)
- **Repo:** go-taskqueue · master · direct owner dispatch ("GET SHIT DONE!
  The WHOLE TODO LIST!"), concurrent agent session active (poolhealth/,
  TODO_LIST.md, deploy/systemd/, docs/problems/ territory)
- **Commits (local, unpushed at report time):** `a981b448` (wave-2
  annotations) → `875a43df` (fresh:skip 11-04) → `41587f59` (T10 fold-below,
  incl. shellcheck cures, collapsed from a 3-commit daemon split) →
  `3ebd0237` (T11 commit-task rails, daemon-fold amended). Pushed earlier
  this window: `abb9e1e5`+`23056a36` (AGENTS-size cure + 3rd-window report,
  rebased onto overnight `cd5bf85d`).

## a) FULLY DONE

1. **Daemon-folded report commit healed** — HEAD `b8a2b16b` was a
   footer-less daemon chore holding exactly my 2 report files (verified
   `git show --stat`); amended into my subject `21c84a7f`. Local-only at
   the time, so the amend was policy-clean.
2. **Cure pushed**: rebased my pair onto overnight `cd5bf85d` (2-line fuzz
   corpus, no overlap) → pushed `23056a36`. Origin had moved twice
   overnight (`0db2ca90`, `cd5bf85d`); the first push attempt was refused
   non-fast-forward and resolved by rebase, not force.
3. **T03/T04 wave-2 CLOSED** — all 8 remaining 2026-10-02 reports
   annotated: d1–d6 re-dry-run-verified 1:1 after the /var/tmp re-emission
   (41/30/27/70/29/24 action lines), d7 (08-04, 19 rows) and d8 (08-06, 18
   rows) derived fresh with grep-verified evidence (metachar/empty-id
   self-test cases exist at heal-daemon-sweep.sh:762/:765; env-var
   transport at :257; the three 2026-10-02 foreign sweep shas are ORPHANED
   reflog objects — no branch contains them, rebuilt lineage pushed →
   wontdo/moot verdicts). Applied 258 verdicts, check-rows "8 file(s)
   complete", docs-gates **PASS** after fresh:skip markers on my own 11-04
   report's session-in-flight §f items. Committed `a981b448`+`875a43df`.
4. **Stale routed anchors caught pre-apply** — d1/d2 cited `TODO_LIST:284`
   as the owner-ruling row; TODO_LIST shifted (row 284 is now SQLITE_BUSY)
   because the concurrent agent edits that file. Re-pointed 9 routed values
   to the stable Pareto-T30 digest anchor BEFORE applying (sed, line counts
   preserved, re-dry-run green). Wave-1's already-pushed annotations may
   still carry the stale anchor (historical; see e).
5. **T10 LANDED** (`41587f59`) — `is_daemon_sweep` recognizes BOTH daemon
   shapes (literal `chore: auto-commit N changed file(s) (heuristic)`
   subject OR `Daemon: true` body marker, row 1825); non-daemon footer-less
   commits in a heal range are now NAMED to stderr (exactly-mine
   confirmation); new `--fold-below <sweep-sha>` mode scripts the
   2026-11-07 sanctioned manual recipe (soft-reset + commit -C + tree/footer
   verification + fork records for both rewritten shas) for the
   daemon-commit-BELOW-footer shape the `--from` walk cannot converge on
   (row 1835). Self-test 38→45 checks (AI-titled heal, both fold shapes,
   5 refusals, dry-run). AGENTS synopsis updated compress-only
   (27,596/27,600, guard green).
6. **T11 LANDED** (`3ebd0237`) — commit-task.sh: `--amend` on an own-footer
   HEAD with a footer-less commit below now WARNs naming it + the
   fold-below/heal pointer instead of silently no-op amending (rows
   1872+1914, which are duplicates of each other); every named file must
   actually differ in the resulting commit or the script fails loudly
   (row 1817 — the daemon-swept-the-named-files residue shape; includes the
   root-commit `--root` diff-tree case my first draft missed and the
   self-test caught); a footer-less commit below a NEW fold is named
   (row 101). Self-test green with 3 new pins.
7. **T02 receipts** — `./scripts/test-cmd-tq.sh` rc=0 (canonical cmd/tq
   gate, log at /var/tmp/pareto-battery/t02-cmdtq.log);
   `./scripts/ci-local.sh` rc=1 where the ONLY failing leg is check-ci
   refusing because origin's latest run is red — and that red run
   (`23056a36`, SC2164) PREDATES my local tree; the gate's own message says
   so and offers CI_CHECK=off. Every other leg green (1 FAIL total in the
   captured log).
8. **Three daemon races, three heals, zero losses** — (i) report commit
   fold (amend), (ii) T10 split: daemon took the script (+231) below my
   AGENTS-only commit below my shellcheck fix → soft-collapse
   `git reset --soft 875a43df` + one commit `41587f59`; (iii) T11 fold
   (amend `3ebd0237`). Each verified exactly-mine by `git show --stat`
   first.

## b) PARTIALLY DONE

1. **T36 (CI watch + tick chain)**: pushed cure `23056a36` went RED —
   `scripts/verify-fork-record.sh:83` SC2164 (bare `cd`), the T06 tool from
   the 3rd window. Cure built + verified locally (`41587f59`: cd guard
   `|| return 1`, self-test rc=0; plus SC1083/SC2034 fixes in my own new
   T10 code caught by today's local gate run) — **UNPUSHED at report time**
   (timing window + report-first ordering). Green-run tick batch therefore
   still owed.
2. **TODO ticks owed (deliberately sequenced after green CI)**: rows 105
   (T09, cite pushed `35393721`), 1825+1835 (cite `41587f59`), 1872+1914
   (duplicate pair, cite `3ebd0237`), 1817, 101, and the CI-reds rows
   1687/1385/1709 (cite the green run id once it exists).
3. **Digest archive half**: `check-archive-eligibility.sh` → "no archive
   candidates" — every wave report holds routed owner-ruling items.
   Honest zero-archive pending the owner's §g1 ruling (carried).
4. **T02 final green receipt**: ci-local re-run owed AFTER the push lands
   green (check-ci leg will then pass; everything else already green).

## c) NOT STARTED (this window; plan T12+)

- T12 (AGENTS budget twin guard), T13/T14 (check-agents-size --self-test +
  delta print, pre-edit rule), T15/T16 (deploy cadence — touches
  deploy/systemd, concurrent-agent territory, needs coordination),
  T17–T20 (smoke-lib), T21–T23 (SSE cluster, commit-msg pins), T24
  (erraudit sibling session), T25/T26 (cqrs archive ritual), T27–T29
  (IO-review rows), T30 (owner digest), T31/T32 (JSON pin, facade-parity),
  T33–T35 (residue batches + pool dispatch wave).

## d) TOTALLY FUCKED UP

1. **Shipped a red origin CI run (SC2164)**: T06's verify-fork-record.sh
   passed the 3rd window's local ci-local (check-script-syntax green there)
   but CI's shellcheck flagged line 83 — and TODAY's local run ALSO flags
   it, so the 3rd-window local gate ran a shellcheck that did not (version
   drift local-vs-CI, confirmed by today's local behavior change). The
   finding class: a script tool landed with its cd-guard style legal under
   one shellcheck and a warning under another. Cost: one red origin run +
   one cure round-trip.
2. **Session-shell rc trap hit AGAIN**: `git push … | tail -5; echo rc=$?`
   recorded tail's rc=0 over a FAILED push — the third window-shell rc
   artifact this session family. Caught by reading the error text, not the
   rc. rc-to-file discipline must exclude pipes entirely.
3. **My own new code carried two shellcheck findings** (SC1083 unquoted
   `HEAD^{tree}` inside `$()`, SC2034 unused `ai_sha`) — I committed T10
   before running check-script-syntax on it; the gate caught them only in
   the post-commit sweep window. Same class as d1 but local-only.
4. **Two edit-tool failures from miscounted hex zeros** in old_string
   task-ids (no damage, but two wasted round-trips): copy identifiers
   byte-exact from file output, never from memory.

## e) WHAT WE SHOULD IMPROVE

1. **Run `scripts/check-script-syntax.sh` on every touched script BEFORE
   committing it** — the daemon race makes post-commit gate catches cost a
   heal cycle; the advisory hook says so and I paid it once this window.
2. **Pin or probe the shellcheck version skew** (d1): either pin CI's
   shellcheck revision alongside the gate or add a self-test that records
   the local shellcheck version next to gate receipts, so a green local /
   red CI pair is diagnosable in one look.
3. **Wave-1 stale-anchor follow-up**: the pushed wave-1 annotations cite
   `TODO_LIST:284` (now a SQLITE row). A small annotate repair spec
   (strike-through-preserving, dry-run-first) could re-point them at the
   T30 digest anchor like wave-2 — or accept them as historical records of
   where the row was. Owner taste; not done unilaterally.
4. **Python-heredoc edit discipline held**: all-asserts-before-write made
   the failed anchor chain a no-op (nothing written). Keep that shape;
   consider asserting the FULL edit list up front so a mid-chain failure
   names the missing anchor in one pass.

## f) NEXT (priority order)

1. Push `41587f59`+`3ebd0237` (timing window open at report time) → watch
   the origin run to GREEN (T36 close).
2. Tick batch (one commit): rows 105 (cite `35393721`), 1825, 1835 (cite
   `41587f59`), 1872+1914 (duplicate pair — tick both, note twin), 1817,
   101 (cite `3ebd0237`), CI-reds 1687/1385/1709 (cite green run id).
3. T02 final: ci-local re-run on the pushed green origin → rc=0 receipt.
4. T12: twin guard test (agentsDocMaxBytes ≡ check-agents-size constant)
   + ci-local wiring.
5. T13/T14: check-agents-size --self-test + delta print; AGENTS pre-edit
   size-check rule (budget: 4 B headroom at HEAD — fold carefully).
6. T15/T16: deploy cadence (SystemNix input flip + system tq rebuild) —
   deploy/systemd is concurrent-agent territory: coordinate before
   touching.
7. T17–T20 smoke-lib extractions (stub-agent.sh, routed_run trio,
   negative-fixture helper, exec-bits sweep).
8. T21/T22 SSE cluster triage + first 5 pins; T23 commit-msg pins batch 1.
9. T24 erraudit sibling session (push 4 commits, close #12/#13/#14).
10. T25/T26 go-cqrs-lite archive ritual (triage + execute).
11. T27–T29 IO-review in-repo rows; T30 owner-decision digest file;
    T31/T32 JSON pin + facade-parity wiring.
12. T33–T35 done-prompt residue batches + the 600-row pool dispatch wave.
13. Wave-1 stale-284-anchor repair spec (e3; owner taste).

## g) QUESTIONS I CANNOT FIGURE OUT MYSELF

1. **Archive bar vs routed items** (carried from 11-04 §g1 and 21-59):
   every wave report holds genuinely-open routed owner-ruling items that
   keep it unarchivable under the every-item-resolved bar. Ratify
   "annotated-but-unarchivable is honest", or loosen the bar for
   routed-only residue?
2. **Push policy for CI cures** (carried from 11-04 §g2): when origin is
   red and the fix is local, the timing guard forces a ~10 min wait per
   push attempt. Keep the friction, or exempt spans that are exactly a
   CI-red cure?
3. **Standing battery home** (carried from 11-04 §g3): confirm
   `/var/tmp/pareto-battery` as the session-artifact home (or name a
   repo-ignored dir you prefer) so the /tmp-wipe class cannot recur?

— Window totals: 5 local commits (2 healed daemon folds + 1 soft-collapse
inside them), 258 annotations applied + 37 derived fresh, 2 tools extended
(heal-daemon-sweep +231 lines/45 self-test checks; commit-task +60 lines/3
new rails), 1 red origin run root-caused + cured locally, 3 daemon races
survived unharmed, 0 pushes of the final span yet.
