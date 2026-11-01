# SUPERB V2 window closed: 5-leg pool smoke green, docs wave, SystemNix staged

Session 19:22→19:50 (2026-11-01), owner whole-list GO resumed from STOP #2.
Scope: everything the handoff listed — V2.10 (the only red), V2.15, V2.18,
docs wave, SystemNix, hygiene. Concurrent agents + auto-commit daemon
active throughout; every gate below ran on the swept tree.

## a) FULLY DONE

- **V2.10 mid-run kill leg GREEN (run12)** — the six-run mystery closed
  with TWO root causes, neither being the handoff's standing theory:
  1. *The standing "inert mock" diagnosis was WRONG.* Run9's own
     artifacts disprove it: the task completed EXACTLY MOCK_STALL
     (25.18 s) after enqueue — the tool branch DID fire and the stall
     DID arm. The real bug was a marker-rm race: with claim-wake, the
     whole pre-stall phase (claim → title → tool_call → tool result)
     takes <1 s, so the script's `rm -f stall-entered.log` (after the
     running-id poll) ERASED the marker; the poll then waited 60 s for
     a second stall that never comes. Fix: rm BEFORE the item commit.
     REQ_LOG instrumentation (the "missing instrument") added too.
  2. *A REAL product bug the leg was built to catch*: routed work turns
     ran `crush run --quiet`, which suppresses the severed-stream WARN
     ENTIRELY — probe-observed quiet severed run = rc 0, 1-byte output
     — so a mid-run server death completed tasks silently (run11: task
     completed, no classification). Fix (agent.go): routed work AND
     closeout turns always run `--verbose`; the closeout rc-0 path now
     classifies too. Verb pinned 3-way in TestAgentCrushServerSeam;
     CHANGELOG Fixed row; executor gate green.
  - Assertion fixed to the honest surface: `tq tasks --json` carries no
    `last_error` field at all — the class rides the requeued FACT; the
    leg asserts `tq show` renders `[crush-server]`.
- **V2.18 routed verdict bridge GREEN (new leg)**: the mock regexes the
  rail's per-run path out of the baked prompt and scripts the REAL
  `tq verdict --file <path> '<json>'` as the bash tool call — executed
  SERVER-side (tq rides the pool's PATH into the server env, the M8
  workaround shape live-proven); the task completes and `tq show`
  carries the verdict. The full bridge in-pool, zero spend.
- **V2.15 cancel-vs-server GREEN (new leg)**: stall-gated `tq cancel
  --force` on a routed running turn → worker heartbeat cancels the run
  context → pool logs "task cancelled by operator request", the routed
  client process exits, and once the held turn drains NO session over
  the repo stays busy (server reaped it); task lands cancelled.
  Discovered en route: agent-pool default lease 5 m → heartbeat 75 s —
  cancel observation is slower than a smoke window (run13 red); the
  smoke pool now runs `--lease 20s` (heartbeat 5 s).
- **crush-pool-supervisor.sh = FIVE legs, run14 ALL GREEN** (429
  fast-refuse, keeper/supervisor restart, mid-run kill no-burn, routed
  verdict bridge, operator cancel). COVERAGE rows for all three new
  legs, including both pinned races (marker rm-order; verbose-only
  classification) and the lease lesson. Header documents the legs.
  Also: the parked kill-leg probe task is now retired post-leg (its
  ~2 m no-burn park could re-claim and steal the running slot).
- **Docs wave (A16)**: README "Server mode" section (flag, keeper,
  no-burn, RSS, rails-not-env, version pin, rollback, smoke pointers);
  FEATURES 3 rows (crush-server seam; live agent pane + stall detector;
  questions pane — the pane rows were missing entirely); DOMAIN_LANGUAGE
  5 terms (routed run, thin client, crush workspace, keeper, supervisor
  ladder); AGENTS.md verdict-channel row extended (--file rails +
  env-gap WHY + R4 `-H`-alone trap) with two offsetting trims —
  24398/24400 B, size-guard green via the cmd/tq gate.
