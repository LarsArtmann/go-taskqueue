# go-taskqueue — Agent Guide

Projects-aware task work queue / worker pool for Go. Embedded SQLite journal,
lease-based claims with crash reclaim, DAG dependencies, retries with a
dead-letter queue, pluggable executors (including headless AI coding agents).
Zero external services — one Go binary, one file.

**STATUS: pre-v0.1.0, actively developed by MULTIPLE concurrent agents.**
Before editing, `git pull` your assumptions: re-read files, re-run tests.
Expect uncommitted changes from parallel sessions — read them, judge them,
build on them, never revert them.

## Commands

```bash
go build ./...            # compile everything
go vet ./...
go test ./... -race       # the standard verify gate
nix build                 # reproducible build (flake, go-standard module)
nix run .#test            # tests via flake app
```

No Makefile, no justfile — flake.nix owns automation. Pure Go
(modernc.org/sqlite): `CGO_ENABLED=0` is set in the flake and safe everywhere.

CLI smoke test (worker has no --once flag; use timeout + background):

```bash
go build -o /tmp/tq ./cmd/tq
TQ_DB=/tmp/tq-smoke.db /tmp/tq enqueue --type sh --project demo --payload 'echo hi'
timeout 5 /tmp/tq worker --poll 100ms
TQ_DB=/tmp/tq-smoke.db /tmp/tq stats
```

## Architecture

Facts-first: every state change is an immutable fact in an append-only
journal; queue views, retry state, and the DLQ are projections of those
facts. Claim exclusivity comes from lease TTL + expiry reclaim.

| Package             | Purpose                                                                                                 |
| ------------------- | ------------------------------------------------------------------------------------------------------- |
| `internal/task`     | Task record, Status enum with `CanTransitionTo`, sentinel errors                                        |
| `internal/journal`  | Fact types, append-only Journal interface, MemoryJournal                                                |
| `internal/queue`    | Store interface + SQLite store; every mutation appends facts in-tx                                      |
| `internal/worker`   | Claim → heartbeat → execute loop; concurrency, panics, drain                                            |
| `internal/bridge`   | Outbound bridges: papdashboard (alerts), cqa (findings → fix tasks)                                     |
| `internal/executor` | Pluggable execution: `sh` command, HTTP, agent (headless AI), registry                                  |
| `internal/harvest`  | Scans repos' TODO_LIST.md and enqueues work items as agent tasks; drift audit (`tq audit`)              |
| `internal/budget`   | Daily-cap + budget-command projections over the journal, checked before each pool tick                  |
| `cmd/tq`            | CLI: enqueue / worker / harvest / agent-pool / stats / audit / top / show / dlq / cancel / facts / tail |

`internal/` layout is deliberate until the API stabilizes (ADR-0001 core,
ADR-0002 agent-pool policies: `docs/adr/0002-agent-pool-autonomy-pacing-drain.md`;
`docs/planning/` holds the broader plan); the module is not importable
externally yet. Domain vocabulary (task, fact, claim, lease, tick, drift, …)
is defined once in `docs/DOMAIN_LANGUAGE.md` — use those terms exactly.

### Invariants worth knowing before you touch the store

- **Single serialized writer**: `OpenSQLite` sets `MaxOpenConns(1)` + WAL +
  `busy_timeout`. Claim atomicity and the in-tx facts guarantee depend on that
  one connection — do not add a connection pool or drop the `RowsAffected()`
  re-checks (they are the multi-process race guard).
- **Task execution context survives pool shutdown** (bounded only by
  `--task-timeout`): a Ctrl-C lets in-flight agents finish and record their
  outcome. Never reintroduce a shared drain deadline into the task context —
  that bug stranded tasks in `running` forever.

### Payload contracts worth memorizing

- **`sh` executor**: payload is the shell line itself. Accepted shapes: raw
  text, JSON string (`"echo hi"`), or `{"cmd":"..."}`. The CLI wraps non-JSON
  payloads for `--type sh` as JSON strings; `unwrapCommand` unwraps all three.
  Other task types require valid JSON payloads — the CLI errors otherwise.
