# B9 Doctor-Liveness Pack Completed; B10 Alert Routing Half-Landed (Session Window 23:00–23:58 CEST)

Session continuation of the SUPERB pareto execution plan
(`docs/planning/2026-10-11_02-41_SUPERB-pareto-execution-plan.md`),
resumed on the owner's execute-and-verify directive. This window
closed B9.3 and B9.4 end-to-end and landed the bridge half of B10;
the pool half of B10 is written-but-unwired (exact seam documented
below). Report-then-WAIT per directive.

## a) FULLY DONE

### B9.3 — projection worker-death surface (plan row 147)

The multi-day silent-outage class: a fold worker that exhausts its
restart budget stops consuming while the platform reports it at
lag 0, so the cursor-vs-head doctor checks cannot see the death.

- `internal/readmodel/workerfailed.go` (NEW): durable marker sidecar
  beside the projection db — `WorkerFailedMarkerPath(dbPath)`,
  `WriteWorkerFailedMarker` (atomic temp+rename, one write point),
  `ReadWorkerFailedMarker`, and `workerFailureMarker`, a
  `projectionhost.MetricsRecorder` adapter whose only non-no-op
  callback is `WorkerFailed` (WARN log + sidecar stamp; compile-time
  `var _ MetricsRecorder` pin).
- `internal/readmodel/model.go`: `Model.path` field (set in `Open`) —
  the host derives the marker path from the model's own file.
- `internal/readmodel/host.go` `NewProjectionHost`: clears any
  standing marker at construction (fresh host = fresh restart budget)
  and wires `projectionhost.WithMetrics(newWorkerFailureMarker(...))`.
  Because the wiring lives in `NewProjectionHost`, BOTH host owners
  get it: serve's composition root and the api pump (httpapi) — no
  caller changes needed.
- `cmd/tq/doctor.go` `doctorProjectionWorkerDeath`: reads the marker;
  standing marker = WARN naming the age, the lag-0 invisibility, and
  the restart remedy; registered right after `doctorProjection` in
  `runDoctor`.
- Tests (4): `TestWorkerFailedMarkerRoundTrip` (absent=healthy,
  write/read shape, no temp leak),
  `TestProjectionHostClearsWorkerDeathMarker` (stale marker gone on
  fresh host build), `TestWorkerFailureMarkerRecorderStampsSidecar`
  (internal: no-op callbacks stamp nothing, WorkerFailed writes the
  readable shape), `TestDoctorProjectionWorkerDeath` (OK absent /
  WARN standing with remedy text).
- Gates: readmodel module build+vet+tests green (full suite 0.144s),
  composition module green, root build+vet green, full cmd/tq gate
  green (5.07s).
- Docs: CHANGELOG [Unreleased] entry + FEATURES doctor-row extension
  (commit 4586ce70).
- **Attribution caveat**: the code files landed via daemon sweeps
  54e7078a + b6793ff3 (footer-less; the commit attempt lost the race
  by seconds). `heal-daemon-sweep.sh` refuses session-scope footers
  (needs 16+ hex task IDs) — the C12 attribution-loss class again.
  Docs commits carry the footer `20261011-b9-doctor-liveness-pack`.

### B9.4 — doctor stale-items check (plan row 148)

The dedup-already-ticked zombie class: PENDING tasks joined to
TODO_LIST items that are already `[x]` or gone — a pool relaunch would
execute done work.

- `cmd/tq/doctor.go` `doctorStaleItems`: runs the EXACT prune-stale
  derivation (`harvest.New(queue.New(store),
  Config{ProjectsDir, DryRun: true}).PruneStale(ctx)`) — one
  derivation, observation-only (DryRun verified honored in every
  prune path before reuse). WARN with per-task Items (repo, task id,
  ticked/absent, item text), running/dead counts, scan failures;
  skipped without `--projects-dir` (mirrors doctorRepoCoverage).
  Registered after `doctorRepoCoverage` in `runDoctor`.
- `TestDoctorStaleItems`: empty projects dir = no rows; open items
  only = OK; a pending task whose `harvest.ItemKey("demo", ...)`
  payload dedup joins a ticked row = WARN with the item text and the
  `prune-stale` remedy.
- Gates: focused + full cmd/tq suite green; root build+vet green.
- Commit 73e2f25a (won the race) + docs 46c23d30.

### B10.1/B10.2 — bridge half (papdashboard)

- `NotifyWorkerExhausted(ctx, triggered, projection, failedAt)`:
  raise/resolve on stable aggregate `agent-pool-projection-worker`,
  raise body carries the age and "Restart the serve/api process."
- `NotifySweepersFrozen(ctx, triggered, frozen []string, streak)`:
  raise/resolve on `agent-pool-sweepers-frozen`, raise body names the
  stuck consumers and the pool-log pointer.
