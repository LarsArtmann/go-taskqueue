#!/usr/bin/env bash
# Loop-detector smoke (M10/M14): `tq stats` must flag a churning task
# (claim count beyond queue.ClaimAnomalyThreshold = 20) and stay silent on
# a healthy journal. The churn fixture seeds claim facts directly into a
# SCRATCH journal — the journal-drift smoke's seeding pattern — because 30
# claims through the CLI are not constructible without a worker loop.
# Production journals are never touched (own scratch DB, --db everywhere).
#
# Assertions:
#   1. healthy journal → no "loop suspects" line, no claim_anomalies key
#   2. 30-claim fixture → "loop suspects 1" naming the task id, JSON
#      claim_anomalies[0].claims = 30, claim_anomaly_threshold = 20
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

echo "== phase 1: healthy journal — detector silent =="
"$TQ" enqueue --db "$DB" --type sh '"true"' >/dev/null || fail "enqueue"
"$TQ" worker --db "$DB" --once >/dev/null 2>&1 || fail "worker --once"

OUT="$("$TQ" stats --db "$DB")" || fail "stats (healthy) exited non-zero"
echo "$OUT" | grep -q "loop suspects" && fail "healthy journal flagged loop suspects: $OUT"

JSON="$("$TQ" stats --db "$DB" --json)" || fail "stats --json (healthy)"
echo "$JSON" | grep -q "claim_anomalies" && fail "healthy JSON carried claim_anomalies: $JSON"
echo "ok: healthy journal silent"

echo "== phase 2: 30-claim churn fixture — detector red =="
"$TQ" enqueue --db "$DB" --type sh '"sleep 0"' >/dev/null || fail "enqueue churn task"

python3 - "$DB" >/dev/null <<'PY' || fail "seeding claim facts"
import sqlite3, sys, time

db = sqlite3.connect(sys.argv[1])

row = db.execute("SELECT id FROM tasks WHERE status = 'pending' LIMIT 1").fetchone()
if row is None:
    sys.exit("no pending task to churn")

task_id = row[0]
seq = db.execute("SELECT COALESCE(MAX(seq), 0) FROM facts").fetchone()[0]
now = int(time.time() * 1000)

for i in range(30):
    seq += 1
    db.execute(
        "INSERT INTO facts (seq, time, task_id, type, owner, attempt, error, detail) "
        "VALUES (?, ?, ?, 'task.claimed', 'smoke-churn', ?, '', '')",
        (seq, now + i, task_id, i),
    )

db.commit()
print(f"seeded 30 claims on {task_id}")
PY

OUT="$("$TQ" stats --db "$DB")" || fail "stats (churn) exited non-zero"
echo "$OUT" | grep -Eq "loop suspects +1 " || fail "churn fixture not flagged: $OUT"
ID="$(python3 - "$DB" <<'PY'
import sqlite3, sys
db = sqlite3.connect(sys.argv[1])
print(db.execute("SELECT id FROM tasks WHERE status = 'pending' LIMIT 1").fetchone()[0])
PY
)"
echo "$OUT" | grep -q "$ID" || fail "suspect list does not name the churning task: $OUT"

JSON="$("$TQ" stats --db "$DB" --json)" || fail "stats --json (churn)"
echo "$JSON" | grep -q '"claims": 30' || fail "JSON claims count wrong: $JSON"
echo "$JSON" | grep -q '"claim_anomaly_threshold": 20' || fail "JSON threshold wrong: $JSON"
echo "ok: churn fixture flagged ($ID, 30 claims)"

echo "PASS: loop detector red on churn, silent on healthy"
