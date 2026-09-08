#!/usr/bin/env bash
# End-to-end verification of the read-only live web UI (no browser needed):
# enqueue tasks, run a worker and `tq serve` side by side, then assert the
# dashboard page, /api/stats and the SSE stream all reflect live state.
set -euo pipefail
cd "$(dirname "$0")/../.."

TMP="$(mktemp -d)"
trap 'kill "${WORKER_PID:-0}" "${SERVE_PID:-0}" "${AUTH_SERVE_PID:-0}" 2>/dev/null || true; rm -rf "$TMP"' EXIT

# Ask the kernel for a free ephemeral port. WEBUI_SMOKE_PORT still pins an
# explicit port; without it a fixed port collides on busy machines.
free_port() {
	python3 - <<'PY'
import socket
with socket.socket() as s:
    s.bind(("127.0.0.1", 0))
    print(s.getsockname()[1])
PY
}

# TQ_BIN points at a prebuilt binary (e.g. the nix-built result/bin/tq);
# unset, the script builds from source with `go build`.
if [ -n "${TQ_BIN:-}" ]; then
	echo "== using prebuilt tq: $TQ_BIN"
	cp "$TQ_BIN" "$TMP/tq"
	chmod +x "$TMP/tq"
else
	echo "== build tq"
	go build -o "$TMP/tq" ./cmd/tq
fi

echo "== seed tasks"
export TQ_DB="$TMP/tasks.db"
"$TMP/tq" enqueue --type sh --project smoke --payload 'echo one'
"$TMP/tq" enqueue --type sh --project smoke --payload 'sleep 1 && echo two'
"$TMP/tq" enqueue --type sh --project smoke --payload 'exit 3' --max-attempts 1

echo "== start worker + serve"
# Pick the port here, not at the top: the closer to the bind, the smaller the
# chance another process grabs the ephemeral port in between.
PORT="${WEBUI_SMOKE_PORT:-$(free_port)}"
echo "== serve on 127.0.0.1:$PORT"
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
    page_headers = dict(r.headers)
csp = page_headers.get("Content-Security-Policy", "")
for frag in ("frag-stats", "frag-table", "frag-dlq", "frag-feed"):
    assert frag in page, f"page missing {frag}"
assert "default-src 'none'" in csp and "script-src 'self'" in csp and "unsafe-inline" not in csp, (
    f"page CSP wrong: {csp}"
)
assert page_headers.get("X-Content-Type-Options") == "nosniff", "missing nosniff"
assert page_headers.get("Referrer-Policy") == "no-referrer", "missing referrer policy"
print("page fragments + security headers OK")

with urllib.request.urlopen(f"{base}/?view=board", timeout=2) as r:
    board = r.read().decode()
for col in ("pending", "running", "completed", "dead", "cancelled"):
    assert f'data-status="{col}"' in board, f"board missing {col} column"
assert 'aria-current="true"' in board, "board page does not mark the board toggle active"
print("board view OK")

req = urllib.request.Request(f"{base}/api/events", headers={"Accept": "text/event-stream"})
with urllib.request.urlopen(req, timeout=3) as r:
    stream = r.read(2048).decode(errors="replace")
assert "event: frag" in stream, "SSE stream missing frag events"
print("SSE stream OK")

import re
task_id = re.search(r'href="/task/([^"]+)"', page).group(1)

with urllib.request.urlopen(f"{base}/task/{task_id}", timeout=2) as r:
    detail = r.read().decode()
for frag in ("frag-detail", "frag-timeline"):
    assert frag in detail, f"detail page missing {frag}"

req = urllib.request.Request(f"{base}/task/{task_id}/events", headers={"Accept": "text/event-stream"})
buf = ""
with urllib.request.urlopen(req, timeout=3) as r:
    deadline = time.time() + 5
    while time.time() < deadline and ("frag-detail" not in buf or "frag-timeline" not in buf):
        try:
            line = r.readline()
        except TimeoutError:
            break
        if not line:
            break
        buf += line.decode(errors="replace")
assert "frag-detail" in buf and "frag-timeline" in buf, (
    f"task SSE stream missing detail fragments; got: {buf[:200]}"
)
print("task detail page + SSE stream OK")
PYEOF

echo "== auth smoke: non-loopback bind refuses without a token"
REFUSE_PORT="$(free_port)"
if "$TMP/tq" serve --addr "0.0.0.0:$REFUSE_PORT" >"$TMP/refuse.log" 2>&1; then
	echo "FAIL: serve accepted 0.0.0.0 bind without --auth-token"
	cat "$TMP/refuse.log"
	exit 1
fi
grep -q "refusing to serve on non-loopback" "$TMP/refuse.log" || {
	echo "FAIL: refusal message missing from log:"
	cat "$TMP/refuse.log"
	exit 1
}
echo "refusal OK"

echo "== auth smoke: token-authenticated serve"
AUTH_PORT="$(free_port)"
"$TMP/tq" serve --addr "127.0.0.1:$AUTH_PORT" --auth-token smoke-secret --poll 100ms >"$TMP/auth-serve.log" 2>&1 &
AUTH_SERVE_PID=$!

for _ in $(seq 1 50); do
	if grep -q "dashboard on" "$TMP/auth-serve.log" 2>/dev/null; then
		break
	fi

	sleep 0.1
done

python3 - "$AUTH_PORT" <<'PYEOF'
import sys, urllib.error, urllib.request

base = f"http://127.0.0.1:{sys.argv[1]}"

def status(url, headers=None):
    req = urllib.request.Request(url, headers=headers or {})
    try:
        with urllib.request.urlopen(req, timeout=2) as r:
            return r.status, r.headers, r.read()
    except urllib.error.HTTPError as e:
        return e.code, e.headers, e.read()

code, headers, _ = status(f"{base}/api/stats")
assert code == 401, f"no-token stats: {code}, want 401"
assert "Bearer" in headers.get("WWW-Authenticate", ""), "401 lacks Bearer challenge"

code, _, _ = status(f"{base}/api/stats", {"Authorization": "Bearer wrong"})
assert code == 401, f"wrong-token stats: {code}, want 401"

code, _, body = status(f"{base}/api/stats", {"Authorization": "Bearer smoke-secret"})
assert code == 200, f"bearer stats: {code}, want 200"

code, _, body = status(f"{base}/?token=smoke-secret")
assert code == 200 and b"frag-stats" in body, f"token-query page: {code}, want 200 with fragments"

code, _, _ = status(f"{base}/static/app.js")
assert code == 401, f"no-token static: {code}, want 401"

print("auth assertions OK (401 challenge, Bearer + ?token= accepted, static guarded)")
PYEOF

echo "== smoke passed"
