# Fleet Red Evidence — Consolidated Inventory (2026-10-25 19:04 CEST)

One dated file consolidating every standing red class at HEAD, per the
SUPERB Pareto plan's F3/F4 (plan:
`2026-10-25_15-40_SUPERB-PARETO-FLEET-GREENING.md`). Receipts under
`/tmp/tq-gates/` are ephemeral; in-repo evidence (commits, gate scripts)
is cited alongside. Owner-gated remainders are marked.

## CI at origin (last runs before this file)

- `40207620242` (head f4ff9b1f, RED): nix job failed ONLY on treefmt —
  single file `internal/harvest/verdicts_test.go` stray blank line.
  Cured: `nix fmt` (daemon-folded); `nix build
  .#checks.x86_64-linux.treefmt` rc=0.
- `40205050267` (head 6f9b4ebc, RED): nix job failed ONLY on the
  app.css byte pin — committed stylesheet was UNMINIFIED (7,454 lines,
  183,147 B) vs the canonical `scripts/build-webui-css.sh --minify`
  output (1 line, 145,582 B; same tailwindcss 4.3.3 binary, bisect-
  confirmed). Cured: regenerated + committed `633a80c6`;
  `scripts/check-webui-css.sh` rc=0.
- Every other CI job (test, test-windows, govulncheck, gosec, go.mod
  health, …) was GREEN on both runs.
- Validation of both cures IN CI is pending the next master push
  (local master ~20 commits ahead; push is owner-gated).

## The "four standing tooling-gate reds" (row 1123's batch)

| Gate                           | State at HEAD                                 | Evidence                                                   |
| ------------------------------ | --------------------------------------------- | ---------------------------------------------------------- |
| check-script-syntax            | GREEN — 102 ok / 0 failed                     | row 871 receipt (`/tmp/tq-gates/syntax2.log`)              |
| check-guard-wiring             | GREEN — 68 wired / 0 orphaned                 | row 871 receipt (`/tmp/tq-gates/guardwiring2.log`)         |
| check-fixture-attribution      | GREEN — 25/25                                 | row 1110 tick, receipt `/tmp/tq-gates/fixattr2.log`        |
| check-todo-list exit semantics | RED-by-design (rc=1 on foreign advisory rows) | OWNER-GATED: structural-violation set ruling (F7/row 1138) |

## CI-red triage classes (2026-10-13 section, rows 892-893)

- govulncheck: FIXED (toolchain bump path; row ticked with URLs).
- test-windows: FIXED 2026-10-24 (CloseReadModel close-ordering +
  bounded transient rename retry), windows-confirmed run 40058586181.

## Local full-gate posture (this session)

- `check-toolchain-pins.sh`: 11 checks ok / 0 failed after the go.work
  surface pin (`TQ_TOOLCHAIN_EXTRA_GOWORKS` self-test branch; receipts
  `/tmp/tq-gates/toolpins-selftest-1852.log`, `toolpins-1852.log`).
- `cmd/tq` module gate (`scripts/test-cmd-tq.sh`, CMD_TQ_OS=windows):
  GREEN at 18:45 (`/tmp/tq-gates/cmdtq-1845.log`, 21.3s) — NOTE: the
  module is RED again at 19:04 from a CONCURRENT rollout: cmd/tq/
  agentpool.go:716 calls `readmodel.ReadWorkerFailedMarker`, which no
  published tag carries (newest = v0.4.0); the calling agent must tag
  internal/readmodel v0.4.1+ and bump cmd/tq (ADR-0017 forbids
  replaces). Not a standing class; tracked here for attribution only.
- Full `scripts/ci-local.sh` with `CI_CHECK=off` was still pending at
  file time (blocked on the same concurrent rollout for the cmd/tq
  leg); check-ci (unbypassed) reports the expected origin-red until
  the push lands.

## Remainders (owner-gated)

1. Push + confirm the next CI run GREEN end-to-end (both nix causes
   cured locally, unproven remotely).
2. css drift policy: heal-on-drift vs detect-only (report 17-30 §g Q2).
3. check-todo-list advisory-vs-structural exit semantics (F7).
4. Dangling-cite heal format + 769-SHA baseline (F8/F9).

## 2026-10-25 late-window append: gate-trust batch 3 outcomes

- check-todo-list rc=1 root cause found and cured: the daemon-sweep doc
  gate propagated the todo-list gate's advisory exit (row 1098 semantics
  shipped: advisories WARN, only structural violations exit 1; --self-test
  10 ok / 0 fail). This was killing ci-local BEFORE the expensive steps —
  not the dead-sha red.
- Four new countable gates shipped and wired self-test-first:
  check-gate-ordering (found + cured SEVEN live dce6c12b-class
  self-test-after-gate inversions), check-smoke-coverage (26/26 indexed),
  check-worktree-consumers (allowlist 2/0), check-tracked-ignored
  (0 tracked-ignored paths at HEAD).
- cmd/tq doctor tests: GREEN through the devmod shim (the plain-module vet
  red is the unpublished readmodel tag only). runGateBoard's injected-list
  parameter was dead code pointing at the package var — fixed; full
  test-cmd-tq.sh rc=0 (20.1s). Rows 1116 (amended) + 1119 closed.
- Two environmental ci-local killers cured: root vendor/ out of sync
  (sibling go.mod edit without `go mod vendor`) and
  scripts/smoke/new-module.sh committed 100644 on 2026-10-23 15:54 (every
  ci-local since dies at the scaffold smoke; ci.yml does not run it —
  local-only red). Receipt runs: 2032 died at todo-list advisory chain,
  2048 at vendor sync, 2058 at the exec bit, final run green-path.
- css row 1125: root cause = cold module cache resolves templ-components
  to the vendored copy (no templates/); cures at both ends
  (CSS_MIN_BYTES floor + commit-advice gate).
