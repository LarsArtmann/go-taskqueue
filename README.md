# go-taskqueue

**Projects-aware task work queue / worker pool for Go — with a self-managing
agent pool built in.** Embedded SQLite journal, lease-based claims with crash
reclaim, DAG dependencies, exponential backoff retries with a dead-letter
queue, and pluggable executors: shell commands, HTTP webhooks, or headless AI
coding agents. Zero external services — the whole system is one Go binary and
one file.

Built on the semantics proven in [go-cqrs-lite](https://github.com/LarsArtmann/go-cqrs-lite)
(facts/journal projection style) and [PapDashboard](https://github.com/LarsArtmann/PapDashboard)
(worker pools over durable queues). Not a wrapper around either — a standalone
library with its own small core, designed to be embeddable and observable.

## Why

I have too many projects. Cross-project work (builds, releases, scrapes,
checks, agent tasks) ends up scattered across cron jobs, shell scripts, and
memory. I want one durable queue where tasks know which project they belong
to, retry themselves, dead-letter when broken, and show me everything in one
place.

**The core idea: facts first.** Every state change is an immutable fact in an
append-only journal; the queue, retries, dead letters, and dashboard are all
projections of those facts. Nothing is ever deleted, so "what exactly
happened to this task?" always has an answer.

## Contents

[Quickstart](#quickstart) ·
[Live dashboard](#the-live-dashboard) ·
[Agent pool](#the-agent-pool) ·
[Bootstrap](#one-command-from-zero-tq-bootstrap) ·
[Running unattended](#running-unattended) ·
[Command map](#every-command-at-a-glance) ·
[Concepts](#concepts) ·
[Embedding in Go](#embedding-it-in-go) ·
[Distribution](#distribution) ·
[Development](#development)

## Quickstart

```sh
go install github.com/larsartmann/go-taskqueue/cmd/tq@latest

# a task whose payload is the shell command itself
tq enqueue --type sh --project demo --payload 'echo hello from $(uname -s)'

# or as JSON
tq enqueue --type sh --project demo --payload '{"cmd":"go test ./..."}'

# run a worker (Ctrl-C for graceful drain; --once drains and exits — cron-friendly)
tq worker

# watch it work
tq stats
tq tail -f
```

Tasks land in `./tasks.db` (override with `$TQ_DB` or `--db PATH` on any
command). Enqueueing is idempotent: give a task a `DedupKey` and re-enqueueing
returns the stored task instead of duplicating work.

## The live dashboard

```sh
tq serve          # http://127.0.0.1:8090 (read-only)
tq serve --addr 0.0.0.0:8090 --auth-token "$(openssl rand -hex 16)"   # LAN
```

One tab shows the whole system updating live: status cards, the task table
(or a kanban **board view** at `/?view=board` — one column per lifecycle
status), the dead-letter queue, per-project progress, and the fact feed —
pushed by SSE as server-rendered fragments, reconnect-safe, filterable and
searchable (`?project=demo&status=running&q=flake`), paginated, with a detail
page per task at `/task/{id}`. A journal browser pages through the full fact
history. Press `/` to search, `1`–`4` to jump between sections.

The dashboard is a pure projection of the journal: it cannot mutate the
queue, and the worst failure is a stale page. Opt into operator actions —
cancel (including cooperative stop of running tasks) and rescue, CSRF-guarded
forms with a failed-attempt lockout — via `tq serve --allow-writes`, off by
default.

**Serving beyond localhost:** the dashboard renders every task payload and
error tail, so `tq serve` refuses to bind a non-loopback address (including
`:port` / `0.0.0.0`) without `--auth-token` / `$TQ_SERVE_TOKEN`. With a token
set, every route — pages, API, SSE, static — answers 401 unless the request
carries it as `Authorization: Bearer <token>` or `?token=<token>` (the
browser EventSource client cannot set headers, so it forwards the query
param; `--verbose` access logs redact the token). Plain HTTP: use a token
only on a trusted LAN, or front it with TLS.

## The agent pool

One process turns every repo's TODO_LIST.md backlog into a self-burning fire:

```sh
# feed the queue from every repo's TODO_LIST.md (idempotent, dedup-keyed)
tq harvest --projects-dir ~/projects --dry-run   # preview
tq harvest --projects-dir ~/projects

# run the whole loop: harvest every 5m + a pool of headless AI agents
# (--project-exclusive guarantees one agent per repo across ALL pools
# sharing the DB; the model is pinned per-repo by tq bootstrap — see below)
tq agent-pool --projects-dir ~/projects --yolo --concurrency 2 --interval 5m \
  --project-exclusive

# cron/timer-friendly: one harvest tick, drain, exit
# (pairs well with --daily-budget, the cost ceiling)
tq agent-pool --projects-dir ~/projects --once --daily-budget 20

# optional: every completed agent task gets ONE review by a second agent;
# --review-autofix turns request_changes findings into deduped fix tasks
# (verdicts land in the task's facts — inspect with `tq show`)
tq agent-pool --projects-dir ~/projects --yolo --review --review-autofix \
  --daily-budget 40

# optional: every N completions per project mint one done-prompt report —
# an agent writes docs/status/<ts>_<name>.md and appends next items back
# into TODO_LIST.md, closing the loop (the pool feeds itself)
tq agent-pool --projects-dir ~/projects --yolo --status-every 10

# optional: Code-Quality-Agent findings become fix tasks each tick
tq agent-pool --projects-dir ~/projects --yolo --cqa-url http://localhost:8080 --cqa-owner $CQA_OWNER_ID
```

Each TODO item becomes one `agent` task: a headless `crush run` in that repo
with a strict contract (read AGENTS.md, smallest correct change, tick the
checkbox, commit, never push). The executor enforces the safety rails:

- **Opt-in autonomy** — agents only run under `--agents`/`tq agent-pool`;
  a plain `tq worker` never spawns one. `--yolo` is the operator's autonomy
  request — crush's headless mode has no yolo flag, so autonomy is actually
  granted by the repo itself: a project-local `.crushrc` declaring which
  tools its agents may use. The pool cannot over-grant what a repo never
  offered; a `--yolo` task on a repo without such a config fails fast with
  remediation guidance instead of burning agent attempts.

  ```sh
  # in each repo that wants agents (committed, reviewable, per-repo scope):
  echo 'permissions allow view ls grep glob edit write bash' > .crushrc
  ```
- **Clean tree required** — agents refuse repos with uncommitted changes
  (the pool never tramples human WIP; `--allow-dirty` opts out).
- **Verify enforced** — a task only completes when the repo still builds and
  tests pass. The verify command is resolved in priority order: the repo's
  own `.tq-verify` file wins (committed, reviewable — the repo decides how
  it is proven), then the payload's `verify`, then auto-detection
  (Go → `go build ./... && go test ./... -count=1`, `package.json` →
  `npm test --silent`, `Makefile` → `make test`, `flake.nix` →
  `nix build && nix flake check`, `Cargo.toml` → `cargo test --quiet`).

  ```sh
  # in the repo: pin exactly how agents must prove their work
  echo 'go vet ./... && go test ./... -count=1' > .tq-verify
  ```
- **Fail fast, not fail often** — input mistakes (bad payload, missing repo,
  unknown type) are dead-lettered after ONE attempt instead of burning the
  retry budget; environment problems (dirty tree, missing autonomy config)
  requeue the task without burning an attempt, so the pool picks it up once
  the human commits or adds the config.
- **Cost ceilings** — `--max-per-tick` bounds new tasks per harvest run,
  `--daily-budget` caps enqueues per calendar day, and `--budget-cmd` lets
  your own accounting veto every tick (exit non-zero = skip the tick).
  Poisoned repos (recent work all dead) pause harvesting for a cooldown
  (`--dlq-backoff`, together with `--repo-interval` for per-repo pacing).
- **Paced & pruned** — at most one in-flight backlog item per repo, and
  `--prune-stale` (on by default for pools) cancels pending tasks whose item
  is now `[x]`, reworded, or gone from the file — work that no longer has a
  reason to run never runs.
- **Machine-wide agent cap** — `--max-concurrent-agents N` bounds agent
  processes across every tq pool on the host via slot files (this is what
  `tq bootstrap --agents N` sets).
- **Full output sidecars** — `--log-dir DIR` (or `$TQ_LOG_DIR`, or a
  `log-dir =` line in the pool config) writes each task's complete agent +
  verify output to `DIR/<task-id>.log` (0600). Without it, only a tail lands
  in the result detail — turn this on for daemon pools where you cannot
  watch the terminal. Retention: `--log-dir-max-age 168h` sweeps logs older
  than the age, `--log-dir-max-bytes 5368709120` caps the directory's total
  size (oldest deleted first) — combine both so a long-running pool's
  output directory is bounded by time AND bytes.
- **Durable** — lease claims with heartbeats, exponential backoff, DLQ on
  exhaustion (`tq dlq --rescue` to retry), and the whole lifecycle replayable
  via `tq facts`.

**Model selection rides the repo `.crushrc`, not the pool**: bootstrap pins
the model + reasoning effort into a managed `.crushrc` block per repo.
A payload-level model would make the executor pass `crush run -m`, which
empirically RESETS the reasoning effort to the provider default (crush debug
telemetry, 2026-09-08). The `.crushrc` `model large <provider/model>
--reasoning-effort <effort>` slot is the only mechanism that carries effort —
it applies to both agent runs and interactive crush sessions in that repo.
(`tq agent-pool --model` exists for the rare case you want payload-level
pinning, but bootstrap deliberately never composes it.)

### One command from zero: `tq bootstrap`

Everything above, wired in one idempotent command — per repo it validates the
checkout, pins the verify contract into `.tq-verify` (auto-detected, or forced
via `--verify name=cmd`), grants agent autonomy in a managed `.crushrc` block
(plus the model + reasoning effort when `--model` is set), commits exactly
those files (never pushes), previews the harvest, then starts the pool:

```sh
tq bootstrap CV,SystemNix --agents 2                        # named repos under --projects-dir
tq bootstrap CV SystemNix --model zai/glm-5.3-flash --once  # explicit model, one supervised tick
tq bootstrap CV --verify 'CV=templ generate && bash scripts/go-change-gate.sh' --dry-run
tq bootstrap --install                                      # systemd user unit + pool.conf, then exit
```

`--agents N` sets pool concurrency and the machine-wide agent cap;
`--reasoning` defaults to `xhigh` (max possible; applied when `--model` is
set). Full agent output sidecars default ON
(`~/.local/state/tq/logs/<task-id>.log`; `--log-dir ""` turns them off) — a
daemon you cannot watch needs its logs. Re-running is safe: the managed block
is replaced in place, existing user config untouched.

## Running unattended

The pool is meant to outlive your terminal. A hardened user-level systemd
unit ships in `deploy/systemd/tq-agent-pool.service` (graceful 45-minute
drain on stop so in-flight agents finish and record their outcome,
`NoNewPrivileges` + read-only system paths, restart on failure):

```sh
mkdir -p ~/.config/systemd/user ~/.config/tq
cp deploy/systemd/tq-agent-pool.service ~/.config/systemd/user/
cat > ~/.config/tq/pool.conf <<'CONF'
# key=value, same names as the flags; flag > env > file precedence
projects-dir = /home/you/projects
yolo = true
project-exclusive = true
concurrency = 2
interval = 5m
daily-budget = 40
CONF
systemctl --user daemon-reload
systemctl --user enable --now tq-agent-pool.service
loginctl enable-linger $USER   # start at boot without a login session
journalctl --user -u tq-agent-pool -f
```

`tq agent-pool --config <file>` (or `$TQ_POOL_CONFIG`) reads flat
`key=value` settings — same names as the flags — applied to every flag you
did not pass explicitly. Precedence: **flag > environment > config file >
built-in default**. Unknown keys are an error, so a typo in the file fails
the pool loudly instead of silently running with defaults.

Prefer cron or a systemd timer? `tq agent-pool --once` runs exactly one
harvest tick, drains the queue, and exits — tasks owned by other pools or
scheduled for later are left alone.

On NixOS, a module ships with the flake (`nixosModules.default`, declaring
`services.tq-agent-pool` with the drain invariants and an explicit agent
toolchain PATH baked in) — see `deploy/nixos/tq-agent-pool.nix`.

## Every command at a glance

| Command          | What it does                                                                            |
| ---------------- | --------------------------------------------------------------------------------------- |
| `tq enqueue`     | Add a task: `--type`, `--project`, `--payload`, `--priority`, `--deps`, `--delay`, `--max-attempts` |
| `tq worker`      | Claim → heartbeat → execute loop; `--once` drains and exits; `--agents` for AI tasks; `--alert-url` for PapDashboard alerts |
| `tq harvest`     | Scan repos' TODO_LIST.md into dedup-keyed agent tasks; `--dry-run`, `--prune-stale`     |
| `tq agent-pool`  | The whole loop in one process: periodic harvest + worker pool + bridges + sweepers      |
| `tq bootstrap`   | One command from zero to a running pool (see above)                                     |
| `tq serve`       | Read-only live dashboard over SSE; `--allow-writes` adds guarded cancel/rescue          |
| `tq api`         | Token-authenticated HTTP API for non-Go producers (`POST /api/v1/tasks`)                |
| `tq stats`       | Counts, per-project table, budget spend, consumer lag (`--json`)                        |
| `tq tasks`       | Filtered task list: `--project/--status/--type/--since/--limit`, newest first (`--json`)|
| `tq top`         | Live per-project view with durations (`--once`, `--json`)                               |
| `tq show`        | One task + its complete fact trail (a unique ID prefix works)                           |
| `tq facts`       | Replay the journal: `--after SEQ`, `--json`, `--detail` for full fact payloads          |
| `tq tail`        | Follow the journal live (`-f`)                                                          |
| `tq dlq`         | Inspect dead letters; rescue one (`--rescue ID`) or in bulk (`--rescue-all --older-than 24h`) |
| `tq cancel`      | Cancel a pending task; `--force` cooperatively stops a running one                      |
| `tq doctor`      | Health checks: DB integrity, expired leases, worker heartbeats, budget, agent binary, per-repo autonomy files |
| `tq audit`       | TODO-vs-queue drift: stale-open items repaired by catch-up tasks, stale-done reported   |
| `tq watermarks`  | Journal consumer cursors + lag (`show`); `set CONSUMER SEQ` rewinds for safe replay     |
| `tq version`     | Build identity (version, VCS revision)                                                  |

## Concepts

- **Task** — unit of work: `type` (executor key), `project`, JSON `payload`,
  `priority`, `deps` (task IDs that must complete first), `maxAttempts`,
  `notBefore` (delayed tasks).
- **Fact** — immutable journal record of everything that happens:
  `task.enqueued`, `claimed`, `heartbeat`, `completed`, `failed`, `requeued`,
  `released`, `dead-lettered`, `cancelled`, `orphaned` (DLQ rescue records
  `task.enqueued` again with a `rescue` detail; failed attempts carry
  structured failure evidence — stage, exit code, output tail — so a failure
  is debuggable from the journal alone). `tq facts` / `tq tail -f` replay the
  entire history; `tq show TASK_ID` renders one task with its complete fact
  trail — for agent tasks that includes the crush session id and the verify
  output tail.
- **Claim / Lease** — a worker claims a due task exclusively; the lease has a
  TTL renewed by heartbeat. If the worker dies, the lease expires and another
  worker reclaims the task. At-least-once execution, no stuck tasks.
- **Deps** — DAG ordering between tasks: a task with un-completed deps is not
  claimable. Completion of A unlocks B.
- **DLQ** — a task exhausting `maxAttempts` lands in Dead status; `tq dlq --rescue`
  re-queues it with a fresh attempt budget.
- **Watermark** — a journal consumer's persisted read cursor. Bridges and
  sweepers checkpoint through it, so restarts resume instead of replaying or
  skipping; `tq watermarks show/set` is the ops escape hatch.
- **Executor** — pluggable execution: `sh` (command), `http` (POST to URL),
  `agent` (headless AI coding agent with an enforced verify gate), or your
  own Go func. Workers look executors up by task `type`.

## Status codes

| status    | meaning                                              |
| --------- | ---------------------------------------------------- |
| pending   | waiting for claim (possibly delayed or dep-blocked)  |
| running   | claimed, lease held                                  |
| completed | done                                                 |
| dead      | exhausted retries (DLQ)                              |
| cancelled | withdrawn by operator (or pruned as stale)           |

## Embedding it in Go

The library core is a set of independently tagged modules, so embedders
depend on exactly the piece they need (import paths are stable; the repo is
a multi-module tree by design):

| Module                    | Purpose                                                                   |
| ------------------------- | ------------------------------------------------------------------------- |
| `internal/task`           | Task record, status state machine, sentinel errors                        |
| `internal/journal`        | Fact types, append-only Journal interface, in-memory Journal              |
| `internal/queue`          | The store CONTRACT: `Store` interface, `Filter`, `Queue` facade           |
| `internal/queue/sqlite`   | Embedded SQLite driver (`sqlite.Store`) — the default backend             |
| `internal/queue/postgres` | Networked PostgreSQL driver (`postgres.Store`) for shared-machine pools   |
| `internal/executor`       | Pluggable execution: `sh`, HTTP, headless agent, review, status, registry |
| `internal/worker`         | Claim → heartbeat → execute loop over any `queue.Store`                   |

Both store drivers implement the same `queue.Store` contract with
byte-compatible facts, so the choice is one import line:

```go
import "github.com/larsartmann/go-taskqueue/internal/queue/sqlite"

store, err := sqlite.Open("tq.db", sqlite.WithProjectExclusivity())
// or
import "github.com/larsartmann/go-taskqueue/internal/queue/postgres"

store, err := postgres.Open(ctx, "postgres://…", 4)
```

SQLite serializes writers through one connection (WAL + busy_timeout);
Postgres uses `SELECT … FOR UPDATE SKIP LOCKED` so competing workers lock
disjoint rows. The CLI itself wires the SQLite driver today; Postgres CLI
wiring (`--store postgres://…`) is on the ROADMAP. Executors are pluggable
in Go (`internal/executor/executor.go`) — note the packages are `internal/`
for now, so the CLI is the public surface until the library API stabilizes.

## Distribution

Single-node SQLite by default. The Store interface is the distribution seam —
and the first two slices of it already ship: a Postgres store
(`SELECT … FOR UPDATE SKIP LOCKED`) lets multiple `tq worker` processes on
different machines share the same queue with the same semantics, and `tq api`
serves non-Go producers over HTTP with token auth. Outbound integrations ship
today: a PapDashboard bridge turns dead letters into alerts — and later
resolves them when the task is rescued and completes (`tq worker --alert-url`)
— and a Code-Quality-Agent bridge turns scan findings into fix
tasks (`tq agent-pool --cqa-url`). See ROADMAP for the remaining distribution
work (CLI store wiring, consumer-group fencing tokens).

## Development

```sh
nix run .#test        # full multi-module suite (root + every internal/* module)
./scripts/ci-local.sh # the pre-push gate: full CI replicant (vet/build/race/smokes/nix)
go build ./... && go vet ./... && go test ./... -race   # root-module suite
./scripts/smoke/multi-repo.sh   # live smoke: 3 repos, 2 pools, 1 shared DB
./scripts/smoke/webui.sh        # live smoke: worker + tq serve + HTTP/SSE assertions
```

CI gates every push on vet, build, tests with `-race`, gofmt, a nix build,
flake check, the live smokes, and a doc ghost-reference check. The e2e suite
drives the real `tq` binary as a subprocess with a stub agent, so the full
agent loop is tested in CI.

For contributors and agents: read AGENTS.md first — it carries the
architecture invariants (single serialized SQLite writer, facts in the same
transaction as state, task contexts that survive shutdown). Feature status
lives in FEATURES.md, upcoming work in TODO_LIST.md, long-term direction in
ROADMAP.md, design decisions in `docs/adr/`, and the domain vocabulary in
`docs/DOMAIN_LANGUAGE.md`.

`examples/api` (enqueue endpoint, Prometheus `/metrics`, live stats page) and
`examples/sse` (fact stream with `Last-Event-ID` resume) are
proofs-of-concept — `examples/` stays experimental by design and `tq serve`
is the production dashboard path. The PoC HTTP server binds loopback only
and has no authentication; do not expose it.

## License

MIT.