- **L4 GO/NO-GO sheet drafted (V3.7/A18)**:
  docs/planning/2026-11-01_19-50_L4-FLEET-GO-NO-GO-SHEET.md — 8 evidence
  slots (RSS, crash classes, no-burn regressions, stalls, success-rate
  delta, channel failures, pane usage, restarts), known costs, rollback,
  ruling ritual.
- **SystemNix staged flip (V1.8 house half)**: module comment-stages
  `crushServerSocket = "/mnt/pool/services/tq/crush-pool.sock"` — NOT
  active because the locked rev (07a396ca, Oct 29) PREDATES the upstream
  option, and the house's own 2026-10-07 rule says version-coupled keys
  land WITH their lock bump, never earlier (unknown option = crash-loop
  both units). Test gains `crush-server-flip-not-yet-active` (pins that
  no --crush-server rides today's ExecStarts) + the exact suffix pins to
  swap in at the flip commit. docs/services/tq.md runbook row: activate
  with the lock float, watch ONE wave, rollback = remove the line.
  `nix build .#checks.x86_64-linux.tq-agent-pool` GREEN.
- **Planning-doc LIVE-STATUS**: V2.10 and V2.15+V2.18 green rows with
  both root causes and the lease lesson recorded.
- **Hygiene (partial, see b)**: trashed the probe stacks —
  /tmp/tq-crush-death-probe.\*, tq-crush-normal.\*, tq-midrun-debug.\*,
  pool-smoke-run5-14.log, the quiet-kill probe, mock python checks, the
  29 MB scratch tq binary. Smoke traps left no scratch dirs behind.

## b) PARTIALLY DONE

- **Full ci-local battery IN FLIGHT** at report time
  (/tmp/ci-local-full-1945.log, rc appended on completion): launched
  with `CI_CHECK=off` — a CONSCIOUS bypass, the gate's own instruction
  for this shape (latest remote CI run is a red ANCESTOR of local HEAD;
  master is unpushed so CI cannot see the fixes). Executor + cmd/tq
  module gates already green individually; `go mod vendor` synced.
  lint-baseline `--check` NOT yet run this window.
- **Module gates**: executor (build+vet+test) and cmd/tq
  (test-cmd-tq.sh) green; the remaining per-module loop (worker, queue,
  webui, journal, …) rides the in-flight battery rather than a separate
  pass this window.

## c) NOT STARTED (with reasons)

- **V1.9/V1.10/V1.16/V2.19/A15**: owner-run deploy (`nix run .#deploy`
  is sudo-gated), watched routed waves, and the enricher real-closeout
  pass — all blocked on the flip actually deploying. The preparables
  (SystemNix staging, L4 sheet, watch checklists) are done above.
- **A11/V2.14** routed prompts surfaced in the browser QuestionsPane:
  not in this window's mandate; the file-rail bridge covers the channel.
- **A17 canary-flip**: upstream-dependent (crush#4210 still open); the
  red-on-purpose note lives in the route smoke header (verified intact).

## d) TOTALLY FUCKED UP

- **The previous window's diagnosis cost runs**: "the pool mock answers
  inertly" was disprovable from run9's timing alone (completed −
  created == MOCK_STALL to the millisecond) WITHOUT a rerun or a new
  instrument. Duration-forensics-first would have closed V2.10 one
  window earlier. Recorded in COVERAGE.
- **multiedit partial-application hit again** (edit 4/4: my old_string
  carried the new value `timeout 420`): caught by the grep-verify rule
  before any run burned. The rule is now reflexive — keep it.
- One hook round-trip burned: a grep OF smoke paths without TQ_DB
  exported is blocked (command-position rule) — export TQ_DB for ANY
  smoke-path-touching command, not just runs.
- run13 (cancel leg) red on a KNOWN default: I sized the cancel window
  against heartbeat cadence only AFTER seeing the red — the 5 m default
  lease was greppable before the run. Cheap-gates-first applies to
  smoke DESIGN too: read the knobs you depend on before burning a run.

## e) WHAT WE SHOULD IMPROVE

- **Timing forensics before instruments**: created/completed/updated
  deltas against known constants (stall lengths, timeouts) are free
  evidence; burn them before adding REQ_LOG-class instrumentation.
- **`tq tasks --json` has no `last_error`** (nor evidence class): an
  operator cannot see WHY a task is parked without `tq show` per task.
  Consider a last_error/evidence column (owner call — JSON shape is
  operator-visible, O14).
- **AGENTS.md is 2 bytes from its ceiling** (24398/24400): the next
  addition MUST prune or reset the budget. The guard's own failure text
  says resetting is a conscious option — flag to owner rather than
  nickel-and-dime forever.
- **Cancel latency under default config**: heartbeat = lease/4 = 75 s at
  the 5 m agent lease. Fine for real agents; worth knowing when the
  dashboard cancel "feels slow" — a shorter lease or a cancel-notify
  channel is a future option, not today's problem.

## f) NEXT (owner-prioritized; NOT harvested into TODO_LIST — this repo's
pool dispatches TODO rows as paid agent tasks, and most items below are
owner actions or owner-gated)

