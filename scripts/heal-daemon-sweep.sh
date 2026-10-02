#!/usr/bin/env bash
# heal-daemon-sweep.sh: scripted heal for footer-less daemon-swept commits.
#
# The auto-commit daemon sweeps files into footer-less `chore:` commits while
# an agent works (the "daemon race", hit 2026-09-20 09-52 §d, 2026-09-21
# 00-27 §d). Without a `Task-Queue-ID:` footer the queue cannot attribute the
# work. This tool bakes the 00-27 §d heal playbook into a runnable script:
# msg-filter the footer onto every footer-less commit in an UNPUSHED range,
# then run the mandatory verification (same subjects, same per-commit stats,
# exactly one well-formed trailing footer each, byte-equal tree, no tags
# affected).
#
# Safety rails (history policy: never rewrite pushed commits):
#   - refuses a dirty worktree;
#   - refuses an empty heal range;
#   - refuses if any range commit is reachable from a remote-tracking ref
#     other than the base itself;
#   - footer-carrying commits in the range (e.g. a real task commit on top
#     of sweeps) are preserved verbatim and still footer-verified;
#   - the backup ref refs/original/heal-daemon-sweep is dropped ONLY after
#     all five verifications pass; on any failure it is kept and the script
#     exits non-zero (recovery: read the printed fork records, or
#     `git reset --soft refs/original/heal-daemon-sweep`).
#   - prints old→new fork-record lines (sanctioned by check-dead-sha-refs)
#     for pasting into the window's status report.
#
# Usage: scripts/heal-daemon-sweep.sh [--from <ref>] <Task-Queue-ID>
#        scripts/heal-daemon-sweep.sh --self-test
#   --from <ref>  base of the heal range (default: origin/master, then
#                 @{upstream}, then the root commit). Everything in
#                 <ref>..HEAD is treated as unpushed-local and verified.
set -uo pipefail

BACKUP_REF="refs/original/heal-daemon-sweep"
tmp=""

die() {
	echo "heal-daemon-sweep: $*" >&2
	exit 1
}

usage() {
	echo "usage: $0 [--from <ref>] <Task-Queue-ID>" >&2
	echo "       $0 --self-test" >&2
	exit 2
}

need_git() {
	command -v git >/dev/null 2>&1 || die "git not found in PATH"
}

footer_well_formed() {
	# A message is healed iff its LAST non-empty line is exactly the footer.
	local msg=$1 id=$2
	local last
	last=$(printf '%s\n' "$msg" | sed '/^[[:space:]]*$/d' | tail -n 1)
	[ "$last" = "Task-Queue-ID: $id" ]
}

has_footer() {
	printf '%s\n' "$1" | grep -q '^Task-Queue-ID: '
}

# resolve_base: default base ref, first existing of origin/master, @{upstream},
# or the root commit.
resolve_base() {
	local requested=$1
	if [ "$requested" != "" ]; then
		git rev-parse --verify --quiet "$requested" >/dev/null && {
			echo "$requested"
			return
		}
		die "base ref '$requested' not found"
	fi
	if git rev-parse --verify --quiet origin/master >/dev/null 2>&1; then
		echo "origin/master"
		return
	fi
	if git rev-parse --verify --quiet '@{upstream}' >/dev/null 2>&1; then
		echo '@{upstream}'
		return
	fi
	git rev-list --max-parents=0 HEAD
}

# check_rails base: every pre-flight refusal. Dies with the reason on hit.
check_rails() {
	local base=$1
	[ -z "$(git status --porcelain)" ] || die "worktree is dirty; commit or stash first"
	[ -n "$(git rev-list "$base..HEAD")" ] || die "range $base..HEAD is empty; nothing to heal"

	local pushed=""
	while IFS= read -r r; do
		local hits
		hits=$(comm -12 <(git rev-list --sort=reverse "$r" | sort) <(git rev-list "$base..HEAD" | sort))
		pushed="$pushed$hits"
	done < <(git for-each-ref --format='%(refname)' 'refs/remotes/*')
	[ -z "$pushed" ] || die "refusing: pushed commits are inside the heal range (history policy)"

	# NOTE: footer-carrying commits inside the range are legitimate (a task
	# commit can sit on top of daemon sweeps) — run_filter preserves their
	# messages verbatim and verify_heal still demands a well-formed footer
	# on them, so they are checked, never rewritten, never refused.
}

# run_filter footer base: the actual msg-filter rewrite.
run_filter() {
	local footer=$1 base=$2
	export TQ_HEAL_FOOTER="$footer"
	FILTER_BRANCH_SQUELCH_WARNING=1 git filter-branch -f --msg-filter '
		msg=$(cat)
		case "$msg" in
			*"Task-Queue-ID: "*) printf "%s\n" "$msg" ;;
			*) printf "%s\n%s\n" "$msg" "$TQ_HEAL_FOOTER" ;;
		esac
	' -- "$base..HEAD" >/dev/null || return 1
	local bref
	bref=$(git for-each-ref --format='%(refname)' 'refs/original/refs/heads/*' | head -n 1)
	[ -n "$bref" ] || return 1
	git update-ref "$BACKUP_REF" "$(git rev-parse --verify "$bref")"
	git update-ref -d "$bref"
}

