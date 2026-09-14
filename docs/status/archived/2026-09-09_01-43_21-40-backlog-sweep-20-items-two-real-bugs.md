# Status Report — 21:40 Dogfood Backlog Sweep: 20 Items Executed, 2 Real Bugs Found

Session: 2026-09-09 00:00–01:45 (interactive, concurrent with the webui
write-actions session and the auto-commit daemon). Scope: the entire
unchecked remainder of the "first full dogfood window (22 agent tasks)"
TODO section — every actionable item executed and pinned; the three
owner-blocked items left blocked (one now with audit evidence attached).

## a) FULLY DONE — code

1. **`tq harvest --prune-stale`** (21:40 §d1/§e1): `harvester.PruneStale`
   (`internal/harvest/prune.go`) + CLI flag. Cancels PENDING tasks whose
   TODO_LIST item is now `[x]` (dedup-key match), reason
   `harvest --prune-stale: TODO_LIST item is now [x]: <text>` on the
   `task.cancelled` fact. Running tasks reported-only (cooperative stop is
   an operator `tq cancel --force` decision), dead listed for the DLQ flow.
   Pinned by three tests (`CancelsPendingZombies`, `ReportsRunningAndDead`,
   `DryRunChangesNothing` — note: the per-repo pacing allows at most ONE
   pending task per repo, so the tests use two repos).
2. **Failure evidence on `task.failed` facts** (§d4): `Store.Fail` /
   `FailPermanent` gained an `evidence json.RawMessage` parameter (both
   stores; Postgres keeps its `{"class":...}` fallback via `failureDetail`
   when no evidence). Executors publish
   `FailureEvidence{stage, exit_code, tail}` through
   `Sink.SetFailureEvidence`: agent runs, agent VERIFY (whose output tail
   was previously discarded on the error path — recovered), and sh
   commands. The worker wires `sink.Failure()` into every terminal fail
   call. End-to-end pin: `TestFailureEvidenceRidesFailedFact` (exit 7 +
   stderr excerpt asserted on the fact).
3. **`tq tasks` list view** (§e7): `--project/--status/--type/--since
   DUR/--limit/--json`, newest-first, full IDs + attempts + last-error
   excerpt. Completion windows no longer need raw sqlite.
4. **`tq show`/`tq cancel` ID prefixes**: `resolveTask` — exact Get fast
   path, unique-prefix scan, ambiguity error that NAMES its candidates.
   Pinned by `TestResolveTaskPrefix` (ULIDs share time prefixes, so the
   test computes the shortest unique prefix dynamically).
5. **`Task-Queue-ID` commit footer** (§e3): the harvest, catch-up and
   status prompt contracts instruct agents to end commits with
   `Task-Queue-ID: {{TASK_ID}}`; the AGENT EXECUTOR resolves the
   placeholder at run time (`runAgent` now receives the task ID — it
   cannot exist at harvest render time). Review/status route through the
   same substitution. Pinned by `TestAgentPromptTaskIDSubstitution`.
   (Lesson applied twice: backticks inside templated Go raw strings
   terminate the literal — the footer lines are unquoted in the prompts.)
6. **`tq stats --json` + budget spend**: `--json` now emits the aggregate
   (`by_status`, `by_project`, `budget{spent_today,cap}`,
   `consumer_lag[]`) instead of the raw task array (that moved to
   `tq tasks --json`); text output always shows
   `budget today N/M enqueued` (`--daily-budget N` for the cap).
7. **Sidecar byte cap** (round-6 open half): `SweepSidecarsByBytes`
   (oldest-first `*.log` deletion until the total fits) behind
   `agent-pool --log-dir-max-bytes` / `$TQ_LOG_DIR_MAX_BYTES`, swept each
   tick after the age pass; README retention paragraph updated.
8. **Web UI cancel reason in the trail** (§e-item): `detailFacts` decodes
   the fact detail's `reason` and renders `— <reason>` on the timeline
   line (`TestDetailFactsSurfacesCancelReason`).
9. **Nightly fuzz rotation**: `scripts/fuzz/nightly.sh` is now a
   pkg|target campaign rotation (FuzzParseRepo + FuzzExtractResultPayload);
   `fuzz.yml` commits both corpora and dumps both crash dirs. Live-verified
   (5s campaign staged +22 executor seeds).
10. **`scripts/smoke/bootstrap-install.sh`**: fake `$HOME`, stubbed
    systemctl/loginctl with a call log, minimal git identity
    (`GIT_CONFIG_GLOBAL` — the fake HOME hides the real signing key);
    asserts pool.conf keys, ALL drain invariants in the unit, the three
    enable calls, host-untouched. Wired into ci-local.sh.
11. **`scripts/check-status-index.sh`** (§d6): ci-local fails on any
    unindexed `docs/status/*.md`. Found 10 MORE unindexed reports beyond
    the five known; the README index now names every report exactly
    (globs removed).
