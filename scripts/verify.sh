#!/usr/bin/env bash
# Shared agent-verify gate wrapper (TODO row 132): the exact `.tq-verify`
# battery (build + vet + test -count=1 -race -timeout 180s + scoped gofmt)
# under the shared known-flaky-signature retry from
# scripts/lib/verify-retry.sh, so a load-flake the repo already cures no
# longer burns an attempt plus a re-dispatch cycle (02-11 report §d4).
# `.tq-verify` can point here once the owner rules the switch (02-11 report
# §g2; agents never edit `.tq-verify`) — the battery line below is
# byte-parity-pinned to the shipped `.tq-verify` by scripts/check-verify.sh,
# so flipping changes ONLY the retry semantics, never what the gate runs.
#
# Env exports match the sibling gates (root-gate.sh, ci-local.sh):
# GOEXPERIMENT is also inline in the battery line, GOTOOLCHAIN=auto is the
# agent-session lifeline (AGENTS.md GOEXPERIMENT/GOTOOLCHAIN hazard).
set -euo pipefail
cd "$(dirname "$0")/.."

export GOEXPERIMENT=jsonv2
export GOTOOLCHAIN=auto

. scripts/lib/verify-retry.sh

# ONE line, byte-identical to the `.tq-verify` gate line (parity pin).
verify_gate() { GOEXPERIMENT=jsonv2 go build ./... && GOEXPERIMENT=jsonv2 go vet ./... && GOEXPERIMENT=jsonv2 go test ./... -count=1 -race -timeout 180s && test -z "$(gofmt -l . | git check-ignore --stdin -v --non-matching | grep '^::')"; }

run_with_flake_retry "VERIFY" verify_gate
