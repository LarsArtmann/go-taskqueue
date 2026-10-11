# B-Track Resume: A7 Close-Out, B9 Stale-Claim Verify, Row-19 Pin (2026-10-11 18:29)

Interactive session (owner GO: continue the SUPERB pareto plan from the
11-20 WAIT state). Scope: finish A7's two red tests, verify the B9.2
stale claim before executing it, then enter B8. No production surface
touched; all work verified by gates before reporting.

## a) FULLY DONE

- **A7 ceremony cuts — CLOSED.** The superseded
  `TestSweeperGateArtifactWithoutShippedProofKeepsAutopsy` pin flipped
  to the refusal contract
  (`TestSweeperGateArtifactWithoutShippedProofNeverAutopsies`,
  internal/dlqfix/sweep_test.go:719): gate-artifact death without
  shipped proof now asserts `EnvSkipped == 1`, zero autopsies minted,
  task stays `task.Dead` — the scanner-run auto-dismiss remains the
  rescue path. The review-test seeding-order fix (sweeper created
  BEFORE `runAgentTask`; NewSweeper bootstraps its watermark at journal
  head) verified green. Commit 15de13d5 (footer
  `20261011-a7-ceremony-cuts`).
- **Full gate battery for A7 — ALL GREEN**: root build+vet+full test
  suite (RC=0); all 22 sub-modules build+vet+test `-count=1` green
  (executor, worker, journal, queue, companion, sqlitev4, postgresv4,
  readmodel, composition, config + facades); `scripts/test-cmd-tq.sh`
  RC=0; `check-agents-size.sh` OK (19587/19600 B); `check-doc-refs.sh`
  ok; `check-status-index.sh` ok; `gofmt -l` clean.
- **B9.2 (TODO row 44, doctor DLQ path split-brain) — VERIFIED
  STALE/DONE-ON-ARRIVAL.** No re-work needed: the single derivation
  already landed (`readmodel.DLQPathFor(readmodel.PathFor(d.DBPath))`
  inside `OpenDeadLetters`, internal/readmodel/dlq.go:54; both serve
  (internal/composition/projection_runtime.go:65) and doctor
  (cmd/tq/doctor.go:865) open through the SAME seam; row 44 was already
  `[x]` with a 2026-10-11 DONE note; TestDoctorProjectionDLQ green in
  the cmd/tq gate).
- **Row 19 seam pin — DONE.** The one open residue the stale-claim
  check surfaced: `TestOpenDeadLettersInputContract`
  (internal/readmodel/dlq_test.go) pins the sidecar seam itself — DSN
  build extracted to `dlqDSN(d)` and asserted literally
  (projection-beside path, deployment DLQ pragma tier, SyncFull flip),
  plus file-existence assertions that the pre-fix queue-beside sidecar
  stays absent (split-brain re-plant guard). Readmodel module
  GOWORK=off `-count=1` green; TODO row 19 ticked with citation.
  Commit fcd7282a (footer `20261011-b9-doctor-liveness-pack`); daemon
  landed the two readmodel files in its own sweep before my commit.

## b) PARTIALLY DONE

- **B8 `tq pool-health` — RESEARCH COMPLETE, CODE NOT STARTED.** All
  seams mapped: fact source via `store.Facts(ctx, after, limit)`
  pagination (journalaudit pattern); `queue.EnqueueDetail` carries
  Project/Type/DedupKey (task→repo map from Enqueued facts);
  `queue.RequeueEvidence.Class` names the skip family (rate-limit,
  gate, budget, preflight); harvest-minted enqueues identifiable by
  dedup prefix `todo:` / `harvest.BatchKeyPrefix` (`batch:`); store
  open via `mustOpenStore(mustDeploymentFromDB(*db))` (cmdAudit
  pattern). Designed output: per-repo window counts (enqueued /
  completed / failed / dead / requeues-by-class), trailing skip streak
  (consecutive requeue-class facts since the repo's last terminal
  Completed/Failed/Dead — a turn that RAN resets), last harvest
  enqueue, last any-fact activity; WARN at streak ≥ 3 (env-streak
  threshold semantics) and at starved (enqueued>0, no terminal in
  window). Zero lines written.
- **Vendor hygiene — HALF.** My internal/readmodel edit made the root
  `vendor/` tree stale (vendor copies replaced-module SOURCE;
  gitignored so invisible to git status). Healed via `go mod vendor`;
  re-run confirms "ok: root vendor/ in sync (content hash 425117d2…)".
  The sync GATE still exits 1 — see §d (pre-existing, not mine).