1. Owner: PUSH master (origin is behind; the red CI run is a stale
   ancestor — CI re-runs green on the new tree if the fixes hold).
2. Owner: SystemNix lock float (`nix flake lock --update-input
   go-taskqueue`) past 2026-11-01 + uncomment the staged
   `crushServerSocket` line + `nix run .#deploy` (V1.9).
3. Watch the FIRST routed wave (V1.10): `tq tasks` attempts sane, DLQ
   empty or `[crush-server]` rows that re-ran cleanly, closeout facts,
   `/agents/live` pane alive.
4. Record flip date + first-wave deltas in the planning doc LIVE-STATUS
   (the deployed-flip paragraph is written and waiting).
5. V2.19/A15: enricher real-closeout pass on the first routed closeout
   (verify crushserver.go facts land in the actual report).
6. V1.16: one parked question routes home end-to-end under routing.
7. A11/V2.14: surface routed-run prompts in the browser QuestionsPane.
8. A17: canary-flip ritual when crush#4210 ships (route smoke header).
9. Fill the L4 sheet's 8 telemetry slots after ≥1 week (A18 ruling).
10. Owner: AGENTS.md budget ruling (prune discipline vs reset 24400).
11. Owner: `tq tasks --json` last_error/evidence column appetite.
12. Re-run `scripts/lint-baseline.sh --check --clean-cache` warm (cut
    from this window's hygiene by the report deadline).
13. Battery receipt: confirm /tmp/ci-local-full-1945.log rc; triage any
    newly-tripped gate per the cheap-gates-first ritual.
14. `tq doctor` on the deployed host after the flip (crush pin 0.99.0
    WARN check) — rides the deploy visit.
15. Consider folding the wc-l marker-gate helper into the smoke's
    taskq.py if a third stall-gated leg ever lands (two copies now).
16. The `docs/drafts/2026-11-01_18-01_crush-server-env-propagation-issue.md`
    can retire into the planning doc's R2 once #4210 moves (housekeeping).
17. Row 1449-adjacent TODO row "L4 pilot on ONE repo + shared-server
    memory measurement" — the RSS watch now feeds it; re-scope the row
    to "read one week of crush-server-rss.log" after the flip.
18. If a second pool ever adopts `--crush-server`: extract the smoke's
    busy-probe into scripts/ (third copy would justify it).

## g) QUESTIONS FOR THE OWNER (cannot self-answer)

1. **Push GO?** (third window asking): master is ahead of origin with
   CI's latest remote run red on an ANCESTOR commit. Pushing re-runs CI
   on the fixed tree. I have not pushed — your call, one word.
2. **Flip timing**: my SystemNix staging assumes the socket activates
   WITH the next lock float ("same change" per the 2026-10-07 rule).
   Alternative you mentioned: hold in-process for one more watched wave
   after pushing, THEN float+flip. Which?
3. **AGENTS.md budget**: 2 bytes spare (24398/24400). Keep
   prune-to-fit discipline, or reset the guard to a new conscious
   ceiling (e.g. 25000) so rows stop costing surgery?

— recorded 19:50; battery rc lands in /tmp/ci-local-full-1945.log after
this file (b above); no verdict channel (owner-directed window, no task
id).
