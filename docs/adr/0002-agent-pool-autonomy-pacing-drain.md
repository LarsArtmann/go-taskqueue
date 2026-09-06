# ADR-0002: The agent pool — autonomy, pacing, budgets, and drain semantics

**Status:** Accepted (2026-09-06)
**Context:** ADR-0001 built the queue core. The `tq agent-pool` layer turns it
into a self-managing improvement loop: repos' TODO_LIST.md backlogs are
harvested into `agent` tasks, and a worker pool drives headless crush agents
that do the work, prove it, and close the loop in the file. That loop spends
real money per task and mutates real repos unattended, so four policy
questions had to be answered before it could run without a human babysitter:
who may act (autonomy), how much at once (pacing), how much per day
(budgets), and what happens when the operator hits Ctrl-C (drain).

**Decisions:**

1. **Autonomy is granted by the repo, not the operator's flag.** `crush run`
   has no `--yolo` flag (v0.92 rejects it), so `--yolo` is only the
   operator's _request_. The actual grant is the repo's own `.crushrc`
   permissions (or a user-global crush config) — the pool cannot over-grant
   what a repo never offered. A yolo task on a repo without any config is a
   **preflight refusal**: it requeues without burning an attempt
   (`Store.Requeue`, `executor.PreflightError`) and becomes claimable again
   after a short backoff, so committing the config is the only fix needed.
   The same class covers dirty trees: the pool never tramples human WIP.
2. **Error classes decide the retry policy, at the store level.**
   Permanent mistakes (bad payload, missing repo, unknown type, non-zero
   verify-less `sh` exit, 4xx HTTP) dead-letter after ONE attempt — retrying
   cannot fix them. Preflight refusals requeue without burning an attempt.
   Everything else retries with exponential backoff into the DLQ. A failed
   verify gate is deliberately _transient_: the agent may have half-finished,
   and a fresh attempt re-runs the whole task.
3. **Pacing is layered, cheapest guard first.** Within one harvest pass:
   at most one in-flight item per repo (skip "repo busy"), one NEW enqueue
   per repo per run, a global tick cap (`--max-per-tick`). Across runs:
   per-repo minimum gaps (`--repo-interval`). Across health: a repo whose
   recent work all died pauses for `--dlq-backoff` (poisoned repos must not
   refill their own attempt budget). Pacing alone, however, is per-harvester
   state — which is why exclusivity exists (next).
4. **Per-project exclusivity lives in the store, not the pool.**
   `queue.WithProjectExclusivity` adds a claim-guard clause: a project with a
   running task yields nothing to ANY process sharing the database (reclaims
   of the same task stay exempt). Tradeoff accepted: per-project throughput
   is serialized by design — two agents editing one repo concurrently is the
   bug this buys away. Opt-in because single-pool users lose nothing and
   empty-projected tasks (ops jobs) are exempt.
5. **Budgets are projections, not state.** `--daily-budget` projects
   "enqueued since local midnight" from the journal facts — no counters to
   persist, no clock-state bugs, timezone-honest on the operator's machine.
   `--budget-cmd` makes an external accounting the final authority (non-zero
   exit vetoes the tick). Both are checked before every harvest tick; the
   budget never blocks draining of already-enqueued work.
6. **Drain semantics: tasks outlive the shutdown context.** `Start(ctx)`
   returns only when the caller's context is done. In-flight tasks (agent
   runs, heartbeats, terminal writes) execute under a shutdown-surviving
   context bounded only by `--task-timeout`, so a task claimed minutes into
   uptime still completes and records its outcome (pinned by a regression
   test that fails on revert). Graceful stop = stop claiming, let in-flight
   work finish. `--once` runs one harvest tick, then drains — and because
   `Start` never returns on its own, the drain watcher cancels the context
   when the queue is quiet.

**Consequences:** The loop is safe to leave unattended within the budget
caps, but the trust root is the filesystem: any repo with a permissive
`.crushrc` and a spot in `--projects-dir` can spend pool money (see
SECURITY.md). Catch-up repairs (`tq audit`) inherit the same autonomy model.
Verify relies on the repo's declared command — a repo that lies about its
`.tq-verify` lies to the gate.

**Cross-check:** claims 1–6 map to `internal/executor/agent.go`
(classification, verify chain), `internal/worker/worker.go` (preflight /
permanent branches, shutdown-surviving task context),
`internal/harvest` (pacing layers, `Audit`),
`internal/queue` (`WithProjectExclusivity`, `Requeue`, `FailPermanent`),
`internal/budget` (projections), `cmd/tq` (flags, `--once` drain watcher).
Multi-repo two-pool behavior is proven live by `scripts/smoke/multi-repo.sh`.
