# Version-Surface Inventory

All the places a release version lives, and who must move when. The flake's
`checks.version-sync` covers only attr ↔ binary; everything else is verified
by `scripts/release.sh` gates or by hand (the flow itself is documented in
`RELEASE.md`). Status report 23:47 f27 asked for
this inventory (2026-09-10).

## The surfaces

| # | Surface            | Where                                                                                                 | Current value at v0.2.0    | Verified by                                                                                                                        |
| - | ------------------ | ----------------------------------------------------------------------------------------------------- | -------------------------- | ---------------------------------------------------------------------------------------------------------------------------------- |
| 1 | Flake version attr | `flake.nix` `go-standard.version`                                                                     | `"0.2.0"`                  | `nix flake check` `checks.version-sync` (attr ↔ binary)                                                                            |
| 2 | ldflags version    | `flake.nix` `buildFlagsArray` (`-X main.version=...`)                                                 | `0.2.0`                    | same check: binary `tq version` must equal attr                                                                                    |
| 3 | Root tag           | git `vX.Y.Z`, annotated, on HEAD                                                                      | `v0.2.0`                   | `scripts/release.sh` preconditions (forward-only, not pre-existing, clean tree, points at HEAD)                                    |
| 4 | Per-module tags    | every module dir (`internal/<mod>` + the seven ADR-0016 facades), `vX.Y.Z` derived from disk          | 8 sub-tags at v0.2.0 (facades join at the next release; 15 total then) | release gate `gate_gomod`: every internal `require` version must have its subdirectory tag BEFORE the root tag is cut              |
| 5 | Internal requires  | each module's `go.mod` requires sibling modules at real tagged versions + relative `replace`          | `v0.2.0`                   | `scripts/check-go-mods.sh` (real versions, never `v0.0.0`, no pseudo-version `00010101`, aligned `go` directives, `go mod verify`) |
| 6 | CHANGELOG          | `CHANGELOG.md` `[Unreleased]` → `## [vX.Y.Z]` section                                                 | append-only                | manual; `--tag` derives release notes from it (reused as tag message + GitHub notes)                                               |
| 7 | Toolchain          | `go` directive in root + every module `go.mod`                                                        | aligned across all modules | `check-go-mods.sh` (per-module `go` must match root)                                                                               |
| 8 | Facade module tags | `task/`, `journal/`, `queue/`, `queue/sqlite/`, `queue/postgres/`, `executor/`, `worker/` sub-tags   | none yet — first cut rides the next release (ADR-0016) | release.sh disk-derived enumeration (same gate_gomod pass; surface 4 generalizes: every module dir, internal or facade, gets its sub-tag) |

## Who must move when (bump order)

Cutting release `vX.Y.Z` — `scripts/release.sh vX.Y.Z --tag` automates 3–5;
1, 2, and 6 are manual prerequisites:

1. **CHANGELOG** (#6): fold `[Unreleased]` into a `## [vX.Y.Z]` section.
   The release notes/tag message come from this — do it first.
2. **flake.nix** (#1 + #2 together): bump `go-standard.version` AND the
   `-ldflags=-X main.version=` line to `X.Y.Z`. They must move in lockstep
   (the version-sync check fails otherwise) and BEFORE the nix build that
   the tag points at — `tq version` on the released binary must report the
   release. Note: `release.sh` does NOT bump these; it only gates.
3. **Root tag** (#3): `vX.Y.Z` on HEAD after 1–2 land. Forward-only.
4. **Per-module tags** (#4): the script derives `internal/<mod>/vX.Y.Z`
   from disk after the root tag. Idempotent (existing tags skipped);
   `--push` pushes them all. Order matters: sub-tags must exist before any
   consumer's `go install` of the root tag resolves internal requires
   through the proxy (#5), which is why `gate_gomod` runs pre-tag.
5. **Publish** (`--push`): master + root tag + all sub-tags, then proxy
   wait, `go mod verify`, and `go install .../cmd/tq@vX.Y.Z` smoke.

## What is NOT version-bearing

- `vendorHash` in `flake.nix`: changes with any go.mod/go.sum edit, carries
  no version semantics (see AGENTS.md vendorHash-drift note).
- `go-standard.version`'s `pname`: immutable identifier.
- Retired `tasks.db` and runtime data: never versioned.

## Coverage gaps (deliberate)

Nothing automated checks CHANGELOG ↔ tag ↔ flake attr agreement as a set;
only attr↔binary is gated. `release.sh`'s forward-only precondition plus
the proxy `go install` smoke catch the worst mismatches after the fact.
