# RELEASE.md — cutting a go-taskqueue release

The release flow is a command, not a memory exercise: `scripts/release.sh`
owns the code, this document explains the flow and the rules behind it. The
single-module v0.1.0 procedure is preserved as
`docs/release/archived/2026-09-06_v0.1.0_CHECKLIST.md`; everything below is
the round-2 multi-module flow (ADR-0011 module split + ADR-0012 store
backend modules), proven live by the v0.2.0 release. The version surfaces
these gates keep in sync are inventoried in `VERSION-SURFACES.md`.

```bash
scripts/release.sh vX.Y.Z            # pre-tag gates only (safe default, read-only)
scripts/release.sh vX.Y.Z --tag      # gates, then cut the annotated tag(s)
scripts/release.sh vX.Y.Z --push     # tag + push + proxy verify + GitHub Release
```

`--push` performs owner-gated actions (pushing, publishing); everything
before it is read-only. Owner decisions baked into the script: no squash
before tag (history ships as-is), v0.x GitHub Releases are `--prerelease`,
tags are annotated and immutable.

## The two-phase --tag/--push split

Tagging and publishing are separate phases so a verified tree can sit tagged
but unpushed until the owner presses the button.

**Phase 1 — gates (no mode flag).** Runs, in order:

1. Preconditions: git repo present; target version sorts AFTER the last tag
   (a release must move forward — fixes ship as a NEW version, never a
   re-tag); tag must not already exist (exception: `--push` resumes when the
   tag already points at HEAD); working tree clean (the tag must point at
   the exact verified tree — commit, or let the auto-daemon commit, first).
2. CHANGELOG: a `## [vX.Y.Z] - YYYY-MM-DD` section must exist (cut
   `[Unreleased]` into it first); the section body is extracted to
   `/tmp/tq-release-notes.md` and reused as tag message and GitHub notes.
3. flake.nix version sync: the `version` attr AND the `-ldflags` line must
   carry the release version, else nix binaries report the wrong `tq
   version`.
4. go.mod hygiene: the sibling-replace allowlist (below).
5. Full CI gate: `scripts/ci-local.sh` (the CI replicant — this script and
   CI can never drift apart), then the web UI smoke against the nix-built
   binary.

**Phase 2 — `--tag`.** Cuts the annotated root tag `vX.Y.Z` on HEAD
immediately after the gates: the auto-commit daemon may commit at any
moment, and a tag on a later daemon commit is fine (bookkeeping only), but a
tag BEFORE the release commits land is the classic mistake. Verifies
`git tag --points-at HEAD` and the tagged tree's `go.mod` header.

**Phase 3 — `--push`.** Pushes `master`, the root tag, and every internal
sub-tag; waits on the module proxy (`go list -m -versions`, 5 attempts);
clean-room verifies the real consumer path (`go get` the module, `go mod
verify`, then `go install .../cmd/tq@vX.Y.Z` and run its `version` — go get
alone only resolves metadata and cannot catch a broken sub-module require);
creates the GitHub Release (`--prerelease`, notes from the CHANGELOG
section). Remaining manual step: confirm CI is green on `refs/tags/vX.Y.Z`.

## Tag ancestry (forked lineages)

Release tags are only meaningful if master can reach them. A lineage fork
(rebase/reword under the auto-commit daemon — the 2026-09-10 incident class)
leaves the old side's tags unreachable: `git describe` answers with fork
tags, and the next release's version-ordering gate compares against a tag
on the wrong side. Rules:

- `scripts/release.sh` gates read the highest `v*` tag GLOBALLY (ordering
  must move forward repo-wide, even across a fork) but cut the new tag on
  HEAD — so a release must never be cut while a higher tag is unreachable;
  heal the fork first (owner call: the forked commits are either cherry-pick
  replayed or the tag is accepted as orphaned history — tags are immutable).
