# B10 Alert Routing WIRED — Pool Detectors Live, e2e Smoke GREEN; Attribution Eaten by the Daemon a 4th Time

Report for the 2026-10-12 ~03:15–07:10 window. Session resumed from the
23-58 WAIT on the owner's "execute and verify" directive at the B10 pool
half (detectors written-unwired). Concurrent sessions stayed extremely
active throughout (seal-chase window, task windows, AGENTS budget reset
19,600→20,000, v0.3.4 seal rounds) — every read was re-verified before
acting.

## a) FULLY DONE (this window)

1. **B10 pool wiring — `cmd/tq/main.go` + `cmd/tq/agentpool.go`**:
   - `workerDeathDetector{dbPath: deployment.DBPath}` + notify closure
     (WARN log; `alertBridge.NotifyWorkerExhausted` when armed;
     delivery-armed retry semantics). Hardened `observe()` with an
     empty-dbPath guard — a postgres deployment carries no sqlite
     readmodel sidecar and an empty path would probe the CWD.
   - `frozenSweepersDetector` constructed from ARMED consumer nilness:
     `incident.ConsumerKey` always (ADR-0021 unconditional sweep) +
     review/status/dlqfix/prioritize only when their sweeper is non-nil;
     streak knob REUSES `--dead-pool-ticks` (both alarms are "N
     consecutive bad liveness ticks"; 0 disables both — documented in
     the wiring comment).
   - Both `observe()` calls added inside the harvest-tick mintPass pass
     beside `deadPool.observe(res)` — placement verified against the
     budget-park false-positive: a parked pool mints nothing, so its
     watermarks may rightly stand still, and the observes skip with the
     harvest pass in lockstep.
   - Pre-wiring RESEARCH that de-risked the design: all five sweepers
     run `watermark.Cursor.Sweep` EVERY tick (status `Every` is a
     completion-count window, not a time gate), so a standing cursor
     with an advancing head is a true freeze, not an interval artifact.
2. **Five detector tests** (`cmd/tq/agentpool_test.go`):
   `TestWorkerDeathDetectorLifecycle` (raise-once, quiet repeats,
   resolve on cleared marker, empty-path never notifies),
   `TestWorkerDeathDetectorRetriesFailedDelivery`,
   `TestFrozenSweepersDetectorLifecycle` (streak→fire once naming only
   the frozen consumer, never-ran consumer skipped, catch-up resolves),
   `TestFrozenSweepersDetectorIdleAtHeadNoFire`,
   `TestFrozenSweepersDetectorRetriesFailedDelivery`. Focused run
   green; FULL `scripts/test-cmd-tq.sh` green (5.8s).
3. **B10.3 e2e smoke extension — GREEN end-to-end**
   (`scripts/smoke/papdashboard-e2e.sh`): new agent-pool phase stages
   the B9.3 marker (`<db>.readmodel.db.worker-failed`), asserts the
   worker-exhausted `alert.triggered` against the stub, removes the
   marker (fresh host), asserts `alert.resolved`; real-mode greps ride
   the pool log's `ingest accepted` lines; `POOL_PID` joined the
   cleanup trap; `wait_for` gained an optional log-file arg. Final
   bounded verification run: exit 0, all phases pass (evidence:
   /tmp/b103-smoke-verify.log).
4. **B10.3 docs**: AGENTS.md PapDashboard bullet extended byte-lean
   (+68 B → 19,970/20,000 — cap was raised to 20k by the concurrent
   seal window; the 19,600 figure in the prior session summary was
   stale); FEATURES.md gained the "Pool liveness pack (B10)" row;
   CHANGELOG [Unreleased] gained the pool-liveness-alerts entry.

## b) PARTIALLY DONE

- **B10 as a whole**: code + tests + smoke + docs rows ALL landed, but
  split across commits with broken attribution (see d); TODO_LIST rows
  for the pack NOT added (harvest-owned surface; next pool relaunch
  will mint from TODO_LIST — deliberate skip, noted for the docs pass).
