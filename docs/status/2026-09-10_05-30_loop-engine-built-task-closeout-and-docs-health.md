# Loop Engine Built: Task Close-out + Docs-Health Status + Workforce Live

_Session 2026-09-10 ~02:00–05:30 (continuation: dogfood directive → "MY GOAL
is: 3-4-5 loops"). Report 05:30. Brutal-honesty mode; scope = this run._

## TL;DR

The owner's goal system is now fully implemented: every agent task ends its
conversation with the brutal a)-g) self-review (`--task-closeout`), every
5th completion runs a docs-health status task (six living docs + archive
discipline), and harvest/review/autofix were already there. The Flash
workforce has been grinding the TODO list autonomously all night (18 tasks
done, 0 dead). The close-out loop is unit-proven but NOT yet live-proven —
no `docs/status/<ts>_task-<id>.md` exists yet. I also burned ~2 hours of
wall clock to blocked waits and shipped one scope flaw I only caught by
accident.

## a) FULLY DONE (verified this run)

1. **Workforce live on the shared pool journal** (`/mnt/pool/services/tq/
   tq.db`, dashboard :8100 sees it): GLM-5.3-Flash agents, review + autofix
   + status-every 5, project-exclusive, budget 60/day. Since launch: **18
   tasks completed, 0 dead-lettered** — govulncheck CI job, gosec triage,
   templ-components audit, webui dedup (−67 LOC), httpapi/webui stats
   split-brain fix, plus reviews, one autofix (`c5c654c`, minted by a
   `request_changes` verdict that caught a real lint-baseline violation
   hidden across a daemon auto-commit) and one status report.
2. **Loop 1 — `--task-closeout`** (`internal/executor/agent.go`): work turn
   runs `--verbose` (conditional — the pinned argv contract for default
   pools is untouched), `ExtractSessionID` gained the crush verbose-marker
   regex (`result.go`), the close-out turn resumes the EXACT session
   (`--continue` would race across concurrent agents) with the owner's
   a)-g) prompt, must re-emit the work turn's `TQ_RESULT` (the gate reads
   the last line), reports at `docs/status/<ts>_task-<id>.md`. Pinned by
   `TestAgentExecutorCloseoutTurn` (two invocations, exact `--session`
   argv, `{{TASK_ID}}` resolution).
3. **Close-out scoping**: reviews and status tasks share the agent
   executor — they now run a closeout-free clone (`registerAgentExecutors`
   in `cmd/tq/agentpool.go`): reviews ARE the second opinion; closeouts on
   them would double cost for zero signal.
4. **Loop 2 — docs-health status prompt** (`internal/executor/status.go`):
   the `--status-every` done-prompt now mandates the docs-health skill,
   reading every 2026-* report, reconciling TODO/CHANGELOG/AGENTS/README/
   ROADMAP/FEATURES with what actually shipped, archiving fully-done
   reports to `docs/status/archived/`, verify-claims-against-code; scope
   rule stays docs-only.
