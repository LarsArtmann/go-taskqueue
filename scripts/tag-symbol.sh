#!/usr/bin/env bash
# tag-symbol.sh <tag> <repo-relative-path> <symbol> — is symbol X present
# in the PUSHED tag Y at repo-relative path Z? (row 41; the
# `git show <tag>:<file> | grep` pattern proven in the 03-51 report §a2,
# so facade-floor precondition audits — e.g. "does internal/queue/v0.4.1
# carry RequeueClassRateLimitGate before cmd/tq may cite it?" — become a
# one-liner instead of tag archaeology.)
#
# Run from anywhere inside the repo (operates on the surrounding git repo).
#
# Exit codes: 0 = present (matches listed as <tag>:<path>:<line> plus a
# countable summary), 1 = absent, 2 = error (usage, unknown tag, path not
# in the tag, or the tag is not on origin — an unpushed or locally-moved
# tag proves nothing about released consumers, which is the whole point
# of the audit).
#
#   scripts/tag-symbol.sh internal/queue/v0.4.1 internal/queue/queue.go RequeueClassRateLimitGate
#   scripts/tag-symbol.sh --self-test
set -euo pipefail

self_test() {
	local self=$0
	case $self in
	/*) ;;
	*) self="$PWD/${self#./}" ;;
	esac
	local tmp
	tmp=$(mktemp -d) || exit 2
	trap 'rm -rf "${tmp:-}"' EXIT
	git init -q -b master "$tmp/origin"
	git -C "$tmp/origin" config user.email t@t
	git -C "$tmp/origin" config user.name t
	printf 'const RequeueClassRateLimitGate = "rate-limit-gate"\n' >"$tmp/origin/queue.go"
	git -C "$tmp/origin" add queue.go
	git -C "$tmp/origin" commit -qm base
	git -C "$tmp/origin" tag -a v1 -m v1
	git clone -q "$tmp/origin" "$tmp/clone"

	local ok=0 fail=0
	expect_rc() {
		local label=$1 want=$2 got=$3
		if [ "$got" = "$want" ]; then
			ok=$((ok + 1))
		else
			fail=$((fail + 1))
			echo "SELF-TEST FAIL ($label): want rc=$want got rc=$got" >&2
		fi
	}

	local rc
	(cd "$tmp/clone" && "$self" v1 queue.go RequeueClassRateLimitGate) >/dev/null 2>&1 && rc=0 || rc=$?
	expect_rc "present in pushed tag" 0 "$rc"
	(cd "$tmp/clone" && "$self" v1 queue.go RequeueClassRateLimitGate) 2>/dev/null | grep -q 'tag-symbol: present' &&
		ok=$((ok + 1)) || {
		fail=$((fail + 1))
		echo "SELF-TEST FAIL: present case must print the countable summary" >&2
	}
	(cd "$tmp/clone" && "$self" v1 queue.go NoSuchSymbol) >/dev/null 2>&1 && rc=0 || rc=$?
	expect_rc "absent symbol" 1 "$rc"
	git -C "$tmp/clone" tag -a v2 -m v2 HEAD
	(cd "$tmp/clone" && "$self" v2 queue.go RequeueClassRateLimitGate) >/dev/null 2>&1 && rc=0 || rc=$?
	expect_rc "unpushed tag refuses" 2 "$rc"
	(cd "$tmp/clone" && "$self" v1 nope.go RequeueClassRateLimitGate) >/dev/null 2>&1 && rc=0 || rc=$?
	expect_rc "path missing from tag" 2 "$rc"
	(cd "$tmp/clone" && "$self") >/dev/null 2>&1 && rc=0 || rc=$?
	expect_rc "usage" 2 "$rc"

	if [ "$fail" = "0" ]; then
		echo "SELF-TEST OK ($ok checks)"
		return 0
	fi
	echo "SELF-TEST FAILED ($ok ok, $fail failed)" >&2
	return 1
}

if [ "${1:-}" = "--self-test" ]; then
	self_test
	exit $?
fi

[ $# -eq 3 ] || {
	echo "usage: $0 <tag> <repo-relative-path> <symbol>" >&2
	exit 2
}
tag=$1
path=$2
symbol=$3

git rev-parse --verify --quiet "refs/tags/$tag^{commit}" >/dev/null ||
	{
		echo "tag-symbol: tag '$tag' not found locally (git fetch first)" >&2
		exit 2
	}

# The tag must be on origin AND at the same commit: an unpushed or
# locally re-cut tag proves nothing about released consumers. For an
# annotated tag ls-remote also reports the peeled commit on the
# 'refs/tags/<tag>^{}' line; for a lightweight tag the plain line is the
# commit.
local_peeled=$(git rev-parse "refs/tags/$tag^{commit}")
remote_lines=$(git ls-remote --tags origin "refs/tags/$tag" "refs/tags/$tag^{}" 2>/dev/null || true)
remote_peeled=$(printf '%s\n' "$remote_lines" | awk '$2 ~ /\^\{\}$/ {print $1}')
if [ -z "$remote_peeled" ]; then
	remote_peeled=$(printf '%s\n' "$remote_lines" | awk '$2 !~ /\^\{\}$/ {print $1}')
fi
if [ -z "$remote_peeled" ]; then
	echo "tag-symbol: tag '$tag' is NOT on origin — an unpushed tag proves nothing about released consumers" >&2
	exit 2
fi
if [ "$remote_peeled" != "$local_peeled" ]; then
	echo "tag-symbol: tag '$tag' moved locally ($local_peeled) vs origin ($remote_peeled); audit the pushed position (git fetch)" >&2
	exit 2
fi

content=$(git show "$tag:$path" 2>/dev/null) || {
	echo "tag-symbol: path '$path' not found in tag '$tag'" >&2
	exit 2
}
matches=$(printf '%s\n' "$content" | grep -nF -- "$symbol" || true)
if [ -z "$matches" ]; then
	echo "tag-symbol: '$symbol' ABSENT in $tag:$path"
	exit 1
fi
printf '%s\n' "$matches" | sed "s|^|$tag:$path:|"
echo "tag-symbol: present — $(printf '%s\n' "$matches" | wc -l) match(es) for '$symbol' in $tag:$path"
