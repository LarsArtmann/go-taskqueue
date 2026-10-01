# Decision — DLQ autopsies: opt-in default + doctor guard (M12)

**Date:** 2026-10-01 · **Plan:** SUPERB queue-health restoration (M12/F46/F47)
· **Context:** P3 — the production pool ran 676 tasks / 317 dead letters with
ZERO `dlqfix:` autopsies ever minted (repair loop never enabled); M3 (Phase 0)
turned `dlq-fix = true` on in the editable pool config and the first autopsies
minted.

## Question

Should `agent-pool --dlq-fix` default to ON so P3 cannot recur?

## Options weighed

| Option | Pro | Con |
| --- | --- | --- |
| **(a) default ON** | The P3 silence is structurally impossible | Autopsies are PAID agent turns; defaulting a money-spending second opinion on for every operator is an economic decision smuggled into a flag default; a dead-storm would mint N autopsies (bounded: one per dead task, dedup forever, budget-guarded) |
| **(b) keep opt-in, guard via doctor** | Spending stays an explicit per-deployment owner choice; the silence class is still caught | Requires the operator to run `tq doctor` (but that is already the "why is nothing happening?" reflex) |
| **(c) default ON + config kill-switch** | Same as (a) with an escape hatch | Two knobs to document; the kill-switch becomes the new silent failure mode |

## Ruling

**(b): the default stays opt-in; the recurrence guard is `tq doctor`.**

Rationale:

1. Autopsies burn paid agent sessions; opting pools into spend by default is
   an owner-level economic ruling, not a library default. (Precedent: the
   pool's other paid second opinions — `--review`, `--prioritize` — are all
   opt-in.)
2. The actual P3 failure was INVISIBILITY, not the default value: nobody saw
   317 dead letters pile up. Detection, not default-flipping, removes the
   failure mode.
3. Enforcement (landed with this memo): the `dlq-repair` check in
   `tq doctor` — dead letters present AND zero autopsies ever minted warns
   "the repair loop looks disabled: agent-pool --dlq-fix, or dlq-fix=true in
   the pool config". Pinned by `TestDoctorDLQRepair` (cmd/tq).
4. The minting contract itself (enabled means exactly ONE autopsy per dead
   AGENT task, dedup forever, never for non-agent deaths, replay-safe) is
   pinned by the dlqfix sweep suite — `TestSweeperMintsOneAutopsyPerDeadAgentTask`,
   `TestSweeperReplayDoesNotDuplicateAutopsy`,
   `TestSweeperNeverAutopsiesNonAgentDeaths` (internal/dlqfix).

## Deployment note

The go-taskqueue production pool has `dlq-fix = true` in its config source
(M3). SystemNix's pool does not yet — that deploy rides the outstanding
owner question (deploy timing under the sudo/systemctl ban) recorded in the
2026-10-01 phase reports.