5. **crush upstream research, deep pass**: #3146 is the only lifecycle-
   hooks PR (maintainer-approved, CI-green, users-on-branch); #2707 is the
   canonical closed issue whose thread IS our use case ("orchestrators
   need an out-of-band turn-done signal"); lineage #1337/#1487/#2612 →
   #2598 (PreToolUse only). Interim trigger ranked: PreToolUse session
   registry > wrapper > crush.db polling > log tailing. Tested falsity:
   undocumented `SessionEnd`/`Stop` hook names do NOT fire.
6. **Upstream PR comment posted + twice revised** (charmbracelet/crush#3146,
   comment 5611995660): plain declarative English after owner feedback;
   sub-agent filter rationale made self-contained.
7. **SystemNix proposal extended**: `task-closeout = "true"` alongside
   concurrency 3 / review-autofix / status-every 5 (parses; daemon
   committed it there).
8. **CHANGELOG** entries for both loops; **TODO_LIST** grew the
   session-close-bridge item with the full trigger research; report
   `2026-09-10_01-55` annotated (ci-local green + push-prerequisite
   correction).
9. **Verification along the way**: env-restricted (`env -i`) dry-run of the
   composed service PATH; `--continue` and `--session` semantics tested
   empirically (banana/pear tests — the latter falsified resume-or-create).

## b) PARTIALLY DONE

1. ~~**Close-out LIVE proof**: unit-tested, flag on, workforce restarted with~~ done (first *_task-<id>.md closeout report exists — docs/status/2026-09-10_05-58_task-…37826.md)
   ~~it — but no `*_task_*.md` report exists yet (the first closeout-enabled~~
   ~~work task is still in flight at report time). The one review that ran~~
   ~~under the closeout build completed without a visible report and its~~
   ~~sidecar was empty — I could not determine whether its close-out ran,~~
   ~~skipped (no session id), or the pre-scoping binary was responsible.~~
2. **AGENTS.md not updated** for the two new loops (payload-contracts
   section still describes the old status prompt). CHANGELOG has it;
   AGENTS.md will drift until the docs-health pass or I fix it.
3. **Full gate pending**: build/vet/cmd/executor suites green after the
   loop implementation, but a full `ci-local.sh` has NOT run on the final
   tree (last full gate predates the executor changes).
4. **SystemNix diff**: committed by ITS daemon — but never reviewed by the
   owner and the deploy chain (push → flip → `nix run .#deploy`) remains
   unexecuted.

## c) NOT STARTED

1. Closeout report **indexing policy** — one report per task will flood
   `docs/status/` root (dozens/day at full throughput); no subdir, no index
   convention decided.
2. Status-prompt **append cap revision** — prompt says "max ~50" next
   items; TODO already sits at 50 unchecked; no flood analysis done.
3. `tq tasks`/`tq show` **liveness affordances** (notBefore countdown,
   stale-lease hint) — designed in my head after the stall, not filed as
   TODO until this report.
4. Reviewer-rides-the-work-session idea (`p.Session` exists in the payload
   contract; a reviewer with the worker's full conversation context is one
   flag away) — noticed, never evaluated.

## d) TOTALLY FUCKED UP

1. **~2 hours of wall clock silently lost** to `job_output wait=true` on
   long-running background jobs. I "slept 240s" and woke up at 05:17. The
   workforce ran unattended (it coped — 0 dead), but I was not watching
   when the restart, the first closeout-enabled claims, and the stall
   happened. Root cause: my process, not the tools — bounded polls only.
2. **Wrong-first zombie diagnosis**: seeing `running` + one `crush run` +
   stale `LAST ERROR` text, I narrated "orphaned crush + stuck task
   deadlocking exclusivity" — it was the NEW pool's live review (PPID
   alive, `--verbose` in argv, fresh `updatedAt`). I checked liveness
   signals only AFTER writing the diagnosis. Evidence before narrative,
   always.
3. **Scope flaw shipped in the first closeout build**: reviews/status got
   close-out turns because they share the agent executor — a function I
   myself decomposed hours earlier. Caught by luck (argv inspection),
   fixed in 10 minutes. Should have been caught at design time.
4. Style iteration on the upstream comment took three versions and two
   rounds of owner feedback — the "polished report voice" reflex again.

## e) WHAT WE SHOULD IMPROVE

- **Blocked waits are banned** in my own process: any job that can exceed
  a minute gets bounded polling, never `wait=true`.
- **Diagnosis discipline**: liveness evidence (PPID, updatedAt, argv)
  before any failure story; stale UI columns (`LAST ERROR` on pending
  tasks) are hints, not facts.
- **Design-review my own wiring**: when a feature threads through shared
  executors/registries, enumerate EVERY consumer before shipping — the
  closeout scope bug was fully predictable from `registerAgentExecutors`.
- **TODO flood policy is now urgent**: with status reports appending up to
  ~50 items each and closeout reports claiming up to 50 "next things" per
  task, the backlog grows faster than 60-enqueues/day can drain it. The
  prompt caps need an owner decision (see g).
- **Report placement symmetry**: per-task closeout reports and per-window
  status reports need separate homes or the docs/status index becomes
  noise (see g).

## f) Up to 50 things next (impact-sorted; ⭐ = owner-gated)

