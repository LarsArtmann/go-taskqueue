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
#
# Canary members (parity with check-gosec.sh): the 0-target guard cannot
# see a MISSING member, so pin the five core sub-modules. A rename or
# removal of any of them must consciously update this list — every
# per-module CI consumer (ci-local.sh, ci.yml, check-gosec.sh) inherits
# the assertion from here.
set -euo pipefail
cd "$(dirname "$0")/.."
mods="$(find internal task journal queue executor worker -name go.mod | sed 's|/go.mod$||' | sort)"
if [ -z "$mods" ]; then
	echo "FAIL: module enumeration returned 0 module targets (silent skip: every per-module gate would narrow to zero iterations; check repo layout / CWD)" >&2
	exit 1
fi
canaries="internal/task internal/journal internal/queue internal/executor internal/worker"
missing=""
for canary in $canaries; do
	if ! grep -qxF -- "$canary" <<<"$mods"; then
		missing+=" $canary"
	fi
done
if [ -n "$missing" ]; then
	echo "FAIL: module enumeration is missing canary module(s):$missing — a short enumeration silently narrows every per-module CI loop to the survivors (renamed or removed sub-module? update the canary list in scripts/for-each-module.sh consciously)" >&2
	exit 1
fi
printf '%s\n' "$mods"
