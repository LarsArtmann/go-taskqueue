#!/usr/bin/env bash
# Disk-derived list of internal sub-module directories (dirs with a go.mod).
# Single source for the per-module CI gates (ci.yml, scripts/ci-local.sh):
# newly added modules are gated without editing the consumers.
set -euo pipefail
cd "$(dirname "$0")/.."
find internal task journal queue executor worker -name go.mod | sed 's|/go.mod$||' | sort
