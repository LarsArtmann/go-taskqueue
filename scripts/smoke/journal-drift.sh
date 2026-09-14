#!/usr/bin/env bash
# Journal-drift audit smoke (ADVISORY — never a hard gate on task state,
# 2026-09-14 O5 ruling): the REAL binary runs `tq audit --journal` over a
# hermetic scratch DB — once against a truthful projection (no drift) and
# once against a seeded-drift fixture (the tasks table corrupted directly,
# facts stay truthful). Production journals are never touched.
#
# Assertions:
#   1. truthful projection → exit 0 + "no drift"
#   2. seeded drift (status/attempts/priority/dedup_key corrupted) → exit 0
#      + drift lines naming the seeded mismatch
#   3. `--json` reports the seeded drift machine-readably
set -uo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
WORK="$(mktemp -d)"
TQ="${TQ_BIN:-$WORK/tq}"
KEEP="${SMOKE_KEEP:-0}"
cleanup() { [ "$KEEP" = 1 ] && echo "workdir kept: $WORK" || rm -rf "$WORK"; }
trap cleanup EXIT

if [ -z "${TQ_BIN:-}" ]; then
	echo "== building tq =="
	"$REPO_ROOT/scripts/build-tq.sh" "$TQ" || exit 1
fi

DB="$WORK/q.db"
fail() { echo "FAIL: $*" >&2; exit 1; }

echo "== phase 1: full lifecycle, truthful projection =="
"$TQ" enqueue --db "$DB" --type sh '"true"' >/dev/null || fail "enqueue #1"
"$TQ" enqueue --db "$DB" --type sh '"true"' >/dev/null || fail "enqueue #2"
"$TQ" worker --db "$DB" --once >/dev/null || fail "worker --once"

OUT="$("$TQ" audit --journal --db "$DB")" || fail "audit --journal (truthful) exited non-zero"
echo "$OUT" | grep -q "no drift" || fail "truthful projection reported drift: $OUT"
echo "ok: no drift over the lifecycle"

echo "== phase 2: seeded drift (projection corrupted, facts truthful) =="
python3 - "$DB" <<'PY' || fail "python drift seeding"
import sqlite3, sys
db = sqlite3.connect(sys.argv[1])
for (task_id,) in db.execute("SELECT id FROM tasks").fetchall():
    db.execute("UPDATE tasks SET status = 'completed', attempts = 99, priority = 42, dedup_key = ? WHERE id = ?", ("seeded-" + task_id, task_id))
db.commit()
PY

OUT="$("$TQ" audit --journal --db "$DB")" || fail "audit --journal (drift) exited non-zero"
for needle in "status" "attempts" "priority" "dedup"; do
	echo "$OUT" | grep -qi "$needle" || fail "drift report missing '$needle': $OUT"
done
echo "ok: seeded drift detected across all four diffed fields"

echo "== phase 3: --json drift output =="
"$TQ" audit --journal --db "$DB" --json | grep -q '"drift"' || fail "json output lacks drift field"
echo "ok: json drift output"

echo "PASS: journal-drift audit smoke"