# verify_heal base id: the five playbook verifications. rc 0 iff all pass.
verify_heal() {
	local base=$1 id=$2
	local fail=0 subj_old subj_new stat_old stat_new c new msg

	# 1+2: same subjects, same per-commit change sets (oldest-first lockstep
	# is guaranteed by equal counts, which the rewrite preserves).
	subj_old=$(git log --reverse --format='%s' "$base..HEAD")
	subj_new=$(git log --reverse --format='%s' "$BACKUP_REF..HEAD")
	if [ "$subj_old" != "$subj_new" ]; then
		echo "FAIL: subjects changed across the heal" >&2
		fail=1
	fi
	stat_old=$(git rev-list --reverse "$base..HEAD" | while IFS= read -r c; do git diff-tree --no-commit-id --name-only -r "$c" | sort | md5sum; done)
	stat_new=$(git rev-list --reverse "$BACKUP_REF..HEAD" | while IFS= read -r c; do git diff-tree --no-commit-id --name-only -r "$c" | sort | md5sum; done)
	if [ "$stat_old" != "$stat_new" ]; then
		echo "FAIL: per-commit change sets changed across the heal" >&2
		fail=1
	fi

	# 3: exactly one well-formed trailing footer in every healed commit.
	while IFS= read -r new; do
		msg=$(git log -1 --format='%B' "$new")
		if ! footer_well_formed "$msg" "$id"; then
			echo "FAIL: commit $new lacks a well-formed trailing footer" >&2
			fail=1
		fi
		if [ "$(printf '%s\n' "$msg" | grep -c '^Task-Queue-ID: ')" != "1" ]; then
			echo "FAIL: commit $new carries more than one footer" >&2
			fail=1
		fi
	done < <(git rev-list --reverse "$BACKUP_REF..HEAD")

	# 4: byte-equal tree (the green battery carries over verbatim).
	if [ -n "$(git diff "$BACKUP_REF" HEAD)" ]; then
		echo "FAIL: tree differs from the pre-heal backup" >&2
		fail=1
	fi

	# 5: no tags reference any rewritten commit.
	while IFS= read -r new; do
		if [ -n "$(git tag --points-at "$new")" ] || [ -n "$(git tag --contains "$new")" ]; then
			echo "FAIL: tag(s) reference rewritten commit $new" >&2
			fail=1
		fi
	done < <(git rev-list "$BACKUP_REF..HEAD")

	return $fail
}

# print_fork_records base: old→new lines via positional map (equal counts and
# identical ordering make index alignment exact).
print_fork_records() {
	local base=$1
	local olds news
	olds=$(git rev-list --reverse "$base..HEAD")
	news=$(git rev-list --reverse "$BACKUP_REF..HEAD")
	while IFS= read -r old && IFS= read -r new <&3; do
		echo "FORK-RECORD: $old→$new"
	done <<<"$olds" 3<<<"$news"
}

main() {
	local base_arg="" footer="" base id
	while [ $# -gt 0 ]; do
		case $1 in
		--from)
			[ $# -ge 2 ] || usage
			base_arg=$2
			shift 2
			;;
		-*) usage ;;
		*)
			[ -z "$footer" ] || usage
			id=$1
			footer="Task-Queue-ID: $1"
			shift
			;;
		esac
	done
	[ -n "$footer" ] || usage
	need_git
	git rev-parse --is-inside-work-tree >/dev/null 2>&1 || die "not inside a git work tree"
	base=$(resolve_base "$base_arg")

	git update-ref -d "$BACKUP_REF" 2>/dev/null
	check_rails "$base"
	echo "healing $(git rev-list --count "$base..HEAD") commit(s) in $base..HEAD with: $footer"

	run_filter "$footer" "$base" || die "filter-branch failed (backup kept at $BACKUP_REF if it existed)"

	if verify_heal "$base" "$id"; then
		print_fork_records "$base"
		git update-ref -d "$BACKUP_REF"
		echo "HEAL OK — backup ref dropped"
		return 0
	fi
	die "verification FAILED — backup kept at $BACKUP_REF; recovery: git reset --soft $BACKUP_REF"
}

