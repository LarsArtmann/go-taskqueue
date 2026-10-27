# SUPERB Pareto Activation Plan — Fleet Greening & Backlog Triage

Created: 2026-10-25 15:40 CEST · Source: TODO_LIST.md at HEAD `ef497944` (569 unchecked rows, 30+ sections)
Method: pareto-planning skill (Step 0: prior plans + reports read; top-tier rows re-verified at HEAD this session).
Supersedes execution-order guidance in `docs/planning/2026-10-13_12-10_SUPERB-PARETO-ACTIVATION-PLAN.md` for everything it left open; rows landed since (1053/1057/1058 tick-audit + tick-claim gate + tick-cite conventions; 1018 verdict-at-exit; 1019 branching-flow inventory) are recorded in the errata.

## Context — where the system stands

- Master pushes trigger CI; several tooling gates stand red at master (owner row: "four standing tooling-gate reds") and the 2026-10-13 CI-red triage section still lists govulncheck/test-windows classes. Every push pays triage attention until these are green or explicitly accepted.
- The backlog itself has a trust problem: 769 dead SHA citations, false "row N ticked" claims (gate shipped 2026-10-25, baseline 10 WARN incl. rows 440/834 residue), check-todo-list exits 1 on foreign owner-gated rows so its exit code is untrustworthy.
- Process fixes from this week (tick-audit, verdict-at-exit, tick-claim gate) are landed; their queue-side/managerial halves are still open rows.
- Docs corpus: docs/status/README.md carries 939 live index rows vs the 400-row threshold; docs-health sweeps (Sept floors) left ~65 unrouted rows; freshest harvest (11th dispatch) adds 25.
- Toolchain: module floors 1.27.3 but tracked `cmd/tq/go.work` still says 1.27.2 (one-line fix; the mismatch produced real LSP/gate noise all week).

## Errata — stale or landed items dropped from ranking (verified 2026-10-25)

| Item                                                                    | Evidence                                                                                                        |
| ----------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------- |
| Rows 1053/1057/1058 (TICK-AUDIT, tick-claim gate, tick-cite convention) | Landed 2026-10-25 (`check-tick-claims.sh` live 318 ok / 10 WARN; AGENTS.md §Conventions)                        |
| Row 1018 (verdict-at-exit) + row 1019 (branching-flow inventory)        | Landed 2026-10-25 (AGENTS.md close-exit clause; status.go Finish rail strengthened; ci-local dupe step comment) |
| Rows 776-778 / 233-238 canary + frozen-date batches                     | Ticked long ago; only false-claim residue (440/834/128/129/131) remains                                         |
| "rows 896 ticked" freshness claims                                      | Ticked at HEAD; only the freshness-convention question remains (row 1127 sibling)                               |

## Tier 1 — the 1% that delivers 51%

Six items. Everything else is noise while master's gates lie.

| #  | Task (rows)                                                                                                                                                                                                 | Min | Why it is the 1%                                                                       |
| -- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --- | -------------------------------------------------------------------------------------- |
| T1 | Green-the-fleet ruling + execution batch: inventory the 4 standing reds (rows ~903, 871/1071), the 2 remaining CI-red classes (govulncheck, test-windows; CI-red section 34 rows), fix or owner-accept each | 100 | Master red = every push pays triage; blocks trusting every other gate                  |
| T2 | Fix `cmd/tq/go.work` floor 1.27.2→1.27.3 (11th-dispatch row) + re-run gates                                                                                                                                 | 12  | One line; kills the go.work/toolchain mismatch noise (LSP + toolchain gates)           |
| T3 | `workflow_dispatch:` trigger on ci.yml (11th-dispatch row)                                                                                                                                                  | 12  | Makes focused test-windows/gate reruns executable — prerequisite for T1 iteration      |
| T4 | check-todo-list exit semantics: advisory WARNs exit 0, only structural violations fail (owner-gated row, 2026-10-24 closeout §e4)                                                                           | 60  | The TODO-trust gate's exit code is currently meaningless (red on foreign rows)         |
| T5 | Dangling-citation debt: owner ruling (baseline vs heal) + first `git log --all` heal wave over the 769 dead SHAs (row 30 + 2026-10-25 row)                                                                  | 100 | Dead cites make every historical claim unfalsifiable; gate is red at master until then |
| T6 | Landed-work tick sweep + audit: tick row 440 (still `- [ ]`, claimed twice), 834/128/129/131 residue, then grep-audit other false "(row N ticked)" closeout claims (11th-dispatch audit row)                | 60  | The backlog must tell the truth before it can be ranked                                |

