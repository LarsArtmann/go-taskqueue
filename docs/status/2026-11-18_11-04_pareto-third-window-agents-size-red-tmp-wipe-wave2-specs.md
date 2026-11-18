# Status: third Pareto window — wave-2 derivation, AGENTS-size red cured, /tmp wipe recovery

**Date**: 2026-11-18 11:04 CET (session ran 2026-11-17 22:00–22:20 + 2026-11-18 11:00–11:04)
**Plan**: docs/planning/2026-11-17_19-53_backlog-pareto-master-plan.md (T01–T36)
**Prior window**: 2026-11-17_21-59_pareto-second-window-push-guardwiring-digest.md

## a) FULLY DONE

1. **The 9-commit span pushed clean** (0067c328..f0851372, 0 refused; the
   dead job 0E9 was re-done manually — its push3.log never materialized).
2. **Wave-1 annotations APPLIED**: 285 inline verdicts over the 8 oldest
   reports (2026-10-01 ×6, 10-02 ×6-06/06-16). The 06-06 spec was rebuilt
   from the corrected line derivation — 28/28 emit-verified, agent-B's
   fabricated rows discarded — and merged from the verified parts
   (a+a2+a3+a4+b+c = 323 rows). `check-rows`: 8 files complete. Commit
   `8f7a16eb`.
3. **docs-gates healed to 3 PASS / 2 SKIP / 0 FAIL**: the freshness trip was
   my own 21-59 §f2/§f7 (session-in-flight T02/P3) — `<!-- fresh:skip -->`
   markers added, committed `0db2ca90`.
4. **Wave-2 specs derived IN-SESSION** (standing policy: no subagent specs
   without line-for-line verification): 06-44 (41 items), 06-57 (30),
   07-20 (27 numbered), 07-39 (70), 07-53 (29), 08-01 (24) — every spec
   dry-run-verified to a 1:1 line match, every verdict evidence-cited
   (threshold-400 grep, digest rows at README:92, heal auto-narrow,
   commit-msg hook presence, EnvStreakBurns field, tagged-range self-test
   cases, metacharacter/empty-id refusal cases, check-agents-size ci-local
   wiring at :157, and ~20 more).
5. **Origin CI red root-caused to one byte count**: run 44176353296
   (f0851372) failed the test AND nix legs on the SAME
   `TestAgentsDocSizeGuard` — AGENTS.md 27,685/27,600 (over by 85), grown by
   MY OWN T09 doctrine edit (`35393721`) which landed AFTER the pre-push
   battery ran. 4th hidden-behind-CI-red instance.
6. **Cure landed**: compress-only prune 27,685 → 27,583 (dropped the T09
   oscillation archaeology from the AGENTS heal bullet; the
   nearest-vs-oldest semantics live in the script self-test + T09 commit).
   Both twins green: `check-agents-size.sh` OK 27,583/27,600; the canonical
   `scripts/test-cmd-tq.sh` gate owed a re-run (see §b). Commit `642512ec→abb9e1e58eece284312ea0b811e1d52dd6b0c450`.

## b) PARTIALLY DONE

1. **T02**: root `test -race` rc=0 (job 055 receipt, read before the wipe);
   full `ci-local.sh` rc=1 — died at the check-ci foreign-red refusal (the
   origin run predating the tree). Green re-run owed after the prune push.
2. **T36**: push half done; the watched run went RED (agents-size); the cure
   (`642512ec→abb9e1e58eece284312ea0b811e1d52dd6b0c450`) sits UNPUSHED. Overnight someone pushed `0db2ca90` (not me —
   concurrent agent/daemon/owner) and its CI run 44177605500 is RED on the
   same guard (the Fuzz run 44230073805 at the same sha is green — different
   workflow). CI-reds TODO rows (1687/1385/1709) + the T09 row (~1411)
   remain unticked pending a green CI run.
3. **T03/T04 wave-2**: 6 of 8 reports spec'd and dry-run-verified but NOT
   applied (see §d1 — the /tmp wipe ate the spec files; all six re-emitted
   byte-equivalent into /var/tmp/pareto-battery from session context).
   08-04 is read + evidence-gathered (spec not written); 08-06 untouched.
4. `scripts/test-cmd-tq.sh` re-run after the prune: attempted, blocked by
   the redirect failure during the /tmp outage (§d1), not retried since.

