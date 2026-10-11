# ADR-0023: Cost-denominated daily budget beside the count cap

Date: 2026-10-11
Status: Accepted
Depends on: ADR-0001 (facts-first), the derived-outcomes contract
(executor usage derivation), the 2026-10-11 throughput-engine redesign
(docs/problems/major-prompting-and-batching.md, SUPERB pareto plan A6).

## Context

The pool's only native spend ceiling was `--daily-budget N`: a count of
task.enqueued facts per calendar day. On 2026-10-10 production parked ALL
real work at "30/30 tasks enqueued" while the day's measured model spend
was **$0.07** — the count was consumed by cheap derived ceremony (status
reports, autopsies, micro-tasks), and the axis the owner actually cares
about (money) was invisible to the gate. The operator escape was
`--budget-cmd` (shell out to an external ledger), which most deployments
do not wire.

The data to do better already exists in the journal: every completed
agent/review/status/dlqfix/prioritize run carries derived session usage
(tokens, messages, USD) in its completion fact; `budget.UsageToday`
already projects it (used until now only for log lines).

## Decision

1. `budget.Guard` gains `DailyCostCapUSD`: when > 0, `Check` refuses once
   today's derived session spend (USD, summed from completion facts) is
   **at or over** the cap. The refusal reason is cost-denominated
   (`daily cost cap reached: $X of $Y spent across N derived runs`) so
   logs and alerts say why in the owner's unit.
2. The count cap stays as-is; the two caps are independent axes and BOTH
   apply (whichever trips first refuses). Ordering is cheap-first: count
   (a SQL pushdown) before cost (a completion-fact scan).
3. Spend is measured on COMPLETED runs, so the cap necessarily lags
   enqueues: it stops FURTHER spend once observed spend crosses it. We
   accept the overshoot (at most the in-flight claims' cost) rather than
   pre-estimating per-task cost, which the queue has no honest way to
   know before a run.
4. Failure mode is fail-open, identical to the count projection: a
   journal error reads as zero spend and never wedges the pool
   (availability over a stuck ceiling; the journal being down is itself
   the louder incident).
5. CLI: `tq agent-pool --daily-cost-cap 5.00` (float USD, 0 = off);
   persisted like every pool flag via the `--config` flag-file
   (key `daily-cost-cap`). The claim-time budget hook inherits the cap
   automatically — it calls the same `Guard.Check`, so paid turns
   requeue class `budget` (no attempt burn) exactly like count refusals.

The count cap's role narrows to a coarse runaway guard (a broken
harvester loop minting thousands of tasks); the cost cap becomes the
real budget. Deployments that want strict per-task accounting keep
`--budget-cmd` (still the final authority when set).

## Consequences

- A cheap day is no longer starved by a count ceiling: with
  `--daily-cost-cap 5` the pool may run 100 micro-tasks costing $2 total,
  where a count cap of 30 would have parked it at 30.
- Budget alerts (papdashboard bridge) key off guard refusals and thus
  report cost exhaustion with the cost reason unchanged.
- The cap under-counts spend whose usage derivation is missing (pre-
  derivation records, sh/http tasks with no session): acceptable, those
  runs cost nothing by measurement.
- `tq doctor`'s budget check and `tq stats` usage surfaces already read
  `UsageToday`, so observability needs no change to see the new axis.