## Tier 2 — the 4% that delivers 64% (Tier 1 + 16 tasks)

| #   | Task (rows)                                                                                                                                                         | Min |
| --- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --- |
| T7  | Queue-side guard: review-fix tasks completing with zero attributable output → auto-DLQ/flag (review-fix twins row, 11th dispatch)                                   | 60  |
| T8  | Fixture-attribution: attribute the 7 unattributed heredocs (check-gosec.sh:159/168/178 et al.) → check-fixture-attribution green (11th dispatch)                    | 30  |
| T9  | CI parity: run the 3 new countable gates' self-tests (check-webui-js, check-toolchain-pins, check-poll-loudness) in ci.yml (11th dispatch)                          | 30  |
| T10 | Fleet gate board: one standing red/green surface at HEAD (ci-local `--summary` or `tq doctor` section) (11th dispatch)                                              | 60  |
| T11 | Index bloat: ANNOTATE+ARCHIVE sweep over the 2026-10-20 done-prompt cluster (939→≤400 live rows) (11th dispatch + owner digest question)                            | 100 |
| T12 | ProjectionRuntime robustness: nil-Host read-only mount pin (cmd/tq) + demoted-serve staleness contract (SSE/stats freshness documented or fixed) (11th dispatch ×2) | 90  |
| T13 | webui smoke forensics: keep TMP + dump serve/worker/auth logs on ANY failure; auth wait loop FAILs with log tail (11th dispatch ×2 + owner-fold row)                | 45  |
| T14 | O14 signature growth policy: owner ruling (closed-set vs mint-on-move) + implement (O14 follow-ups section)                                                         | 45  |
| T15 | Self-test ordering guard: assert each countable gate's --self-test precedes its real gate in ci-local (owner-gated row, dce6c12b class)                             | 45  |
| T16 | heal-daemon-sweep: span-terminator bug + UX (`--from` hint on refusal) (rows 46/47)                                                                                 | 45  |
| T17 | check-webui-css output guard (refuse < N lines before advising rebuild) (11th dispatch)                                                                             | 30  |
| T18 | renameWithTransientRetry graduation to shared home after row 906's audit (11th dispatch)                                                                            | 30  |
| T19 | COVERAGE.md freshness gate + check-ci self-test env guard (11th dispatch ×2)                                                                                        | 30  |
| T20 | scripts/commit-task.sh success echo (hash + subject) (row 32) + root-battery-before-tick discipline note (row 31) rollout into conventions check                    | 30  |
| T21 | DLQPathFor sidecar cluster: unexported `dlqPathForDeployment`, doctor legacy surfacing, guard script (rows 39/41/43/44)                                             | 100 |
| T22 | gitignore the `.readmodel.db.fold-lock` sidecar family (11th dispatch)                                                                                              | 12  |

## Tier 3 — the 20% that delivers 80% (Tiers 1-2 + the live workstreams)

Medium-granularity tasks (30-100 min each), sorted by impact/effort:

| #   | Task                                                                                                                                | Rows covered                | Min |
| --- | ----------------------------------------------------------------------------------------------------------------------------------- | --------------------------- | --- |
| T23 | WorktreeInFlight + OR-audit batch 1 (audit half: every os.Rename/terminal-status consumer)                                          | 17-row section, first half  | 90  |
| T24 | WorktreeInFlight + OR-audit batch 2 (pins + docs)                                                                                   | 17-row section, second half | 90  |
| T25 | cqrs-lint A014/D013 upstream batch: triage the 21 branch follow-ups into go-cqrs-lite PRs vs local rulings                          | 21-row section              | 100 |
| T26 | Release-hardening harvest batch (wait-loop extraction, http_code split, --publish-steps-only, reflow pins, self-test, log-line pin) | 14-row section              | 100 |
| T27 | Zero-deployment freeze-class residue + 10-03/10-04 window follow-ups                                                                | 12 + 12 rows                | 100 |

## Tier 4 — the remaining 20% to reach 100%

Batched by section, each batch = one medium task slot (30-100 min), executed in this order after Tiers 1-3:

