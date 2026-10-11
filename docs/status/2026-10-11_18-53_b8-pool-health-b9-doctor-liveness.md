# B8 pool-health shipped + B9.1 consumer-liveness closed; B9.3 seam mapped (2026-10-11 18:53)

Owner directive: report FULL COMPREHENSIVE status after every task and WAIT.
This window executed the SUPERB pareto plan's B-track observability items from
the 18-29 WAIT state (resume signal: "READ, UNDERSTAND, RESEARCH, REFLECT …
Execute and Verify them one step at a time", then interrupted mid-B9.3 with
"DO!" → this report).

## a) What was accomplished (evidence attached)

### B8 `tq pool-health` — DONE, end to end (rows 55 + 148 ticked)

- `cmd/tq/poolhealth.go` NEW: `poolHealthFrom` pure fold + `cmdPoolHealth`
  (`--since DUR` default 168h, `--json`; advisory exit 0 like audit). Per
  repo: enqueued/completed/failed/dead counts, requeues by class, **skip
  streak** = trailing environment-class requeues (rate-limit/gate/budget/
  preflight) since the repo's last terminal fact — Completed/Failed/
  DeadLettered/Cancelled reset (a turn that RAN), Claimed deliberately does
  NOT (the pathological loop IS claim→requeue); WARN at streak ≥3 mirroring
  the env-streak breaker threshold. **Starvation**: enqueued>0 with zero
  outcomes in the window → WARN. Harvest recency via `todo:`/`batch:` dedup
  rows from the TASKS table (S1 divergence D2: fresh enqueue facts carry
  thin `{project,type}` only — dedup keys are NOT fact-readable, so meta
  comes from `store.List`, not fact details).
- `cmd/tq/journalaudit.go`: `drainAllFacts` extracted (shared bulk read;
  `journalDrift` refactored onto it — no behavior change).
- `cmd/tq/main.go`: `"pool-health": cmdPoolHealth` registered + usage line.
- `cmd/tq/poolhealth_test.go` NEW (5 tests over a real scratch sqlite
  journal, the journalaudit_test pattern): streak WARNs (3× claim→requeue
  rate-limit → streak 3, WARN, `requeuesByClass{"rate-limit":3}`); terminal
  resets (requeue×2 then Complete → streak 0, OK, 1/1 counts); harvest
  detection (todo:+batch: mark farm, dedup-less direct stays nil); starved
  WARN; JSON shape pins (factsFolded/verdict/skipStreak/requeuesByClass/repo).
- Gates: `bash scripts/test-cmd-tq.sh` RC=0 (twice: 8.350s pre-B9.1, 11.930s
  after); root `GOEXPERIMENT=jsonv2 go build ./... && go vet ./...` green;
  `check-doc-refs.sh` ok; `check-todo-list.sh` ok; gofmt clean.
- LIVE SMOKE (scratch DB, `TQ_DB` overridden — production untouched): table
  render prints `WARN smokerepo … starved: 1 enqueued…`; `--json`
  round-trips through python (repo/verdict/warnings verified).
- Docs: TODO rows 55/148 ticked with citations; FEATURES.md row
  (`tq pool-health` pool liveness, FULLY_FUNCTIONAL); CHANGELOG [Unreleased]
  Added entry.
- Attribution: commit 9957aedd carries docs + footer; the CODE files lost
  the daemon race (55517cc6: journalaudit/main/poolhealth + 2 foreign test
  tweaks; fbd2c1ad: poolhealth.go final + test) — see §d1.

### B9.1 doctor consumer-liveness — DONE (row 399 ticked)

- `doctorWatermarkLiveness` now names ALL five cursor-bearing consumers:
  review-sweeper, status-sweeper (existing) + dlqfix-sweeper,
  prioritize-sweeper, incident-policy (added). Missing cursor = OK "never
  ran here"; cursor < HeadSeq = WARN with lag count. The papdashboard
  bridge keeps NO journal cursor (documented in the comment — no row
  possible); the readmodel fold's cursor stays doctorProjection's own check.
- `TestDoctorWatermarkLiveness` extended: the no-cursor loop pins all five
  names; `dlqfix.ConsumerKey` lag case pins WARN propagation.
- Gates: `test-cmd-tq.sh` RC=0; gofmt clean.
- Commit 0e400985 (won the race — all three files in one footer commit).

