# CI Topology One-Pager

Every job and step across `.github/workflows/ci.yml` and `fuzz.yml` — what it
gates, whether it loops the internal sub-modules, and why. Written so the next
agent does not re-derive it (TODO row 64, 08:47 report f20).

**Why the module loop exists at all:** the repo is multi-module (ADR-0011).
`go <cmd> ./...` from the root NEVER descends into nested modules, so any
gate run only at the root silently excludes every `internal/*` sub-module.
The disk-derived loop is the fix: `scripts/for-each-module.sh` (find over
`internal task journal queue executor worker` for go.mod dirs, sorted) is the
single source; a new module is gated with zero workflow edits. Every in-loop
run uses `GOWORK=off` (there is no go.work — replace-only by decision).
`cmd/tq` is its own replace-free module (ADR-0017) outside the loop's find
roots, gated separately via the devmod shim (`scripts/test-cmd-tq.sh`).

Workflow-level: both workflows pin `GOEXPERIMENT=jsonv2` workflow-wide
and pin setup-go (never `stable` — see AGENTS.md known issue; the pins sit
at `1.26.7` while the go.mod tree migrates to `go 1.27.1` — `GOTOOLCHAIN`
defaults resolve each module up via its own `toolchain` directive). ci.yml
carries a `concurrency: ci-${{ github.ref }}` group with `cancel-in-progress:`
(2026-09-16) so the daemon's burst pushes cancel superseded runs. Since
2026-09-19/20 every per-module loop feeds from a CAPTURED enumeration
(`mods="$(./scripts/for-each-module.sh)"`) — the enumerator hard-fails on
zero targets and pins five canary sub-modules, so a renamed module can no
longer silently narrow a loop.

## ci.yml

| Job                             | Hard/Advisory                                           | Step                                                                                                               | Loops modules?  | Why / notes                                                                                                                                 |
| ------------------------------- | ------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------ | --------------- | ------------------------------------------------------------------------------------------------------------------------------------------- |
| `test` (ubuntu)                 | HARD                                                    | Vet + Build (root `./...`)                                                                                         | no              | Root app layer only; sub-modules covered by the loop step below                                                                             |
|                                 | HARD                                                    | Windows cross-compile (root + loop + `test-cmd-tq.sh` CMD_TQ_OS=windows)                                           | **YES**         | Platform honesty: every module must cross-compile, not just the root                                                                        |
|                                 | HARD                                                    | Test (`-race -count=1`)                                                                                            | no              | Root; race coverage lives here (Linux, cgo not needed — pure Go)                                                                            |
|                                 | HARD                                                    | Module isolation gates (build+vet+test, GOWORK=off)                                                                | **YES**         | The core loop: without it sub-modules drop out of every gate silently                                                                       |
|                                 | HARD                                                    | `test-cmd-tq.sh` (devmod shim)                                                                                     | cmd/tq          | ADR-0017: cmd/tq is its own module, invisible to `./...` and to the find roots                                                              |
|                                 | HARD                                                    | `check-go-mods.sh`                                                                                                 | all go.mods     | Replaces, pins, toolchain alignment, `go mod verify` across every module                                                                    |
|                                 | HARD                                                    | `check-facade-parity.sh` (ADR-0016)                                                                                | facades         | go/parser walk: every internal export needs a facade alias                                                                                  |
|                                 | HARD                                                    | Gofmt                                                                                                              | whole tree      |                                                                                                                                             |
|                                 | advisory                                                | golangci-lint run (root + loop)                                                                                    | **YES**         | Advisory baseline; root `./...` alone misses sub-modules, so the loop mirrors ci-local.sh                                                   |
|                                 | advisory                                                | lint annotations (`scripts/lint-annotations.sh` — module-looping since 2026-09-15, `--new-from-rev`, changed lines only)                                 | **YES** (in-script) | Scoped so new findings fit GitHub's 10-annotation cap                                          |
|                                 | HARD                                                    | TODO_LIST harvest-parse guard                                                                                      | harvest pkg     | Machine-consumed TODO_LIST format                                                                                                           |
|                                 | HARD                                                    | `smoke/webui.sh`, `smoke/release-gates.sh`, `check-doc-refs.sh`, `check-ghost-archives.sh`, `check-features-ci.sh` | —               | Smokes + doc/artifact gates                                                                                                                 |
| `test-windows` (windows-latest) | HARD                                                    | Test (no -race: needs cgo+mingw; unix-tagged suites drop out)                                                      | no              | Root                                                                                                                                        |
|                                 | HARD                                                    | Module isolation gates (build+test)                                                                                | **YES**         | Same loop, Windows side; platform honesty for every module                                                                                  |
| `test-postgres`                 | HARD                                                    | Postgres conformance (`internal/queue/postgres`, TQ_TEST_POSTGRES service)                                         | postgres module | ADR-0007 conformance parity with SQLite; env-gated locally, CI runs it (the caller-owned-pool test is a red-master guard, not a local skip) |
| `nix`                           | HARD                                                    | `nix build`, `nix flake check`, `check-webui-css.sh` (byte-canonical stylesheet pin, added 2026-09-15)              | whole flake     | Hermetic reproducible build; NOT `--all-systems` (no aarch64/darwin runners); the css pin needs nix, so it lives in this job |
| `govulncheck`                   | HARD (since round-13 T7)                                | root + loop                                                                                                        | **YES**         | Needs network (vuln DB live), so it can never be hermetic/ci-local; loop because root misses sub-modules                                    |
| `gosec`                         | advisory (job-level continue-on-error, owner ruling O5) | root + loop with triage-encoded excludes                                                                           | **YES**         | Post-excludes = 0 findings; any finding is a NEW class needing triage                                                                       |
| `cqrs-lint`                     | advisory                                                | builds tool from go-cqrs-lite checkout, lints `internal/journal/cqrs`                                              | one pkg         | ADR-0014 seam lint; non-blocking pending clean soak + hermetic tool source                                                                  |

## fuzz.yml (nightly, 03:17 UTC + workflow_dispatch)

| Job    | Hard/Advisory                                               | Step                                                            | Loops modules? | Why / notes                                                                                                                                                                 |
| ------ | ----------------------------------------------------------- | --------------------------------------------------------------- | -------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `fuzz` | HARD (job itself; a crasher must be fixed, never committed) | `scripts/fuzz/nightly.sh`                                       | no             | Fixed committed targets (FuzzParseRepo, FuzzExtractResultPayload) — targets live in harvest + executor modules, invoked from root via devmod-free go test inside the script |
|        | —                                                           | Commit new seed corpora back to master (bot push → triggers CI) | —              | Corpus growth must not depend on session memory; pushed commits are validated by the CI `test` job                                                                          |

## Parity rule

`scripts/ci-local.sh` mirrors every HARD gate above (plus the nix build and
lint-baseline growth check) locally; the advisory steps (golangci-lint,
gosec, cqrs-lint) run as loops there too. When adding a CI step: add it to
ci-local.sh in the same change, and if it is a Go gate over the whole repo,
route it through `for-each-module.sh` — otherwise the sub-modules are
ungated by construction.
