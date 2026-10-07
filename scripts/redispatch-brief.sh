#!/usr/bin/env bash
# redispatch-brief.sh — the re-dispatch FIRST BATCH as one command (row 442:
# a compliant-looking agent skipped `tq show` in three windows while citing
# the convention that mandates it — willpower is not the fix, mechanism is).
#
# Usage: scripts/redispatch-brief.sh <task-id>
#
# Prints, in order:
#   1. tq show <id>          — the queue's own view of the task being
#                              re-dispatched (payload, status, last error)
#   2. newest prior report   — latest close-out docs/status report naming
#                              the task (tasks/ placement, then legacy root,
#                              then archived/)
#   3. git log --oneline -5  — the tree the re-dispatch will build on
#
# A re-dispatch window starts ONLY after this brief has been read end to end.
set -euo pipefail

id="${1:?usage: scripts/redispatch-brief.sh <task-id>}"

tq_bin="${TQ_BIN:-tq}"

echo "== tq show ${id} =="
if ! command -v "${tq_bin}" >/dev/null 2>&1; then
	echo "(tq not on PATH; set TQ_BIN or install it — DO NOT skip this half)"
else
	"${tq_bin}" show "${id}"
fi

echo
echo "== newest prior report for ${id} =="
found=""
for pattern in \
	"docs/status/tasks/*_task-${id}*.md" \
	"docs/status/*_task-${id}*.md" \
	"docs/status/archived/*_task-${id}*.md"; do
	newest=$(ls -t ${pattern} 2>/dev/null | head -1 || true)
	if [ -n "${newest}" ]; then
		found="${newest}"
		break
	fi
done

if [ -n "${found}" ]; then
	echo "${found}"
else
	echo "(no prior report — this is a first dispatch, not a re-dispatch)"
fi

echo
echo "== git log --oneline -5 =="
git log --oneline -5
