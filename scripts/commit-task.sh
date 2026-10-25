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
# Usage: scripts/commit-task.sh <task-id> <subject> <file>… [--body <text>]
#
#   <task-id>   queue-assigned ID, verbatim (hex, 16+ chars)
#   <subject>   commit subject (first line)
#   <file>…     files to stage — each must exist; nothing else is staged
#   --body <text>   optional commit body paragraph, placed between the
#                   subject and the footer so the Task-Queue-ID trailer
#                   stays the LAST line (closeout commits with a real
#                   message no longer need a hand-rolled heredoc)
#
# Exit codes: 0 committed; 1 bad args / missing file / staging or commit
# failure (nothing is left staged on failure paths before commit).
set -euo pipefail

self_dir=$(cd "$(dirname "$0")" && pwd)

# --self-test (2026-10-12 03-19 §e1): pins the footer-as-LAST-line shape
# in a scratch repo — with and without a body, the Task-Queue-ID trailer
# must be the final non-empty line of the commit message, or a closeout
# commit can silently end up footer-last-broken (heal-daemon-sweep then
# rejects it).
if [ "${1:-}" = "--self-test" ]; then
	tmp=$(mktemp -d)
	trap 'rm -rf "$tmp"' EXIT
	git init -q "$tmp/repo"
	cd "$tmp/repo"
	git config user.email self@test
	git config user.name selftest
	fail() {
		echo "commit-task self-test FAIL: $1" >&2
		exit 1
	}
	echo one >f.txt
	stamp=$("$self_dir/commit-task.sh" 0000000000000000abcd subject-only f.txt)
	printf '%s\n' "$stamp" | grep -qE '^committed: [0-9a-f]{7,} subject-only$' \
		|| fail "success line missing or malformed: '$stamp'"
	last=$(git log -1 --format=%B | grep -v '^$' | tail -n 1)
	[ "$last" = "Task-Queue-ID: 0000000000000000abcd" ] || fail "subject-only footer is not the last line: '$last'"
	echo two >>f.txt
	"$self_dir/commit-task.sh" 0000000000000000abcd with-body f.txt --body "Closeout report: what changed and why."
	full=$(git log -1 --format=%B)
	printf '%s' "$full" | grep -qF 'Closeout report: what changed and why.' || fail "body paragraph missing from commit message"
	last=$(printf '%s' "$full" | grep -v '^$' | tail -n 1)
	[ "$last" = "Task-Queue-ID: 0000000000000000abcd" ] || fail "with-body footer is not the last line: '$last'"
	echo "commit-task self-test ok: footer is the last line with and without a body"
	exit 0
fi

if [ $# -lt 3 ]; then
	echo "usage: $0 <task-id> <subject> <file>… [--body <text>]" >&2
	exit 1
fi

task_id=$1
subject=$2
shift 2

body=''
files=()

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

files=()
while [ $# -gt 0 ]; do
	if [ "$1" = "--body" ]; then
		[ $# -ge 2 ] || {
			echo "FAIL: --body requires a value" >&2
			exit 1
		}
		body=$2
		shift 2
	else
		files+=("$1")
		shift
	fi
done

for f in "${files[@]}"; do
	if [ ! -e "$f" ]; then
		echo "FAIL: file does not exist: $f" >&2
		exit 1
	fi
done

git add -- "${files[@]}"

# Heal-on-sweep: the daemon may have committed the same files between the
# caller's edit and now (sweeps land <60s). A bare "nothing to commit"
# here strands the work unattributed — detect the same-path daemon commit
# and heal it onto the task instead of failing blind (02-35 §f row;
# heal-daemon-sweep.sh owns the actual rewrite, with all its rails).
body_args=()
[ -z "$body" ] || body_args=(-m "$body")
if ! commit_out=$(git commit -m "$subject" "${body_args[@]}" -m "Task-Queue-ID: $task_id" 2>&1); then
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

# Row 41: a silent success forces every window into a follow-up `git log`
# to confirm the footer commit landed — echo the hash + subject instead.
echo "committed: $(git log -1 --oneline)"
