#!/usr/bin/env bash
# rebuild-lineage.sh: commit-tree lineage-rebuild recovery for an unpushed
# span (the 2026-11-09 wave-completion closeout §d1/§f5 recipe, pinned as a
# runnable tool because the heal-tooling rows guarantee a next time).
#
# What it does: rebuilds every commit in <base>..HEAD via git commit-tree,
# preserving each original's message, tree, author name/email/date AND
# committer name/email/date byte-exact. Commits listed via --mine (1-based,
# oldest-first) get a `Task-Queue-ID: <id>` trailer appended — footer ONLY
# on mine, foreign subjects untouched. The branch then moves with
# `git reset --soft` (tree-identical, so index and worktree carry over
# untouched).
#
# Verification (backup ref refs/original/rebuild-lineage dropped ONLY after
# all pass; kept + non-zero exit otherwise):
#   - same commit count and per-commit trees in the span;
#   - `git diff <oldtip> <newtip>` empty (tree-identical span);
#   - every --mine commit carries exactly one well-formed, trailer-parseable
#     footer; every other commit's message is byte-identical to the original.
#
# Unlike heal-daemon-sweep.sh this is NOT msg-filter based: it rebuilds the
# commit DAG, so it also fixes cases the msg-filter cannot (e.g. rebuilding
# a span onto a corrected parent). Refuses a pushed commit or a tag inside
# the span (same history policy).
#
# Usage: scripts/rebuild-lineage.sh [--repo <path>] [--base <ref>] [--dry-run] <id> --mine 2,3
#        scripts/rebuild-lineage.sh --self-test
#   <id>     hex task id or session-scope slug (same grammar as heal).
#   --mine   comma-separated 1-based indices (oldest-first) of the span's
#            commits that are YOURS — they get the footer; all others keep
#            their messages verbatim. Required (default-empty rebuilds are
#            a footgun).
set -uo pipefail

BACKUP_REF="refs/original/rebuild-lineage"

die() {
	echo "rebuild-lineage: $*" >&2
	exit 1
}

usage() {
	echo "usage: $0 [--repo <path>] [--base <ref>] [--dry-run] <id> --mine <i,j,...>" >&2
	echo "       $0 --self-test" >&2
	exit 2
}

footer_well_formed() {
	local msg=$1 id=$2 last
	last=$(printf '%s\n' "$msg" | sed '/^[[:space:]]*$/d' | tail -n 1)
	[ "$last" = "Task-Queue-ID: $id" ]
}

