# Domain Language

The ubiquitous language of go-taskqueue. Every identifier in the codebase,
every flag in the CLI, and every fact in the journal should use these terms
with exactly these meanings. When code and this file disagree, one of them is
a bug.

Cross-links: [ADR-0001](../docs/adr/0001-facts-first-sqlite-leases.md)
(facts-first core), [ADR-0002](../docs/adr/0002-agent-pool-autonomy-pacing-drain.md)
(agent-pool policies), [ADR-0003](../docs/adr/0003-web-ui-architecture.md)
(live web UI: serve, tailer, hub, fragment, projection).

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

| Term                        | Meaning                                                                                                                                                                                                                                                                                                                                                        |
| --------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Claim**                   | The atomic act of one worker taking exclusive responsibility for a due task. Due = pending, not-before passed, deps completed.                                                                                                                                                                                                                                 |
| **Lease**                   | The time-boxed exclusivity a claim grants, renewed by **heartbeat**. Expired lease ⇒ another worker may **reclaim** — the at-least-once contract in one mechanism.                                                                                                                                                                                             |
| **Retry**                   | A failed transient attempt, after exponential **backoff** (gated by `not_before`), within the task's attempt budget.                                                                                                                                                                                                                                           |
| **Permanent error**         | An error class retrying cannot fix (bad payload, missing repo, unknown type). Dead-letters after ONE attempt.                                                                                                                                                                                                                                                  |
| **Preflight refusal**       | "The environment wasn't ready" (dirty tree, missing autonomy config). Requeues WITHOUT burning an attempt — the task retries when a human fixes the environment.                                                                                                                                                                                               |
| **Rate-limit park**         | A provider exhaustion response (Z.ai usage window, synthetic.new quota) requeues the task to `not_before` = reset time WITHOUT burning an attempt — identical retries fail identically until the window resets. `tq tasks --parked` lists them; the parked count is a WAITING pool, not a broken one.                                                          |
| **Rate-limit gate**         | The executor's in-process memory of a provider's exhaustion (per-REPO: a repo's `.crushrc` fixes its provider, so a Z.ai 429 in one repo never parks another repo's tasks). Armed on detection; fast-refuses sibling runs without spawning the agent binary.                                                                                                   |
| **DLQ (dead-letter queue)** | Where tasks land when attempts are exhausted or the error is permanent. Inspect with `tq dlq`, inspect reasons via the fact's error class.                                                                                                                                                                                                                     |
| **Rescue**                  | Re-queueing a dead task with a fresh attempt budget (`tq dlq --rescue`, or the DLQ-autopsy sweeper acting on a `fixed` verdict).                                                                                                                                                                                                                               |
| **Autopsy (dlqfix task)**   | The ONE second-agent diagnosis a dead AGENT task gets (`--dlq-fix`): it reads the failure evidence, either fixes the root cause (→ rescue) or rules the task unfixable with a required reason (→ **Dismiss**: `Dead → Cancelled`, reason recorded on the cancelled fact — `tq dlq --dismiss`). One autopsy per dead task, ever; autopsies are never autopsied. |

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

## Prioritization

| Term             | Meaning                                                                                                                                                                                                      |
| ---------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| **Importance**   | Per-repo owner intent, 0–100, read from each repo's `.config/metadata.yaml` (project-meta file contract). Default 50 when absent; malformed file ⇒ repo skip reason. Ordering only, never budget (ADR-0015). |
| **Marker**       | Trailing TODO_LIST item suffix `— P[1-4]` (optional `: <note>`), mapping to 90/70/50/30. Highest-precedence human priority signal; stripped before the dedup-key hash so edits never fork tasks.             |
| **Item score**   | Harvest-time effective priority resolved by precedence: marker > hot (same-session) > AI score > importance + keyword bumps > default 50. Clamped to the backlog band (0–99).                                |
| **Band**         | Reserved range of the ONE priority scale: backlog 0–99, hot 100–149 (session urgency), machine 150+ (operational tasks). Crossing upward requires an explicit human/flag act; scorers clamp below it.        |
| **Working set**  | The pending slice actually eligible for admission (`MaxPendingPerRepo`); TODO_LIST.md is the warehouse, the queue is the working set — AI scoring cost is O(working set), not O(backlog).                    |
| **Aging**        | Claim-query scheduling term: `priority + LEAST(age/AgingDays, MaxAgeBonus)` keyed on immutable `created_at`. Scheduling, not state — stored priority never changes (ADR-0015 §4).                            |
| **Unblock bump** | Priority bump applied to dependents when their dependency completes — the queue's answer to "the blocker finished, now it matters".                                                                          |
| **Score cache**  | `priority_scores` rows keyed by the item dedup-key derivation: (score, effort, source, reasoning, tokens, scored_at). An AI verdict persists and re-derives the same priority until the text changes.        |

