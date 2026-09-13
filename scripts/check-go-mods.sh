#!/usr/bin/env bash
# Multi-module go.mod health gate: portable replaces, pinned internal
# requires, toolchain alignment across every module, and dependency
# verification. Called from scripts/ci-local.sh AND
# .github/workflows/ci.yml — keep both call sites in sync.
set -euo pipefail
cd "$(dirname "$0")/.."

mods="$(find internal task journal queue executor worker -name go.mod | sed 's|/go.mod$||' | sort)"
mapfile -t modfiles < <(find internal task journal queue executor worker -name go.mod | sort)
fail=0

bad="$(grep -hE '^replace ' "${modfiles[@]}" | grep -E '=> */' || true)"
if [ -n "$bad" ]; then
	echo "$bad"
	echo "FAIL: absolute replace paths are not portable"
	fail=1
fi

bad="$({ grep -hE '^[[:space:]]*github.com/larsartmann/go-taskqueue/internal/' "${modfiles[@]}" go.mod | sed 's|//.*||'; } | grep -vE ' v[0-9]+\.[0-9]+\.[0-9]+$' || true)"
if [ -n "$bad" ]; then
	echo "$bad"
	echo "FAIL: internal requires must be real tagged versions (vX.Y.Z) —"
	echo "go install of the published module resolves sub-modules through the"
	echo "proxy, where v0.0.0 never exists (ADR-0011; local replaces make the"
	echo "version cosmetic in-repo, which is why a wrong pin stays invisible)"
	fail=1
fi

want="$(awk '$1 == "go" { print $2; exit }' go.mod)"
for m in $mods; do
	got="$(awk '$1 == "go" { print $2; exit }' "$m/go.mod")"
	if [ "$got" != "$want" ]; then
		echo "FAIL: $m/go.mod declares go $got, root declares go $want — keep toolchains aligned"
		fail=1
	fi
done

for m in . $mods; do
	if ! (cd "$m" && GOWORK=off go mod verify >/dev/null); then
		echo "FAIL: go mod verify in $m"
		fail=1
	fi
done

exit $fail