- **Status index hygiene**: this report + index row land with this
  commit; the standing warnings (2 TRAILER f26 class, INDEX BLOAT
  ~300>100) remain.

## c) NOT STARTED (plan rows, untouched this window)

B11.1–B11.2 (DLQ hygiene: pre-breaker suspect dismissal 477/476/169
class; autopsy-yield audit query), C12.2 (GitLogScanner %B fallback),
C12.3 (e2e trailer pin), C13.1–C13.3, C15.1, C16.1–C16.4, row-48
(api-pump poison-DLQ WIRE e2e — NOTE: the 01-41 task window already
shipped a mutation-verified wire pin for row 50; row 48's exact scope
needs a re-read against that landing before execution), docs pass
(major-prompting-and-batching EXECUTED with landed numbers).

## d) TOTALLY FUCKED UP (process failures, all recoverable)

1. **Daemon ate the wiring attribution a 4th time**: while the first
   focused test cycle ran (>60 s), the auto-commit daemon folded
   `agentpool.go` + `main.go` into footer-less `88e93daf`; my footered
   commit `97a8f4e0` therefore carries ONLY the test file. The smoke
   script edits later met the same fate (footer-less `8005938e` et
   al.). `heal-daemon-sweep.sh` line 249 refuses non-hex footers, so
   session-scope IDs like `20261011-b10-alert-routing-pack` cannot be
   healed — this is now a 3-attribution-loss streak for session packs
   and C12.4 (heal-script identity fix) is the root cure.
2. **First smoke run was NOT hermetic**: the agent-pool phase ran with
   the default `--projects-dir` and harvested 194 REAL repos (3,482
   items) into the scratch journal. No production writes occurred
   (TQ_DB pointed at scratch; harvest only READS TODO_LISTs) and the
   stray-kill was narrowly patterned so the production nix-store pool
   (pid 1875977) survived untouched — but the run was noisy, slow, and
   dead-lettered 12 agent-type tasks against a worker with no agent
   executor. Fix: `--projects-dir "$TMP/pool-projects"`. LESSON: any
   smoke that spawns `tq agent-pool` MUST pin an empty projects dir.
3. **Three wasted gate cycles on signature sloppiness**: I wrote the
   frozen-sweeper tests calling `d.observe()` without `(ctx, s)` (6
   call sites) and read `HeadSeq`'s 2-value signature in queue.go and
   STILL destructured 3 values — tests written against remembered
   shapes instead of the just-read ones.
4. **Greps written without checking serialization order**: the ingest
   envelope is `map[string]any` → alphabetical marshal → `aggregateId`
   precedes `type`, so my type-first grep could never match. Verified
   the detector fired via manual repro BEFORE blaming the wiring (the
   WARN line appeared on the first tick) — the debugging order was
   right, the assert order was wrong.
5. **One backgrounded verification became uninspectable** (grep pipe
   buffering + a stuck pipeline): killed at the owner's report
   directive; the smoke's EXIT trap had already partially cleaned its
   evidence dir. Re-ran bounded with the log OUTSIDE the smoke's TMP —
   green in one pass. RULE: never pipe a long smoke through
   grep|head; tee to a persistent file.

## e) WHAT WE SHOULD IMPROVE

1. **C12.4 first** (heal script accepts session-scope footers): it has
   now blocked attribution recovery in three consecutive windows
   (b9-pack code, this window's wiring, this window's smoke script).
2. **Smoke-hermeticism checklist**: agent-pool invocations in smokes
   pin `--projects-dir` to an empty scratch dir; long smokes tee to a
   file outside their TMP; waits use order-insensitive greps (two
   patterns, not one ordered regex).
3. **Test-writing discipline**: copy signatures from the file just
   read, not memory — the two shape errors cost 3 of 4 gate cycles.
4. **Commit granularity under the daemon**: stage+commit the MOMENT
   `gofmt`+build pass for .go changes, then run the focused test as a
   follow-up commit if it finds anything — the current
   "focused-green-then-commit" rule loses races whenever the first
   test cycle exceeds ~60 s. (Owner may prefer keeping the rule and
   eating the loss until C12.4; flagged, not changed.)

