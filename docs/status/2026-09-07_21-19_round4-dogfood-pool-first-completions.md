# Status Report — Round 4 Dogfood: the Pool Ate Its First TODO Items (2 completions, 1 preflight save, LAN dashboard live)

**Date:** 2026-09-07 21:19 CEST
**Session scope:** Executed the round-4 plan
(`docs/planning/2026-09-07_20-47_SUPERB-PLAN-ROUND4-DOGFOOD-POOL-EATS-THIS-REPO.md`):
dogfooded go-taskqueue on itself — rails (`.crushrc`, `.tq-verify`), plan doc,
launch of a budget-capped `agent-pool --repos ~/projects/go-taskqueue`, LAN
exposure of `tq serve`, and observation of the first autonomous completions.
**Format note:** skill default is a styled HTML report; the owner explicitly
requested `.md`, so this file is Markdown. Flagged per skill spec, not
propagated as a new default.
**Verification state at report time:** pool RUNNING (background shell), serve
RUNNING on `0.0.0.0:8090`, `./tq stats`: 2 completed / 1 running / 0 dead /
0 pending; journal has 10 facts; TODO_LIST.md: 21 open / 2 ticked-by-agents;
working tree clean; both agent commits inspected at source level.

---

## a) FULLY DONE

| Work | Evidence |
| ---- | -------- |
| Round-4 plan written: Pareto tiers, 24 coarse tasks (ALL 22 TODO items mapped to an owner), ~100 fine tasks, mermaid execution graph, launch command, guardrails | `docs/planning/2026-09-07_20-47_SUPERB-PLAN-ROUND4-DOGFOOD-POOL-EATS-THIS-REPO.md` |
| Dogfood rails: `.crushrc` = `permissions allow view ls grep glob edit write bash` (README minimum); `.tq-verify` = build + vet + race tests + gofmt (the CI hard gates) | both files committed, tracked |
| ROADMAP Open question #1 ANSWERED inline (pool on this repo — the owner's call), AGENTS.md dogfood known-issue added (rails, launch pointer, stop/rescue) | ROADMAP.md:137-142, AGENTS.md known issues |
| Pushed with authorization: 9 accumulated daemon commits + rails + plan + AGENTS note → origin/master `99f8031` | `git ls-remote` = HEAD at push time |
| Pool launched with the full rail set: `--yolo --project-exclusive --concurrency 2 --interval 5m --task-timeout 45m --max-per-tick 3 --daily-budget 15 --repo-interval go-taskqueue=10m --dlq-backoff 30m` | startup line: "2 agent(s) … (yolo=true, dirty=false, exclusive=true, harvest every 5m0s, verify enforced)" + autonomy WARNING |
| Harvest semantics proven on the real backlog: first tick = 22 items → 1 enqueued (per-repo pacing), 19 paced, **2 blocked** (v0.2.0, CQA — the BLOCKED marker enforced by the code shipped this morning) | pool log + `harvest --dry-run` (22 open, 1 would enqueue) |
| **Autonomous completion #1** (TODO #1, the Critical item): `ci-local.sh` — a real crush agent worked 9 min (20:56:59 → 21:05:58), `.tq-verify` (race suite) passed, checkbox ticked, commit `115854a` "Add ci-local.sh as the one-command pre-push gate replicating CI" — agents never pushed | journal facts 1-3; `git show 115854a` |
| **Autonomous completion #2** (TODO #2, this morning's filed regression): papdashboard `startWatermark` ctx guard + a 27-line pre-cancelled-ctx test + TODO tick — commit `aaabe1c`, ~4 min agent work (21:08:59 → 21:12:41) | journal facts 4-8; `git show aaabe1c` (guard verified in source: `ctx.Err() != nil → return nil`) |
| **Preflight dirty-tree guard proven live**: task 2 claimed at 21:06:59 hit MY uncommitted plan doc → `task.requeued` WITHOUT attempt burn → reclaimed at 21:08:59 after the daemon committed → completed. The at-least-once/no-burn contract worked on its first real trigger | journal fact 6 (reason names the exact file) |
| Agent-work review (both commits inspected): ci-local.sh faithfully replicates ci.yml order incl. advisory lint + staged-tree nix check; ctx fix is exactly the filed guard + test | `git show` both |
| **LAN dashboard live**: `tq serve --addr 0.0.0.0:8090` (replacing the loopback instance), verified via `http://192.168.1.150:8090/api/stats` returning live JSON | fetch + serve startup line |
| TODO_LIST routing kept honest: new W16 item added (serve auth for non-localhost binds — now actually needed) | TODO_LIST.md (21 open / 2 done) |

## b) PARTIALLY DONE

