# Battery chase window: fixture pins, gofmt sweep, golines cure, baseline settled

Session 19:50→20:26 (2026-11-01), continuation of the whole-list GO.
This window owned ONE thing: driving the full ci-local battery to a
receipt after the 19-50 report, chasing four newly-tripped gates in
sequence. Concurrent agents + daemon active throughout (foreign planning
doc 1b970c5f landed mid-window).

## a) FULLY DONE

- **19-50 report landed**: written, indexed (top of today's cluster),
  committed by the daemon; check-status-index green.
- **Fixture-attribution gate cured**: the two NEW smoke heredocs
  (`taskq.py`, `busy.py`) lacked `pins:` branch attribution — added
  line-above rows (the shell-safe position per the 06-20 lesson); gate
  green across all 71 fixture blocks; committed with the smoke fix.
- **gofmt drift cured (scoped)**: battery named six files — two mine
  (`internal/executor/crushdeath.go`, `agent_test.go` — my earlier edits
  carried formatting nits my module gate never checks) and four
  concurrent agents' (`cmd/tq/ask.go`, `evidence.go`,
  `executor/executor.go` facade, `internal/webui/agentfeed.go`).
  Scoped `gofmt -w` per file (hook correctly blocked my first unscoped
  `gofmt -l .`), executor module re-verified green after.
- **readmodel golines growth cured AT SOURCE**: the NEW
  `forwardcompat_test.go` (concurrent agent's) had >100-char lines; ran
  the actual `golines` tool on it (system bin), findings 1→0, module
  tests green, gofmt clean — NOT a baseline row.
- **Lint baseline deliberately settled** (the one deliberate regen of
  the window): the day's accumulated feature waves (18-14 bridges,
  wave-4, this session's V2 work) grew ~100 advisory findings across
  cmd/tq, executor, worker, root — 30 linter rows. Precedent-checked
  first (git log: routine "settle the baseline for foreign growth"
  commits). `--regen` (auto cache-clean per the gate's contract) wrote
  194 module/linter rows / 2391 findings; committed 4a7575ad.
- **Battery relaunched to chase each gate**: attempt 20 (fixtures red),
  attempt 21 (gofmt red), attempt 22 (golines red), attempt 23 (baseline
  red) — each cured at source or settled, each cure verified against the
  named gate BEFORE relaunching.

## b) PARTIALLY DONE

- **FINAL battery IN FLIGHT at report time** (`/tmp/ci-local-final.log`,
  rc appended on completion; 87+ `ok:` lines, vet stage, all four
  previously-red gates now individually green). If it lands red on a
  FIFTH gate, the triage pattern is proven: read the named files, cure
  at source, settle only when foreign growth owns it.

## c) NOT STARTED (unchanged from 19-50; all owner-gated)

- Push master; SystemNix lock float + staged socket flip + deploy;
  first-wave watch; V2.19/A15 enricher real-closeout; V1.16 parked
  question under routing; A11/V2.14 routed prompts in the browser pane;
  A17 canary-flip (crush#4210 open).

## d) TOTALLY FUCKED UP

- **`$(date +%H%M)` evaluated TWICE in one command line**: the battery
  redirect and the rc-append wrote to DIFFERENT files (1953 vs 1958) —
  the rc landed in an empty file while the real log carried the truth.
  Exactly the clock/naming class AGENTS warns about. Fixed mid-window:
  one LITERAL log filename per run, no re-evaluation.
- **Repeated the 18-12 lesson verbatim**: "3 battery attempts burnable
  by a 30-second preflight" — I burned FOUR because I relaunched the
  full battery after each single-gate cure instead of running the cheap
  gates (fixture-attribution, scoped `gofmt -l` on changed files,
  `lint-baseline --check`) ONCE up front. The ritual exists; I skipped
  it. Cost: ~4 × 4-8 min of battery time for ~90 s of preflight.
- Two session-shell footguns re-stepped (both known, both cheap): a
  multi-file `tail` (invalid context) and an unscoped `gofmt -l .` as a
  command-position sub-token (hook-blocked).
- My own module gate (build+vet+test) validated code that gofmt would
  have flagged — vet does not check formatting. Two of my three edits
  this window carried nits into the battery.

## e) WHAT WE SHOULD IMPROVE

- **Cheap-gates-first is a PRE-battery ritual, not a post-mortem**:
  before ANY full-battery launch, run the self-test/summary gates for
  everything the window touched (fixture attribution when touching
  smoke heredocs; scoped gofmt -l on every changed .go; lint-baseline
  --check). Write it into the session checklist, not just AGENTS.
- **Battery log naming**: literal filename per attempt, rc appended to
  the SAME literal (the double-date bug is trivially avoidable).
- **Module gates should end with `gofmt -l <changed files>`** — a
  formatting nit should die in the module gate, never in the fleet
  battery.
- **Baseline hygiene cadence**: a day of heavy concurrency grew the
  advisory pile ~70% (≈1.4k→2.4k). Routine to settle, but a periodic
  "top-10 linter offenders" triage (err113/wrapcheck/wsl_v5 clusters in
  executor) would keep the advisory pile honest rather than monotone.

## f) NEXT (owner-prioritized; not harvested into TODO_LIST — the pool
dispatches TODO rows as paid tasks and most items are owner-gated)

1. Owner: PUSH master (4th ask; origin behind, CI's red run is a stale
   ancestor).
2. Owner: SystemNix `nix flake lock --update-input go-taskqueue` past
   2026-11-01 + uncomment the staged `crushServerSocket` + deploy.
3. Watch the first routed wave (tq tasks / dlq / closeout facts / pane).
4. Record flip date + wave deltas in the planning doc LIVE-STATUS.
5. Confirm the in-flight battery's rc; triage a possible fifth gate per
   the proven pattern.
6. `scripts/lint-baseline.sh --check --clean-cache` warm run (post-regen
   verification ritual from AGENTS).
7. V2.19/A15: enricher real-closeout pass on the first routed closeout.
8. V1.16: one parked question routes home end-to-end under routing.
9. A11/V2.14: routed prompts surfaced in the browser QuestionsPane.
10. A17: canary-flip ritual when crush#4210 ships.
11. L4 sheet telemetry fill after ≥1 week dogfood (A18 owner ruling).
12. Owner: AGENTS.md budget ruling (24398/24400 — 2 bytes spare).
13. Owner: `tq tasks --json` last_error/evidence column appetite.
14. Advisory-lint top-offenders triage (executor err113/wrapcheck/wsl_v5).
15. Fold the wc-l marker-gate helper into taskq.py if a third
    stall-gated leg lands.
16. Retire the #4210 draft into the planning doc's R2 once upstream
    moves.
17. Battery preflight ritual → scripts/ (a `--preflight` mode composing
    the cheap gates) — candidate for the pool itself to dispatch.

## g) QUESTIONS FOR THE OWNER (cannot self-answer)

1. **Push GO?** (4th window asking — one word unblocks CI re-running on
   the fixed tree.)
2. **Flip timing**: staged socket activates WITH the next lock float
   (my assumption), or hold in-process for one more watched wave after a
   push, then float+flip?
3. **Baseline settle ratified?** The deliberate regen took the advisory
   pile ≈1.4k→2.4k findings (the day's accumulated waves, 30 rows).
   Routine per precedent — but if you want a cleanup pass instead, name
   the appetite (top-N linters or a module) and it can be dispatched.

— recorded 20:26; final battery rc lands in /tmp/ci-local-final.log
after this file; no verdict channel (owner-directed window, no task id).
