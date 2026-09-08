# SUPERB PLAN ROUND8 — SystemNix/Evo-x2 Integration

**Date:** 2026-09-08 22:55 · **Goal:** go-taskqueue runs as a declarative NixOS
service on Evo-x2 (primary workstation, 128 GB RAM) via SystemNix, dogfooding
the agent-pool against CV + SystemNix + fleet repos.

**Method:** Pareto tiers (1% → 51%, 4% → 64%, 20% → 80%, rest → 100%), ALL
tasks sliced to ≤12 min, sorted by importance/impact/effort/customer-value.
Predecessor plans: ROUND4 (dogfood launch), ROUND6, ROUND7.

## Current state (verified 2026-09-08 22:50)

| Fact | Evidence |
| --- | --- |
| go-taskqueue flake exposes only `packages.default` (tq binary) — no `nixosModules` | `flake.nix` (go-standard module) |
| SystemNix integrates sibling Go flakes as inputs + `nixosModules.*` wired in host file | `systems/evo-x2.nix:74-75` (crush-daily, bank-sync) |
| SystemNix house modules add data placement, Caddy, sops, hardening | `modules/nixos/services/bank-sync.nix` |
| Upstream module template to copy | `~/projects/bank-sync/deploy/nixos/bank-sync.nix`, declared at bank-sync `flake.nix:1202` |
| pool.conf is flat `key=value`, precedence flag > env > file | `cmd/tq/poolconfig.go:21` |
| Unit drain invariants (SIGINT, 45 min stop, no shared deadline) | `cmd/tq/bootstrap.go:32-67`, AGENTS.md invariants |
| `tq serve`: read-only, non-loopback bind requires `--auth-token` | ADR-0003, AGENTS.md |
| SystemNix has ZERO tq references today; TODO_LIST clean of it | grep 2026-09-08 |
| go-taskqueue is ~30+ commits ahead of origin (push BLOCKED: owner) | TODO_LIST line 97 |
| Dogfood rails live in SystemNix repo already (`.crushrc`, `.tq-verify`, open TODO items) | AGENTS.md dogfooding section |
| Working tree churn from concurrent agents is expected | AGENTS.md Known Issues |

## ⛔ Decision gates (owner, not pool food)

1. **Origin push go/no-go** (TODO_LIST line 97): the `github:` input route
   needs master pushed. Interim: `git+file://` path input (storage-collector
   precedent) — flips to `github:` after push.
2. **Pool knobs**: `--agents` N, `--daily-budget`, `--max-per-tick`,
   `--review` / `--review-autofix`, `--status-every` N (line 53 also blocked
   on owner N), `--exclusive`, `--allow-dirty`.
3. **Input pinning**: floating `?ref=master` (sibling convention) vs pinned
   rev + re-verify-on-bump (hermes-agent comment precedent).

## Pareto tiers

- **1% (→51%)**: upstream `nixosModules.default` + SystemNix input + evo-x2
  wiring + house defaults + rebuild test + first live tick → the pool runs
  under systemd, declaratively.
- **4% (→64%)**: + declarative pool.conf surface, DB on mirrored pool,
  bootstrap seeding, knob calibration, runbook + rollback → survivable ops.
- **20% (→80%)**: + `tq serve` unit behind Caddy, btrbk snapshots, module eval
  check, bootstrap --install smoke, CI/docs → polish + regression safety.
- **Rest (→100%)**: watch-driven harvest (fleet TODO line 58),
  `--status-every` in module (blocked on N), optional VM test, pinning notes.

## Master task table (execution order)

