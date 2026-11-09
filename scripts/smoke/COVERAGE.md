# Smoke coverage index — leg → assertion map

Answers "which surface has no E2E" without grep (minted from
docs/status/2026-09-20_06-45 §f20). Keep it current when a leg grows:
one row per leg, naming the surface, the key assertions, and the output
stream each assertion reads.

## `tq session` bridge + `tq crush` wrapper (`session-close.sh`)

| Leg                        | Surface                | Key assertions                                                                                                                        | Stream                  |
| -------------------------- | ---------------------- | ------------------------------------------------------------------------------------------------------------------------------------- | ----------------------- |
| build/seed                 | —                      | binary builds or `TQ_BIN` used; footer commit seeded                                                                                  | —                       |
| `session begin`            | journal fact           | "session $SID opened"                                                                                                                 | stdout                  |
| `session ping` ×2          | session registry       | 2 appended JSONL observations, cwd recorded, journal fact count unchanged (registry-only contract)                                    | file (`--registry`)     |
| `session close`            | attribution + mint     | "1 attributed commit"; review + status enqueued; exactly 1 of each by type                                                            | stdout                  |
| second close               | dedup/replay           | "known (dedup)"; task counts unchanged; 1 `session.opened`, 2 `session.closed` facts                                                  | stdout                  |
| `tq crush` no-id           | wrapper passthrough    | child exit 7 preserved; child stdout forwarded; "no session id" note; DB never created                                                | stdout + stderr         |
| `tq crush` committed       | id scan + close + mint | child exit 5 preserved; "1 attributed commit"; review + status minted; 1 `session.closed` fact in the leg's own `--db`                | stdout                  |
| payload contract           | minted payload JSON    | `"yolo": true`, no `"model":`, `"require_clean": true` in BOTH minted payloads                                                        | stdout (`tasks --json`) |
| `tq crush --allow-dirty`   | RequireClean mirror    | child exit 6 preserved; both minted payloads carry `"require_clean": false`                                                           | stdout                  |
| `tq crush` zero-attributed | footer-missing branch  | child exit 3 preserved; "no commits carry … footer" + "nothing to review or report"; zero review/status tasks in the leg's own `--db` | stdout                  |

Deliberately NOT covered at smoke level (unit-pinned in `internal/session`
/ `cmd/tq`): `$CRUSH_SESSION_ID` env-carried id source, child killed by
signal (`exec.ExitError`, code −1).

## Fleet inventory

| Smoke                     | Contract area                                 |
| ------------------------- | --------------------------------------------- |
| `api.sh`                  | machine API: token gate, lockout, POST /tasks |
| `archive-evidence.sh`     | evidence-archive helper rails                 |
| `bootstrap-install.sh`    | `tq bootstrap` install path                   |
| `consumer-install.sh`      | journal consumer wiring                       |
| `crush-pool-supervisor.sh` | routed 429 fast-refuse + pool keeper/supervisor (table below) |
| `crush-server-route.sh`    | crush client/server route wire legs (table below) |
| `crush-server-memory.sh`   | L4 shared-server memory measurement: 2 waves x 4 concurrent routed turns (mock provider, ONE repo, ONE server), RSS sampler baseline/peak/quiescent per wave + 3x-baseline accumulation guard; M7 idle-exit races healed by a keeper-shaped respawn + one turn retry |
| `dep-sweep.sh`             | `--dep-sweep` sweeper turn                    |
| `dlq-evidence-stage.sh`   | `tq dlq --evidence-stage` filter vs real DLQ  |
| `doctor-hygiene.sh`       | `tq doctor` hygiene probes                    |
| `dogfood-once.sh`         | one self-hosted pool tick                     |
| `fullcore.sh`             | core enqueue → claim → execute → done loop    |
| `help-text.sh`            | CLI help surface                              |
| `journal-drift.sh`        | journal vs store drift detection              |
| `legacy-serve-upgrade.sh` | pre-flip DB auto-upgrade on open              |
| `loop-detector.sh`        | requeue-loop detection                        |
| `multi-repo.sh`           | multi-project claiming                        |
| `new-module.sh`           | module scaffold + mirror-clone gate           |
| `papdashboard-e2e.sh`     | dead-letter alert bridge                      |
| `postgres-conform.sh`     | scratch postgres cluster + conform suite      |
| `proxy-wait.sh`           | module-proxy readiness                        |
| `questions-e2e.sh`        | `tq ask` park/answer routing                  |
| `ratelimit-e2e.sh`        | 429 requeue-without-burn                      |
| `readme-install.sh`       | README install instructions                   |
| `redaction.sh`            | secrets redaction in output tails             |
| `release-gates.sh`        | release gate battery                          |
| `reviews.sh`              | review executor + verdict gate                |
| `session-close.sh`        | session bridge + crush wrapper (table above)  |
| `status-rewind.sh`        | status sweeper rewind-replay done-guard       |
| `status-loop.sh`          | status sweeper turn                           |
| `webui.sh`                | serve dashboard + SSE                         |
| `worktree.sh`             | `--worktrees` agent branches                  |

