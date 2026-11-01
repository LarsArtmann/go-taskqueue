#!/usr/bin/env bash
# Pool-side companion to crush-server-route.sh (TODO row 1450's 429 leg +
# row 1449's keeper/supervisor, live): the wire legs live in the sibling
# smoke; this one pins the two E-gap proofs that need REAL components —
#
#   429 fast-refuse (routed) — MOCK_429 provider + CRUSH_CLIENT_SERVER=1 +
#     -H (the executor's exact seam argv, internal/executor/agent.go
#     agentEnv): the run fails fast (rc!=0) with the rate-limit error text
#     the executor's DetectRateLimit classifies. Routed runs surface ONLY
#     the final "Agent error: retry error: too many requests: ..." (no
#     WARN retry lines — those are client-side provider calls, local runs
#     only); the assertion matches that shape.
#   keeper/supervisor — tq agent-pool --crush-server: the pool boots the
#     server (runactor actor), kill -9 -> "exited unexpectedly ... restart"
#     + a NEW server pid + healthy socket, SIGTERM pool -> clean exit with
#     the server torn down (never orphaned). Unit pins cover the ladder
#     table; this is the live process proof.
#
# Zero spend by construction (mock provider, XDG-isolated scratch config;
# the isolation gate refuses to run anything before it is proven). Needs
# crush + python3 + git; TQ_BIN can pre-supply the tq binary for the
# keeper leg. Scratch-only (TQ_DB hook rail), size-guarded (the 7.3G
# crash-loop lesson, 2026-11-01 §d2).
set -euo pipefail
cd "$(dirname "$0")/../.."
REPO_ROOT="$(pwd)"

fail() {
	echo "FAIL: $*"
	exit 1
}

command -v crush >/dev/null 2>&1 || fail "crush binary not on PATH"
command -v python3 >/dev/null 2>&1 || fail "python3 required (mock provider)"

TMP="$(mktemp -d /tmp/tq-crush-pool-smoke.XXXXXX)"
export TQ_DB="$TMP/tasks.db" # hook contract: smokes never ride the production journal
SOCK="$TMP/crush.sock"
PORT="${CRUSH_MOCK_PORT:-$(python3 -c 'import socket
s = socket.socket()
s.bind(("127.0.0.1", 0))
print(s.getsockname()[1])
s.close()')}"

cleanup() {
	pkill -9 -f "crush server --host unix://$SOCK" 2>/dev/null || true
	pkill -f "$TMP/tq agent-pool" 2>/dev/null || true
	pkill -f "mock_crush_pool.py $PORT" 2>/dev/null || true
	rm -rf "$TMP"
}
trap cleanup EXIT

size_guard() {
	local mb
	mb="$(du -sm "$TMP" | cut -f1)"
	if [ "$mb" -gt 1024 ]; then
		echo "FAIL: scratch grew past 1G (${mb}M) — aborting before /tmp fills"
		exit 1
	fi
}

wait_pattern() { # file pattern timeout_s label
	local i
	for i in $(seq 1 "$(($3 * 2))"); do
		grep -q "$2" "$1" 2>/dev/null && return 0
		size_guard
		sleep 0.5
	done
	echo "FAIL: $4 — pattern '$2' never appeared in $1 within ${3}s"
	tail -20 "$1" 2>/dev/null
	return 1
}

probe_health() { # -> "200" or the error
	python3 - "$SOCK" <<'PY'
import http.client, socket, sys

class UnixHTTPConnection(http.client.HTTPConnection):
    def __init__(self, path):
        super().__init__("localhost")
        self.unix_path = path
    def connect(self):
        s = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        s.settimeout(5)
        s.connect(self.unix_path)
        self.sock = s

try:
    conn = UnixHTTPConnection(sys.argv[1])
    conn.request("GET", "/v1/health")
    print(conn.getresponse().status)
    conn.close()
except OSError as e:
    print(f"OSError {e}")
PY
}

echo "== scratch: $TMP (mock on 127.0.0.1:$PORT)"

XDG="$TMP/xdg"
DATA="$TMP/data"
REPO="$TMP/repo"
mkdir -p "$XDG/crush" "$DATA" "$REPO"

