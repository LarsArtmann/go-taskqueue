#!/usr/bin/env bash
# Dep-bump drift gate: vendor/ and per-module go.mod/go.sum must be in sync
# with the module graph. The 2026-10-07 03:12 dep bump (b886a677) landed
# without `go mod vendor` or the 15 tree-wide `go mod tidy` follow-ups,
# costing a multi-hour red window — this gate fails the NEXT bump at gate
# time instead of at the next consumer build.
#
# Method (content-based, mtime-free — git checkout rewrites mtimes so
# timestamps lie): regenerate (go mod vendor / go mod tidy) and require an
# empty `git diff`. An out-of-sync tree is BOTH detected and healed by the
# same run: the files on disk are correct afterwards and only need staging.
#
# Modes:
#   ./scripts/check-gomod-vendor-sync.sh            gate the real repo
#   VENDOR_SYNC_SELF_TEST=1 ./scripts/check-…       pin drift detection
#   VENDOR_SYNC_OFF=1                              skip (network-tolerant escape)
set -euo pipefail
cd "$(dirname "$0")/.."

# encoding/json/v2 + the 1.27.1 floor need the same exports ci-local.sh
# makes global (AGENTS.md: GOEXPERIMENT/GOTOOLCHAIN hazards) — standalone
# runs must not depend on the caller's env.
export GOEXPERIMENT=jsonv2
export GOTOOLCHAIN=auto

# Drift detector: scoped git status — catches modified tracked files AND
# untracked new files (a module whose go.sum is missing entirely is drift
# too; `git diff` alone misses untracked). By gate convention (edit →
# commit → battery) these paths are committed before gates run, so any
# non-empty scoped status is real drift.
detect_drift() {
	local label="$1"
	shift
	local out
	out="$(git status --porcelain -- "$@")"
	if [ -z "$out" ]; then
		echo "ok: $label in sync"
		return 0
	fi
	echo "FAIL: $label DRIFTED — regenerated files differ from the committed tree:" >&2
	printf '%s\n' "$out" | sed 's/^/  /' >&2
	echo "  The regeneration above already healed the tree: review + stage the diff." >&2
	echo "  (dep-bump class of 2026-10-07 03:12: bump without go mod vendor / tidy;" >&2
	echo "  if a concurrent agent landed a bump mid-gate, let it commit and re-run.)" >&2
	return 1
}

self_test() {
	local tmp rc
	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' RETURN
	git init -q "$tmp/repo"
	mkdir -p "$tmp/repo/sub" "$tmp/repo/vendor"
	printf 'module test\n\ngo 1.27\n' >"$tmp/repo/go.mod"
	printf '# vendor/modules.txt\n# test test\n' >"$tmp/repo/vendor/modules.txt"
	printf 'module sub\n\ngo 1.27\n' >"$tmp/repo/sub/go.mod"
	git -C "$tmp/repo" add -A
	# A committed HEAD makes "in sync" mean clean-vs-HEAD, exactly like the
	# real repo at gate time (edit → commit → battery convention).
	git -C "$tmp/repo" -c user.email=t@t -c user.name=t commit -qm init

	# Case 1: untouched tree → detect_drift must pass for both scopes.
	if ! (cd "$tmp/repo" && detect_drift "root vendor" go.mod go.sum vendor/); then
		echo "SELF-TEST FAIL: in-sync tree reported as drifted" >&2
		return 1
	fi
	if ! (cd "$tmp/repo" && detect_drift "sub tidy" sub/go.mod sub/go.sum); then
		echo "SELF-TEST FAIL: in-sync sub-module reported as drifted" >&2
		return 1
	fi

	# Case 2: bump go.mod without vendor/modules.txt (today's incident).
	printf 'module test\n\ngo 1.27\n\nrequire x/y v1.2.3\n' >"$tmp/repo/go.mod"
	rc=0
	(cd "$tmp/repo" && detect_drift "root vendor" go.mod go.sum vendor/) || rc=1
	if [ "$rc" -ne 1 ]; then
		echo "SELF-TEST FAIL: drifted go.mod NOT detected" >&2
		return 1
	fi

	# Case 3: sub-module go.sum drift without go.mod change.
	printf 'x/y v1.2.3 h1:zzz=\nx/y v1.2.3/go.mod h1:zzz=\n' >"$tmp/repo/sub/go.sum"
	rc=0
	(cd "$tmp/repo" && detect_drift "sub tidy" sub/go.mod sub/go.sum) || rc=1
	if [ "$rc" -ne 1 ]; then
		echo "SELF-TEST FAIL: drifted sub go.sum NOT detected" >&2
		return 1
	fi

	echo "self-test ok: sync/drift detection pinned (root + sub-module scopes)"
	return 0
}

if [ "${VENDOR_SYNC_SELF_TEST:-0}" = 1 ]; then
	self_test
	exit $?
fi

if [ "${VENDOR_SYNC_OFF:-0}" = 1 ]; then
	echo "SKIP: VENDOR_SYNC_OFF=1 (network-tolerant escape)"
	exit 0
fi

fail=0

# Root: vendor/ must match the module graph (root auto-uses vendor/ for
# builds — AGENTS.md known issue: stale vendor is invisible until a
# consumer build dies).
GOWORK=off go mod vendor
detect_drift "root vendor/ (go mod vendor)" go.mod go.sum vendor/ || fail=1

# Per-module: go.mod/go.sum must be tidy (b886a677 class: 15 files across
# modules needed tidy after the bump). cmd/tq sits outside
# for-each-module's set (ADR-0017) — include it explicitly.
mods="$(scripts/for-each-module.sh; echo cmd/tq)"
for m in $mods; do
	[ -f "$m/go.mod" ] || { echo "FAIL: $m/go.mod vanished mid-gate" >&2; fail=1; continue; }
	(cd "$m" && GOWORK=off go mod tidy)
	detect_drift "$m tidy" "$m/go.mod" "$m/go.sum" || fail=1
done

if [ "$fail" -eq 0 ]; then
	echo "vendor-sync summary: root vendor + all module tidies in sync"
fi
exit $fail