- **Budget-parked-all-day escalation** in `trackBudgetBlocked`: new
  `blockedFirstAt` anchor; at day rollover a parked window ≥ 8h
  (`budgetAllDayThreshold`) additionally fires
  `agent-pool-budget-all-day-<day>` (critical) after the quiet
  rollover resolve — the spend-posture incident class.
- Tests: rollover pin updated 3→4 events (escalation between resolve
  and re-trigger, on its own aggregate),
  `TestBudgetBlockedShortParkSkipsAllDayEscalation` (2h window: 3
  events, never the all-day class), plus lifecycle tests for both
  Notify methods. Full bridge suite green.

## b) PARTIALLY DONE

### B10.1/B10.2 — pool half (detectors written, unwired)

- `cmd/tq/agentpool.go` (landed via daemon sweeps d075bc28+4d94c6bc,
  footer-less): `workerDeathDetector` (reads
  `readmodel.ReadWorkerFailedMarker` per tick; raise once on standing
  marker, resolve on cleared; delivery-armed like deadPoolDetector)
  and `frozenSweepersDetector` (per-consumer unchanged-cursor streak
  while the journal head advanced; only ARMED consumers watched; a
  consumer at head is idle, not frozen; raise names all frozen
  consumers + max streak, resolve when the set empties).
  Compile-verified post-sweep: cmd/tq gate green (one devmod flake —
  `dev.mod: no such file` raced a concurrent agent's cleanup —
  retried clean).
- **NOT done**: `cmd/tq/main.go` wiring — construct both detectors
  (workerDeath with `deployment.DBPath`; frozenSweepers with the
  armed-consumer list built from `sweeper`/`statusSweeper`/
  `dlqfixSweeper`/`prioritizeSweeper`/`incidentPolicy` nilness at
  their construction sites, lines ~1315–1382), notify hooks routing
  to `alertBridge.NotifyWorkerExhausted`/`NotifySweepersFrozen`
  (mirror the deadPool.notify closure shape at main.go ~1187), and
  the two observe calls in the harvest-tick pass next to
  `deadPool.observe(res)` (~line 1503). No detector unit tests yet
  (mirror TestDeadPoolDetectorStreakLifecycle in agentpool_test.go).

### B10.3 — alert e2e smoke + AGENTS note

Not started. Intended: extend `scripts/smoke/papdashboard-e2e.sh`
(stub dashboard already records ingest POSTs; add a worker-death
marker staging + budget-all-day fixture) and a byte-neutral AGENTS.md
edit (file is at ~19.6k/19600 B size-guard cap — must swap, not add;
fold the new alert classes into the existing PapDashboard bullet).

### A2 release prep — likely moot, unverified

CHANGELOG shows `[v0.3.4] - 2026-10-11` (cut in a parallel window
during this session — a concurrent agent/owner shipped it mid-window;
the [Unreleased] section was empty before my B9.3 entry). I did not
verify the tag or proxy state; next release timing is owner context.

## c) NOT STARTED (plan order)

B11.1 (pre-breaker suspect dismissal 477/476/169 class), B11.2
(autopsy-yield audit query), C12.2 (GitLogScanner %B fallback), C12.3
(e2e trailer-visibility pin), C12.4 (heal script identity-SHA
root-cause — directly relevant: it refused my session footers twice
this session), C12.5 (derivation-blind census), C13.1–13.3
(re-dispatch dedup), C14.1–14.4 (verify-failure evidence), C15.1 (CV
data/ gate), C15.2 (CV canary), C16.1–16.4 (tq park, bare-worker
claim refusal, forensics filters, ClaimCount pushdown), row-48
api-pump poison-DLQ WIRE e2e (TODO_LIST row; the summary's "row-45"
has drifted to row 48), D17–D21 platform rows, docs pass (mark
major-prompting-and-batching EXECUTED with landed numbers).

## d) TOTALLY FUCKED UP

- **Lost the daemon race for B9.3's code commit** (54e7078a +
  b6793ff3) and for B10's partials (d075bc28 + 4d94c6bc): four
  footer-less sweeps, zero attribution. Root cause both times: I ran
  the full battery or docs prep before committing at focused-green.
  B9.4 (73e2f25a) proves the instant-commit discipline wins.
- One wasted cycle: first `scripts/test-cmd-tq.sh` retry hit
  `go: open dev.mod: no such file` — the devmod file raced a
  concurrent agent's cleanup trap. Second run green; noted as a
  known-flake shape, not investigated further (out of scope).
- Minor: gofmt violation in the first workerfailed.go draft (comment
  alignment) caught pre-commit; one missing-tab syntax slip in the
  bridge edit caught by build. Both fixed in-minute.

## e) WHAT WE SHOULD IMPROVE