## `crush-server-route.sh` — the L1 client/server seam's wire legs (2026-11-01)

Boots an isolated `crush server` (XDG-redirected scratch config, mock
OpenAI-compatible provider, zero spend) and routes real `crush run`
clients through it. Asserts the wire contract the `--crush-server` pool
flag depends on (planning doc §L1; TODO row 1450).

| Leg                 | Surface                       | Key assertions                                                                                                                 | Stream                        |
| ------------------- | ----------------------------- | ------------------------------------------------------------------------------------------------------------------------------ | ----------------------------- |
| isolation gate      | `crush dirs` + config parse   | first dirs line = scratch config dir; crush.json parses with `disable_default_providers` — no run before the gate                | stderr (dirs prints there!)   |
| boot + health       | server + `/v1/health`         | 200 on the scratch unix socket                                                                                                 | unix socket HTTP              |
| routed verdict run  | `CRUSH_CLIENT_SERVER=1` + `-H`| rc 0; `Created session … session_id=` line (M4); every provider call on the mock (request log = zero-spend receipt); tool-call wrote the verdict file (M1) | run log + mock request log + file |
| live workspace + M5 | `/v1/workspaces[/{id}/sessions\|/messages]` | workspace listed DURING the run (the routing discriminator — local runs never appear); session keys (id/title/message_count/is_busy); message part types (text/tool_call/tool_result/finish) | unix socket HTTP (de-chunked) |
| env-gap canary      | tool exec env, two-sided      | server env var VISIBLE in the routed bash tool; client `$TQ_RESULT_FILE` EMPTY — v0.99.0 does not forward workspace Env to tool exec (`internal/shell/background.go` builds the shell without it); flips red when upstream fixes it | files written by the tool     |
| verdict bridge      | `tq verdict --file` (real binary) | bash tool runs the REAL tq with `--file <abs-path> '<json>'`; the verdict file lands with ZERO env dependency (the executor rail's command; stays green even after an upstream env fix) | bridge verdict file           |
| hooks server-side   | repo `.crushrc` PreToolUse    | hook fired (its own log line); exit-2 deny BLOCKED the marker command (the file it would write is absent) — M3, deny contract per runner.go exit 2 | hook log + side-effect file   |
| server-kill respawn | `ensureServer` (M7)           | kill -9 the server; the next routed client respawns it, run rc 0, health 200 after                                              | process + socket              |

Deliberately NOT covered at smoke level: the question-parking E-gap
(`$TQ_QUESTION_FILE` under routed runs) — BLOCKED on the env gap above
upstream; unit-pinned in-process (ratelimit-e2e stub class) until then.
The verdict E-gap IS covered twice — the plain tool-call write (routed
verdict run leg) AND the flag channel (verdict bridge leg, real
`tq verdict --file`).
The 429 E-gap IS covered — `crush-pool-supervisor.sh` below (the
rate-limit signature rides the agent-stream error text, not the env
channel, so it is env-gap-independent). Also not covered:
`--session` resume through the server (M4's second half,
executor-pinned via ExtractSessionID) and SSE events attach (M2/M6,
webui unit tests with a fake server).

## `crush-pool-supervisor.sh` — the pool-side E-gap legs (2026-11-01)

Same isolation rails as the sibling smoke (XDG scratch config, mock
provider, `crush dirs` gate, size-guard); needs the tq binary (TQ_BIN
or build-tq.sh) for the keeper leg.

| Leg                    | Surface                                            | Key assertions                                                                                                                                                                          | Stream                   |
| ---------------------- | -------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------ |
| routed 429 fast-refuse | `CRUSH_CLIENT_SERVER=1` + `-H` (agentEnv argv) + `MOCK_429` provider | rc != 0; output carries the rate-limit error text (`rate limit exceeded …`) DetectRateLimit classifies — routed runs surface ONLY the final Agent error (no WARN retry lines; those are local-run client-side calls) | run log                  |
| keeper/supervisor      | `tq agent-pool --crush-server` actor               | "listening on" line; kill -9 the server child → "exited unexpectedly … restart" line + NEW server pid; SIGTERM pool → clean exit, server torn down (never orphaned) — the row-1449 ladder, live | pool log + process table |
| mid-run kill (V2.10)   | executor classification + worker requeue, live     | marker-gated kill -9 INSIDE the mock's stalled tool-result turn → severed client → pool WARN "crush server died around the routed run"; task PENDING with attempts 0 and `tq show` carrying `[crush-server]`; supervisor "restart 2" rides the same death. Two races pinned: the marker rm must precede the item commit (claim-wake arms the stall <1s after enqueue — a later rm erases the marker), and routed turns only classify in `--verbose` (quiet suppresses the WARN; run11 lesson) | pool log + tasks/show JSON |
| routed verdict bridge (V2.18) | prompt rail → server-side bash → `tq verdict --file` → facts | the mock regexes the rail's per-run path out of the baked prompt and scripts the REAL `tq verdict --file <path> '<json>'` as the bash tool call (tq rides the pool PATH into the server env — the M8 workaround shape); the task completes and `tq show` carries the verdict note — the full bridge, in-pool, zero spend | task show JSON |
| operator cancel (V2.15) | cooperative cancel + server-side reap, live        | a second stalled routed turn; `tq cancel --force` → the worker's heartbeat cancels the run context (pool runs `--lease 20s` → 5s heartbeats; the 5m default lease makes cancel observation slower than a smoke window); pool logs "task cancelled by operator request"; the routed client process exits; once the held turn drains NO session over the repo stays busy (the server reaped the cancelled client's turn); task cancelled | pool log + process table + busy probe |

