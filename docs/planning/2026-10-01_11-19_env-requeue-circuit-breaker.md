# Env-Requeue Circuit Breaker — Design Memo (M7)

**Date:** 2026-10-01 11:19 CEST · **Parent plan:**
`docs/planning/2026-10-01_04-27_SUPERB-QUEUE-HEALTH-RESTORATION.md`
(M7; feeds M8/M9) · **Ruling:** R3 = option (a) — burn attempt after
N=3 consecutive environmental requeues + exponential NotBefore escalation
+ alert fact. Owner order: execute the whole list.

## Problem (P1, from the 04-17 diagnosis)

A task whose verify gate dies pre-existing/environmental (gate-dead,
gate-slow) or whose preflight refuses (dirty tree) requeues WITHOUT
burning an attempt. Without a bound, one task churns claims forever:
CV task `000001a0eebb…` logged 164 claims in ~6.5 h (~1 paid agent
session per ~8 min, budget cap 30/day drained).

## Requeue path map (F26 — worker.go @ be28c964)

| Line | Class | Burns attempt? | Delay | Streak? |
| --- | --- | --- | --- | --- |
| worker.go:401-417 | `PreflightError` (dirty tree, missing autonomy) | no | `preflightDelay` ladder (base 2m ×2^n, cap 15m, ±20% jitter) | climbs |
| worker.go:426-435 | `VerifyGateError` **environmental** (vendor-gofmt signature) | no — `FailPermanent` NOW | — (dead-lettered) | n/a |
| worker.go:437-456 | `VerifyGateError` gate-dead / gate-slow | no | `preflightDelay` ladder (intended) | INTENDED to climb |
| worker.go:458-484 | `RateLimitError` (429) | no | parsed reset ±5% jitter, cap 6h | no |
| worker.go:486-516 | `QuestionPendingError` (owner park) | no | question expiry | no |
| worker.go:518-527 | `PermanentError` | dead-letter now | — | n/a |
| worker.go:529-546 | pool-shutdown cancel | burns (Fail, 0 delay) | 0 | n/a |
| worker.go:548-561 | real execution failure | burns (`Fail`, `ExpBackoff(attempts+1)`) | exponential, cap 5m | n/a |

Ladder state: `preflightSeen` map (worker.go:102-150), reset at
worker.go:363-366 for every non-PreflightError outcome.

## New finding: the gate ladder never escalates (root cause of the cadence)

The reset at worker.go:363-366 runs for ANY outcome that is not a
`PreflightError` — including the gate requeue itself. Sequence: claim →
gate fails → `preflightDelay` (count=1) → Requeue → reclaim → reset
(gate error ≠ PreflightError) → `preflightDelay` (count=1 again) → …
The gate class therefore bounces at the base rung forever: 2 m ±20% ≈
96-144 s, uncapped — exactly the ~100-130 s cadence the diagnosis
measured on `000001a0eebb…`. The ladder's cap (worker.go:116,
preflightMaxBackoff 15 m) is dead code for the gate class. The breaker
below fixes escalation as a side effect of moving the reset rule.

## Design (R3 option (a), N=3)

1. **Streak** = consecutive ladder-climbing requeues of ONE task:
   PreflightError + VerifyGateError(gate-dead/gate-slow). The existing
   `preflightSeen` counter is the streak; no new state.
2. **Reset rule** (replaces worker.go:363-366): forget the ladder only
   on a JUDGED outcome — Complete, Fail (real), FailPermanent, cancel.
   429 and question parks neither climb nor reset (parked, not judged;
   a 429 proves nothing about gate health). This single change restores
   ladder escalation for the gate class (bug above) and makes
   "consecutive" true.
3. **Breaker**: when the streak reaches N=3 on a climbing requeue, call
   `store.Fail` instead of `store.Requeue` — the attempt burns,
   NotBefore = `ExpBackoff(attempts+1)` (exponential escalation, R3),
   and the reason text carries a distinct `env-streak` code plus the
   streak count so the fact is greppable in the journal. After the
   burn, reset the ladder (the streak was consumed into an attempt).
   With default max-attempts 3: ≤ 2 free requeues between every pair of
   burned attempts, ≤ 9 claims total worst case, then DLQ — P1 dies as
   a class. Tasks whose gate heals mid-streak simply pass (reset on
   Complete) — no behavior change for healthy tasks.
4. **Alert fact** (R3): the burned attempt IS the alert — a
   `task.failed` fact carrying the `env-streak` reason code, durable in
   the journal. PapDashboard notification rides the existing
   dead-letter path (bridge alert.triggered); the sustained-churn case
   is additionally covered by the dead-pool detector (cmd/tq/main.go
   NotifyDeadPool) and, from M10, the claims-per-task anomaly check.
   A new mid-ladder push channel is deliberately NOT built: the worker
   module must not grow a bridge dependency.
5. **Counter persistence** (F27 ruling): in-memory, as today. A pool
   restart forgets streaks (amnesia bounded: N-1 extra free requeues
   per restart, still capped by attempts). Durable escalation comes
   from attempts/not_before in the store, not the counter. M9's
   `requeue_class` fact field makes historical streaks derivable from
   the journal for forensics — observability input, not breaker input.
6. **Config**: `Config.EnvRequeueBurn` (int, default 3 when unset; `< 0`
   disables the breaker, restoring pure requeue behavior — escape hatch
   for a deliberate long-hold). Named after the ruling vocabulary
   ("environmental requeue"), not the mechanism.

## Test plan (M8, F28-F33)

- Worker test: 3 consecutive gate requeues → `Fail` called (attempt
  burned), reason carries the code, ladder reset; preflight class same.
- Streak reset: Complete/Fail/permanent/cancel reset; 429 and question
  parks do not climb and do not reset (table test).
- Escalation: gate requeue #2 delay > #1 (ladder alive after the fix).
- Disposition: `EnvRequeueBurn: 0` never burns.
- Module gates: worker + executor + root `-race` + cmd/tq shim
  (F33), commit with footer (F34).
