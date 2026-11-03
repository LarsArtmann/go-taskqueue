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
#
# --stamp-residue <task-id> <note> (TODO 02-56 §f8): when a dispatch row's
# premise partially landed BEFORE the session (row 1378's "top cites" were
# already healed by an intervening sweep), stamp a residue note onto the
# row so the next dispatch does not re-chase healed cites. Finds the
# UNCHECKED TODO_LIST.md row containing the task id and appends
# ` — RESIDUE (<date>): <note>` to it. Dry-run-first: without --apply the
# new row is only PRINTED (a wrong filter is free to print, fatal to run);
# `--apply` writes. A row already carrying a RESIDUE note is refused —
# edit manually rather than stacking notes. Target file overridable via
# TQ_TODO_LIST (self-test fixture).
#
# --self-test pins: stamp appends to the matching unchecked row only,
# dry-run writes nothing, already-stamped row refuses, no-matching-row
# fails with guidance.
set -euo pipefail

todo="${TQ_TODO_LIST:-TODO_LIST.md}"

stamp_residue() {
	local sid=$1 note=$2 apply=${3:-}
	local rownum row new
	rownum=$(grep -n '^- \[ \]' "$todo" | grep -F "$sid" | head -1 | cut -d: -f1 || true)
	if [ -z "$rownum" ]; then
		echo "FAIL: no unchecked TODO_LIST row mentions ${sid} — find the batch row manually and append the residue note by hand" >&2
		return 1
	fi
	row=$(sed -n "${rownum}p" "$todo")
	if grep -q 'RESIDUE (' <<<"$row"; then
		echo "FAIL: row ${rownum} already carries a RESIDUE note — edit it manually instead of stacking: ${row}" >&2
		return 1
	fi
	new="${row} — RESIDUE ($(date -u +%F)): ${note}"
	echo "row ${rownum}:"
	echo "$new"
	if [ "$apply" = "--apply" ]; then
		python3 - "$todo" "$rownum" "$new" <<'PY'
import sys
path, num, new = sys.argv[1], int(sys.argv[2]), sys.argv[3]
lines = open(path).read().split('\n')
lines[num - 1] = new
open(path, 'w').write('\n'.join(lines))
PY
		echo "residue note stamped into ${todo}:${rownum}"
	else
		echo "(dry-run — nothing written; re-run with --apply to stamp)"
	fi
}

if [ "${1:-}" = "--self-test" ]; then
	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' EXIT
	cat >"$tmp/TODO_LIST.md" <<'EOF'
- [ ] Unrelated row without the id
- [ ] Batch row for task deadbeefcafe1234 — heal the cites (premise partially landed elsewhere)
- [x] Done row mentioning deadbeefcafe1234
EOF
	st_fail=0
	out="$(TQ_TODO_LIST="$tmp/TODO_LIST.md" bash "$0" --stamp-residue deadbeefcafe1234 'top cites already healed by the 22nd-dispatch sweep')" || st_fail=$((st_fail + 1))
	grep -q '^row 2:$' <<<"$out" || {
		echo "FAIL: dry-run did not target the unchecked batch row: $out"
		st_fail=$((st_fail + 1))
	}
	grep -q 'RESIDUE (' <<<"$out" || {
		echo "FAIL: dry-run did not print the stamped row: $out"
		st_fail=$((st_fail + 1))
	}
	grep -q 'dry-run' <<<"$out" || {
		echo "FAIL: dry-run did not say nothing written: $out"
		st_fail=$((st_fail + 1))
	}
	grep -qE '^- \[ \] Batch row.*RESIDUE \(' "$tmp/TODO_LIST.md" && {
		echo "FAIL: dry-run WROTE the row: $(cat "$tmp/TODO_LIST.md")"
		st_fail=$((st_fail + 1))
	}
	# apply: writes exactly the one row, date-stamped
	TQ_TODO_LIST="$tmp/TODO_LIST.md" bash "$0" --stamp-residue deadbeefcafe1234 'already healed' --apply >/dev/null
	grep -qE '^- \[ \] Batch row for task deadbeefcafe1234 — heal the cites \(premise partially landed elsewhere\) — RESIDUE \([0-9]{4}-[0-9]{2}-[0-9]{2}\): already healed$' "$tmp/TODO_LIST.md" || {
		echo "FAIL: --apply did not stamp the expected row: $(cat "$tmp/TODO_LIST.md")"
		st_fail=$((st_fail + 1))
	}
	# idempotence refusal on a second stamp
	TQ_TODO_LIST="$tmp/TODO_LIST.md" bash "$0" --stamp-residue deadbeefcafe1234 'more' --apply >/dev/null 2>"$tmp/err" && {
		echo "FAIL: second stamp should refuse"
		st_fail=$((st_fail + 1))
	}
	grep -q 'already carries a RESIDUE note' "$tmp/err" || {
		echo "FAIL: refusal message missing: $(cat "$tmp/err")"
		st_fail=$((st_fail + 1))
	}
	# no matching row → guided failure
	TQ_TODO_LIST="$tmp/TODO_LIST.md" bash "$0" --stamp-residue 0000ffff0000ffff 'x' --apply >/dev/null 2>"$tmp/err2" && {
		echo "FAIL: unknown id should fail"
		st_fail=$((st_fail + 1))
	}
	grep -q 'no unchecked TODO_LIST row' "$tmp/err2" || {
		echo "FAIL: no-match guidance missing: $(cat "$tmp/err2")"
		st_fail=$((st_fail + 1))
	}
	# done rows are never stamped even when they carry the id
	if grep -qE '^- \[x\].*RESIDUE' "$tmp/TODO_LIST.md"; then
		echo "FAIL: a done row was stamped"
		st_fail=$((st_fail + 1))
	fi
	if [ "$st_fail" -ne 0 ]; then
		echo "redispatch-brief self-test FAILED ($st_fail cases)"
		exit 1
	fi
	echo "redispatch-brief self-test ok (7 checks: dry-run targets unchecked row + prints + writes nothing, apply stamps expected row, idempotence refusal, no-match guidance, done rows untouched)"
	exit 0
fi

if [ "${1:-}" = "--stamp-residue" ]; then
	shift
	sid="${1:?usage: scripts/redispatch-brief.sh --stamp-residue <task-id> <note> [--apply]}"
	note="${2:?usage: scripts/redispatch-brief.sh --stamp-residue <task-id> <note> [--apply]}"
	apply="${3:-}"
	stamp_residue "$sid" "$note" "$apply"
	exit $?
fi

id="${1:?usage: scripts/redispatch-brief.sh <task-id> | --stamp-residue <task-id> <note> [--apply]}"

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
