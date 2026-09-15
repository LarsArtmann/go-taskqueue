# Post-Deploy Rate-Limit Retro (M124, 13:29 report f50) — 2026-09-15 02-30

Interim post-deploy retro requested by the flip runbook
(`docs/planning/2026-09-11_14-45_FLIP-CHECKLIST-AND-DLQ-TRIAGE-RUNBOOK.md` §5)
and TODO item "Post-deploy retro" (M124 / 13:29 report f50). Method follows the
runbook: count `task.requeued` facts carrying rate-limit evidence vs 429-class
dead-letters, pre vs post deploy.

**Window note:** the runbook scheduled the retro for 2026-09-18 (flip + 7
days); this report runs at day +4 (2026-09-15). The headline result is
window-length-insensitive: zero 429 events of any class have occurred since
the flip, so the 7-day re-check can only add idle days, not change the
comparison.

## Method

Production journal `/mnt/pool/services/tq/tq.db` read via `tq facts -json`
(build at HEAD, 2026-09-15) and classified offline. A failed attempt is
429-class when its error tail matches provider-429 evidence (`too many
requests`, `rate limit reached`, `usage limit`, `insufficient_quota`, `429`).
A dead-letter is 429-class when ANY of its task's attempts is 429-class.
Flip boundary: 2026-09-12T00:00+02:00 (flip v1 landed 2026-09-11 evening,
flip v2 GOEXPERIMENT env same day; last pre-flip 429 attempt 2026-09-11
12:10:35, task `000001a08edfbc90`).

## Numbers

| Metric                                  | Pre-flip (09-10 → 09-11)     | Post-flip (09-12 → 09-15) |
| --------------------------------------- | ---------------------------- | ------------------------- |
| 429-class failed attempts               | 28                           | **0**                     |
| 429-class dead-letters                  | 8 (of 21 dead total)         | **0** (of 52 dead total)  |
| attempts burned by 429-class dead tasks | 24                           | **0**                     |
| `task.requeued` via the rate-limit path | n/a (mechanism not deployed) | **0**                     |

Baseline check: the runbook's §4 snapshot (2026-09-11 14:40) counted 27 dead
total, 6 of them 429-class; this retro's journal-derived count is 21 dead / 8
429-class for the full two pre-flip days. The delta is snapshot timing and a
slightly wider evidence regex (includes `insufficient_quota` and `429`
status matches), not a contradiction.

## Reading

1. **Zero 429 requeues post-flip — because zero 429s occurred.** The
   requeue-without-attempt-burn mechanism (DetectRateLimit →
   `task.requeued` with `retry_in_ms`) has NOT been exercised by production
   traffic in the observation window; no `task.requeued` fact carries
   rate-limit evidence. The headline win is therefore "no 429 dead-letters
   since deploy" (was 8 in two days pre-flip), not "requeues are working".
   The mechanism remains armed but production-unproven; the first real 429
   window should be watched for `retry_in_ms` requeues with zero attempt
   increment (runbook §4 step 3).
2. **Post-flip deaths are all non-429**: verify failures, permanent-class
   errors, exhausted retries on real failures. One post-flip dead task
   (`000001a09cefead4`, 09-14) has "rate limit" in its evidence tail —
   incidental prompt text (the task was implementing a rate limiter), the
   death cause was a `go test` verify failure. Excluded from the 429 class.
3. **DLQ hygiene**: the 8 pre-flip 429-class dead tasks remain in the DLQ
   surface for the owner's rescue-or-cancel pass (runbook §4 table); the
   retro does not touch them.

## Left open

1. 7-day checkpoint 2026-09-18: re-run this classification; expect zero
   additional 429 events; first genuine 429 window still owed as the
   mechanism's production proof.
2. Owner: DLQ triage of the 8 pre-flip 429-class dead tasks (rescue or
   cancel per runbook §4).
