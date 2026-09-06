# Features

> Honest inventory of what this project does, by status. Studied from the
> code, not the marketing claims. Updated as features ship, change, or break.

## Status legend

| Status                    | Meaning                                                      |
| ------------------------- | ------------------------------------------------------------ |
| 🟢 `FULLY_FUNCTIONAL`     | Works as intended, exercised by tests or daily use.          |
| 🟡 `PARTIALLY_FUNCTIONAL` | Ships but has known gaps, edge-case bugs, or missing pieces. |
| 🔴 `BROKEN`               | Present in code but not working / disabled / failing.        |
| ⚪ `PLANNED`              | Designed or documented but **not yet implemented** in code.  |

## Queue core

| Feature                                                                   | Status                | Notes                                                                                                            |
| ------------------------------------------------------------------------- | --------------------- | ---------------------------------------------------------------------------------------------------------------- |
| Durable enqueue (type, project, payload, priority, delay, attempt budget) | 🟢 `FULLY_FUNCTIONAL` | `internal/queue/sqlite.go` (`Enqueue`); semantics pinned by `store_test.go`                                      |
| Idempotent enqueue via dedup keys                                         | 🟢 `FULLY_FUNCTIONAL` | Partial unique index + in-tx re-check; legacy-DB migration tested                                                |
| Lease-based exclusive claims                                              | 🟢 `FULLY_FUNCTIONAL` | Claim atomicity under concurrency: `TestExactlyOnceUnderConcurrency`                                             |
| Crash reclaim (lease expiry)                                              | 🟢 `FULLY_FUNCTIONAL` | `TestLeaseExpiryAllowsReclaim`; reclaim records `task.released`                                                  |
| Heartbeat lease renewal                                                   | 🟢 `FULLY_FUNCTIONAL` | `TestHeartbeatExtendsLease`; lease-lost mid-run discards the attempt (at-least-once)                             |
| Retries with exponential backoff                                          | 🟢 `FULLY_FUNCTIONAL` | `worker.ExpBackoff` (2^n s, 5m cap); backoff gates via `not_before`                                              |
| Dead-letter queue + rescue                                                | 🟢 `FULLY_FUNCTIONAL` | `tq dlq --rescue` re-queues with a fresh attempt budget                                                          |
| DAG dependencies (deps gate at claim)                                     | 🟢 `FULLY_FUNCTIONAL` | `TestDepsBlockUntilCompleted`; no orchestrator, one `NOT EXISTS` clause                                          |
| Delayed tasks (`NotBefore`)                                               | 🟢 `FULLY_FUNCTIONAL` | `TestNotBeforeDelays`                                                                                            |
| Priority ordering                                                         | 🟢 `FULLY_FUNCTIONAL` | Higher priority claims first, then FIFO                                                                          |
| Cancellation                                                              | 🟢 `FULLY_FUNCTIONAL` | Pending tasks only; running tasks must fail/complete naturally                                                   |
| Append-only fact journal + replay                                         | 🟢 `FULLY_FUNCTIONAL` | `tq facts`, `tq tail -f`; facts written in the same tx as state                                                  |
| Per-project stats and filters                                             | 🟢 `FULLY_FUNCTIONAL` | `tq stats [--project] [--json]`, `List` filters                                                                  |
| Permanent vs transient error classes                                      | 🟢 `FULLY_FUNCTIONAL` | `executor.PermanentError` → `FailPermanent` dead-letters after ONE attempt; `tq facts` shows `[class=permanent]` |
| Preflight requeue (no attempt burn)                                       | 🟢 `FULLY_FUNCTIONAL` | `executor.PreflightError` → `Store.Requeue`: pending again, attempts unchanged, `task.requeued` fact             |
| Per-project claim exclusivity (opt-in)                                    | 🟢 `FULLY_FUNCTIONAL` | `WithProjectExclusivity` / `--project-exclusive`; store-level, cross-pool; live-smoked with 2 pools × 3 repos    |

## Executors

| Feature                           | Status                | Notes                                                                                                                                                                                                                                                                                                                                 |
| --------------------------------- | --------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `sh` command executor             | 🟢 `FULLY_FUNCTIONAL` | Payload shapes: raw line, JSON string, `{"cmd":...}`; output tail in errors                                                                                                                                                                                                                                                           |
| HTTP webhook executor             | 🟢 `FULLY_FUNCTIONAL` | POSTs the task envelope; 2xx completes; 408/429/5xx retry, other failures dead-letter after one attempt                                                                                                                                                                                                                               |
| Headless agent executor (`agent`) | 🟢 `FULLY_FUNCTIONAL` | Live-proven end-to-end (2026-09-06); verify gate enforced (`.tq-verify` file wins, then payload, then auto-detect incl. Makefile/flake/cargo), clean-tree guard, argv contract test. Input mistakes dead-letter after one attempt; dirty tree / missing autonomy requeue without attempt burn. Gap: Windows has no process-group kill |
| Custom Go executors (Registry)    | 🟢 `FULLY_FUNCTIONAL` | 1-method interface + `RegisterFunc`; module-internal until packages go public                                                                                                                                                                                                                                                         |

## Agent pool

