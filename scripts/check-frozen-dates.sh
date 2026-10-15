#!/usr/bin/env bash
# Frozen-date fixture gate (04-03 §f3/§g3 sweep; TODO_LIST 2026-09-17 harvest):
# smoke scripts and testdata fixtures must COMPUTE calendar dates at run time.
# A frozen literal rots silently — the ratelimit-e2e lesson: a frozen past
# reset date degraded the smoke to the 15-min fallback for ~4 days while it
# stayed green (docs/status/archived/2026-09-17_00-25 §45).
#
# Scope: scripts/smoke/*.sh + files under any */testdata/ tree, scanned for
# `YYYY-MM-DD` literals. Out of class:
#   - comment lines (leading # or //)
#   - fuzz corpus blobs (testdata/fuzz/) — parser input bytes, not fixtures
#   - run-time computations (`date +%…`, `date -d '+…'`)
# Ratified exceptions (verbatim historical captures whose past-date behavior
# is EXPLICITLY pinned by tests — allowlist by basename):
#   - ratelimit_incident_000001a08edf.txt
#     (executor; "reset already past" timezone-accident case in
#     ratelimit_test.go — a past reset there is a covered input, not rot)
#
# --self-test: hermetic scratch-tree probe (no repo mutation): a planted
# literal must fire, a comment-only file must pass, the allowlisted basename
# must pass.
set -euo pipefail
cd "$(dirname "$0")/.."

pattern='20[0-9]{2}-[0-9]{2}-[0-9]{2}'
allow_basename='ratelimit_incident_000001a08edf.txt'

scan_files() {
	local base="$1"
	[ -d "$base/scripts/smoke" ] &&
		find "$base/scripts/smoke" -type f -name '*.sh' || true
	find "$base" -type f -path '*/testdata/*' \
		-not -path '*/testdata/fuzz/*' \
		-not -path "$base/vendor/*" \
		-not -path "$base/.git/*" 2>/dev/null || true
}

scan() { # scan <base> — prints findings to stdout, returns 1 on any hit
	local base="$1" file hits=0
	while IFS= read -r file; do
		[ -n "$file" ] || continue
		case "$(basename "$file")" in "$allow_basename") continue ;; esac
		while IFS= read -r match; do
			[ -n "$match" ] || continue
			ln="${match%%:*}"
			# Skip comment lines (shell + Go): leading whitespace, then # or //.
			sed -n "${ln}p" "$file" | grep -qE '^[[:space:]]*(#|//)' && continue
			echo "frozen-dates: $file:$ln: $match"
			hits=$((hits + 1))
		done < <(grep -nE "$pattern" "$file" 2>/dev/null || true)
	done < <(scan_files "$base")
	[ "$hits" -eq 0 ]
}

self_test() {
	local t hits
	t="$(mktemp -d)"
	trap 'rm -rf "$t"' RETURN
	mkdir -p "$t/scripts/smoke" "$t/internal/executor/testdata"
	printf 'X="reset at 2030-01-02"\n' >"$t/scripts/smoke/dirty.sh"
	printf '# provenance comment cites 2030-01-02\n' >"$t/scripts/smoke/comment.sh"
	printf 'log reset at 2030-01-02\n' \
		>"$t/internal/executor/testdata/ratelimit_incident_000001a08edf.txt"
	if hits="$(scan "$t")"; then
		echo "self-test FAIL: planted literal not caught"
		exit 1
	fi
	printf '%s\n' "$hits" | grep -q 'scripts/smoke/dirty.sh' || {
		echo "self-test FAIL: hit points at the wrong file"
		printf '%s\n' "$hits"
		exit 1
	}
	printf '%s\n' "$hits" | grep -q 'comment.sh\|ratelimit_incident' && {
		echo "self-test FAIL: comment line or allowlisted file flagged"
		printf '%s\n' "$hits"
		exit 1
	}
	echo "frozen-dates self-test OK (literal caught, comments + allowlist clean)"
}

case "${1:-}" in
--self-test)
	self_test
	exit 0
	;;
esac

if hits="$(scan .)"; then
	echo "frozen-dates: clean — smoke + testdata compute dates at run time"
	exit 0
fi
echo
echo "FAIL: frozen calendar-date literal(s) — compute at run time (date +%Y-%m-%d…)."
echo "A frozen fixture rots silently (the ratelimit-e2e 4-day degradation)."
echo "Verbatim historical captures go in the allowlist at the top of this script,"
echo "with the test that pins their past-date behavior named beside the entry."
exit 1
