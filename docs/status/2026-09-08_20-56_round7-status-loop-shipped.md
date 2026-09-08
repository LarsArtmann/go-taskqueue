# Status Report — ROUND7: The Automated DONE-PROMPT Loop Shipped

**Written:** 2026-09-08 20:56 CEST (interactive session, not a status task)
**Session scope:** harvest explanation → todo-list-ai relation → status-loop design →
full implementation → gates → push. No unrelated research.

---

## a) FULLY DONE

1. **Explained `tq harvest` end-to-end** (parser, dedup keys, pacing rules, loop
   closure) and mapped the pipeline position of `todo-list-ai` (TS) /
   `todo-list-ai-go` (Go): both are TODO_LIST.md **producers**; go-taskqueue is
   the **consumer/executor**.
2. **Designed the automated done-prompt loop** — pareto plan with mermaid graph at
   `docs/planning/2026-09-08_20-40_SUPERB-PLAN-ROUND7-STATUS-LOOP-AUTOMATED-DONE-PROMPT.md`
   (1%→51% executor, 4%→64% sweeper, 20%→80% wiring/tests, rest docs/gates).
3. **`internal/executor/status.go`** — `StatusExecutor` (type `status`): decodes
   `StatusPayload` (repo, project, completed window), runs the done-prompt agent
   via the shared agent mechanics (clean-tree preflight, timeout default 15m,
   yolo mirror), enforces the mechanical contract: output ends with
   `TQ_RESULT: {"report":"...","next_items":N}` naming an **existing,
   repo-relative** file (`filepath.IsLocal` confinement; absolute/`..` refused;
   misses retryable, payload misses permanent). Result detail = `StatusResult`.
4. **The DONE PROMPT itself** (headless-adapted): full status report at
   `docs/status/<YYYY-MM-DD_HH-MM_WELL-NAMED>.md` with sections a–g; "wait for
   instructions" dropped (impossible headless); **loop-closing append** to
   TODO_LIST.md — next items as `- [ ]`, questions as `— BLOCKED:` items the
   harvester already skips; commit, never push.
