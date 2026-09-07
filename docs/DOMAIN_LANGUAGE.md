# Domain Language

The ubiquitous language of go-taskqueue. Every identifier in the codebase,
every flag in the CLI, and every fact in the journal should use these terms
with exactly these meanings. When code and this file disagree, one of them is
a bug.

Cross-links: [ADR-0001](../docs/adr/0001-facts-first-sqlite-leases.md)
(facts-first core), [ADR-0002](../docs/adr/0002-agent-pool-autonomy-pacing-drain.md)
(agent-pool policies).

## Queue core

| Term          | Meaning                                                                                                                                                                                                           |
| ------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Task**      | One unit of durable work. Has an immutable identity (`ID`), a `Project`, a `Type`, a JSON `Payload`, and lifecycle state. Created via `Enqueue`; never mutated except through store operations that append facts. |
| **Project**   | The repo (or logical owner) a task belongs to. The unit of pacing, exclusivity, budgets, and stats. An empty project means "ungrouped ops task" and is exempt from per-project policies.                          |
| **Type**      | The executor key: which `Executor` runs this task (`sh`, `http`, `agent`, or a registered Go func).                                                                                                               |
| **Payload**   | The opaque-to-core JSON instruction the executor interprets (a shell line, a URL envelope, an agent `{repo, prompt}`).                                                                                            |
| **Dedup key** | Stable identity of _recurring intentions_: enqueueing twice with the same key yields one task (partial unique index). Harvest derives it from repo + item text; catch-up tasks prefix it with `catchup:`.         |
| **Priority**  | Claim-order weight. Higher claims first — the queue is priority-ordered, never FIFO.                                                                                                                              |
| **Deps**      | Task IDs that must be `completed` before this task is claimable. DAG ordering with no scheduler: one `NOT EXISTS` clause in the claim query.                                                                      |
| **Status**    | `pending → running → completed`, with `dead` (exhausted) and `cancelled` (withdrawn) as exits.                                                                                                                    |
| **Fact**      | One immutable observation about one task (`task.enqueued`, `task.claimed`, …) — the only write besides state changes, in the same transaction.                                                                    |
| **Journal**   | The append-only sequence of facts. Every view (stats, DLQ, budget projection) is a replay over it; nothing is ever deleted.                                                                                       |

## Claiming & failure

| Term                        | Meaning                                                                                                                                                            |
| --------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| **Claim**                   | The atomic act of one worker taking exclusive responsibility for a due task. Due = pending, not-before passed, deps completed.                                     |
| **Lease**                   | The time-boxed exclusivity a claim grants, renewed by **heartbeat**. Expired lease ⇒ another worker may **reclaim** — the at-least-once contract in one mechanism. |
| **Retry**                   | A failed transient attempt, after exponential **backoff** (gated by `not_before`), within the task's attempt budget.                                               |
| **Permanent error**         | An error class retrying cannot fix (bad payload, missing repo, unknown type). Dead-letters after ONE attempt.                                                      |
| **Preflight refusal**       | "The environment wasn't ready" (dirty tree, missing autonomy config). Requeues WITHOUT burning an attempt — the task retries when a human fixes the environment.   |
| **DLQ (dead-letter queue)** | Where tasks land when attempts are exhausted or the error is permanent. Inspect with `tq dlq`, inspect reasons via the fact's error class.                         |
| **Rescue**                  | Re-queueing a dead task with a fresh attempt budget — always a human decision (`tq dlq --rescue`).                                                                 |

## Harvest loop

| Term              | Meaning                                                                                                                                                                                              |
| ----------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Harvest**       | One scan of repos' TODO_LIST.md files that enqueues one agent task per open, untracked checkbox item. Idempotent via dedup keys.                                                                     |
| **Item**          | One checkbox line in a todo file. Editing its text re-arms it (new dedup key); ticking it removes it from harvest.                                                                                   |
| **Tick**          | One pass of the pool loop: harvest, ingest bridges, then let workers claim. Budgets and caps bound each tick.                                                                                        |
| **Verify gate**   | The command a task must pass to complete. Resolution order: repo's `.tq-verify` file → payload `verify` → auto-detect. A failed verify is transient.                                                 |
| **Pacing**        | The layered rate limits of the loop: one in-flight item per repo, one new enqueue per repo per run, tick cap, per-repo interval, poisoned-repo (DLQ) backoff.                                        |
| **Exclusivity**   | Store-level guarantee (opt-in): a project never has two running tasks across ALL processes sharing the database.                                                                                     |
| **Budget**        | Cost ceiling for unattended pools: daily cap projected from the journal, or an external budget command whose non-zero exit vetoes the tick.                                                          |
| **Drift**         | File-vs-queue disagreement found by `tq audit`: _stale-open_ (task completed, checkbox unticked — repaired by a **catch-up task**) or _stale-done_ (checkbox ticked, task unfinished — report-only). |
| **Catch-up task** | A dedup-keyed (`catchup:` prefix) agent task whose only job is to close the loop in the file for work already done and verified. Armed at most once.                                                 |

## Observation

| Term              | Meaning                                                                                                                                             |
| ----------------- | --------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Replay**        | Reading the journal (`tq facts`, `tq tail -f`) to reconstruct anything that happened. The answer to "what exactly happened" is always in the facts. |
| **Result detail** | Structured data recorded in the `task.completed` fact (agent session id, verify output tail) — rendered by `tq show`.                               |
| **Serve**         | `tq serve`: the read-only live dashboard. Binds localhost by default and can only render projections, never mutate the queue.                       |
| **Tailer**        | The single goroutine polling `Facts(after)` behind `tq serve`; it advances a **watermark** and notifies once per burst of new facts.                 |
| **Hub**           | The fan-out point every dashboard browser subscribes to; one notification re-renders one full snapshot per client.                                  |
| **Fragment**      | One named server-rendered HTML region (stats cards, task table, DLQ, fact feed) the browser swaps by container id — the client keeps no state.      |
| **Projection**    | Any view derived from facts (CLI tables, stats, and every dashboard fragment). Staleness is the worst failure; facts are never corrupted by reading. |

## Bounded contexts

- **Queue core** (`internal/queue`, `internal/task`, `internal/journal`):
  tasks, claims, leases, facts. Knows nothing about repos or agents.
- **Execution** (`internal/executor`, `internal/worker`): turns payloads
  into done work and classifies failures. Knows payloads, not the loop.
- **Ingestion loop** (`internal/harvest`, `internal/budget`,
  `internal/bridge/*`): decides what work exists and how much may run.
  Knows repos and money, not leases.
- **Observation** (`cmd/tq` read commands, `internal/journal`): projections
  and replay. Writes nothing.

The seams are one-directional: the loop enqueues through the core; workers
execute through the core; observation only reads.
