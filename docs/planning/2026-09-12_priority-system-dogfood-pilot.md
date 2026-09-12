# Priority-System Dogfood Pilot — SystemNix Pool Proposal

Status: PROPOSAL (owner-run steps only; every flag below is shipped on
master behind defaults that change nothing until set)

Plan: `docs/planning/2026-09-12_14-19_PRIORITY-SYSTEM-SUPERB-PARETO-EXECUTION-PLAN.md` (T40)
Spec: ADR-0015 (`docs/adr/0015-priority-bands-importance-aging.md`)
Deployment surface: SystemNix `services.tq-agent-pool` (binary pinned by
the SystemNix input; the input flip + `nix run .#deploy` are owner-run by
design — AGENTS.md "Pool-deploy failure mode")

## Goal

Prove the ADR-0015 priority system on the PRODUCTION dogfood pool
(GLM-5.3-Flash agents over go-taskqueue, SystemNix, CV, working repos)
with a flag set chosen so the pilot is observable, cheap, and instantly
reversible. No schema migration, no new binary surface: everything below
is command-line flags on the existing pool unit.

## Proposed flag set

| Flag                         | Value                          | Why                                                                                                                                                                                                                                                                |
| ---------------------------- | ------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| (aging)                      | unconditional — nothing to set | Aging is a claim-time query term (ADR-0015 §4), always on since T09; the pilot only OBSERVES it (claim-order flips as items age past thresholds).                                                                                                                  |
| `--priority-from importance` | enable                         | Resolves backlog priorities from each repo's `.config/metadata.yaml` importance (0-100, default 50) through the one resolution ladder (marker > AI > keyword > importance); `importance: 0` pauses a repo's auto-admission entirely (T35, tested both directions). |
| `--max-pending-per-repo 4`   | working-set cap                | Replaces the legacy any-pending-is-busy rule: a repo may hold up to 4 PENDING tasks (queue = working set, TODO_LIST.md = warehouse) while one agent per repo still executes at a time. 3-5 is the sane band; 4 gives the pilot signal without flooding.            |
| `--starvation-after 24h`     | alarm only                     | The oldest PENDING task past 24h despite aging fires ONE PapDashboard trigger per episode (`agent-pool-starvation`, direct-notify pattern) plus a WARN log even without `--alert-url`. Pure observability; frees the owner from watching claim order.              |
| `--prioritize`               | NOT yet (see gate)             | The AI batch scorer. Shipped, default OFF, budget-gated like every mint. Enabling spends real API money; gate below.                                                                                                                                               |

Existing stays as-is: `--reprioritize` on (startup re-resolution — already
the deployed default), `--dead-pool-ticks 3` (shipped 2026-09-11),
`--dlq-fix`/`--review-autofix` per current unit config.

## The --prioritize gate (T28 cost measurement)

Before `--prioritize` goes live, run ONE batch scorer task against a
SCRATCH journal (not the production DB — the TQ_DB env trap, AGENTS.md):

1. Copy 10-20 real open TODO items from one repo into a scratch repo +
   scratch journal.
2. Enable `--prioritize` on a one-shot `tq agent-pool --once` against the
   scratch DB.
3. Record: wall time, model, tokens (usage parsing lands via the TODO
   follow-up; until then read the crush log), verdict quality spot-check.

Go/no-go rule (owner): if one batch costs more than ~one small agent work
turn, keep `--prioritize` off and rely on importance + markers; otherwise
enable on the pool with the same budget cap as every other mint.

## Expected behavior to watch (first 48h)

- Startup log: `startup reprioritize` lines — importance resolutions land
  as ONE `task.reprioritized` fact each (source `importance`), hot/machine
  tasks untouched (band protection), same-value resolutions write nothing.
- `tq tasks` / webui: priorities spread across the backlog band
  (0-99) instead of the flat default; `?band=` filter shows the split.
- Claim order: repos with higher importance drain first; a repo's items
  age up (+1 per 3 days, capped +10 — ADR-0015 §4 constants).
- Admissions: `tq harvest` skip reasons may show
  `paused: importance 0` for repos explicitly zeroed, and
  `admission: repo holds N pending task(s), cap 4` once a busy repo hits
  the working-set cap.
- Starvation: at most one alert per 24h+ episode; back under threshold
  auto-resolves.
- Rollbacks: each flag is independently removable; removing
  `--priority-from importance` reverts NEW resolutions to flat + markers
  (already-applied repri facts stay — they are journal history; a fresh
  `tq reprioritize` pass re-resolves everything to the current ladder).

## Owner steps (in order)

1. Land master on the deployed binary: flip the SystemNix input to the
   current master rev (needs the `internal/journal/cqrs/v0.2.0` tag pushed
   first — the module require resolves through the proxy; TODO_LIST item).
2. Add the three flags to `services.tq-agent-pool` config
   (`--priority-from importance --max-pending-per-repo 4
   --starvation-after 24h`).
3. `nix run .#deploy` (SystemNix repo), then watch `journalctl -u
   tq-agent-pool` for the first tick's repri lines.
4. After 48h: read the band mix + claim order (webui or `tq top`), decide
   the T28 spend go/no-go, then calibrate (T34: keyword table + marker
   ladder from real review verdicts × score sources).

## What the pilot deliberately does NOT do

- No effort-aware budget changes (G2 stands: budget global-flat; ADR-0015
  §10 documents the deferral).
- No `--prioritize` before the cost measurement (money).
- No marker-syntax changes (`— P1:` .. `— P4:` are stable; parser shipped
  T12).