| Work | What works | What remains |
| ---- | ---------- | ------------ |
| The campaign | 2 of 20 unblocked items done, 1 running (claimed 21:16:58), budget spent 2/15 | 17 items left; ~2 days at the current budget pacing |
| `ci-local.sh` (agent-authored) | Written, reviewed, faithful to ci.yml; `git add -A` + stray-check before nix matches the item's intent | **Never executed end-to-end by me** (deliberate: it races the pool's own verify runs and takes minutes); its `git add -A` leaves changes STAGED → the tree reads "dirty" to agent preflight until the daemon commits — could cause requeue cycles right after a run |
| LAN exposure | Serving on `0.0.0.0:8090`, live stats verified | Reachability from a SECOND device unproven (host-local fetch may bypass NixOS firewall rules); zero auth — anyone on the LAN sees all payloads/error tails; W16 routed but unbuilt |
| Post-run review (plan C22) | Started: both agent diffs inspected, scope clean so far | `.crushrc` self-modification monitoring, agent-quality pass over future diffs, budget telemetry — not institutionalized |
| Push state | origin = `99f8031` (my push) | The 2 agent commits + daemon blobs since are UNPUSHED (this message did not authorize push; push-then-verify repo) |

## c) NOT STARTED

All deliberate:

- The remaining 17 unblocked TODO items (the pool will eat them at its pace)
- The owner-gated pair (v0.2.0 cut, CQA live verify — BLOCKED in TODO_LIST)
- systemd user-unit handover for 24/7 pool operation (currently a
  session-owned background shell: if this session dies, the pool dies)
- DLQ/rescue drill (no dead letters yet — the failure path is unexercised
  live)
- Budget telemetry (cost per task invisible — crush reports no tokens back)
- serve auth (W16) — routed, unbuilt

## d) TOTALLY FUCKED UP

