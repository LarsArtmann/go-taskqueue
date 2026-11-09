# Library Fleet Follow-Through — Battery Cures & Self-Review (2026-11-09_21-42)

Follow-through window for the 08-54 execution session's Exact Next Steps
(M15 receipt, M19 digest tail, report amendment, M20 final, hygiene).
This report covers ONLY this window's run. Parent: the amended
`2026-11-09_08-54_libs-fleet-plan-execution-status.md` (all 20 M-tasks
now closed there, §a16–§a20).

## a) FULLY DONE

1. **M19 digest tail** (commit 64241d98 + daemon-folded README edit
   1f7ce896): digest 2026-11 accrual row (+5 archived 2026-10-09
   passes 12–16) + `archived/README.md` manifest section (5 lines, one
   per file — restores the bulk-sweep convention the 10-28/T13 sweeps
   had lapsed). check-status-index, check-ghost-archives, check-docs-gates
   all rc=0 verified 2026-11-09 by direct runs (receipts in the gates'
   own summaries: "status index ok", "asset archives ok", "3 PASS /
   2 SKIP / 0 FAIL").
2. **M15 battery receipt** (attempt 9, log archived at
   `docs/status/assets/2026-11-09_m15-battery/`): full
   `CI_CHECK=off ./scripts/ci-local.sh` → **rc=1, 99 ok-countable lines
   (`grep -c '^ok:'` over the captured log), sole test red =
   `TestDoctorHealthyEmptyDB` (tag-ancestry v0.4.2 forked lineage,
   retried 3× by the wrapper)** — identical shape to the 08-10 batch
   receipt. Everything through module isolation gates green, including
   the windows loop and tests -race. CI_CHECK=off is the gate's own
   printed escape for the red master run c84d2b5a (predates this tree).
3. **Four battery-forced cures, each the gate doing its job**:
   check-doc-refs allow entry for the CHANGELOG version-wave module-path
   cites (3fb7387c, ls-remote-verified tags); COVERAGE.md rows for the
   concurrent sessions' unindexed `_lib.sh`/`session-start.sh` smokes
   (722712d6, 34/0); dated grandfather block for 47 footer-less
   fleet-era daemon sweeps (89ed017c, 1649 baselined / 0 new); and the
   worker FACADE go.mod tidy + root `go mod vendor` (63372890,
   check-gomod-vendor-sync rc=0 after) — the facade had been tidy-stale
   since the 08-54 session's M6 go-codec promotion updated only the
   internal module.
4. **AGENTS budget rescue**: concurrent sessions grew AGENTS.md
   27,028→27,557 B past the 27,200 budget (a hard ci-local step).
   Compress-only prune (constant resets are owner-gated by the standing
   row) to **27,193/27,200, check-agents-size rc=0** (e80b6e7e + a6a139c8).
5. **08-54 report amended + indexed**: §a16–§a20 added (M13/M14/M15/
   M19/M20 receipts), §b/§c emptied, §d5–§d8 appended, follow-through
   note on §f, WAIT line refreshed, README index-row scope updated to
   "ALL 20 M done" (c72ca4f3).
6. **M20 final + hygiene**: check-todo-list rc=0, check-docs-gates
   3 PASS / 2 SKIP / 0 FAIL, status-index ok, ghost-archives ok,
   `gofmt -l cmd internal` empty, tree quiet.

## b) PARTIALLY DONE

None — every step of the handoff's Exact Next Steps closed this window.

## c) NOT STARTED

1. **Post-abort ci-local tail is structurally dark at HEAD**: because
   the deterministic `TestDoctorHealthyEmptyDB` red kills the run at the
   cmd/tq gate, every later ci-local step (embed tidy, go-mods hygiene,
   go-floor, toolchain pins, dep-bump drift, vendor-hash drift, facade
   parity, dead-exports, script-syntax, guard-wiring, ghost/tick/feature
   gates) NEVER runs inside ci-local until the v0.4.2 lineage is healed.
   This window compensated with standalone runs (vendor-sync, doc gates,
   module loops) — but that is manual coverage, not the battery.

## d) TOTALLY FUCKED UP (caught, fixed, recorded)

1. **Facade/module name-collision misdiagnosis (three burns)**: the
   failing step label "windows cross-compile (worker)" names the
   FACADE `worker/`, but I reproduced against `internal/worker` — which
   passes. Attempt 7's failure was then misread as "transient concurrent
   edit" (the wrapper's own wording encouraged it) and attempt 8 burned
   identically. Only the full per-module loop reproduction (rc printed
   per module: `internal/worker rc=0` … `worker rc=1`) exposed the
   collision. Lesson: when a repo has facade/internal twins with the
   same name, disambiguate the label BEFORE reproducing.
2. **Stepped on the documented PIPESTATUS class myself**: the
   repro `go build … | head -3; echo rc=$?` printed rc=0 for a FAILING
   build (head's rc) and briefly "confirmed" the transient theory.
   AGENTS documents this exact trap; the rc-to-file rerun + a visible
   tidy diff broke the illusion.
3. **Compress-prune broke a pinned grep**: my budget prune reworded the
   load-flaky bullet to "+ … are load-flaky"; `check-verify.sh:101`
   greps the literal `` `<Test> is load-flaky` `` from AGENTS.md and the
   verify-wrapper self-test failed ("KNOWN_FLAKY pin has nothing to bind
   to") — the fleet's pinned-summary convention catching its own
   session's edit. Restored verbatim; trimmed an unbound reader-route
   line instead. Before rewording AGENTS bullets, grep scripts/ for the
   literal phrase first.
4. **No pre-flight before launching a 40-minute battery**: attempts 4
   (agents-size) and 5 (doc-refs) died on cheap standalone-runnable
   gates I could have greened in ~2 minutes first. I only adopted
   pre-flighting after attempt 5; attempts 6–9 paid for that discipline.
5. **Receipt evidence nearly lost twice**: the 08-54 session's ci-local
   log was /tmp-pruned before pickup (bare rc survived), and my own
   attempt-9 receipt sat in /tmp for an hour before I archived it
   (`docs/status/assets/2026-11-09_m15-battery/`, SHA256SUMS manifest).
   The archive-at-filing lesson (2026-10-08 row) is old; adherence is
   still manual.

## e) WHAT WE SHOULD IMPROVE

1. **Pre-flight the cheap gates before any full ci-local launch** — the
   doc/gate section (agents-size, doc-refs, smoke-coverage,
   daemon-attribution, release-docs, status-index) runs in minutes
   standalone; burning 40-minute batteries on them is pure waste.
2. **Pinned-phrase inventory for AGENTS.md**: at least check-verify.sh
   greps AGENTS prose literally; an editor has no way to know a phrase
   is load-bearing. Either an in-file marker convention or a
   scripts/-side inventory list.
3. **Battery step labels should be unambiguous**: `worker` vs
   `internal/worker` cost three runs; label module-loop steps with the
   full relative path.
4. **Archive receipt logs at capture time**, not end-of-window — make
   it a reflex, or hook ci-local to tee into a repo-adjacent path.
5. **The battery needs a resume/continue-past mechanism** while a
   documented deterministic red owns one step (see §c1/§g1) — otherwise
   the entire post-cmd/tq tail silently rots from the battery's
   protection.

## f) Up to 50 things we should get done next

<!-- fresh:skip: this window's §f is process-hardening derived from §d/§e of this report; items overlapping standing rows cite them instead of re-minting -->

1. ci-local resume mechanism: `TQ_CI_LOCAL_SKIP=<step>` or a
   continue-past-documented-red list so the post-cmd-tq tail gates run
   at HEAD despite the lineage red (this window proved they are dark).
   <!-- fresh:skip: blocked on the §g1 lineage ruling's shape; companion to the standing TestDoctorHealthyEmptyDB owner row -->
2. Pre-flight helper: `scripts/ci-preflight.sh` running the cheap
   doc/gate section standalone, called at the top of ci-local (or
   documented in AGENTS Commands) — the attempts-4/5 lesson.
3. Module-loop label disambiguation in ci-local.sh (`worker` →
   `facade worker/` or full path) + `--self-test` pin for the label
   format (fleet countable-gate convention).
4. Pinned-AGENTS-phrase inventory: `grep -rn "AGENTS.md" scripts/*.sh`
   for literal-phrase greps; document each load-bearing phrase in a
   scripts/PINNED-PHRASES note or as inline comments in AGENTS.
5. AGENTS budget: execute the standing re-tighten row (compress to
   ~24,000 + re-tighten BOTH twins) — the margin is 7 B and the next
   concurrent addition re-trips it. <!-- fresh:skip: standing row (Consciously re-tighten the AGENTS.md budget constant) owns this -->
6. TestDoctorHealthyEmptyDB fixture-vs-exemption shape — standing
   owner-gated row; ci-local tail darkness (§c1) adds urgency.
   <!-- fresh:skip: standing UNBLOCKED-OWNER-GATED row owns this -->
7. v0.4.2 lineage heal or tag-ancestry baseline — standing §g1.
   <!-- fresh:skip: standing owner question, not agent-executable -->
8. Coordination note for the 22 unpushed grandfathered sweeps: owning
   sessions can still heal them (attribution restored) before the owner
   pushes; after push the grandfather is permanent. One-line note in
   the fleet channel or the next done-prompt.
9. Windows-loop after dep changes: the 08-54 session's miss was a
   linux-only loop; add "windows loop too" to the AGENTS dep-bump
   bullet (it already implies per-module tidy — make the windows loop
   explicit). Small AGENTS edit within budget.
10. COVERAGE.md row-at-landing habit: the concurrent sessions landed
    smokes without rows; consider wiring check-smoke-coverage into
    install-pre-commit (it currently rides ci-local only).
11. Grandfather-block compaction policy: the baseline is 1649 shas and
    grows monotonically; decide a retention shape (e.g., drop entries
    older than N months once pushed-and-landed). Owner-lean question,
    not urgent.
12. Archive-evidence at capture: hook `archive-evidence.sh --wait` into
    the ci-local tail (or the report skeleton checklist) so battery
    receipts never sit only in /tmp.

## g) Questions I cannot figure out myself

1. **v0.4.2 forked lineage** (re-surfaced with new evidence): the
   standing §g1 asked heal-vs-baseline; this window adds that ci-local
   can never run green — nor reach ANY gate after the cmd/tq step —
   until it is resolved. Re-tag/history surgery (barred without you),
   or baseline the tag-ancestry check? This now gates both v0.4.3 AND
   the battery's own tail coverage.
2. **Push timing vs the 22 healable sweeps**: ~70+ unpushed commits
   (the fleet wave + today's cures) sit local. 22 of the 47 grandfathered
   footer-less sweeps are still unpushed and COULD be healed properly by
   their owning sessions (restoring attribution) — but only before the
   push. Push now and bake the grandfather in, or hold briefly for a
   heal pass?
3. **AGENTS budget authority**: the margin is 7 B (27,193/27,200). The
   standing row makes constant resets owner-conscious; the next
   concurrent addition will trip the hard gate and force another
   compress-only prune (mine touched 19 sites to save 366 B). Reset to
   a roomier constant now (you name the number), or keep pruning per
   incident until the re-tighten row lands?

— Recorded 2026-11-09_21-42 CET. Session WAITS FOR INSTRUCTIONS.