- **`agent`/`crush` executors**: payload is `AgentPayload`/`CrushPayload`
  JSON (repo, prompt, verify command, timeout). Verification must exit 0.
- **Idempotent enqueue**: `task.New.DedupKey` set → re-enqueue returns the
  stored task unchanged (no duplicate row, no duplicate fact). Backed by a
  partial unique index; `dedup_key` is added to legacy DBs by migration.
  A cancelled/dead task's key still suppresses re-enqueue — for harvested
  TODO items the escape hatch is editing the item text (the key is a hash of
  repo + text), so a wording change re-arms the item.
- **PapDashboard ingest contract**: `userId` is a REQUIRED metadata property
  (huma schema — the field has no omitempty); omit it and ingest returns 422.
  The bridge always sends `userId: ""`.

### SQLite migrations

Schema evolution happens in `migrate()` (sqlite.go): run the `CREATE TABLE IF
NOT EXISTS` schema, then pragma-check per column and `ALTER TABLE ADD COLUMN`
for legacy DBs, then create dependent indexes AFTER the column is guaranteed.
Indexes on new columns must NOT go into the schema const — legacy DBs would
fail with "no such column" before the ALTER runs.

## Conventions

- Table-driven tests with plain `testing` (no Ginkgo here, unlike PapDashboard)
- Sentinel errors in `internal/task/errors.go`; check with `errors.Is`
- Facts are the source of truth: a code change that mutates task state must
  append a fact in the same transaction
- Pure-Go deps only; keep `CGO_ENABLED=0` valid
- Go 1.26 idioms are deliberate (`errors.AsType[E]`, `strings.SplitSeq`,
  `for range n`) — do not "modernize" them back to older equivalents
- `TODO_LIST.md` is machine-consumed by the harvester (`internal/harvest`
  parses `- [ ]` checkboxes and the nearest heading): keep that format, one
  item per line, never convert it to tables

## Known Issues

- ⚠️ **Concurrent agents commit constantly**: the working tree may gain
  changes mid-task (e.g. a new executor, a migration). Re-run `go test ./...
  -race` right before declaring success; treat unexpected diffs as someone
  else's forward progress.
- ⚠️ **vendorHash drift**: after go.mod/go.sum changes run the fakeHash dance
  (`vendorHash = lib.fakeHash` → `nix build` → copy `got:`). The
  `checks.vendor-hash` gate fails fast on drift.
- ⚠️ **Flakes only see git-tracked files**: `git add` new files before
  `nix build` or Nix cannot see them.
- ⚠️ **tq worker runs until signalled**: there is no one-shot mode; scripts
  must wrap it in `timeout`/supervisor.
- ⚠️ **golangci-lint is not a CI gate**: CONTRIBUTING mentions it, but the
  baseline carries errcheck findings on idiomatic `defer x.Close()` lines.
  Do not mass-"fix" them; CI enforces vet + gofmt + tests only.
- ⚠️ **Agent tasks need repo-local autonomy**: `crush run` has no yolo flag;
  a `--yolo` task on a repo without a project-local `.crushrc` fails fast by
  design. When touching the agent argv, update
  `TestAgentExecutorArgvContract` — stub-based tests cannot catch flag drift.

## Relation to other projects

Semantics proven in go-cqrs-lite (facts/journal) and PapDashboard (worker
pools over durable queues); composes with both, depends on neither.

**PapDashboard bridge contract:** run
`tq worker --alert-url http://<pap>:8080 --alert-api-key <PAP_API_KEY>` (env:
`TQ_PAP_URL`/`TQ_PAP_API_KEY`). Dead-lettered tasks raise `alert.triggered`
(sourceApp `go-taskqueue`, severity critical, task ID as correlationId, fact
Seq as Idempotency-Key); a later completion of an alerted task posts
`alert.resolved` with the same derived title, so rescue flows close their own
alerts. The journal watermark starts at the head per bridge process —
incidents fired while it was down are not replayed (review via `tq dlq`).
Status and verification details: FEATURES.md, CHANGELOG.md.