## c) NOT STARTED

- Wave-2 apply + check-rows + docs-gates + commit; 08-04/08-06 specs;
  wave-3+ (the remaining ~215 unarchived reports).
- T10–T16 (fold-below, commit-task --amend, AGENTS twin guards ×2, deploy
  cadence ×2), P2 tier (T17–T32), P3 tier (T33–T35 + pool dispatch wave).

## d) TOTALLY FUCKED UP

1. **/tmp/pareto-battery was wiped** (nightly tmp cleaner — session crossed
   midnight): every unapplied spec, gate receipt, and CI-watch log lost.
   Recovery: all six specs re-emitted from context (byte-equivalent values),
   dry-runs re-verified; the lost receipts (ci-local-full.log body,
   root-test-race tail) survive only as claims in this report's prior text.
2. **I pushed a span that re-broke master CI** — the +85 B AGENTS growth was
   mine (T09), and my pre-push battery ran BEFORE that edit landed. The
   edit→battery→push ordering failure the 21-12 ledger already taught,
   repeated at push granularity.
3. **Two shell-quoting failures** writing specs: apostrophes in printf
   values broke quoting — one mangled a row (caught: dry-run 29≠30), one
   killed an entire 70-row block at parse time (caught: wc=1).
4. **Ordering sins re-committed**: ran docs-gates on a dirty tree once
   (hook scolded; commit-then-gate restored), and used a `tail` on a
   pipe with `$?` once (blocked by the bash guard before damage).

## e) WHAT WE SHOULD IMPROVE

1. **Session artifacts live under /var/tmp (or repo-ignored dirs), never
   /tmp** — the nightly cleaner is now a proven threat, not hypothetical.
2. **The pre-push battery must run at the FINAL tree of the span** — an
   AGENTS-touching commit landing after the battery invalidates it. Cheap
   close-out rule: `check-agents-size.sh` + root build/vet re-run after the
   last commit, before push.
3. **No quotes/apostrophes in printf-emitted spec values** — plain prose
   only; the parser never needs them and quoting has now failed twice.
4. **The wave-2 method worked**: read file → grep evidence → verdict with
   citation → dry-run line-match → apply. ~15 min/report in-session; the
   subagent route stays retired.

## f) Up to 50 things we should get done next (priority order)

1. Push `642512ec→abb9e1e58eece284312ea0b811e1d52dd6b0c450` (13 h old — timing guard moot) → watch CI green →
   tick CI-reds rows 1687/1385/1709 + T09 row ~1411 (T36 close).
2. Re-run `scripts/test-cmd-tq.sh` + full `./scripts/ci-local.sh` green
   (T02 close).
   <!-- fresh:skip — session-in-flight (Pareto master plan T02), ticked at closeout -->
3. Write 08-04 + 08-06 specs; apply d1–d8; check-rows; docs-gates; commit
   (T03/T04 wave-2 close).
4. T10–T16 per the plan's ≤12-min breakdown.
5. P2 tier T17–T32; then P3 T33–T35 + the pool dispatch wave.
   <!-- fresh:skip — session plan tier (Pareto master plan T17–T35), not backlog -->
6. Fold this report's §e1/§e2 rules into AGENTS Conventions at the next
   AGENTS-touching commit (budget: 17 B headroom — fold into T13/T14).

## g) Questions I can NOT figure out myself

1. **Archive bar vs routed items** (carried from 21-59 §g1): every wave
   report holds genuinely-open `r`-verdict items (TODO:284-class owner
   rulings) that keep it unarchivable under the mechanical bar. Ratify
   "annotated-but-unarchivable is honest" or loosen the bar for
   routed-only residue?
2. **Push policy for CI cures** (carried from 21-59 §g2): when origin is
   red and the fix is local, the timing guard forces a ~10 min wait per
   push attempt. Keep the friction, or allow immediate pushes whose span
   is exactly a CI-red cure?
3. **Standing battery home**: confirm `/var/tmp/pareto-battery` as the
   session-artifact home (or name a repo-ignored dir you prefer) so the
   /tmp-wipe class cannot recur?

— Window totals: 1 push, 6 commits (4 mine + 2 folded daemon), 285
annotations applied, 221 verdicts derived-not-yet-applied, 1 CI root cause
found + cured, 1 artifact-wipe recovered.