echo "== scratch config (mock provider only; auto-summarize OFF)"
cat >"$XDG/crush/crush.json" <<EOF
{
	"\$schema": "https://charm.land/crush.json",
	"providers": {
		"mock": {
			"api_key": "smoke-key",
			"base_url": "http://127.0.0.1:$PORT/v1",
			"type": "completions",
			"models": [
				{"id": "mock-1", "name": "Mock 1", "context_window": 8192, "max_tokens": 1024}
			]
		}
	},
	"models": {
		"large": {"provider": "mock", "model": "mock-1"},
		"small": {"provider": "mock", "model": "mock-1"}
	},
	"permissions": {"allowed_tools": ["bash", "write", "edit", "read", "view"]},
	"options": {"disable_default_providers": true, "disable_auto_summarize": true}
}
EOF
python3 -m json.tool "$XDG/crush/crush.json" >/dev/null || fail "scratch crush.json does not parse"

echo "== isolation gate (crush dirs under candidate env)"
DIRS_LINE1="$(XDG_CONFIG_HOME="$XDG" XDG_DATA_HOME="$DATA" crush dirs 2>&1 | head -1 || true)"
[ "$DIRS_LINE1" = "$XDG/crush" ] || fail "crush dirs first line is '$DIRS_LINE1', want '$XDG/crush' — isolation NOT proven, refusing to run anything"

git -C "$REPO" init -q
git -C "$REPO" config user.email smoke@tq.local
git -C "$REPO" config user.name "tq smoke"
printf 'scratch\n' >"$REPO/f.txt"
git -C "$REPO" add -A
git -C "$REPO" commit -qm seed

echo "== write mock provider (429 mode via MOCK_429=1)"
cat >"$TMP/mock_crush_pool.py" <<'PYEOF'
#!/usr/bin/env python3
# Minimal OpenAI-compatible mock for the pool smoke: every completion is
# either an inert streamed text answer or, with MOCK_429=1, a 429 with
# rate-limit error text (the fast-refuse signature). No tool scripting
# needed here — the wire legs own that surface.
import json
import os
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


class Handler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def log_message(self, *_args):
        pass

    def _json(self, status, body):
        payload = json.dumps(body).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(payload)))
        self.end_headers()
        self.wfile.write(payload)

    def do_POST(self):
        if not self.path.startswith("/v1/chat/completions"):
            self._json(404, {"error": "not found"})
            return
        if os.environ.get("MOCK_429") == "1":
            self._json(429, {"error": {"message": "rate limit exceeded (mock): retry after 60s",
                                       "type": "rate_limit_error", "code": 429}})
            return
        length = int(self.headers.get("Content-Length", "0"))
        req = json.loads(self.rfile.read(length) or b"{}")
        base = {"id": "chatcmpl-mock", "object": "chat.completion.chunk",
                "created": 0, "model": "mock-1"}
        if req.get("stream"):
            self.send_response(200)
            self.send_header("Content-Type", "text/event-stream")
            self.send_header("Cache-Control", "no-cache")
            self.send_header("Transfer-Encoding", "chunked")
            self.end_headers()

            def chunk(payload):
                data = f"data: {json.dumps(payload)}\n\n".encode()
                self.wfile.write(f"{len(data):x}\r\n".encode() + data + b"\r\n")
                self.wfile.flush()

            chunk({**base, "choices": [{"index": 0, "delta": {"role": "assistant", "content": "ok"},
                                        "finish_reason": None}]})
            chunk({**base, "choices": [{"index": 0, "delta": {}, "finish_reason": "stop"}]})
            self.wfile.write(b"0\r\n\r\n")
            self.wfile.flush()
            return
        self._json(200, {"id": "chatcmpl-mock", "object": "chat.completion", "created": 0,
                         "model": "mock-1",
                         "choices": [{"index": 0, "message": {"role": "assistant", "content": "ok"},
                                      "finish_reason": "stop"}]})


if __name__ == "__main__":
    ThreadingHTTPServer(("127.0.0.1", int(sys.argv[1])), Handler).serve_forever()
PYEOF

start_mock_429() {
	pkill -f "mock_crush_pool.py $PORT" 2>/dev/null || true
	sleep 0.3
	MOCK_429=1 python3 "$TMP/mock_crush_pool.py" "$PORT" >"$TMP/mock.log" 2>&1 &
	sleep 1
}

start_server() {
	pkill -9 -f "crush server --host unix://$SOCK" 2>/dev/null || true
	sleep 0.3
	XDG_CONFIG_HOME="$XDG" XDG_DATA_HOME="$DATA" \
		setsid crush server --host "unix://$SOCK" >"$TMP/server.log" 2>&1 &
	sleep 2
}