### B9.3 projection-worker-death — seam mapped, ZERO code (interrupted)

- The surface exists upstream: go-cqrs-lite projectionhost/v4 staleness.go —
  `Host.CheckStaleness` returns `ErrWorkerFailed` (Infrastructure class:
  "worker(s) exhausted their restart budget and stopped consuming") via
  `failedWorkers()` (WorkerFailed status per worker). The critical
  subtlety: a failed worker can read **lag==0** — "its read model can no
  longer be assumed fresh even when lag reads zero" — exactly the case
  doctorProjection's cursor-vs-head WARN cannot see (the 3-day silent
  outage class).
- Doctor is one-shot; the Host lives inside composition's
  ProjectionRuntime while serve runs. Open design (one alternative):
  RECOMMENDED — composition wires the host's WorkerFailed transition (the
  metrics OnWorkerFailed hook exists) to a durable marker (WARN log + a
  sidecar marker the one-shot doctor reads), because the failure state
  must outlive the process; ALTERNATIVE — doctor-only inference from
  cursor stall + heartbeat absence, dismissed: cannot distinguish
  failed-at-head from idle-at-head (lag==0 is the whole problem).
  Execution resumes on the resume signal.

## b) What's in progress

- B9.4 (dedup-key-already-ticked doctor check): not started;
  `harvest.RedispatchAudit` already exposes live-task-on-closed-row
  findings — the doctor row wants the cheap queue-side variant.
- A2 release prep: [Unreleased] CHANGELOG grew by the pool-health entry
  this window; the version cut still awaits §g1.

## c) Blockers

None code-side. Carried owner rulings (§g) gate: v0.3.4 release timing,
vendor-sync gate direction, production budget values.

## d) What I totally fucked up

1. **Lost the daemon race on B8's code files — twice in one window.** I ran
   the full module gate, then wrote docs (TODO/FEATURES/CHANGELOG) BEFORE
   committing; the daemon swept cmd/tq code into 55517cc6 + fbd2c1ad
   (footer-less; 15 lines of a concurrent agent's dlqfix/status test tweaks
   rode along). My 9957aedd carried only docs. The edit→commit→battery rule
   (commit immediately after focused-green) was violated by batching docs
   into the same intended commit. LESSON encoded: commit at focused-green
   ALWAYS; docs tick = follow-up commit.
2. **heal-daemon-sweep.sh refuses my session-scope footer** (needs 16+ hex
   task IDs): I relied on the heal tool without checking its ID contract
   first (verify-before-execute applied to the plan, not my own tooling).
   The two sweeps stay footer-less — the known attribution-loss class C12
   exists to fix systemically.
