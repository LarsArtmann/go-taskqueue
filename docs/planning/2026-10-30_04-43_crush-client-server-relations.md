# Crush client/server relations — leverage map and migration ladder

**Severity: OPPORTUNITY (architecture decision pending owner)** · **Status:
RESEARCHED + MECHANICS PROVEN 2026-10-30 on installed crush v0.98.2 (all
proofs below are captured, reproducible commands in §Evidence) · RE-VERIFIED
on v0.99.0 2026-11-01: routing + live workspace listing HOLD with the env
gate (E7 correction below), M8 DISPROVEN for tool exec (§v0.99
re-verification), M5 shapes captured, L1.8 smoke SHIPPED
(`scripts/smoke/crush-server-route.sh`)** · Session
context: hooks fleet shipped same day (AGENTS.md §Conventions).

Thesis: the installed crush already ships a full client/server architecture
(`crush server` + 77-endpoint v1 API + `CRUSH_CLIENT_SERVER=1` client mode),
and the pool dispatches one in-process `crush run` per task instead — every
dispatched agent is an island. Routing dispatches through a pool-owned server
buys live operator observability, browser-answerable questions, structured
closeout data, and a cancel/steering surface, at the cost of one shared
process. This doc maps the leverage, the proven mechanics, the risks, and a
ladder from zero-risk to full adoption. **Nothing here is executed; L1 is the
recommended first rung.**

---

## Proven mechanics (v0.98.2, unix socket, 2026-10-30)

| # | Mechanic | Proof | Executor compatibility |
| --- | --- | --- | --- |
| M1 | `CRUSH_CLIENT_SERVER=1` + `-H unix://<sock>` turns `crush run` into a thin client; the agent executes INSIDE the server process (workspace created per run, deleted on exit) | workspace list shows `{path, data_dir, version, config}` during run; gone after | drop-in: same argv + 2 additions |
| M2 | SSE live telemetry per workspace: `session.created/updated` (titles, prompt/completion tokens), `message.created/updated` with streaming `tool_call` parts (`{"command":"sleep 30",...}`) and `tool_result` frames | captured live during a routed run (§Evidence E3); frames are `data:`-only (no `event:` names) | none needed; this is the webui feed |
| M3 | Project `.crushrc` hooks run SERVER-side (workspace path = cwd) | routed `gofmt -l .` in this repo hit `tq-bash-guard` deny with full recipe (§Evidence E2) | hook fleet survives unchanged |
| M4 | `--verbose` prints the byte-identical `Created session for non-interactive run session_id=<uuid>` line; `--session <id>` resume through the server carries memory | E4/E5 (codeword round-trip) | `ExtractSessionID` + closeout resume need ZERO changes |
| M5 | Structured session data via API: `GET .../sessions`, `.../sessions/{sid}/messages` (full message/parts JSON) | 200 + payload during run | future closeout/report surface |
| M6 | Permission/question prompts are delivered ONLY to SSE-attached clients whose `current-session` views the session (attach FIRST, then `POST .../current-session`, else 409 "client not attached") | E6 | the authority surface for L3 |
| M7 | Idle server self-terminates; clients auto-respawn it (`ensureServer` in `connectToServer`) | server vanished after last client detached, next run recreated it | lifecycle is self-healing; a keeper client (L2) removes idle churn |
| M8 | **DISPROVEN for tool exec on v0.99.0 (2026-11-01; source-read claim below was too optimistic):** the client DOES forward its env with workspace creation (`root.go` `wsReq.Env = os.Environ()`), but the server's bash TOOL never sees it — `internal/shell/background.go` builds the tool shell WITHOUT workspace Env and `internal/shell/shell.go` falls back to the server's `os.Environ()`. Two-sided empirical pin: server env vars ARE visible in the routed bash tool, client `$TQ_RESULT_FILE` expands EMPTY (3×, fresh stacks + repeat runs) | original v0.98.2 claim: `connectToServer` source `Env: os.Environ()` (true but insufficient — the env stops at the workspace struct; only `RunShellCommand`, the interactive `$` shell, merges `ws.Env`) | `$TQ_RESULT_FILE` / `$TQ_QUESTION_FILE` semantics BREAK under routing — the L1 env channels stay caveated until upstream fixes tool-env propagation or the executor moves them out of env; the smoke's env-gap canary pins the current behavior |

## Lifecycle facts that shape the design