| Batch | Sections                                                                                                    | Rows            |
| ----- | ----------------------------------------------------------------------------------------------------------- | --------------- |
| B1    | Docs-health 2026-10-11 fifth sweep floor                                                                    | 16              |
| B2    | Docs-health 2026-10-05 fourth sweep floor                                                                   | 16              |
| B3    | Docs-health 2026-09-21 archive sweep floor                                                                  | 17              |
| B4    | Docs-health 2026-09-16 window harvest                                                                       | 16              |
| B5    | 2026-10-17 windows (five-task 9 + row-110 4 + row-111 2 + footer-gate 8)                                    | 23              |
| B6    | 2026-10-20 appends (00-31: 9, dupe-baseline: 8, 02:31: 10, fifth-dispatch: 9, third-dispatch residue)       | ~38             |
| B7    | NAF-surface grid gaps + 2026-10-13 02-13 window                                                             | ~14             |
| B8    | Dogfood round + Process follow-ups + 09-17 harvests                                                         | ~30             |
| B9    | Fleet/deploy + webui P2 + owner-gated BLOCKED rows (SystemNix row 38 etc. — owner-run)                      | ~30             |
| B10   | Long tail: pre-2026-09-16 unsectioned rows + ROADMAP v0.4 Intelligence candidates (re-row before execution) | remainder ≈ 120 |

Coverage check: Tiers 1-3 (≈90 rows) + B1-B10 (≈349 rows) + Tier-1/2 section overlaps + already-ticked residue ≈ 569 unchecked rows accounted for; every open row maps to exactly one batch. Owner-BLOCKED rows stay parked until answered (harvester skips them by convention).

## Execution graph

```mermaid
flowchart TD
    subgraph T1["Tier 1 — 1% → 51% (trust & signal)"]
        T2[T2 go.work floor fix] --> T3[T3 workflow_dispatch]
        T3 --> T1[T1 green-the-fleet]
        T4[T4 todo-list exit semantics] --> T1
        T5[T5 dead-SHA ruling + heal wave] --> T1
        T6[T6 landed-work tick sweep] --> T4
    end
    subgraph T2x["Tier 2 — 4% → 64% (closeout & gate trust)"]
        T7 --> T10[T10 fleet gate board]
        T8[T8 fixture attribution] --> T10
        T9[T9 CI self-test parity] --> T10
        T11[T11 index-bloat archive sweep]
        T12[T12 ProjectionRuntime pins]
        T13[T13 webui smoke forensics]
        T14[T14 O14 policy] --> T15[T15 self-test ordering guard]
        T16[T16 heal UX] --> T20[T20 commit-task echo]
        T17[T17 webui-css guard] --> T13
        T18[T18 retry graduation]
        T19[T19 COVERAGE + check-ci guards]
        T21[T21 DLQPathFor cluster]
        T22[T22 fold-lock gitignore]
    end
    subgraph T3x["Tier 3 — 20% → 80% (live workstreams)"]
        T23 --> T24
        T25[T25 cqrs-lint upstream]
        T26[T26 release hardening]
        T27[T27 freeze + window residue]
    end
    subgraph T4x["Tier 4 — remaining 20% → 100%"]
        B1[B1-B4 docs-health floors] --> B5[B5-B8 windows & process]
        B5 --> B9[B9 fleet/deploy + owner-gated]
        B9 --> B10[B10 long tail + v0.4 re-row]
    end
    T1 --> T7
    T1 --> T11
    T10 --> T23
    T27 --> B1
```

## Medium plan (30-100 min, ALL todos) — T1-T27 + B1-B10 above is the authoritative list

## Fine plan (≤12 min each) — first 40 of the execution queue, rest generated per batch on dispatch

