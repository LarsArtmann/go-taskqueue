#!/usr/bin/env bash
# Dep-bump drift gate: vendor/ and per-module go.mod/go.sum must be in sync
# with the module graph. The 2026-10-07 03:12 dep bump (b886a677) landed
# without `go mod vendor` or the 15 tree-wide `go mod tidy` follow-ups,
# costing a multi-hour red window — this gate fails the NEXT bump at gate
# time instead of at the next consumer build.
#
# Method (content-based, mtime-free — git checkout rewrites mtimes so
# timestamps lie): regenerate (go mod vendor / go mod tidy) and require
# zero change. Tracked files (go.mod/go.sum): empty scoped git status.
# vendor/ is GITIGNORED (.gitignore:64) — git status is structurally
# blind to it — so its anchor is a content hash of the tree compared
# across the regeneration boundary. An ABSENT tree (fresh CI checkout —
# vendor/ never lands in git) is not drift: nothing stale exists to
# detect, the regeneration IS the heal. An out-of-sync tree is BOTH
# detected and healed by the same run: the files on disk are correct
# afterwards and only need staging.
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

# Deterministic content hash over the vendor tree. vendor/ is gitignored,
# so git status can never report it (verified 2026-10-07: a mutated
# vendor/modules.txt yields an EMPTY scoped status) — the gate compares
# this hash across the regeneration boundary instead.
vendor_tree_hash() {
	if [ ! -d vendor ]; then
		printf 'absent\n'
		return 0
	fi
	find vendor -type f -print0 | sort -z | xargs -0 -r sha256sum | sha256sum | cut -d' ' -f1
}

self_test() {
	local tmp rc h1 h2 blind
	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' RETURN
	git init -q "$tmp/repo"
	mkdir -p "$tmp/repo/sub" "$tmp/repo/vendor"
	printf 'module test\n\ngo 1.27\n' >"$tmp/repo/go.mod"
	printf '# vendor/modules.txt\n# test test\n' >"$tmp/repo/vendor/modules.txt"
	printf 'module sub\n\ngo 1.27\n' >"$tmp/repo/sub/go.mod"
	# Mirror the real repo: vendor/ is gitignored — git never sees it.
	printf 'vendor/\n' >"$tmp/repo/.gitignore"
	git -C "$tmp/repo" add -A
	# A committed HEAD makes "in sync" mean clean-vs-HEAD, exactly like the
	# real repo at gate time (edit → commit → battery convention).
	git -C "$tmp/repo" -c user.email=t@t -c user.name=t commit -qm init

	# Case 1: untouched tree → tracked scopes pass; vendor hash captured.
	if ! (cd "$tmp/repo" && detect_drift "root go.mod/go.sum" go.mod go.sum); then
		echo "SELF-TEST FAIL: in-sync tree reported as drifted" >&2
		return 1
	fi
	h1="$(cd "$tmp/repo" && vendor_tree_hash)"
	if ! (cd "$tmp/repo" && detect_drift "sub tidy" sub/go.mod sub/go.sum); then
		echo "SELF-TEST FAIL: in-sync sub-module reported as drifted" >&2
		return 1
	fi

	# Case 2: bump go.mod without re-vendoring (today's incident).
	printf 'module test\n\ngo 1.27\n\nrequire x/y v1.2.3\n' >"$tmp/repo/go.mod"
	rc=0
	(cd "$tmp/repo" && detect_drift "root go.mod/go.sum" go.mod go.sum) || rc=1
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

	# Case 4: the blindness pin — a drifted gitignored vendor tree is
	# INVISIBLE to git status (empty scoped status) yet MOVES the content
	# hash; git-status anchoring would have waved it through.
	printf '# drifted\n' >>"$tmp/repo/vendor/modules.txt"
	blind="$(cd "$tmp/repo" && git status --porcelain -- vendor/)"
	if [ -n "$blind" ]; then
		echo "SELF-TEST FAIL: fixture no longer gitignores vendor/ (git status saw it)" >&2
		return 1
	fi
	h2="$(cd "$tmp/repo" && vendor_tree_hash)"
	if [ "$h1" = "$h2" ]; then
		echo "SELF-TEST FAIL: drifted vendor tree did not move the content hash" >&2
		return 1
	fi

	# Case 5: absent vendor tree at gate start (fresh CI checkout) must
	# PASS — the regeneration creates the tree, there is nothing stale to
	# detect (CI run 37593760430: before=absent failed every fresh run).
	rm -rf "$tmp/repo/vendor"
	printf 'module test\n\ngo 1.27\n' >"$tmp/repo/go.mod"
	case5="$(cd "$tmp/repo" && root_vendor_in_sync)" || {
		echo "SELF-TEST FAIL: absent vendor tree at start reported as drift" >&2
		return 1
	}
	case "$case5" in
	*"absent at start"*) ;;
	*)
		echo "SELF-TEST FAIL: absent-start run did not take the absent branch: $case5" >&2
		return 1
		;;
	esac
	# Case 6: the tree Case 5 generated must re-run in sync (idempotent
	# regeneration — before is now a real hash, stable across the boundary).
	case6="$(cd "$tmp/repo" && root_vendor_in_sync)" || {
		echo "SELF-TEST FAIL: freshly generated vendor tree reported as drift on re-run" >&2
		return 1
	}
	case "$case6" in
	*"in sync (content hash"*) ;;
	*)
		echo "SELF-TEST FAIL: re-run did not report the generated tree in sync: $case6" >&2
		return 1
		;;
	esac

	echo "self-test ok: sync/drift detection pinned (tracked scopes + ignored-vendor hash anchor + absent-start)"
	return 0
}

