# ADR-0019 endgame — full migration onto go-cqrs-lite (auto-upgradeable), legacy removal

Date: 2026-10-03 · Owner directive: "FULLY migrate to metaengine/ + system/
in a backward auto-upgradeable way; remove all legacy ASAP." · Status:
EXECUTING (this window)

Base state verified at HEAD 563c30c3 (+ vendored module sync):

- S1 LANDED (f0643178): `internal/queue/{sqlite,postgres}` are thin drivers
  over `{sqlitev4,postgresv4}` over upstream `queue/{sqlite,postgres}/v4`;
  hand-rolled engines and mirrored suites deleted; finalizes are
  token-fenced (upstream ADR-0134).
- S2 PARTIAL: only the flip-independent slice shipped (AppendFact rides the
  engine `FactTx.WithFacts` sink). Consumers still speak tq
  `journal.Fact` (`Detail jsontext.Value`) mapped from engine facts at
  companion reads — the two-vocabulary split ADR-0019 S2 exists to kill.
- S3 CODE-COMPLETE, FLAGGED OFF: `internal/readmodel` metaengine Store
  collections in `<db>.readmodel.db`; `--read-model` default false on
  stats/api/serve; parity harness `internal/readmodel/parity_test.go`.
- S4 NOT STARTED: composition map archived 2026-10-01 (verdict table:
  read side rides S3; worker loops/harvest/process supervision stay
  `runactor` — system/ wraps, never replaces).
- NO AUTO-UPGRADE: `sqlitev4/replay.Migrate` is an unwired library; the
  production dogfood journal (`/mnt/pool/services/tq/tq.db`) is still on
  the pre-flip schema, so the next deploy needs transparent on-open
  upgrade or the pool breaks.
- `internal/queue/cqrsqlite`: a third S1 spike (owner-string finalize
  emulation) referenced only by the conform suite — legacy once S2 lands.
- vendor/ was stale vs go.mod (fixed this window: `go mod vendor`, build
  green).

## Phases (each ships green; gates listed per phase)

### P0 — Baseline green ✅ (2026-10-03)

`go mod vendor`; root `go build ./... && go vet ./...` green;
`go test ./... -race -count=1` green; nested-module loop green; committed
as the starting point.

### P1 — Backward auto-upgrade on open (unblocks the next deploy)

`sqlitev4.Open` (hence the `internal/queue/sqlite` facade and every
caller) detects a legacy pre-flip database (old `tasks`/`facts` schema,
no engine tables) and upgrades in place BEFORE serving:

1. Probe (read-only): engine tables absent + legacy `tasks` present.
2. Replay: `replay.Migrate` into `<path>.upgrade-tmp` (facts never lie —
   ADR-0019 data-migration design).
3. Swap: atomic rename; legacy file preserved as `<path>.legacy.bak`
   (never deleted — owner can diff/rollback).
4. Kill switch: `TQ_NO_AUTO_UPGRADE=1` makes Open fail with a loud error
   naming the manual `replay` path instead of upgrading.

Gates: new upgrade tests over generated legacy fixtures (round-trip:
legacy fixture → Open → row/fact equality vs replay.Migrate baseline),
sqlitev4 module suite, root build/vet/test, cmd/tq gate.

### P2 — S3 flip: readmodel becomes the default read side

`--read-model` default flips to TRUE on `tq stats`, `tq api`, `tq serve`
(escape: `--read-model=false` during probation). Row-rich board/table
views stay store-backed per the S3 design; `TestRoutesAreReadOnly`,
health-CSP pins, and the parity harness stay green. Usage text updated to
say the metaengine projection is the default.

Gates: readmodel parity + module suites, cmd/tq gate, webui/httpapi
smokes (`scripts/smoke/`), root gates.

### P3 — S2: one journal vocabulary (rides upstream `facts.Fact`)

`internal/journal`: `Fact = facts.Fact` (type alias), `FactType =
facts.FactType` (open string); tq-only families (`session.*`,
`question-*`, budget/scorer facts) become tq-side constants on the open
type. `Detail` becomes `[]byte` — sweep all `jsontext.Value` Detail sites
(jsontext is imported today only in journal + journal/cqrs among
consumers; 79 non-test `.Detail` usages repo-wide). `internal/journal/cqrs`
re-points at the unified journal (ADR-0014 adapter, sanctioned by the
ADR-0019 S2 text). Memory journal + tests follow. companion `scanFacts`
stops translating vocabulary (row scan → facts.Fact directly).

Gates: every touched module's loop, root build/vet/test -race, cmd/tq
gate, `check-facade-parity.sh`, `check-go-mods.sh`.

### P4 — S4: composition via system/ (the sanctioned shape, not a rewrite)

Per the archived composition map: a ROOT-module internal composition
package (root go.mod carries the `system/v4` require — map §5 option A;
cmd/tq stays replace-free) wraps runtime construction in `system.New`:

- DeploymentConfig: sqlite engine at `<db>.readmodel.db` (durable engine
  for projections/checkpoints — §4a/§4b defaults: `system_checkpoints`
  home; sweeper ticking STAYS runactor).
- DomainConfig: the readmodel collections declared as system projections
  where the feed maps 1:1; readmodel's projector cursor moves onto the
  system checkpoint store; tq `queue.Store` stays injected (S1 seam,
  NOT moved into DeploymentConfig — map §6.3).
- Lifecycle: `GracefulClose` owns close ordering for serve; agent-pool
  keeps runactor first-exit/interrupt semantics (§4c "adopt per
  surface").
- No second source of truth: system's own event journal stays unused
  (empty DomainConfig events); the queue engine journal IS the journal
  (the §3 split-brain gate is discharged by P3 landing first).

Gates: root build/vet/test -race + module loop + cmd/tq gate + smokes +
`go mod vendor` + nix vendorHash regen (`.#checks...vendor-hash`) +
lint-baseline `--check`.

### P5 — Legacy deletion + docs truth pass

- DELETE `internal/queue/cqrsqlite` (superseded spike) + its conform-suite
  wiring (suite keeps running for sqlitev4/postgresv4).
- Remove flag-divergence leftovers from P2 (dual tally paths collapse to
  the projection-backed reads where the S3 design says so).
- Docs: ADR-0019 stages marked landed (S2/S3/S4 + auto-upgrade), AGENTS.md
  platform section rewritten to the endgame truth, FEATURES.md rows
  re-pointed, CHANGELOG entry, TODO_LIST ADR-0019 rows closed (dogfood
  cutover row stays owner-blocked), LOC + art-dupl delta report.
- Full gate matrix: ci-local equivalent + all-module loop + cmd/tq +
  smokes + guards (`check-*.sh`).

## Pareto lens

- 1% → 51%: P1 (auto-upgrade — production unblock) + P2 (readmodel
  default — the S3 user-visible flip). Small diffs, outsized value.
- 4% → 64%: P3 (one vocabulary — kills the last split brain; gates S4).
- 20% → 80%: P4 + P5 (system composition root + legacy deletion — the
  "fully migrated" claim becomes literally true).

## Invariants held throughout

- Facts in the same tx as state (engine-enforced since S1) — never
  hand-INSERT a fact again (S2 memo §2).
- tq `queue.Store` stays the in-repo boundary; facades keep compiling
  (ADR-0016); facades never gain low-level requires.
- Single serialized writer on sqlite; task contexts survive pool
  shutdown; no unauthenticated oracles; CSP pins green.
- Concurrent-agent discipline: commit before gating; never revert
  foreign changes; re-read before editing.
