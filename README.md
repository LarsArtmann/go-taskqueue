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

### Watch it live in the browser

```sh
tq serve          # http://127.0.0.1:8090 (read-only)
tq serve --addr 0.0.0.0:8090 --auth-token "$(openssl rand -hex 16)"   # LAN
```

One tab shows the whole system updating live: status cards, the task table
(or a kanban **board view** at `/?view=board` — one column per lifecycle
status), the dead-letter queue, per-project progress and the fact feed —
pushed by SSE as server-rendered fragments, reconnect-safe, filterable and
searchable (`?project=demo&status=running&q=flake`), with a detail page per
task at `/task/{id}`. The dashboard is a pure projection of the journal:
it cannot mutate the queue, and the worst failure is a stale page. Opt into
operator actions (cancel / stop / rescue, CSRF-guarded forms) with
`tq serve --allow-writes` — off by default.

**Serving beyond localhost:** the dashboard renders every task payload and
error tail, so `tq serve` refuses to bind a non-loopback address (including
`:port` / `0.0.0.0`) without `--auth-token` / `$TQ_SERVE_TOKEN`. With a
token set, every route — pages, API, SSE, static — answers 401 unless the
request carries it as `Authorization: Bearer <token>` or `?token=<token>`
(the browser EventSource client cannot set headers, so it forwards the
query param; `--verbose` access logs redact the token). Plain HTTP: use a
token only on a trusted LAN, or front it with TLS.

The default `sh` executor runs the payload as a shell line (raw text, JSON
string, or `{"cmd":...}` all unwrap to the command). Executors are pluggable
in Go (`internal/executor/executor.go`) — note the packages are `internal/`
for now, so the CLI is the public surface until the library API stabilizes.

## The Agent Pool — self-managing improvement loop

One process turns every repo's backlog into a self-burning fire:

```sh
# feed the queue from every repo's TODO_LIST.md (idempotent, dedup-keyed)
tq harvest --projects-dir ~/projects --dry-run   # preview
tq harvest --projects-dir ~/projects

# run the whole loop: harvest every 5m + a pool of headless crush agents
# (--model pins the model in every payload; --project-exclusive guarantees
# one agent per repo across ALL pools sharing the DB)
tq agent-pool --projects-dir ~/projects --yolo --concurrency 2 --interval 5m \
  --model anthropic/claude-sonnet-4-5 --project-exclusive

# cron/timer-friendly: one harvest tick, drain, exit
# (pairs well with --daily-budget, the cost ceiling)
tq agent-pool --projects-dir ~/projects --once --daily-budget 20

# optional: Code-Quality-Agent findings become fix tasks each tick
tq agent-pool --projects-dir ~/projects --yolo --cqa-url http://localhost:8080 --cqa-owner $CQA_OWNER_ID

# optional: every completed agent task gets ONE review by a second agent;
# --review-autofix turns request_changes findings into deduped fix tasks
# (verdicts land in the task's facts — inspect with `tq show`)
tq agent-pool --projects-dir ~/projects --yolo --review --review-autofix \
  --daily-budget 40
```

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
set — note the pinned model+reasoning lives in each repo's `.crushrc`, so it
also applies to interactive crush sessions in that repo). Full agent output
sidecars default ON (`~/.local/state/tq/logs/<task-id>.log`; `--log-dir ""`
turns them off) — a daemon you cannot watch needs its logs. Re-running is
safe: the managed block is replaced in place, existing user config untouched.

**Model selection rides the repo `.crushrc`, not the pool**: bootstrap never
composes `--model` into the pool args, because a payload model makes the
executor pass `crush run -m`, which empirically RESETS the reasoning effort
to the provider default (crush debug telemetry, 2026-09-08). The `.crushrc`
`model large <provider/model> --reasoning-effort <effort>` slot is the only
mechanism that carries effort — it is written by the managed block above and
applies to both agent runs and interactive crush in that repo.

### Running it as a daemon

For unattended machines there is a systemd user unit with a wide graceful
stop window (in-flight agents finish and record their outcome):

```sh
go install github.com/larsartmann/go-taskqueue/cmd/tq@latest
mkdir -p ~/.config/systemd/user
cp "$(go env GOPATH)/pkg/mod"/github.com/larsartmann/go-taskqueue*/deploy/systemd/tq-agent-pool.service \
  ~/.config/systemd/user/ 2>/dev/null || true   # or copy from a git clone
systemctl --user daemon-reload
systemctl --user enable --now tq-agent-pool
journalctl --user -u tq-agent-pool -f
```

Prefer cron or a systemd timer? `tq agent-pool --once` runs exactly one
harvest tick, drains the queue, and exits — tasks owned by other pools or
scheduled for later are left alone.

On NixOS, a module ships with the flake (`nixosModules.default`, declaring
`services.tq-agent-pool` with the drain invariants baked in) — see
`deploy/nixos/tq-agent-pool.nix`.

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
- **Paced** — at most one in-flight backlog item per repo, `--max-per-tick`
  bounds cost per harvest run.
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

### Running the pool as a service

The pool is meant to outlive your terminal. A hardened user-level systemd
unit ships in `deploy/systemd/tq-agent-pool.service` (graceful 45-minute
drain on stop, `NoNewPrivileges` + read-only system paths, restart on
failure):

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

With `--cqa-url`, the latest Code-Quality-Agent scan's fixable findings for
each locally-present repo are enqueued as per-file fix tasks (dedup key
includes the scan ID, so a new scan arms new work) — the pool fixes what the
scanners find and the next scan proves it worked.