Not covered here: the env-gap canary + wire shapes (sibling smoke owns
them); the verdict/question env channels stay upstream-blocked per its
canary.

## `--db` flag-placement audit (2026-10-24)

Every DB-backed subcommand takes `--db PATH` as a flag of its OWN
subcommand (after the subcommand word); no command accepts a global
pre-subcommand `--db`. DB-free surfaces (`session ping` — registry-only,
`verdict` — env-file, `ask`) take no `--db` at all. The smoke legs copy
the fresh-DB-per-leg pattern (dedicated `--db` per counting leg) so mint
counts stay exact. Scripted callers: put `--db` after the subcommand,
always.

## Output-stream assumptions (2026-09-20_06-45 §e4 sweep)

`session-close.sh` annotations name the stream per assertion; the map:
`runSessionClose`/close summaries and child passthrough → stdout; the
wrapper's no-id note → stderr. Other multi-leg smokes with redirected
legs (`ratelimit-e2e.sh`, `questions-e2e.sh`, `webui.sh`) assert against
captured files per leg — no cross-stream grep found in the sweep.
Future legs: grep the `fmt.Printf` vs `fmt.Fprintln(os.Stderr` map of
the code under test BEFORE writing the assertion (§e1 lesson).

## Scratch-repo factory + session-start pin (2026-11-09)

| Leg                                  | Surface                              | Key assertions                                                                                                                                                                                              | Stream  |
| ------------------------------------ | ------------------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------- |
| `_lib.sh` (sourced; `--self-test`)   | scratch-repo factory for repo-surface smokes | fail exits 1; pass prints `ok: …`; `new_repo` yields an init-committed repo with smoke identity; `run_in` executes in-repo; bootstrap-install/release-gates deliberately exempt (flag-only git identity is a load-bearing premise) | —       |
| `session-start.sh`                   | `scripts/session-start.sh` WITH-ARGS branch | hermetic ID-args path via grep-backed `rg` + `tq` PATH stubs: prior-reports lookup, verdict surfacing, DONE-row detection, footer scan, queue-record block; degraded-path messages for absent check-ci/hooks/docs asserted | stdout  |