# --- self-test: fixture repo exercising the heal + every refusal rail ---
self_test() {
	local repo
	tmp=$(mktemp -d) || die "mktemp failed"
	trap 'rm -rf "$tmp"' EXIT
	local repo="$tmp/repo"
	git init -q "$repo"
	git -C "$repo" config user.email t@t
	git -C "$repo" config user.name t

	local ok=0 fail=0
	# expect_refusal <label> <rc>: a refusal rc must be non-zero.
	expect_refusal() {
		local label=$1 rc=$2
		if [ "$rc" = "0" ]; then
			fail=$((fail + 1))
			echo "SELF-TEST FAIL (expected refusal): $label" >&2
		else
			ok=$((ok + 1))
		fi
	}

	echo a >"$repo/a.txt"
	git -C "$repo" add a.txt
	git -C "$repo" commit -qm "base"
	git -C "$repo" update-ref refs/remotes/origin/master HEAD
	local old1 old2
	echo b >"$repo/b.txt"
	git -C "$repo" add b.txt
	git -C "$repo" commit -qm "chore: sweep one"
	old1=$(git -C "$repo" rev-parse HEAD)
	echo c >"$repo/c.txt"
	git -C "$repo" add c.txt
	git -C "$repo" commit -qm "chore: sweep two"
	old2=$(git -C "$repo" rev-parse HEAD)

	# Happy path: heal both sweeps.
	(cd "$repo" && "$0" --from origin/master deadbeef00000000000000000000000000000001) >/dev/null 2>"$tmp/err1" &&
		ok=$((ok + 1)) || {
		fail=$((fail + 1))
		echo "SELF-TEST FAIL: happy path exited non-zero" >&2
		cat "$tmp/err1" >&2
	}
	local new1 new2 m1 m2
	new1=$(git -C "$repo" rev-list --reverse origin/master..HEAD | sed -n 1p)
	new2=$(git -C "$repo" rev-list --reverse origin/master..HEAD | sed -n 2p)
	m1=$(git -C "$repo" log -1 --format='%B' "$new1")
	m2=$(git -C "$repo" log -1 --format='%B' "$new2")
	if footer_well_formed "$m1" "deadbeef00000000000000000000000000000001" &&
		footer_well_formed "$m2" "deadbeef00000000000000000000000000000001" &&
		[ "$m1" = "$(printf 'chore: sweep one\nTask-Queue-ID: deadbeef00000000000000000000000000000001')" ] &&
		[ -z "$(git -C "$repo" for-each-ref 'refs/original/*')" ]; then
		ok=$((ok + 1))
	else
		fail=$((fail + 1))
		echo "SELF-TEST FAIL: healed messages not as expected" >&2
	fi
	if [ "$new1" != "$old1" ] && [ "$new2" != "$old2" ]; then
		ok=$((ok + 1))
	else
		fail=$((fail + 1))
		echo "SELF-TEST FAIL: shas did not move" >&2
	fi

	# Rail: dirty worktree refusal.
	echo dirt >>"$repo/a.txt"
	(cd "$repo" && "$0" --from origin/master deadbeef00000000000000000000000000000002) >/dev/null 2>&1
	expect_refusal "dirty worktree" $?
	git -C "$repo" checkout -q -- a.txt

	# Footered commits in the range are preserved verbatim, not refused:
	# add one and re-heal; its message must survive byte-identical.
	local footered_before
	echo d >"$repo/d.txt"
	git -C "$repo" add d.txt
	git -C "$repo" commit -qm "work: real task"
	git -C "$repo" commit -q --amend -m "$(printf 'work: real task\n\nTask-Queue-ID: feedface00000000000000000000000000000009')"
	footered_before=$(git -C "$repo" log -1 --format='%B' HEAD)
	if (cd "$repo" && "$0" --from origin/master deadbeef00000000000000000000000000000003) >/dev/null 2>"$tmp/err3"; then
		ok=$((ok + 1))
	else
		fail=$((fail + 1))
		echo "SELF-TEST FAIL: re-heal of a fully-footered range should succeed" >&2
		cat "$tmp/err3" >&2
	fi
	local footered_after
	footered_after=$(git -C "$repo" log -1 --format='%B' HEAD)
	if [ "$footered_before" = "$footered_after" ]; then
		ok=$((ok + 1))
	else
		fail=$((fail + 1))
		echo "SELF-TEST FAIL: footered commit message was rewritten" >&2
	fi

	# Rail: pushed commit inside the heal range.
	git -C "$repo" update-ref refs/remotes/origin/master HEAD
	(cd "$repo" && "$0" --from refs/remotes/origin/master~1 deadbeef00000000000000000000000000000004) >/dev/null 2>&1
	expect_refusal "pushed commit in range" $?

	# Rail: empty heal range.
	(cd "$repo" && "$0" --from origin/master deadbeef00000000000000000000000000000005) >/dev/null 2>&1
	expect_refusal "empty range" $?

	if [ "$fail" = "0" ]; then
		echo "SELF-TEST OK ($ok checks)"
		return 0
	fi
	echo "SELF-TEST FAILED ($ok ok, $fail failed)" >&2
	return 1
}

if [ "${1:-}" = "--self-test" ]; then
	self_test
else
	main "$@"
fi
