#!/usr/bin/env bash
# Status-index check: every docs/status/*.md report must be indexed in
# docs/status/README.md. 5 of 2026-09-08's 8 reports went unindexed within a
# single day (21:40 report §d6) — the index is the only discovery surface for
# point-in-time reports, so an unindexed report is effectively lost.
set -uo pipefail
cd "$(dirname "$0")/.."

index="docs/status/README.md"
[ -f "$index" ] || {
	echo "MISSING: $index does not exist"
	exit 1
}

fail=0
while IFS= read -r report; do
	name="$(basename "$report")"
	if ! grep -qF "$name" "$index"; then
		echo "UNINDEXED: $report (add a row to $index)"
		fail=1
		continue
	fi

	# DATE-column honesty (round-10 T25): the index row carrying the report
	# must repeat the filename's own date — a wrong DATE column silently
	# mis-sorts the report history.
	date_part="${name%%_*}"
	row="$(grep -F "$name" "$index" | head -1)"
	if ! grep -qF "$date_part" <<<"$row"; then
		echo "DATE MISMATCH: $report indexed with a row that lacks its date $date_part:"
		echo "  $row"
		fail=1
	fi

	# Filename-date vs report-body drift (plan Round 11 M43): the report's
	# own "# Status Report — <date>" / "Written" header line must carry the
	# same date as the filename — a renamed-but-not-redated report reads as
	# a different point in time than it was written at.
	body_date="$(grep -m1 -oE '20[0-9]{2}-[0-9]{2}-[0-9]{2}' "$report" || true)"
	if [ -n "$body_date" ] && [ "$body_date" != "$date_part" ]; then
		echo "BODY-DATE DRIFT: $report filename says $date_part but its header says $body_date"
		fail=1
	fi

	# Filename-ID ↔ commit-trailer cross-check (plan Round 11 M42): a
	# task-NNN report promises a queue task; its task ID must appear as a
	# Task-Queue-ID trailer in git history (the f26 three-ID cluster class
	# surfaces mechanically here). Reports without a task-NNN name are exempt.
	if [[ "$name" =~ task-([0-9a-f]{16,}) ]]; then
		task_id="${BASH_REMATCH[1]}"
		if ! git log --grep "Task-Queue-ID: .*${task_id: -12}" --oneline -1 | grep -q .; then
			echo "TRAILER MISSING: $report names task $task_id but no commit carries its Task-Queue-ID trailer"
			fail=1
		fi
	fi
done < <(find docs/status -maxdepth 1 -name '*.md' ! -name 'README.md' | sort)

if [ "$fail" = 0 ]; then
	echo "status index ok"
fi
exit "$fail"
