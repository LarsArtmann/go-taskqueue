#!/usr/bin/env bash
# One-command evidence copy into the repo (00-43 report §e1/§f1, asked for in
# four consecutive windows): check-ignore FIRST, copy second, SHA256SUMS
# regen, daemon --stat diff. A clean `git status` is not a complete archive
# while the auto-commit daemon is live — ignore rules + the daemon + a
# clean-looking commit form the silent-loss triangle (f9 near-miss: 16 `*.log`
# files invisible to git behind an archive README; 06-41 report §d1/§e1-2).
# This script makes the safe order the lazy order.
#
# Usage: scripts/archive-evidence.sh [--wait SECONDS] <src>… <archive-dir>
#
#   <src>…        evidence files (flat archive — directories are rejected,
#                 tar trees first). Caller-cwd-relative or absolute.
#   <archive-dir> destination INSIDE the repo (docs/status/assets/<name> is
#                 the convention), caller-cwd- or repo-root-relative.
#   --wait N      poll up to N seconds for the auto-commit daemon's commit
#                 and run the --stat diff mechanically (default: print the
#                 verify commands and exit 0 — the daemon sweeps within
#                 minutes; blocking on it is the caller's call).
#
# Every archive regenerates its SHA256SUMS manifest with the documented
# command shape (ls-equivalent: non-hidden entries, manifest excluded — the
# ghost-archive gate's coverage check assumes exactly that file set).
#
# Exit codes: 0 success (or verification PENDING, printed loudly), 1 failure
# (ignored destination, outside-repo destination, copy/manifest error, or a
# post-copy daemon commit missing an intended file — the f9 shape).
set -uo pipefail

wait_secs=0
while [ $# -gt 0 ]; do
	case $1 in
	--wait)
		if [ $# -lt 2 ]; then
			echo "FAIL: --wait needs a seconds value" >&2
			exit 1
		fi
		wait_secs=$2
		shift 2
		;;
	-h | --help)
		sed -n '2,27p' "$0" | sed 's/^# \{0,1\}//'
		exit 0
		;;
	-*)
		echo "FAIL: unknown flag: $1" >&2
		exit 1
		;;
	*)
		break
		;;
	esac
done

if [ $# -lt 2 ]; then
	echo "usage: scripts/archive-evidence.sh [--wait SECONDS] <src>… <archive-dir>" >&2
	exit 1
fi

argc=$#
adir_arg=${!argc}
src_args=()
i=1
while [ "$i" -lt "$argc" ]; do
	src_args+=("${!i}")
	i=$((i + 1))
done

root=$(git rev-parse --show-toplevel 2>/dev/null) || {
	echo "FAIL: not inside a git repository (run from the repo the evidence goes into)" >&2
	exit 1
}

srcs_abs=()
for s in "${src_args[@]}"; do
	if ! abs=$(realpath -e -- "$s"); then
		echo "FAIL: source does not exist: $s" >&2
		exit 1
	fi
	if [ ! -f "$abs" ]; then
		echo "FAIL: not a regular file: $s (archives are flat — tar directory trees first)" >&2
		exit 1
	fi
	srcs_abs+=("$abs")
done

adir=$(realpath -m -- "$adir_arg")
case $adir in
"$root")
	echo "FAIL: archive dir must be inside the repo, not the repo root itself" >&2
	exit 1
	;;
