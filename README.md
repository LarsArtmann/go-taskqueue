# go-taskqueue

**Projects-aware task work queue / worker pool for Go.** Embedded SQLite journal,
lease-based claims with crash reclaim, DAG dependencies, exponential backoff
retries with a dead-letter queue, pluggable executors. Zero external services —
the whole system is one Go binary and one file.

Built on the semantics proven in [go-cqrs-lite](https://github.com/LarsArtmann/go-cqrs-lite)
(facts/journal projection style) and [PapDashboard](https://github.com/LarsArtmann/PapDashboard)
(worker pools over durable queues). Not a wrapper around either — a standalone
library with its own small core, designed to be embeddable and observable.

## Why

I have too many projects. Cross-project work (builds, releases, scrapes, agent
tasks, checks) ends up scattered across cron jobs, shells, and memory. I want
one durable queue where tasks know which project they belong to, retry
themselves, dead-letter when broken, and show me everything in one place.

## Quickstart

```sh
go install github.com/larsartmann/go-taskqueue/cmd/tq@latest

# a task whose payload is the shell command itself
tq enqueue --type sh --project demo --payload 'echo hello from $(uname -s)'

# or as JSON
tq enqueue --type sh --project demo --payload '{"cmd":"go test ./..."}'

# run a worker (Ctrl-C for graceful drain)
tq worker

# watch it work
tq stats
tq tail -f
```

The default `sh` executor runs the payload as a shell line (`{"cmd":...}` JSON
is unwrapped). Register your own executor types in Go — see
`internal/executor/executor.go`.

## Concepts

- **Task** — unit of work: `type` (executor key), `project`, JSON `payload`,
  `priority`, `deps` (task IDs that must complete first), `maxAttempts`,
  `notBefore` (delayed tasks).
- **Fact** — immutable journal record of everything that happens:
  `task.enqueued/claimed/completed/failed/dead-lettered/cancelled/rescued/lease-lost`.
  `tq facts` / `tq tail -f` replay the entire history.
- **Claim / Lease** — a worker claims a due task exclusively; the lease has a
  TTL renewed by heartbeat. If the worker dies, the lease expires and another
  worker reclaims the task. At-least-once execution, no stuck tasks.
- **Deps** — DAG ordering between tasks: a task with un-completed deps is not
  claimable. Completion of A unlocks B.
- **DLQ** — a task exhausting `maxAttempts` lands in Dead status; `tq dlq --rescue`
  re-queues it with a fresh attempt budget.
- **Executor** — pluggable execution: `sh` (command), `http` (POST to URL),
  or your own Go func. Workers look executors up by task `type`.

## Status codes

| status    | meaning                                             |
| --------- | --------------------------------------------------- |
| pending   | waiting for claim (possibly delayed or dep-blocked) |
| running   | claimed, lease held                                 |
| completed | done                                                |
| dead      | exhausted retries (DLQ)                             |
| cancelled | removed by operator                                 |

## Distribution

v0.1 is single-node. The Store interface is the distribution seam: a Postgres
or Redis store lets multiple `tq worker` processes on different machines share
the same queue with the same semantics (claim exclusivity via lease, crash
reclaim via lease expiry). v0.2 plan: Postgres store (`SELECT … FOR UPDATE
SKIP LOCKED`), HTTP API server, PapDashboard webhook bridge.

## Development

```sh
go test ./... -race
```

## License

MIT.
