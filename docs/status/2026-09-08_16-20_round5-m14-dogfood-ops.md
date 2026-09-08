# Round-5 M14 — Dogfood Ops: Agent-Commit Review, Session Status, Budget Telemetry

**Date:** 2026-09-08 16:20 CEST
**Scope:** plan `docs/planning/2026-09-07_23-51_SUPERB-PLAN-ROUND5-PARETO-100-IMPROVEMENTS.md`
M14/F72–F76 (ideas I23, I24, I25). Wave 2 (M11–M13) closed earlier today.

---

## a) F72 — agent-commit review (quality + self-modification safety)

Reviewed the pool-era commits not authored by this session's plan work,
focused on the five most substantial:

| Commit | Verdict |
| --- | --- |
| `b420aa8` feat(webui): filter query params in EventSource | Good: feat + tests + changelog. Nit: commit body carries a leftover "Closes #<issue_number>" template line. |
| `19bdf70` feat(webui): SQL pagination @100k scale | Good: measured evidence, race-tagged test files (`race_on.go`/`race_off.go`), store tests. |
| `3721eb0` feat(webui): strict CSP + headers | Good: ADR-0003 amendment, guardrail test, smoke assertions added. |
| `0dd4409` docs: ROUND6 plan | Fine: plan artifact; touches ROADMAP/TODO_LIST only. |
| `50711df` / `c5336e1` docs commits | Fine: each marks TODO items done + changelog entries — the designed loop. |

**Self-modification safety: PASS.** No agent commit touched pool config,
`deploy/`, budget values, CI gates, or AGENTS contracts. The only
TODO_LIST.md edits are the harvester's own checkbox contract (by design).
The commits that DID touch `deploy/`, `flake.nix`, `.github/` since the
pool started (`9ae3ccd` systemd+`--config`, `f7d3188` CI matcher,
`57d24f6` flake binary-runs check) are interactive-session work (M4/M11),
not pool agents.

## b) F73 — `.tq-verify` evidence per agent task

All recent agent completions carry a green `verify_tail` in the
`task.completed` fact detail (full `go build && go vet && go test` output),
e.g. seq 135/138/141 (06:35–07:00 today) against the dogfood DB
(`tasks.db`, 141 facts). Verify enforcement is live and auditable via
`tq show <id>`. The DLQ shows two dead tasks whose verify FAILED (exit
non-zero) — the gate rejects broken work rather than trusting the agent.

## c) F74 — `scripts/tq-session-status.sh`

One read-only command answering "what is running, on which DB, since when,
how fresh is the journal". Scans `/proc` cmdlines for
`agent-pool`/`worker`/`serve`, resolves the DB (`--db` > `$TQ_DB` >
`cwd/tasks.db`), prints uptime, serve addr, and the newest journal fact.
Current live output: pool PID 3117483 (up 19h, `--daily-budget 15`) and
the still-old unauthenticated `tq serve` PID 3654482 on `0.0.0.0:8090`
(owner-gated restart pending, see prior report §g).

## d) F75 — budget telemetry into PapDashboard

The papdashboard bridge now mirrors the pool's `--daily-budget`: the day
the Nth task is enqueued, one `alert.triggered` fires (severity warning,
synthetic aggregate `agent-pool-budget-YYYY-MM-DD`, fact-seq idempotency
key), and the first enqueue of the next day resolves it automatically.
`tq agent-pool` gained `--alert-url/--alert-api-key/--alert-poll` (env
`TQ_PAP_URL`/`TQ_PAP_API_KEY`) — previously only `tq worker` could
forward alerts, so the dogfood pool had no alerting at all. Off by
default (`DailyBudget: 0`). Three stub-E2E tests: fires exactly once at
cap (not per over-cap enqueue), resolves on rollover, silent when off.

## e) Follow-ups

- Owner-gated: LAN serve restart (PID 3654482 still old binary), push
  authorization (now 15+ unpushed), v0.2.0 timing.
- The dogfood pool (started before `--review` existed) runs without agent
  reviews; a restart would pick them up (owner decision — restarting the
  pool mid-flight is gated).
