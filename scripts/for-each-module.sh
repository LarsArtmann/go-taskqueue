#!/usr/bin/env bash
# Disk-derived list of internal sub-module directories (dirs with a go.mod).
# Single source for the per-module CI gates (ci.yml, scripts/ci-local.sh):
# newly added modules are gated without editing the consumers.
#
# Zero-target guard (d035911 gosec 0-target window; 02-18 report §e6 family,
# one consumer layer up): an empty list silently narrows every per-module
# loop to zero iterations, and a `for m in $(...)` consumer ignores the
# enumerator's exit status entirely (word-position command substitution
# never triggers errexit), so a broken or empty enumeration must fail here
# instead of exiting 0 — consumers that capture the list under errexit
# inherit both guards for free.
set -euo pipefail
cd "$(dirname "$0")/.."
mods="$(find internal task journal queue executor worker -name go.mod | sed 's|/go.mod$||' | sort)"
if [ -z "$mods" ]; then
	echo "FAIL: module enumeration returned 0 module targets (silent skip: every per-module gate would narrow to zero iterations; check repo layout / CWD)" >&2
	exit 1
fi
printf '%s\n' "$mods"