3. **First test draft shipped a wrong assertion**: the JSON-shape test
   expected verdict OK on a starved repo (enqueued>0, no outcomes → WARN by
   the fold's own rule). Caught by the module gate (one red cycle), fixed by
   tracing the fold before re-asserting. Same draft carried two unused
   variables (vet). Cost: one gate rerun.

## e) What could be improved

Top of §f is B-track completion; the systemic version of §d1 is the C12
attribution arc (footer-last-trailer + heal identity-SHA).

## f) The next 50 things (ranked)

1. B9.3 finish: composition WorkerFailed → durable marker + doctor read (design §a)
2. B9.4 dedup-already-ticked doctor check
3. B10.1 bridge worker-exhausted → alert.triggered (papdashboard)
4. B10.2 budget-parked-all-day + sweepers-frozen alert classes
5. B10.3 alert e2e smoke (scratch journal)
6. B11.1 dismiss pre-breaker loop suspects (477/476/169 class) with reasons
7. B11.2 autopsy-yield audit query (fixes vs wontfix vs futile-env)
8. C12.2 GitLogScanner %B fallback + quoted-footer guarantee
9. C12.3 e2e trailer-visibility pin (harness-shaped fixture)
10. C12.1 footer-as-FINAL-line ruling apply (row 278)
11. C12.4 heal-daemon-sweep identity-SHA root-cause (rows 69/455)
12. C12.5 historical derivation-blind census (rows 354/355)
13. C13.1 re-dispatch repro fixture (stale TODO snapshot mint)
14. C13.2 fresh COMPLETED short-circuit at mint + footer grep probe
15. C13.3 regression pin: re-dispatch after tick refused/no-op
16. C14.1 verify runner capture failing-stage output
17. C14.2 FailureEvidence tail enlargement (FAIL + cause lines)
18. C14.3 `tq dlq --evidence-stage` + `tq show` render
19. C14.4 audit --journal verify-deaths-by-stage drill-down
20. C15.1 CV repo: data/ dir in verify path + dead-portal fixture
21. C15.2 CV canary task → gate green at base
22. C15.3 SystemNix park policy (uses C16.1)
23. C16.1 `tq park TASK_ID --until DUR`
24. C16.2 bare-worker agent-claim refusal + test
25. C16.3 forensics filters: audit --task-id, tasks --dedup
26. C16.4 ClaimCount GROUP BY pushdown
27. Row-45 pin: api-pump poison-DLQ WIRE e2e test
28. D17.1 TQ_SQLITE_SYNC self-open verify (row 37 — likely stale, verify)
29. D17.2 stats/doctor single-opener seam (row 34)
30. D17.3 pragma-union equality pin (row 41)
31. D17.4 single_opener.md → LANDED (row 40)
32. D18.1 S1 divergence register doc
33. D18.2 heal stale enqueue-snapshot docs (row 338)
34. D19.1 sweep release-conditional rows (row 300)
35. D19.2 verify suspect-stale rows (durable cursor 517, lint 89)
36. D19.3 check-todo-list /tmp-cite aging flag (row 596)
37. D20.1 --store postgres CLI wiring
38. D20.2 postgres drift-audit executed + runner evidence
39. D21.2 session polish: --dry-run/--json/--all + stale-session doctor
40. D21.3 CloseoutPending lifetime audit (row 139)
41. E22 webui value bundle (budget meter, parked badge, LogPath pin)
42. E23 webui health/security (SSE header e2e, smoke asserts, CSP)
43. F24 docs-health mechanization (archive-eligibility gate, digest rows)
44. F25 gate hardening bundle (dead-sha self-tests, ci parity)
45. F26 test-pin bundle (secretPatterns, auth-plane, lockout clock)
46. A2.1 CHANGELOG v0.3.4 cut (after §g1)
47. A2.2 VERSION-SURFACES walk (root + facades + cmd/tq floor)
48. A2.5 release-gates run + rc receipts (tag wave owner-side)
49. Docs pass: mark major-prompting-and-batching EXECUTED with landed numbers
50. F27 owner rulings session #2 (~40 BLOCKED rows, one sheet)

## g) 3 questions I cannot figure out myself

1. **Release timing (carried §g1)**: cut v0.3.4 NOW (ship the merged months —
   auto-upgrade, env-streak breaker, ceremony cuts, budget cap, pool-health,
   doctor liveness) or push B/C-track first? Everything code-side for the cut
   is one CHANGELOG version line + VERSION-SURFACES walk away.
2. **Vendor-sync gate direction (carried §g2)**: check-gomod-vendor-sync is
   red-on-clean at HEAD (cmd/tq tidy dies: internal/incident vs root@v0.3.3).
   (a) probe cmd/tq via the devmod shim instead of tidy, (b) skip cmd/tq tidy
   with a documented exception until the tag, (c) leave red until v0.3.4
   contains internal/incident?
3. **Production budget values (carried §g3)**: `--daily-cost-cap` USD value
   for the pool unit, and does the 30/day count cap stay?

## Verification receipts

- `bash scripts/test-cmd-tq.sh` → ok github.com/larsartmann/go-taskqueue/cmd/tq
  (8.350s then 11.930s, RC=0 both)
- root: `GOEXPERIMENT=jsonv2 go build ./... && go vet ./...` → RC=0
- `bash scripts/check-doc-refs.sh` → ok; `bash scripts/check-todo-list.sh` → ok
- Scratch-DB smoke: pool-health table + JSON verified (§a)
- Commits: 9957aedd (docs, footer 20261011-b8-pool-health), 0e400985
  (doctor+test+TODO, footer 20261011-b9-doctor-liveness-pack); daemon:
  55517cc6, fbd2c1ad (footer-less, §d1/§d2)

**STATUS: WAITING FOR OWNER INSTRUCTIONS.**
