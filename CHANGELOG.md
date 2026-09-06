# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

### Added

- Nothing yet.

### Changed

- Nothing yet.

### Fixed

- Nothing yet.

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