| Feature                                                                              | Status                    | Notes                                                                                                                                                                                                                      |
| ------------------------------------------------------------------------------------ | ------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Harvest TODO_LIST.md → agent tasks                                                   | 🟢 `FULLY_FUNCTIONAL`     | Checkbox parsing (code-fence aware), stable dedup keys, per-repo pacing, tick cap, dry-run. Proven live (harvest → agent → `[x]` → commit → complete)                                                                      |
| `tq agent-pool` self-managing loop                                                   | 🟢 `FULLY_FUNCTIONAL`     | Periodic harvest + worker pool in one process; graceful drain lets agents finish. `--once` runs one tick and exits (systemd unit in `deploy/systemd/`); store-level per-project exclusivity opt-in (`--project-exclusive`) |
| Autonomy guard (`.crushrc` required for yolo)                                        | 🟢 `FULLY_FUNCTIONAL`     | Fails fast with remediation guidance; user-global crush config satisfies the probe; refusals requeue without burning an attempt                                                                                            |
| Cost ceilings (`--daily-budget`, `--budget-cmd`, `--repo-interval`, `--dlq-backoff`) | 🟢 `FULLY_FUNCTIONAL`     | Daily cap projected from journal facts; budget command is the final authority; per-repo pacing + poisoned-repo cooldown. Unit-tested + live-smoked                                                                         |
| CQA findings → fix tasks                                                             | 🟡 `PARTIALLY_FUNCTIONAL` | `internal/bridge/cqa` groups fixable issues per file, dedup keys include scan ID. httptest-tested only — never verified against a live CQA API                                                                             |

## Bridges

| Feature                         | Status                | Notes                                                                                                                                                                                                               |
| ------------------------------- | --------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| PapDashboard dead-letter alerts | 🟢 `FULLY_FUNCTIONAL` | `alert.triggered` on dead-letter, `alert.resolved` on later completion; E2E-verified against a live PapDashboard instance. Journal starts at head per process: incidents while the bridge was down are not replayed |

## CLI (`tq`)

| Feature                                                       | Status                | Notes                                                                                               |
| ------------------------------------------------------------- | --------------------- | --------------------------------------------------------------------------------------------------- |
| enqueue / worker / stats / show / dlq / cancel / facts / tail | 🟢 `FULLY_FUNCTIONAL` | `tq worker` runs until signalled (no one-shot mode); `tq facts` renders the dead-letter error class          |
| `tq show` — task + full fact trail                              | 🟢 `FULLY_FUNCTIONAL` | Completed agent tasks record session id + verify tail in `task.completed`; `tq show` renders the whole trail |
| harvest / agent-pool                                            | 🟢 `FULLY_FUNCTIONAL` | See Agent pool; `--dry-run` for preview; `--model`, `--once`, cost ceilings                                  |

## Tooling

| Feature                             | Status                | Notes                                                                                                                 |
| ----------------------------------- | --------------------- | --------------------------------------------------------------------------------------------------------------------- |
| Nix flake build + vendor-hash check | 🟢 `FULLY_FUNCTIONAL` | `nix build` produces the `tq` binary (`CGO_ENABLED=0`); `nix flake check` passes including the vendor-hash drift gate |
| CI (vet, build, test -race, gofmt)  | 🟢 `FULLY_FUNCTIONAL` | `.github/workflows/ci.yml`; runs on every push; includes TODO_LIST harvest-parse guard + doc ghost-reference check    |
| CI nix build + flake check          | 🟢 `FULLY_FUNCTIONAL` | Keyless runner-safe (HTTPS flake inputs); caught the vendor-hash drift class in review, not in production             |
| Multi-repo two-pool live smoke      | 🟢 `FULLY_FUNCTIONAL` | `scripts/smoke/multi-repo.sh`: 3 repos, 2 pools, 1 DB — no double-enqueue, one claim per task, both pools work        |
| E2E subprocess suite                | 🟢 `FULLY_FUNCTIONAL` | `internal/e2e`: builds the real `tq` binary, drives `agent-pool --once` with a stub agent from outside; runs in CI    |
| Property + fuzz tests (harvest)     | 🟢 `FULLY_FUNCTIONAL` | Dedup-key stability property; `FuzzParseRepo` (CRLF/BOM/nesting), 1.8M execs clean                                    |
| Chaos test (kill mid-run)           | 🟢 `FULLY_FUNCTIONAL` | `internal/e2e/chaos_test.go`: SIGKILL victim, lease-expiry reclaim, exactly one completion in the journal             |

## Planned (no code yet)

| Feature                                    | Status       | Notes                                                                     |
| ------------------------------------------ | ------------ | ------------------------------------------------------------------------- |
| Postgres store (`SKIP LOCKED`)             | ⚪ `PLANNED` | The `Store` interface is the seam (ADR-0001)                              |
| HTTP API server for non-Go producers       | ⚪ `PLANNED` | v0.2 direction (ROADMAP)                                                  |
| Decision → question fan-out (PapDashboard) | ⚪ `PLANNED` | Agent asks, human answers in the dashboard, queue proceeds                |
| Per-repo daily budgets                     | ⚪ `PLANNED` | Global daily cap + per-repo intervals ship; per-REPO daily caps don't yet |