## f) NEXT (ranked, ≤50)

1. C12.4 heal-daemon-sweep.sh session-scope footer fix (+ heal this
   window's three footer-less sweeps once it lands).
2. B11.1 pre-breaker suspect dismissal (477/476/169 class).
3. B11.2 autopsy-yield audit query.
4. C12.2 GitLogScanner %B fallback.
5. C12.3 e2e trailer pin.
6. C13.1.
7. C13.2.
8. C13.3.
9. C15.1.
10. C16.1.
11. C16.2.
12. C16.3.
13. C16.4.
14. Row-48 api-pump poison-DLQ WIRE e2e — re-scope against the 01-41
    row-50 landing first (may be DONE or narrowed to the remaining
    seam).
15. Docs pass: mark major-prompting-and-batching EXECUTED with landed
    numbers (52% ceremony figure now stale after B/C-track cuts).
16. TODO_LIST rows for any remaining B/C-track items the owner wants
    harvest-visible (currently only in the plan doc).
17. ci-local 3 FAILs carried from the v0.3.4 window (seal-chase §b
    says seal round 17 + BuildFlow full + push remain owner-side).
18. Smoke hardening: order-insensitive waits + tee-outside-TMP for the
    papdashboard e2e (fold into the next touch of that script).
19. AGENTS.md known-issue line for "smokes spawning agent-pool must
    pin --projects-dir" (byte-capped; fold into the next prune).
20. Frozen-sweeper knob: split from --dead-pool-ticks IF an operator
    ever needs different thresholds (YAGNI today — documented coupling).
21. Real-mode (`PAP_URL=…`) run of the extended smoke against the live
    dashboard (stub-mode green; real-mode greps written but unexercised).

(Rows 22–50 of the standing backlog are unchanged from the 23-58
report §f and the seal-chase §f 25; not restated here — both reports
remain live in the index.)

## g) QUESTIONS FOR THE OWNER (cannot be answered from the repo)

1. **Next-release timing** (carried, third ask): v0.3.4 is tagged and
   its GitHub Release published, but ci-local carried 3 FAILs and
   sub-tag cleanroom installs were pending at the 21-16 window's close
   — should the next version cut AFTER the B/C-track lands, or sooner
   to unblock external consumers of the new alert surface?
2. **Production budget values** (carried): what USD value should
   `--daily-cost-cap` carry in the production pool unit, and does the
   30/day task-count cap stay as-is? (The ≥8h all-day escalation from
   B10.1 now makes a mis-set cap loud, but the values themselves are
   owner-side.)
3. **Daemon-attribution policy** (new, sharpened by loss #4): keep the
   "commit at focused-green" rule and accept footer-less daemon sweeps
   until C12.4 lands, or switch to "commit at gofmt+build-green, amend
   or follow-up if tests fail"? The second loses fewer attributions
   but bends the edit→commit→battery ordering convention — your call.

## Verification (this window)

- `bash scripts/test-cmd-tq.sh -run 'TestWorkerDeathDetector|TestFrozenSweepersDetector' -count=1 -v` → 5/5 PASS.
- `bash scripts/test-cmd-tq.sh` (full module) → ok 5.805s.
- `bash scripts/smoke/papdashboard-e2e.sh` (bounded, log at
  /tmp/b103-smoke-verify.log) → exit 0; worker phases + pool
  worker-death raise + resolve all asserted.
- AGENTS.md 19,970 B ≤ 20,000 guard (`TestAgentsDocSizeGuard`).
- Commits: `97a8f4e0` (tests, footer `Task-Queue-ID:
  20261011-b10-alert-routing-pack`); code + smoke landed via daemon
  sweeps `88e93daf`, `8005938e` (footer-less, see §d1); docs + this
  report in the closing commit (same footer).

WAITING FOR INSTRUCTIONS.
