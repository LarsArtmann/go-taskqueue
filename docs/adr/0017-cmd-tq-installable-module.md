# ADR-0017: cmd/tq as a replace-free installable module

Date: 2026-09-14
Status: Accepted
Task: 000001a09ccb4fb72fc0fd35c832b31bfea9 (TODO_LIST "Restore
proxy-installability of the tq binary")

## Context

`go install github.com/larsartmann/go-taskqueue/cmd/tq@v0.3.0` fails. The
CLI package lived inside the ROOT module, whose `go.mod` carries relative
`replace` directives for local development (ADR-0011's replace-only
decision, no go.work). `go install pkg@version` builds the named module as
the main module and APPLIES its replaces; the relative targets (`./internal/task`)
resolve against the proxy-downloaded module directory, where no such
directories exist — the install dies. release.sh's clean-room step caught
this post-tag on v0.3.0.

## Decision

`cmd/tq` becomes its own Go module:
`module github.com/larsartmann/go-taskqueue/cmd/tq`.

- Its `go.mod` is REPLACE-FREE. Requires pin the root module AND the
  internal sub-modules it imports at real tagged versions (the same
  proxy-resolvable discipline `gate_gomod` already enforces for facades;
  the require-tag check was widened to cover the bare root-module path).
- In-repo builds (smokes, screenshots, e2e TestMain, CI gates) run the
  module through a GENERATED `dev.mod`/`dev.sum` pair
  (`scripts/lib/cmd-tq-devmod.sh`, consumed by `scripts/build-tq.sh` and
  `scripts/test-cmd-tq.sh`) that appends the repo's local replace set —
  root module plus every internal sub-module — and builds with
  `-modfile=dev.mod`. The derived files are never committed; the committed
  `go.mod` stays install-clean.
- `flake.nix` builds `modRoot = "cmd/tq"` with the same replace lines
  injected in `preBuild`, so the hermetic build tracks the local tree.
- `cmd/tq/vX.Y.Z` rides every release: `release.sh` derives sub-tags from
  disk, so adding the directory to the enumeration lists is sufficient.
  The version sweep must ALSO bump cmd/tq's root-module require.

## Consequences

- `go install …/cmd/tq@vX.Y.Z` resolves purely through the proxy again.
  This is only provable once a root tag containing the split exists
  (any root version ≤ v0.3.0 also ships a `cmd/tq` package, which makes
  the replace-free build ambiguous until the next tag lands on the
  proxy) — the next release's clean-room step is the proof.
- The CLI's local dependency truth is now lagged: between releases, cmd/tq
  builds against the TAGGED root/internal versions through the devmod
  shim's replaces... the shim replaces ALL of them with the local tree, so
  in-repo gates keep testing local code; only proxy resolution (install)
  sees tags.
- Converting the remaining root-module packages (budget, harvest, httpapi,
  webui, sweepers, …) into modules was rejected: the CLI must require them
  through the root module, whose replaces are ignored when consumed as a
  dependency — that is exactly the semantics consumers already rely on
  for facades.
- Gates updated: `check-go-mods.sh` and `release.sh` enumerate cmd/tq;
  ci-local + ci.yml gained a dedicated cmd/tq shim step (the generic
  per-module loops run plain builds that cannot resolve the module's
  bootstrap window); release-gates smoke gained positive/negative CLI
  fixtures.
- Bootstrap window (until the next release's tags are pushed): committed
  replace-free builds of cmd/tq fail with "ambiguous import" against old
  root tags. The shim gates stay green; the clean-room install proves the
  fix at the next release.
