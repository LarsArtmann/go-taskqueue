# Derived outcomes & the verdict channel

Owner direction 2026-09-14: the `TQ_RESULT: {"files_changed": [...],
"commit_sha": "..."}` self-report line is hated. Two rulings:

1. The queue DERIVES what an agent session did (go-crush-data + git);
   agents stop reporting derivable facts.
2. Where direct agent→queue communication is genuinely needed (verdicts),
   it goes through a small focused CLI — not stdout parsing. A custom MCP
   was evaluated and REJECTED: it adds server lifecycle + tool-grant churn
   (headless mode denies unlisted tools) for exactly one call site that
   `bash` + the `tq` binary already cover.

## What the queue wants to know after a work task

| Question                                  | Source of truth                        | Self-report needed? |
| ----------------------------------------- | -------------------------------------- | ------------------- |
| Did it succeed?                           | exit code + verify gate (existing)     | no                  |
| Which commits landed?                     | git log `Task-Queue-ID: <id>` trailers | no — derivable      |
| Which files shipped?                      | `git show --name-only` over those SHAs | no — derivable      |
| What did the session do?                  | go-crush-data (cost, tokens, messages) | no — derivable      |
| Verdict (review/dlqfix/prioritize/status) | the agent's judgment                   | YES — communication |

The recurring no-op/TQ_RESULT sha-semantics ruling asks (TODO rows 250,
311; a dozen report §g items) existed only because the agent had to CHOOSE
what to cite. Derivation removes the choice: a no-op re-dispatch derives
"zero footer commits" automatically. No ruling needed.

## Design

### 1. Trailer scan moves into internal/executor

`internal/session/scan.go` (root module) moves verbatim to
`internal/executor/gitscan.go` (executor sub-module). internal/session
already imports internal/executor, so the bridge keeps working via
`executor.GitLogScanner`; `session.Commit` becomes a type alias keeping
the CloseDetail wire shape. cmd/tq/session.go constructs the executor
scanner. Facade aliases added for the moved exports (parity gate).

### 2. Derivation fills AgentResult (best-effort, never fails a task)

After verify succeeds, executor.Execute derives:

- `Commits []Commit` — `CommitsByTrailer(repo, "Task-Queue-ID", taskID)`,
  oldest first.
- `FilesChanged` — union of files in those commits (`git show --name-only`).
  Overrides the self-report when commits exist; self-report (legacy
  in-flight tasks) still fills the gap when none do.
- `CommitSHA` — newest derived commit (compat field: review sweeper +
  webui read it).
- Session stats via go-crush-data (new dep of the executor module,
  v0.4.0, MIT, single dep modernc.org/sqlite): registry lookup
  repo→dataDir, `db.Session(sessionID)` → cost/tokens/message count,
  only when a session id was extractable from the run output. Failures
  degrade to absent fields.

### 3. Verdict channel: TQ_RESULT_FILE + `tq verdict`

- runAgent creates a per-run temp file and exports `TQ_RESULT_FILE` in the
  agent process env (single choke point; all four verdict executors +
  work tasks inherit it). argv itself is unchanged (argv contract test
  stays green).
- `tq verdict '<json>'` (arg or `-` = stdin): validates JSON syntax,
  writes $TQ_RESULT_FILE, errors with guidance when unset. No store
  access, no TQ_DB coupling.
- runAgent appends the file's content as a synthetic trailing
  `TQ_RESULT: <json>` line to the returned output. File wins over any
  stdout line (appended last); stdout line remains a LEGACY fallback for
  in-flight pool tasks and stub smokes. All existing strict parsers
  (review/status/dlqfix/prioritize) and fuzz corpus stay unchanged.
- Fallback for PATHs without tq: write the JSON to `$TQ_RESULT_FILE`
  directly.

### 4. Prompt contracts drop the line

- Work contracts (harvest, catchup/drift, cqa): TQ_RESULT step deleted;
  footer step stays. The closeout prompt drops its re-emit paragraph
  (derivation covers it; the mechanical gate no longer reads the work
  turn's line).
- Verdict contracts (review, status, dlqfix, prioritize): the
  "end with TQ_RESULT" teaching becomes "run tq verdict '<json>'" with
  the same shape examples; a clause names the $TQ_RESULT_FILE fallback.

## Migration

Legacy stdout TQ_RESULT stays parseable (result.go regex untouched);
prompts stop teaching it. In-flight pool tasks complete under the old
contract; new enqueues use derivation + verdict channel. The legacy path
is deleted in a later release once the live pool shows derived outcomes.

## Rejected alternatives

- **MCP server**: one tool call (`report verdict`) does not justify a
  server binary lifecycle, per-agent process spawns, managed-block tool
  grants, and a new install surface. Revisit if agents ever need queue
  READS (claim subtasks, list blockers) mid-run.
- **Verdicts via journal writes from agents**: breaks the single-writer
  state machine discipline (facts ride task transitions only) and couples
  agents to TQ_DB (scratch agents inherit the production path).
- **Session-id-pinned derivation for commits**: unnecessary — the
  Task-Queue-ID trailer is exact; session data only enriches.