| #       | Subtask                                                                                                       | Parent | Min  |
| ------- | ------------------------------------------------------------------------------------------------------------- | ------ | ---- |
| F1      | Edit `cmd/tq/go.work` go directive to 1.27.3; run test-cmd-tq + toolchain-pins gate                           | T2     | 12   |
| F2      | Add `workflow_dispatch:` to ci.yml; actionlint; scratch dispatch dry-run                                      | T3     | 12   |
| F3      | Inventory the 4 standing reds: capture current failing jobs + gate outputs to a dated file                    | T1     | 12   |
| F4      | Classify each red: fix-here vs owner-accept; open/refresh one row per red with evidence                       | T1     | 12   |
| F5      | Fix red #1 (govulncheck class): root-cause + patch + local gate rerun                                         | T1     | 60   |
| F6      | Fix red #2 (test-windows class): reproduce via workflow_dispatch, patch, rerun                                | T1     | 60   |
| F7      | Rewrite check-todo-list exit semantics + add self-test pin (advisory rc=0)                                    | T4     | 60   |
| F8      | Draft dangling-cite ruling memo (baseline vs heal, cost estimate per option) → owner row                      | T5     | 30   |
| F9      | Heal wave 1: resolve 100 oldest dead SHAs via `git log --all` subject match + fork-record arrows              | T5     | 60   |
| F10     | Tick row 440 with landed-evidence citation or re-row as genuinely open                                        | T6     | 12   |
| F11     | Tick/annotate 834/128/129/131 residue via check-tick-claims WARN list                                         | T6     | 12   |
| F12     | Grep-audit "(row N ticked)" closeout claims; file false hits as review-fix rows                               | T6     | 30   |
| F13     | Design review-fix zero-output guard (store-side completion-without-output detector)                           | T7     | 30   |
| F14     | Implement + self-test the review-fix guard; wire into ci-local                                                | T7     | 30   |
| F15     | Attribute check-gosec.sh:159/168/178 heredocs; rerun check-fixture-attribution                                | T8     | 30   |
| F16     | Extend ci.yml with the 3 countable-gate self-tests; actionlint + push smoke                                   | T9     | 30   |
| F17     | Implement fleet gate board (`tq doctor` section first draft)                                                  | T10    | 60   |
| F18     | ANNOTATE+ARCHIVE wave A: 2026-10-20 done-prompt cluster (≈200 index rows)                                     | T11    | 100  |
| F19     | Pin projectionPump nil-Host read-only mount (cmd/tq unit test)                                                | T12    | 45   |
| F20     | Document/implement demoted-serve staleness contract (SSE + /api/stats)                                        | T12    | 45   |
| F21     | webui smoke: keep TMP + log dump on failure                                                                   | T13    | 30   |
| F22     | webui smoke auth-wait FAIL with log tail                                                                      | T13    | 15   |
| F23     | Owner memo + implementation for O14 signature growth policy                                                   | T14    | 45   |
| F24     | Implement self-test-ordering guard with --self-test pin                                                       | T15    | 45   |
| F25     | Fix heal-daemon-sweep span terminator + add regression self-test                                              | T16    | 30   |
| F26     | heal-daemon-sweep UX: print exact `--from` invocation on refusal                                              | T16    | 15   |
| F27     | check-webui-css output-plausibility guard                                                                     | T17    | 30   |
| F28     | Graduate renameWithTransientRetry after row 906 audit confirms                                                | T18    | 30   |
| F29     | COVERAGE.md freshness gate                                                                                    | T19    | 15   |
| F30     | check-ci self-test env guard                                                                                  | T19    | 15   |
| F31     | commit-task.sh success echo                                                                                   | T20    | 12   |
| F32     | Root-battery-before-tick: conventions note + gate-adjacent reminder                                           | T20    | 18   |
| F33-F36 | DLQPathFor cluster: introduce `dlqPathForDeployment`, route call sites, doctor legacy surfacing, guard script | T21    | 4×25 |
| F37     | gitignore fold-lock sidecar family                                                                            | T22    | 12   |
| F38-F40 | WorktreeInFlight audit sweep part 1 (enumerate consumers, classify, pin first two)                            | T23    | 3×30 |

Fine tasks for T24-T27 and B1-B10 are generated at dispatch time from their section rows (each row is ≤12 min of work by the TODO_LIST contract: "items agent-executable, one per line"); the row ids above guarantee full coverage.

## Verification contract for every task

- Edit→stage→commit BEFORE battery (daemon sweeps <60 s); ONE footer when a queue task exists.
- Gates: `check-todo-list`, `check-doc-refs`, `check-script-syntax`, `check-gomod-vendor-sync` (after Go changes), `test-cmd-tq` (after cmd/tq/AGENTS changes), O14 `check-release-docs` (after CHANGELOG changes); targeted module build+vet+test with `GOWORK=off GOEXPERIMENT=jsonv2`.
- Claims carry citations (gate output files, SHAs); gate receipts are `grep -c '^ok:'` over captured output, never context counts.
