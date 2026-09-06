# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

### Added

- Tooling policy decided and enforced: golangci-lint is CI-gated
  (`.golangci.yml` with errcheck exclusions for idiomatic deferred Close and
  HTTP body/rows Close), dprint joins the flake devShell and the living docs
  are formatted with it; CONTRIBUTING lists all local gates
- Operator documentation set: `docs/DOMAIN_LANGUAGE.md` (the ubiquitous
  language: task, fact, claim, lease, tick, drift, catch-up, …), ADR-0002
  (agent-pool autonomy, pacing, budgets and drain semantics), and SECURITY.md
  (trust model, blast radius of agent `bash`, hardening checklist)
- The pool prints an autonomy warning at start under `--yolo`: agents may
  run shell commands unsandboxed per repo `.crushrc`, with pointers to the
  budget caps that bound the blast radius
- Windows/i18n hygiene: `GOOS=windows` build + vet is now a CI gate; golden
  dedup-key vectors pin non-ASCII item hashing (CJK, emoji, Unicode
  whitespace collapsing) and keys are proven independent of path spelling,
  so harvesting a repo via relative or absolute paths never double-enqueues
- Docs-drift auditor (`tq audit`): compares repos' TODO_LIST.md checkboxes
  with terminal task states and repairs the drift harvest can't see — work
  an agent completed but never ticked off gets a dedup-keyed catch-up task
  (armed once, re-audits never pile up); hand-ticked items with unfinished
  tasks are reported for the operator to cancel
- `tq top`: live per-project view over the journal — pending/running/done/
  dead counts, the last run duration and the active run's elapsed time
  (`--once`, `--json`, `--interval`; repaints on terminals)
- Task result detail: completed agent tasks record the crush session id and
  the verify output tail in the `task.completed` fact; `tq show TASK_ID`
  renders the task together with its full fact trail, so an operator can
  trace exactly what an agent did and how the work was proven
- End-to-end CLI suite (`internal/e2e`): builds the real `tq` binary and
  drives `agent-pool --once` as a subprocess with a stub agent — the full
  harvest → claim → work → verify → complete loop is proven from outside the
  process, in CI, at zero API cost
- Property test for harvest dedup keys (stable under whitespace reflow,
  distinct across repos with identical item text) and a fuzz harness for the
  TODO parser (CRLF, BOM, nesting — 1.8M executions, zero findings)
- Chaos test: a worker SIGKILLed mid-task is reclaimed via lease expiry and
  the work completes exactly once in the journal
- Cost ceilings for unattended pools: `--daily-budget` (max enqueues per
  calendar day, projected from the journal), `--budget-cmd` (your own
  accounting vetoes each tick), `--repo-interval` (per-repo enqueue gap),
  `--dlq-backoff` (pauses poisoned repos whose recent work all died)
- `tq agent-pool --once`: one harvest tick, drain, exit — cron/timer
  friendly, with a systemd user unit in `deploy/systemd/`
- Preflight refusals: `executor.PreflightError` + `Store.Requeue` — a dirty
  tree or missing autonomy config requeues a task WITHOUT burning an
  attempt (claimable again after a delay); the autonomy probe also accepts
  a user-global crush config
- Verify strategy: the repo's `.tq-verify` file wins over payload and
  auto-detection; auto-detection adds Makefile, flake.nix and Cargo repos;
  the harvester pins a repo's `.tq-verify` command into every payload
- `--model` on `tq harvest` / `tq agent-pool`: pin the crush model in every
  harvested agent payload

### Changed

- Nothing yet.

### Fixed

- `tq agent-pool --once` hung after draining: the pool stopped but the
  process waited for a signal, because `Start` only returns when the
  caller's context is done — the drain watcher now cancels it too
- Two processes opening a brand-new queue database raced the schema writes
  and the loser failed with SQLITE_BUSY; the migration now retries with a
  bounded backoff (always converges on `IF NOT EXISTS` DDL)
- A task claim racing shutdown logged a spurious "claim failed" error

## [0.1.0] - 2026-09-06

### Added

- **Agent pool**: `tq agent-pool` runs a self-managing loop — periodic
  harvest of repos' TODO_LIST.md backlogs into dedup-keyed `agent` tasks,
  worked by a pool of headless crush agents with an enforced verify gate
  (build + tests), clean-tree protection, per-repo pacing and cost caps
- `tq harvest` command: idempotent backlog ingestion (`--dry-run`,
  `--max-per-tick`, `--allow-dirty`)
- `agent` executor (`internal/executor/agent.go`): runs crush headlessly per
  task, process-group kill on timeout, output tails in errors
- Idempotent enqueue via `DedupKey` (partial unique index, legacy-DB
  migration) — repeated harvests never double-enqueue
- CQA bridge (`internal/bridge/cqa`): Code-Quality-Agent scan findings become
  per-file agent fix tasks (`--cqa-url` on `tq agent-pool`)
- PapDashboard bridge (`internal/bridge/papdashboard`): dead-lettered tasks
  raise alerts; rescues resolve them (`--alert-url` on `tq worker`)
- Permanent-vs-transient error classes: `executor.PermanentError` marks
  failures retrying can never fix (bad payload, missing repo, dirty tree,
  missing autonomy, unknown task type, non-zero sh exit, permanent HTTP
  statuses); the worker dead-letters them after ONE attempt instead of
  burning the retry budget — for agent tasks every retry is real money
- Store-level per-project claim exclusivity (`queue.WithProjectExclusivity`,
  `tq worker/agent-pool --project-exclusive`): across ALL pools and processes
  sharing the database, a project never has two running tasks at once — the
  per-repo serialization guarantee for multi-pool agent setups
- CI reliability bundle: nix build + flake check job, TODO_LIST
  harvest-parse guard, and a ghost-reference check that fails when a living
  doc cites a repo path that does not exist

### Changed

- `tq enqueue --type sh` accepts a raw shell line as `--payload` (previously
  JSON-only, contradicting the README quickstart): non-JSON payloads are
  wrapped as JSON strings; the `sh` executor unwraps all three payload shapes
  (raw text, JSON string, `{"cmd":...}`)
- Agent autonomy is granted per-repo, not per-flag: `crush run` has no
  `--yolo` flag (v0.92: "Unknown flag: --yolo"), so `--yolo` now requests
  the repo's own `.crushrc` permissions model — yolo tasks on repos without
  a project-local crush config fail fast with remediation guidance instead
  of stalling headless runs on permission prompts

### Deprecated

### Removed

### Fixed

- Worker pool executed every task under the 30-second drain context created
  at pool start: any task claimed after 30 seconds of uptime ran with an
  already-expired context, its Complete/Fail write failed with
  "context deadline exceeded", and the task was stuck `running` forever.
  Tasks (execution, heartbeats, terminal writes) now run under a
  shutdown-surviving context bounded only by `--task-timeout`; graceful stop
  lets in-flight agents finish and record their outcome. Pinned by a
  long-task regression test (fails on revert) and a de-flaked shutdown test
- `--yolo` injected a nonexistent flag into every autonomous agent run (all
  yolo tasks failed instantly with "Unknown flag: --yolo"); the agent argv
  is now pinned by a contract regression test so flag placement cannot
  silently regress again
- Test-only: removed invalid `_ = t.Cleanup(...)` / single-value Enqueue
  assignments that broke compilation of queue and worker test files
- `nix build` failed with a vendor-hash mismatch after the go.sum bump
  (modernc.org/libc v1.75.7); `vendorHash` re-pinned via the fakeHash dance,
  `nix build` and `nix flake check` verified green

### Security

