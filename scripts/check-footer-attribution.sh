#!/usr/bin/env bash
# check-footer-attribution.sh: post-heal/rebuild attribution gate.
#
# After ANY heal (heal-daemon-sweep) or later history rebuild, the queue's
# attribution view must still agree with the actual span: every commit in
# <base>..HEAD carries the target `Task-Queue-ID:` footer MACHINE-visibly
# (trailer-parseable), with the same subjects and per-commit change sets.
# A rebuild that lands after a heal can change SHAs or move files between
# commits; only this footer-vs-span cross-check catches the drift —
# verify_heal only pins a single heal, not later rebuilds.
#
# Usage: scripts/check-footer-attribution.sh [--from <ref>] <Task-Queue-ID>
#        scripts/check-footer-attribution.sh --self-test
#
# Exit 0 = attribution intact; non-zero = drift (with per-commit FAIL lines).
set -uo pipefail

die() {
	echo "check-footer-attribution: $*" >&2
	exit 1
}

# top-level so subshells inheriting the EXIT trap can see it
tmp=""
__top_pid=$BASHPID
trap '[ -n "$tmp" ] && [ "$BASHPID" = "$__top_pid" ] && rm -rf "$tmp" || true' EXIT

usage() {
	echo "usage: $0 [--from <ref>] <Task-Queue-ID>" >&2
	echo "       $0 --self-test" >&2
	exit 2
}

has_target_footer() {
	local sha=$1 id=$2 parsed
	parsed=$(git log -1 --format='%B' "$sha" | git interpret-trailers --parse 2>/dev/null) || return 1
	printf '%s\n' "$parsed" | grep -qi "^task-queue-id: $id\$"
}

# resolve_base: same default chain as heal-daemon-sweep.
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

# span_filesets base: per-commit change-set fingerprints over base..HEAD,
# oldest first — printed for the FAIL diff when the footer view diverges.
span_filesets() {
	local base=$1
	git rev-list --reverse "$base..HEAD" | while IFS= read -r c; do
		printf '%s\n' "$(git diff-tree --no-commit-id --name-only -r "$c" | sort | md5sum | cut -d' ' -f1)"
	done
}

check() {
	local id="" base_arg=""
	while [ $# -gt 0 ]; do
		case $1 in
		--from)
			[ $# -ge 2 ] || usage
			base_arg=$2
			shift 2
			;;
		-*) usage ;;
		*)
			[ -z "$id" ] || usage
			case $1 in
			"" | *[!0-9A-Za-z._-]*) die "invalid Task-Queue-ID '$1': must be a hex task id or session-scope slug" ;;
			esac
			id=$1
			shift
			;;
		esac
	done
	[ -n "$id" ] || usage
	git rev-parse --is-inside-work-tree >/dev/null 2>&1 || die "not inside a git work tree"
	local base
	base=$(resolve_base "$base_arg")
	local span
	span=$(git rev-list --reverse "$base..HEAD")
	if [ -z "$span" ]; then
		echo "ATTRIBUTION OK (0 commits in $base..HEAD; nothing to check)"
		return 0
	fi

	local fail=0 n=0
	# 1: every span commit carries the target footer machine-visibly.
	# 2: the footer-attributed view (git log --grep) lists EXACTLY the span
	#    commits, oldest-first — a rebuild that re-footers a different set
	#    shows up as a set mismatch even when each side looks fine alone.
	local footer_view
	footer_view=$(git log --reverse --format='%H' --grep="^Task-Queue-ID: $id\$" -i "$base..HEAD")
	if [ "$footer_view" != "$span" ]; then
		echo "FAIL: footer-attributed commit set differs from span $base..HEAD — rebuild changed which commits carry the footer" >&2
		diff <(printf '%s\n' "$span") <(printf '%s\n' "$footer_view") >&2 || true
		fail=1
	fi
	local c
	while IFS= read -r c; do
		n=$((n + 1))
		if ! has_target_footer "$c" "$id"; then
			echo "FAIL: span commit $c ($(git log -1 --format='%s' "$c")) lacks a trailer-parseable footer 'Task-Queue-ID: $id'" >&2
			fail=1
		fi
	done <<<"$span"

	# 3: on a set mismatch, print the span's per-commit file-set
	# fingerprints so a rebuild that moved files between commits is visible
	# in the FAIL output next to the queue's attribution.
	if [ "$footer_view" != "$span" ]; then
		span_filesets "$base" | sed 's/^/  span fileset: /' >&2
	fi

	if [ "$fail" = "0" ]; then
		echo "ATTRIBUTION OK ($n commits in $base..HEAD all footer-attributed to $id)"
		return 0
	fi
	die "attribution drift in $base..HEAD (see FAIL lines above); re-heal or re-footer before closeout"
}

