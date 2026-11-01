# vendorHash drift + drop-day watch (T22)

**Created 2026-11-01 03:30 CET · operational runbook for the two recurring
nix triage classes · owner of the ritual: `scripts/check-vendor-hash.sh`**

## vendorHash drift — detector + recipe

The one command (also wired into ci-local):

```sh
./scripts/check-vendor-hash.sh            # real: nix build .#checks.x86_64-linux.vendor-hash
./scripts/check-vendor-hash.sh --self-test  # countable-gate pin (drift branch + recipe + ok summary)
```

On drift it prints the exact copy recipe: the `got:` hash, the
`flake.nix:<line>` of the `vendorHash = "…"` pin to replace, and the
re-verify commands. On a non-hash nix failure it dumps the build tail
instead of pretending drift (the two cures differ).

## Provenance (row 504, closed operationally)

`vendorHash` is a function of the committed **go.mod graph ONLY** —
requires + replace pins across root and sub-modules. It has NO
relationship to the go toolchain tarball: `goTarballHash =
goToolchainHash` is single-sourced in flake.nix (~line 54) and feeds only
the toolchain build. Therefore drift triage is always:

1. What changed in any `go.mod`/`go.sum` (or the replace set)?
2. `./scripts/check-vendor-hash.sh` → copy `got:` into the pin.
3. `nix build` to confirm the full build (the FOD gate is fast, not
   complete).

The 2026-09-30 "provenance contradiction" was the worry that the
tarball-hash twin could silently drive vendorHash drift; it cannot — the
twin is one variable, used once, and the drift detector's recipe names
the exact pin to edit.

## Drop-day watch (formatterWithGo / goTarball blocks)

Trigger: **nixpkgs ships go ≥ the go.mod floor** (today 1.27.1; the
toolchain sweep of 2026-10-24 pinned 1.27.3 in lockstep — go.mod floors,
CI setup-go, flake tarball). Check cheaply:

```sh
nix-instantiate --eval -E '(import <nixpkgs> {}).go.version' 2>/dev/null || \
  nix eval nixpkgs#go.version
```

When the condition holds, ALL FOUR blocks drop TOGETHER (a partial drop
leaves the build riding two toolchain sources):

1. `goTarballVersion` / `goTarballHash` attrs (flake.nix).
2. The `goTarball*` builder block (fetchurl from go.dev).
3. `formatterWithGo` (the templ/goimports wrapper riding the tarball).
4. The drop-day comments pointing here (flake.nix + AGENTS.md).

After the drop: `nix flake check`, `./scripts/check-toolchain-pins.sh`,
`./scripts/check-vendor-hash.sh`, and a full `nix build` — the tarball
hash pairing (`goTarballHash`) dies with the block, so a leftover
reference fails evaluation loudly, which is the desired guard.

## Related rows

- TODO row 1389 (origin CI nix red, vendorHash mismatch): cure = the
  recipe above; verified GREEN at HEAD 2026-11-01 (no drift after the
  retryx + go-health graph changes landed with their pin refresh).
- TODO row 504 (provenance): CLOSED by this doc's provenance section.
