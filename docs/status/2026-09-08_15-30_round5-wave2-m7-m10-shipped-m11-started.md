# Round-5 Execution — Wave 2 Shipped (M7–M10), M11 Just Started

**Point-in-time status report, 2026-09-08 15:30 CEST.**
Continuation of the round-5 Pareto execution
(`docs/planning/2026-09-07_23-51_SUPERB-PLAN-ROUND5-PARETO-100-IMPROVEMENTS.md`,
27 medium tasks M1–M27 / 150 micro-tasks F1–F150). Prior wave-1 report:
`docs/status/2026-09-08_05-54_round5-execution-wave1-bounded-reads-pagination-security.md`.

| Fact | Value |
| --- | --- |
| HEAD | `99e1d34` (master) |
| Unpushed | **10 commits** (my four detailed M7–M10 commits + the daemon blobs carrying their code). ~34 older commits were pushed mid-session by someone else (not me — likely the pool/daemon). |
| Working tree | clean at report time |
| Gates at each commit | build + vet + gofmt + `go test ./... -race -count=1` green; webui smoke green when webui touched; doc-refs check green when docs touched |
| Agent pool | PID 3117483 still alive (18.5h) |
| **LAN serve** | **PID 3654482 STILL the old binary on `0.0.0.0:8090`, unauthenticated — unchanged top risk** |
| Session commits | `a836cd4` (M7), `960de3e` (M8), `a505e54` (M9), `99e1d34` (M10) |

Parallel agents shipped meanwhile (not mine): agent-review feature
(`--review`/`--review-autofix`), SSE filter forwarding fix, ADR-0004,
persisted-bridge-watermark designs, ROUND6 plan draft, cordis verification.

---

## a) FULLY DONE this session (each: implemented, tested, gates green, committed, documented)

1. **M7 — Budget + retry visibility UI** (`a836cd4` + daemon blobs):
   - Budget StatCard ("2/5 · budget today", tone green/amber/red at
     75%/100% of cap) next to the project chips; absent without
     `--daily-budget`.
   - "ready" column in the task table: "in 12m" for future `notBefore`,
     "ready" once claimable, empty otherwise; exact timestamp on tooltip.
   - **Bug found & fixed**: `readiness()` humanized future waits with
     ago-semantics → always "in 0s". New `durationUntil()` owns forward
     durations; `timeAgo()` delegates to it.
   - Tests: snapshot wiring (cap set → card with real counts), absent
     without cap, tone thresholds, readiness strings, column rendering.
   - Honest gap: plan F38's "screenshots, both themes" was NOT done (no
     browser tooling); covered by fragment assertions + tone classes.

2. **M8 — Live task detail pages** (`960de3e`):
   - `GET /task/{id}/events`: task-scoped SSE mirroring the dashboard
     protocol (subscribe → snapshot → tick; trailing title event carries
     the watermark id; unknown ids 404 before stream open).
   - `TaskDetail` split into `taskDetailCard` + `taskDetailTimeline`
     inside `#frag-detail` / `#frag-timeline` containers; no-JS fallback
     preserved; connection lamp reused.
   - `app.js` routes `/task/{id}` pages to the task-scoped endpoint and
     forwards the auth token (EventSource cannot set headers).
   - Route joined the read-only table → guardrail test still proves zero
     mutating handlers.
   - Tests: connect snapshot order, live pending→completed transition,
     404; smoke script now drives the detail page + stream end to end.
   - Smoke gotcha solved: fixed-size SSE reads hang — line-based reading
     with a deadline instead.

3. **M9 — Cooperative cancel of running tasks** (`a505e54`, ADR-0005):
   - `task.cancel-requested` fact IS the flag (facts-first, zero schema
     change; observation = indexed EXISTS; request idempotent).
   - Worker heartbeats observe it (≤ lease/4 latency), cancel the exec
     context; `sh` executor now sets a process group like `agent` (whole
     tree SIGKILLed); terminal write `CancelOwned` is owner-guarded.
   - Crashed-worker path: expired-lease reclaim in `ClaimDue` finalizes
     the cancel (Released + Cancelled in one tx) instead of re-executing.
   - CLI: `tq cancel` refuses running tasks with the `--force` remedy;
     `--force` requests the cooperative cancel.
   - Fixed a pre-existing mislabel: parent-cancelled executions were
     reported as "task timeout after 10m0s".
   - Fixed a rollback bug I introduced mid-implementation and caught
     before commit: withTx rolls back on ANY closure error, so the
     ClaimDue finalize uses a committed-then-flag pattern.
   - Tests: store contract (request/honour/idempotency/reclaim),
     worker mid-run race test (exactly-once executor, 0 attempts burned),
     e2e subprocess test cancelling a live 30s sleep within a heartbeat.