1. ~~Live-prove the close-out: first `docs/status/<ts>_task-<id>.md` from a~~ done (live proof landed as docs/status/2026-09-10_05-58_task-…37826.md)
   ~~work task; inspect its verdict/review interplay.~~
2. ~~Full `./scripts/ci-local.sh` on the final tree (post loop changes).~~ done (05-58 session ran full ci-local — ALL CI GATES GREEN at 05:58)
3. ~~AGENTS.md payload-contract section: document `--task-closeout` and the~~ done (docs-health pass 2026-09-10 06:25 — AGENTS.md agent payload contract now documents --task-closeout + the close-out-free clone)
   ~~docs-health status prompt.~~
4. ⭐ Closeout report placement: `docs/status/tasks/` subdir + index rule
   (or root with naming convention) — needs a call before volume arrives.
5. ⭐ Status-prompt append cap: ~50 → ~10? (TODO at 50 unchecked and
   growing; harvest pacing means old items wait days.)
6. `tq tasks`: add notBefore/requeue-backoff column; `tq show`: surface
   lease staleness (the stall I fumbled would be self-evident).
7. ⭐ Push master (~60+ commits incl. every fix and loop) — the deploy
   chain dead-ends without it.
8. ⭐ SystemNix: review proposal, flip input, `nix run .#deploy`; then
   confirm journalctl shows `harvest: enqueued` and the loops under
   systemd.
9. Watch crush#3146 for merge; swap the PreToolUse registry trigger →
   SessionEnd hook; PR #3482 (`hook_event_name`) relevance check after.
10. Build the session-close bridge (TODO item carries the full research).
11. Reviewer-session idea: pass the worker's session id to the review
    executor (`p.Session`) so the reviewer reads the full work
    conversation — evaluate quality delta on 5–10 reviews.
12. Worktree-per-agent design doc (TODO exists).
13. Dead-pool detection alert (TODO exists) — now doubly justified: I
    misread a live system as dead for the inverse reason.
14. Dogfood evidence archive (`/tmp/tq-dogfood.db` still ephemeral).
15. `tq pool-health` one-shot (skip streaks + last activity).
16. dogfood-once smoke script (env-gated).
17. Remaining ~27 unchecked TODO_LIST items (the workforce is eating them;
    ~4–6/day per repo at current pacing).
18. 6 BLOCKED owner decisions (consumer wire-or-delete, three interfaces,
    Postgres CLI timing, dependabot policy, release retrospective).
19. Closeout prompt tuning after live evidence: does the agent actually
    answer a)–g) honestly, or perfume it? Sample 5 reports and grade.
20. Review-autofix loop budget: confirm fix tasks terminate via dedup +
    budget guard under the new closeout cost (each task now ~2 agent
    turns).
21. Cost telemetry: per-task token spend sidecar (Flash is cheap; the
    closeout doubles turns — measure, don't assume).
22. module-eval assertion for agentPath (TODO exists).
23. cwd-dependence sweep + checkProjectsDir + skip-log dedup + doctor PATH
    warning (TODO items exist).
24. ETXTBSY retry firing check in pool journal (first live evidence).
25. Status task window payload: include the window's closeout report paths
    so the docs-health pass reads them (currently only task ids + items).

## g) Questions I cannot answer myself

1. **Closeout report volume**: every task writes a status report. Root of
   docs/status/, a `docs/status/tasks/` subdir, or sampled (every task
   writes, only flagged ones get indexed)? At ~10 tasks/day this decides
   whether the status corpus stays readable.
2. **Append caps**: status reports may append up to ~50 TODO items each
   and closeout reports may claim up to 50 "next things" — the backlog now
   outgrows the drain rate (60 enqueues/day). Cap appends at ~10, raise
   the budget, or let the backlog grow?
3. **Push**: same as before, still the #1 blocker — master is ~60+ commits
   ahead with every fix and both loops; nothing deploys until it moves.
   Push now (and do the two backend tags ride along)?

---

*Format: Markdown per explicit instruction (skill default HTML, override
flagged). Workforce note: the pool kept working while this report was
written — counts are as of 05:30.*