resolve_base() {
	local requested=$1
	if [ -n "$requested" ]; then
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

# is_mine <index> <mine-list>
is_mine() {
	local idx=$1 list=$2
	printf '%s\n' "$(tr ',' '\n' <<<"$list")" | grep -qx "$idx"
}

main() {
	local repo_arg="" base_arg="" dry_run="" id="" mine=""
	while [ $# -gt 0 ]; do
		case $1 in
		--repo)
			[ $# -ge 2 ] || usage
			repo_arg=$2
			shift 2
			;;
		--base)
			[ $# -ge 2 ] || usage
			base_arg=$2
			shift 2
			;;
		--dry-run)
			dry_run=1
			shift
			;;
		--mine)
			[ $# -ge 2 ] || usage
			mine=$2
			shift 2
			;;
		-*) usage ;;
		*)
			[ -z "$id" ] || usage
			case $1 in
			"" | *[!0-9A-Za-z._-]*) die "invalid id '$1': hex task id or session-scope slug only" ;;
			esac
			id=$1
			shift
			;;
		esac
	done
	[ -n "$id" ] || usage
	[ -n "$mine" ] || die "--mine is required (comma-separated 1-based indices of YOUR commits, oldest-first)"
	command -v git >/dev/null 2>&1 || die "git not found in PATH"
	if [ -n "$repo_arg" ]; then
		[ -d "$repo_arg" ] || die "--repo '$repo_arg' is not a directory"
		cd "$repo_arg" || die "cannot cd into --repo '$repo_arg'"
	fi
	git rev-parse --is-inside-work-tree >/dev/null 2>&1 || die "not inside a git work tree"
	local base
	base=$(resolve_base "$base_arg")
	local span
	span=$(git rev-list --reverse "$base..HEAD")
	[ -n "$span" ] || die "range $base..HEAD is empty; nothing to rebuild"

	# History policy: never rewrite pushed commits; a tag inside the span
	# would dangle off-branch after the rebuild.
	local r hits
	while IFS= read -r r; do
		hits=$(comm -12 <(git rev-list "$r" | sort) <(git rev-list "$base..HEAD" | sort))
		[ -z "$hits" ] || die "refusing: pushed commits are inside the rebuild span (history policy)"
	done < <(git for-each-ref --format='%(refname)' 'refs/remotes/')
	while IFS= read -r r; do
		hits=$(comm -12 <(git rev-list "$r" | sort) <(git rev-list "$base..HEAD" | sort))
		[ -z "$hits" ] || die "refusing: tag(s) reference commits inside the rebuild span; re-tag after rebuilding"
	done < <(git for-each-ref --format='%(objectname)' refs/tags)

	local footer="Task-Queue-ID: $id"
	local oldtip
	oldtip=$(git rev-parse HEAD)

	if [ -n "$dry_run" ]; then
		echo "DRY-RUN: would rebuild $(git rev-list --count "$base..HEAD") commit(s) in $base..HEAD, footer on mine=[$mine]"
		local i=0 c
		while IFS= read -r c; do
			i=$((i + 1))
			if is_mine "$i" "$mine"; then
				echo "  $i $(git log -1 --oneline "$c") -> footer added"
			else
				echo "  $i $(git log -1 --oneline "$c") -> message kept verbatim"
			fi
		done <<<"$span"
		echo "DRY-RUN: no commits were rebuilt"
		return 0
	fi

	# Rebuild via commit-tree: preserves message, tree, author AND committer
	# identity + dates; appends the footer only to --mine commits.
	git update-ref -d "$BACKUP_REF" 2>/dev/null
	git update-ref "$BACKUP_REF" "$oldtip"
	local parent="$base" newtip="" i=0 c tree msg new
	while IFS= read -r c; do
		i=$((i + 1))
		tree=$(git rev-parse "$c^{tree}")
		msg=$(git log -1 --format='%B' "$c")
		if is_mine "$i" "$mine"; then
			msg=$(printf '%s\n\n%s' "${msg%%$'\n'}" "$footer")
		fi
		new=$(GIT_AUTHOR_NAME="$(git log -1 --format='%an' "$c")" \
			GIT_AUTHOR_EMAIL="$(git log -1 --format='%ae' "$c")" \
			GIT_AUTHOR_DATE="$(git log -1 --format='%aD' "$c")" \
			GIT_COMMITTER_NAME="$(git log -1 --format='%cn' "$c")" \
			GIT_COMMITTER_EMAIL="$(git log -1 --format='%ce' "$c")" \
			GIT_COMMITTER_DATE="$(git log -1 --format='%cD' "$c")" \
			git commit-tree "$tree" -p "$parent" -m "$msg") || die "commit-tree failed at $c"
		parent="$new"
		newtip="$new"
	done <<<"$span"

	# Verify BEFORE moving the branch.
	local fail=0
	if [ "$(git rev-list --count "$parent" --not "$base")" != "$(git rev-list --count "$base..HEAD")" ]; then
		echo "FAIL: rebuilt span count differs" >&2
		fail=1
	fi
	if [ -n "$(git diff "$oldtip" "$newtip")" ]; then
		echo "FAIL: tree differs from the original tip" >&2
		fail=1
	fi
	local j=0 oc nc
	while IFS= read -r oc && IFS= read -r nc <&3; do
		j=$((j + 1))
		if [ "$(git rev-parse "$oc^{tree}")" != "$(git rev-parse "$nc^{tree}")" ]; then
			echo "FAIL: commit $j tree changed" >&2
			fail=1
		fi
		local omsg nmsg
		omsg=$(git log -1 --format='%B' "$oc")
		nmsg=$(git log -1 --format='%B' "$nc")
		if is_mine "$j" "$mine"; then
			if ! footer_well_formed "$nmsg" "$id"; then
				echo "FAIL: rebuilt mine-commit $j lacks a well-formed trailing footer" >&2
				fail=1
			fi
			if ! git log -1 --format='%B' "$nc" | git interpret-trailers --parse | grep -qi "^task-queue-id: $id\$"; then
				echo "FAIL: rebuilt footer in commit $j is not trailer-parseable" >&2
				fail=1
			fi
		elif [ "$omsg" != "$nmsg" ]; then
			echo "FAIL: commit $j message changed (must be byte-identical)" >&2
			fail=1
		fi
		for field in an ae aD cn ce cD; do
			if [ "$(git log -1 --format="%$field" "$oc")" != "$(git log -1 --format="%$field" "$nc")" ]; then
				echo "FAIL: commit $j identity/date field %$field changed" >&2
				fail=1
			fi
		done
	done <<<"$span" 3<<<"$(git rev-list --reverse "$parent" --not "$base")"
	if [ "$fail" -ne 0 ]; then
		die "verification FAILED — backup kept at $BACKUP_REF; recovery: git reset --soft $BACKUP_REF"
	fi

	# Tree-identical, so a soft reset moves the branch and leaves index +
	# worktree (which already match the rebuilt tree) untouched.
	git reset --soft "$newtip" || die "branch move failed — backup kept at $BACKUP_REF"
	echo "FORK-RECORD: $oldtip→$newtip"
	git update-ref -d "$BACKUP_REF"
	echo "REBUILD OK — backup ref dropped"
}