4. **M10 — `tq doctor`** (`99e1d34`):
   - Checks: SQLite quick_check + WAL mode, queue mix with expired-lease
     detection, worker liveness (no heartbeats in 10m while work waits =
     FAIL; idle empty queue = ok), budget vs `--daily-budget` (warn ≥75%),
     agent binary on PATH, per-repo TODO_LIST.md + `.crushrc` via `--repos`.
   - Human table + `--json`; exit 1 on failures (cron/systemd-able).
   - Verified live against a real temp DB (correctly FAILs "worker down"
     with a pending task and no worker).
   - Tests: healthy DB, dead-worker signature, budget at cap, corrupt
     DB file, autonomy files, JSON shape.

## b) PARTIALLY DONE

- **M11 — Platform honesty (just started, ~15%)**: Audit complete, code
  changes not yet made. Findings so far:
  - `internal/e2e`, `internal/e2e/chaos_test.go`,
    `internal/harvest/e2e_test.go` already carry `//go:build unix` — F55
    is largely satisfied already.
  - `internal/executor/agent_test.go` uses os/exec shell stubs with NO
    build tag — needs a look (candidate for `//go:build unix`).
  - CI has a Windows **cross-compile** gate but no Windows **test** job —
    F56 (honestly-tagged Windows test run) not implemented.
  - F57 (`nix flake check --all-systems` + CI matrix note) not done.
  - F58 (smoke the **nix-built** binary through `scripts/smoke/webui.sh`;
    the script already supports `TQ_BIN`) not done.

## c) NOT STARTED (from the round-5 plan)

- M12 release automation + docs truth pass (F60–F65)
- M13 hygiene pack (F66–F71)
- Wave 3: M14 dogfood ops, M15 pool ops, M16 queue health, M17 journal
  future, M18 UI interactions, M19 UI metrics, M20 UI QA, M21 Postgres
  store, M22 v0.2 surfaces
- Wave 4: M23 feature designs, M24 executor pack, M25 CLI pack, M26
  quality/CI pack, M27 security/misc/docs pack
- Final `scripts/ci-local.sh` pre-push gate for the session

## d) TOTALLY FUCKED UP (nothing destroyed; honest mistakes this session)

1. **Self-inflicted edit damage in webui_test.go (M7)**: my insert-anchor
   edit REPLACED the first lines of `TestStoreClosedErrorPaths` instead of
   inserting before it; caught immediately and restored. Lesson applied:
   insert with unique full-context anchors, verify diff after each edit.
2. **ClaimDue finalize rollback bug (M9, caught pre-commit)**: returning
   `ErrNoTaskDue` from the withTx closure would have rolled back the
   cancel-finalize it just wrote → infinite finalize loop. Fixed with the
   commit-then-flag pattern + a dedicated regression test.
3. **runExecutor composite-literal brace error (M10)**: one-line syntax
   slip, fixed on first build.
4. **e2e flag-order bug (M9)**: put `--db` after the positional id; Go's
   flag parsing stops at the first positional → usage error. Fixed.
5. **Doctor test logic bug (M10)**: second `ClaimDue` expected a due task
   while the first lease was still valid; rewrote to a single 1ms lease.
6. **Repeated edit-tool mtime rejections**: the daemon/templ-watch touches
   files constantly; I burned several round-trips retrying without
   re-viewing. (Tool-enforced, but I should always view-then-edit in one
   motion.)

## e) WHAT WE SHOULD IMPROVE (my work + process, this session)

- **Commit fast enough**: the daemon swept M7's and part of M9's code into
  generic `chore: auto-commit` blobs before my detailed commits landed;
  history quality suffers. Mitigation: stage + commit immediately after
  gates rather than writing docs first.
- **Doctor worker-liveness false positive risk**: pending tasks that are
  merely NotBefore-delayed count as "work waits" — a pool blocked on
  delays could read as "worker down". Should exclude not-yet-due tasks.
- **Doctor DLQ warn threshold (>2) is arbitrary** — should derive from
  baseline or make configurable.
- **Doctor budget detail line prints "cap N" twice** (cosmetic wording).
- **`factBadgeClass` in render.go appears unused** (only factTone feeds
  the journal pane) — dead code candidate for the M13 cleanup pass.
- **SSE smoke pattern**: the line-based stream reading solved for the
  detail stream should also harden the dashboard `/api/events` assertion
  (currently a fixed 2048-byte read that works by luck of ordering).
- **No `nix build` run this session** — correct per gates (no
  go.mod/flake changes), but M11 will need it anyway; run once before the
  next push.
- **Screenshots (F38/F43) skipped** — no verified headless-browser path;
  M20 (UI QA pack) should set up `scripts/webui-screenshots.sh` properly
  instead of pretending coverage.
- **Owner questions from the previous session went unanswered again** —
  re-asked below in (g).

## f) NEXT — up to 50 things, in execution order

