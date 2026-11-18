#!/usr/bin/env bash
# scripts/lib/gate-fixture.sh — shared negative-fixture battery for check-*
# gate self-tests. Generalizes the ad-hoc mktemp + expect() prologue every
# gate hand-rolled (row 135; the 16-42 report §e5 class where a negative
# run died on a missing fixture file instead of exercising the pin).
#
# Usage inside a gate's --self-test branch:
#   . "$(dirname "$0")/lib/gate-fixture.sh"
#   tmp="$(gate_fixture_tree)"
#   trap 'rm -rf "$tmp"' EXIT                  # caller keeps its own trap
#   gate_fixture_write "$tmp" "doc.md" 'content'
#   gate_fixture_expect "derived budget trips" 1 "over by 2 B" \
#     TQ_...=v "$self_abs" "$tmp/doc.md"       # cmd runs under env(1)
#   gate_fixture_summary "agents-size self-test" "pins tagline"
#
# Counters GF_PASS/GF_FAIL are the lib's own; the expect command words run
# under env(1) so caller-side VAR=v prefixes keep working. The summary is
# the pinned countable line (fleet convention) and exits 1 on any failure.

GF_PASS=0
GF_FAIL=0

gate_fixture_tree() {
	mktemp -d
}

gate_fixture_write() { # dir relpath [content] — content via stdin when omitted
	local dir="$1" rel="$2"
	shift 2
	mkdir -p "$(dirname "$dir/$rel")"
	if [ "$#" -gt 0 ]; then
		printf '%s' "$1" >"$dir/$rel"
	else
		cat >"$dir/$rel"
	fi
}

gate_fixture_expect() { # desc want_rc [want_fragment] cmd...
	local desc="$1" want_rc="$2" want="$3"
	shift 3
	local out rc=0
	out="$(env "$@" 2>&1)" || rc=$?
	if [ "$rc" -ne "$want_rc" ]; then
		echo "SELF-TEST FAIL: $desc (rc=$rc, want $want_rc): $out" >&2
		GF_FAIL=$((GF_FAIL + 1))
		return
	fi
	if [ -n "$want" ] && ! grep -qF -- "$want" <<<"$out"; then
		echo "SELF-TEST FAIL: $desc (output missing '$want'): $out" >&2
		GF_FAIL=$((GF_FAIL + 1))
		return
	fi
	GF_PASS=$((GF_PASS + 1))
}

gate_fixture_summary() { # name tagline
	if [ "${GF_FAIL:-0}" -ne 0 ]; then
		echo "$1 FAILED ($GF_FAIL of $((GF_PASS + GF_FAIL)) checks)" >&2
		exit 1
	fi
	echo "$1 ok ($GF_PASS checks: $2)"
}

if [ "${1:-}" = "--self-test" ] && [ "${BASH_SOURCE[0]}" = "$0" ]; then
	set -euo pipefail
	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' EXIT
	meta=0

	gate_fixture_write "$tmp" "doc.md" 'line1
line2'
	[ "$(cat "$tmp/doc.md")" = 'line1
line2' ] || {
		echo "SELF-TEST FAIL: arg-content write round-trip" >&2
		meta=1
	}

	printf 'stdin-payload' | gate_fixture_write "$tmp" "nested/dir/f.txt"
	[ "$(cat "$tmp/nested/dir/f.txt")" = 'stdin-payload' ] || {
		echo "SELF-TEST FAIL: stdin write + nested mkdir" >&2
		meta=1
	}

	GF_PASS=0
	GF_FAIL=0
	gate_fixture_expect "pass pin" 0 "hello" sh -c 'echo hello'
	[ "$GF_PASS" -eq 1 ] && [ "$GF_FAIL" -eq 0 ] || {
		echo "SELF-TEST FAIL: expect pass path counters ($GF_PASS/$GF_FAIL)" >&2
		meta=1
	}

	gate_fixture_expect "wrong-rc pin" 1 "x" sh -c 'echo x; exit 3'
	[ "$GF_FAIL" -eq 1 ] || {
		echo "SELF-TEST FAIL: wrong rc not counted" >&2
		meta=1
	}

	gate_fixture_expect "fragment-miss pin" 0 "absent-token" sh -c 'echo hello'
	[ "$GF_FAIL" -eq 2 ] || {
		echo "SELF-TEST FAIL: missing fragment not counted" >&2
		meta=1
	}

	if (GF_FAIL=1 GF_PASS=1 gate_fixture_summary "meta" "t") >/dev/null 2>&1; then
		echo "SELF-TEST FAIL: summary exited 0 with failures" >&2
		meta=1
	fi

	GF_PASS=7
	GF_FAIL=0
	gate_fixture_summary "gate-fixture lib self-test" "write round-trips + expect pins + summary contract"
	[ "$meta" -eq 0 ] || exit 1
	exit 0
fi