1. **I was the dirty tree.** The second task's first claim bounced off the
   preflight because MY plan-doc edit was uncommitted at 21:06. The system
   self-healed exactly as designed (requeue, zero burn — honestly the
   dogfood's best moment), but the incident was self-inflicted: I launched a
   pool that refuses dirty repos and then left doc edits lying around.
   Protocol fix: during pool operation, commit immediately, always.
2. **Security note came AFTER the bind.** I exposed the dashboard to the LAN
   and then explained the no-auth posture. The right order: state the
   tradeoff, get the nod, then bind. Mitigated: read-only by construction
   (ADR-0003), LAN is the owner's call, W16 routed — but the sequencing was
   backwards.
3. **I reviewed the agent's ci-local.sh ~10 minutes late.** The staged-tree
   side effect (`git add -A` → transiently dirty → preflight-refuse risk) sat
   undiscovered until this report's review pass. The pool self-heals via the
   daemon, but "review agent output within seconds of completion" should be
   the standing rule for a pool that commits to master.
4. **LAN reachability is claimed on one data point.** I verified the URL from
   the host itself, which can short-circuit host firewall rules. "Available
   on the whole LAN" is PROBABLY true and UNPROVEN from a second device.
5. Carried-over chronic (observed, not fixed): the daemon shredded
   attribution again (agent work landed fine here — its commits are clean —
   but daemon blobs still interleave); templ LSP still poisons every tool
   response (61 warnings/session); advisory lint still recomputes ~400
   findings per CI run.

## e) WHAT WE SHOULD IMPROVE

1. **Clean-tree discipline during pool operation** — the pool turns "uncommitted
   work" into visible requeue noise; commit immediately after every edit.
2. **Review agent commits immediately** — a pool committing to master needs a
   human/agent review loop with SLA measured in seconds, not end-of-session.
3. **`ci-local.sh` should say what it leaves behind**: after `git add -A`,
   print "staged changes remain — commit or reset" (or reset after the nix
   check with the user's choice encoded).
4. **Security tradeoff before exposure** — bind flags that change the trust
   boundary (`0.0.0.0`) should print a louder warning than "(read-only)".
5. **`--once` as the first live run** would have been the more conservative
   proof before the continuous pool; it worked out, but the staircase was
   skipped.
6. **Budget telemetry** — `task.completed` records duration + verify tail but
   no cost signal; even a wall-clock-per-task rollup in `tq top` would make
   the daily budget tunable by data.
7. **Prove LAN claims from a second device** (or open the firewall
   deliberately and document it).
8. **The pool's third task claimed within 0s of enqueue** — claim-order is
   priority/FIFO; fine, but `tq top` could show WHICH item is running (it
   shows project-level only today).

## f) Up to 50 things we should get done next

*Ranked view. TODO_LIST.md owns the routing (21 open items, evidence-cited);
the pool is eating them. Items marked [POOL] are already TODO_LIST pool food;
[NEW] surfaced this session and should be routed at the next HARVEST.*

1. [POOL] Let the pool finish the campaign — 17 items left, next up already claimed
2. [NEW] Run `scripts/ci-local.sh` once end-to-end (when the pool is idle) and fix whatever it catches
3. [NEW] Address ci-local.sh's staged-tree side effect (print/avoid lingering staged changes)
4. [POOL] `tq serve` auth for non-localhost binds (W16 — routed this session)
5. [NEW] Prove LAN reachability from a second device; open/document the NixOS firewall for 8090
6. [NEW] Hand the pool to the systemd user unit for unattended operation (supervised-only today)
7. [NEW] C22 post-run review: agent-quality audit of all pool commits + `.crushrc` self-modification check (diff-watch `.crushrc`/TODO_LIST each tick)
8. [NEW] Budget telemetry: per-task wall-clock + (if crush ever reports it) token cost in `tq top`
9. [NEW] DLQ/rescue drill: force one verify-failure task to exercise dead-letter → `tq dlq --rescue` live
10. [POOL] Fix papdashboard watermark ctx regression — **DONE by the pool** (aaabe1c); tick lands in CHANGELOG at next cut
11. [POOL] `checks.nix-binary-runs` (flake) — TODO #3
12. [POOL] Verify advisory-lint annotations surface on green CI runs — TODO #4
13. [POOL, BLOCKED owner] Cut v0.2.0 — now extra attractive: two agent-shipped fixes + web UI are in [Unreleased]
14. [POOL] papdashboard-e2e real-dashboard mode — TODO #6
15. [POOL] harvest.Audit per-repo failure reporting — TODO #7
16. [POOL] cmd/tq cyclop extractions (cmdHarvest/aggregateTop/cmdStats/cmdDLQ) — TODO #8
17. [POOL] CLI-level tests (audit golden, dispatch, splitRepos) — TODO #9
18. [POOL] `tq worker --once` — TODO #10
19. [POOL] Nightly fuzz job + corpus — TODO #11
20. [POOL] `nix flake check --all-systems` + nix-binary smoke — TODO #12
21. [POOL] Windows honesty (`//go:build unix` or real runner) — TODO #13
22. [POOL] `TestCheckProjectsDir` unit tests — TODO #14
23. [POOL] D91-lite crush `--version` startup line — TODO #15
24. [POOL] dprint into treefmt or formal manual decision — TODO #16
25. [POOL] Sidecar retention flags — TODO #17
26. [POOL] Fuzz `ExtractResultPayload` — TODO #18
27. [POOL] `tq audit --json` + parity flags — TODO #19
28. [POOL] webui.sh free-port selection — TODO #20
29. [POOL] `tq serve --verbose` request logging — TODO #21
30. [POOL] Re-verify the 5 hearsay-routed items — TODO #22
31. [POOL, BLOCKED owner] CQA live verification — TODO (BLOCKED)
32. [NEW] `tq top`: show the running task's item text (project-level only today)
33. [NEW] Pool status visibility in the dashboard (which pool/worker owns the claim — worker id is in facts, not UI)
34. [NEW] `.crushrc` guardrail: agent prompt should forbid editing `.crushrc`/`.tq-verify` (self-modifying autonomy risk, accepted but reducible)
35. [NEW] Serve log line should call out non-localhost exposure loudly when addr is not loopback
36. [ROADMAP, owner] Lint endgame (a/b/c) — gates the biggest debt cluster
37. [ROADMAP, owner] Master push workflow (direct-push vs protection) — now with agents committing to master too
38. [ROADMAP, owner] PR-mode enablement (`OPEN_PR=1`)
39. [ROADMAP] Journal compaction design → `tq journal compact`
40. [ROADMAP] `Store.List` filter pushdown (UI/top full scans)
41. [ROADMAP] SSE Replay + ring buffer
42. [ROADMAP] `tq journal verify` checksum chain
43. [ROADMAP] Phase D web UI (W15 write actions, W17 /metrics, W18 pagination, W19 budget panel…)
44. [ROADMAP] CI: concurrency group; advisory lint scheduled/diff-scoped; govulncheck; dependabot; Node-20 upgrades
45. [ROADMAP] `tq version` subcommand
46. [ROADMAP] Fuzz `unwrapCommand`
47. [ROADMAP] Sentinel errors per package (err113)
48. [ROADMAP] Triage 32 gosec findings
49. [ROADMAP] templ LSP false diagnostics investigation
50. [ROADMAP] Example corpus `examples/agent-pool/` (now has a REAL reference config: this repo's `.crushrc` + `.tq-verify`)

## g) QUESTIONS ONLY YOU CAN ANSWER

1. **Pool persistence:** after I stop, should the pool keep running unattended
   (systemd user-unit handover, surviving logouts/reboots) or stay
   session-supervised? And is `--daily-budget 15` the right daily spend for
   this repo (2 tasks done ≈ 13 min agent time so far)?
2. **LAN posture:** accept zero-auth read-only exposure for now (I bind
   `0.0.0.0:8090` today), or should W16 auth land BEFORE you actually rely
   on it from other devices — and do you want the bind narrowed to
   `192.168.1.150` instead of all interfaces? (I cannot verify from here
   whether the NixOS firewall lets other devices reach 8090.)
3. **Model/cost control:** the pool uses crush's default model. Want
   `--model provider/model` pinned (cheaper model for small items, or a
   per-pool choice), or leave the default? I have no visibility into your
   crush plan/pricing, so the budget cap is wall-clock-blind to real cost.

---

*Point-in-time snapshot — everything above reflects this session's run
(2026-09-07, ~20:45–21:19 CEST). The pool and serve are STILL RUNNING in
background shells (047 pool, 051 serve): kill them before assuming quiescence.
When later work makes this stale, ANNOTATE it — don't rewrite. Section (f) is
a ranked view; TODO_LIST.md/ROADMAP.md own the routing.*
