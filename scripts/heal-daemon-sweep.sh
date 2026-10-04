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

# check_rails base id: every pre-flight refusal. Dies with the reason on hit;
# echoes "noop" when every range commit already carries the target footer.
check_rails() {
	local base=$1 id=$2
	[ -z "$(git status --porcelain)" ] || die "worktree is dirty; commit or stash first"
	[ -n "$(git rev-list "$base..HEAD")" ] || die "range $base..HEAD is empty; nothing to heal"

	local pushed=""
	while IFS= read -r r; do
		local hits
		hits=$(comm -12 <(git rev-list "$r" | sort) <(git rev-list "$base..HEAD" | sort))
		pushed="$pushed$hits"
	done < <(git for-each-ref --format='%(refname)' 'refs/remotes/*')
	[ -z "$pushed" ] || die "refusing: pushed commits are inside the heal range (history policy)"

	# Tag rail: the pushed-commit rail does not catch tags — a tag on an
	# unpushed range commit would dangle off-branch after the rewrite. Refuse
	# before run_filter instead of relying on post-rewrite detection.
	local tagged=""
	while IFS= read -r t; do
		local hits
		hits=$(comm -12 <(git rev-list "$t" | sort) <(git rev-list "$base..HEAD" | sort))
		tagged="$tagged$hits"
	done < <(git tag --format='%(refname)')
	[ -z "$tagged" ] || die "refusing: tag(s) reference commits inside the heal range ($tagged); re-tag after healing"

	# NOTE: footer-carrying commits inside the range are legitimate (a task
	# commit can sit on top of daemon sweeps) — run_filter preserves their
	# messages verbatim and verify_heal demands the TARGET footer on them.
	# A commit footered with a DIFFERENT id is never rewritten (footer
	# rewrites are a manual, reviewed operation); a fully footered range is
	# a no-op.
	local c msg unfootered=0
	while IFS= read -r c; do
		msg=$(git log -1 --format='%B' "$c")
		if has_footer "$msg"; then
			footer_well_formed "$msg" "$id" && continue
			die "refusing: commit $c carries a different Task-Queue-ID footer"
		fi
		unfootered=$((unfootered + 1))
	done < <(git rev-list "$base..HEAD")
	[ "$unfootered" -gt 0 ] || echo "noop"
}

# run_filter footer base: the actual msg-filter rewrite. On failure the
# filter's stderr is printed (a bare rc=1 hid the cause once — 08-06 report
# §e2 polish row).
run_filter() {
	local footer=$1 base=$2
	local err
	err=$(FILTER_BRANCH_SQUELCH_WARNING=1 TQ_HEAL_FOOTER="$footer" git filter-branch -f --msg-filter '
		git interpret-trailers --if-exists doNothing --trailer "$TQ_HEAL_FOOTER"
	' -- "$base..HEAD" 2>&1 >/dev/null) || {
		echo "filter-branch failed:" >&2
		printf '%s\n' "$err" >&2
		return 1
	}
	# Self-test seam: filter-branch -f wipes unrelated refs/original backups
	# on modern git, so the stale-sibling hijack state the current-branch
	# resolution (92c9193c) guards against cannot be built from outside —
	# the self-test recreates it HERE, after the wipe, before resolution.
	if [ -n "${TQ_HEAL_TEST_STALE_BACKUP:-}" ]; then
		git update-ref "refs/original/refs/heads/$TQ_HEAL_TEST_STALE_BACKUP" "$(git rev-parse HEAD~1)"
	fi
	local bref
	bref=$(git symbolic-ref -q HEAD || return 1)
	bref="refs/original/$bref"
	git show-ref --verify --quiet "$bref" || return 1
	git update-ref "$BACKUP_REF" "$(git rev-parse --verify "$bref")"
	git update-ref -d "$bref"
}