# Root: vendor/ must match the module graph (root auto-uses vendor/ for
# builds — AGENTS.md known issue: stale vendor is invisible until a
# consumer build dies). vendor/ is gitignored, so the anchor is the
# content hash across the regeneration, NOT git status. before=absent is
# a fresh checkout (CI) or a wiped tree: there is no stale tree to
# detect, the regeneration IS the heal, so it passes — only an existing
# tree that MOVES under regeneration is drift (CI 37593760430: the
# absent-start branch failed every fresh checkout).
root_vendor_in_sync() {
	local before after
	before="$(vendor_tree_hash)"
	GOWORK=off go mod vendor
	after="$(vendor_tree_hash)"
	if [ "$before" = "absent" ]; then
		echo "ok: root vendor/ absent at start — generated fresh, in sync (content hash $after)"
		return 0
	fi
	if [ "$before" != "$after" ]; then
		echo "FAIL: root vendor/ DRIFTED — on-disk tree differed from go mod vendor output:" >&2
		echo "  before=$before after=$after" >&2
		echo "  vendor/ is gitignored; git status cannot see this class, the content hash" >&2
		echo "  is the anchor. The regeneration above already healed the tree: re-run to" >&2
		echo "  confirm, then exercise a root build." >&2
		return 1
	fi
	echo "ok: root vendor/ in sync (content hash $after)"
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

root_vendor_in_sync || fail=1
detect_drift "root go.mod/go.sum (go mod vendor)" go.mod go.sum || fail=1

# Per-module: go.mod/go.sum must be tidy (b886a677 class: 15 files across
# modules needed tidy after the bump). cmd/tq sits outside
# for-each-module's set (ADR-0017) — include it explicitly.
mods="$(
	scripts/for-each-module.sh
	echo cmd/tq
)"
for m in $mods; do
	[ -f "$m/go.mod" ] || {
		echo "FAIL: $m/go.mod vanished mid-gate" >&2
		fail=1
		continue
	}
	(cd "$m" && GOWORK=off go mod tidy)
	detect_drift "$m tidy" "$m/go.mod" "$m/go.sum" || fail=1
done

if [ "$fail" -eq 0 ]; then
	echo "vendor-sync summary: root vendor + all module tidies in sync"
fi
exit $fail