## c) NOT STARTED

B8 code+tests+doc row; B9.1/B9.3/B9.4/B9.5 (doctor liveness pack:
consumer-cursor staleness, projection-worker-death WARN,
dedup-ticked check); B10 alert routing; B11 DLQ hygiene; C12-C16 waste
streams (attribution %B fallback, re-dispatch dedup, evidence quality,
CV gate, park verb, forensics filters); D17-D21 platform residue;
E22-E23 webui; F24-F27 process tail; A2 release prep (CHANGELOG cut,
VERSION-SURFACES walk, release-gates); A3 deploy flip (owner-gated);
docs pass (mark major-prompting-and-batching EXECUTED with the landed
before/after numbers; harvest new §f items into TODO_LIST).

## d) TOTALLY FUCKED UP / FOUND BROKEN

- **FOUND (pre-existing at HEAD, verified present at session start):
  `scripts/check-gomod-vendor-sync.sh` exits 1 every run.** Its
  per-module tidy loop explicitly includes `cmd/tq`, and `(cd cmd/tq &&
  GOWORK=off go mod tidy)` dies: cmd/tq imports
  `internal/incident` (ADR-0021, landed by a concurrent agent) but
  pins the root module `github.com/larsartmann/go-taskqueue v0.3.3`,
  whose tagged tree predates that package — "module @latest found
  (v0.3.3), but does not contain package …/internal/incident". With
  `set -euo pipefail` the script aborts there (all module lines print
  ok, then silence, exit 1). cmd/tq BUILDS fine via the devmod shim
  (`test-cmd-tq.sh` RC=0), so only the tidy probe is broken — the gate
  has been red-on-clean since internal/incident landed. Needs an owner
  ruling on direction (see §g) because ADR-0017 owns cmd/tq's
  replace-free design.
- **Self-inflicted, healed same session**: (1) skipped `go mod vendor`
  after editing a sub-module file (§b) — the AGENTS rule exists, I
  missed it once; (2) one edit round-trip wasted guessing the
  seed-helper signature instead of re-reading; (3) wrote a nonexistent
  `hasSuffix` helper in the new test (caught at write time, fixed
  before running).
- **Daemon interference twice**: review/sweep_test.go already committed
  under me; readmodel dlq.go/dlq_test.go taken by a daemon sweep before
  my commit could include them (commit carried only the TODO tick).
  Harmless here (footer-less heal not needed — files landed clean),
  but the edit→commit→battery ordering discipline slipped once.

## e) IMPROVE (what I forgot / would do differently)

1. Mechanize "sub-module source edit → root `go mod vendor`" (the
   drift class I hit is exactly what check-gomod-vendor-sync exists
   for; a pre-commit or session-habit gap let it through once).
2. Read exact call signatures before editing foreign test files — the
   failed first edit was pure guess-then-retry.
3. Commit IMMEDIATELY after a green focused test, before running the
   broad battery (daemon window), per the documented convention.
4. When a plan row claims "already fixed" (B9.2), the verify-grep is
   cheap and should be the FIRST act, before any design work — it was
   here, keep it.
5. The vendor-sync gate red-on-clean means every future session will
   re-trip on it; fix or document the escape (`VENDOR_SYNC_OFF=1`)
   prominently once ruled.
6. Advisory-lint noise seen while working: `mintAutopsy` cyclop=13,
   review/sweep_test ineffassign ×4, dlqfix/sweep_test golines — all in
   the advisory baseline, none gate-blocking; fold into F26/lint-baseline
   work rather than ad-hoc fixes.

## f) NEXT UP TO 50 (ranked, Pareto order)

1. B8.1 pool-health: per-repo skip-streak fold over facts (design
   above; ~12 min)
2. B8.2 pool-health: last-harvest-activity per repo + table render +
   WARN verdicts (~10 min)
3. B8.3 `cmdPoolHealth` wiring: flags `--since` (default 168h) /
   `--json` / `--db`; register in command table; help text
4. B8.4 pool-health test: scratch store, seeded requeue-streak vs
   completed repo, JSON fields pinned (cmd/tq test pattern from
   journalaudit_test)
5. B8.5 tick TODO rows 55/148 + FEATURES row + CHANGELOG entry
6. B9.1 doctor: consumer-cursor staleness check (names consumers)
7. B9.3 doctor: projection-worker-death surface (restart budget
   exhausted → WARN)
