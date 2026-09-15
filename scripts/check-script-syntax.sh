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
for f in "${files[@]}"; do
	if ! bash -n "$f"; then
		fail=1
	fi
done
if [ "$fail" = 0 ]; then
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

findings="$("${sc[@]}" -f gcc -S warning "${files[@]}" 2>&1)"
if [ -n "$findings" ]; then
	echo "$findings"
	fail=1
else
	echo "shellcheck ok (0 findings, severity >= warning)"
fi

exit "$fail"
