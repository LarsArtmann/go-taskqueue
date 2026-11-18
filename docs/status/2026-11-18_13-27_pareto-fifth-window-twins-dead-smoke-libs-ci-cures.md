# Fifth-window Pareto execution — twins dead, smoke libs landed, CI cures ticked

- **Date:** 2026-11-18 13:27 CET (window: ~12:25–13:30)
- **Repo:** go-taskqueue · master · direct owner dispatch ("GET SHIT DONE!
  The WHOLE TODO LIST!"), concurrent agent session active

Batch: backlog-pareto master-plan continuation (docs/planning/2026-11-17_19-53_backlog-pareto-master-plan.md),
session under the standing dispatch; T36/T02/T12/T13/T14/T17/T18/T20 executed,
T15/T16 confirmed owner-gated, drift row minted.

## a) What was done

1. **Push + CI green (T36 closed)**: the 5-commit cure span (SC2164 shellcheck
   cure in `scripts/verify-fork-record.sh` + T10/T11 + wave-2 annotations +
   fourth-window report) was already on origin at resume; origin run
   **44294443292** on `02e657e2` completed **GREEN — all 8 jobs** (test,
   test-windows, test-postgres, nix, gosec, govulncheck, cqrs-lint, consumer).
   Verified 2026-11-18 by `gh run view --json jobs` (job-level conclusions,
   not summary claims).
2. **Tick batch (16 rows, 4 commits)**: T09/T10/T11 cure rows (105, 1825, 1835,
   1817, 101, 1872+1914 duplicate pair, 1411), CI-red rows (1687, 1709, 1385)
   citing green run 44294443292, T12/T13/T14 rows (1809, 1811, 1832, 1834),
   T17 row (1843), T18 row (1828). Every tick evidence-grepped against code
   first; every verdict cites the landing sha + the verifying gate.
3. **T12 — budget-twin drift class killed by GENERATION** (`c16a808d`):
   `scripts/check-agents-size.sh` no longer carries its own budget — it DERIVES
   it from `agentsDocMaxBytes` in cmd/tq/facts_json_test.go via a pinned sed
   parse; a failed parse fails LOUDLY at exit 2; `AGENTS_BUDGET_BYTES` stays
   the negative-path override. Drift is structurally impossible.
4. **T14 — check-agents-size --self-test + delta print** (`1656e384`): 5-check
   battery (derived-budget pin, underscore-literal const, override-beats-
   derived, loud parse-fail exit 2, real-file OK summary format) + headroom
   delta on the OK line; wired into ci-local self-test-first. The battery
   caught my own fixture arithmetic on first run (13 B vs 12 B) — worked as
   designed.
5. **T13 — pre-edit size rule lives in the session loop** (`5e44dd68`):
   tq-edit-advice's AGENTS.md note now computes LIVE size/budget/headroom from
   the real const at hook time and instructs: run check-agents-size BEFORE
   composing; prune first when headroom <100 B. The hook fired on my own
   AGENTS.md edit minutes later — dogfooded in-window.
6. **AGENTS.md drift cure** (`0f977393`): the load-flaky bullet had drifted
   from the gate's pinned per-test phrasing (`<Test> is load-flaky`) —
   check-verify caught it the moment check-ci stopped masking the battery
   (twice-masked local red). Restored compress-only; 27,598/27,600 B.
7. **T17 — shared stub-agent smoke lib** (`bb6f9714`):
   `scripts/smoke/lib/stub-agent.sh` owns the prompt-parse prologue + the
   FILE-ONLY `stub_verdict` rail; status-loop, dogfood-once, status-rewind,
   redaction emit only their case bodies. All four smokes live-green with a
   prebuilt TQ_BIN (rc=0 each, /var/tmp/pareto-battery/t17-*.log); lib
   --self-test green; wired into ci-local.
8. **T18 — crush_smoke_routed_run** (`467aa935`): the three hand-rolled
   routed-turn runners unified in lib/crush-server.sh with
   CRUSH_SMOKE_{BIN,TIMEOUT,PROMPT,PIN_FILES} knobs + a --self-test over a
   FAKE crush binary (seam argv, XDG isolation, pins branch, rc propagation).
   Memory smoke (heaviest user, 20+ concurrent calls) live-green.
9. **T20 — exec-bits gate** (`071223cc`): check-smoke-exec-bits.sh pins every
   `scripts/**/*.sh` executable (summary pin + 644-negative via
   TQ_EXEC_BITS_SCOPE + 755-positive); one-time chmod sweep of the five 644
   sourced libs verified via `git diff --summary`; guard-wiring 98/0.
10. **Crush-drift row minted** (`027f2c10`): route/pool smokes fail at HEAD —
    **pre-existing**, proven by a scratch-worktree A/B at the pre-T18 parent
    (`bb6f9714`): both fail identically WITHOUT the extraction. Since the
    2026-11-17 21:02 system-profile activation (new crush v0.101.0), route's
    M7 respawn-health leg and pool's verdict probe read red; memory stays
    green (its per-turn retry is idle-exit-tolerant by design). Receipts:
    /var/tmp/pareto-battery/t18-{route,pool}-PRET18.log.

## b) What was verified (gates, captured)

- `heal-daemon-sweep.sh --self-test`: 45 checks rc=0.
- `commit-task.sh --self-test`: rc=0 (footer-last, fold, sweep-below WARN,
  named-file residue).
- `check-agents-size.sh --self-test`: 5 checks rc=0; live 27,598/27,600.
- `check-verify.sh`: rc=0 after the phrasing restore (KNOWN_FLAKY↔AGENTS
  bidirectional check).
- `check-script-syntax.sh`: 142 checks ok / 0 failed (scoped runs over every
  touched script; full-tree run green).
- `check-guard-wiring.sh`: 98 wired / 0 orphaned (new libs counted via
  consumer sourcing; new gate wired).
- `check-smoke-exec-bits.sh --self-test` rc=0; live 142/142 after the sweep.
- Four stub-lib smokes: rc=0 each with TQ_BIN prebuilt
  (/var/tmp/pareto-battery/t17-*.log).
- `crush-server-memory.sh` smoke: rc=0 with the extracted runner.
- **A/B exoneration**: route + pool smokes fail IDENTICALLY at pre-T18
  `bb6f9714` in a scratch worktree — the extraction introduced nothing.
- Origin CI run **44294443292** GREEN (8/8 jobs); run 44302476463 (pushed
  span `027f2c10`) watched to completion — see §d.
- ci-local battery: progressed past check-ci + check-verify + exec-bits +
  stub-lib steps (104 ok-countable lines in
  /var/tmp/pareto-battery/t02-cilocal-final.log), stopped at docs-gates
  FRESHNESS (no date-named report at HEAD yet — this report IS that cure);
  re-run owed on the post-report tree.

## c) What did NOT land

- **T02 full-green ci-local receipt**: blocked by the pre-existing crush-drift
  smoke reds (route M7 + pool legs) + the freshness gate wanting this report.
  The battery is re-run after this report lands; the smoke reds are minted as
  their own row (drift decision is owner-visible: accommodate idle-exit vs
  pin the crush input).
- **T15/T16 (deploy cadence)**: owner-gated, sudo on evo-x2 — an agent cannot
  execute; rows 319/163/1942 stay BLOCKED. The 2026-11-17 systemd override
  contract doc (a0513ffc lineage) covers the deployed-unit split.
- **T19, T21–T32, T33–T35**: untouched this window (P2/P3 remainder; see §g).
- **Owner §g rulings**: three carried — answered with recommendations in §g,
  rows stay owner-gated (no RULING-BY-AGENT marker: nothing owner-pending was
  executed by agent choice).

## d) Pass counts (grep -c over captured files)

- Battery ok-lines: `grep -c '^ok:' /var/tmp/pareto-battery/t02-cilocal-final.log`
  → **104** (pre-freshness-abort run).
- docs-gates self-test: **6 ok / 0 fail** (captured in the same log).
- CI run 44294443292: `gh run view --json jobs` → **8/8 success**.
- Tick commits: 4 (`e078ccd5`, `62f9385d`, `6ddcda9e`, `4a44873e`, `027f2c10`
  carry ticks; 16 rows total).
- Work commits this window (pushed `02e657e2..027f2c10`): 14 (7 before the
  tick batch incl. the 5-commit cure span re-push, 7 after).

## e) Surprises

1. **check-ci masked two local reds**: the battery's early-abort at check-ci
   (red origin run) hid the check-verify AGENTS-drift red for two runs. Any
   battery run that stops at check-ci is NOT a clean receipt — run it to
   completion with CI_CHECK=off when the tree is the subject under test.
2. **The daemon swept T17 and T18 mid-flight** (two-fold each): both healed by
   the documented exactly-mine verify + soft-reset fold; T17 folded two sweeps
   into `bb6f9714`, T18 split cleanly into code+tick (`467aa935`+`4a44873e`).
   The commit-task named-file verify + sweep-below WARN (T11, landed earlier
   today) are the rails this session used.
3. **The self-tests caught MY OWN errors three times**: fixture byte-count
   (check-agents-size battery), inverted negative assertion (crush-server lib:
   empty `TQ_RESULT_FILE=` is the GOOD bare state), and a hook path bug
   (one-level-up repo root). Fleet convention vindicated.
4. **Yesterday's system rebuild is the probable root of the smoke reds**: new
   crush v0.101.0 in the 2026-11-17 21:02 profile; route/pool asserts are
   drift-intolerant, memory's design is not. A/B evidence in the minted row.
5. **Row anchors stale again**: TODO_LIST line numbers moved since the plan
   was written (1600-row file, concurrent agents) — all ticks located by
   content grep, never line number.

## f) Judgment calls

- **T12 shipped as a generator, not a checker**: deriving the bash budget from
  the Go const kills the drift class outright; a checker would only detect it.
  The row explicitly allowed "(or generator)".
- **T17's lib lives at `scripts/smoke/lib/`, not the row's `scripts/lib/`**:
  guard-wiring's consumer-source exemption is smoke-dir-scoped and
  crush-server.sh set the smoke-lib precedent; the tick note states the
  deviation openly.
- **Route/pool reds NOT papered over**: weakening M7's health assert to green
  the battery would have hidden a real crush-behavior drift. The row presents
  both remedies (accommodate idle-exit / pin crush input) for the owner.
- **T02 receipt honestly partial**: no green claim without a green battery;
  the partial receipt (104 ok-lines, exact abort point) is captured and the
  re-run is queued post-report.

## g) Open items + owner questions

**Rulings carried (×3) — recommendations attached, rows stay owner-gated:**

1. **§g1 archive-vs-routed (T03/T04 archive half)**: the wave-2 digest's
   archive half is honestly zero — reports whose open items were routed
   inline (`**→ …**` markers) are not archive candidates until the owner
   rules whether routed-clean reports may archive. Recommendation: allow
   archiving when EVERY open item carries a routed marker or inline verdict —
   routing IS the residue; otherwise the archive ritual dead-locks on the
   routed class.
2. **§g2 push policy for CI cures (row 1387)**: today's evidence — the
   origin-red cure pushed and went green (44294443292), while TWO local reds
   hid behind check-ci's early abort. Recommendation: keep origin-CI-first
   for cures, but require the local battery to run TO COMPLETION
   (CI_CHECK=off) before any push — aborting at check-ci is not a receipt.
3. **§g3 battery home**: `/var/tmp/pareto-battery/` held every receipt this
   window and survived where /tmp was wiped between windows. Recommendation:
   confirm it as the standing battery home (out of the gated tree, host-local,
   daemon-blind).

**Remaining plan work**: T19 (shared negative-fixture helper), T21/T22 (SSE
triage + keep-set pins), T23 (commit-msg pin batch), T24 (erraudit sibling —
other repo), T25/T26 (go-cqrs-lite archive ritual — other repo), T27/T28
(IO-review in-repo rows), T29–T32 per plan, T33–T35 + pool dispatch wave (P3),
T15/T16 (owner sudo), crush-drift row cure (new).

**Premises to re-check next window**: the TODO row anchors again (file grows);
the crush-drift row's claim (re-run route/pool after any crush input change);
AGENTS headroom is 2 B — the next AGENTS edit MUST prune first (the hook now
says so live).
