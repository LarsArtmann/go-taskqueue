# Status: tail-skip root cause + fix, M09 slice 1, master CI settling

2026-10-07 10:30, covering the window 2026-10-06 ~13:30 → now
(continuation of the v0.3.2-recovery session; owner directive:
READ/UNDERSTAND/RESEARCH/REFLECT, execute and verify step by step
until done).

## a) FULLY DONE

1. **The webui smoke red is ROOT-CAUSED and FIXED (M08 slice 2
   unblocked).** The projectionhost live tail passed `SeqToEventID(after
   - 1)`to`ReadFrom`, but`ReadFrom`is EXCLUSIVE of the ID it is
   given (`journal.AfterSeq`): every poll boundary permanently skipped
   the fact at anchor+1, and a skipped terminal fact (Completed,
   DeadLettered) wedged the fold's ledger — the smoke's persistent
   running=1. This was a GAP, not a lag; no amount of waiting would
   converge it. Fix:`tailAnchor(after)` anchors at the LAST delivered
     seq; the zero event ID reads from the journal start. Committed as
     the session's own fix, then rebased overnight to b6b1fd11 by a
     concurrent agent — content intact.
2. **Regression pin proven to catch the bug**: fast internal-package
   test (10ms poll, one fact per poll window, asserts exactly
   [3 4 5]). Sensitivity proven empirically in a /tmp copy: under the
   old cursor the test delivers `[4]` (seqs 3 and 5 permanently
   skipped) and times out. The existing `TestProjectionHostTailsLiveFacts`
   could never catch this class — its skipped fact was a benign
   Claimed.
3. **Webui smoke green twice** with the fixed binary (stats converge
   completed=2/dead=1/running=0), papdashboard smoke green standalone
   (rc=0) — the two ci-local reds during the day were load flakes
   (host load 13–24 from concurrent agents; both smokes pass in
   isolation with the same binary).
4. **Composition module go.sum fixed**: internal/readmodel's M08
   dependency on journal/cqrs left composition's go.sum stale —
   GOWORK=off builds of the S4 root failed. Tidy-only, verified by the
   per-module ladder (all 24 modules green).
5. **Windows CI red root-caused and fixed, plus a REAL goroutine
   leak.** `TestProjectionHostAdvancesPastPoison` failed on the
   windows runner: the tail subscriber spawned `context.Background()`
   ticker goroutines with NO stop path — one per worker AND one more
   per worker restart, outliving the test's store close ("database is
   closed" spam) and racing the poison worker's checkpoint save (the
   test asserted the watermark AFTER `Stop()`, so a slow worker's
   in-flight save lost the race). Fixes: `ProjectionHost` wrapper
   (embeds the platform host, adds `Close()` which stops every tail
   and waits via WaitGroup; the platform never closes subscribers, the
   Bus doc delegates cleanup to the owner); pointer-receiver subscriber
   shared across worker generations; the poison test now waits for the
   watermark WHILE the host is live and asserts after Stop as
   `Close`-guarded sanity; Close cleanups in all host tests and in
   serve's `runReadModel`. Readmodel `-race` green.
6. **M09 slice 1 landed (F041 + F042 + the serve half of the DLQ
   item)** — platform health visibility:
   - `internal/readmodel/dlq.go`: `DLQPathFor` (`<db>.readmodel.db`
     - `.dlq.db`) and the `DeadLetters` sidecar (`OpenDeadLetters`,
       `Store`, `Count`, `Recent`, `Close`; queue sqlite posture: WAL,
       busy_timeout, MaxOpenConns(1)).
   - serve wiring: `runReadModel` opens the sidecar and passes it as
     the host's `DeadLetterStore` (threshold 1); a sidecar failure is
     warn-and-continue — the fold runs without a DLQ rather than
     refusing to serve.
   - health rollup: the prober grew a `projection` check emitted only
     when a read model is mounted — pass within a 100-fact head
     tolerance, warn past it ("the fold is wedged or restarting");
     store-measured, consistent with the health surface's
     never-process-state stance.
   - `tq doctor` projection section: cursor lag vs journal head
     (serve-aware wording), projection-file presence including the
     delete-to-replay escape hatch, folded-count drift vs the queue's
     own counts. Built strictly on RELEASED readmodel v0.3.2 APIs —
     cmd/tq is replace-free and must build against proxy tags.
   - Tests: `TestDoctorProjectionSection` (fresh / replay-hatch / lag /
     drift), `TestHealthProjectionCheck` (absent / at-head / wedged).
   - Webui smoke re-run green with the DLQ + projection check live.
7. **Three foreign lint classes fixed at source** (overnight agents'
   commits had tripped the shared baseline gate): worker `recvcheck`
   (`Config.idleGap` to pointer receiver), harvest `errname`
   (`RedispatchRefusal` → `RedispatchRefusalError` across 8 sites),
   cmd/tq `unparam` (`commitsTask` dropped its constant `id` param —
   every call site passed the same literal).