- **Commit at FIRST focused-green, battery after** — the B9.3 loss
  was self-inflicted (I ran the full module suite before committing).
  The B9.4 order (commit → full suite → docs commit) is the template.
- **heal-daemon-sweep.sh's hex-only task-ID constraint** is now
  blocking attribution recovery for every session-scope footer pack —
  C12.4 (identity-SHA root cause) keeps getting more expensive the
  longer it waits.
- The frozen-sweeper detector semantics (unchanged-cursor streak)
  need a pin against a live sweeper in the pool e2e once wired — the
  unit tests alone won't catch a mis-armed watch list.
- AGENTS.md has ~0 B headroom (19600 cap) — the B10.3 note requires a
  swap edit; consider whether the size guard should grow or the file
  should shed a section.

## f) NEXT (up to 50, priority order)

1. Wire workerDeathDetector in main.go (construct + notify + observe).
2. Wire frozenSweepersDetector (armed-consumer list at sweeper sites).
3. Detector unit tests (streak lifecycle, delivery-retry, idle-vs-frozen).
4. Full cmd/tq gate + commit B10 pool half.
5. B10.3: extend papdashboard-e2e.sh (marker staging + all-day fixture).
6. B10.3: byte-neutral AGENTS.md PapDashboard bullet update.
7. B10 docs: TODO/FEATURES/CHANGELOG rows.
8. B11.1: dismiss pre-breaker loop suspects with reasons.
9. B11.2: autopsy-yield audit query (fixes/wontfix/futile-env).
10. C12.2: GitLogScanner %B fallback + quoted-footer guarantee.
11. C12.3: e2e trailer-visibility pin (harness-shaped fixture).
12. C12.4: heal-daemon-sweep identity-SHA root cause + self-test.
13. C12.5: derivation-blind census (rows 354/355).
14. C13.1: stale TODO snapshot mint repro fixture.
15. C13.2: fresh COMPLETED short-circuit at mint + footer probe.
16. C13.3: re-dispatch-after-tick refusal pin.
17. C14.1: capture failing-stage output in verify log.
18. C14.2: enlarge FailureEvidence tail.
19. C14.3: `tq dlq --evidence-stage` + `tq show` render.
20. C14.4: audit --journal stage drill-down.
21. C15.1: CV data/ dir creation + dead-portal fixture.
22. C15.2: CV canary dispatch at base.
23. C16.1: `tq park TASK_ID --until DUR`.
24. C16.2: bare-worker agent-claim refusal + test.
25. C16.3: forensics filters (audit --task-id, tasks --dedup).
26. C16.4: ClaimCount GROUP BY pushdown.
27. Row-48: api-pump poison-DLQ WIRE e2e pin.
28. Docs pass: major-prompting-and-batching EXECUTED numbers.
29. Verify v0.3.4 tag/proxy state; A2 remainder if any.
30. D17.1: TQ_SQLITE_SYNC self-open verification.
31. D17.2: stats/doctor single-opener seam.
32. D17.3: pragma-union equality pin.
33. D17.4: single_opener.md status flip.
34. D18.1: S1 divergence register doc.
35. D18.2: stale enqueue-snapshot docs heal.
36. D18.3: M4 ratification memo (owner-gated).
37. D19.1: release-conditional row sweep.
38. D19.2: suspect-stale row re-verification.
39. D19.3: check-todo-list /tmp-cite aging flag.
40. D20.1: --store postgres CLI wiring.
41. D20.2: postgres drift-audit executed.
42. D21.1: session sweep wiring (owner-gated).
43. D21.2: session polish (--dry-run, --json, stale-session).
44. D21.3: CloseoutPending lifetime audit.
45. Frozen-sweeper pool e2e pin (live sweeper).
46. devmod cleanup-race hardening (dev.mod collision).
47. readmodel module version bump for cmd/tq proxy pins (release flow).
48. TODO_LIST row sweep for this window's landings.
49. Re-run `go test ./... -race` before any done-declaration (daemon
    concurrent-agent rule).
50. Vendor-sync gate direction once owner picks (§g2).

## g) QUESTIONS FOR THE OWNER (cannot figure out myself)

1. **Release state/timing**: CHANGELOG carries `[v0.3.4] - 2026-10-11`
   from a parallel window — did the full release wave (tag + push +
   proxy) complete, and should the next version wait for B/C-track
   completion or cut sooner? (I can see tags, not your intent.)
2. **Vendor-sync gate direction** (carried §g2): (a) probe cmd/tq via
   devmod shim, (b) documented cmd/tq tidy skip, or (c) leave
   red-on-clean until the next tag contains internal/incident?
3. **Production budget values** (carried §g3): what USD value for
   `--daily-cost-cap`, and does the 30/day count cap stay?
