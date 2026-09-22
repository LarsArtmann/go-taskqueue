#!/usr/bin/env bash
# ADR-0017 gate for the tq CLI module: build + vet + test with GOWORK=off.
# The committed cmd/tq/go.mod is replace-free (proxy-installable), so this
# runs through the generated dev.mod replace shim — see
# scripts/lib/cmd-tq-devmod.sh. CMD_TQ_OS=windows does the cross-compile
# gate (build + vet only, matching the per-module windows loop).
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"

source "$root/scripts/lib/cmd-tq-devmod.sh"
cmdtq_devmod
trap cmdtq_devmod_cleanup EXIT

cd "$CMD_TQ_DIR"
export GOWORK=off GOEXPERIMENT=jsonv2 CGO_ENABLED=0 GOFLAGS=-modfile=dev.mod
if [ "${CMD_TQ_OS:-}" = "windows" ]; then
	export GOOS=windows
	go build ./...
	go vet ./...
	exit 0
fi
go build -o "$(mktemp -d)/tq" .
go vet ./...
# GOFLAGS carries -modfile for the gate itself; clear it for `go test` so the
# flag does not leak into subprocess probes (doctor's go-env check builds in
# a temp dir where dev.mod does not exist). Any script arguments are
# forwarded to the test line (targeted runs: -run/-count scoping without
# hand-rolling the devmod shim, 10-19 §d1); no arguments = the full suite.
GOFLAGS='' go test -modfile=dev.mod ./... -count=1 -timeout 120s "$@"