8. **Master CI's two overnight reds fixed and pushed** (ca6fd6ef):
   vendorHash drift from the M09 go.sum changes (new `got:` hash
   pinned, `nix build .#checks.x86_64-linux.vendor-hash` green) and
   the AGENTS.md size budget (16.9k → over by 1.5k). The stale
   "cmd/tq pins nominal v0.3.0" note was corrected to "pins the
   release floor"; the budget is consciously reset to 18.5k in the Go
   guard and its twin shell script, per the guard's own escape hatch —
   the growth was legitimate (wake-seam ADR-0020 docs, the package
   table, the dep-bump gate line).
9. **Host ops**: /mnt/buildcache was 100% full (ENOSPC breaking gates
   mid-run); trimmed stale go-build entries (atime +2d) → 115G freed,
   45% used. A dev.mod race with a concurrent gate run and one
   mid-run cache exhaustion were re-run clean.

## b) PARTIALLY DONE

1. **CI on the fix push (ca6fd6ef) is still in flight at report time**
   (run 37593760430). The prior push's only red was the windows
   poison-test race, which is fixed and pinned; the watch job is
   running and must be checked on resume.
2. **ci-local full rc=0 not achieved at session end**: the last full
   run stopped at its check-ci step because master CI was red
   (predating my tree); those reds are now fixed and pushed, but the
   gate has not been re-run to completion on the settled tree.
3. **M09 slice 1 is a slice**: `tq doctor --dlq` (the doctor half of
   the DLQ item) is deferred — `OpenDeadLetters` is not in the
   RELEASED readmodel v0.3.2 that replace-free cmd/tq builds against.
   It lands with the next readmodel tag cut.
4. **lint-baseline growth from concurrent agents** (cmd/tq
   cyclop/goconst/staticcheck/varnamelen/wsl_v5, postgresv4
   paralleltest, sqlitev4 err113) kept moving between checks while
   other agents worked; my own classes are fixed, theirs were left to
   their owners. The baseline is red against the moving tree — needs
   one settled-tree pass.

## c) NOT STARTED

- Composition-root `NewProjectionHost` factory wiring (f-list item 4) —
  now has two potential consumers (webui serve, httpapi).
- M11 redispatch burn killers — NOTE: another agent is visibly
  mid-flight in exactly that area (redispatch.go mint-time gate,
  show_commits, dep-bump gate all appeared overnight); coordinate
  before starting.
- M12 daemon attribution + hooks tooling, M13 S2 vocabulary flip,
  M14–M25, M26 upstream filings, M27 docs-health tail burn.
- Dogfood cutover and v0.3.1 release-page warning — still awaiting
  owner answers (13-13 report §g.2/§g.3).

## d) TOTALLY FUCKED UP

1. **I temporarily put the BROKEN cursor back into the main tree**
   during the regression-pin sensitivity proof (the plan was a /tmp
   copy; the sed-restore round-tripped through the repo file). The
   window was seconds and the restore is verified, but a daemon sweep
   in that window would have committed a broken tree. The proof
   belongs entirely outside gated trees.
2. **The amend mix-up**: I amended the daemon's show_commits sweep
   with the worker/harvest lint message (wrong commit, wrong message),
   then re-amended correctly — but the worker/harvest fixes remain a
   footer-less `chore:` sweep (2fd0e17d) with no heal (no Task-Queue-ID).
3. **Two consecutive broken edits on host.go**: the multiedit's
   whitespace-equivalent match nested `tailAnchor` inside `tail()`,
   and the first repair consumed the method's closing brace. Cost: a
   syntax-error cycle that exact-byte reading would have avoided —
   I had the cat -A output and did not use it before editing.
4. **Self-inflicted load flakes**: I ran the module ladder, the smoke,
   and ci-local simultaneously on a 28-user host → SQLITE_BUSY +
   alert-timing reds that cost two ci-local runs and a standalone
   attribution cycle before I serialized.
5. **My doctor test was wrong, not the code**: the lag assertion ran
   with head==cursor (t1 never enqueued before the watermark save);
   burned a cmd/tq gate cycle on a test bug.
6. **Vendor-hash drift discovered by CI, not by me**: the AGENTS.md
   gotcha says run the vendor-hash check after go.mod changes; the
   M09 go.sum edits sat for hours before the nix job flagged them.
7. **The tail lifecycle design was mine to get right and I missed it**:
   slice 1 shipped a subscriber that could never stop. The platform's
   Bus doc delegates cleanup to the owner via io.Closer — I should
   have read that contract before spawning an infinite goroutine.

## e) WHAT WE SHOULD IMPROVE

1. Never round-trip experimental code through the repo tree — even
   for seconds; gated trees are daemon-food.
2. Add the vendor-hash check to the personal post-go.mod battery
   (edit go.mod → tidy → vendor → `nix build .#checks...vendor-hash`).
3. Serialize heavy gates on this host; parallel ci-local runs are
   strictly worse (self-inflicted SQLITE_BUSY class).
4. Before any amend, re-read what HEAD actually is (the daemon moves
   it between operations).
5. The projectionhost platform could own a poll-subscriber primitive
   with lifecycle (stop channel + WaitGroup) — every DB-tailed
   projection will re-commit this leak otherwise. Candidate for the
   M26 upstream filings.
