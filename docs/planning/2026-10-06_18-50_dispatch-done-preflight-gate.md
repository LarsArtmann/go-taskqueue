# Dispatch-time done-preflight gate — spec (2026-10-06 18:50 CEST)

**Problem (journal-proven, SystemNix pool, task `000001a0f97e2c06e0f314b5991900000000`):**
the "re-fire loop" class is NOT re-harvesting. Facts for that task: `task.enqueued`
**exactly once** (2026-10-02 00:03), then **15 × task.claimed / 13 × task.requeued**
(rate-limit, gate-dead, gate-slow classes) before completion. Every claim re-ran a
full paid agent session on work that had already landed under the task's own
`Task-Queue-ID` footer (21 footer commits at re-fire 10). The worker requeues without
burning attempts — correctly — but nothing re-checks DONENESS at claim time, so the
same done task is re-dispatched forever. A second shape: **fix tickets**
(`reviewfix:` dedup) re-fire after their finding is cured because a Task-ID footer
filter alone passes them through — the rejected commit is cited by SHA only inside
the prompt text (SystemNix `docs/status/2026-10-02_07-35_…fada….md` §e.1; the
`4eb7fc12` fix landed 06:24, the ticket re-fired 07:3x).

Existing coverage and its gap: `harvest --prune-stale` already withdraws pending
tasks whose item is `[x]`/absent — but it runs ONCE at pool start. The re-fire
class happens mid-run, between ticks, exactly where prune cannot see it. Prune also
covers only harvest-keyed items: fix tickets have NO dedup key and no item row.

## Design

A claim-time gate, structurally identical to the existing Budget hook
(`worker.Config.Budget`): before a claimed task's executor runs, ask whether the
task's work is provably already done; if yes, complete it with ZERO agent spend.

### Hook contract (worker module, generic)

```go
// DonePreflight is the claim-time done gate: before a claimed task's
// executor runs, the hook decides whether the work is already done. A done
// task completes immediately with a preflight-done result detail (no
// attempt burn, no agent session, no review minted). Nil = ungated.
DonePreflight func(ctx context.Context, t task.Task) (done bool, reason string)
```

Placement in `Pool.execute`: directly after the Budget gate, before the heartbeat
goroutine starts. On `done`: `store.Complete(ctx, id, claim, detail)` where detail
is an `executor.AgentResult` JSON with the new field
`PreflightDone string` (`json:"preflight_done,omitempty"`) carrying the reason;
`log.Warn("done preflight: completed without agent run", …)`. Preflight-done
completions must NOT mint reviews: `review.enqueueReview` skips completions whose
`AgentResult.PreflightDone != ""` (a review of a never-run session is another no-op
agent burn — the exact class this gate kills).

### Done signals (harvest module, repo-aware; any ONE ⇒ done)

Implemented as `(*Harvester).DonePreflight(ctx, t task.Task) (bool, string)` in
`internal/harvest`, wired by `cmd/tq agent-pool` behind `--done-preflight`
(pool.conf key `done-preflight = true`). Foreign payload shapes (no Repo/Prompt)
⇒ not done — the gate never guesses.

Ordered, strongest first; the FIRST hit is the recorded reason:

1. **Footer commit exists** (all agent + fix tasks):
   `executor.GitLogScanner{}.CommitsByTrailer(ctx, repoDir, executor.TaskTrailer, t.ID)`
   returns ≥1 commit ⇒ done (`"footer commit(s) exist under Task-Queue-ID"`).
   This is the signal the 21-commit re-fire class trips on day one.
2. **Fix-ticket rejected-SHA disposition** (payload gains structured fields):
   `mintFixes` sets `AgentPayload.RejectedSHA` (finding.CommitSHA ‖ reviewPayload.CommitSHA)
   and `AgentPayload.Anchor` (finding.Anchor). Then:
   - SHA exists (`git cat-file -e`) AND some commit on any ref cites the SHA
     (`git log --all --format=%H --grep <sha>`, the superseding-fix pattern) ⇒ done.
   - SHA gone (rebased away) AND the anchor text has zero hits in the current
     tree (`git grep -F -- anchor`, working tree) ⇒ done (`"finding cured: anchor gone"`).
   - otherwise ⇒ not done (the finding may still be live).
3. **Harvest item ticked/absent** (tasks carrying `todo:` dedup keys — the
   prune-stale rules applied per claim): re-parse the repo's todo file; exact-key
   item now `[x]` ⇒ done; key absent from present-and-done keys ⇒ done
   (`"item withdrawn or reworded"`). Batch tasks: all members ticked/absent ⇒ done
   (same all-stale rule as prune).
4. **Status report exists**: `docs/status/*_task-<id>*` present in the repo ⇒ done
   (the closeout-report convention both dogfood repos use; absent dir ⇒ signal absent).

Safety valve for the 2026-09-25 counter-evidence (the 5th re-fire caught un-harvested
§f follow-ups behind a closed `[x]`): the SystemNix-side mitigation is already
landed (AGENTS.md self-harvest-at-authoring rule, TODO_LIST:185). The gate does not
re-litigate it; the completion reason names WHICH signal fired so an operator can
audit skips (`tq facts` / dashboard).

### Prompt-side gate (cheap second belt)

The agent prompt (harvest + fixPrompt) gains one line: "BEFORE doing any work: run
`git log --format=%B --grep 'Task-Queue-ID: <your id>'` and check the item's
checkbox in the todo file; if either shows the work already done, complete as a
no-op closeout stating the evidence instead of redoing." Catches the window between
enqueue and pool redeploy.

## Telemetry (spec extension)

- Completion detail `preflight_done` surfaces in `tq show`/dashboard via the
  existing AgentResult fold — no new fact type needed.
- Per-ticket re-fire counts: `task.claimed` facts per task ID are countable in the
  readmodel; `tq show <id>` should display "claims: N" (readmodel addition,
  separate slice).
- `task.reprioritized` journal fact: stays its own queued row (upstream
  SystemNix TODO_LIST row; orthogonal to this gate).

## Tests (m12 selftest)

- harvest: fixture git repo + todo file — (a) footer commit ⇒ done; (b) ticked
  item ⇒ done; (c) absent item ⇒ done; (d) open item + no commits ⇒ NOT done;
  (e) fix payload rejected-SHA superseded ⇒ done; (f) rejected-SHA gone + anchor
  zero-hit ⇒ done; (g) rejected-SHA present + anchor live ⇒ NOT done;
  (h) foreign payload ⇒ NOT done.
- worker: hook says done ⇒ task completed, executor NEVER invoked (counting
  executor), zero attempt burn.
- review: completion with `preflight_done` set ⇒ no review task minted.

## Rollout

go-taskqueue lands the gate default-OFF (`--done-preflight`); SystemNix
`tq-agent-pool.nix` poolSettings sets `"done-preflight" = "true"` in the same
wave as the input bump (owner push + deploy per the standing chains).