12. **ADR-0010** (`docs/adr/0010-journal-retention-stance.md`): compaction
    is operator-owned and manual (never a pool tick), the CLI surface
    waits for real demand (~50k facts or an operator ask), the retention
    floor becomes first-class observability when it lands, Postgres parity
    in the same change.
13. **Docs/hygiene**: `taskid.txt` git-rm'd + ignored (§d2); AGENTS.md
    Known Issues gained the queue-tasks-outlive-TODO-items bullet and the
    manual-worker `--once` runbook rule (§d1/§d3); TODO_LIST items marked
    with evidence; CHANGELOG/FEATURES updated.

## b) FULLY DONE — the two REAL bugs the new tests caught

1. **Budget bypass on same-tick mints** (`TestBudgetCapsStatusMintedEnqueues`,
   internal/e2e): the agent pool's review/status sweepers, cqa ingest, and
   the `--once` DRAIN sweeps never checked the daily budget — a completion
   inside the same tick could spend the last slot and still mint
   status/review tasks past the cap, contradicting SECURITY.md's "caps
   EVERY enqueue incl. status-minted". Fixed: `guard.Check` before every
   minting pass, each with its own skip warning. The e2e test has a
   positive control (uncapped twin mints exactly 1) so a broken sweeper
   cannot fake a pass. Green 3×.
2. **`FactsForTask(limit>0)` returned the FIRST n facts on BOTH stores**
   while the interface documents the MOST RECENT n (bounded callers: the
   webui detail render budget and status window reads). Found by the new
   Postgres battery against a real postgres:16 container. Both stores now
   read the tail (DESC LIMIT) and flip to ascending;
   `TestFactsForTaskFiltersAndBounds` re-pinned to the documented
   semantics.

## c) AUDITS — verdicts, no defects

- **Papdashboard watermark persistence** (§d5): the eager head-insert
  WORKS. The 18:31 bridge runs from `/tmp/papdbg` and its default DB path
  is CWD-relative — it reads/writes its OWN `tasks.db`, which carries
  `papdashboard:http://127.0.0.1:18100 @ 4` (= its head). The repo
  `tasks.db` was never that process's store; the 21:40 observation was a
  wrong-DB read. Already pinned by `TestFirstRunBootstrapsAtHead` + the
  restart battery. Evidence appended to the still-blocked papdbg TODO item
  (safe to kill; it forwards nothing — its head is still seq 4 and :18100
  is a local stub).
- **Postgres conformance** (§e-item): `TestPostgresConformance` now runs
  the full battery in CI's existing `-run TestPostgres` job (DAG gating,
  delay/priority order, retry ladder WITH evidence, permanent dead-letter,
  requeue-no-burn, cancel reasons pending+cooperative, heartbeat lease
  guard, filter/list/count, bounded fact reads). Verified locally against
  a throwaway `postgres:16` container (2× green) before removing it.
- **`TestRestartMidStreamLosesZeroFacts` deflake** (§d4-style): the 23:26
  B-phase fix already held; the remaining race was bridge A's crash window
  (its 200ms poll tick could legitimately pre-checkpoint seq 549 before
  the test's cancel landed). A's interval widened to 600ms (~150× margin)
  and `waitFor` deadline 2s→5s for loaded -race machines. 8× green.

## d) NOT STARTED / left for the owner (blocked, untouched)

- Push master to origin (never without go/no-go).
- Fate of the `/tmp/papdbg` worker — now decidable with the audit evidence
  (self-contained, inert wrt the repo queue, safe to kill).
- `tq cancel` dedup-key release semantics — policy decision, documented.

## e) Verification

- `go build ./...`, `go vet`, full `go test ./... -race` via
  `./scripts/ci-local.sh` (the pre-push gate: vet → build → windows
  cross-compile → race tests → gofmt → advisory lint → scoped lint
  annotations → harvest-parse guard → webui smoke → status-loop smoke →
  bootstrap-install smoke → doc-ref check → status-index check → nix
  build → nix flake check).
- Live CLI smokes: prune-stale, tasks, stats (--json + budget), prefix
  show (incl. the ambiguity error), bootstrap-install.
- Postgres battery against a real container (removed afterwards).
- Concurrent-session note: the webui write-actions session was active the
  whole time (AllowWrites/CSRF/board view landed around this work); the
  one shared-file moment (cmd/tq/main.go) was resolved by re-reading and
  re-applying. No foreign diffs were reverted.

## f) Follow-ups worth queueing

- `tq tasks --json` could gain `--since`-aware SQL pushdown if queue sizes
  ever make the CLI-side filter matter (Filter.Since on both stores).
- The prune reason prefix and the audit catch-up prompt now both say
  "item done" in different voices — one shared wording constant would keep
  the journal greppable.
