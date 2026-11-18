#!/usr/bin/env bash
# verify-fork-record.sh <old> <new> — the mechanical receipt for fork records
# in the dead-SHA heal (TODO 24th-dispatch §f2/§e2): a `<sha>→<sha>` cite is
# only a HEAL if the new side is reachable and describes the same change as
# the dangling old side. This tool verifies one claimed pair:
#
#   rc=0  verified — printed reason names the strongest evidence:
#           patch-id   the two commits carry the identical patch
#                      (git patch-id --stable over git diff-tree -p; the
#                      rebase/cherry-pick class — same change, new parent)
#           tree       the two commits point at the identical tree
#                      (the amend/reword class — same snapshot, new sha)
#         Both checks require <new> reachable from HEAD first: a fork record
#         pointing at ANOTHER dangling sha heals nothing (the eb401d1a
#         audit class, 24th-dispatch §f1).
#   rc=1  refused — printed reason:
#           not-a-commit(<which>)   an arg does not resolve to a commit
#           old-reachable           old is a HEAD-ancestor: cite it directly,
#                                   no fork record needed (arrowing a healthy
#                                   cite would whitewash it)
#           new-unreachable         new is not a HEAD-ancestor
#           no-evidence             patch-ids differ AND trees differ
#
# Merge commits have no usable patch-id (git diff-tree -p prints nothing for
# them); they verify via tree identity only.
#
# --self-test builds a /tmp fixture repo and pins every branch: amend pair
# (tree), cherry-pick pair (patch-id, different tree), unrelated pair,
# not-a-commit arg, reachable-old refusal, dangling-new refusal.
set -uo pipefail

usage() {
	printf 'usage: %s <old-sha> <new-sha> | --self-test\n' "${0##*/}" >&2
	exit 2
}

# patch_id <sha>: stable patch-id of a non-merge commit, empty for merges.
patch_id() {
	git diff-tree -p "$1" 2>/dev/null | git patch-id --stable | awk '{print $1}'
}

# is_commit <sha>: rc=0 iff the arg resolves to a commit object.
is_commit() {
	[ "$(git cat-file -t "$1" 2>/dev/null)" = commit ]
}

# reachable_from_head <sha>: rc=0 iff the commit is an ancestor of HEAD
# (or HEAD itself) — the "still citable" test.
reachable_from_head() {
	git merge-base --is-ancestor "$1" HEAD 2>/dev/null
}

verify_pair() {
	local old=$1 new=$2
	is_commit "$old" || { printf 'not-a-commit(old)\n'; return 1; }
	is_commit "$new" || { printf 'not-a-commit(new)\n'; return 1; }
	if reachable_from_head "$old"; then
		printf 'old-reachable\n'
		return 1
	fi
	reachable_from_head "$new" || { printf 'new-unreachable\n'; return 1; }
	local po pn
	po=$(patch_id "$old")
	pn=$(patch_id "$new")
	if [ -n "$po" ] && [ "$po" = "$pn" ]; then
		printf 'patch-id\n'
		return 0
	fi
	if [ "$(git rev-parse "$old^{tree}")" = "$(git rev-parse "$new^{tree}")" ]; then
		printf 'tree\n'
		return 0
	fi
	printf 'no-evidence\n'
	return 1
}

self_test() {
	local tmp
	tmp=$(mktemp -d /tmp/verify-fork-record-selftest.XXXXXX)
	trap 'rm -rf "$tmp"' RETURN
	git -C "$tmp" init -q 2>/dev/null || git init -q "$tmp"
	git -C "$tmp" config user.email t@t && git -C "$tmp" config user.name t
	cd "$tmp" || return 1
	local ok=0 fail=0
	check() { # check <desc> <want-rc> <got-rc> <want-reason> <got-reason>
		if [ "$2" = "$3" ] && [ "$4" = "$5" ]; then
			ok=$((ok + 1))
		else
			fail=$((fail + 1))
			printf 'FAIL: %s (want rc=%s/%s got rc=%s/%s)\n' "$1" "$2" "$4" "$3" "$5"
		fi
	}
	# Fixture history: base, A (will be amended), B, C.
	echo base >"$tmp/f" && git -C "$tmp" add f && git -C "$tmp" commit -qm base
	echo a >"$tmp/a" && git -C "$tmp" add a && git -C "$tmp" commit -qm A
	local old_amend
	old_amend=$(git -C "$tmp" rev-parse HEAD)
	git -C "$tmp" commit -q --amend -m "A reworded"
	local new_amend
	new_amend=$(git -C "$tmp" rev-parse HEAD)
	echo b >"$tmp/b" && git -C "$tmp" add b && git -C "$tmp" commit -qm B
	# Cherry-pick class: same patch on a new parent (different tree).
	git -C "$tmp" checkout -q -b side HEAD~1
	echo c >"$tmp/c" && git -C "$tmp" add c && git -C "$tmp" commit -qm C
	local old_pick
	old_pick=$(git -C "$tmp" rev-parse HEAD)
	git -C "$tmp" checkout -q master 2>/dev/null || git -C "$tmp" checkout -q main
	git -C "$tmp" cherry-pick "$old_pick" >/dev/null
	local new_pick
	new_pick=$(git -C "$tmp" rev-parse HEAD)
	# Dangling new: amend twice, verify old→middle where middle dangles.
	echo d >"$tmp/d" && git -C "$tmp" add d && git -C "$tmp" commit -qm D
	local mid
	mid=$(git -C "$tmp" rev-parse HEAD)
	git -C "$tmp" commit -q --amend -m "D reworded"

	local rc reason
	reason=$(verify_pair "$old_amend" "$new_amend"); rc=$?
	check "amend pair verifies (patch survives reword)" 0 "$rc" patch-id "$reason"
	reason=$(verify_pair "$old_pick" "$new_pick"); rc=$?
	check "cherry-pick pair verifies via patch-id" 0 "$rc" patch-id "$reason"
	reason=$(verify_pair "$old_amend" "$new_pick"); rc=$?
	check "unrelated pair refused" 1 "$rc" no-evidence "$reason"
	reason=$(verify_pair "$(git -C "$tmp" rev-parse 'HEAD^{tree}')" "$new_amend"); rc=$?
	check "non-commit old refused" 1 "$rc" 'not-a-commit(old)' "$reason"
	reason=$(verify_pair "$new_amend" "$(git -C "$tmp" rev-parse HEAD)"); rc=$?
	check "reachable old refused" 1 "$rc" old-reachable "$reason"
	reason=$(verify_pair "$old_pick" "$mid"); rc=$?
	check "dangling new refused" 1 "$rc" new-unreachable "$reason"

	printf 'verify-fork-record self-test: %d ok, %d fail\n' "$ok" "$fail"
	[ "$fail" -eq 0 ]
}

case "${1:-}" in
--self-test)
	self_test
	exit $?
	;;
-h | --help)
	usage
	;;
esac

[ $# -eq 2 ] || usage
cd "$(git rev-parse --show-toplevel 2>/dev/null)" 2>/dev/null || {
	printf 'not a git repo\n' >&2
	exit 2
}
reason=$(verify_pair "$1" "$2")
rc=$?
printf '%s: %s %s — %s\n' "${0##*/}" "$1" "$2" "$reason"
exit $rc
