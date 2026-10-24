#!/usr/bin/env bash
# Webui JS unit-harness gate: runs internal/webui/jstest/app_test.mjs (Node,
# zero external deps) over the app.js swap-guard + state re-apply contract
# documented in docs/webui-live-contract.md. Born from the 2026-09-14 UI
# overhaul's verification debt (the guard shipped unverified).
set -uo pipefail
cd "$(dirname "$0")/.." || exit 1
self_abs="$(cd "$(dirname "$0")" && pwd)/$(basename "$0")"

must_mention() {
	if grep -qF -- "$1" <<<"$2"; then
		echo "ok: $3"
	else
		echo "FAIL: $3 (output is missing '$1')"
		printf '%s\n' "$2"
		exit 1
	fi
}

# --self-test pins the end-of-run summary line (fleet convention: countable
# gate summaries are PINNED — 01-12 §e7/§g2) and runs the negative path via
# the TQ_WEBUIJS_FAIL hook the harness reads (one forced failing test).
self_test() {
	local out rc=0
	out="$(TQ_WEBUIJS_FAIL=1 "$self_abs" 2>&1)" || rc=$?
	if [ "$rc" -ne 1 ]; then
		echo "FAIL: negative battery (want rc=1, got rc=$rc)"
		printf '%s\n' "$out"
		exit 1
	fi
	must_mention 'webui-js summary: 12 tests ok, 1 failed' "$out" "summary pins the negative counters"
	must_mention 'intentional failure for gate self-test' "$out" "forced-failure branch reports by name"
	local pos_out pos_rc=0
	pos_out="$("$self_abs" 2>&1)" || pos_rc=$?
	must_mention 'webui-js summary: 13 tests ok, 0 failed' "$pos_out" "positive run pins the full counter"
	if [ "$pos_rc" -ne 0 ]; then
		echo "FAIL: positive run exited $pos_rc"
		printf '%s\n' "$pos_out"
		exit 1
	fi
	echo "webui-js self-test ok (summary format pin + negative-path battery)"
}

if [ "${1:-}" = "--self-test" ]; then
	self_test
	exit 0
fi

command -v node >/dev/null 2>&1 || {
	echo "FAIL: node not found — the JS harness gate must not silently skip"
	exit 1
}

out="$(node internal/webui/jstest/app_test.mjs 2>&1)" || {
	echo "$out"
	exit 1
}
echo "$out"