5. **`internal/status/sweep.go`** — the sweeper: `status-sweeper` watermark
   (clone of review's cursor semantics: head-bootstrap, checkpoint-after-page,
   pending-checkpoint retry), counts only `agent`-type completions per project
   (**structural loop guard** — status never reports on itself), N-window
   derived from queue state (completed agent tasks updated since the last status
   task's creation → restart-safe, no new persisted state), in-flight
   suppression (one report pending/running per project), window capped at 50
   entries, mints deduped by `status:<project>:<trigger-task-id>`.
6. **Wiring** — `tq agent-pool --status-every N` (0=off, config-file compatible),
   status executor registered in EVERY agent-capable pool (`agent-pool` AND
   `tq worker --agents` — carry parity with reviews), sweeps in the tick loop
   and in the `--once` drain watcher, startup banner.
7. **`executor.ResultLine`** shared helper — TQ_RESULT regex extraction deduped;
   `review.ParseResult` now delegates (behavior identical, review tests
   unchanged and green).
8. **15 new tests** — executor: parse table, happy path (stub writes report via
   cwd=repo), missing-file retryable, path-escape refusal, payload-contract
   permanence, prompt-context pin. Sweeper: below-N no-mint, Nth-completion
   mint (window order, trigger detail), replay dedup, status-never-triggers,
   in-flight suppression + re-arm after landing, bootstrap-at-head (no history
   replay, backlogged window reported on first post-start completion),
   foreign-payload skip.
9. **Docs** — AGENTS.md (package row + full `status` payload-contract bullet),
   CHANGELOG `[Unreleased]` entry, FEATURES.md row (🟢 FULLY_FUNCTIONAL).
10. **Gates** — `go build`, `go vet ./...`, full `go test ./...` (incl. `-race`
    on touched packages), **`./scripts/ci-local.sh` ALL GATES GREEN** (incl.
    GOOS=windows cross-compile, webui smoke, nix flake checks). Pushed
    `d4df3bf..9e2bf34` (push explicitly requested in the task prompt).
11. **In passing:** fixed the concurrent `runactor` package's windows
    cross-compile break (`syscall.Kill` test → `runactor_unix_test.go` with
    `//go:build unix`, repo platform-honesty convention). CI was red on
    windows without it.

## b) PARTIALLY DONE

1. **Live verification**: the status loop is stub-tested and CI-gated but has
   **never run against a real crush agent**. First live dogfood run owed.
2. **Window detail**: `Commit`/`Files` attach only to the triggering
   completion; other window entries carry task-id/time/excerpt only (the
   prompt compensates with "inspect git log"; richer detail is a small
   follow-up).
3. **Item excerpts**: for HARVESTED tasks the excerpt is the template's
   boilerplate first line ("You are an autonomous agent…") — low signal,
   known tradeoff (documented in code); custom/manual agent tasks get real
   signal.
4. **This report itself** was written by the interactive session — the loop
   hasn't eaten its own dogfood yet (meta-irony acknowledged).

## c) NOT STARTED

- End-to-end smoke script for the status loop (no `scripts/smoke/status-loop.sh`).
- Web UI surfacing of `StatusResult` (report-path link, next_items badge) —
  reviews just got theirs (commit `48a9854`, noticed in passing; its TODO_LIST
  checkbox is still open — flagging, not judging).
- `tq show` rendering + `tq doctor` liveness check for the `status-sweeper`
  watermark (review has neither for its sweeper either — shared gap).
- DOMAIN_LANGUAGE.md entries for "status window", "done prompt", "status report".
- Fuzz tests for `parseStatusResult` (sibling `result_fuzz_test.go` exists).

## d) TOTALLY FUCKED UP!

Nothing shipped broken (all gates green before push). Honest near-misses:

1. **The paste demanded "VERY DETAILED commit message(s)" — none exists.** The
   auto-commit daemon raced me and committed my work into generic
   `chore: auto-commit N changed file(s) (heuristic)` commits; I verified my
   files landed in HEAD and pushed. The detailed record lives in the plan file
   - CHANGELOG instead. (Deliberate choice: rewriting daemon history = worse
     than generic messages.)
2. Wrote an invalid stray line (`func new SweeperFactory = nil`) into the first
   test draft — caught by the compiler, fixed immediately.
3. First sweeper-test ordering created the sweeper AFTER completions and
   expected mints — misunderstanding of bootstrap-at-head; restructured to
   sweeper-first (matching review's tests) and turned the misunderstanding
   into the dedicated `TestBootstrapAtHeadNoHistoryReplay`.
4. A `multiedit` on `cmd/tq/main.go` failed because a concurrent agent
   refactored it to `runactor` mid-task — no damage (edit tool refused
   cleanly); re-read and rewired against the new structure. Expected churn per
   AGENTS.md, but it cost a round trip.
5. `StatusCompletion` field typo (`Attempt` for `CompletedAt`) — caught by
   `go build`, one-minute fix.

## e) WHAT WE SHOULD IMPROVE!

1. **Status executor runs NO repo verify gate** (`.tq-verify`/auto-detect)
   after the status agent commits — the agent executor enforces verify, the
   status executor only checks the report file exists. A status agent could
   commit a broken tree.
2. The sweeper does two `List` queries per agent completion (status tasks +
   agent tasks per project) — fine at dogfood scale, batchable per sweep.
3. Item-excerpt boilerplate (see b3): pin the real work-item text into
   `AgentPayload` (new optional field set by the harvester) instead of
   excerpting the prompt's first line.
4. Templ/Go LSP diagnostics remain a time sink (stale "typecheck" phantoms
   after every write); the AGENTS.md "verify with CLI" rule saved me twice
   this session — the LSP cache should be restarted or the client excluded.
5. The daemon-vs-agent commit race means feature work lands as anonymous
   heuristic commits — a `git commit` hook reserving staged-by-session work
   or a daemon allowlist for authored commits would preserve intent.

## f) NEXT — up to 50 (top ~20, prioritized; actionable subset appended to TODO_LIST.md)

| #  | Task                                                                                                                        | Impact             |
| -- | --------------------------------------------------------------------------------------------------------------------------- | ------------------ |
| 1  | Run repo verify after status-agent run in `StatusExecutor` (same gate as agent executor)                                    | HIGH — correctness |
| 2  | Live dogfood: enable `--status-every` on this repo's pool, watch one full window                                            | HIGH — proof       |
| 3  | `scripts/smoke/status-loop.sh`: stub-agent end-to-end (mint → run → report → TODO_LIST append → harvest re-arm) in ci-local | HIGH               |
| 4  | Web UI: status badge + report-path link from `StatusResult` detail                                                          | MED                |
| 5  | `internal/e2e` status-loop test (unix build tag)                                                                            | MED                |
| 6  | Full window detail: attach commit/files to every entry (lookup per completion fact)                                         | MED                |
| 7  | Pin real item text into `AgentPayload` (harvester change) → excerpts stop being boilerplate                                 | MED                |
| 8  | `tq show`: render `StatusResult`; `tq doctor`: `status-sweeper` watermark liveness                                          | MED                |
| 9  | DOMAIN_LANGUAGE.md: status window / done prompt / status report                                                             | LOW                |
| 10 | Fuzz `parseStatusResult` (mirror `result_fuzz_test.go`)                                                                     | LOW                |
| 11 | Batch sweeper List calls per sweep (perf hygiene)                                                                           | LOW                |
| 12 | SECURITY.md: status agents mint future work by appending TODO_LIST.md — blast radius + budget guard note                    | MED                |
| 13 | Document dead-status-task semantics (window re-arms at next completion; DLQ rescue)                                         | LOW                |
| 14 | Metrics: status mints/runs/deads in `tq stats` / webui metrics row                                                          | LOW                |
| 15 | Verify dead status tasks raise PapDashboard alerts like every dead letter (probably free; confirm)                          | LOW                |
| 16 | Prompt scope guard: status agent touches only `docs/status/*` + `TODO_LIST.md` + its commit                                 | MED                |
| 17 | `--status-every` in the ROUND4 dogfood launch command + `deploy/systemd` sample                                             | MED                |
| 18 | Config-file test coverage for `status-every` key (poolconfig tests)                                                         | LOW                |
| 19 | Consider `--status-model` (separate cheaper model for reports)                                                              | LOW                |
| 20 | Close/recheck the stale "surface agent-review verdicts" TODO checkbox (commit 48a9854 looks done)                           | LOW                |

## g) QUESTIONS I CANNOT ANSWER MYSELF

1. **Default-on policy:** should the dogfood pool on THIS repo enable
   `--status-every` (and at what N — 5? 10?), or stay opt-in until the live
   smoke passes? Cost/verbosity tradeoff is yours.
2. **Trust policy:** status reports are never reviewed (loop guard exempts the
   `status` type like reviews). Should reports be reviewed too (a second
   agent judging the report), or is the mechanical file-exists contract the
   right ceiling?
3. **Blast radius:** the status agent appends to TODO_LIST.md = it mints future
   autonomous work. Prompt-level caps only today — do you want a hard
   mechanical gate (parse the TODO_LIST diff, refuse runaway appends), or is
   budget-cap + prompt trust enough?

---

_Reported from this session's work only. Items 1–12 of (f) appended to
TODO_LIST.md (questions as BLOCKED items). Waiting for instructions._
