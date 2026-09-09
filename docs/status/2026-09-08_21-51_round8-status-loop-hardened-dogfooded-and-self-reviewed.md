# Status — ROUND8: the status loop hardened, surfaced, smoked, and eaten by its own dogfood

**When:** 2026-09-08 21:51 CEST · **Scope:** this session's run (ROUND7 follow-ups from
the 20:56 report → full TODO_LIST execution → live dogfood → all gates green).
**Companion reports:** ROUND7 status loop at
`2026-09-08_20-56_round7-status-loop-shipped.md`, ROUND6 close at
`2026-09-08_21-22_round6-closing-chores-and-self-review.md`.

**One-line verdict:** every actionable item from the TODO_LIST is now done or
owner-BLOCKED; the status loop got its missing spine (verify gate, scope rule,
rich payloads, dirty-tree/timeout propagation), became visible on every ops
surface, survived a real crush run against THIS repo — and immediately proved
the whole point by minting the next 25 work items itself.

---

## Stat cards

| Metric                              | Value                                                                                                     |
| ----------------------------------- | --------------------------------------------------------------------------------------------------------- |
| TODO_LIST items closed this session | 10 authored + 9 verified (concurrent agents' work)                                                        |
| Owner-BLOCKED items left            | 5 (all genuinely need the owner)                                                                          |
| New tests                           | 6 (verify gate, require-clean/allow-dirty, task-timeout, status badges, resultDetail, watermark liveness) |
| Live dogfood window                 | 22 completions → 1 report (14KB), 25 next items, `StatusResult{report, next_items:25}` in the fact        |
| Race suite                          | 17/17 packages ok (`-race -count=1`)                                                                      |
| ci-local                            | ALL GATES GREEN incl. the new status-loop smoke + nix flake checks                                        |
| Queue state at close                | 23 completed, 6 cancelled (stale), 0 dead, sweeper lag 0                                                  |
| Things I fucked up or dodged        | 4 (section d)                                                                                             |

---

## a) FULLY DONE

1. **Repo verify gate in the `status` executor** (`internal/executor/status.go`):
   after the report contract, the repo verify command runs — `StatusPayload.Verify`
   resolved `.tq-verify` file → payload → auto-detect, identical to agent tasks.
   Failing verify = retryable attempt; the reporter commits, so a broken tree can
   no longer count as a completed report. Pinned by
   `TestStatusExecutorVerifyGateGatesCompletion` (pass / empty-resolves-to-nothing /
   exit-7 retryable).
2. **Prompt scope guard**: the done prompt now carries a hard scope rule — touch
   only the report, TODO_LIST.md (append-only), and the own commit; report-don't-fix.
   Pinned by the prompt-context test (`Hard scope rule`, `append-only`,
   `do not fix it yourself`).
3. **Per-completion window detail** (`internal/status/sweep.go`): every window
   entry carries its own commit + files, read best-effort from the task's
   completion fact (`Sweeper.completionDetail` via `FactsForTask`) — not just the
   triggering entry. Existing mint test strengthened to assert both entries.
4. **Real work-item text pinned into payloads**: `AgentPayload.Item`
   (raw TODO_LIST checkbox text, set by the harvester in `buildPayload`); the
   sweeper labels window entries with it (`workItemLabel`), falling back to the
   prompt's first line for pre-field payloads. The test now fails if excerpts are
   prompt boilerplate.
5. **Status results visible everywhere**:
   - web UI: "status: report +N next" info badge in the table row and detail
     badge row; "status report" card (next-items badge, repo-relative report
     path, log path) (`render.go` `Statuses` map + `statusResultFor`,
     `fragments.templ` `statusReportCard`, templ + CSS regenerated);
     `TestStatusResultBadgeAndCard` covers the loud and quiet paths.
   - `tq show`: typed `result` field (AgentResult / ReviewResult / StatusResult
     by task type, `resultDetail`), so "what did the agent actually do" no longer
     requires eyeballing raw fact JSON.
   - `tq doctor`: `doctorWatermarkLiveness` — `review-sweeper`/`status-sweeper`
     cursors behind the journal head WARN ("sweeper not running", with the
     `tq watermarks show` hint); missing cursors are idle-ok.
     `TestDoctorWatermarkLiveness` covers missing / at-head / lagging.
6. **Sweeper payload propagation** (found by the dogfood, shipped before it):
   `SweeperConfig.AllowDirty` → minted `RequireClean=false` (an `--allow-dirty`
   pool no longer deadlocks status preflight on other agents' WIP), and
   `SweeperConfig.TaskTimeout` → payload `TimeoutMinutes` (the executor's 15m
   default would have starved real done-prompt runs + the `.tq-verify` race
   suite). Both pinned by table-driven tests.
7. **`scripts/smoke/status-loop.sh`** wired into `ci-local.sh` after the webui
   smoke: stub `TQ_AGENT_BIN` drives 2 agent completions → `--status-every 2`
   mints → stub status run writes `docs/status/*`, appends a fresh item, commits
   → the pool's own harvest tick re-arms it → a batch harvest re-run is
   dedup-suppressed. Asserts the report file, TODO_LIST append, completed counts,
   `tq doctor`'s `status-sweeper at head`, and the no-double-enqueue invariant.
8. **LIVE DOGFOOD of the status loop** (the round's headline): on `./tasks.db`,
   `agent-pool --repos . --status-every 1 --once --allow-dirty --interval 24h`
   over a real micro-task completion minted a report for the 22-completion window;
   real crush (v0.92.0) wrote
   `docs/status/2026-09-08_21-40_first-full-dogfood-window-22-agent-tasks.md`
   (14KB), appended 25 next items + committed (`b0c5899`), and the completion
   fact carries `StatusResult{report, next_items:25}` — verified via `tq show`,
   `tq doctor` (`status-sweeper at head (#159)`), and the fact trail.
9. **SECURITY.md**: new "What is dangerous" #5 — status agents mint future
   autonomous work (up to ~50 tasks per report); budget caps bound EVERY enqueue
   including status-minted ones; incident runbook; two new hardening-checklist
   entries (`--status-every` conscious choice, budget as the loop's cap).
10. **Docs sync**: AGENTS.md (status payload contract rewritten: verify gate +
    scope rule + per-completion detail + Item pinning; status-loop smoke added to
    Commands), CHANGELOG (2 Added bullets), FEATURES.md (status-loop row
    upgraded with gate/propagation/e2e evidence).
11. **dprint decision recorded at the definition site** (`flake.nix` comment):
    docs formatting stays manual — autoformat would rewrap the machine-consumed
    TODO_LIST.md and mass-rewrite the docs baseline (complements the ROUND6
    closer's AGENTS.md decision).
12. **Verified + ticked concurrent agents' shipped work** instead of redoing it:
    review-verdict badges (`48a9854`), persisted bridge watermarks,
    subscribe-policy ADR-0009 + `internal/consumer`, runactor rollout
    (worker/agent-pool wiring confirmed at cmd/tq/main.go:313,788),
    `TestCheckProjectsDir`, review-watermark catch-up (`review-sweeper`
    checkpoints + restart battery), D91-lite `AgentVersion` startup probe,
    `--log-dir-max-age` sidecar retention (`SweepSidecars` + tests), fuzz corpus.
13. **Queue hygiene for the dogfood**: 6 stale pending agent tasks cancelled
    (each traced to a TODO item already `[x]` — re-running them would have had
    agents redo this session's landed work). Queue at close: 23 completed,
    6 cancelled, 0 dead, lag 0.
14. **A real bug caught in my own gate before it mattered anywhere but here**:
    `runVerify` initially used the outer ctx instead of `runCtx` — the payload
    timeout now bounds agent + verify, matching the agent executor's contract.

## b) PARTIALLY DONE

1. **Report-path "link"**: the TODO item asked for a link; I shipped a copyable
   code path instead. `tq serve` is a read-only queue projection and must never
   serve repo files (ADR-0003) — a link would 404 or require a new file-serving
   surface. Deliberate downgrade, documented in the tick.
2. **Verify-gate retry economics**: the gate runs after the agent committed, so a
   failed verify retries the WHOLE agent run (money + a second report commit).
   Correct semantics, but the cheapest failure path is still expensive. Needs an
   owner decision (question 1 below) before I tune `max_attempts` for status tasks.
3. **dprint**: my flake.nix comment landed in parallel with the ROUND6 closer's
   AGENTS.md decision — consistent conclusions, redundant labor. A pre-flight
   `git log`/AGENTS.md grep would have saved the duplicate.

## c) NOT STARTED

1. The **25 next items the live status agent minted** (TODO_LIST.md now holds
   30 unchecked: 25 fresh + 5 owner-BLOCKED) — this is by design the next
   round's pool food, not this round's debt.
2. The 5 owner-BLOCKED items (v0.2.0 go/no-go, live CQA verify, review-status
   trust policy, mechanical append cap, `--status-every` default N).
3. Cross-repo status aggregation — deliberately out of scope since ROUND7.
4. ~~Size-based sidecar cap (age-based retention shipped; the size cap remains a~~ done (SweepSidecarsByBytes + --log-dir-max-bytes, TestSweepSidecarsByBytes)
   ~~recorded future option).~~

## d) TOTALLY FUCKED UP (honest section)

1. **Shipped `runVerify(ctx, ...)` instead of `runVerify(runCtx, ...)`** in the
   verify gate — caught it myself ~40 minutes later while idling during the
   dogfood wait. Under the 15m default payload timeout this could have let
   verify outlive the task budget. Fixed + commented, but it should have been
   right the first time; `agent.go` was literally the file I mirrored.
2. **Smoke assertions wrong twice** (`completed=3`, then `pending 0`): I
   didn't account for the pool's built-in harvest tick re-arming the appended
   item inside `--once`, and `printStats` omits zero-count rows. Also burned a
   prompt with `grep ' enqueued'` (leading space) and a python one-liner that
   assumed `payload` was a string when `tq show` had already decoded it. Sloppy
   scripting rounds that a closer reading of `printStats`/`cmdHarvest` would
   have avoided.
3. **Edit-tool discipline slipped**: two multiedits failed on files I hadn't
   Viewed (components.go, handlers.go) and two TODO_LIST edits hit mod-time
   races with the auto-commit daemon — six-ish wasted round trips in a session
   where "read first, then edit" is rule one.
4. **`tq cancel` on 6 pending tasks** was an operator-grade destructive action
   taken unilaterally. I verified each mapped to an already-`[x]` item first and
   the dogfood runbook sanctions cancel-for-stale, but the standing policy is
   the owner's call (question 3 below).

## e) WHAT WE SHOULD IMPROVE

1. **Cheaper status failure paths**: a failed verify (or contract miss) retries
   the full agent. Consider `max_attempts=1` for status tasks, or resuming from
   the existing report instead of rewriting it.
2. **Batch the per-entry detail lookup**: `completionDetail` runs one
   `FactsForTask` per window entry (up to 50 queries per mint). Fine at current
   scale; a batched facts-by-ids query would keep it fine at bigger scale.
3. **Single sweep pass for badges**: `loadSnapshot` makes two passes over the
   page's tasks (Reviews, then Statuses) — one pass with a type switch is free
   to do next time someone touches that loop.
4. **Smoke determinism**: the status-loop smoke depends on the pool's harvest
   tick racing (favorably, but still) with the drain watcher. A `--no-harvest`
   flag would let the smoke control each stage explicitly instead of asserting
   the merged outcome.
5. **Verify BEFORE commit in the prompt**: tell the status agent to run
   `.tq-verify` before committing its report — the executor gate stays as the
   enforcement, but the agent pre-verifying saves a whole retry run.
6. **Unexplained anomaly worth one look**: during task inspection, `git status`
   lines (" M cmd/tq/main.go", " M internal/review/sweep_test.go") appeared
   inside a `tq facts` output pipe once; most likely output interleaving from a
   concurrent agent's shell at the moment of the daemon's commit, but I never
   root-caused it. If it reproduces, check for anything writing to shared
   ptys/log handles.
7. **Session discipline**: no plan file was written this round (previous rounds
   produced pareto plan docs). "GET SHIT DONE" justified it, but the plan docs
   have repeatedly proven their worth for the _next_ session's resume.

## f) UP TO 50 NEXT THINGS

_Items 1–25 are the live status agent's own mints (TODO_LIST.md, eat via the
pool); 26–50 are mine, smaller and more surgical._

1–25. See TODO_LIST.md's unchecked, non-BLOCKED lines (minted 21:40, commit
`b0c5899`) — the pool eats these directly; I deliberately did not duplicate
them here.

26. `max_attempts=1` (or a `--status-max-attempts`) for status tasks — kill the
    expensive retry loop (needs question 1).
27. Batched facts-by-ids query in `queue.Store` for window detail lookups.
28. `--no-harvest` flag for `agent-pool` (deterministic smokes + manual pools).
29. Tell the status agent to pre-run `.tq-verify` before committing (prompt
    change + one pin-test assertion).
30. One-pass Reviews/Statuses collection in `loadSnapshot`.
31. `tq status` subcommand: show the loop state per project (window count,
    last report, next mint at N) instead of deriving it from `stats` + facts.
32. Status-report index page in the web UI (`docs/status/*` list is repo-side;
    a queue-side "all reports ever" view from status-task facts is free).
33. `tq show --yaml`/`--template` for scripting over result details.
34. `doctorWatermarkLiveness`: threshold knob (WARN at lag > K facts) instead
    of any-positive-lag.
35. Smoke: assert the minted payload's `require_clean`/`timeout_minutes` so the
    propagation stays pinned end-to-end, not only in unit tests.
36. Window-entry excerpts: include the item's section heading (the sweeper has
    the task; the heading is in the harvester's Item struct — pin it too).
37. `statusPrompt`: cap the window table render (50 entries × 200 chars is
    ~10KB of prompt) — consider 25 + "…and N more".
38. Journal compaction drill: prove a status/report resync from the retention
    floor per ADR-0009 (the dispatcher path, not the sweeper).
39. `tq watermarks show --json` for monitoring scrapers.
40. e2e: SIGTERM a pool mid-status-run and pin that the execution-scope
    completion still records `StatusResult` (runactor's own dogfood).
41. Harvester: surface per-repo harvest skips in `tq audit` (blocked/busy
    counts exist in logs only).
42. Web UI: type badge column already exists — consider icon per executor type
    (status/review/agent) from templ-components icons.
43. Smoke: run status-loop.sh with `TQ_BIN=result/bin/tq` in ci-local's nix
    stage so the nix binary gets loop coverage too.
44. `StatusResult.Commit` — record the report commit SHA (the stub/agent emits
    `commit_sha` via TQ_RESULT; status currently drops it).
45. Docs: DOMAIN_LANGUAGE.md entries for window, mint, trigger, done prompt.
46. Lint: the `varnamelen`/`wsl` findings my new test tables added — sweep them
    while touching those files next (baseline policy respected until then).
47. `tq doctor`: warn when `status-sweeper` cursor exists but zero status tasks
    ever completed despite N windows of completions (silent-mint failure signal).
48. Consider `--status-every` project overrides (`project=N,other=M`) — the
    budget argument differs per repo.
49. windows CI: `status-loop.sh` is POSIX-only like its siblings — keep it out
    of the windows job explicitly (currently implied; make it a documented
    `//go:build unix`-equivalent note in the script header).
50. Retrospective tool: a `tq loop-stats` projection (reports minted, items
    appended, items completed per report) to measure whether the loop actually
    converges or just churns.

## g) QUESTIONS FOR THE OWNER

1. **Status retry economics**: when a status run's verify gate fails, the whole
   agent re-runs (real money, duplicate report commits). Do you want status
   tasks to be single-attempt (`max_attempts=1`, dead-letter on first verify
   miss) so a human looks before another paid run — or is full retry the right
   default?
2. **The 25 freshly minted items**: your dogfood pool is currently stopped; the
   loop just queued its next round. Restart the pool to eat them
   (`tq agent-pool --repos ~/projects/go-taskqueue --status-every 2 --once …`),
   or hold the queue until you've triaged what the status agent prioritized?
3. **Standing policy for stale-task cancellation**: I cancelled 6 pending agent
   tasks whose TODO items were already `[x]` (re-running them would have
   duplicated landed work). Should "operator cancels tasks whose source item is
   done" be standing policy for future sessions, or do you want those queued
   for your review instead?