self_test() {
	local self=$0
	case $self in
	/*) ;;
	*) self="$PWD/${self#./}" ;;
	esac
	tmp=$(mktemp -d) || die "mktemp failed"
	local repo="$tmp/repo"
	git init -q -b master "$repo"
	git -C "$repo" config user.email t@t
	git -C "$repo" config user.name t
	git -C "$repo" config tag.gpgSign false
	local ok=0 fail=0

	# Green path: healed span — every commit footered, file-sets intact.
	echo a >"$repo/a.txt"
	git -C "$repo" add a.txt
	git -C "$repo" commit -qm "base"
	git -C "$repo" update-ref refs/remotes/origin/master HEAD
	echo b >"$repo/b.txt"
	git -C "$repo" add b.txt
	git -C "$repo" commit -qm $'chore: sweep one\n\nTask-Queue-ID: deadbeef00000000000000000000000000000001'
	echo c >"$repo/c.txt"
	git -C "$repo" add c.txt
	git -C "$repo" commit -qm $'chore: sweep two\n\nTask-Queue-ID: deadbeef00000000000000000000000000000001'
	local good_base
	good_base=$(git -C "$repo" rev-parse HEAD~2)
	(cd "$repo" && "$self" --from "$good_base" deadbeef00000000000000000000000000000001) >"$tmp/out_good" 2>"$tmp/err_good" &&
		grep -qF "ATTRIBUTION OK (2 commits" "$tmp/out_good" &&
		ok=$((ok + 1)) || {
		fail=$((fail + 1))
		echo "SELF-TEST FAIL: clean span must pass" >&2
		cat "$tmp/out_good" "$tmp/err_good" >&2
	}

	# Drift path: a later rebuild re-set the message of the HEAD commit
	# (footer dropped, SHAs changed again post-heal) — the set mismatch AND
	# the machine-visibility check must both fire.
	local drift="$tmp/drift"
	cp -r "$repo" "$drift"
	git -C "$drift" commit -q --amend -m "chore: sweep two rebuilt"
	(cd "$drift" && "$self" --from "$good_base" deadbeef00000000000000000000000000000001) >"$tmp/out_drift" 2>"$tmp/err_drift"
	if [ "$?" != "0" ] &&
		grep -qF "footer-attributed commit set differs" "$tmp/err_drift" &&
		grep -qF "lacks a trailer-parseable footer" "$tmp/err_drift" &&
		grep -qF "span fileset:" "$tmp/err_drift"; then
		ok=$((ok + 1))
	else
		fail=$((fail + 1))
		echo "SELF-TEST FAIL: footer dropped by rebuild must FAIL on set + visibility + filesets" >&2
		cat "$tmp/out_drift" "$tmp/err_drift" >&2
	fi

	# Glued-footer class (row 276): the last span commit's
	# footer is stranded above an attribution block — invisible to
	# interpret-trailers, so the machine-visibility check must flag it.
	local glued="$tmp/glued"
	cp -r "$repo" "$glued"
	git -C "$glued" commit -q --amend -m "$(printf 'chore: sweep two\n\nTask-Queue-ID: deadbeef00000000000000000000000000000001\n\nGenerated with Crush\n\nAssisted-by: t')"
	(cd "$glued" && "$self" --from "$good_base" deadbeef00000000000000000000000000000001) >"$tmp/out_glued" 2>"$tmp/err_glued"
	if [ "$?" != "0" ] && grep -qF "lacks a trailer-parseable footer" "$tmp/err_glued"; then
		ok=$((ok + 1))
	else
		fail=$((fail + 1))
		echo "SELF-TEST FAIL: glued footer (unparseable trailer) must FAIL" >&2
		cat "$tmp/out_glued" "$tmp/err_glued" >&2
	fi

	# Footer-set drift: rebuild dropped the footer off the middle commit —
	# the set mismatch (check 2) must fire even though the remaining footer
	# commits look fine.
	local unfootered="$tmp/unfootered"
	cp -r "$repo" "$unfootered"
	git -C "$unfootered" commit -q --amend -m "chore: sweep two rebuilt"
	(cd "$unfootered" && "$self" --from "$good_base" deadbeef00000000000000000000000000000001) >"$tmp/out_unfootered" 2>"$tmp/err_unfootered"
	if [ "$?" != "0" ] && grep -qF "footer-attributed commit set differs" "$tmp/err_unfootered" && grep -qF "lacks a trailer-parseable footer" "$tmp/err_unfootered"; then
		ok=$((ok + 1))
	else
		fail=$((fail + 1))
		echo "SELF-TEST FAIL: footer dropped by rebuild must FAIL on set + visibility" >&2
		cat "$tmp/out_unfootered" "$tmp/err_unfootered" >&2
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
	check "$@"
fi
