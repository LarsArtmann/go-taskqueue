# ADR-0005: Cooperative cancel of running tasks

- Status: Accepted
- Date: 2026-09-08
- Deciders: owner + agent session (round-5 plan, idea I58 / task M9)
- Relates to: ADR-0001 (facts-first core), ADR-0002 (agent-pool policies)

## Context

`tq cancel` could only withdraw **pending** tasks. A running agent task —
potentially minutes of real LLM spend — could not be stopped at all: the
operator's only options were waiting out `--task-timeout` or SIGKILLing the
worker and eating the lease-expiry wait. Cancelling a leased task by
flipping the row from outside the worker also risks split-brain: the worker
would keep executing and then write a terminal outcome over the cancel.

## Decision

Cancel of a running task is **cooperative**: an operator records a request;
the executing worker observes it and stops itself. The facts journal is the
only carrier of the request — no task-row column, no out-of-band channel.

1. **Request**: `tq cancel <id> --force` appends a `task.cancel-requested`
   fact (`Store.CancelRunning`). The fact IS the flag; observation is an
   indexed `EXISTS` over `facts(task_id, type)`. Re-requests append nothing
   (idempotent by construction).
2. **Observation**: the worker's heartbeat goroutine, after each
   successful lease extension, checks `Store.CancelRequested`. Detection
   latency is bounded by the heartbeat cadence (lease/4 by default).
3. **Stop**: the heartbeat goroutine cancels the execution context.
   Executors kill their whole process tree on context cancellation
   (`prepareProcessGroup`, SIGKILL to the group — `sh` and `agent` both).
4. **Finalize**: the worker records the terminal state with
   `Store.CancelOwned(id, owner)` — Running → Cancelled guarded by the
   lease owner, appending `task.cancelled` (`cooperative: true`). The
   attempt does not burn; a cancelled task is withdrawn, not failed.
5. **Crashed worker**: if the worker dies before honoring the request, the
   expired-lease reclaim inside `ClaimDue` sees the pending request and
   finalizes the cancel (Released + Cancelled facts) instead of
   re-executing the task. The cancel is durable precisely because the
   journal, not the worker, holds it.

## Consequences

- **No split-brain**: only the lease holder finalizes; a stale worker's
  `CancelOwned` fails with `ErrLeaseNotHeld` and its result is discarded.
- **At-least-once unchanged**: a cancel that races a completion loses —
  whoever writes the terminal state first wins, and the journal shows which.
- **No new schema**: zero migration; `idx_facts_task` already serves the
  observation query.
- **CLI honesty**: `tq cancel` without `--force` on a running task refuses
  with the exact remedy, rather than pretending to cancel something it
  cannot stop.
- **UI for free**: the request fact flows into the fact feed and the task
  detail timeline (warning tone) without webui changes.

## Alternatives considered

- **Row flag (`cancel_requested` column)**: rejected — duplicates journal
  state in the projection and requires a migration + every scan path.
- **Immediate row flip to `cancelled`**: rejected — the running worker
  would heartbeat-fail, discard its result, and the task could complete
  "after death" in the journal's eyes; also steals the lease semantics.
- **Signal the worker process directly**: rejected — multi-process pools
  share one DB; the journal is the only channel every worker already
  tails.