# --- self-test ---
self_test() {
	local self=$0
	case $self in
	/*) ;;
	*) self="$PWD/${self#./}" ;;
	esac
	local tmp
	tmp=$(mktemp -d) || die "mktemp failed"
	trap 'rm -rf "$tmp"' EXIT
	local repo="$tmp/repo"
	git init -q -b master "$repo"
	git -C "$repo" config user.email t@t
	git -C "$repo" config user.name t
	git -C "$repo" config core.editor true

	local ok=0 fail=0

	commit_at() {
		local f=$1 m=$2 date=$3
		echo "$f" >"$repo/$f"
		git -C "$repo" add "$f"
		git -C "$repo" commit -qm "$m"
		GIT_COMMITTER_DATE="$date" git -C "$repo" commit -q --amend --no-edit --date "$date"
	}

	echo base >"$repo/base.txt"
	git -C "$repo" add base.txt
	git -C "$repo" commit -qm "base"
	commit_at a.txt "work: one" "2026-11-09T10:00:00 +01:00"
	commit_at b.txt "work: two" "2026-11-09T11:00:00 +01:00"
	commit_at c.txt "work: three" "2026-11-09T12:00:00 +01:00"
	git -C "$repo" update-ref refs/remotes/origin/master "$(git -C "$repo" rev-parse HEAD~3)"
	local old1 old2 old3 oldtip rc
	old1=$(git -C "$repo" rev-parse HEAD~2)
	old2=$(git -C "$repo" rev-parse HEAD~1)
	old3=$(git -C "$repo" rev-parse HEAD)
	oldtip=$old3

	# Dry-run: prints the plan, rebuilds nothing.
	(cd "$repo" && "$self" --base origin/master --dry-run deadbeef00000000000000000000000000000001 --mine 2) >"$tmp/out_dry" 2>&1
	rc=$?
	if [ "$rc" = 0 ] &&
		grep -qF 'DRY-RUN: would rebuild 3 commit(s)' "$tmp/out_dry" &&
		grep -qF 'work: two -> footer added' "$tmp/out_dry" &&
		grep -qF 'message kept verbatim' "$tmp/out_dry" &&
		[ "$(git -C "$repo" rev-parse HEAD)" = "$oldtip" ]; then
		ok=$((ok + 1))
	else
		fail=$((fail + 1))
		echo "SELF-TEST FAIL: dry-run rc + plan + no rewrite" >&2
		cat "$tmp/out_dry" >&2
	fi

	# Real rebuild: footer ONLY on mine=2; dates preserved; tree identical.
	(cd "$repo" && "$self" --base origin/master deadbeef00000000000000000000000000000001 --mine 2) >"$tmp/out1" 2>"$tmp/err1"
	rc=$?
	if [ "$rc" = 0 ]; then
		ok=$((ok + 1))
	else
		fail=$((fail + 1))
		echo "SELF-TEST FAIL: rebuild rc" >&2
		cat "$tmp/out1" "$tmp/err1" >&2
	fi
	local new1 new2 new3 newtip
	new1=$(git -C "$repo" rev-parse HEAD~2)
	new2=$(git -C "$repo" rev-parse HEAD~1)
	new3=$(git -C "$repo" rev-parse HEAD)
	newtip=$new3
	if [ "$new1" != "$old1" ] && [ "$new2" != "$old2" ] && [ "$new3" != "$old3" ]; then
		ok=$((ok + 1))
	else
		fail=$((fail + 1))
		echo "SELF-TEST FAIL: shas moved" >&2
	fi
	if [ -z "$(git -C "$repo" diff "$oldtip" "$newtip")" ]; then
		ok=$((ok + 1))
	else
		fail=$((fail + 1))
		echo "SELF-TEST FAIL: tree-identical tip" >&2
	fi
	if ! git -C "$repo" log -1 --format='%B' "$new1" | grep -q '^Task-Queue-ID: ' &&
		footer_well_formed "$(git -C "$repo" log -1 --format='%B' "$new2")" "deadbeef00000000000000000000000000000001" &&
		! git -C "$repo" log -1 --format='%B' "$new3" | grep -q '^Task-Queue-ID: '; then
		ok=$((ok + 1))
	else
		fail=$((fail + 1))
		echo "SELF-TEST FAIL: footer only on mine=2" >&2
	fi
	if [ "$(git -C "$repo" log -1 --format='%B' "$new1")" = "work: one" ] &&
		[ "$(git -C "$repo" log -1 --format='%B' "$new3")" = "work: three" ]; then
		ok=$((ok + 1))
	else
		fail=$((fail + 1))
		echo "SELF-TEST FAIL: messages preserved off-mine" >&2
	fi
	if [ "$(git -C "$repo" log -1 --format='%aI' "$new2")" = "2026-11-09T11:00:00+01:00" ] &&
		[ "$(git -C "$repo" log -1 --format='%cI' "$new2")" = "2026-11-09T11:00:00+01:00" ]; then
		ok=$((ok + 1))
	else
		fail=$((fail + 1))
		echo "SELF-TEST FAIL: author+committer dates preserved" >&2
	fi
	if [ -z "$(git -C "$repo" for-each-ref 'refs/original/*')" ]; then
		ok=$((ok + 1))
	else
		fail=$((fail + 1))
		echo "SELF-TEST FAIL: backup dropped" >&2
	fi
	if grep -qF "FORK-RECORD: $oldtip→$newtip" "$tmp/out1"; then
		ok=$((ok + 1))
	else
		fail=$((fail + 1))
		echo "SELF-TEST FAIL: fork record printed" >&2
	fi

	# Rails: pushed commit inside the span refuses.
	git -C "$repo" update-ref refs/remotes/origin/master "$(git -C "$repo" rev-parse HEAD~1)"
	(cd "$repo" && "$self" --base origin/master~1 deadbeef00000000000000000000000000000002 --mine 1) >/dev/null 2>"$tmp/err2"
	rc=$?
	if [ "$rc" != 0 ]; then
		ok=$((ok + 1))
	else
		fail=$((fail + 1))
		echo "SELF-TEST FAIL: pushed commit refusal" >&2
	fi

	# Tag inside the span refuses.
	git -C "$repo" update-ref refs/remotes/origin/master "$(git -C "$repo" rev-parse HEAD~3)"
	git -C "$repo" tag spantag HEAD~1
	(cd "$repo" && "$self" --base origin/master deadbeef00000000000000000000000000000003 --mine 1) >/dev/null 2>"$tmp/err3"
	rc=$?
	if [ "$rc" != 0 ]; then
		ok=$((ok + 1))
	else
		fail=$((fail + 1))
		echo "SELF-TEST FAIL: tag refusal" >&2
	fi

	# Missing --mine refuses (footgun default).
	(cd "$repo" && "$self" --base origin/master deadbeef00000000000000000000000000000004) >/dev/null 2>"$tmp/err4"
	rc=$?
	if [ "$rc" != 0 ] && grep -qF -- '--mine is required' "$tmp/err4"; then
		ok=$((ok + 1))
	else
		fail=$((fail + 1))
		echo "SELF-TEST FAIL: missing --mine refusal" >&2
	fi

	# Empty span refuses.
	(cd "$repo" && "$self" --base HEAD deadbeef00000000000000000000000000000005 --mine 1) >/dev/null 2>&1
	rc=$?
	if [ "$rc" != 0 ]; then
		ok=$((ok + 1))
	else
		fail=$((fail + 1))
		echo "SELF-TEST FAIL: empty span refusal" >&2
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
