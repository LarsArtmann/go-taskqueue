#!/usr/bin/env bash
# Status-index check: every docs/status/*.md report must be indexed in
# docs/status/README.md. 5 of 2026-09-08's 8 reports went unindexed within a
# single day (21:40 report §d6) — the index is the only discovery surface for
# point-in-time reports, so an unindexed report is effectively lost.
set -uo pipefail
cd "$(dirname "$0")/.." || exit 1

# Index-scannability cadence (TODO row 166, 08-25 f20 / 07-49 f39): the live
# index grows ~10 rows/day; past 100 rows it no longer fits one screen, so
# the nudge is an archive sweep (docs-health ANNOTATE) or a monthly digest
# row. 100 is the owner-ratified default (02-24 f41), not data-derived —
# re-derive only with a measured rows/day cliff.
readonly INDEX_BLOAT_THRESHOLD=100

# Row classification contract (pinned by --self-test):
#   live     = row starts `| 20` (a report row) that is neither archived nor a digest
#   archived = row carries a BACKTICKED `archived/…` path — the backticks are
#              the convention (README cadence paragraph): an unbackticked
#              archive row counts as LIVE forever, which is the point — it
#              surfaces as bloat instead of silently vanishing from the count
#   digest   = row starts `| digest 20` (monthly aggregation, format proposed
#              in docs/status/README.md). The prefix is deliberately NOT a
#              date, so a digest row can never inflate the live count —
#              digest-awareness folded in before the first digest lands (02-24 e3)
count_live_rows() {
	local index="$1" total archived
	total=$(grep -cE '^\| 20[0-9]{2}-' "$index" || true)
	archived=$(grep -cE '^\| 20[0-9]{2}-.*`archived/' "$index" || true)
	echo $((total - archived))
}

# --self-test: pin the live-row counter against a fixture index (live,
# archived with and without backticks, digest, malformed) instead of the real
# index — a counter regression must fail here, not silently mis-warn (02-24 e2).
if [ "${1:-}" = "--self-test" ]; then
	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' EXIT
	cat >"$tmp/index.md" <<'EOF'
| DATE | REPORT | NOTES |
|---|---|---|
| 2026-09-14_10-00 | [a](a.md) | live row |
| 2026-09-14_11-00 | [b](b.md) | live row |
| 2026-09-14_12-00 | [c](c.md) | live row |
| 2026-09-13_09-00 | [d](`archived/d.md`) | archived, backticked — not live |
| 2026-09-13_10-00 | [e](`archived/e.md`) | archived, backticked — not live |
| 2026-09-13_11-00 | [f](archived/f.md) | archived WITHOUT backticks — convention bug, counts live |
| digest 2026-08 | 44 reports archived (scopes: executor 12, gates 9, webui 7) | digest row — never counts live |
| see 2026-09-01 report | not a table row | malformed — the date-row regex skips it |
EOF
	got=$(count_live_rows "$tmp/index.md")
	want=4
	if [ "$got" = "$want" ]; then
		echo "status-index self-test ok (live=$got; digest excluded, unbackticked-archive counts live, malformed skipped)"
		exit 0
	fi
	echo "status-index self-test FAIL: want live=$want, got $got"
	exit 1
fi

index="docs/status/README.md"
[ -f "$index" ] || {
	echo "MISSING: $index does not exist"
	exit 1
}

fail=0
while IFS= read -r report; do
	name="$(basename "$report")"
	if ! grep -qF "$name" "$index"; then
		echo "UNINDEXED: $report (add a row to $index; if a daemon chore-commit folded it in unindexed, amend the row into that commit — AGENTS.md status-reports rule)"
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
	body_date="$(grep -m1 -oE '20[0-9]{2}-[0-9]{2}-[0-9]{2}' "$report" | head -1)"
	if [ -n "$body_date" ] && [ "$body_date" != "$date_part" ]; then
		echo "BODY-DATE DRIFT: $report filename says $date_part but its header says $body_date"
		fail=1
	fi

	# Filename-ID ↔ commit-trailer cross-check (plan Round 11 M42): a
	# task-NNN report promises a queue task; its task ID should appear as a
	# Task-Queue-ID trailer in git history (the f26 three-ID cluster class
	# surfaces mechanically here). WARNING only for now: the one live hit
	# is the f26 cluster itself (07-49 report filename ...c3919ce4 vs
	# trailer ...b141f022) and its canonical ID is an owner-blocked ruling
	# (TODO_LIST "Canonical ID for the f26 cluster") — flip to fail=1 when
	# that ruling lands. Reports without a task-NNN name are exempt.
	if [[ "$name" =~ task-([0-9a-f]{16,}) ]]; then
		task_id="${BASH_REMATCH[1]}"
		if ! git log --grep "Task-Queue-ID: .*${task_id: -12}" --oneline -1 | grep -q .; then
			echo "TRAILER WARNING: $report names task ${task_id:0:16}… but no commit carries its Task-Queue-ID trailer (f26 cluster class; owner ruling pending)"
		fi
	fi
done < <(find docs/status -maxdepth 1 -name '*.md' ! -name 'README.md' | sort)

# Index-scannability cadence (see INDEX_BLOAT_THRESHOLD above for the
# contract): when unarchived rows exceed the threshold, nudge a monthly
# archive sweep into docs/status/archived/ (docs-health ANNOTATE mode) or a
# monthly digest row so the index stays scannable.
live=$(count_live_rows "$index")
if [ "$live" -gt "$INDEX_BLOAT_THRESHOLD" ]; then
	echo "INDEX BLOAT WARNING: $live live rows in $index (threshold $INDEX_BLOAT_THRESHOLD) — run an archive sweep (docs-health ANNOTATE) or add a monthly digest row (08-25 f20)"
fi

if [ "$fail" = 0 ]; then
	echo "status index ok"
fi
exit "$fail"
