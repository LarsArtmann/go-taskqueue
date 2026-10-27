#!/usr/bin/env bash
# Tracked-but-ignored path guard (the fold-lock class, §g3): the daemon
# committed `tasks.db.readmodel.db.fold-lock` to master (934c55e5) and
# `git check-ignore` stayed SILENT on it — check-ignore is about would-be
# traversal, not about what is already tracked. `git ls-files -ci
# --exclude-standard` is the direct mechanical answer: every tracked path
# that matches an ignore rule. A tracked generated artifact can never be
# cleaned by the ignore rule and rots forever, so ANY hit fails the gate.
#
# Countable-gate convention: pinned summary + --self-test battery over a
# scratch repo (TQ_TRACKED_IGNORED_REPO overrides the scanned repo).
set -euo pipefail

self_abs="$(cd "$(dirname "$0")" && pwd)/$(basename "$0")"

repo="${TQ_TRACKED_IGNORED_REPO:-$(cd "$(dirname "$0")/.." && pwd)}"

if [ "${1:-}" = "--self-test" ]; then
	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' EXIT
	okc=0
	failc=0
	check() {
		local desc="$1" want_rc="$2" needle="${3:-}" rc out repo_dir="$4"
		rc=0
		out="$(TQ_TRACKED_IGNORED_REPO="$repo_dir" "$self_abs" 2>&1)" || rc=$?
		if [ "$rc" -ne "$want_rc" ]; then
			echo "FAIL: $desc (want rc=$want_rc, got rc=$rc)"
			printf '%s\n' "$out" | sed 's/^/    /'
			failc=$((failc + 1))
			return
		fi
		if [ -n "$needle" ] && ! grep -qF -- "$needle" <<<"$out"; then
			echo "FAIL: $desc (missing: $needle)"
			printf '%s\n' "$out" | sed 's/^/    /'
			failc=$((failc + 1))
			return
		fi
		echo "ok: $desc (rc=$rc)"
		okc=$((okc + 1))
	}

	git init -q "$tmp/clean"
	git -C "$tmp/clean" config user.email t@t
	git -C "$tmp/clean" config user.name t
	printf '*.sidecar\n' >"$tmp/clean/.gitignore"
	echo x >"$tmp/clean/keep.txt"
	git -C "$tmp/clean" add .gitignore keep.txt
	git -C "$tmp/clean" commit -qm base
	check "clean repo green" 0 "tracked-ignored summary: 0 tracked path(s) matching ignore rules" "$tmp/clean"

	echo y >"$tmp/clean/q.readmodel.db.fold-lock"
	git -C "$tmp/clean" add -f q.readmodel.db.fold-lock
	git -C "$tmp/clean" commit -qm "the daemon sweep class"
	printf '*.fold-lock\n' >>"$tmp/clean/.gitignore"
	git -C "$tmp/clean" add .gitignore
	git -C "$tmp/clean" commit -qm "ignore it after the fact"
	check "tracked ignored sidecar fails" 1 "TRACKED-IGNORED: q.readmodel.db.fold-lock" "$tmp/clean"

	echo "tracked-ignored self-test ok ($okc ok / $failc fail) — clean-repo green + committed-sidecar refusal"
	[ "$failc" -eq 0 ] || exit 1
	exit 0
fi

cd "$repo"
hits="$(git ls-files -ci --exclude-standard || true)"
count=0
if [ -n "$hits" ]; then
	while IFS= read -r p; do
		[ -n "$p" ] || continue
		echo "TRACKED-IGNORED: $p — untrack it (git rm --cached) or drop the ignore rule; a tracked generated artifact can never be cleaned by the ignore"
		count=$((count + 1))
	done <<<"$hits"
fi

echo "tracked-ignored summary: $count tracked path(s) matching ignore rules"
[ "$count" -eq 0 ]
