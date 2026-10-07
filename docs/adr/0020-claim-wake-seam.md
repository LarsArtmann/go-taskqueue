# ADR-0020: Claim-wake seam — buffered-1 store signal, polls stay the fallback

Date: 2026-10-07
Status: Accepted (M7 of docs/planning/2026-10-07_03-34_pareto-trunk-green-to-production.md)

## Context

Every latency-sensitive surface of tq reacts to the SAME event — a commit
that lands work in PENDING — but each learned about it only at its own poll
cadence: worker idle loops at the ladder cap (2s at defaults, after the
2026-10-07 IO pass), the consumer dispatcher at 1s. A freshly enqueued task
waited out the poll gap for no reason: the store knew, at commit time, that
a claim was now due.

ADR-0009 D4 already established the pattern for the dispatcher: notify
OUTSIDE the mutation transaction, keep the poll as the degraded fallback.
The wake-trace design memo (docs/planning/2026-10-05_wake-trace-design-memo.md)
proposes a `task.wake` journal FACT for audit visibility — a different
(owner-gated, §g-2) layer; this ADR is the in-process channel.

## Decision

1. `queue.Waker` is a SIDE interface (`Notify() <-chan struct{}`), not a
   Store method: optional runtime capability must not break every Store
   fake and conform pin (the ADR-0009 D4 precedent). Both v4 backends
   implement it — sqlitev4 (which the sqlite facade aliases) and
   postgresv4.
2. The channel is buffered-1 with non-blocking sends: a burst of commits
   coalesces to one signal, and a writer never blocks on a slow consumer.
3. It fires after commits that may have re-landed work in PENDING:
   enqueue, fail-with-retry (harmless extra probe on the dead-letter
   path), requeue, rescue-dead, record-answer (an answered question
   clears `not_before`).
4. Consumers select wake against their existing timer, ctx, and stop:
   the worker's idle wait (`worker.Pool.idleWait`) and the dispatcher's
   Run loop. A wake RESETS the worker's idle ladder — the pool returns
   to full cadence; a nil Wake channel never fires (Go's nil-channel
   rule), which IS the pure-poll fallback.
5. Multi-process semantics: the signal is process-local by design. Two
   pools over the same DB in different processes do not see each other's
   wakes — they keep polling, which is exactly the fallback. A
   cross-process wake (postgres LISTEN/NOTIFY, SQLite via a sidecar
   file) is a future option, not a current need: same-host pools share
   one process today (`tq serve` + pools).

## Evidence

- Latency: `TestPoolWakePreemptsIdleLadder` (internal/worker/wake_test.go)
  pins a parked 10s gap re-claiming within 250ms CI-stable (observed
  ~10ms; design target 50ms); ladder reset pinned by
  `TestPoolWakeResetsLadder`.
- Idle IO before/after: the ladder already cut idle probes
  (`TestPoolIdleBackoff`: 8 probes in 500ms at a 10ms/80ms ladder where
  fixed cadence issues ~50; observed max gap 83.6ms). The wake seam adds
  ZERO additional idle probes — an armed pool issues no claim it would
  not otherwise make; it only stops OVER-waiting after a landing. There
  is no "after" bench delta to publish because the wake path performs
  no IO beyond the claim it saves.
- Coalescing + never-blocking: `TestNotifyCoalesces` in both backends
  (sqlitev4, postgresv4 — the latter verified live under
  TQ_TEST_POSTGRES); fire sites pinned by `TestNotifyFiresAfterEnqueue`
  / `TestNotifyFiresAfterRequeue` per backend.
- Dispatcher: `TestWakeDrainsBeforePoll` (internal/consumer/wake_test.go)
  delivers within 2s against a 10s interval via the store's own Notify.

## Consequences

- Reaction latency decouples from poll cadence everywhere the seam is
  armed (both tq pools, the dispatcher once a composition root wires
  it); poll ladders can safely stay conservative.
- Store fakes and conform suites stay unbroken: Waker is opt-in by type
  assertion at the wiring site (or direct call on the concrete store).
- Operators see the armed state: `tq agent-pool` prints one stderr line
  at startup.
