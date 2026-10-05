#!/usr/bin/env bash
# Dogfood dry-run for the ADR-0019 endgame serve path: a LEGACY pre-flip
# journal (hand-rolled DDL) opened by `tq serve` must transparently
# auto-upgrade (P1), serve reads from the default-ON readmodel projection
# (S3/S4 composition root), and shut down through the GracefulClose
# ordering (store.Close + sys.GracefulClose) with the journal intact.
#
# Asserts, in order:
#   1. serve comes up and /api/stats reflects the migrated legacy task
#   2. exactly ONE <db>.legacy-*.bak snapshot exists (auto-upgrade ran,
#      re-opens do not re-upgrade) and the migration logged to stderr
#   3. <db>.readmodel.db exists (projection home materialized, default ON)
#   4. SIGTERM exits 0 (GracefulClose path — no truncation, no wedging)
#   5. post-exit: tq facts still lists the legacy fact, stats re-opens the
#      upgraded DB without minting a second backup
#
# The explicit TQ_DB export is the production-DB trap guard: agent shells
# inherit TQ_DB (the production journal); this smoke must never touch it.
set -euo pipefail
cd "$(dirname "$0")/../.."
REPO_ROOT="$(pwd)"

export GOEXPERIMENT=jsonv2

TMP="$(mktemp -d)"
cleanup() {
	[ -n "${SERVE_PID:-}" ] && kill "$SERVE_PID" 2>/dev/null || true
	rm -rf "$TMP"
}
trap cleanup EXIT

if [ -n "${TQ_BIN:-}" ]; then
	echo "== using prebuilt tq: $TQ_BIN"
	cp "$TQ_BIN" "$TMP/tq"
	chmod +x "$TMP/tq"
else
	echo "== build tq"
	"$REPO_ROOT/scripts/build-tq.sh" "$TMP/tq"
fi

export TQ_DB="$TMP/tasks.db"
SERVE_LOG="$TMP/serve.log"

PORT="$(python3 - <<'PY'
import socket
with socket.socket() as s:
    s.bind(("127.0.0.1", 0))
    print(s.getsockname()[1])
PY
)"

echo "== seed legacy pre-flip journal (hand-rolled DDL, one pending task)"
python3 - "$TQ_DB" <<'PY'
import sqlite3, sys, time

db = sqlite3.connect(sys.argv[1])
db.executescript(
    """
CREATE TABLE tasks (
	id             TEXT PRIMARY KEY,
	project        TEXT NOT NULL DEFAULT '',
	type           TEXT NOT NULL,
	payload        TEXT NOT NULL DEFAULT '',
	deps           TEXT NOT NULL DEFAULT '[]',
	priority       INTEGER NOT NULL DEFAULT 0,
	attempts       INTEGER NOT NULL DEFAULT 0,
	max_attempts   INTEGER NOT NULL DEFAULT 3,
	not_before     INTEGER NOT NULL DEFAULT 0,
	status         TEXT NOT NULL DEFAULT 'pending',
	lease_owner    TEXT NOT NULL DEFAULT '',
	lease_expires  INTEGER,
	last_error     TEXT NOT NULL DEFAULT '',
	created_at     INTEGER NOT NULL,
	updated_at     INTEGER NOT NULL,
	completed_at   INTEGER,
	dedup_key      TEXT NOT NULL DEFAULT ''
);
CREATE TABLE facts (
	seq      INTEGER PRIMARY KEY AUTOINCREMENT,
	time     INTEGER NOT NULL,
	task_id  TEXT NOT NULL,
	type     TEXT NOT NULL,
	owner    TEXT NOT NULL DEFAULT '',
	attempt  INTEGER NOT NULL DEFAULT 0,
	error    TEXT NOT NULL DEFAULT '',
	detail   TEXT NOT NULL DEFAULT ''
);
CREATE TABLE watermarks (
	consumer   TEXT PRIMARY KEY,
	seq        INTEGER NOT NULL,
	updated_at INTEGER NOT NULL
);
"""
)
now = int(time.time() * 1000)
db.execute(
    "INSERT INTO tasks (id, project, type, payload, status, created_at, updated_at)"
    " VALUES ('t-legacy', 'dogfood', 'sh', '\"echo hi\"', 'pending', ?, ?)",
    (now, now),
)
db.execute(
    "INSERT INTO facts (time, task_id, type, detail) VALUES (?, 't-legacy', 'task.enqueued', '{}')",
    (now,),
)
db.commit()
db.close()
print("legacy journal seeded: 1 task, 1 fact")
PY

echo "== tq serve (SIGTERM at the end; GracefulClose ordering)"
"$TMP/tq" serve --addr "127.0.0.1:$PORT" >"$SERVE_LOG" 2>&1 &
SERVE_PID=$!

python3 - "$PORT" "$TQ_DB" <<'PY'
import glob, json, sys, time, urllib.request

port, db = sys.argv[1], sys.argv[2]
base = f"http://127.0.0.1:{port}"

stats = None
for _ in range(50):
    try:
        with urllib.request.urlopen(f"{base}/api/stats", timeout=2) as r:
            stats = json.load(r)
        break
    except Exception:
        time.sleep(0.2)
else:
    print(f"FAIL: /api/stats never answered (last: {stats})")
    sys.exit(1)
if stats.get("pending") != 1 or stats.get("total") != 1:
    print(f"FAIL: stats {stats}, want pending=1 total=1 (the migrated legacy task)")
    sys.exit(1)
print(f"stats OK (readmodel-backed): {stats}")

backups = glob.glob(db + ".legacy-*.bak")
assert len(backups) == 1, f"want exactly one legacy backup, got {backups}"
print(f"auto-upgrade snapshot OK: {backups[0].split('/')[-1]}")

readmodel = db + ".readmodel.db"
import os

assert os.path.exists(readmodel), f"readmodel projection missing: {readmodel}"
print("readmodel projection OK (default ON)")
PY

echo "== SIGTERM -> GracefulClose"
kill -TERM "$SERVE_PID"
wait "$SERVE_PID"
echo "serve exit rc=$? (0 = GracefulClose clean)"

echo "== post-exit journal integrity"
"$TMP/tq" facts | grep -q "task.enqueued"
echo "legacy fact survived: seq 1 task.enqueued"

"$TMP/tq" stats --json | python3 -c '
import json, sys
d = json.load(sys.stdin)
assert d["by_status"].get("pending") == 1, d
assert d["journal_head"] == 1, d
print("stats re-open OK: by_status=%s journal_head=%d" % (d["by_status"], d["journal_head"]))
'
BACKUPS_AFTER="$(ls "$TQ_DB".legacy-*.bak | wc -l)"
[ "$BACKUPS_AFTER" = "1" ]
echo "re-open minted no second backup: still $BACKUPS_AFTER"

grep -q "migration: auto-upgraded legacy database onto the go-cqrs-lite engine" "$SERVE_LOG"
echo "migration log line OK"

echo "PASS: legacy auto-upgrade + readmodel-default serve + GracefulClose"
