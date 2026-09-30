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

files=()
while IFS= read -r f; do
	files+=("$f")
done < <(git ls-files '*.sh')

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
	sc=(nix shell nixpkgs#shellcheck -c shellcheck)
	echo "shellcheck via nix shell"
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
