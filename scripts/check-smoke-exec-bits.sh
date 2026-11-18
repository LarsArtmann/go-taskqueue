#!/usr/bin/env bash
# scripts/check-smoke-exec-bits.sh — every .sh under scripts/ must be
# executable. Born from the 2026-11-09 review finding that _lib.sh and
# session-start.sh landed 644 and only a paid review round-trip caught it
# (exec-bits rows); the one-time chmod sweep's state is pinned here so the
# class cannot quietly return via a new 644 script.
#
# --self-test pins the summary line + a 644-negative / 755-positive battery
# via the TQ_EXEC_BITS_SCOPE hook (fleet convention).
set -euo pipefail
cd "$(dirname "$0")/.."

self_abs="$(cd "$(dirname "$0")" && pwd)/$(basename "$0")"

if [ "${1:-}" = "--self-test" ]; then
	set -euo pipefail
	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' EXIT
	printf '#!/bin/sh\n' >"$tmp/good.sh"
	chmod 755 "$tmp/good.sh"
	printf '#!/bin/sh\n' >"$tmp/bad.sh"
	chmod 644 "$tmp/bad.sh"
	out="$(TQ_EXEC_BITS_SCOPE="$tmp/*.sh" "$self_abs" 2>&1)" && rc=0 || rc=$?
	[ "$rc" -eq 1 ] || {
		echo "exec-bits self-test FAIL: 644 battery rc=$rc (want 1): $out"
		exit 1
	}
	grep -qF "$tmp/bad.sh is not executable" <<<"$out" || {
		echo "exec-bits self-test FAIL: FAIL line does not name the 644 file: $out"
		exit 1
	}
	grep -qF "1 executable / 2 checked" <<<"$out" || {
		echo "exec-bits self-test FAIL: summary shape wrong: $out"
		exit 1
	}
	out="$(TQ_EXEC_BITS_SCOPE="$tmp/good.sh" "$self_abs" 2>&1)" && rc=0 || rc=$?
	[ "$rc" -eq 0 ] || {
		echo "exec-bits self-test FAIL: 755 battery rc=$rc (want 0): $out"
		exit 1
	}
	grep -qF "1 executable / 1 checked" <<<"$out" || {
		echo "exec-bits self-test FAIL: green summary wrong: $out"
		exit 1
	}
	echo "exec-bits self-test ok (summary pin + 644 negative + 755 positive)"
	exit 0
fi

scope="${TQ_EXEC_BITS_SCOPE:-}"
if [ -z "$scope" ]; then
	scope="$(find scripts -name '*.sh' -print | sort)"
fi

fails=0 total=0
# shellcheck disable=SC2086 # word-splitting IS the scope list (find lines / glob words)
for f in $scope; do
	total=$((total + 1))
	if [ ! -x "$f" ]; then
		echo "FAIL: $f is not executable (mode $(stat -c %a "$f")) — chmod +x it (2026-11-09 644 class)" >&2
		fails=$((fails + 1))
	fi
done
echo "exec-bits summary: $((total - fails)) executable / $total checked"
[ "$fails" -eq 0 ]
