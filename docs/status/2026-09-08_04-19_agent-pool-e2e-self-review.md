# Status Report — Agent-Pool Session (Sep 6) & Self-Review

**Date:** 2026-09-08 04:19 CEST
**Scope:** This session's run only — the "self-managing pool of crush/AI agents" question and its execution. Not a repo-wide audit.
**Session window:** Sep 6, ~08:30–17:00 CEST; retrospective written Sep 8 04:19.

---

## What this session did (recap, evidence-first)

1. **Discovered and fixed broken CI on master** — commit 918c3ea (repo-hygiene pass) introduced vet errors (`_ = s.Enqueue(...)` rewritten into single-value assignment errors, `_ = t.Cleanup(...)` mangled). Repaired on `fix/ci-vet-errors`, shipped as **PR #1 (MERGED)**. Master green since (verified: run 34165549692, success, Sep 7 22:07Z).
2. **Proved crush headless** — `crush run` works in any directory with `ZAI_API_KEY` exported (zai coding endpoint). `~/.local/share/crush/providers.json` is crush-GENERATED and must never be hand-written.
3. **Built an agent executor** (internal/executor/agent.go, 434 lines) + limitedBuffer/json helpers + CLI registration + fake-binary test.
4. **E2E proof #1 (scratch dir):** enqueue → claim → crush writes file → completed, **4s cycle**.
5. **Merge collision with a parallel session** — origin/master had independently landed a **superset** agent-pool (a72cefd: verify gate, RequireClean, harvest, cqa/papdashboard bridges, DedupKey idempotency, worker drain fix). Per merge-reconciler: superseded → `git rebase --skip` dropped my redundant commit. Correct call.
6. **E2E proof #2 (real repo, real TODO):** BerryBig `VerifyAge` leap-year bug. Cloned writable → enqueued as agent task → agent produced the canonical one-line fix (`dob.AddDate(age, 0, 0).After(now)`) → **verify gate ran all 14 test packages → completed 3m06s** under load-40. Patch at `~/shared/berrybig-verify-age-fix.patch`.

---

## a) FULLY DONE

- CI repair via PR #1 — merged, master green.
- Headless crush proven — the unlock for everything below.
- E2E agent-pool loop proven twice — scratch write (4s) and real bugfix with verify gate (3m06s, journal-verified).
- Merge adjudication — redundant executor dropped per merge-reconciler; superset kept.
- Durable facts recorded (crush config quirk, E2E proof, heredoc lesson).
- Deliverable handed off — BerryBig patch at ~/shared/ (reachable by Lars; /tmp is not).

## b) PARTIALLY DONE

- **"Which repos leverage go-taskqueue"** — answered with evidence (158 repos / 1,853 TODO items in harvest dry-run; CQA + PapDashboard first-class bridges) but NOT turned into a standing pipeline. The dry-run never became a scheduled pool.
- **ZAI_API_KEY env quirk** — memory entry fixed, but there is no shared env-file convention for every context that launches agents.
- **Postgres SKIP LOCKED store (v0.2)** — seam documented in ADR-0001, not started (deliberately out of scope).
- **The BerryBig bug itself** — fix proven and patch delivered, but never applied/committed upstream. The repo still carries the bug; the fix lives in a /tmp clone + a .patch file.

## c) NOT STARTED

