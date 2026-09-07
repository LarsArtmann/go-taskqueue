#!/usr/bin/env bash
# End-to-end verification of the read-only live web UI (no browser needed):
# enqueue tasks, run a worker and `tq serve` side by side, then assert the
# dashboard page, /api/stats and the SSE stream all reflect live state.
set -euo pipefail
cd "$(dirname "$0")/../.."

TMP="$(mktemp -d)"
PORT="${WEBUI_SMOKE_PORT:-8095}"
trap 'kill "${WORKER_PID:-0}" "${SERVE_PID:-0}" 2>/dev/null || true; rm -rf "$TMP"' EXIT

echo "== build tq"
go build -o "$TMP/tq" ./cmd/tq

echo "== seed tasks"
export TQ_DB="$TMP/tasks.db"
"$TMP/tq" enqueue --type sh --project smoke --payload 'echo one'
"$TMP/tq" enqueue --type sh --project smoke --payload 'sleep 1 && echo two'
"$TMP/tq" enqueue --type sh --project smoke --payload 'exit 3' --max-attempts 1

echo "== start worker + serve"
"$TMP/tq" worker --poll 100ms >"$TMP/worker.log" 2>&1 &
WORKER_PID=$!
"$TMP/tq" serve --addr "127.0.0.1:$PORT" --poll 100ms >"$TMP/serve.log" 2>&1 &
SERVE_PID=$!

for _ in $(seq 1 50); do
	if grep -q "dashboard on" "$TMP/serve.log" 2>/dev/null; then
		break
	fi

	sleep 0.1
done

echo "== assert /api/stats"
"$TMP/tq" >/dev/null 2>&1 || true
sleep 2

python3 - "$PORT" <<'PYEOF'
import json, sys, time, urllib.request

port = sys.argv[1]
base = f"http://127.0.0.1:{port}"

stats = None
for _ in range(30):
    try:
        with urllib.request.urlopen(f"{base}/api/stats", timeout=2) as r:
            stats = json.load(r)
        if stats.get("completed") == 2 and stats.get("dead") == 1:
            break
    except Exception:
        pass
    time.sleep(0.2)
else:
    print(f"FAIL: stats never reached completed=2 dead=1 (last: {stats})")
    sys.exit(1)
print(f"stats OK: {stats}")

with urllib.request.urlopen(f"{base}/", timeout=2) as r:
    page = r.read().decode()
for frag in ("frag-stats", "frag-table", "frag-dlq", "frag-feed"):
    assert frag in page, f"page missing {frag}"
print("page fragments OK")

req = urllib.request.Request(f"{base}/api/events", headers={"Accept": "text/event-stream"})
with urllib.request.urlopen(req, timeout=3) as r:
    stream = r.read(2048).decode(errors="replace")
assert "event: frag" in stream, "SSE stream missing frag events"
print("SSE stream OK")
PYEOF

echo "== smoke passed"
