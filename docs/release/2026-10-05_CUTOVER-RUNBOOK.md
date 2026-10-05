# tq dogfood cutover runbook

The production dogfood (this machine's `tq serve` + `tq agent-pool`
systemd services, journal at `/mnt/pool/services/tq/tq.db`) runs on the
nix-built flake binary. This runbook covers the platform cutover
(ADR-0019: legacy pre-flip DB → go-cqrs-lite engine tables, in place)
and its rollback. Executed 2026-10-05 19:41 (service restart onto the
0.3.1 binary); recorded here so the next cutover is a checklist, not an
archaeology dig.

## Cutover checklist

1. **Green tree first**: `./scripts/release.sh vX.Y.Z` passes (it runs
   ci-local + release-gates; the tag wave must exist — internal modules
   require real tags).
2. **Build + deploy**: `nix build` produces the flake-version binary;
   the NixOS module (`deploy/nixos/tq-agent-pool.nix`) + home-manager
   restart the services onto it.
3. **First open converges the DB in place** (facade `Open` auto-upgrade):
   snapshot `<db>.legacy-*.bak` → migrate → verify → on mismatch
   auto-restore the snapshot. Absent feature-era tables are tolerated
   (fresh DBs boot clean).
4. **Post-restart verification** (all read-only):
   - `tq stats` — task totals match the pre-restart baseline (2026-10-05:
     805 = 1 pending / 461 completed / 319 dead / 24 cancelled) and the
     per-project table renders from the projection (`--read-model`
     default ON reads `<db>.readmodel.db`).
   - `ls /mnt/pool/services/tq/*.bak` — exactly ONE snapshot per cutover
     (zero here = the DB had already converged in an earlier window).
   - `tq doctor` — journal head reachable, projection row counts sane.
   - Dashboard `http://127.0.0.1:8100` serves (serve's pump feeds SSE).
5. **Watch one pool tick** (`~/.local/state/tq/logs`): claims, verdicts,
   no SQLITE_BUSY storm.

## Rollback runbook

- **Refuse the auto-upgrade** (binary stays new, DB stays legacy):
  set `TQ_NO_AUTO_UPGRADE=1` in the service environment and restart —
  the facade refuses to converge and reports instead.
- **Restore the snapshot**: stop the services, copy
  `<db>.legacy-*.bak` over `<db>` (the snapshot is the byte-exact
  pre-flip DB), remove `<db>.readmodel.db*` (disposable projection),
  start the services on the PREVIOUS binary (the old release's nix
  store path or `scripts/build-tq.sh` output).
- **Manual replay** (legacy DB suspected half-migrated, no snapshot):
  `go run ./replay` in `internal/queue/sqlitev4` rebuilds the engine
  tables from the fact journal.
- **Projection-only problems** (dashboard wrong, queue fine): delete
  `<db>.readmodel.db*`; the next open replays the journal from zero and
  re-seals the durable cursor (an empty projection under a nonzero
  checkpoint self-heals by design).

## Verification record — 2026-10-05

- Services: `tq serve --addr 127.0.0.1:8100` + `tq agent-pool` on the
  0.3.1 nix binary since 19:41.
- `tq stats` (production binary, production DB): totals 805
  (1/461/319/24) — byte-parity with the 2026-10-05 adoption deep-dive
  measurement of the same journal.
- `*.legacy-*.bak` count: 0 (converged in an earlier window; no
  repeated re-converge attempts — the upgrade is idempotent).
- Projection files live and churning (readmodel.db 3.66 MB + WAL) —
  the pre-durable-cursor binary still full-replays on restart; the
  M03 `WithDurableCursor` (v0.3.1 source) ends that on the next
  service rebuild.