- Postgres store, HTTP API (v0.2 slice from ADR-0001).
- CQA findings→tasks bridge **live** (code exists, never pointed at a running CQA).
- Harvest pacing/budget tuning for 158-repo scale.
- `tq agent-pool` as a supervised service (systemd/SystemNix — Lars's infra, his call).
- BerryBig patch upstream.
- Agent-pool on a second box.
- Session continuation (AgentPayload.Session) exercised E2E.

## d) TOTALLY FUCKED UP (honest ledger)

1. **Split-DB bug in first E2E** — enqueued without `--db` (task landed in ./tasks.db) while worker polled /tmp/e2e.db. Four minutes of confusion until diagnosis. Fix: consistent `--db` everywhere.
2. **Heredoc corruption ×3** — hand-wrote crush providers.json twice via `cat <<EOF` producing garbage JSON, plus a payload typo (`VERBD.txt`). All corrected same session; lesson banked.
3. **Agent-test first drafts sloppy** — nonsense `agentOK` wrapper helper and stray text in a failure message; caught by vet before merge. Full-file rewrites fixed them.
4. **Pattern recognition latency ~2 failures** — the drift family (long-content terminal writes) had a documented fix from the prior day, yet I let a heredoc corrupt a config file a SECOND time before switching to write_file. Target: recognize after 1.
5. **Pushed to master pre-rebase** — parallel session had landed 25+ commits; push rejected. Avoidable: pull-before-push discipline slipped after 5+ hours in-session.
6. **Orphan risk from rebase --skip** — my buffer.go/json.go helpers were staged during the conflict resolution before the skip; verify they are NOT lingering as dead code in the landed tree (check on next repo visit).

## e) WHAT WE SHOULD IMPROVE

1. **write_file-only rule for config/payload content** — evidence now 3 incidents deep.
2. **`--payload-file` CLI flag** — eliminates 500-char shell-quoted JSON payloads; direct fix for the corruption family.
3. **`tq agent-pool` as supervised service** — today the pool only lives as long as a terminal session.
4. **Env-file convention for agent keys** — one source of truth for ZAI_API_KEY/endpoint across pool + crush sessions.
5. **Cost budget flag for harvest** (`--max-tasks-hour` or token budget) — at 10/tick every 5m, worst case ~120 agent-runs/hour on the zai coding plan. Make the 24/7 pool safe to leave running.
6. **Verify gate stays build+test only** — golangci-lint is explicitly not a CI gate (AGENTS.md); do not VERSCHLIMMBESSER the gate.
7. **Orphan cleanup check** — buffer.go/json.go/agent_test.go state post-skip; delete dead code or fold into landed implementation.

## f) Next things (impact-sorted)

**P0 — apply the proven work**
1. Apply BerryBig patch upstream (test → commit → push)
2. `--payload-file` flag for `tq enqueue`
3. Verify buffer.go/json.go/agent_test.go orphans removed post-skip
4. Cut v0.1.1 (agent-pool + harvest + bridges + PR #1; CHANGELOG, tag, push)
5. Stand up the pool as a supervised service on one repo, with pacing + budget flag
6. CQA bridge live: point at a running Code-Quality-Agent instance
7. PapDashboard webhook live: dead-letter alerts flowing
8. SystemNix module for `tq agent-pool` (Lars's merge call)
9. Env-file convention shared by pool + crush sessions

**P1 — v0.2 slice**
10. Postgres SKIP LOCKED store (multi-box)
11. HTTP API (enqueue/list/stats)
12. Session continuation E2E (real crush session ID)
13. Task result artifacts: agent output tail + diff as task metadata
14. Task result → branch + PR automation (human merge gate stays)

**P2 — deeper integration**
15. mr-sync 93 TODO-bearing files → harvest source
16. CV yq-parse CI item → task → fixed → closed
17. StopTube nixpkgs Go-toolchain item → task
18. Standup-Killer ~150 lint debt → paced tasks
19. BerryBig remaining ~15 file:line items → paced tasks
20. Kernovia F40–F42 hot-reload rebuild → task
21. Full 158-repo backlog sweep — paced, budgeted, PR-gated
22. `tq stats` → PapDashboard queue-health widget
23. `tq tail -f` → Discord channel (queue observability)

**P3 — quality of life**
24. `tq show` enhanced output (attempts, last error tail, dedup key)
25. `tq dlq --rescue-all --max N` for burst recovery
26. `tq harvest --dry-run --json` for scripted planning
27. TODO_LIST.md format lint (checkbox + nearest-heading contract)
28. Agent prompt templates (bugfix shape: file:line + canonical pattern + constraint)

**P4 — ambition**
29. Cross-repo dependency graph (task depends on task in another repo)
30. Repo-name routing: `tq enqueue --repo BerryBig --type agent ...`
31. Second-box worker (needs Postgres store first)
32. Pool self-improvement loop: agent fixes → CQA rescan → new findings → tasks (the full Ouroboros)

## g) Questions I cannot figure out myself

1. **Where should the pool run as a service?** My sandbox is ephemeral. A systemd unit / SystemNix module for `tq agent-pool` — which box, which user, which DB path? (If "not yet", the pool stays a CLI you invoke when wanted.)
2. **Spend ceiling for autonomous agents?** At 10 tasks/tick every 5m, worst case ~120 agent-runs/hour on the zai coding plan. What is your monthly ceiling? ($0 = pool stays manual.)
3. **BerryBig patch: how does it land upstream?** (a) you `git apply` from ~/shared/, (b) a PR from my clone, or (c) ignore — let the pool re-harvest the TODO and have an agent commit it properly?

---

*Report written 2026-09-08 04:19 CEST from session evidence: journal facts, git history, CI runs. Waiting for instructions.*
