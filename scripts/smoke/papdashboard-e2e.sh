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
REPO_ROOT="$(pwd)"

PAP_URL="${PAP_URL:-}"
REAL_MODE=""
[ -n "$PAP_URL" ] && REAL_MODE=1
TMP="$(mktemp -d)"
cleanup() {
	for pid in "${WORKER_PID:-}" "${STUB_PID:-}" "${POOL_PID:-}"; do
		[ -n "$pid" ] && kill "$pid" 2>/dev/null
	done
	rm -rf "$TMP"
}
trap cleanup EXIT

# A kernel-chosen ephemeral port — the stub BINDS port 0 and reports the
# port it actually got via a file, so there is no probe→close→bind race
# window for another listener to steal (the historical fixed-port collision
# class; free_port() had the same TOCTOU, just narrower — the smoke failed
# twice on 2026-10-31 with a live stub but no ingest rows).
# PAP_SMOKE_PORT still pins an explicit port for debugging collisions.
STUB_PORT_FILE="$TMP/stub.port"

# TQ_BIN points at a prebuilt binary (e.g. the nix-built result/bin/tq);
# unset, the script builds from source with `go build`. Either way the
# binary's version + path print first, so a green can never hide WHICH tq
# produced it (the 09-01 green-lie class).
if [ -n "${TQ_BIN:-}" ]; then
	echo "== using prebuilt tq: $TQ_BIN"
	cp "$TQ_BIN" "$TMP/tq"
	chmod +x "$TMP/tq"
else
	echo "== build tq"
	"$REPO_ROOT/scripts/build-tq.sh" "$TMP/tq"
fi
"$TMP/tq" version

INGEST_LOG="$TMP/ingest.log"
if [ -z "$PAP_URL" ]; then
	echo "== start stub dashboard (records ingest POSTs)"
	# pins: stub-dashboard-ingest — stub HTTP server logs dead-letter POSTs
	cat >"$TMP/stub.py" <<'PYEOF'
import sys
from http.server import BaseHTTPRequestHandler, HTTPServer
LOG, PORTFILE = sys.argv[1], sys.argv[2]
want = int(sys.argv[3]) if len(sys.argv) > 3 else 0
class H(BaseHTTPRequestHandler):
    def do_POST(self):
        n = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(n).decode()
        with open(LOG, "a") as f:
            f.write(self.path + " " + self.headers.get("Idempotency-Key", "") + " " + body + "\n")
        self.send_response(200); self.end_headers(); self.wfile.write(b"ok")
    def log_message(self, *a): pass
srv = HTTPServer(("127.0.0.1", want), H)
with open(PORTFILE, "w") as f:
    f.write(str(srv.server_address[1]))
srv.serve_forever()
PYEOF
	python3 "$TMP/stub.py" "$INGEST_LOG" "$STUB_PORT_FILE" "${PAP_SMOKE_PORT:-0}" &
	STUB_PID=$!
	for _ in $(seq 1 20); do
		kill -0 "$STUB_PID" 2>/dev/null || {
			echo "FAIL: stub dashboard exited (PAP_SMOKE_PORT=${PAP_SMOKE_PORT:-unset} taken?)"
			exit 1
		}
		[ -s "$STUB_PORT_FILE" ] && break
		sleep 0.25
	done
	STUB_PORT="$(cat "$STUB_PORT_FILE" 2>/dev/null || true)"
	[ -n "$STUB_PORT" ] || {
		echo "FAIL: stub dashboard never reported its bound port"
		exit 1
	}
	PAP_URL="http://127.0.0.1:$STUB_PORT"
fi

export TQ_DB="$TMP/tasks.db"
FLAG="$TMP/flaky.flag"

# One tq process pays the migrate serially BEFORE the worker and enqueue
# can race each other's open (M25 class — 2026-10-31: the worker lost the
# open-migrate race to the enqueue's first attempt twice; the enqueue leg
# already retried, the worker leg just died, and the smoke asserted
# against a workerless pool for 120s).
"$TMP/tq" watermarks show >/dev/null 2>&1 || true

start_worker() {
	"$TMP/tq" worker --alert-url "$PAP_URL" --alert-api-key smoke-key --alert-poll 1s \
		>"$TMP/worker.log" 2>&1 &
	WORKER_PID=$!
}

echo "== start worker with alert bridge -> $PAP_URL"
start_worker
for _ in 1 2 3 4 5; do
	sleep 1
	if kill -0 "$WORKER_PID" 2>/dev/null; then
		break
	fi
	grep -q 'database is locked' "$TMP/worker.log" 2>/dev/null || break
	echo "WARN: worker lost the open-migrate race — retrying" >&2
	start_worker
done