- **No yolo flag on `crush run`** (any position; verified "Unknown shorthand
  flag") — autonomy stays permissive-`.crushrc`, exactly the pool's existing
  `requireRepoAutonomy` mechanism. Non-yolo scratch cwds HANG on permission
  prompts (two runs stalled 5+ min in the proof session; nobody viewed the
  session to answer).
- **Workspace = per-run ephemeral; session = durable in the data-dir.** Even
  LOCAL runs' sessions are visible to a server sharing the data-dir
  (workspace lists sessions from disk) — observability partially decouples
  from dispatch routing.
- **Two sessions per run**: the main turn + a small-model title-generation
  session (seen in the SSE stream; 29k prompt tokens = system prompt on the
  main one).
- **`client_id` must be a UUID** on the events stream (400 otherwise).

## Leverage map (what each relation buys THIS system)

| Relation | Buys | Consumer today | Rung |
| --- | --- | --- | --- |
| SSE events (M2) | live per-task agent activity: tool calls in flight, token burn, message stream — the "what is it DOING" pane `tq serve` lacks | webui (ADR-0003 hub already exists) | L2 |
| questions/answer (M6) | browser-answerable agent questions: surface the pending prompt in webui, answer via `POST .../questions/answer` — replaces CLI-only `tq ask` round-trips | AnswerPoller + webui | L3 |
| sessions/messages (M5) | structured closeout data (no stdout parsing for report facts) | `--task-closeout` reporting | L3 |
| agent cancel | dashboard kill-switch for a stuck dispatched turn | pool-health detectors | L3 |
| prompt queue + (v0.99) notes | steering a running turn without killing it | future | L4 |
| permissions/grant (M6) | central policy authority (queue answers prompts) | — pool runs allow-list `.crushrc` by design; LOW value now | L4 (skip unless yolo dies) |
| shared server process | no per-task process spawn; ONE place to observe | executor | L4 |

## Migration ladder

- **L0 (today)**: nothing changes. Every dispatched `crush run` in-process.
- **L1 — opt-in seam (recommended, ~hours)**: executor gains a
  `--crush-server <socket>` pool flag: when set, `agentEnv` adds
  `CRUSH_CLIENT_SERVER=1` and args add `-H <socket>`; absent → today's path.
  Pool starts `crush server --host unix://<data-dir>/crush-pool.sock` as a
  runactor actor. Rollback = unset flag. Proven safe by M3/M4/M8 (hooks,
  session-id extraction, env, verdict files all survive).
- **L2 — observability (webui)**: `tq serve` attaches one SSE client per
  active workspace (race-attach pattern from E3), streams tool-call events
  into the existing hub → live agent pane. Read-only; no dispatch coupling:
  works even for LOCAL runs' sessions via shared data-dir sessions (M5) —
  though live tool events need routed runs (L1).
- **L3 — interaction**: webui surfaces pending questions for viewing
  sessions (M6 ordering!) and answers via the API; structured closeout pulls
  from messages; cancel button → agent cancel endpoint.
- **L4 — full routing**: all dispatched agents through the shared server.
  Crash domain becomes fleet-wide (one server OOM = every in-flight task
  requeues); needs keeper client (M7), memory watch, version pinning.

## Risks & open questions

- **Shared crash domain** (L4): server death fails all in-flight agent tasks
  at once (requeue recovers, attempts burn unless `IsEnvironmentalDeath`
  classifies it). L1–L3 keep the blast radius of today.
- **Memory accumulation**: N concurrent agent goroutines in one process vs N
  processes; MEASURED 2026-11-03 (scripts/smoke/crush-server-memory.sh, the
  L1 pilot protocol at zero spend: one isolated server, 2 waves x 4
  concurrent routed turns, mock provider): baseline 55-58 MB, wave peak
  110-135 MB at 4 concurrent, quiescent 87-120 MB; wave-2 quiescent never
  above wave-1 — shared-server RSS does NOT accumulate per wave (4/4 green
  runs). Concurrency sweep beyond N=4 still open (TODO_LIST row 1519).
- **Idle-suicide churn** (M7): without an attached keeper, the server dies
  between waves and respawns per task (fine functionally; wasted spawn).
- **Version coupling**: server and client are the same binary today; a
  half-upgraded host (server 0.98, client 0.99) is untested.
- **Concurrent same-path workspaces**: two routed runs in one cwd create two
  workspaces (observed); session list merges by data-dir — titles collide but
  ids don't. Executor uses per-task worktrees anyway (`--worktrees`).

## v0.99 deltas (upgrade notes, NOT required for L1–L3)

`notes` (append-to-next-prompt), host-context, host-theme, piped permissions
(`CRUSH_PERMISSIONS_PIPED=1`), SIGHUP config reload, graceful shutdown. The
v0.98.2→v0.99 endpoint diff is exactly 5 endpoints (host-context ×2, theme
×2, notes ×1) — everything this plan relies on exists today.

## Evidence (captured 2026-10-30, crush v0.98.2, unix socket)

- **E1 routing**: `crush server --host unix:///tmp/crush-relate.sock` +
  `CRUSH_CLIENT_SERVER=1 crush run -H unix:///tmp/crush-relate.sock ...` →
  run executes server-side; `GET /v1/workspaces` shows it live.
- **E2 hooks server-side**: routed run `--cwd <repo>` with `gofmt -l .` →
  `Tool call blocked by hook. Reason: BLOCKED: unscoped gofmt ...` (full
  tq-bash-guard recipe).
- **E3 SSE**: attach `GET /v1/workspaces/{id}/events?client_id=<uuid>` with
  `Accept: text/event-stream` during a routed `sleep 30` run → captured
  `session.created`, `message.created` (user + assistant), `session.updated`
  (prompt_tokens 161 → 29179), `message.updated` with tool_call input
  `{"command":"sleep 30","description":...}`, `tool_result` (`no output`,
  start_time metadata).
- **E4 session-id line**: `--verbose` routed run prints `INFO Created
  session for non-interactive run session_id=c7da7ebd-...`.
- **E5 resume**: `--session c7da7ebd-...` through the server → codeword
  `AMBER-RELATION` returned (memory intact).
- **E6 prompt scoping**: `POST .../current-session` before SSE attach → 409
  `client not attached`; after attach → 200 + `client_session` event.

## Evidence E7 (HTTP envelope, captured 2026-11-01, crush v0.99.0, unix socket)

Full request/response shapes for the M5 surface, captured live against a
mocked provider (recipe: `/tmp/tq-crush-recipe/launch.sh`, scratch stack
`/tmp/tq-crush2`; raw bodies: `messages-envelope.json`, route table:
`openapi.json`).

- **Route authority**: `GET /v1/docs/openapi.json` → 200, 72 paths (v0.99.0).
  This endpoint is the source of truth for the whole surface — no binary
  strings archaeology needed again.
- **`GET /v1/version`** → `{"version":"v0.99.0","commit":"unknown",
  "build_id":"gjdgxs","go_version":"go1.27.1","platform":"linux/amd64"}`.
- **M1 delta (v0.99.0)**: ~~`GET /v1/workspaces` stays `[]` for the whole
  duration of a non-interactive routed `crush run`~~ **CORRECTED 2026-11-01:
  the quiet-listing observation was a false negative — the probe recipe
  (`/tmp/tq-crush-recipe/launch.sh`) omits `CRUSH_CLIENT_SERVER=1`, so the
  observed run was a LOCAL in-process run (which also writes session rows
  to `<repo>/.crush/crush.db`, hence the misleading "executes server-side"
  inference). With the env gate set, the workspace IS listed live for the
  whole run (pinned 4× in `scripts/smoke/crush-server-route.sh`: "live
  workspace listed mid-run", id+path+version). v0.98.2's E1 behavior
  HOLDS on v0.99.0.** Discriminator for future probes: a run without
  `CRUSH_CLIENT_SERVER=1` NEVER connects — `-H` alone is inert for
  `crush run` (silently local, rc 0; see §v0.99 re-verification R4).
- **Workspace registration (the M5 doorway)**: `POST /v1/workspaces` with
  `{"client_id": "<uuid4>", "path": "<repo>"}` → 200 `{"id": "<uuid4>",
  "path", "data_dir": "<repo>/.crush", "version": "v0.99.0", "config":
  {…full resolved config…}}`. Missing/invalid `client_id` → 400
  `{"message":"invalid client_id"}`. A registered workspace over a path
  reads ALL sessions of that project (sessions persist per-project in
  `data_dir`, newest-first) — so an executor-side enricher can register its
  own workspace and read the routed run's session without the run's own
  workspace id. `DELETE /v1/workspaces/{id}` by a different client → 400.
- **`GET /v1/workspaces/{wid}/sessions`** → bare JSON array (no envelope
  object), newest first: `{"id","parent_session_id","title",
  "message_count","prompt_tokens","completion_tokens",
  "summary_message_id","cost","created_at","updated_at","is_busy",
  "attached_clients"}` (timestamps unix SECONDS; `is_busy` true mid-turn;
  `attached_clients` counts live SSE attaches).
- **`GET /v1/workspaces/{wid}/sessions/{sid}/messages`** → bare JSON array
  of messages: `{"parts":[{"type","data"}],"id","role","session_id",
  "model","provider","created_at","updated_at"}`. Observed parts (wire
  shapes, `input`/`metadata` are JSON STRINGS):
  - user: `{"type":"text","data":{"text":<prompt>}}` +
    `{"type":"finish","data":{"reason":"stop","time":0}}`
  - assistant: `{"type":"tool_call","data":{"id":"call_1","name":"bash",
    "input":"{\"command\":…,\"description\":…}","finished":true}}` +
    `{"type":"finish","data":{"reason":"tool_use","time":<unix-sec>}}`
  - tool: `{"type":"tool_result","data":{"tool_call_id","name","content",
    "metadata":"{\"start_time\":<ms>,\"end_time\":<ms>,\"output\":…,
    \"description\":…,\"working_directory\":…}","is_error":false}}` +
    finish `{"reason":"stop","time":0}`
  - a trailing assistant message can carry `parts:[]` while a turn is
    mid-flight (the stall window) — enrichers must tolerate empty parts.
- **Session correlation for an enricher**: match by `created_at` window
  around the run start (seconds granularity) or scan newest-first; the
  `--verbose` `session_id=` log line (E4) remains the exact-match key when
  the executor can parse it.
- DB mirror (per-project `<repo>/.crush/crush.db`, tables `sessions` /
  `messages`: `parts` column holds the same part JSON) — the API is the
  read seam; the DB is the durability fact, not the interface.

## v0.99.0 re-verification (2026-11-01, smoke session — pins ride `scripts/smoke/crush-server-route.sh`)

- **R1 routing + live listing**: `CRUSH_CLIENT_SERVER=1` + `-H <sock>` on
  v0.99.0 behaves exactly as M1 (v0.98.2): the run executes server-side and
  `GET /v1/workspaces` lists the workspace for the whole run. The env gate
  is REQUIRED — see R4.
- **R2 M8 disproof (the E-gap)**: the routed bash tool's env is the SERVER
  process env, full stop. Client env crosses to the workspace struct
  (`wsReq.Env`) but `internal/shell/background.go` constructs the tool
  shell without it and `shell.go` falls back to `os.Environ()` (server).
  `$TQ_RESULT_FILE`/`$TQ_QUESTION_FILE` therefore BREAK under routing.
  Consequence for the executor: env-channel contracts need either an
  upstream fix (bash tool should merge workspace Env, like
  `RunShellCommand` already does at `backend/agent.go:315`) or an
  executor-side move out of env (e.g. baking the result path into the
  payload). The smoke's canary leg flips red the day upstream fixes it.
- **R3 respawn adopts the respawning client's env (M7 amendment)**: a
  client that finds a dead socket auto-spawns a replacement server
  (`ensureServer`) that INHERITS THAT CLIENT'S environment — including its
  `$TQ_RESULT_FILE`. Under the pool this is a CROSS-TASK ENV BLEED vector:
  after a client-led respawn, every server-side bash call could see
  ANOTHER task's result-file path. The pool's keeper (45s dial) exists
  precisely to keep the server from ever idle-dying; the supervisor's
  restarts spawn with the POOL's env (no per-task vars) — only client-led
  respawns bleed. L4 (full routing) must treat any client-respawned server
  as suspect (or have clients connect without per-task env in their own
  environment).
- **R4 `-H` alone never routes**: without `CRUSH_CLIENT_SERVER=1`, `crush
  run -H <socket>` silently executes LOCALLY (rc 0, no connection, no
  error — even with the socket missing). Any probe or seam that "routes"
  must set the env gate or it is proving nothing. (This is also a
  footgun for the executor seam: the env and the arg must travel
  together — `TestAgentCrushServerSeam` pins exactly that pair.)
- **R5 gotchas pinned for future probes** (each cost a debugging round):
  mock models need `context_window` ≥ the real system prompt (~17k tokens
  — 8192 triggers an endless summarize/compact cascade in the
  conversation); `crush dirs` prints on STDERR (a `2>/dev/null` swallow
  reads as "no output"); GET responses on the unix socket are
  chunked-transfer (probe clients must de-chunk); fixed mock ports get
  hijacked by zombie providers from earlier probe stacks (bind port 0);
  a killed `timeout` client leaks its server-side workspace (workspace
  cleanup runs at client exit).
- **R6 L1.8 shipped**: `scripts/smoke/crush-server-route.sh` (7 legs:
  isolation, health, routed verdict round-trip, live workspace + M5
  shapes, env-gap canary, hooks server-side deny, server-kill respawn) —
  COVERAGE.md carries the leg table. Row 1450's pool-integration E-gaps
  (verdict file via the executor channel, question park, 429 evidence)
  stay BLOCKED on R2 upstream.