8. B9.4 doctor: dedup-key-already-ticked check
9. C12.2 GitLogScanner `%B` fallback + quoted-footer guarantee
10. C12.3 E2E trailer-visibility pin (harness-shaped commit fixture)
11. C12.1 dispatch/closeout template: footer-as-FINAL-line apply
12. C12.4 heal-daemon-sweep identity-SHA root cause + self-test
    red→green
13. C13.1 re-dispatch dedup repro (stale TODO snapshot mint fixture)
14. C13.2 fix: fresh COMPLETED short-circuit at mint + footer grep
    probe
15. C13.3 regression pin: re-dispatch after tick = refused/no-op
16. C16.1 `tq park TASK_ID --until DUR`
17. C16.2 bare-worker agent-claim refusal + test
18. C16.3 forensics filters: `audit --task-id`, `tasks --dedup`
19. C16.4 ClaimCount GROUP BY pushdown
20. C15.1 CV repo unblock: data/ dir creation in verify path +
    dead-portal test fixture
21. B10.1 bridge: worker-exhausted → alert.triggered
22. B10.2 bridge: budget-parked-all-day + sweepers-frozen classes
23. B10.3 alert e2e smoke (scratch journal) + AGENTS note
24. B11.1 dismiss pre-breaker loop suspects with reasons
25. B11.2 autopsy-yield audit query (fixes vs wontfix vs futile-env)
26. C14.1 verify runner: capture failing-stage output into failure log
27. C14.2 enlarge FailureEvidence tail (FAIL + cause lines)
28. C14.3 `tq dlq --evidence-stage` + `tq show` render
29. C14.4 `audit --journal`: verify-deaths-by-stage drill-down
30. A2.1 CHANGELOG [Unreleased] → v0.3.4 cut + entries audit
31. A2.2 version bump walk per VERSION-SURFACES (root + facades +
    cmd/tq floor)
32. A2.5 release-gates run + rc receipts to file (tag wave itself
    owner-gated)
33. Docs pass: mark major-prompting-and-batching EXECUTED with landed
    numbers; harvest §f items into TODO_LIST
34. Row 45 pin: api-pump poison-DLQ WIRE e2e (fold non-retryable
    poison through api-armed host; sidecar row lands; stats served)
35. D17.1 verify TQ_SQLITE_SYNC resolves in self-open; fix or document
36. D17.2 stats/doctor single-opener seam or documented exception
37. D17.3 pragma-union equality pin
38. D17.4 single_opener.md status → LANDED
39. D18.1 S1 divergence register doc + ADR links
40. D18.2 heal stale enqueue-snapshot docs + conform cross-links
41. D19.1 stale-row sweep: release-conditional rows re-verified vs
    tags/proxy
42. D19.2 verify suspect-stale rows (durable cursor 517, lint 89-class)
43. F24.3 index digest row / archive sweep — README at 290 live rows
    (gate warned this session; threshold 100)
44. F25.2 check-dependabot-coverage.sh + new-module emit
45. F25.5 check-release-docs: nullglob + code-mutation self-test
46. F26.1 secretPatterns overlap-table pin + property test
47. F26.2 lockout fake-clock suite
48. F26.3 SECURITY header-matrix pin + header→want table
49. E22.1 serve --daily-budget flag + meter aria
50. F27.1 rulings sheet from ~40 BLOCKED rows (groups the tail)

## g) QUESTIONS (max 3)

1. **Release timing (blocks A2/A3):** cut v0.3.4 NOW — I do the
   CHANGELOG cut, VERSION-SURFACES walk and release-gates, you do the
   tag wave + `systemctl restart tq-serve` + SystemNix flip — or push
   B/C-track further first so v0.3.4 ships pool-health and the
   attribution fixes in one wave?
2. **Vendor-sync gate direction (§d):** the gate's `cmd/tq go mod
   tidy` probe cannot pass while cmd/tq imports `internal/incident`
   and pins root@v0.3.3 (ADR-0017 replace-free). (a) run that probe
   through the devmod shim, (b) skip cmd/tq tidy with a documented
   exception comment, or (c) leave red-on-clean until the v0.3.4 tag
   makes root@latest contain internal/incident?
3. **Production budget numbers (A3.3):** what USD value should
   `--daily-cost-cap` carry in the deployed pool conf, and does the
   count cap stay at 30/day alongside it?
