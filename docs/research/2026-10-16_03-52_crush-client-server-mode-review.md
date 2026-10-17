# Crush client/server mode — code review for the tq serve-per-repo gate

**Date**: 2026-10-16 03:52 CEST · **Kind**: research review (read-only; no code changed)
**Question**: should the standing client/server-mode-OFF experiment (owner-gated
since 2026-09-14, re-armed by the channels design input
`docs/planning/2026-10-16_01-11_TQ-MCP-CHANNELS-DESIGN-INPUT.md`) be green-lit,
and in what shape?

**Method**: every claim below is read from the crush source (charmbracelet/crush,
fetched via GitHub API 2026-10-16), with the `internal/server` package verified
present at the installed tag **v0.98.1** (host probe: `crush --version`);
host-side facts probed live (`env`, `crush --help`). Files read:
`internal/cmd/{server,root,run}.go`, `internal/server/{server,socket,
socket_classify,endpoints,docs}.go`, `internal/backend/{backend,agent,channels}.go`,
`internal/client/{client,errors,dial_other}.go`,
`internal/workspace/{app_workspace,client_workspace}.go`.

---

## 1. What it is (architecture map)

- **Server**: `crush server` — HTTP/1 + unencrypted HTTP/2 over a Unix socket
  (default `$XDG_RUNTIME_DIR/crush-<uid>.sock`; TCP and Windows npipe also
  supported). Full REST API under `/v1` with OpenAPI 3.1 + human docs served at
  `/v1/docs/` (single source of truth in `endpoints.go`).
- **Backend**: workspaces keyed by **resolved absolute path** (`pathIndex`);
  duplicate creates at the same path dedupe deterministically. Each workspace
  wraps one in-process `app.App` (sessions, agent, MCP, LSP, permissions).
- **Client**: the SAME `crush` binary; `--host` defaults to the socket. Opt-in
  via `CRUSH_CLIENT_SERVER=1` (`useClientServer()`, root.go:235) — **client/
  server is NOT the default**; without the env, every command (incl.
  `crush run`) uses the in-process `AppWorkspace`. **Host probe: env NOT set
  here** — the tq pool fleet still runs process-per-run.
- **Lifecycle (mature, race-hardened)**: client auto-spawns a detached server
  if the socket is absent (`ensureServer`); stale-socket detection + removal;
  **version mismatch → old server shut down, fresh one spawned**
  (`restartIfStale`/`replaceExitingServer`); workspace create grace 30s,
  detach grace 10s, idle shutdown 60s after last workspace (env-tunable:
  `CRUSH_SERVER_IDLE_TIMEOUT`, `CRUSH_SERVER_DETACH_GRACE`); shutdown-vs-create
  race protection (`pending` counter + `closing` latch); retired-client
  semantics; a dedicated `cmd/clientserverrace` harness.

## 2. Concurrency & run semantics (the part tq cares about)

- **One coder agent per workspace** — `ErrAgentBusy` (HTTP 409) on a second
  concurrent message. Per-repo serialization is enforced BY CRUSH, not by tq.
- **Prompts can QUEUE per session** (`QueuedPrompts`/`QueuedPromptsList`/
  `ClearQueue`) — busy-queueing exists natively.