**Finish Wave 2 (immediate):**
1. M11/F55: decide `agent_test.go` build tag (shell-stub POSIX usage)
2. M11/F56: CI Windows job running honestly-tagged tests only
3. M11/F57: `nix flake check --all-systems` locally + CI matrix note
4. M11/F58: `TQ_BIN=$(nix build --print-out-paths)/bin/tq scripts/smoke/webui.sh`
5. M11/F59: docs + commit M11
6. M12/F60: `scripts/release.sh` codifying the v0.1.0 checklist
7. M12/F61: tag + GitHub-release steps scripted; nix-binary smoke inside it
8. M12/F62: FEATURES Web UI section rewrite (redesign/theming/CSS/adoption)
9. M12/F63: FEATURES pool/CLI rows + README screenshot + `--once` quickstart
10. M12/F64: CONTRIBUTING: CSS build step + doc-reference check
11. M12/F65: commit M12
12. M13/F66: OFL-1.1 license file into `static/fonts/`
13. M13/F67: guard test — AGENTS adoption table vs actual templ-components imports
14. M13/F68: helper table tests batch 1 (statusBadgeType, factTone, shortIDTail, factTimestamp)
15. M13/F69: helper table tests batch 2 (detailItems, statusHref, taskRowClass)
16. M13/F70: templ-LSP false-diagnostics workaround documented
17. M13/F71: sentinel-error burn-down pass 1 (+ delete/verify `factBadgeClass`)
18. Commit M13 → **Wave 2 complete (64% of value)**

**Wave 3 (M14–M22):**
19. M14/F72: review the five newest agent commits (quality + self-modification safety)
20. M14/F73: verify `.tq-verify` ran per agent commit; record findings
21. M14/F74: `scripts/tq-session-status.sh` (live PIDs, ports, DB paths)
22. M14/F75: budget telemetry → papdashboard alert + stub E2E
23. M15/F77: pool chaos test — SIGKILL mid-drain under `--once`
24. M15/F78: `--repo-timeout repo=10m` ladder flag + validation
25. M15/F79: machine-wide `--max-concurrent-agents` cap
26. M15/F80: `crush --version` probe at pool start
27. M15/F81: model pin + budget value propagation (owner decisions)
28. M16/F83: stuck-running query + `task.orphaned` fact (doctor already detects; make it a fact)
29. M16/F84: doctor integration for orphaned leases (sharpen the NotBefore false-positive too)
30. M16/F85: heartbeat cadence = lease/3 knob + tests
31. M16/F86: multi-process SQLite contention test (2 writers + N readers)
32. M16/F87: 10k-task load script + baseline into FEATURES
33. M17/F88: compaction ADR for the facts-first journal
34. M17/F89: `tq journal compact --before SEQ` design sketch
35. M17/F90: hot-cold archive schema sketch/prototype
36. M18/F92: fact lines link to `/task/{id}` in the dashboard feed
37. M18/F93: `/project/{name}` page reusing the filter pipeline
38. M18/F94: sortable columns via library DataTable
39. M18/F95: expandable error popover + `?` keyboard-shortcut overlay
40. M18/F96: live relative-age ticking between SSE bursts
41. M19/F98: fact-rate sparkline + task-duration histogram
42. M19/F99: journal-size + watermark stat card
43. M19/F100: `/facts?after=` cursor endpoint + infinite-scroll viewer
44. M20/F103: a11y pass (focus order, skip link, labels)
45. M21/F109: pgx dependency + DDL schema (CGO stays off)
46. M22/F115: HTTP API design (token auth + route set)
47. Commit per task; full `scripts/ci-local.sh` before any push claim

**Wave 4 (M23–M27) — F121–F150 per the plan file; highlights:**
48. M25/F133: `tq version` ldflags verification + test
49. M26/F140: extend nightly fuzz to `unwrapCommand` + `ExtractResultPayload`
50. M27/F145: secrets-in-logs test + `--redact` payload redaction

**Session hygiene (do before stopping):**
- Update TODO_LIST.md/AGENTS.md with ADR-0005 + doctor facts (daemon may
  have done parts — check first)

## g) QUESTIONS FOR THE OWNER (cannot figure these out myself)

1. **LAN dashboard restart (critical, 4+ days old risk)**: PID 3654482
   still serves the OLD unauthenticated binary on `0.0.0.0:8090`. May I
   kill it and restart under the current binary with a token (either give
   me a token value or approve me generating one)? This needs your OK
   since others may be watching that dashboard.
2. **Push authorization**: 10 commits sit unpushed (my M7–M10 work).
   Someone pushed ~34 commits mid-session — was that you/the daemon, and
   should I run `scripts/ci-local.sh` + push the rest now?
3. **v0.2.0 go/no-go (M12 gate)**: M12 prepares release automation; the
   plan gates the actual tag on your decision. Ship v0.2.0 after Wave 2,
   after Wave 3, or hold?

---

**Bottom line:** Wave 2 is 4/5 shipped and verified (M7 budget/readiness
UI, M8 live task SSE, M9 cooperative cancel with ADR-0005, M10 doctor);
M11 is scoped with findings in hand. The 1%+4% Pareto tiers (Wave 1+2,
~64% of the plan's value) close with M11–M13. Nothing is broken; all
gates green at every commit; the oldest open risk remains the
unauthenticated LAN serve process.
