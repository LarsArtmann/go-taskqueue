# Done-prompt window — reviewer-fix seams, smoke pins, budget-meter a11y (2026-11-22 16:05)

**Window span**: 2026-11-21 ~19:00 → 2026-11-22 15:14 (5 tasks listed at dispatch)
**Dispatch dedup**: full pass — docs/status/README.md index grepped for all 5
window task IDs (zero hits) and for the window span (nearest neighbor is the
2026-11-21_02-21 done-prompt covering 11-19→11-20). No same-window report;
this is a full derivation.

**Journal caveat**: this shell's `tq facts` reads the cwd fallback
`./tasks.db` (TQ_DB is denylist-stripped in agent shells, known hazard). That
journal holds 397 facts but ZERO entries for any of the 5 window task IDs —
it is not the dispatching pool's journal. Completion evidence below is
therefore git-derived (footers, closeouts, HEAD greps), which is honest
provenance per the trust vocabulary; the §h band-drift read shares the same
caveat.

## a) FULLY DONE (verified at HEAD 7791b46e)

1. **Agent-pool flag-parity reviewer fix** (dispatch task
   000001a1fbb309...; fix landed under the closeout's own ID
   000001a200f919594e67f1e7ca3300000000 — see §e1): commit 40ee6fc2
   extracts `newAgentPoolFlagSet` as the single registration point and
   derives `TestAgentPoolOptionsFieldParity`'s registered surface via
   `fs.VisitAll` — the hand-pinned self-referential flag list (a flag added
   without a struct field failed nothing) is gone by construction. Verified:
   commit in history, `VisitAll` present in cmd/tq/flag_parity_test.go.
   Closeout report committed (0e246016).
2. **Worker flag-parity reviewer fix** (000001a200f9195c5b...): commit
   6e1c4245 — `newWorkerFlagSet()` seam (cmd/tq/main.go:648-674 verified at
   HEAD) mirrors the serve pattern; the parity test now derives the
   registered surface from the real FlagSet (2 `VisitAll` uses in
   flag_parity_test.go), the false "help-text.sh pins the worker surface"
   comment is gone, three serve/api flags that were never worker flags are
   unlisted by construction. Full cmd/tq gate green per closeout
   docs/status/tasks/2026-11-22_15-11_task-000001a200f9195c5b7c81f4bdc000000000.md.
3. **`tq serve` flag-list pin in the smoke** (000001a1f751764...): the pin
   LANDED via the daemon sweep b2ecf030 ("...+ help smoke flag pins") —
   scripts/smoke/help-text.sh:85 `expected_serve_flags` pins the exact
   serve set including `health-metrics`, with a reverse grep so an
   unregistered smoke-listed flag fails too (lines 85-94 verified at HEAD).
   The task's own footered commit ca3bb089 is the TODO tick; the work ride
   is disclosed below (§e1). Dispatch text's ask (flag drift incl.
   `--health-metrics` fails the SMOKE, not just the unit test): met.
4. **Budget-meter `aria-valuetext` evaluation + adoption** (000001a1ff8ad6...):
   commits 56c1db19 (aria-valuetext "N of M" in fragments.templ:72 + test
   pins webui_test.go:1223/1243, raw-over-cap included) and 1fb98179
   (CHANGELOG Changed row + verdict tick: ADD adopted, native `<meter>`
   rejected — class-quantized CSP-safe fill beats no announcement). Both
   verified at HEAD.
5. **Exec-bits restore** (000001a1fbb3088...): 7fbead01 chmods
   scripts/smoke/budget-meter.sh + scripts/lib/receipt.sh back to 0755
   (verified `-rwxr-xr-x` at HEAD) — the pre-existing
   check-smoke-exec-bits gate red the done-prompt-residue batch introduced
   is cured, with a correction note appended to the falsified closeout.
6. **Adjacent window work (same span, different task IDs)**:
   1991f721 makes smoke `wait_dashboard` return nonzero on timeout instead
   of `fail()`-exiting (budget-meter's serve-log-tail fallback was dead
   code) + closeout 48387a24; a70d8878/fc8a41f7 shared scratch-serve smoke
   helpers; d0afa842 the dead-field parity audit this window's two
   reviewer findings were filed against.

## b) PARTIALLY DONE

1. **Worker parity is forward-only** (closeout §b1, rowed): the derivation
   pins flag EXISTENCE + the wired five, but worker lacks the agent-pool
   test's both-directions audit (pinned flag folding into NO field fails
   nothing). Rowed by the 15-11 closeout — NOT re-listed in §f.
2. **No value-level parse-wiring pin for the worker seam** (closeout §b2):
   serve has TestServeFlagParseWiresConfig; worker's flag→Config fold is
   hand-wired inline. NEW row appended (§f1).
3. **Help-text pins cover serve only**: worker/api `-h` surfaces stay
   unpinned in the smoke (the 15-11 closeout's own rowed follow-up — not
   re-listed).

## c) NOT STARTED (window-skipped backlog)

