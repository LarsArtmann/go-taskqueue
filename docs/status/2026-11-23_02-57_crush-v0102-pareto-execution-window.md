# crush v0.102.0 pareto execution — window 1 status (plan 00-57)

**Date**: 2026-11-23 02:57 CET. **Session**: interactive owner directive —
"GET SHIT DONE, the WHOLE TODO LIST" over
`docs/planning/2026-11-23_00-57_SUPERB-crush-v0102-upgrade-channels-unlock.md`.
**Inputs**: research note 2026-11-23_00-32 (17 tag-verified citations),
status report 2026-11-23_00-38, host state evo-x2 (crush v0.101.0 via
NUR pin). Verified at HEAD b766d60c / SystemNix ded2dcea this window.

## a) FULLY DONE (verified this window)

| # | Item | Receipt |
| - | ---- | ------- |
| 1 | **Plan file shipped + whole research window PUSHED** (18 commits, `5e659eae..e8bc7058`): research note, TODO rows, AGENTS channels bullet, 00-38 report, plan file (landed via daemon fold `30eb6e08`, 17989 B verified intact; left unhealed — rewriting mid-stack below a foreign commit would orphan `970c616c`'s just-landed cites) | push rc=0, origin at e8bc7058 |
| 2 | **M1 — crushdeath v0.102.0 markers**: `lost connection to the crush server` + `agent run was cancelled on the server` added to `crushServerDeathMarkers` (internal/executor/crushdeath.go), both-sided tests incl. verbose-completion negative (crushdeath_test.go). Executor module gate GREEN (`go test ./... -count=1`, 93.5 s, GOWORK=off jsonv2) | landed via daemon sweeps `96eac6b5` (impl) + `f893a14f` (tests) — cited by foreign `970c616c`; PUSHED |
| 3 | **M2 — keeper idle disarm**: `CRUSH_SERVER_IDLE_TIMEOUT=43200` in `runCrushServerActor` spawn env; `crushKeeperInterval`/keeper/actor comments repaired (GET holds nothing on v0.102.0 idle model — probe, not keepalive); stub-server env-pin test `TestRunCrushServerActorDisarmsIdleShutdown` PASS (1.01 s, cmd/tq via go.work) | attributed commit `9ba9a30a` (amended from daemon fold, guarded by rev-parse); PUSHED |
| 4 | **M11 — hygiene A**: `check-doc-refs.sh` rc=0 (the vendor-window cite healed by the foreign vendor-retirement window `153d357d` — resolved-by-foreign, no action); research-note-template.md gained the upstream-cite example row (`` `repo@tag <path>` (lines N-M) `` vs gate-rejected `path:line`) with a real citation example | commit `0eb3b616` (rebased from b611acdf by a concurrent fold); PUSHED |
| 5 | **M6+M7 core — `internal/mcpserver` module GREEN**: hand-rolled JSON-RPC 2.0 stdio server (jsonrpc.go, server.go: initialize negotiation over go-sdk v1.8.0's revision set {2025-11-25, 2025-06-18, 2025-03-26} verified from module cache, ping, tools/list, tools/call, notifications silent, parse/-32601/-32602 error shapes, in-band isError tool errors); read tools task_list / task_show / queue_stats / dlq_list over a minimal local `Store` seam (Get/List/FactsForTask/StatusCounts/ProjectCounts — `*sqlite.Store` satisfies it); queue_stats rides `task.StatusCountsView` (the shared httpapi/webui wire contract); tests: full pipe-harness conversation + golden shape pins + pushdown filters + error-class pins. Module gate: vet + tests GREEN, gofmt clean. Scaffolded via `new-module.sh` (+dependabot entry) | module test run this window; files landed via daemon sweep `b766d60c` (partial — see b) |
| 6 | **M3 build half**: SystemNix NUR input bumped `bfe0696e` (2026-11-18) → `ded2dcea` (2026-11-23); `nix eval` confirms NUR carries **crush 0.102.0**; evo-x2 toplevel `nix build` GREEN. Undeployed SystemNix delta verified benign (forgejo log-level info→debug + docs) | flake.lock diff + eval output "0.102.0" |

## b) PARTIALLY DONE

| # | Item | What remains |
| - | ---- | ------------ |
| 1 | **M3 host flip** (the 1% tier's unlock act): NUR bump + toplevel build done; **switch NOT run** — two `sudo nix run .#deploy` attempts died during the `project-discovery-daemon` git+ssh fetch on a github.com DNS flap (see §d3). The deploy app's eval pulls perSystem packages (the raw toplevel build did not — lesson) | one retry with DNS-warmed prefetch in the same command; then `crush --version` receipt + F12-F17 (doctor pin bump, smokes) in the same change |
| 2 | **M6/M7 wiring**: root go.mod require+replace, cmd/tq/go.mod require, cmd/tq/go.work use — all applied; `cmd/tq/mcp.go` written (cmdMCP, explicit `--db` chain, stdin/stdout serve). **Dispatch-table registration NOT applied** (insertion point found: cmd/tq/main.go:151 next to `"park"`); cmd/tq module gate NOT yet run; CHANGELOG row not written | one edit + `scripts/test-cmd-tq.sh` + scoped gofmt |
| 3 | **TODO_LIST ticks**: the four 2026-11-23 upgrade rows (markers, keeper, host+pin, probe) are landed-but-unticked for M1/M2 (M3/M5 rows correctly still open) | F51-F53 at closeout, tick-claims citing `9ba9a30a` / `96eac6b5`+`f893a14f` |

## c) NOT STARTED (plan order preserved)

- **M4** observe-first fail-on-drop stderr pin (blocked on host 0.102.0 binary; F18-F19).
- **M5** live channels-capability probe (F20-F23; same block).
- **M8** park_task write tool — owner-gated by the plan (open gate 2); never self-scheduled.
- **M9** bootstrap mcp-block emission + doctor mcp-present WARN (F39-F41).
- **M10** doctor channels-capability probe + `channel_enabled`/`channel_reply` emission design ruling (F42-F43).
- **M12** post-upgrade drift checks (env-gap canary legs, `is_busy` wideness; F46-F47).
- **M13** DOMAIN_LANGUAGE truth rows (Routed run server-side-cancel, Keeper idle knob; F48-F50).
- **M14** closeout battery + TICK-AUDIT + report + push (F51-F58).
- doctorCrushProvenVersion `0.99.0`→`0.102.0` (F16; same-change-coupled to the flip per doctor.go comment contract).

## d) TOTALLY FUCKED UP (honest)

1. **Pipe-rc lie**: the first push attempt captured rc from `tail`, printed `push-rc=0` while the push FAILED (hook refusal). The rc-to-file convention exists for exactly this; I piped anyway and could have believed the window shipped.
2. **Commit-after-gate race — the Known Issue I re-read and still tripped**: ran the 93 s executor gate BETWEEN edit and commit; the daemon folded M1 into TWO sweeps and M2 into a third before my `git commit` ran (both commits failed with "nothing to commit"). AGENTS.md says stage+commit BEFORE anything. Recovered: M2 via guarded tip-amend (`9ba9a30a`); M1 left as sweep-folds (healing them = history surgery below foreign `970c616c` + orphaning its cites — declined, documented). Cost: M1 has no attributed message.
3. **Deploy DNS misdiagnosis**: called the first failure "negative-cache poisoning, flushed it, fixed" — `resolvectl flush-caches` later proved a NO-OP (no systemd-resolved manager on this host); the healing was TTL expiry coincidence, and the retry died identically. Root cause: upstream DNS flap (github.com A-record intermittently NXDOMAIN while api.github.com resolves). Real fix is warm-prefetch + deploy in ONE command, not resolver surgery.
4. **Golden-first instead of observe-first**: wrote TestTaskListGoldenShape's `want` from memory of the shape — two drift iterations (missing `attempts`/`maxAttempts` zero fields; byte-vs-rune `len()` on a UTF-8 ellipsis) that one captured-output read would have prevented. The repo's observe-first rule applies to goldens too.
5. **Phantom import in mcp.go**: drafted `internal/sqlite` + a lint-guard hack without checking the real path (`internal/queue/sqlite`, unused anyway). One read of main.go's imports first = zero wasted edit.
6. **go.mod wiring via python heredoc** instead of the edit tool — worked, but the hook advisories (daemon-food class) apply; surgical edits were the right tool.

## e) WHAT WE SHOULD IMPROVE (process, from §d)

- **add+commit in ONE tool call, before ANY gate** — the 60 s daemon window is not survivable any other way; the M1 loss proves it again.
- **rc files, never pipes** — unify: `cmd > log 2>&1; echo rc=$? > rc.file` with no intermediate pipe.
- **Deploy playbook for private inputs**: `git ls-remote` warm + `nix flake prefetch` + deploy chained in one shell; DNS flap then has no window to hit.
- **Goldens from captured output** (run the handler, print, pin) — same doctrine as marker strings.
- **Import-path check before writing new command files** (grep main.go imports first).
- The push-timing hook treadmill (fresh foreign sweeps refusing pushes) is a standing tax on every window; worth a TODO row (heal-then-push batch or an allowlist for attributed tips).

## f) NEXT — up to 50 (plan residue first, then noticed)

**Plan residue (M-then-F order):**
1. Register `"mcp": cmdMCP` in cmd/tq/main.go dispatch (insertion at :151).
2. Run `scripts/test-cmd-tq.sh` (cmd/tq module gate incl. windows cross-compile).
3. Scoped gofmt over cmd/tq/mcp.go + commit the tq-mcp wiring as ONE change (module + wiring + command).
4. CHANGELOG `[Unreleased]` row for `tq mcp` (operator-visible: new MCP server) — O14 rules.
5. ADR-0016 check: mcpserver needs NO facade (internal-only) — confirm in the commit message, not a new ADR.
6. Retry M3: DNS-poll → warm-prefetch project-discovery-daemon → `sudo nix run .#deploy` (one chain).
7. F12 `crush --version` → v0.102.0 receipt; record SystemNix commit for the NUR bump.
8. F13 `scripts/smoke/crush-server-route.sh` with TQ_DB=scratch; capture output file.
9. F14 triage route-smoke legs vs 0.102.0 (fail-on-drop rc≠0, viewer self-report).
10. F15 `scripts/smoke/crush-pool-supervisor.sh`; capture output.
11. F16 bump `doctorCrushProvenVersion` → "0.102.0" (same change as receipts land).
12. F17 `tq doctor` run → no proven-version WARN receipt.
13. F18 scratch socket + routed run + kill -9 server mid-run; capture combined stderr.
14. F19 diff observed lines vs pinned markers; amend tests if prefixes differ (observe-first close).
15. F20 write the channels probe server (~30 lines, stdio JSON-RPC echoing `experimental:{"claude/channel":{}}`).
16. F21 wire probe into scratch crushrc (`mcp add` + `--channel true`), `crush run --verbose`.
17. F22 emit one `notifications/claude/channel` push; capture whether `<channel …>` renders.
18. F23 record verdict in TODO row + plan-doc errata.
19. F24-F30 residue: none — module home decided (internal/mcpserver); store seam pinned.
20. F31-F36 residue: task_list/task_show/queue_stats/dlq_list + shape pins — DONE in the module (re-verify at M14).
21. F39 bootstrap mcp-block emission (managed .crushrc block, explicit --db, flag-gated).
22. F40 bootstrap golden test (renders + idempotent re-run).
23. F41 doctor WARN when mcp block absent on pool repos.
24. F42 doctor channels-probe design note in plan doc (signal + WARN shape).
25. F43 channel_enabled/channel_reply emission ruling: official `mcp add --channel`/`--channel-reply` vs raw keys.
26. F44-F45 residue: DONE (docrefs green; template row landed).
27. F46 route-smoke env-gap canary leg re-read vs 0.102.0 (SetCurrentSession interplay).
28. F47 `is_busy` wideness vs close-out fact asserts.
29. F48 DOMAIN_LANGUAGE Routed run: interrupted runs cancelled server-side (5 s budget).
30. F49 DOMAIN_LANGUAGE Keeper: idle knob; remove poll-keeps-alive phrasing.
31. F50 DOMAIN_LANGUAGE: mcp/verdict rows if the mcp surface changed the rails (deferred to post-M9 ruling).
32. F51 tick M1/M2 TODO rows citing `9ba9a30a` + the sweep-folds.
33. F52 tick M3/M4 rows after F12-F17 receipts.
34. F53 tick M5 row with the F22-F23 verdict.
35. F54 ci-local battery on final tree (rc to file).
36. F55 TICK-AUDIT: grep TODO_LIST for every batch anchor; landed rows read `- [x]`.
37. F56 closeout report + status-index row.
38. F57 `git log -1` trailer re-read after the final commit (daemon-fold heal if empty).
39. F58 final push.
40. AGENTS.md channels bullet: flip "host runs v0.101.0" fact once the flip lands (it currently says TAG-verified; host state changes).
41. Plan-doc errata section: record the sweep-fold SHAs that carried M1 (M1's attributed-message loss).
42. Push-hook treadmill TODO row (fresh-sweep refusals vs attributed tips) — new row, dedup-grepped at mint.

**Noticed in passing (not plan, cheap):**
43. `/tmp/crush-src` blobless clone is tag-only and file-less — a worktree add would make future verification windows cheaper.
44. `paralleltest`/`wsl_v5` gopls advisories fire on the new module (repo style ignores them — baseline OK).
45. mcpserver tests intentionally lack t.Parallel() matching repo style — fine, noted so nobody "fixes" it.
46. The `Index` in tools_test stubStore records filters — extend if F46/F47 want canary pins.

## g) OWNER QUESTIONS (cannot self-resolve)

1. **Host flip green light?** `sudo nix run .#deploy` restarts services (post-deploy checks) on a box currently running concurrent agent windows + this session; the alternative is `deploy-when-quiescent.sh` (waits for quiet — may never come while the pool dispatches). Proceed hot, or park until a quiet window?
2. **park_task (M8)**: rule it into `tq mcp` v1 or defer? (Plan open gate 2; write-tool surface: explicit task_id+duration+reason, running refused.)
3. **M1's sweep-fold commits** (`96eac6b5`/`f893a14f`, no attributed message, cited by foreign `970c616c`): leave as-is (my call — healing = surgery below a foreign commit), or pay the rewrite + cite-repair anyway?

fresh:skip — §f items 1-42 are the plan's own rows (TODO_LIST already carries the four upgrade rows + the standing tq-mcp rows; re-minting here would fork them); items 43-46 are session observations, minted only on owner instruction.
