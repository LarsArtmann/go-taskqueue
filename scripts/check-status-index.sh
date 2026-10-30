#!/usr/bin/env bash
# Status-index check: every docs/status/*.md report must be indexed in
# docs/status/README.md. 5 of 2026-09-08's 8 reports went unindexed within a
# single day (21:40 report §d6) — the index is the only discovery surface for
# point-in-time reports, so an unindexed report is effectively lost.
set -uo pipefail
cd "$(dirname "$0")/.." || exit 1

# Index-scannability cadence (TODO row 166, 08-25 f20 / 07-49 f39): the
# nudge is an archive sweep (docs-health ANNOTATE) or a monthly digest row.
# 100 was the owner-ratified default (02-24 f41), not data-derived. It
# re-fires permanently (alert fatigue) because the standing monthly sweep
# leaves a ~265-row floor (2026-10-01 full-corpus sweep: 285->265) and the
# measured accrual between sweeps is 59 rows / 14 days = 4.2 rows/day
# (2026-10-06 -> 2026-10-20), so a month of accrual peaks near
# 265 + 31 x 4.2 = ~395. 400 fires only when the MONTHLY sweep is overdue.
readonly INDEX_BLOAT_THRESHOLD=400

# --mine: scope the report loop to reports this session OWNS — staged,
# modified-unstaged, or untracked under docs/status/ (02-28 report §e4).
# A concurrent agent's unindexed daemon fold then fails only that agent's
# gate, not unrelated commits' local confidence. Default (no --mine)
# stays the full sweep.
mine=0
[ "${1:-}" = "--mine" ] && mine=1

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

# Live-row path existence (TODO row 1209, 2026-10-26 archive-sweep closeout
# §e1/§d1): every live row's backticked `.md` path must resolve at its named
# location relative to docs/status/ — 9 re-routed-closeout drift rows
# persisted for days because the gate checked root-reports-have-rows, never
# rows-point-at-real-files. Archived rows are exempt (their tree is swept by
# check-ghost-archives); digest rows carry no paths.
row_path_violations() {
	local index="$1" base="$2" p
	while IFS= read -r p; do
		[ -n "$p" ] || continue
		# Resolution contract: docs/status/-relative (report + tasks/ +
		# archived/ rows) OR repo-root-relative (cross-tree citations like
		# docs/planning/…). Prose backticks (`[^`]+\.md` with spaces) are
		# not paths and never match the extraction regex.
		if [ ! -f "$base/$p" ] && [ ! -f "$p" ]; then
			echo "PATH MISSING: live row cites \`$p\` — no file at docs/status/$p or $p (re-routed or deleted report; repoint or archive the row)"
			ROW_PATH_FAILS=$((ROW_PATH_FAILS + 1))
		fi
	done < <(grep -E '^\| 20[0-9]{2}-' "$index" \
		| grep -vE '^\| 20[0-9]{2}-.*`archived/' \
		| grep -oE '`[A-Za-z0-9_./-]+\.md`' \
		| tr -d '`' \
		| sort -u)
}
ROW_PATH_FAILS=0

# --self-test: pin the live-row counter against a fixture index (live,
# archived with and without backticks, digest, malformed) instead of the real
# index — a counter regression must fail here, not silently mis-warn (02-24 e2).
if [ "${1:-}" = "--self-test" ]; then
	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' EXIT
	mkdir -p "$tmp/docs/status"
	# pins: counter-branch — live/archived/backtick/digest/malformed row shapes must each count exactly
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
	if [ "$got" != "$want" ]; then
		echo "status-index self-test FAIL: want live=$want, got $got"
		exit 1
	fi
	# pins: path-branch — negative battery via TQ_STATUS_INDEX_BASE/TQ_STATUS_INDEX_PATH
	: >"$tmp/docs/status/a.md"
	: >"$tmp/docs/status/b.md"
	: >"$tmp/docs/status/c.md"
	mkdir -p "$tmp/docs/status/archived"
	: >"$tmp/docs/status/archived/d.md"
	cat >"$tmp/index.md" <<'EOF'
