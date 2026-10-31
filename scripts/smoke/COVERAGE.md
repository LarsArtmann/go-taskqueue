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
| `consumer-install.sh`     | journal consumer wiring                       |
| `dep-sweep.sh`            | `--dep-sweep` sweeper turn                    |
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