6. lint-baseline on a shared moving tree needs a settle protocol:
   one owner regenerates after a quiet window, or growth classes are
   assigned per-agent in the sweep baseline.

## f) NEXT (prioritized)

1. Confirm CI green on ca6fd6ef (watch running); if test-windows
   reds, fix forward — the poison-race fix is already pinned, so a
   red would be a new class.
2. Re-run ci-local to full rc=0 on the settled tree.
3. Cut the next internal/readmodel tag (release flow, probe-before-
   pre-cut per RELEASE.md) so replace-free cmd/tq sees the M09 APIs.
4. Land `tq doctor --dlq` against that tag: DLQ count + recent entries
   via `DeadLetters.Recent` (the doctor half of the DLQ item).
5. Composition-root factory `NewProjectionHost` wiring (S4 completion)
   with the two real consumers.
6. Fold the M09 surfaces into AGENTS.md (platform section, size-
   budgeted at 18.5k) + FEATURES.md rows (managed-projection pump,
   health projection check) in ONE size-guarded edit.
7. TODO_LIST reconciliation: rows closed by the tail fix, the
   composition go.sum fix, and M09 slice 1 (docs-health VERIFY).
8. M11 redispatch burn killers (F046–F051) — AFTER coordinating with
   the agent already working redispatch.go.
9. M12 daemon attribution + hooks tooling (F052–F056).
10. M13 S2 one-vocabulary flip (F058–F064).
11. M14 gates: `nix flake check` into ci-local (F066), load-aware
    skips (F067 — today's flakes are the case for it), devmod
    per-invocation dev.mod suffix (F068 — kills the race I hit twice),
    per-user golangci cache (F069 — ends /mnt/buildcache contention).
12. M15 quiet-host ci-local capture + evidence archive.
13. M16 token/cost-denominated cap (O2) + UsageToday settlement.
14. M17 JIT frontier scoring (O3), retire DefaultScoreTTL.
15. M18 DefaultBatchItems=3 + prompt outcome-contract rewrites.
16. M19 claim-time executor-type filter, `tq park`, closeoutPending
    leak audit.
17. M20 security pin bundle.
18. M21 audit drill-downs.
19. M22 doctor consumer-liveness + derivation-blind commit census.
20. M23 ci.yml parity + required-checks P1 + toolchain pins.
21. M24 webui customer surfaces (budget meter reachability, dlqfix
    result surfaces, fragment collapse).
22. M25 SQLITE_BUSY class fix (repro harness, bounded open retry) —
    today's load flakes are fresh evidence.
23. M26 upstream filings — ADD: projectionhost poll-subscriber
    lifecycle primitive (this session's leak as the evidence);
    go-nix-helpers mkDefault footgun; art-dupl templ suppression;
    M4 ratification memo; cqrs-lint branch.
24. M27 docs-health mechanization + standing tail burn (297→0).
25. Pin govulncheck in the flake (agent-runnable).
26. VERSION-SURFACES.md re-pin + release.sh bare-tag-at-gate-time
    redesign.
27. Sweep remaining fold error paths for error-family classification
    (heartbeat/cancel detail parsers).
28. A smoke-level fold-lag print for diagnosability (the no-skip pin
    covers the class; the print shortens future attribution).
29. HealthCheckDetailed metadata (per-check Duration) into /health if
    the dashboard renders it — F042's remaining "Detailed" half.
30. Dogfood serve restart (owner-run, O12) once CI is green — the
    tail-skip fix makes the projectionhost flip production-safe by my
    read; the 805-task production DB still folds with the pre-durable-
    cursor pump (WAL churn).
31. v0.3.1 release-page warning (owner call).
32. Dependabot first-week check (O15, earliest 2026-10-13).
33. `--dep-sweep` retirement row.
34. 13-13 f-list residue: fold-marker.sh, heal task-less mode,
    footer-attach heal script, install-pre-commit hooksPath fix,
    inert-hooks detection.
35. Annotate this report's index row + harvest the window's claims
    into TODO_LIST per docs-health.

## g) OWNER QUESTIONS (3)

1. **Dogfood cutover**: with the tail-skip fix pinned and the DLQ
   wired, the projectionhost flip is production-safe by my read. The
   production serve (805 tasks) still folds with the pre-durable-
   cursor pump (WAL churn). Rebuild + restart the service NOW on a
   fresh master build, or hold for the next release cut? (Operator-run
   per O12 — your hands, my prep.)
2. **v0.3.1's public record** (carried from 13-13 §g.2, still open):
   publish a one-paragraph warning Release on the retracted v0.3.1
   tag ("broken module graph, use v0.3.2"), or leave it silent
   (retraction visible via `go list -m -retracted`)?
3. **lint-baseline under concurrent agents**: three classes from other
   agents' in-flight work were still growing when I stopped chasing
   them. Should the owning agents fix-at-source (current discipline,
   my choice) or should you want one settle-window regeneration of
   `.golangci-baseline.txt` once the tree quiets down?