# verify_heal base id subj_old stat_old: the five playbook verifications.
# subj_old/stat_old are snapshots taken BEFORE the rewrite (over base..HEAD);
# comparing against base..HEAD AFTER works because the base ref never moves
# and preserved commits keep their SHAs. rc 0 iff all checks pass.
verify_heal() {
	local base=$1 id=$2 subj_old=$3 stat_old=$4
	local fail=0 subj_new stat_new c new msg

	# 1+2: same subjects, same per-commit change sets (oldest-first lockstep
	# is guaranteed by equal counts, which the rewrite preserves).
	subj_new=$(git log --reverse --format='%s' "$base..HEAD")
	if [ "$subj_old" != "$subj_new" ]; then
		echo "FAIL: subjects changed across the heal" >&2
		fail=1
	fi
	stat_new=$(git rev-list --reverse "$base..HEAD" | while IFS= read -r c; do git diff-tree --no-commit-id --name-only -r "$c" | sort | md5sum; done)
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
	done < <(git rev-list --reverse "$base..HEAD")

	# 4: byte-equal tree (the green battery carries over verbatim).
	if [ -n "$(git diff "$BACKUP_REF" HEAD)" ]; then
		echo "FAIL: tree differs from the pre-heal backup" >&2
		fail=1
	fi

	# 5: no tags reference any rewritten commit — belt-and-suspenders behind
	# the pre-flight tag rail in check_rails (row 434: the comment names the
	# promise and the mechanism as one story): reaching here with a tag hit
	# means the tag appeared between the rail check and the rewrite.
	while IFS= read -r new; do
		if [ -n "$(git tag --points-at "$new")" ] || [ -n "$(git tag --contains "$new")" ]; then
			echo "FAIL: tag(s) reference rewritten commit $new" >&2
			fail=1
		fi
	done < <(git rev-list "$base..HEAD")

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
			case $1 in
			*[!0-9a-fA-F]* | "") die "invalid Task-Queue-ID '$1': must be non-empty hex" ;;
			esac
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
	local rail
	rail=$(check_rails "$base" "$id") || exit 1
	if [ "$rail" = "noop" ]; then
		echo "nothing to heal: every commit in $base..HEAD already carries the footer"
		return 0
	fi
	echo "healing $(git rev-list --count "$base..HEAD") commit(s) in $base..HEAD with: $footer"

	local subj_old stat_old
	subj_old=$(git log --reverse --format='%s' "$base..HEAD")
	stat_old=$(git rev-list --reverse "$base..HEAD" | while IFS= read -r c; do git diff-tree --no-commit-id --name-only -r "$c" | sort | md5sum; done)

	run_filter "$footer" "$base" || die "filter-branch failed (backup kept at $BACKUP_REF if it existed)"

	if verify_heal "$base" "$id" "$subj_old" "$stat_old"; then
		print_fork_records "$base"
		git update-ref -d "$BACKUP_REF"
		echo "HEAL OK — backup ref dropped"
		return 0
	fi
	die "verification FAILED — backup kept at $BACKUP_REF; recovery: git reset --soft $BACKUP_REF"
}