## Observation

| Term                     | Meaning                                                                                                                                                                                                                                                                                      |
| ------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Replay**               | Reading the journal (`tq facts`, `tq tail -f`) to reconstruct anything that happened. The answer to "what exactly happened" is always in the facts.                                                                                                                                          |
| **Watermark**            | A journal consumer's persisted read cursor: the last fact seq it accepts as delivered (`watermarks` table, `queue.Store.Watermark/SaveWatermark`). Monotonic at runtime — never regresses except by an explicit ops `set`.                                                                   |
| **Consumer key**         | The watermark-table identity of one resumable reader, namespaced per source (e.g. `papdashboard:<endpoint>`). Distinct endpoints hold distinct cursors.                                                                                                                                      |
| **Checkpoint**           | The write that persists a watermark — always AFTER the last accepted fact of a batch, never before (a pre-acceptance checkpoint would silently convert at-least-once delivery to at-most-once).                                                                                              |
| **Lag**                  | `HeadSeq − watermark`: how far behind the journal head a consumer's cursor sits (`tq watermarks show`).                                                                                                                                                                                      |
| **Dispatcher**           | The `internal/consumer` fan-out over `Store` bounded reads: per-subscriber cursor, at-least-once in-order delivery, lag observability (ADR-0009). Deliberately OFF the `Store` interface.                                                                                                    |
| **Exact consumer**       | A consumer class that requires EVERY fact, in seq order, at-least-once (bridges, sweepers, workers). Slow exact consumers apply backpressure — the dispatcher blocks, never skips. Persisted via a **watermark**.                                                                            |
| **Signal consumer**      | A consumer class that only needs a wake-up when facts changed (dashboard hub/tailer); payloads stay off the wire (ADR-0003). Overflow drops + coalesces — the next snapshot re-renders truth, so nothing is lost.                                                                            |
| **Result detail**        | Structured data recorded in the `task.completed` fact (agent session id, verify output tail) — rendered by `tq show`.                                                                                                                                                                        |
| **Serve**                | `tq serve`: the read-only live dashboard. Binds localhost by default and can only render projections, never mutate the queue.                                                                                                                                                                |
| **Tailer**               | The single goroutine polling `Facts(after)` behind `tq serve`; it advances a **watermark** and notifies once per burst of new facts.                                                                                                                                                         |
| **Hub**                  | The fan-out point every dashboard browser subscribes to; one notification re-renders one full snapshot per client.                                                                                                                                                                           |
| **Fragment**             | One named server-rendered HTML region (stats cards, task table, DLQ, fact feed) the browser swaps by container id — the client keeps no state.                                                                                                                                               |
| **Projection**           | Any view derived from facts (CLI tables, stats, and every dashboard fragment). Staleness is the worst failure; facts are never corrupted by reading.                                                                                                                                         |
| **Session bridge**       | `tq session begin/close`: gives INTERACTIVE crush sessions the pool's close-out. Close attributes the session's footer commits, then directly enqueues one review + one status task; the pool does the rest (facts-first: `session.opened`/`session.closed` observations, never task state). |
| **Attributed commit**    | A commit carrying the `Crush-Session: <id>` git footer — the session it belongs to. Exact trailer matching; quoting someone else's footer never attributes.                                                                                                                                  |
| **Synthetic session ID** | The `session:<id>` lineage key standing in for a session wherever a task ID shape is expected (facts' TaskID, dedup keys, payload lineage). Never a real task row.                                                                                                                           |

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

## Public surface

- **Facade module**: a public module (`task`, `journal`, `queue`,
  `queue/sqlite`, `queue/postgres`, `executor`, `worker`) that re-exports
  one internal implementation module's entire exported surface via type
  aliases and var/const re-exports (ADR-0016). The facade pins NAMES; the
  implementation stays internal and free to refactor. In-repo code keeps
  importing `internal/…` — facades are the external contract only, and
  every new internal export gains its alias in the same change (pinned by
  `scripts/check-facade-parity.sh`).
