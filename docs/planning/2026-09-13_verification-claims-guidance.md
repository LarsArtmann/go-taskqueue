# Verification-Claims Guidance: DONE Notes Must State Gate SCOPE

_Status: guidance (policy); created 2026-09-13 (task 000001a0987864f3)._

## The rule

Every DONE note, verify-then-close annotation, and close-out claim that asserts
"green"/"clean"/"passing" must name the **scope** the gate actually covered:
which modules, which environments, which commands.

A claim without a stated scope is a hypothesis, not a fact. This is already
the standing rule in `AGENTS.md` ("Claims carry citations"); this document
defines what a scope statement looks like and why it exists.

## Why: the cautionary tale (2026-09-10, archived report 04-09 §d2/§92)

A task's commit message said **"Local root + sqlite scans clean"** for the
newly-wired govulncheck job. Accurate — but scope-narrow. The first CI runner
run scanned ALL modules and found a real, reachable vulnerability
(GO-2026-5970, `golang.org/x/text@v0.29.0` infinite loop) in
`internal/queue/postgres` — a module the local spot-check never touched.
Scope-qualifying the claim would have made it a 30-second catch instead of a
CI surprise. Two harder failures (the 2026-09-11 env-only `.tq-verify` lies;
the 2026-09-12 `go`-directive downgrade that passed locally and failed the
CI `go.mod health` step) are the same class: **the gate that ran is not the
gate you think ran** unless you name it.

## What a scope statement must include

For each claim, state at least:

1. **Commands / gate run** — e.g. `go test ./... -race`, the per-module
   loop, `ci-local.sh`, `govulncheck ./...`. A gate name alone ("CI green")
   is insufficient.
2. **Module scope** — root only? all `internal/*` sub-modules? which ones
   were skipped and why (multi-module repo: `./...` never descends into
   nested modules — "all tests pass" over the root means exactly nothing
   for `internal/queue/postgres`).
3. **Environment** — local devShell vs. systemd unit env vs. runners.
   Known liars: the pool unit has no `GOEXPERIMENT=jsonv2`; `setup-go`
   version drift changed vet semantics; runner-only nix FOD mismatches
   reproduce nowhere locally. If you verified locally only, SAY SO — a
   runner verdict is a different fact.
4. **Evidence pointer** — the CI run ID, commit SHA, or command transcript
   the claim rests on.

## Good vs. bad

- BAD: "Tests pass." / "Scans clean." / "Verified."
- BAD: "Local scans clean." (scope omitted — reads as ALL modules)
- GOOD: "govulncheck clean at HEAD across root + all 7 sub-modules
  (v1.8.0), runner-verified in run 34661978016."
- GOOD: "Per-module gate loop green over all 7 sub-modules (local,
  GOEXPERIMENT=jsonv2 exported); runner state not yet observed."

## When scope is partial

Partial verification is fine — **stated** partial verification is fine.
An honest "root module only, sub-modules not re-run" lets the next agent
target the gap. An unqualified "done" actively hides it.

See also: `AGENTS.md` ("Claims carry citations", Known Issues on
GOEXPERIMENT/setup-go/vendorHash drift).