# The executor's exact seam argv: CRUSH_CLIENT_SERVER=1 + -H (agentEnv).
routed_run() { # log-name
	(
		cd "$REPO"
		XDG_CONFIG_HOME="$XDG" XDG_DATA_HOME="$DATA" CRUSH_CLIENT_SERVER=1 \
			timeout 150 crush run -H "unix://$SOCK" --verbose \
			"please write the verdict file now" >"$TMP/$1" 2>&1
	)
}

echo "== leg: routed 429 fast-refuse (executor seam argv)"
start_mock_429
start_server
[ "$(probe_health)" = "200" ] || fail "server health probe failed before the 429 leg"
routed_run run429.log
RC=$?
[ "$RC" -ne 0 ] || { cat "$TMP/run429.log"; fail "routed 429 run exited 0 — fast-refuse contract broken"; }
grep -qi "rate limit exceeded" "$TMP/run429.log" || {
	echo "--- run output ---"
	cat "$TMP/run429.log"
	fail "routed 429 output carries no rate-limit error text (DetectRateLimit input missing)"
}
echo "PASS: routed 429 fails fast (rc=$RC) with the rate-limit signature"
pkill -f "mock_crush_pool.py $PORT" 2>/dev/null || true
pkill -9 -f "crush server --host unix://$SOCK" 2>/dev/null || true

echo "== leg: pool keeper/supervisor (kill -9 -> restart -> clean pool exit)"
if [ -n "${TQ_BIN:-}" ]; then
	echo "== using prebuilt tq: $TQ_BIN"
	cp "$TQ_BIN" "$TMP/tq"
	chmod +x "$TMP/tq"
else
	echo "== build tq"
	"$REPO_ROOT/scripts/build-tq.sh" "$TMP/tq"
fi

# The pool's crush server boots from the same scratch config (providers
# are lazy — nothing dials the dead mock port while the pool idles on an
# all-done TODO list: no harvest mint, no agent turn).
PSOCK="$TMP/pool-crush.sock"
mkdir -p "$TMP/poolproj/repo"
git -C "$TMP/poolproj/repo" init -q
git -C "$TMP/poolproj/repo" config user.email smoke@tq.local
git -C "$TMP/poolproj/repo" config user.name "tq smoke"
printf -- '- [x] seeded done item (idle pool — no harvest mint)\n' >"$TMP/poolproj/repo/TODO_LIST.md"
git -C "$TMP/poolproj/repo" add -A
git -C "$TMP/poolproj/repo" commit -qm seed

(
	cd "$TMP/poolproj"
	XDG_CONFIG_HOME="$XDG" XDG_DATA_HOME="$DATA" \
		setsid timeout 180 "$TMP/tq" agent-pool --db "$TMP/pool.db" \
		--projects-dir "$TMP/poolproj" --repos repo \
		--poll 200ms --task-timeout 30s --interval 5s --crush-server "$PSOCK" \
		>"$TMP/pool.log" 2>&1 </dev/null
) &
wait_pattern "$TMP/pool.log" "listening on" 45 "pool never started the crush server"
SRVPATT="crush server --host unix://$PSOCK"
SRVPID="$(pgrep -f "$SRVPATT" | head -1)"
[ -n "$SRVPID" ] || { tail -20 "$TMP/pool.log"; fail "no crush server child under the pool"; }
echo "PASS: pool up, server child pid $SRVPID"

pkill -9 -f "$SRVPATT"
wait_pattern "$TMP/pool.log" "restart 1" 45 "supervisor did not log a restart after kill -9"
NEWPID=""
for i in $(seq 1 90); do
	NEWPID="$(pgrep -f "$SRVPATT" | head -1)"
	[ -n "$NEWPID" ] && [ "$NEWPID" != "$SRVPID" ] && break
	size_guard
	sleep 0.5
done
[ -n "$NEWPID" ] && [ "$NEWPID" != "$SRVPID" ] || { tail -20 "$TMP/pool.log"; fail "no replacement server process"; }
echo "PASS: supervisor restarted the server ($SRVPID -> $NEWPID)"

pkill -TERM -f "$TMP/tq agent-pool"
for i in $(seq 1 30); do
	pgrep -f "$TMP/tq agent-pool" >/dev/null || break
	size_guard
	sleep 0.5
done
if pgrep -f "$TMP/tq agent-pool" >/dev/null; then
	tail -20 "$TMP/pool.log"
	fail "pool did not exit on SIGTERM"
fi
sleep 1
pgrep -f "$SRVPATT" >/dev/null && fail "crush server orphaned after pool exit"
echo "PASS: pool exited clean on SIGTERM, server torn down with it"

echo
echo "PASS: crush-pool-supervisor smoke — routed 429 fast-refuse + keeper/supervisor restart both green"