| # | ID | Task | Repo | Tier | Min | Depends |
|---|----|------|------|------|-----|---------|
| 1 | A1 | Options block `services.tq-agent-pool` in new `deploy/nixos/tq-agent-pool.nix`: enable, package (`mkPackageOption`), user, dbPath, poolSettings (attrsOf str), serve sub-options, extraArgs | go-taskqueue | 1% | 12 | — |
| 2 | A2 | Render poolSettings → flat `key=value` pool.conf via `pkgs.writeText`; wire `--config` in ExecStart; document flag>env>file precedence | go-taskqueue | 1% | 12 | A1 |
| 3 | A3 | Unit drain invariants: `KillSignal=SIGINT`, `TimeoutStopSec=45min`, `KillMode=process`, `ProtectSystem=full`, `NoNewPrivileges`, `Restart=on-failure`, `RestartSec=30s`, `After=network-online.target` | go-taskqueue | 1% | 12 | A1 |
| 4 | A5 | Declare `nixosModules.default = import ./deploy/nixos/tq-agent-pool.nix` in flake.nix (bank-sync `flake.nix:1202` pattern) | go-taskqueue | 1% | 8 | A1-A3 |
| 5 | B0 | Decision gate: interim `git+file:///home/lars/projects/go-taskqueue` input vs wait for origin push (line 97); record choice in flake comment | SystemNix | 1% | 5 | owner |
| 6 | B1 | Add go-taskqueue input to SystemNix flake.nix (`?ref=master`, `nixpkgs.follows`; do NOT follow go-nix-helpers — bank-sync FOD-mismatch trap, flake.nix:276-280) | SystemNix | 1% | 5 | B0 |
| 7 | B3 | Wire `inputs.go-taskqueue.nixosModules.default` into `systems/evo-x2.nix` modules list | SystemNix | 1% | 3 | B1 |
| 8 | B5 | House module `modules/nixos/services/tq-agent-pool.nix`: `user = lars` (git/crush creds + ~/projects writes), db `/mnt/pool/services/tq/tq.db`, `RequiresMountsFor`, `ioTier`, `onFailure` | SystemNix | 1% | 12 | B3 |
| 9 | C4 | `nixos-rebuild test` on Evo-x2; assert units start: `systemctl status tq-agent-pool`, journal clean | SystemNix | 1% | 12 | B5 |
| 10 | C5 | First live tick: harvest enqueues → agent claims (real crush) → completion fact → `tq doctor` all-green; review/status sweepers at head | evo-x2 | 1% | 12 | C4 |
| 11 | C1 | Data dir: tmpfiles rule + pool-gated oneshot creating `/mnt/pool/services/tq` (owner lars, 0700) | SystemNix | 4% | 8 | B5 |
| 12 | C2 | Seed queue: `tq bootstrap CV,SystemNix --dry-run` first; real init with `TQ_DB=/mnt/pool/services/tq/tq.db`; decide fresh-DB vs import `./tasks.db` | evo-x2 | 4% | 12 | C1 |
| 13 | C3 | Calibrate knobs from decision gate #2 into house module defaults; every enqueue budget-capped | SystemNix | 4% | 10 | owner |
| 14 | B6 | Full poolSettings surface: repos (CV,SystemNix + fleet), interval, repo-interval, log-dir + `--log-dir-max-age`, discovery-addr seam | SystemNix | 4% | 12 | B5 |
| 15 | C7 | Runbook in SystemNix AGENTS.md: misbehavior → stop unit → `tq dlq` → rescue/cancel `--reason`; dedup escape hatch = edit item text | SystemNix | 4% | 10 | C5 |
| 16 | C8 | Rollback: documented disable + journal snapshot + fallback to `tq bootstrap --install` user-unit path | SystemNix | 4% | 5 | C5 |
| 17 | A4 | `tq serve` sub-module: second unit `tq-serve` (127.0.0.1 bind, `--auth-token` from file option, read-only ADR-0003 guardrail intact) | go-taskqueue | 20% | 12 | A3 |
| 18 | B7 | Caddy `protectedVHost` for the dashboard + `ports.tq` entry in house ports lib | SystemNix | 20% | 10 | A4 |
| 19 | C6 | Verify dashboard over Caddy: LAN URL, token auth, SSE live ticks (smoke parity with `scripts/smoke/webui.sh`) | evo-x2 | 20% | 8 | B7 |
| 20 | B8 | btrbk-pool snapshot stanza for `/mnt/pool/services/tq` (bank-sync/atticd placement precedent) | SystemNix | 20% | 8 | C1 |
| 21 | A6 | `checks.module-eval`: instantiate the module against nixpkgs eval so option typos fail `nix flake check` | go-taskqueue | 20% | 10 | A5 |
| 22 | A7 | Go smoke: `tq bootstrap --install` renders unit + pool.conf into fake $HOME, touches nothing live (TODO_LIST line 95) | go-taskqueue | 20% | 12 | A5 |
| 23 | D1 | `go test ./... -race` + `./scripts/ci-local.sh` green on go-taskqueue after A-tasks | go-taskqueue | 20% | 10 | A7 |
| 24 | D2 | `git add` new files (flakes see only tracked files!) → `nix build` + `nix flake check` both repos; vendor-hash drift guard | both | 20% | 10 | A5,B5 |
| 25 | D4 | Docs: FEATURES.md deployment row + CHANGELOG (go-taskqueue); SystemNix CHANGELOG + AGENTS.md service entry | both | 20% | 10 | C5 |
| 26 | A8 | Usage header in `deploy/nixos/tq-agent-pool.nix` (bank-sync header style: usage snippet + invariants) | go-taskqueue | 20% | 5 | A1 |
| 27 | B9 | sops decision note in house module: pool needs NO secrets (creds are user-level); serve token via env-file option, sops template optional | SystemNix | 20% | 5 | B5 |
| 28 | B4 | Pinning note: floating `?ref=master` chosen (sibling convention); re-verify-on-bump checklist comment (hermes-agent precedent) | SystemNix | 20% | 5 | B1 |
| 29 | E1a | Watch-driven harvest (TODO line 58) slice 1: SSE client for daemon `GET /v1/watch` + `--discovery-addr` gating | go-taskqueue | 100% | 12 | — |
| 30 | E1b | Watch-driven slice 2: per-repo debounce vs `--repo-interval`; interval tick stays fallback heartbeat (dead stream degrades, never stalls) | go-taskqueue | 100% | 12 | E1a |
| 31 | E1c | Watch-driven slice 3: httptest SSE + dead-stream + fallback tests | go-taskqueue | 100% | 12 | E1b |
| 32 | E2 | `--status-every` in module + deploy/systemd sample — BLOCKED: owner picks N (TODO line 53) | go-taskqueue | gated | 8 | owner |
| 33 | E3 | Optional SystemNix VM test `tests/test-tq.nix`: stub agent, `--once` pool, unit assertions (test-paperless.nix pattern) | SystemNix | 100% | 12 | B5 |
| 34 | E4 | Queue agent-safe slices in go-taskqueue TODO_LIST (module, install smoke, docs) citing this plan; SystemNix-side stays plan-only (needs rebuild privileges) | go-taskqueue | 20% | 5 | plan |

