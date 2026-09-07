#!/usr/bin/env bash
# End-to-end verification of the PapDashboard bridge (no docker needed):
# a stub dashboard receives alert.triggered when a task dead-letters and
# alert.resolved after `tq dlq --rescue` lets the task complete.
#
# Against a REAL dashboard: PAP_URL=https://pap.example ./scripts/smoke/papdashboard-e2e.sh
# (real mode verifies against the bridge worker log, which records every
# accepted ingest POST; then optionally check the dashboard UI too).
set -euo pipefail
cd "$(dirname "$0")/../.."

PAP_URL="${PAP_URL:-}"
REAL_MODE=""
[ -n "$PAP_URL" ] && REAL_MODE=1
TMP="$(mktemp -d)"
trap 'for pid in "${WORKER_PID:-}" "${STUB_PID:-}"; do [ -n "$pid" ] && kill "$pid" 2>/dev/null; done; rm -rf "$TMP"' EXIT

echo "== build tq"
go build -o "$TMP/tq" ./cmd/tq

INGEST_LOG="$TMP/ingest.log"
if [ -z "$PAP_URL" ]; then
	echo "== start stub dashboard (records ingest POSTs)"
	cat >"$TMP/stub.py" <<'PYEOF'
import sys
from http.server import BaseHTTPRequestHandler, HTTPServer
LOG, PORT = sys.argv[1], int(sys.argv[2])
class H(BaseHTTPRequestHandler):
    def do_POST(self):
        n = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(n).decode()
        with open(LOG, "a") as f:
            f.write(self.path + " " + self.headers.get("Idempotency-Key", "") + " " + body + "\n")
        self.send_response(200); self.end_headers(); self.wfile.write(b"ok")
    def log_message(self, *a): pass
HTTPServer(("127.0.0.1", PORT), H).serve_forever()
PYEOF
	python3 "$TMP/stub.py" "$INGEST_LOG" 18099 &
	STUB_PID=$!
	PAP_URL="http://127.0.0.1:18099"
	sleep 0.5
fi

export TQ_DB="$TMP/tasks.db"
FLAG="$TMP/flaky.flag"

echo "== start worker with alert bridge -> $PAP_URL"
"$TMP/tq" worker --alert-url "$PAP_URL" --alert-api-key smoke-key --alert-poll 1s \
	>"$TMP/worker.log" 2>&1 &
WORKER_PID=$!

echo "== enqueue a task that fails once, succeeds after rescue"
TASK_ID="$("$TMP/tq" enqueue --type sh --project pap-e2e \
	--payload "test -f '$FLAG' || { touch '$FLAG'; exit 1; }")"
echo "   task $TASK_ID"

# Real mode has no stub log; the bridge logs each accepted ingest POST
# ("papdashboard ingest accepted"), so assert on the worker log there.
if [ -n "$REAL_MODE" ]; then
	ALERT_LOG="$TMP/worker.log"
else
	ALERT_LOG="$INGEST_LOG"
fi

wait_for() { # wait_for <grep-pattern>
	for _ in $(seq 1 15); do
		[ -f "$ALERT_LOG" ] && grep -q "$1" "$ALERT_LOG" && return 0
		sleep 1
	done
	return 1
}

echo "== wait for alert.triggered (worker dead-letters the first attempt)"
if [ -n "$REAL_MODE" ]; then
	wait_for 'ingest accepted.*alert\.triggered' || {
		echo "FAIL: no alert.triggered accepted by dashboard"
		cat "$ALERT_LOG"
		exit 1
	}
else
	wait_for '"type":"alert.triggered"' || {
		echo "FAIL: no alert.triggered"
		exit 1
	}
fi
echo "   triggered OK"

echo "== rescue the dead task; the second run succeeds"
"$TMP/tq" dlq --rescue "$TASK_ID" >/dev/null

echo "== wait for alert.resolved"
if [ -n "$REAL_MODE" ]; then
	wait_for 'ingest accepted.*alert\.resolved' || {
		echo "FAIL: no alert.resolved accepted by dashboard"
		cat "$ALERT_LOG"
		exit 1
	}
else
	wait_for '"type":"alert.resolved"' || {
		echo "FAIL: no alert.resolved"
		cat "$INGEST_LOG"
		exit 1
	}
fi
echo "   resolved OK"

echo "== task state"
"$TMP/tq" show "$TASK_ID" | python3 -c 'import json,sys; d=json.load(sys.stdin); print("   status:", d["task"]["status"])'

echo "PASS: dead letter raised an alert and the rescue resolved it"
