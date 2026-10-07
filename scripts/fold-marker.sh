#!/usr/bin/env bash
# fold-marker.sh: create the empty footer commit that claims a footer-less
# daemon fold onto a task (TODO row 206). The daemon sweeps source edits
# into `chore:` commits with no Task-Queue-ID, leaving the queue's git
# cross-reference blind to the work; when folding is sanctioned (local-only
# + contiguous + exactly-mine files), this script stamps the standard
# marker commit instead of each window hand-inventing the maneuver.
#
# Usage: scripts/fold-marker.sh <task-id> <folded-sha> <subject> [file]…
#   <task-id>     queue task ID (hex, 16+ chars)
#   <folded-sha>  the daemon commit that swept the work (must be unpushed)
#   <subject>     what the folded work did (commit subject line)
#   [file]…       optional explicit claim list; when given, EVERY folded
#                 file must be named (the exactly-mine rail), else refuse
#
# Rails: refuses a folded sha that is missing, pushed, or already carries
# a footer; --allow-empty flag not needed (the marker is empty by design).
set -euo pipefail

usage() {
	echo "usage: $0 <task-id> <folded-sha> <subject> [file]…" >&2
	exit 2
}

[ $# -ge 3 ] || usage
task_id=$1
folded=$2
subject=$3
shift 3

case "$task_id" in
*[!a-f0-9]* | '') echo "FAIL: task id '$task_id' is not a queue task ID (hex, 16+ chars)" >&2; exit 1 ;;
esac
if [ ${#task_id} -lt 16 ]; then
	echo "FAIL: task id '$task_id' is shorter than 16 chars" >&2
	exit 1
fi

git rev-parse --verify --quiet "$folded^{commit}" >/dev/null || {
	echo "FAIL: folded sha '$folded' does not resolve to a commit" >&2
	exit 1
}

# History policy: the fold disclosure points at a commit the queue will
# attribute; a pushed fold is already history and must not be re-claimed
# by a NEW marker (heal-daemon-sweep refuses pushed ranges for the same
# reason).
if git merge-base --is-ancestor "$folded" origin/master 2>/dev/null; then
	echo "FAIL: folded sha '$folded' is already pushed (history policy; claim it in prose instead)" >&2
	exit 1
fi

if git log -1 --format='%B' "$folded" | grep -q '^Task-Queue-ID: '; then
	echo "FAIL: folded sha '$folded' already carries a Task-Queue-ID footer (nothing to fold)" >&2
	exit 1
fi

# Exactly-mine rail: with an explicit claim list, the folded commit's
# file set must match it exactly — a foreign file inside the fold is the
# row-165 not-exactly-mine refusal class.
if [ $# -gt 0 ]; then
	folded_files=$(git diff-tree --no-commit-id --name-only -r "$folded" | sort)
	claimed_files=$(printf '%s\n' "$@" | sort)
	if [ "$folded_files" != "$claimed_files" ]; then
		echo "FAIL: folded file set does not match the claim (exactly-mine rail):" >&2
		diff <(printf '%s\n' "$folded_files") <(printf '%s\n' "$claimed_files") | sed 's/^/  /' >&2
		exit 1
	fi
fi

git commit --allow-empty -m "$subject" -m "Fold disclosure: $folded swept this task's source delta into a
footer-less daemon commit (auto-commit sweep); this marker claims that
work onto $task_id for the queue's git cross-reference.

Foreign-hunk disclaimer: only files this window authored are claimed;
any foreign hunks inside $folded belong to their own windows.

Task-Queue-ID: $task_id"
echo "fold marker created for $task_id claiming $folded"
