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

| Feature                                                                   | Status                | Notes                                                                                |
| ------------------------------------------------------------------------- | --------------------- | ------------------------------------------------------------------------------------ |
| Durable enqueue (type, project, payload, priority, delay, attempt budget) | 🟢 `FULLY_FUNCTIONAL` | `internal/queue/sqlite.go` (`Enqueue`); semantics pinned by `store_test.go`          |
| Idempotent enqueue via dedup keys                                         | 🟢 `FULLY_FUNCTIONAL` | Partial unique index + in-tx re-check; legacy-DB migration tested                    |
| Lease-based exclusive claims                                              | 🟢 `FULLY_FUNCTIONAL` | Claim atomicity under concurrency: `TestExactlyOnceUnderConcurrency`                 |
| Crash reclaim (lease expiry)                                              | 🟢 `FULLY_FUNCTIONAL` | `TestLeaseExpiryAllowsReclaim`; reclaim records `task.released`                      |
| Heartbeat lease renewal                                                   | 🟢 `FULLY_FUNCTIONAL` | `TestHeartbeatExtendsLease`; lease-lost mid-run discards the attempt (at-least-once) |
| Retries with exponential backoff                                          | 🟢 `FULLY_FUNCTIONAL` | `worker.ExpBackoff` (2^n s, 5m cap); backoff gates via `not_before`                  |
| Dead-letter queue + rescue                                                | 🟢 `FULLY_FUNCTIONAL` | `tq dlq --rescue` re-queues with a fresh attempt budget                              |
| DAG dependencies (deps gate at claim)                                     | 🟢 `FULLY_FUNCTIONAL` | `TestDepsBlockUntilCompleted`; no orchestrator, one `NOT EXISTS` clause              |
| Delayed tasks (`NotBefore`)                                               | 🟢 `FULLY_FUNCTIONAL` | `TestNotBeforeDelays`                                                                |
| Priority ordering                                                         | 🟢 `FULLY_FUNCTIONAL` | Higher priority claims first, then FIFO                                              |
| Cancellation                                                              | 🟢 `FULLY_FUNCTIONAL` | Pending tasks only; running tasks must fail/complete naturally                       |
| Append-only fact journal + replay                                         | 🟢 `FULLY_FUNCTIONAL` | `tq facts`, `tq tail -f`; facts written in the same tx as state                      |
| Per-project stats and filters                                             | 🟢 `FULLY_FUNCTIONAL` | `tq stats [--project] [--json]`, `List` filters                                      |

## Executors

| Feature                           | Status                | Notes                                                                                                                                                                     |
| --------------------------------- | --------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `sh` command executor             | 🟢 `FULLY_FUNCTIONAL` | Payload shapes: raw line, JSON string, `{"cmd":...}`; output tail in errors                                                                                               |
| HTTP webhook executor             | 🟢 `FULLY_FUNCTIONAL` | POSTs the task envelope; 2xx completes, anything else fails the attempt                                                                                                   |
| Headless agent executor (`agent`) | 🟢 `FULLY_FUNCTIONAL` | Live-proven end-to-end (2026-09-06); verify gate enforced, clean-tree guard, argv contract test. Gaps: verify auto-detects Go/npm only; Windows has no process-group kill |
| Custom Go executors (Registry)    | 🟢 `FULLY_FUNCTIONAL` | 1-method interface + `RegisterFunc`; module-internal until packages go public                                                                                             |

## Agent pool

| Feature                                       | Status                    | Notes                                                                                                                                                                         |
| --------------------------------------------- | ------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Harvest TODO_LIST.md → agent tasks            | 🟢 `FULLY_FUNCTIONAL`     | Checkbox parsing (code-fence aware), stable dedup keys, per-repo pacing, tick cap, dry-run. Proven live (harvest → agent → `[x]` → commit → complete)                         |
| `tq agent-pool` self-managing loop            | 🟢 `FULLY_FUNCTIONAL`     | Periodic harvest + worker pool in one process; graceful drain lets agents finish. Per-repo serialization relies on harvester pacing only — store-level exclusivity is PLANNED |
| Autonomy guard (`.crushrc` required for yolo) | 🟢 `FULLY_FUNCTIONAL`     | Fails fast with remediation guidance. Known false positive: repos relying on user-global crush permissions                                                                    |
| CQA findings → fix tasks                      | 🟡 `PARTIALLY_FUNCTIONAL` | `internal/bridge/cqa` groups fixable issues per file, dedup keys include scan ID. httptest-tested only — never verified against a live CQA API                                |

## Bridges

| Feature                         | Status                | Notes                                                                                                                                                                                                               |
| ------------------------------- | --------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| PapDashboard dead-letter alerts | 🟢 `FULLY_FUNCTIONAL` | `alert.triggered` on dead-letter, `alert.resolved` on later completion; E2E-verified against a live PapDashboard instance. Journal starts at head per process: incidents while the bridge was down are not replayed |

## CLI (`tq`)

| Feature                                                       | Status                | Notes                                               |
| ------------------------------------------------------------- | --------------------- | --------------------------------------------------- |
| enqueue / worker / stats / show / dlq / cancel / facts / tail | 🟢 `FULLY_FUNCTIONAL` | `tq worker` runs until signalled (no one-shot mode) |
| harvest / agent-pool                                          | 🟢 `FULLY_FUNCTIONAL` | See Agent pool; `--dry-run` for preview             |

## Tooling

| Feature                             | Status                | Notes                                                                                                                 |
| ----------------------------------- | --------------------- | --------------------------------------------------------------------------------------------------------------------- |
| Nix flake build + vendor-hash check | 🟢 `FULLY_FUNCTIONAL` | `nix build` produces the `tq` binary (`CGO_ENABLED=0`); `nix flake check` passes including the vendor-hash drift gate |
| CI (vet, build, test -race, gofmt)  | 🟢 `FULLY_FUNCTIONAL` | `.github/workflows/ci.yml`; runs on every push                                                                        |

## Planned (no code yet)

| Feature                                    | Status       | Notes                                                             |
| ------------------------------------------ | ------------ | ----------------------------------------------------------------- |
| Postgres store (`SKIP LOCKED`)             | ⚪ `PLANNED` | The `Store` interface is the seam (ADR-0001)                      |
| HTTP API server for non-Go producers       | ⚪ `PLANNED` | v0.2 direction (ROADMAP)                                          |
| Store-level per-project claim exclusivity  | ⚪ `PLANNED` | Multi-pool per-repo serialization; SQL sketched, unimplemented    |
| Decision → question fan-out (PapDashboard) | ⚪ `PLANNED` | Agent asks, human answers in the dashboard, queue proceeds        |
| Cost budgets per repo/day                  | ⚪ `PLANNED` | Agent runs cost real money; `--max-per-tick` bounds per tick only |
