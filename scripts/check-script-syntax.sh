#!/usr/bin/env bash
# Script syntax gate: bash -n + shellcheck (severity >= warning, zero-findings
# policy) over every tracked *.sh. The first shellcheck pass measured 21 real
# findings across 12 files (2026-09-15) — all fixed, so the gate is born HARD:
# any finding fails. Style/info severities are deliberately not gated.
#
# The shellcheck binary resolves from PATH first (preinstalled on GitHub
# ubuntu runners), else through `nix shell` (this host). If neither exists
# the gate FAILS — a syntax gate that silently skips would be a lying green
# banner.
set -uo pipefail
cd "$(dirname "$0")/.." || exit 1
# Recursive --self-test re-invokes THIS script; capture the absolute self
# path BEFORE any cd (the gosec self_abs pattern; 02-38 §b).
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
# gate summaries are PINNED, not advisory — 01-12 §e7/§g2) and runs a
# negative-path counter battery over the if/else branches via a /tmp
# fixture and the TQ_SYNTAX_FILES hook (files list override).
self_test() {
	# tmp is deliberately global: the EXIT trap reads it after self_test
	# returned, and set -u would kill the trap on a function-local name.
	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' EXIT
	printf '#!/bin/sh\necho hi\n' >"$tmp/good.sh"
	printf '#!/bin/sh\nif then\nfi\n' >"$tmp/badsyn.sh"
	printf '#!/bin/sh\necho $foo\n' >"$tmp/badlint.sh"
	local files out rc=0
	files="$(printf '%s\n' "$tmp/good.sh" "$tmp/badsyn.sh" "$tmp/badlint.sh")"
	out="$(TQ_SYNTAX_FILES="$files" "$self_abs" 2>&1)" || rc=$?
	if [ "$rc" -ne 1 ]; then
		echo "FAIL: negative battery (want rc=1, got rc=$rc)"
		printf '%s\n' "$out"
		exit 1
	fi
	must_mention 'syntax summary: 2 checks ok, 2 failed' "$out" "summary pins exact counters (bash -n: 2 ok 1 fail; shellcheck: 1 fail)"
	must_mention 'badsyn.sh' "$out" "bash -n failure branch reports the broken fixture"
	must_mention 'badlint.sh' "$out" "shellcheck findings branch reports the lint fixture"
	local pos_out pos_rc=0
	pos_out="$("$self_abs" 2>&1)" || pos_rc=$?
	if grep -Eq '^syntax summary: [0-9]+ checks ok, [0-9]+ failed$' <<<"$pos_out"; then
		echo "ok: real-run summary matches the pinned format (rc=$pos_rc)"
	else
		echo "FAIL: real-run summary missing or malformed"
		printf '%s\n' "$pos_out"
		exit 1
	fi
	echo "script-syntax self-test ok (summary format pin + negative counter battery)"
}

if [ "${1:-}" = "--self-test" ]; then
	self_test
	exit 0
fi

files=()
if [ -n "${TQ_SYNTAX_FILES:-}" ]; then
	while IFS= read -r f; do
		[ -n "$f" ] && files+=("$f")
	done <<<"$TQ_SYNTAX_FILES"
else
	while IFS= read -r f; do
		files+=("$f")
	done < <(git ls-files '*.sh')
fi

if [ "${#files[@]}" -eq 0 ]; then
	echo "no tracked shell scripts"
	exit 0
fi

fail=0
checks_ok=0
checks_failed=0
for f in "${files[@]}"; do
	if bash -n "$f"; then
		checks_ok=$((checks_ok + 1))
	else
		checks_failed=$((checks_failed + 1))
		fail=1
	fi
done
if [ "$checks_failed" = 0 ]; then
	echo "bash -n ok (${#files[@]} scripts)"
fi

if command -v shellcheck >/dev/null 2>&1; then
	sc=(shellcheck)
elif command -v nix >/dev/null 2>&1; then
	# Prefetch on a QUIET, SEPARATE path before the findings capture: a cold
	# nix fetch once raced its own output into the capture and produced a
	# phantom exit-1 red (01-46 report f15). By the time shellcheck runs, the
	# store path exists and nix is out of the picture entirely.
	echo "shellcheck via nix shell (prefetching, quiet on a cold cache)"
	prefetched="$(nix build nixpkgs#shellcheck --no-link --print-out-paths 2>/dev/null | { grep -m1 -- '-bin$' || true; })" || {
		echo "FAIL: nix prefetch of shellcheck failed (run 'nix build nixpkgs#shellcheck' for stderr)" >&2
		exit 1
	}
	[ -n "$prefetched" ] || {
		echo "FAIL: nix prefetch of shellcheck returned no bin output" >&2
		exit 1
	}
	sc=("$prefetched/bin/shellcheck")
else
	echo "FAIL: shellcheck unavailable (preinstalled on GitHub runners; locally: nix shell nixpkgs#shellcheck -c shellcheck)" >&2
	exit 1
fi

# stderr (nix fetch chatter on a cold cache) is kept OUT of the findings
# capture: only shellcheck's own stdout may set findings (a first-run
# "copying path … from cache.nixos.org" line once failed the gate as a
# phantom finding).
sc_err="$(mktemp)"
trap 'rm -f "$sc_err"' EXIT
findings="$("${sc[@]}" -f gcc -S warning "${files[@]}" 2>"$sc_err")"
sc_rc=$?
if [ -s "$sc_err" ]; then
	echo "shellcheck stderr:" >&2
	cat "$sc_err" >&2
fi
if [ "$sc_rc" -ne 0 ] && [ -z "$findings" ]; then
	echo "FAIL: shellcheck exited $sc_rc without findings" >&2
	fail=1
	checks_failed=$((checks_failed + 1))
elif [ -n "$findings" ]; then
	echo "$findings"
	checks_failed=$((checks_failed + 1))
	fail=1
else
	echo "shellcheck ok (0 findings, severity >= warning)"
	checks_ok=$((checks_ok + 1))
fi

echo "syntax summary: $checks_ok checks ok, $checks_failed failed"
exit "$fail"
