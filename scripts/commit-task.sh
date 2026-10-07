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

# Heal-on-sweep: the daemon may have committed the same files between the
# caller's edit and now (sweeps land <60s). A bare "nothing to commit"
# here strands the work unattributed — detect the same-path daemon commit
# and heal it onto the task instead of failing blind (02-35 §f row;
# heal-daemon-sweep.sh owns the actual rewrite, with all its rails).
if ! commit_out=$(git commit -m "$subject" -m "Task-Queue-ID: $task_id" 2>&1); then
	if printf '%s' "$commit_out" | grep -q 'nothing to commit'; then
		head_subj=$(git log -1 --format='%s')
		if printf '%s' "$head_subj" | grep -Eq '^chore: auto-commit [0-9]+ changed file\(s\) \(heuristic\)$'; then
			echo "FAIL: the daemon already committed these files as:" >&2
			echo "  $(git log -1 --oneline)" >&2
			echo "Heal it onto the task (unpushed only; rails + verification built in):" >&2
			echo "  scripts/heal-daemon-sweep.sh <task-id>   # from the sweep's base" >&2
			echo "or, if the sweep is exactly yours and local-only, claim it with:" >&2
			echo "  scripts/fold-marker.sh $task_id $(git rev-parse --short=8 HEAD) \"$subject\"" >&2
			exit 1
		fi
	fi
	printf '%s\n' "$commit_out" >&2
	exit 1
fi