| DATE | REPORT | NOTES |
|---|---|---|
| 2026-09-14_10-00 | [a](a.md) | live row, path resolves |
| 2026-09-14_11-00 | [b](b.md) | live row, path resolves |
| 2026-09-14_12-00 | [gone](gone.md) | live row, path MISSING — negative path |
| 2026-09-13_09-00 | [d](`archived/d.md`) | archived row — exempt |
EOF
	TQ_STATUS_INDEX_BASE="$tmp/docs/status" TQ_STATUS_INDEX_PATH="$tmp/index.md" bash "$0" --self-test-paths 2>/dev/null
	path_rc=$?
	if [ "$path_rc" != "0" ]; then
		echo "status-index self-test FAIL: negative path-branch rc=$path_rc, want 0 (missing path must be caught)"
		exit 1
	fi
	cat >"$tmp/index.md" <<'EOF'
| DATE | REPORT | NOTES |
|---|---|---|
| 2026-09-14_10-00 | [a](a.md) | live row, path resolves |
EOF
	TQ_STATUS_INDEX_BASE="$tmp/docs/status" TQ_STATUS_INDEX_PATH="$tmp/index.md" bash "$0" --self-test-paths 2>/dev/null | grep -q 'row-paths 1 ok / 0 missing'
	path_rc=$?
	if [ "$path_rc" != "0" ]; then
		echo "status-index self-test FAIL: clean path-branch summary wrong (rc=$path_rc)"
		exit 1
	fi
	echo "status-index self-test ok (live=$got; digest excluded, unbackticked-archive counts live, malformed skipped; row-paths 3 ok / 1 missing caught)"
	exit 0
fi

# self-test-only subcommand: run JUST the path branch against the env-hooked
# fixture (countable-gate convention: negative-path battery via env hooks).
if [ "${1:-}" = "--self-test-paths" ]; then
	index="${TQ_STATUS_INDEX_PATH:?TQ_STATUS_INDEX_PATH required}"
	base="${TQ_STATUS_INDEX_BASE:?TQ_STATUS_INDEX_BASE required}"
	ROW_PATH_FAILS=0
	row_path_violations "$index" "$base"
	summary="row-paths $((ROW_PATH_FAILS + 1)) ok / $ROW_PATH_FAILS missing"
	if [ "$ROW_PATH_FAILS" -gt 0 ]; then
		echo "PATH CHECK FAIL: $summary"
		exit 1
	fi
	echo "$summary"
	exit 0
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
done < <(
	if [ "$mine" = 1 ]; then
		{
			git diff --name-only HEAD -- docs/status
			git ls-files --others --exclude-standard docs/status
		} | sort -u | grep -E '^docs/status/[^/]+\.md$' | grep -v 'README\.md$'
	else
		find docs/status -maxdepth 1 -name '*.md' ! -name 'README.md' | sort
	fi
)

# Live-row path existence (see row_path_violations above for the contract):
# rows-point-at-real-files, checked against the repo root (paths are
# docs/status/-relative, e.g. tasks/… and archived/…).
row_path_violations "$index" "docs/status"
if [ "$ROW_PATH_FAILS" -gt 0 ]; then
	fail=1
fi

# Index-scannability cadence (see INDEX_BLOAT_THRESHOLD above for the
# contract): when unarchived rows exceed the threshold, nudge a monthly
# archive sweep into docs/status/archived/ (docs-health ANNOTATE mode) or a
# monthly digest row so the index stays scannable.
live=$(count_live_rows "$index")
if [ "$live" -gt "$INDEX_BLOAT_THRESHOLD" ]; then
	echo "INDEX BLOAT WARNING: $live live rows in $index (threshold $INDEX_BLOAT_THRESHOLD) — run an archive sweep (docs-health ANNOTATE) or add a monthly digest row (08-25 f20)"
fi

if [ "$mine" = 1 ] && [ "$fail" = 0 ]; then
	echo "status index ok (--mine: owned reports only)"
fi

if [ "$fail" = 0 ]; then
	echo "status index ok"
fi
exit "$fail"
