#!/usr/bin/env bash
# Prompt-const fold gate: prompt-source edits are dedup-INVISIBLE to the
# footer channel — when a footer-less daemon `chore:` sweep folds a
# prompt-const file, no re-dispatch suppression can see the wording
# already landed (the 3810c13c fold pre-landed row 938's wording). The
# daemon bypasses every hook, so like check-daemon-sweep-docs.sh this is
# the detector half: point it at a range and it FAILS listing the
# footer-less daemon-swept commits that touched a prompt-const file,
# with the heal command.
#
# Usage: scripts/check-prompt-const-folds.sh [--since <ref>]
#        PROMPT_CONST_GATE_SELF_TEST=1 scripts/check-prompt-const-folds.sh
#   --since <ref>   base of the sweep scan (default origin/master)
#
# Exit: 0 clean (or no prompt-const fold found); 1 a fold was found.
set -uo pipefail
cd "$(dirname "$0")/.." || exit 1

# Prompt-source files (prompt template consts a daemon fold must never
# land unattributed). Keep in sync: each entry carries a `You are` /
# prompt-template const owned by its executor or harvest package.
is_prompt_const_file() {
	case "$1" in
	internal/harvest/harvest.go | \
		internal/executor/review.go | \
		internal/executor/status.go | \
		internal/executor/dlqfix.go | \
		internal/executor/prioritize.go) return 0 ;;
	*) return 1 ;;
	esac
}

# The daemon's subject template — mirrors the queue's folded-here regex
# (cmd/tq daemonCommitSubject; keep both in sync, row 243).
is_daemon_subject() {
	printf '%s' "$1" | grep -Eq '^chore: auto-commit [0-9]+ changed file\(s\) \(heuristic\)$'
}

has_footer() {
	printf '%s' "$1" | grep -q '^Task-Queue-ID: '
}

# scan <base>: prints the footer-less, daemon-swept commits that touched
# a prompt-const file (one sha per line). Empty = clean.
scan() {
	local base=$1 c subj msg hit
	while IFS= read -r c; do
		subj=$(git log -1 --format='%s' "$c")
		is_daemon_subject "$subj" || continue
		msg=$(git log -1 --format='%B' "$c")
		has_footer "$msg" && continue
		hit=""
		while IFS= read -r f; do
			if is_prompt_const_file "$f"; then
				hit=1
				break
			fi
		done < <(git diff-tree --no-commit-id --name-only -r "$c")
		[ -n "$hit" ] && printf '%s\n' "$c"
	done < <(git rev-list --reverse "$base..HEAD")
}

self_test() {
	local tmp base
	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' RETURN
	git init -q "$tmp/repo"
	git -C "$tmp/repo" -c user.email=t@t -c user.name=t commit -qm init --allow-empty
	base=$(git -C "$tmp/repo" rev-parse HEAD)

	# Case 1: daemon-shaped footer-less sweep touching a prompt-const
	# file → detected.
	mkdir -p "$tmp/repo/internal/executor"
	printf 'package executor\nconst p = `You are`\n' >"$tmp/repo/internal/executor/review.go"
	git -C "$tmp/repo" add internal/executor/review.go
	git -C "$tmp/repo" -c user.email=t@t -c user.name=t commit -qm 'chore: auto-commit 1 changed file(s) (heuristic)'
	if [ -z "$(cd "$tmp/repo" && scan "$base")" ]; then
		echo "SELF-TEST FAIL: prompt-const daemon fold not detected" >&2
		return 1
	fi

	# Case 2: same sweep shape but a NON-prompt-const .go file → clean.
	printf 'package executor\n' >"$tmp/repo/other.go"
	git -C "$tmp/repo" add other.go
	git -C "$tmp/repo" -c user.email=t@t -c user.name=t commit -qm 'chore: auto-commit 1 changed file(s) (heuristic)'
	if [ "$(cd "$tmp/repo" && scan "$base" | wc -l)" != 1 ]; then
		echo "SELF-TEST FAIL: non-prompt-const sweep wrongly detected" >&2
		return 1
	fi

	# Case 3: footer-carrying commit touching a prompt-const file → the
	# queue can attribute it; never detected.
	printf 'package executor\nconst q = `You are`\n' >>"$tmp/repo/internal/executor/review.go"
	git -C "$tmp/repo" add internal/executor/review.go
	git -C "$tmp/repo" -c user.email=t@t -c user.name=t commit -qm 'task work

Task-Queue-ID: 000001a0c698deadbeef000000000000'
	if [ "$(cd "$tmp/repo" && scan "$base" | wc -l)" != 1 ]; then
		echo "SELF-TEST FAIL: footer-carrying prompt edit wrongly detected" >&2
		return 1
	fi

	# Case 4: non-daemon subject touching a prompt-const file → not this
	# gate's class (manual commits are attributed by their author).
	printf 'package executor\nconst r = `You are`\n' >>"$tmp/repo/internal/executor/review.go"
	git -C "$tmp/repo" add internal/executor/review.go
	git -C "$tmp/repo" -c user.email=t@t -c user.name=t commit -qm 'docs: manual prompt edit'
	if [ "$(cd "$tmp/repo" && scan "$base" | wc -l)" != 1 ]; then
		echo "SELF-TEST FAIL: non-daemon commit wrongly detected" >&2
		return 1
	fi

	echo "self-test ok: fold selection (daemon shape, footer-less, prompt-const) pinned"
	return 0
}

if [ "${PROMPT_CONST_GATE_SELF_TEST:-0}" = 1 ]; then
	self_test
	exit $?
fi

since="origin/master"
if [ "${1:-}" = "--since" ] && [ $# -ge 2 ]; then
	since=$2
elif [ $# -ge 1 ]; then
	echo "usage: $0 [--since <ref>]" >&2
	exit 2
fi
git rev-parse --verify --quiet "$since" >/dev/null || {
	echo "FAIL: base ref '$since' not found" >&2
	exit 1
}

folds="$(scan "$since")"
if [ -z "$folds" ]; then
	echo "ok: no footer-less daemon sweeps touched prompt-const files since $since"
	exit 0
fi

echo "FAIL: footer-less daemon sweeps touched prompt-const files (prompt edits are dedup-invisible to the footer channel — no re-dispatch suppression can see the pre-landed wording):"
printf '%s\n' "$folds" | sed 's/^/  /'
echo "heal: scripts/heal-daemon-sweep.sh <Task-Queue-ID> (unpushed only; re-dispatch the wording if it already landed)"
exit 1