- The window carried no dispatch for: docs-gates freshness-matcher cure,
  `tq tasks --json` empty-array shape, docsgates-bridge liveness wiring
  (TODO_LIST rows 66-71 all untouched this window — correct, out of scope).
- Root battery (ci-local) was not run by any window leg for the
  cmd/tq-scoped fixes — the module gate covered them; a root pass remains
  unclaimed (closeout §c2, honest).

## d) TOTALLY FUCKED UP

1. **Daemon race hit THIS done-prompt too**: my skeleton commit was swept
   footer-less into 1c460bc9 minutes after the files were written (the
   commit gate's UNINDEXED failure opened the window). Recovered via
   `git show --stat` proven-mine + amend with the task footer (HEAD
   7791b46e). This is the same Edit→commit→gap failure the 15-11 closeout
   §d1 documented — it recurs because the queue's own status-index gate
   forces an edit-index-commit-verify round trip on a tree the daemon
   sweeps sub-minute.
2. **check-footer-attribution.sh FAILs on my healed HEAD**: the checker
   expects a healed/amended daemon-sweep commit to carry a footer equal to
   its OWN sha; a commit deliberately amended onto the REAL task ID
   (mine, footer-last) fails the span check. Attribution is correct for
   the queue; the checker's heuristic has no "task-footer amend" shape.
   Reported, not fixed (scope rule) — §f3.
3. **heal-daemon-sweep.sh footers sweeps with the sweep's own sha, not a
   caller-supplied ID**: no `--footer <id>` flag exists, so the sanctioned
   heal tool cannot produce the queue-attributable footer; the manual
   amend fallback is the only route (compounding d1).
4. **Cross-ID work attribution drift in the window itself**: the
   help-text pin (dispatch task 000001a1f751764) landed inside b2ecf030
   whose footer belongs to task 000001a0f4b467 (an older daemon fold) —
   the pin's provenance is only reconstructible via this report's commit
   content grep. The agent-pool reviewer fix (000001a1fbb309) likewise
   landed under the closeout's own 000001a200f919594e ID.

## e) WHAT WE SHOULD IMPROVE

1. **Commit-task script with retry-first shape**: stage + footer-commit
   THE MOMENT files exist (gates after, amend while unpushed) — both this
   window's races and the 15-11 closeout's two losses share this single
   cause. Rowed (§f2).
2. **Window-dedup grep as a mechanical step**: this dispatch mandated the
   README-index same-window grep and it worked (full-pass verdict proven
   in minutes); session-start.sh should run it so it is not prompt-
   dependent. Rowed (§f3).
3. **Review-fix task wrappers**: the two reviewer-rejection tasks' fixes
   landed under OTHER task IDs' footers; the queue's git cross-reference
   for 000001a1fbb309 finds nothing. The re-dispatch wrapper should either
   re-footer the fix commit or the closeout should cite the wrapper ID.
   Owner-shaped (§g1).

## f) NEXT THINGS (appended to TODO_LIST.md where new; ≤10)

Already-rowed follow-ups NOT re-minted: help-text worker/api pins,
worker both-directions audit, doctor WriteString (15-11 closeout rows);
375px mobile follow-up, webui design-pass batch rows (0769b178/290e8139).

New rows appended this pass (3):
1. Worker flag→Config parse-wiring pin (value-level, TestServeFlagParseWiresConfig
   analog over newWorkerFlagSet).
2. Promote the staged-set-assertion commit retry loop into
   scripts/commit-task.sh / scripts/lib (the recurring daemon-race cure).
3. Encode the turn-1 same-window dispatch dedup grep in session-start.sh.

## g) QUESTIONS FOR THE OWNER (≤3, appended as BLOCKED items where new)

1. **Attribution contract for reviewer-fix wrappers**: when a
   reviewer-rejection task's fix lands under the original closeout's task
   ID, should the wrapper ID get a marker commit, or is report-cited
   provenance acceptable? (affects tq's git cross-reference and the
   re-dispatch dedup grep) — appended as BLOCKED item.
2. Should check-footer-attribution.sh learn the "amended onto the real
   task footer" shape (footer ≠ commit sha but footer ∈ {dispatch task
   IDs}) instead of FAILing it? — folded into the same BLOCKED item.
3. (carried, unchanged) index-bloat digest ritual is due: 410 live index
   rows vs the 400 threshold, WARN firing on every commit — the ratified
   monthly ANNOTATE+ARCHIVE sweep has an existing row (65-adjacent); no
   new mint needed.

## h) BAND DRIFT

Journal read: `tq facts` over the cwd fallback `./tasks.db` (397 facts,
2026-09-07 → 2026-11-22) contains **zero `task.reprioritized` facts** —
none recorded, for the window span or ever in this journal. Caveat as
stated at the top: this is not the dispatching pool's journal (TQ_DB
stripped in agent shells), so band drift on the POOL's journal cannot be
read from here; the fallback journal is the only one reachable and it
holds no reprioritizations. Sixth-plus window running with a "none
recorded" drift section — consistent with ADR-0015's stored-priority
never-mutates design; aging does the scheduling, not mutation.
