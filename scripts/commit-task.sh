#!/usr/bin/env bash
# Stage-specified-files + commit-with-task-footer wrapper (the sanctioned
# prevention for footer-less daemon sweeps, receipts #2–#6 — TODO row
# "Footered-commit atomicity", 2026-09-29 23-57 report d1/e1/g3): the
# auto-commit daemon sweeps any edit left unstaged across a multi-minute
# gate run as a `chore:` commit, which the queue's git cross-reference
# never attributes to the task. This script makes the safe order the lazy
# order: stage EXACTLY the named files, commit immediately with the
# Task-Queue-ID trailer as the LAST line (git interpret-trailers must see
# it — the commit-msg hook enforces the same shape).
#
# Usage: scripts/commit-task.sh <task-id> <subject> <file>…
#
#   <task-id>   queue-assigned ID, verbatim (hex, 16+ chars)
#   <subject>   commit subject (first line)
#   <file>…     files to stage — each must exist; nothing else is staged
#
# Exit codes: 0 committed; 1 bad args / missing file / staging or commit
# failure (nothing is left staged on failure paths before commit).
set -euo pipefail

if [ $# -lt 3 ]; then
	echo "usage: $0 <task-id> <subject> <file>…" >&2
	exit 1
fi

task_id=$1
subject=$2
shift 2

case $task_id in
*[!a-f0-9]* | '')
	echo "FAIL: task id '$task_id' is not a queue task ID (hex, 16+ chars)" >&2
	exit 1
	;;
esac
if [ ${#task_id} -lt 16 ]; then
	echo "FAIL: task id '$task_id' is shorter than 16 chars" >&2
	exit 1
fi

for f in "$@"; do
	if [ ! -e "$f" ]; then
		echo "FAIL: file does not exist: $f" >&2
		exit 1
	fi
done

git add -- "$@"
git commit -m "$subject" -m "Task-Queue-ID: $task_id"
