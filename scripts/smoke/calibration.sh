#!/usr/bin/env bash
# Calibration live-path smoke (TODO row 1677): one stub agent completes a
# task end-to-end and the derived-outcome CalibrationRow must land in
# $TQ_LOG_DIR/calibration.jsonl — closes the observe-first gap for the
# write path (unit tests pin the record site; this pins the live pool).
#
# Asserts the row's join keys against the derived outcome: kind=agent,
# task_id matches the completed task, the RAW verdict body recorded
# verbatim, commit_count/commit_sha/file_count derived from the footer
# commit, and NO failure half (absence = a completion row).
#
# Scratch DB everywhere (production-journal trap guard); scratch repo; no
# network, no API spend. One --once drain.
set -euo pipefail
cd "$(dirname "$0")/../.."
REPO_ROOT="$(pwd)"

TMP="$(mktemp -d)"
cleanup() {
	# TQ_SMOKE_KEEP=1 keeps the scratch tree for forensics on a failure.
	if [ "${TQ_SMOKE_KEEP:-0}" = "1" ]; then
		echo "== TQ_SMOKE_KEEP=1: scratch tree left at $TMP"
		return
	fi
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
export TQ_LOG_DIR="$TMP/logs"
mkdir -p "$TQ_LOG_DIR"

echo "== scratch repo (one completable item)"
REPO="$TMP/repo"
mkdir -p "$REPO"
git -C "$REPO" init -q
git -C "$REPO" config user.email smoke@tq.local
git -C "$REPO" config user.name "tq smoke"
printf '%s\n' \
	'- [ ] stub calibration-ok item. Commit with footer: Task-Queue-ID: {{TASK_ID}}' \
	>"$REPO/TODO_LIST.md"
printf 'test -f work.txt\n' >"$REPO/.tq-verify"
git -C "$REPO" add -A
git -C "$REPO" commit -qm "seed"

echo "== stub agent: completes the turn, records a verdict"
STUB="$TMP/stub-agent"
cat >"$STUB" <<EOF
#!/bin/sh
prompt=""
prev=""
for a in "\$@"; do
	[ "\$prev" = "--" ] && prompt="\$a"
	prev="\$a"
done
qid="\$(printf '%s\n' "\$prompt" | grep -o 'Task-Queue-ID: [0-9a-f]*' | head -1 | cut -d' ' -f2)"
echo "calibration stub at work"
echo done >work.txt
sed -i 's/^- \[ \] stub calibration-ok item.*/- [x] stub calibration-ok item/' TODO_LIST.md
git add -A
git commit -qm "stub calibration work" -m "Task-Queue-ID: \$qid"
printf '%s\n' '{"summary":"calibration smoke ok"}' > "\$TQ_RESULT_FILE"
EOF
chmod +x "$STUB"
export TQ_AGENT_BIN="$STUB"

cd "$TMP"

echo "== one --once drain: complete the turn"
timeout 120 "$TMP/tq" agent-pool --projects-dir "$TMP" --repos repo \
	--batch-items 1 --once --max-per-tick 1 --poll 50ms --task-timeout 30s \
	>"$TMP/pool.log" 2>&1 || {
	echo "FAIL: pool drain exited nonzero"
	cat "$TMP/pool.log"
	exit 1
}
sleep 5

echo "== assert the turn completed"
"$TMP/tq" stats | tee "$TMP/stats.out"
grep -Eq '^completed\s+1$' "$TMP/stats.out" || {
	echo "FAIL: the turn did not complete"
	exit 1
}

echo "== assert the CalibrationRow landed with the derived outcome"
DATASET="$TQ_LOG_DIR/calibration.jsonl"
[ -f "$DATASET" ] || {
	echo "FAIL: $DATASET missing (no row written for a completed run)"
	exit 1
}
ROWS="$(wc -l <"$DATASET")"
[ "$ROWS" -eq 1 ] || {
	echo "FAIL: wanted exactly 1 calibration row, got $ROWS"
	cat "$DATASET"
	exit 1
}
TASK_ID="$("$TMP/tq" tasks -json -limit 10 | python3 -c '
import json, sys
tasks = json.load(sys.stdin)
if isinstance(tasks, dict):
    tasks = tasks.get("tasks", [])
agent = [t for t in tasks if t.get("type") == "agent"]
if len(agent) != 1:
    sys.exit("wanted one agent task, got %d" % len(agent))
print(agent[0]["id"])
')"
python3 - "$DATASET" "$TASK_ID" <<'PY'
import json, sys
row = json.loads(open(sys.argv[1]).read().strip())
task_id = sys.argv[2]
checks = [
    ("kind", row.get("kind") == "agent"),
    ("task_id", row.get("task_id") == task_id),
    ("raw verdict body", (row.get("verdict") or {}).get("summary") == "calibration smoke ok"),
    ("commit_count=1", row.get("commit_count") == 1),
    ("file_count=2", row.get("file_count") == 2),
    ("verify receipt + attestation", row.get("verify", {}).get("rc") == 0 and row.get("attestation", {}).get("ok") is True),
    ("commit_sha", bool(row.get("commit_sha"))),
    ("no failure half", "failure" not in row),
]
bad = [name for name, ok in checks if not ok]
if bad:
    sys.exit("FAIL: calibration row check(s) %s\nrow: %s" % (bad, row))
print("row ok:", {k: row.get(k) for k in ("kind", "task_id", "commit_count", "file_count")})
PY

echo "PASS: calibration smoke (one row, kind+task_id+verdict+derived outcome pinned, failure half absent)"
