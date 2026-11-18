#!/usr/bin/env bash
# Fixture-attribution v2 accuracy layer (TODO row: 2026-10-21 batch; 07-28 §e
# contract): `pins:` comments enforce LABELED INTENT only — a negative fixture
# that no-ops, or that drifts to a different failing branch, still trips the
# rc!=0 assertion and the harness false-greens. The accuracy fix is a branch
# CANARY TOKEN: the failing run must PRINT the token (impossible if the
# fixture no-ops) and the harness must grep for it BEFORE accepting rc!=0.
#
# Usage: source this file, then
#   expect_branch_fail <token> <label> <cmd...>
# Requires a numeric FAILS variable in the caller's scope; each failure
# echoes a `FAIL <label>` line and increments FAILS (countable summary,
# fleet convention 2026-10-24). Tokens are a distinctive substring of the
# branch's own FAIL line — one token per branch, never a shared summary word.
expect_branch_fail() {
	local token=$1 label=$2
	shift 2
	local out rc=0
	out=$("$@" 2>&1) || rc=$?
	if [ "$rc" -eq 0 ]; then
		echo "FAIL $label (gate accepted poisoned input)"
		FAILS=$((FAILS + 1))
		return 1
	fi
	if ! printf '%s\n' "$out" | grep -q -- "$token"; then
		echo "FAIL $label (rc!=0 but canary '$token' absent — wrong branch or fixture no-op)"
		printf '%s\n' "$out"
		FAILS=$((FAILS + 1))
		return 1
	fi
	echo "ok   $label"
	return 0
}

if [ "${1:-}" = "--self-test" ]; then
	FAILS=0
	expect_branch_fail 'CANARY-OK' 'self: canary present on rc!=0' sh -c 'echo branch fired CANARY-OK; exit 3'
	expect_branch_fail 'CANARY-OK' 'self: must reject rc=0' true
	expect_branch_fail 'CANARY-OK' 'self: must reject canary-absent rc!=0' sh -c 'echo wrong branch; exit 4'
	if [ "$FAILS" != 2 ]; then
		echo "FAIL: expect-branch self-test wanted exactly 2 failures, got $FAILS"
		exit 1
	fi
	echo "ok: expect-branch self-test 1/3 (canary present accepted, rc=0 + canary-absent negatives pinned)"
	exit 0
fi