echo "== enqueue a task that fails once, succeeds after rescue"
# The worker was started first; under gate load its migrate can hold the
# write lock past the 5s busy_timeout (M25 class — isolated runs never
# reproduce). Bounded retry keeps the gate load-robust.
TASK_ID=""
for _ in 1 2 3 4 5; do
	if TASK_ID="$("$TMP/tq" enqueue --type sh --project pap-e2e \
		--payload "test -f '$FLAG' || { touch '$FLAG'; exit 1; }")"; then
		break
	fi
	echo "WARN: transient enqueue failure — retrying" >&2
	sleep 2
done
[ -n "$TASK_ID" ] || {
	echo "FAIL: enqueue kept failing after retries" >&2
	exit 1
}
echo "   task $TASK_ID"

# Real mode has no stub log; the bridge logs each accepted ingest POST
# ("papdashboard ingest accepted"), so assert on the worker log there.
if [ -n "$REAL_MODE" ]; then
	ALERT_LOG="$TMP/worker.log"
else
	ALERT_LOG="$INGEST_LOG"
fi

wait_for() { # wait_for <grep-pattern> [log-file]
	# 120s, not 60s: inside the full ci-local battery the host is busier
	# than during the release gates alone — the 60s window itself was lost
	# once (2026-10-07 final battery, "no alert.triggered" at wait 61)
	# while the isolated run passed in ~3s. The assertion stays strict;
	# the window absorbs battery load.
	local log="${2:-$ALERT_LOG}"
	for _ in $(seq 1 120); do
		[ -f "$log" ] && grep -q "$1" "$log" && return 0
		sleep 1
	done
	echo "WARN: wait_for window exhausted without '$1' in $log — caller decides FAIL or absorb" >&2
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

echo "== agent-pool liveness alerts: stage the B9.3 worker-death marker"
# The durable sidecar the serve/api fold host stamps when its projection
# worker exhausts the restart budget (WorkerFailedMarkerPath =
# <db>.readmodel.db.worker-failed): the pool's workerDeathDetector reads
# it every harvest tick, raises once, and a cleared marker (a fresh host)
# resolves the standing alert.
MARKER="$TQ_DB.readmodel.db.worker-failed"
# failedAt computed at run time — a frozen literal rots silently (the
# ratelimit-e2e frozen-date class; check-frozen-dates.sh enforces).
printf '%s\n' "{\"projection\":\"tasks\",\"failedAt\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"}" >"$MARKER"

echo "== start agent pool with alert bridge -> $PAP_URL"
# Hermetic: an empty projects dir means the harvest scan finds no repos
# (the default would scan the operator's REAL projects dir and enqueue
# real TODO items into the smoke's scratch journal).
mkdir -p "$TMP/pool-projects"
"$TMP/tq" agent-pool --interval 2s --projects-dir "$TMP/pool-projects" \
	--alert-url "$PAP_URL" --alert-api-key smoke-key \
	--alert-poll 1s >"$TMP/pool.log" 2>&1 &
POOL_PID=$!

# Real mode has no stub log; the pool's own bridge logs each accepted
# ingest POST, so assert on the pool log there.
if [ -n "$REAL_MODE" ]; then
	POOL_LOG="$TMP/pool.log"
else
	POOL_LOG="$INGEST_LOG"
fi

echo "== wait for the worker-exhausted alert.triggered"
if [ -n "$REAL_MODE" ]; then
	wait_for 'ingest accepted.*worker-exhausted' "$POOL_LOG" || {
		echo "FAIL: no worker-exhausted alert accepted by dashboard"
		cat "$POOL_LOG"
		exit 1
	}
else
	wait_for 'agent-pool-projection-worker.*alert\.triggered' "$POOL_LOG" || {
		echo "FAIL: no worker-death alert.triggered"
		cat "$TMP/pool.log"
		exit 1
	}
fi
echo "   worker-death triggered OK"

echo "== clear the marker (a fresh host) and expect alert.resolved"
rm "$MARKER"
if [ -n "$REAL_MODE" ]; then
	wait_for 'ingest accepted.*worker-exhausted-resolve' "$POOL_LOG" || {
		echo "FAIL: no worker-exhausted resolve accepted by dashboard"
		cat "$POOL_LOG"
		exit 1
	}
else
	wait_for 'agent-pool-projection-worker.*alert\.resolved' "$POOL_LOG" || {
		echo "FAIL: no worker-death alert.resolved"
		cat "$TMP/pool.log"
		exit 1
	}
fi
echo "   worker-death resolved OK"

kill "$POOL_PID" 2>/dev/null
POOL_PID=""

echo "PASS: dead letter raised an alert, the rescue resolved it, and the agent pool surfaced + cleared a dead projection worker"
