# Crush client/server relations — leverage map and migration ladder

**Severity: OPPORTUNITY (architecture decision pending owner)** · **Status:
RESEARCHED + MECHANICS PROVEN 2026-10-30 on installed crush v0.98.2 (all
proofs below are captured, reproducible commands in §Evidence)** · Session
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
| M8 | Workspace creation carries `os.Environ()` server-side | `connectToServer` source (v0.98.2 `internal/cmd/root.go`): `Env: os.Environ()` | `$TQ_RESULT_FILE` / `$TQ_QUESTION_FILE` / `agentEnv` semantics survive |

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
  processes; unmeasured. Measure in an L1 pilot before L4.
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