**Total: 34 tasks, ~5h50m focused effort, 10 tasks form the 51% core.**

## Execution graph

```
A1 → A2 → A3 → A5 → B0 → B1 → B3 → B5 → C4 → C5   (51% core: pool runs)
                  └→ A6, A7, A8
B5 → C1 → C2/C3/B6 → C7/C8                        (64%: survivable ops)
A3 → A4 → B7 → C6; C1 → B8; C5 → D1..D4           (80%: polish)
E1a → E1b → E1c; E2 (gated); E3                   (100%)
```

## Risks / gotchas (from AGENTS.md, verified)

- **Flakes see only git-tracked files** — `git add` new .nix files before any
  `nix build`/`nixos-rebuild` (task D2 ordering).
- **vendorHash drift** — go.mod/go.sum changes need the fakeHash dance;
  `checks.vendor-hash` fails fast.
- **GOEXPERIMENT=jsonv2** — already set in go-taskqueue flake; a SystemNix
  input must NOT override the build env.
- **Never reintroduce a shared drain deadline** — the module MUST keep
  SIGINT + 45 min stop window; task contexts outlive pool shutdown.
- **`tq serve` never gets write endpoints**; non-loopback default-deny.
- **Cancelled tasks hold their dedup key** — zombie cleanup needs item-text
  edits until `--prune-stale` (TODO line 76) ships.
- **Concurrent agents** — re-run `-race` before declaring success; expect
  working-tree churn (present at plan time: CHANGELOG/FEATURES/TODO_LIST).
- **templ LSP diagnostics are false positives** — trust build/vet/test.

## Post-integration

Flip `git+file://` → `github:` after the origin push lands; SystemNix repo
then mirrors the dogfood loop (status reports, harvest drift audits via
`tq audit`). Fleet items E1 and discovery-daemon wiring continue per
TODO_LIST "Fleet integration" section.