## Concepts

- **Task** — unit of work: `type` (executor key), `project`, JSON `payload`,
  `priority`, `deps` (task IDs that must complete first), `maxAttempts`,
  `notBefore` (delayed tasks).
- **Fact** — immutable journal record of everything that happens:
  `task.enqueued/claimed/heartbeat/completed/failed/dead-lettered/cancelled/released`
  (DLQ rescue records `task.enqueued` again with a `rescue` detail).
  `tq facts` / `tq tail -f` replay the entire history; `tq show TASK_ID`
  renders one task with its complete fact trail — for agent tasks that
  includes the crush session id and the verify output tail.
- **Claim / Lease** — a worker claims a due task exclusively; the lease has a
  TTL renewed by heartbeat. If the worker dies, the lease expires and another
  worker reclaims the task. At-least-once execution, no stuck tasks.
- **Deps** — DAG ordering between tasks: a task with un-completed deps is not
  claimable. Completion of A unlocks B.
- **DLQ** — a task exhausting `maxAttempts` lands in Dead status; `tq dlq --rescue`
  re-queues it with a fresh attempt budget.
- **Executor** — pluggable execution: `sh` (command), `http` (POST to URL),
  `agent` (headless AI coding agent with an enforced verify gate), or your
  own Go func. Workers look executors up by task `type`.

## Status codes

| status    | meaning                                             |
| --------- | --------------------------------------------------- |
| pending   | waiting for claim (possibly delayed or dep-blocked) |
| running   | claimed, lease held                                 |
| completed | done                                                |
| dead      | exhausted retries (DLQ)                             |
| cancelled | removed by operator                                 |

## Distribution

v0.1 is single-node SQLite. The Store interface is the distribution seam —
and the first two slices of it already ship: a Postgres store
(`SELECT … FOR UPDATE SKIP LOCKED`, ADR-0007) lets multiple `tq worker`
processes on different machines share the same queue with the same
semantics, and `tq api` (ADR-0008) serves non-Go producers over HTTP with
token auth. Outbound integrations ship today: a PapDashboard bridge turns
dead letters into alerts (`tq worker --alert-url`), and a
Code-Quality-Agent bridge turns scan findings into fix tasks
(`tq agent-pool --cqa-url`). See ROADMAP for the remaining distribution
work (CLI store wiring, consumer-group fencing tokens).

## Development

```sh
go test ./... -race             # root-module suite (CI also gates on go vet + gofmt)
for m in $(find internal -name go.mod | sed 's|/go.mod$||' | sort); do
  ( cd "$m" && GOWORK=off go test ./... -count=1 ) || exit 1
done                            # every sub-module (ADR-0011/0012) — ./... never
                                # crosses module boundaries, so test them in place
nix run .#test                  # the same loop as one command
./scripts/smoke/multi-repo.sh   # live smoke: 3 repos, 2 pools, 1 shared DB —
                                # proves dedup, pacing and per-project exclusivity
./scripts/smoke/webui.sh        # live smoke: worker + tq serve + HTTP/SSE assertions
```

### Module map

The repo is a multi-module tree (ADR-0011, ADR-0012): the root module builds
the `tq` CLI, and each library core is an independently tagged module under
`internal/` so embedders can depend on exactly the piece they need.

| Module                    | Purpose                                                                  |
| ------------------------- | ------------------------------------------------------------------------ |
| `internal/task`           | Task record, status state machine, sentinel errors                       |
| `internal/journal`        | Fact types, append-only Journal interface, in-memory Journal             |
| `internal/queue`          | The store CONTRACT: `Store` interface, `Filter`, `Queue` facade          |
| `internal/queue/sqlite`   | Embedded SQLite driver (`sqlite.Store`) — the default backend            |
| `internal/queue/postgres` | Networked PostgreSQL driver (`postgres.Store`) for shared-machine pools  |
| `internal/executor`       | Pluggable execution: `sh`, HTTP, headless agent, review, status, registry |
| `internal/worker`         | Claim → heartbeat → execute loop over any `queue.Store`                  |

The root module keeps the CLI and the integration packages (harvest, bridges,
sweepers, web UI, e2e) until the API stabilizes.

### Picking a store backend

Both drivers implement the same `queue.Store` contract with byte-compatible
facts (ADR-0007), so the choice is one import line:

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
wiring (`--store postgres://…`) is on the ROADMAP.

CI gates every push on vet, build, tests with `-race`, gofmt, a nix build

- `nix flake check`, the two live smokes above, a TODO_LIST
  harvest-parse guard, and a doc ghost-reference check. golangci-lint runs
  advisory (non-blocking): its ~400-finding repo baseline is documented in
  AGENTS.md, findings stay visible as CI annotations, and the hard gates are
  vet, gofmt and tests. The e2e suite (`internal/e2e`) drives the real `tq`
  binary as a subprocess with a stub agent, so the full agent loop is tested
  in CI at zero API cost. Reproducible builds via `nix build`. Agent sessions
  should read AGENTS.md first; feature status lives in FEATURES.md, upcoming
  work in TODO_LIST.md, long-term direction in ROADMAP.md, and the domain
  vocabulary in docs/DOMAIN_LANGUAGE.md.

### Proof-of-concept examples

`examples/api` (enqueue endpoint, Prometheus `/metrics`, live stats page)
and `examples/sse` (fact stream with `Last-Event-ID` resume) are PoCs —
`examples/` stays experimental by design and `tq serve` is the production
dashboard path. The PoC HTTP server binds loopback only and has no
authentication; do not expose it.

## License

MIT.
