#!/usr/bin/env bash
# go-floor gate (CV row "Add a go-floor gate to go-taskqueue, mirror of CV's
# scripts/go-floor-gate.sh" — INVERTED direction, the 2026-10-25
# deploy-blocker class): every tracked go.work floor must be >= the max
# tracked go.mod floor. The tracked cmd/tq/go.work once sat at 1.27.1 while
# module floors were 1.27.2+ — workspace commands silently resolved an OLD
# toolchain contract and masked the cmd/tq "undefined symbols" report all
# the way into a deploy blocker (go-taskqueue 70338631). A stale go.work
# directive is invisible to every other gate; this is the millisecond grep
# that catches it.
#
# Usage:
#   scripts/check-go-floor.sh             # check every tracked go.work
#   scripts/check-go-floor.sh --self-test # discrimination proof
#
# Exit 0 = every tracked go.work floor >= max module floor (and at least one
#         go.mod measured). Exit 1 = stale go.work (named). Exit 2 = no
#         tracked go.mod at all (the gate must MEASURE).
set -uo pipefail

floor_of() {
	# First `go` directive line of a go file, e.g. "go 1.27.3" -> "1.27.3".
	sed -n 's/^go \([0-9][0-9.]*\)$/\1/p' "$1" | head -n1
}

check_floors() {
	local base="$1"
	local rc=0 count=0 workfile workfloor maxfloor f floor low

	maxfloor=""
	while IFS= read -r f; do
		[ -n "$f" ] || continue
		floor="$(floor_of "$base/$f")"
		[ -n "$floor" ] || continue
		if [ -z "$maxfloor" ]; then
			maxfloor="$floor"
		else
			maxfloor="$(printf '%s\n%s\n' "$maxfloor" "$floor" | sort -V | tail -n1)"
		fi
	done < <(git -C "$base" ls-files 'go.mod' '*/go.mod')

	if [ -z "$maxfloor" ]; then
		echo "FAIL: no tracked go.mod with a go directive found under $base — the gate must MEASURE"
		return 2
	fi

	while IFS= read -r workfile; do
		[ -n "$workfile" ] || continue
		count=$((count + 1))
		workfloor="$(floor_of "$base/$workfile")"
		if [ -z "$workfloor" ]; then
			echo "FAIL: $workfile has NO parsable go directive"
			rc=1
			continue
		fi
		low="$(printf '%s\n%s\n' "$workfloor" "$maxfloor" | sort -V | head -n1)"
		if [ "$low" = "$workfloor" ] && [ "$workfloor" != "$maxfloor" ]; then
			echo "FAIL: $workfile floor $workfloor is BELOW the max module floor $maxfloor — the stale-go.work masking class (go-taskqueue 70338631); run 'go work edit -go=$maxfloor' in that module dir"
			rc=1
		fi
	done < <(git -C "$base" ls-files 'go.work' '*/go.work')

	if [ "$count" -eq 0 ]; then
		echo "go-floor: no tracked go.work (nothing to check; max module floor $maxfloor)"
		return 0
	fi

	if [ "$rc" -eq 0 ]; then
		echo "PASS: $count go.work floor(s) >= max module floor $maxfloor"
	fi
	return "$rc"
}

if [ "${1:-}" = "--self-test" ]; then
	# shellcheck source=/dev/null
	tmp="$(mktemp -d "${TMPDIR:-/tmp}/check-go-floor-selftest.XXXXXX")"
	trap 'rm -rf "${tmp:-}"' EXIT
	rc=0

	git -C "$tmp" init -q
	git -C "$tmp" config user.email t@t
	git -C "$tmp" config user.name t

	# Clean shape: go.work == module floor.
	printf 'go 1.27.3\n\nuse .\n' >"$tmp/go.work"
	printf 'module example.com/root\n\ngo 1.27.3\n' >"$tmp/go.mod"
	git -C "$tmp" add -f go.work go.mod
	if out="$(check_floors "$tmp" 2>&1)"; then
		[[ "$out" == *"1 go.work"* ]] || {
			echo "FAIL: self-test — clean fixture must measure 1 go.work: $out"
			rc=1
		}
	else
		echo "FAIL: self-test — clean fixture must pass: $out"
		rc=1
	fi

	# FAIL shape: stale go.work below the module floor (the incident).
	printf 'go 1.27.1\n\nuse .\n' >"$tmp/go.work"
	git -C "$tmp" add -f go.work
	if out="$(check_floors "$tmp" 2>&1)"; then
		echo "FAIL: self-test — the stale 1.27.1 go.work was NOT caught"
		rc=1
	elif [[ "$out" != *"BELOW the max module floor 1.27.3"* ]]; then
		echo "FAIL: self-test — stale-floor report not named: $out"
		rc=1
	fi

	# FAIL shape: unmeasured tree (no tracked go.mod).
	git -C "$tmp" rm -q --cached go.mod
	if check_floors "$tmp" >/dev/null 2>&1; then
		echo "FAIL: self-test — an unmeasured tree must fail the measure guard"
		rc=1
	fi

	if [ "$rc" -eq 0 ]; then
		echo "PASS: check-go-floor --self-test (stale go.work caught + clean passes + unmeasured tree fails)"
	fi
	exit "$rc"
fi

check_floors "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