- **Runs are detached from the HTTP connection** (`POST /v1/workspaces/{id}/
  agent` returns 202; "the run survives client disconnects and is only ended
  by the explicit cancel endpoint"). Client crash ≠ run death; server death
  kills the live run, but sessions persist in the DB and a replacement server
  - retry/resume-by-session-id is the documented recovery (`errors.go`).
- **Channels routing is server-owned in server mode** (`backend/channels.go`):
  each push injected into EXACTLY ONE session per opted-in workspace, arrival
  order preserved, delivery concurrent across workspaces, coder agent
  initialized ON DEMAND — "including zero [clients], which keeps the
  'a pushed event is never dropped' contract for a headless `crush serve`."
  Frontends do not inject (no double-delivery).
- **Headless permission/question handling exists as API**: `permissions/grant`,
  `permissions/skip`, `questions/answer`, `questions/cancel` — programmable
  mid-run, which the subprocess mode cannot do (today: `--yolo` or nothing).

## 3. Security posture

- **No auth middleware anywhere** — the API trusts whoever holds the socket.
  Unix default = per-user runtime dir (0700) → OS-enforced same-user boundary.
  Acceptable for the pool (all crush instances run as the same user).
- **TCP mode has NO auth** — must never bind beyond loopback, ever. Treat
  `tcp://` as forbidden in any tq-managed configuration.

## 4. What tq would actually gain, per deployment shape

**A. Env flip only (`CRUSH_CLIENT_SERVER=1` on the pool)** — server process,
config/catalog/db loading amortized while it lives; but every `crush run`
still CREATES + DELETES its workspace, and MCP/LSP init happens per workspace
at agent init → the original MCP-init-amortization goal is only PARTIALLY met,
and with 5-min pool ticks the 60s idle shutdown kills the server between
ticks anyway (raise `CRUSH_SERVER_IDLE_TIMEOUT` or accept respawn per tick).
Cheap to trial, small payoff.

**B. Channels push (the phase-2 design)** — a persistently hosted workspace
per repo (channels keep the agent alive on demand; tq pushes work; replies
route back via `channel_reply`). This is where MCP/LSP warm-ness, busy-
queueing, exactly-once, and zero-client headlessness all land together.
BUT: a workspace lives while a client holds it (SSE = refcount; detach grace
10s) — **a headless always-on workspace needs a tiny holder client** (or a
documented server-side keep-alive) or pushes route into a workspace that has
been torn down. This is THE operational gap the pilot must solve.

**C. Direct API orchestration (new option this review surfaces)** — tq's
executor could skip the `crush run` subprocess entirely: create workspace,
`agent/init` non-interactive, create/resume session, `POST agent` (202
detached), stream SSE for completion, answer permissions/questions
programmatically. Kills stdout-parsing, argv-quoting, and the ETXTBSY class;
gives tq structured events instead of scraped tails. Largest integration
surface change; most aligned with "communication should not be shit".

## 5. Risks / gaps

1. **Experimental env gate**: `CRUSH_CLIENT_SERVER` is an unset-default
   experiment; semantics may shift across releases (host churn 0.94→0.98 in
   one month). Pin the pool's crush version per experiment window.
2. **Server = single point of failure** for live runs (crash kills in-flight
   agents; sessions survive, runs don't). tq's lease/reclaim machinery
   already models this — treat server death like worker death.
3. **Workspace teardown between runs** (per-invocation create/delete) —
   amortization claims must be measured, not assumed (see A vs B).
4. **Same-user trust boundary** — fine for the pool; do not expose TCP.
5. **Version negotiation is replace-on-mismatch** — a NEW client binary
   silently restarts the server, killing live workspaces' hosts. Pool
   upgrades must drain first (or tolerate the kill; tq re-dispatches).

## 6. Verdict

**Green-light the experiment — it's no longer speculative.** The architecture
is complete, race-hardened, e2e-tested at the installed tag, and the exact
semantics the 2026-09-14 assessment called "unverified" (concurrent headless
runs, session resume, MCP amortization) are now answerable from shipped code:
one-agent-per-workspace serialization, detached 202 runs, durable sessions,
per-workspace MCP. Recommended pilot ladder:

1. **Scratch repo, shape A** (env flip) — measure: server lifetime vs 5-min
   ticks, MCP/LSP re-init cost per run (workspace create/delete), run-latency
   delta vs in-process. Days, reversible, zero pool risk.
2. **Scratch repo, shape B holder** — solve the persistent-workspace gap
   (tiny SSE holder client), verify the channels never-dropped contract with
   zero interactive clients, measure warm-agent dispatch latency.
3. **Dogfood repo** (this one) only after 1–2 pass — then wire the phase-2
   channel rows on top.

## 7. Open questions (owner)

1. Pilot ladder OK — scratch repo first, this repo last?
2. Shape C (direct API orchestration) — park as a design note now, or fold
   into the phase-2 channels row as the executor-side integration story?
3. Pool crush-version pinning policy for experiment windows (replace-on-
   mismatch restarts servers under live workspaces).
