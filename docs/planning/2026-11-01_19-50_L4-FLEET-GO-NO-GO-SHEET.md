# L4 Fleet GO/NO-GO Sheet — fleet-wide crush-server routing

Status: DRAFT (data slots fill from ≥1 week of dogfood telemetry after
the SystemNix flip; A18). One page, one ruling: route EVERY pool's agent
turns through pool-owned `crush server` instances, or stay in-process.

Owner: Lars. Prepared by: the adoption ladder (SUPERB plan,
docs/planning/2026-11-01_06-08_SUPERB-CRUSH-SERVER-ADOPTION.md).

## The decision

| Question | Answer slot |
| --- | --- |
| Route all pools (L4)? | GO / NO-GO / GO-with-conditions: ______ |
| Date ruled | ______ |
| Conditions (if any) | ______ |

## Evidence slots (fill from the dogfood pool)

| # | Metric | Source | Threshold intuition | Observed |
| --- | --- | --- | --- | --- |
| 1 | Server RSS, 7-day max | `crush-server-rss.log` → `tq pool-health` (`crushServerRss.maxKB`) | < 2 GiB band, no sustained climb (leak shape) | ______ |
| 2 | `[crush-server]` requeue count vs total agent runs | `tq dlq --json` + task facts | rare; every one should have re-run cleanly after restart | ______ |
| 3 | Attempt-burn regressions traced to routing | DLQ autopsies | zero (the no-burn contract held) | ______ |
| 4 | Stall detector hits (`/agents/live` stall lines) | webui feed + pool-health | real stalls surface; no false-positive storms | ______ |
| 5 | Routed vs local run success rate (same period) | `tq tasks --json` | no routed-vs-local gap | ______ |
| 6 | Verdict/question rails: recording failures | closeout reports + `tq show` | zero "no verdict channel" refusals | ______ |
| 7 | Operator pane usage | serve logs / owner impression | the pane answered "what is it doing" at least once mid-incident | ______ |
| 8 | Server restarts (keeper ladder) | pool log `restart N` | occasional, self-healed, never orphaned | ______ |

## Known costs accepted at L1-L3 (already shipped + pinned)

- Env gap (crush#4210): server-side tools see only server env — cured by
  prompt-baked rails; the canary in `crush-server-route.sh` flips
  green-red the day upstream ships the fix (A17 ritual).
- One more supervised process per pool (the keeper); SIGTERM-proven
  teardown, RSS watched.
- `-H` without `CRUSH_CLIENT_SERVER=1` runs silently LOCAL (R4) — the
  seam always sets both; doctor pins the proven version.

## Rollback

Unset `crushServerSocket` (or the `--crush-server` flag): identical
in-process behavior, zero residue. The socket file is the only artifact.

## Ruling ritual

1. Flip the dogfood pool (V1.9 owner-run deploy), watch ONE wave (V1.10).
2. Let ≥1 week of telemetry accumulate (slots 1-8).
3. Fill the table, rule above, tick TODO row 1448 (the L4 gate row).