"$root"/*) ;;
*)
	echo "FAIL: archive dir resolves outside the repository: $adir_arg -> $adir" >&2
	exit 1
	;;
esac
adir_rel=${adir#"$root"/}

declare -A seen_base=()
dests_abs=()
dests_rel=()
for abs in "${srcs_abs[@]}"; do
	base=$(basename -- "$abs")
	if [ -n "${seen_base[$base]+x}" ]; then
		echo "FAIL: duplicate basename among sources: $base" >&2
		exit 1
	fi
	seen_base[$base]=1
	dest_abs="$adir/$base"
	if [ "$abs" = "$dest_abs" ]; then
		echo "FAIL: source is already the destination: $abs" >&2
		exit 1
	fi
	dests_abs+=("$dest_abs")
	dests_rel+=("$adir_rel/$base")
done

cd "$root" || exit 1

# Step 1 — check-ignore FIRST: a destination matching an ignore rule is an
# untracked ghost-to-be (the daemon's add-everything sweep skips it and the
# tree looks clean). check-ignore reports only untracked paths (tracked
# files are carried by git regardless of rules). A NEGATION (`!`) hit is
# the opposite of ghost-risk: the repo explicitly un-ignores the path
# (.gitignore:94, evidence logs under docs/status/assets), so the daemon
# sweep carries it — only positive-rule hits fail BEFORE any copy
# (03-47 report §b5/§e5).
rc=0
ignored=$(git check-ignore -v -- "${dests_rel[@]}") || rc=$?
if [ "$rc" -gt 1 ]; then
	echo "FAIL: git check-ignore failed (rc=$rc)" >&2
	exit 1
fi
fail=0
if [ -n "$ignored" ]; then
	while IFS= read -r hit; do
		[ -n "$hit" ] || continue
		meta=${hit%%$'\t'*}
		rule=${meta#*:*:}
		ipath=${hit##*$'\t'}
		case $rule in
		'!'*)
			echo "ok: $ipath matches the negation rule $rule (deliberately un-ignored content) — proceeding"
			continue
			;;
		esac
		echo "FAIL: $ipath matches a gitignore rule and would be silently dropped by the auto-commit daemon (ghost-to-be)"
		echo "  rule: $hit"
		echo "  fix: un-ignore it, or copy consciously and git add -f it (the ghost-archive gate demands it tracked)"
		fail=1
	done < <(printf '%s\n' "$ignored")
fi
if [ "$fail" -eq 1 ]; then
	echo "nothing copied — check-ignore runs FIRST (06-41 report §d1)" >&2
	exit 1
fi

# Step 2 — copy second.
copy_start=$(date +%s)
if ! mkdir -p -- "$adir"; then
	echo "FAIL: cannot create $adir" >&2
	exit 1
fi
while IFS= read -r sub; do
	echo "FAIL: subdirectory inside archive (flat archives only): $sub" >&2
	fail=1
done < <(find "$adir" -mindepth 1 -maxdepth 1 -type d)
if [ "$fail" -eq 1 ]; then
	echo "nothing copied — flatten the archive first" >&2
	exit 1
fi
for i in "${!srcs_abs[@]}"; do
	if ! cp -p -- "${srcs_abs[$i]}" "${dests_abs[$i]}"; then
		echo "FAIL: copy failed: ${srcs_abs[$i]} -> ${dests_abs[$i]}" >&2
		exit 1
	fi
done

# Step 3 — SHA256SUMS regen (documented command shape: non-hidden entries,
# manifest excluded, sorted; flat dir guaranteed above).
if ! (cd "$adir" && find . -mindepth 1 -maxdepth 1 ! -name SHA256SUMS ! -name '.*' -printf '%f\n' | sort | xargs sha256sum >SHA256SUMS); then
	echo "FAIL: SHA256SUMS regeneration failed in $adir_rel" >&2
	exit 1
fi

echo "intended file set under $adir_rel/ ($((${#dests_rel[@]} + 1)) files):"
printf '  %s\n' "${dests_rel[@]}" "$adir_rel/SHA256SUMS"

# Step 4 — daemon --stat diff: the newest auto-commit touching the archive
# dir must carry every intended file. A commit from the same second as the
# copy or earlier only means the daemon has not swept yet (PENDING — the
# safe direction: 1s git timestamp granularity makes same-second ambiguous);
# a strictly post-copy commit missing a file is the f9 silent-loss shape
# (FAIL).
verified=0
daemon_sha=""
check_daemon() {
	local ctime missing want
	daemon_sha=$(git log -1 --format=%H --grep='^chore: auto-commit' -- "$adir_rel")
	if [ -z "$daemon_sha" ]; then
		return 0
	fi
	ctime=$(git show -s --format=%ct "$daemon_sha")
	missing=0
	while IFS= read -r want; do
		if ! git show --name-only --format= "$daemon_sha" | grep -Fxq -- "$want"; then
			missing=1
			break
		fi
	done < <(printf '%s\n' "${dests_rel[@]}" "$adir_rel/SHA256SUMS")
	if [ "$missing" -eq 0 ]; then
		verified=1
	elif [ "$ctime" -gt "$copy_start" ]; then
		echo "FAIL: daemon commit $(git show -s --format=%h "$daemon_sha") is post-copy yet misses intended files (the f9 silent-loss shape):"
		git show --name-only --format='  %h %s' "$daemon_sha"
		return 1
	fi
	return 0
}

fail=0
check_daemon || fail=1
if [ "$fail" -eq 0 ] && [ "$verified" -eq 0 ]; then
	deadline=$(($(date +%s) + wait_secs))
	while [ "$(date +%s)" -lt "$deadline" ]; do
		sleep 2
		check_daemon || {
			fail=1
			break
		}
		[ "$verified" -eq 1 ] && break
	done
fi

if [ "$fail" -eq 1 ]; then
	echo "FAIL: evidence archive verification failed — resolve above, then re-run" >&2
	exit 1
fi
if [ "$verified" -eq 1 ]; then
	sha=$(git show -s --format=%h "$daemon_sha")
	echo "SUCCESS: daemon commit $sha carries the full intended set (git show --stat $sha to eyeball)"
else
	echo "PENDING: the auto-commit daemon has not swept $adir_rel/ yet."
	echo "Do NOT trust a clean git status — once the daemon commits, diff its stat against the intended set:"
	echo "  git log -1 --format=%h --grep='^chore: auto-commit' -- $adir_rel"
	echo "  git show --stat <sha>"
	echo "(or re-run with --wait SECONDS to block for the daemon)"
fi
exit 0