# --- self-test: fixture repo exercising the heal + every refusal rail ---
self_test() {
	local self=$0
	case $self in
	/*) ;;
	*) self="$PWD/${self#./}" ;;
	esac
	local repo
	tmp=$(mktemp -d) || die "mktemp failed"
	trap 'rm -rf "$tmp"' EXIT
	local repo="$tmp/repo"
	git init -q -b master "$repo"
	git -C "$repo" config user.email t@t
	git -C "$repo" config user.name t
	git -C "$repo" config tag.gpgSign false

	local ok=0 fail=0
	# expect_refusal <label> <rc> [want-substring] [errfile]: a refusal rc
	# must be non-zero; when want+errfile are given the captured stderr must
	# ALSO contain want, so a different refusal firing first cannot mask a
	# rotted rail (the dead --sort=reverse class, 2026-10-03 01-22 report
	# §d1). Cases left bare stay bare pending the row-452 owner test-norm
	# call (retroactive tightening is BLOCKED there).
	expect_refusal() {
		local label=$1 rc=$2 want=${3:-} errfile=${4:-}
		if [ "$rc" = "0" ]; then
			fail=$((fail + 1))
			echo "SELF-TEST FAIL (expected refusal): $label" >&2
			return
		fi
		if [ -n "$want" ] && ! grep -qF -- "$want" "$errfile"; then
			fail=$((fail + 1))
			echo "SELF-TEST FAIL ($label): wrong refusal reason — stderr missing: $want; got:" >&2
			cat "$errfile" >&2
			return
		fi
		ok=$((ok + 1))
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

	# Happy path: heal both sweeps. "$self" (NOT "$0"): the fixture subshell
	# cds away, so a relative $0 resolves into the temp repo and dies with
	# "No such file or directory" — the 2026-10-04 00-52 red (row 456).
	(cd "$repo" && "$self" --from origin/master deadbeef00000000000000000000000000000001) >/dev/null 2>"$tmp/err1" &&
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
		[ "$m1" = "$(printf 'chore: sweep one\n\nTask-Queue-ID: deadbeef00000000000000000000000000000001')" ] &&
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
	(cd "$repo" && "$self" --from origin/master deadbeef00000000000000000000000000000002) >/dev/null 2>&1
	expect_refusal "dirty worktree" $?
	git -C "$repo" checkout -q -- a.txt

	# A fully-footered range is a no-op success; a different-id footer is a
	# refusal; a mixed range heals only the sweeps and preserves the task
	# commit's message byte-identical.
	echo d >"$repo/d.txt"
	git -C "$repo" add d.txt
	git -C "$repo" commit -qm "work: real task"
	git -C "$repo" commit -q --amend -m "$(printf 'work: real task\n\nTask-Queue-ID: deadbeef00000000000000000000000000000001')"
	(cd "$repo" && "$self" --from origin/master deadbeef00000000000000000000000000000001) >/dev/null 2>&1
	local noop_rc=$?
	if [ "$noop_rc" = "0" ]; then
		ok=$((ok + 1))
	else
		fail=$((fail + 1))
		echo "SELF-TEST FAIL: fully-footered range should no-op successfully" >&2
	fi

	(cd "$repo" && "$self" --from origin/master deadbeef0000000000000000000000000000000f) >/dev/null 2>"$tmp/err3"
	expect_refusal "different-id footer" $? "carries a different Task-Queue-ID footer" "$tmp/err3"

	local mixed_rc
	(cd "$repo" && git reset -q --soft HEAD~1)
	(cd "$repo" && git commit -qm "work: real task")
	(cd "$repo" && "$self" --from origin/master deadbeef00000000000000000000000000000001) >/dev/null 2>"$tmp/err4" && mixed_rc=0 || mixed_rc=1
	if [ "$mixed_rc" = "0" ]; then
		ok=$((ok + 1))
	else
		fail=$((fail + 1))
		echo "SELF-TEST FAIL: mixed-range heal should succeed" >&2
		cat "$tmp/err4" >&2
	fi
	local footered_after
	footered_after=$(git -C "$repo" log -1 --format='%B' HEAD)
	if [ "$footered_after" = "$(printf 'work: real task\n\nTask-Queue-ID: deadbeef00000000000000000000000000000001')" ]; then
		ok=$((ok + 1))
	else
		fail=$((fail + 1))
		echo "SELF-TEST FAIL: task commit message not preserved in mixed heal" >&2
	fi

	# Rail: pushed commit inside the heal range.
	git -C "$repo" update-ref refs/remotes/origin/master HEAD
	(cd "$repo" && "$self" --from refs/remotes/origin/master~1 deadbeef00000000000000000000000000000004) >/dev/null 2>&1
	expect_refusal "pushed commit in range" $?

	# Rail: empty heal range.
	(cd "$repo" && "$self" --from origin/master deadbeef00000000000000000000000000000005) >/dev/null 2>&1
	expect_refusal "empty range" $?

	# Rail: tag on a commit inside the heal range (the pushed-commit rail
	# does not catch tags).
	echo e >"$repo/e.txt"
	git -C "$repo" add e.txt
	git -C "$repo" commit -qm "chore: sweep three"
	git -C "$repo" tag sweep-tag HEAD
	(cd "$repo" && "$self" --from origin/master deadbeef00000000000000000000000000000006) >/dev/null 2>"$tmp/err_tag"
	expect_refusal "tag in heal range" $? "tag(s) reference commits inside the heal range" "$tmp/err_tag"
	git -C "$repo" tag -d sweep-tag

	# A tag on the BASE commit is outside the heal range — heal succeeds.
	git -C "$repo" tag base-tag origin/master
	(cd "$repo" && "$self" --from origin/master deadbeef00000000000000000000000000000006) >/dev/null 2>"$tmp/err5"
	if [ "$?" = "0" ]; then
		ok=$((ok + 1))
	else
		fail=$((fail + 1))
		echo "SELF-TEST FAIL: tag on base should not block the heal" >&2
		cat "$tmp/err5" >&2
	fi
	git -C "$repo" tag -d base-tag

	# Rail: metacharacter + empty Task-Queue-ID — the hex validation
	# (3f9f497d) must refuse both with the same reason line.
	(cd "$repo" && "$self" --from origin/master 'dead;beef00000000000000000000000000') >/dev/null 2>"$tmp/err_meta"
	expect_refusal "metacharacter id" $? "must be non-empty hex" "$tmp/err_meta"
	(cd "$repo" && "$self" --from origin/master "") >/dev/null 2>"$tmp/err_empty_id"
	expect_refusal "empty id" $? "must be non-empty hex" "$tmp/err_empty_id"

	# Multi-branch backup-ref resolution (92c9193c): a stale
	# refs/original/refs/heads/<other> (sorting before the current branch's
	# backup, pointing at an older tree) must not hijack the resolution —
	# the TQ_HEAL_TEST_STALE_BACKUP seam recreates the sibling backup after
	# filter-branch's wipe, and the heal must still succeed.
	echo f >"$repo/f.txt"
	git -C "$repo" add f.txt
	git -C "$repo" commit -qm "chore: sweep four"
	(cd "$repo" && TQ_HEAL_TEST_STALE_BACKUP=aaa-stale "$self" --from origin/master deadbeef00000000000000000000000000000006) >/dev/null 2>"$tmp/err6"
	if [ "$?" = "0" ]; then
		ok=$((ok + 1))
	else
		fail=$((fail + 1))
		echo "SELF-TEST FAIL: multi-branch backup-ref resolution hijacked the heal" >&2
		cat "$tmp/err6" >&2
	fi

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
