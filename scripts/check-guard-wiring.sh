#!/usr/bin/env bash
# Orphaned-guard audit (TODO_LIST 2026-09-11 evening row; 15-39 report c7/f5, e1):
# every scripts/check-*.sh and scripts/smoke/*.sh must be referenced by
# ci-local.sh, ci.yml, or flake.nix. A guard nothing runs is the
# check-webui-css failure mode (unminified/hand-edited css shipped to
# master twice in 2026-09-11 behind an orphaned guard).
set -euo pipefail
cd "$(dirname "$0")/.."

refs=(scripts/ci-local.sh .github/workflows/ci.yml flake.nix)
rc=0
wired_ok=0
wired_failed=0
for f in scripts/check-*.sh scripts/smoke/*.sh; do
	wired=0
	for r in "${refs[@]}"; do
		if grep -qF "$f" "$r"; then
			wired=1
			break
		fi
	done
	if [ "$wired" = 0 ]; then
		echo "ORPHANED GUARD: $f is not referenced by ci-local.sh, ci.yml, or flake.nix — wire it or delete it"
		rc=1
		wired_failed=$((wired_failed + 1))
	else
		wired_ok=$((wired_ok + 1))
	fi
done

if [ "$rc" = 0 ]; then
	echo "guard wiring ok: every check-*/smoke script is referenced"
fi
echo "guard-wiring summary: $wired_ok scripts wired, $wired_failed orphaned"
exit "$rc"
