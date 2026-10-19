#!/usr/bin/env bash
# Regenerate or verify the branching-flow dupe baseline (.dupe-baseline.txt):
# one line per ACTIONABLE duplicate-type GroupKey. Groups the 2026-10-05
# type-dupe review accepted live in docs/dupe-type-accepted-ledger.md (the
# tool's own intentional/false_positive verdicts are not baselined — they are
# machine-classified noise). The gate distinguishes the accepted sea from
# GROWTH: a duplicate-type group nobody has adjudicated.
#
#   scripts/dupe-baseline.sh            regenerate (deliberate, after a fresh
#                                       review accepted new groups)
#   scripts/dupe-baseline.sh --check    gate: exit 1 on NEW actionable
#                                       GroupKeys; a group disappearing is
#                                       advisory-only (shrink is free)
#
# Requires the branching-flow CLI on PATH; the ci-local wiring skips the gate
# (with a note) where the tool is unavailable, mirroring the golangci-lint
# guard.
set -euo pipefail
export LC_ALL=C
cd "$(dirname "$0")/.."

mode=""
for arg in "$@"; do
	case "$arg" in
	--check) mode="--check" ;;
	*) echo "usage: $0 [--check]" >&2; exit 2 ;;
	esac
done

baseline=".dupe-baseline.txt"

current="$({ branching-flow dupe --format json . 2>/dev/null || true; } |
	jq -r '.[] | select(.Verdict == "actionable") | .GroupKey' |
	LC_ALL=C sort -u)"

if [[ -z "$mode" ]]; then
	printf '%s\n' "$current" >"$baseline"
	echo "dupe-baseline: wrote $(wc -l <"$baseline") actionable group(s) to $baseline"
	exit 0
fi

if [[ ! -f "$baseline" ]]; then
	echo "dupe-baseline: $baseline missing — run scripts/dupe-baseline.sh to seed it" >&2
	exit 1
fi

new="$(LC_ALL=C comm -13 "$baseline" <(printf '%s\n' "$current"))"
gone="$(LC_ALL=C comm -23 "$baseline" <(printf '%s\n' "$current"))"

if [[ -n "$gone" ]]; then
	echo "dupe-baseline: shrink (advisory only) — regen when convenient:" >&2
	printf '%s\n' "$gone" >&2
fi

if [[ -n "$new" ]]; then
	echo "dupe-baseline: NEW actionable duplicate-type group(s) (gate failure):" >&2
	printf '%s\n' "$new" >&2
	echo "review at source; accepted groups get a rationale row in" >&2
	echo "docs/dupe-type-accepted-ledger.md, then regen the baseline" >&2
	exit 1
fi

echo "dupe-baseline: ok ($(wc -l <"$baseline") accepted group(s), no growth)"
