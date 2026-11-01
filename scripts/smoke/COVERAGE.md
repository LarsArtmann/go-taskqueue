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
| `crush-server-route.sh`    | crush client/server route wire legs (table below) |
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
| hooks server-side   | repo `.crushrc` PreToolUse    | hook fired (its own log line); exit-2 deny BLOCKED the marker command (the file it would write is absent) — M3, deny contract per runner.go exit 2 | hook log + side-effect file   |
| server-kill respawn | `ensureServer` (M7)           | kill -9 the server; the next routed client respawns it, run rc 0, health 200 after                                              | process + socket              |

Deliberately NOT covered at smoke level: the pool-integration E-gaps
(verdict file via the executor's `$TQ_RESULT_FILE` channel, question
parking, 429 evidence under routed runs) — BLOCKED on the env gap above
upstream; unit-pinned in-process (ratelimit-e2e stub class) until then.
Also not covered: `--session` resume through the server (M4's second half,
executor-pinned via ExtractSessionID) and SSE events attach (M2/M6,
webui unit tests with a fake server).

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
