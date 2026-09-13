#!/usr/bin/env bash
# Build the tq CLI from its own module (ADR-0017) through the generated
# dev.mod replace shim — see scripts/lib/cmd-tq-devmod.sh for why.
# Usage: scripts/build-tq.sh <output-path>
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
out="${1:?usage: build-tq.sh <output-path>}"
mkdir -p "$(dirname "$out")"
out="$(cd "$(dirname "$out")" && pwd)/$(basename "$out")"

source "$root/scripts/lib/cmd-tq-devmod.sh"
cmdtq_devmod
trap cmdtq_devmod_cleanup EXIT

cd "$CMD_TQ_DIR"
GOEXPERIMENT=jsonv2 GOWORK=off CGO_ENABLED=0 go build -modfile=dev.mod -o "$out" .