- `tq doctor` carries a `tag-ancestry` check: WARNs when any `v*` tag is
  not an ancestor of HEAD, OK-skips outside a git repo (deployed
  environments). It is a warning, never a failure — healing is an owner
  decision.
- Fixture-proof note (round-11 T14): the sibling-replace allowlist and the
  require-tag gate hardcode the real module path, so fixture releases must
  mimic it (`github.com/larsartmann/go-taskqueue`). `gate_gomod` is
  parameterized by nothing else by design — the gate SHOULD refuse foreign
  module shapes.

## The --push failure path

`--push` is the only owner-gated phase and the only one that touches the
outside world; its steps and their failure modes:

1. `git push origin master` + root tag + sub-tags — a rejected push (remote
   moved) is a HARD stop: re-run the gates (the tree changed under you).
   Never `--force`.
2. Module proxy wait: `go list -m -versions`, 5 attempts x 30s. Timeout is
   NOT a failure of the release — verify
   `https://proxy.golang.org/<module>/@v/vX.Y.Z.info` manually; NEVER re-tag
   (the proxy caches forever; a re-tag poisons every future consumer).
3. Clean-room `go get` + `go install` of `cmd/tq@vX.Y.Z` — this is the proof
   the published tree is installable; a failure here means a sub-module
   require is broken and the release must be superseded by a NEW version.
4. GitHub Release (`--prerelease` for v0.x).
5. CI green on the tag (automated since round-11 T15): the script polls
   `gh run list --commit <tag-sha>` until the CI run completes (up to 30
   minutes). A red tag run does not un-publish (impossible — immutable) but
   MUST be triaged before the next release.

## Sub-tag cutting (internal/<mod>/vX.Y.Z)

Every internal sub-module ships with the release under a shared version:
after the root tag, the script derives the sub-tag list FROM DISK (`find
internal -name go.mod`) and cuts one annotated
`internal/<mod>/vX.Y.Z` tag per module — including modules nothing requires
yet (e.g. `internal/queue/postgres` before CLI store wiring), so they stay
proxy-resolvable. Sub-tag cutting is idempotent (existing tags are skipped),
and `--push` pushes every sub-tag it cut.

Consequence for development: when a sub-module changes semantically between
releases, bump BOTH its `require` line in the root `go.mod` AND cut the
matching `internal/<mod>/vX.Y.Z` subdirectory tag before the release tag is
cut. `scripts/check-go-mods.sh` enforces the pin shape (real tagged
versions, never `v0.0.0` — `go install` resolves them via the proxy).

## Sibling-replace allowlist (release gates)

The root `go.mod` uses relative `replace` directives for local dev only;
published consumers ignore them and resolve via the `require` versions,
which the sub-tags make real. Anything else in `replace` position is proxy
poison. The rules live in `scripts/lib/release-gates.sh` (`gate_gomod`),
sourced by both `scripts/release.sh` (real tree) and
`scripts/smoke/release-gates.sh` (fixtures), so the rules can never drift
from their tests. `gate_gomod` fails the release on:

- **Non-sibling replaces.** Allowlist shape:
  `github.com/larsartmann/go-taskqueue/internal/<mod>(/<nested>)? =>
  ./internal/<mod>(/<nested>)?` — one nesting level, added in round-2 when
  the store backends became nested modules (`internal/queue/sqlite`). The
  original single-segment pattern silently stopped matching nested paths:
  the gate false-positived the real replace while SKIPPING the require-tag
  check for the very module it failed to parse. Regex gates without
  fixtures rot — the smoke covers the real tree plus poison fixtures
  (absolute path, parent-relative path, untagged require, pseudo-version).
- **Pseudo-versions** (`00010101` in any require) — the signature of a
  replace-directive leak.
- **Requires without cut sub-tags.** Every
  `.../go-taskqueue/internal/...` require must reference a version whose
  `internal/<mod>/vX.Y.Z` tag already exists in git — checked BEFORE the
  release tag is cut, so `go install` of the published tree can never
  resolve nothing.
