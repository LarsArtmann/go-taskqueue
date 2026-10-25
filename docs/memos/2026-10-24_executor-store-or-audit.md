# Executor-state-over-store audit (WorktreeInUse pattern)

Source: TODO row "Audit other consumers that OR executor state over store
state (WorktreeInUse pattern)" (closeout 2026-10-13 04-11 §e1/§f2).
Date: 2026-10-24 03:17. Scope: every site that combines in-memory
executor/pool state with store task status in a liveness or resume
decision — the shape that gave the worktree reaper its cancelled-task pin
(00-11 report e5).

Method: grep for `WorktreeInUse`, `closeoutPending`, per-task in-memory
maps (`sync.Map`, `map[task.ID]`, `map[string]`) across cmd/tq,
internal/executor, internal/worker, internal/bridge, plus the resume
paths (answer poller, closeout resume, rate-limit gates).

## Verdicts

| # | Site                                                                                                                  | Pattern                                                                           | Verdict                                                                                                                                                                                                                                                                                   |
| - | --------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1 | `cmd/tq/agentpool.go` `worktreeInFlight` (reaper probe, wired at cmd/tq/main.go:1592)                                 | store status OR `agentExec.WorktreeInUse(id)`                                     | FIXED — terminal status wins (CANCELLED/DEAD never consult the executor branch); pinned by `TestWorktreeInFlightTerminalStatusWins` (store side) and `TestWorktreeInFlightCancelledParkedWithExecutor` (exact cancelled+parked shape via `executor.ParkCloseoutForTest`, commit 20fc83ab) |
| 2 | `internal/queue/companion/questions.go` `RecordAnswer` (AnswerPoller resume, internal/bridge/papdashboard/answers.go) | answer record + store status read BEFORE `unblockParkedTask`                      | SAFE — store wins in the correct direction: only `status == "pending"` unblocks; a cancelled/dead parked task just gets the answer recorded (idempotent fact), never resumed                                                                                                              |
| 3 | `internal/executor/agent.go` closeoutPending resume (pop at :783, arm at :882/:950)                                   | executor map read on re-claim                                                     | SAFE — the resume is reachable only through a fresh claim; terminal tasks never claim, so a stale entry can only linger (two strings, in-memory, documented lifetime audit at closeoutPending) — it cannot pin a worktree or resurrect a task                                             |
| 4 | `AgentExecutor.rateLimitUntil` / `rateLimitGates` (internal/executor/ratelimit.go)                                    | executor gate consulted before claim/execute                                      | NOT APPLICABLE — keys are pool-wide or per-repo, never per-task; a terminal task owns no resources these gates can pin                                                                                                                                                                    |
| 5 | `cmd/tq/doctor.go` `doctorWorktreeOrphans` (worktree-orphans check)                                                   | store-only in-flight probe (PENDING+RUNNING), then `executor.ScanWorktreeOrphans` | SAFE (inverse gap only) — no executor OR, so it cannot pin anything; for a cancelled+parked task it flags the worktree as an orphan, which is the desired report (the worktree IS reclaimable). The reaping path never goes through doctor — it rides the fixed worktreeInFlight          |
| 6 | review/dlqfix/status/prioritize second-opinion executors (`WithoutCloseout`)                                          | closeout-free clones                                                              | NOT APPLICABLE — `secondOpinion` clones never arm closeoutPending, so they carry no OR surface at all                                                                                                                                                                                     |

## Conclusion

The WorktreeInUse call site was the ONLY consumer that OR'd executor
state over store state for a liveness/resume decision; it is fixed and
double-pinned. All other executor/store combination points either read
the store first (RecordAnswer), are reachable only through a claim
(closeout resume), or are not task-keyed (rate gates, doctor probe).
New callers of `AgentExecutor.WorktreeInUse` or new executor-state maps
keyed by task.ID must re-run this audit — the regression class is:
"executor still holds the task's resources, executor consulted after
store says terminal, resources never reaped."

Guard note: no mechanical check pins this today (a lint rule for
"executor-state read after terminal store status" is grep-shaped and
not worth it at one real call site). If a second OR-pattern caller
appears, revisit.
