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
#   4. positive-path SECRET EVIDENCE: a fake token-shaped string seeded
#      into a task.failed fact's error field → the audit prints a SECRET
#      EVIDENCE row labelled `locations=N` (pins the renamed row label
#      end-to-end)
#   5. positive-path SECRET EVIDENCE `--json`: the seeded token shows up
#      machine-readably as a non-empty `secret_evidence` array with
#      `count == 1` on the entry
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
fail() {
	echo "FAIL: $*" >&2
	exit 1
}

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

echo "== phase 4: zero SECRET EVIDENCE over the lifecycle (row 134 fold) =="
# The sh-payload lifecycle stores no tails carrying token shapes, so the
# audit must report zero SECRET EVIDENCE rows here; a hit means the
# redaction pass stopped covering a fact field.
OUT="$("$TQ" audit --journal --db "$DB")" || fail "audit --journal (secret pass) exited non-zero"
echo "$OUT" | grep -q "SECRET EVIDENCE" && fail "audit reports SECRET EVIDENCE over a clean lifecycle: $OUT"
echo "ok: zero SECRET EVIDENCE"

echo "== phase 5: positive-path SECRET EVIDENCE (locations row label) =="
# Seed the shared SMOKETEST fake shape (single definition:
# fakeAuditProbe in internal/executor/redact_test.go, grep-extracted so
# the smoke and the unit table cannot drift) into a task.failed fact's
# error field, then the audit MUST surface it as a SECRET EVIDENCE row
# with `locations=1` — the end-to-end pin of the renamed
# `hits=`→`locations=` CLI surface.
PROBE="$(sed -n 's/^const fakeAuditProbe = "\(.*\)"$/\1/p' "$REPO_ROOT/internal/executor/redact_test.go")"
[ -n "$PROBE" ] || fail "fakeAuditProbe missing from internal/executor/redact_test.go — the shared SECRET EVIDENCE fixture moved; update this extraction"
python3 - "$DB" "$PROBE" <<'PY' || fail "python secret seeding"
import sqlite3, sys
db = sqlite3.connect(sys.argv[1])
probe = sys.argv[2]
row = db.execute("SELECT id FROM tasks LIMIT 1").fetchone()
assert row, "no tasks seeded"
db.execute(
    "INSERT INTO facts (time, task_id, type, error) VALUES (strftime('%s','now'), ?, 'task.failed', ?)",
    (row[0], "boom: " + probe),
)
db.commit()
PY
OUT="$("$TQ" audit --journal --db "$DB")" || fail "audit --journal (secret positive) exited non-zero"
echo "$OUT" | grep -q "SECRET EVIDENCE" || fail "seeded secret not reported: $OUT"
echo "$OUT" | grep -Eq 'task\.failed .*locations=1' || fail "SECRET EVIDENCE row lacks 'locations=1': $OUT"
echo "ok: SECRET EVIDENCE locations=1 row printed"

echo "== phase 6: positive-path SECRET EVIDENCE --json (secret_evidence count) =="
JSON="$($TQ audit --journal --db "$DB" --json)" || fail "audit --journal --json (secret positive) exited non-zero"
python3 -c '
import json, sys
report = json.loads(sys.argv[1])
ev = report.get("secret_evidence")
assert ev, "secret_evidence missing or empty: %r" % (report.get("secret_evidence"),)
assert len(ev) == 1, "expected exactly 1 secret_evidence entry, got %d" % len(ev)
assert ev[0].get("count") == 1, "secret_evidence[0].count = %r, want 1" % (ev[0].get("count"),)
' "$JSON" || fail "json secret_evidence assertion"
echo "ok: json secret_evidence non-empty with count=1"

echo "PASS: journal-drift audit smoke"
