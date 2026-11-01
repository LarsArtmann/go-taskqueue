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
#   mid-run kill (V2.10) — kill -9 the server INSIDE a stalled routed agent
#     turn (mock scripts the tool_call, stalls the tool-result): the
#     severed client exits 0 with the WARN line, the executor classifies
#     crush-server death, the worker REQUEUES without attempt burn (task
#     pending, attempts 0, last error carries [crush-server]), and the
#     supervisor restart ladder rides the same death.
#   routed verdict bridge (V2.18) — the mock regexes the verdict rail's
#     per-run path out of the work prompt and scripts the REAL
#     `tq verdict --file <path> '<json>'` as the bash tool call: the
#     SERVER-side tool runs it (tq rides the pool's PATH into the server
#     env), the executor reads the file after the run, and the completion
#     fact carries the verdict — the full M8 bridge, in-pool, zero spend.
#   operator cancel (V2.15) — tq cancel --force on a stalled routed turn:
#     the worker's heartbeat cancel stops the run context (the routed
#     client process exits), the task lands cancelled, and once the held
#     server-side turn drains no session over the repo stays busy (the
#     server reaped the cancelled client's workspace).
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
	local _
	for _ in $(seq 1 "$(($3 * 2))"); do
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
# pins: XDG isolation leg — mock is the ONLY resolvable provider
cat >"$XDG/crush/crush.json" <<EOF
{
	"\$schema": "https://charm.land/crush.json",
	"providers": {
		"mock": {
			"api_key": "smoke-key",
			"base_url": "http://127.0.0.1:$PORT/v1",
			"type": "completions",
			"models": [
				{"id": "mock-1", "name": "Mock 1", "context_window": 200000, "max_tokens": 1024}
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
# pins: the 429 fast-refuse branch + the inert streamed-text branch
cat >"$TMP/mock_crush_pool.py" <<'PYEOF'
#!/usr/bin/env python3
# Minimal OpenAI-compatible mock for the pool smoke: every completion is
# either an inert streamed text answer or, with MOCK_429=1, a 429 with
# rate-limit error text (the fast-refuse signature). MOCK_STALL seconds
# stall the TOOL-RESULT turn so the mid-run kill leg gets its window; the
# tool_call scripting keys on "verdict" in the last NON-TEMPLATE user
# message (the route smoke's proven content-key).
import json
import os
import re
import sys
import time
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
        messages = req.get("messages", [])
        # Full content for the content-key (a harvested work prompt carries
        # its "verdict" item text hundreds of chars in); truncated copy for
        # logging only.
        last_user_full = next((str(m.get("content")) for m in reversed(messages)
                               if m.get("role") == "user"), "")
        last_user = last_user_full[:80]
        req_log = os.environ.get("MOCK_REQ_LOG", "")
        if req_log:
            roles = ",".join(str(m.get("role")) for m in messages)
            with open(req_log, "a") as fh:
                fh.write(f"#req stream={req.get('stream')} roles={roles} last_user={last_user!r}\n")
        delta = {"role": "assistant", "content": "ok"}
        is_template = last_user_full.startswith("Generate a concise title") or last_user_full.startswith("Provide a detailed summary")
        has_tool = any(m.get("role") == "tool" for m in messages)
        if has_tool:
            # Stall ONLY the kill/cancel chains (their scripted command is
            # the probe echo) so the verdict leg completes at full speed.
            if "pool-leg-probe" in json.dumps(messages):
                stall = float(os.environ.get("MOCK_STALL", "0"))
                if stall > 0:
                    marker = os.environ.get("MOCK_STALL_MARKER", "")
                    if marker:
                        with open(marker, "a") as fh:
                            fh.write("stall\n")
                    time.sleep(stall)
            delta = {"role": "assistant", "content": "mock done"}
        elif "record the verdict result" in last_user_full and not is_template:
            # V2.18: run the rail VERBATIM — path regexed out of the baked
            # prompt rail, executed by the server-side bash tool (the tq
            # binary rides the pool's PATH into the server's env).
            m = re.search(r"tq verdict --file (\S+)", last_user_full)
            path = m.group(1) if m else "/tmp/mock-verdict-path-missing"
            cmd = "tq verdict --file " + path + " '{\"status\":\"done\",\"note\":\"routed e2e\"}'"
            args = json.dumps({"command": cmd, "description": "record the routed verdict"})
            delta = {"role": "assistant", "content": None,
                     "tool_calls": [{"id": "call_1", "type": "function",
                                     "function": {"name": "bash", "arguments": args}}]}
        elif "verdict" in last_user_full.lower() and not is_template:
            args = json.dumps({"command": "echo pool-leg-probe", "description": "probe tool call"})
            delta = {"role": "assistant", "content": None,
                     "tool_calls": [{"id": "call_1", "type": "function",
                                     "function": {"name": "bash", "arguments": args}}]}
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

            calls = delta.get("tool_calls")
            if calls:
                fn = calls[0]["function"]
                chunk({**base, "choices": [{"index": 0, "delta": {"role": "assistant", "tool_calls": [{"index": 0, "id": calls[0]["id"], "type": "function", "function": {"name": fn["name"], "arguments": ""}}]}, "finish_reason": None}]})
                chunk({**base, "choices": [{"index": 0, "delta": {"tool_calls": [{"index": 0, "function": {"arguments": fn["arguments"]}}]}, "finish_reason": None}]})
                chunk({**base, "choices": [{"index": 0, "delta": {}, "finish_reason": "tool_calls"}]})
            else:
                chunk({**base, "choices": [{"index": 0, "delta": delta, "finish_reason": None}]})
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
RC=0
routed_run run429.log || RC=$?
[ "$RC" -ne 0 ] || {
	cat "$TMP/run429.log"
	fail "routed 429 run exited 0 — fast-refuse contract broken"
}
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
	XDG_CONFIG_HOME="$XDG" XDG_DATA_HOME="$DATA" PATH="$TMP:$PATH" \
		setsid timeout 420 "$TMP/tq" agent-pool --db "$TMP/pool.db" \
		--projects-dir "$TMP/poolproj" --repos repo \
		--poll 200ms --task-timeout 30s --interval 5s --lease 20s --crush-server "$PSOCK" \
		>"$TMP/pool.log" 2>&1 </dev/null
) &
wait_pattern "$TMP/pool.log" "listening on" 45 "pool never started the crush server"
SRVPATT="crush server --host unix://$PSOCK"
SRVPID="$(pgrep -f "$SRVPATT" | head -1)"
[ -n "$SRVPID" ] || {
	tail -20 "$TMP/pool.log"
	fail "no crush server child under the pool"
}
echo "PASS: pool up, server child pid $SRVPID"

pkill -9 -f "$SRVPATT"
wait_pattern "$TMP/pool.log" "restart 1" 45 "supervisor did not log a restart after kill -9"
NEWPID=""
for _ in $(seq 1 90); do
	NEWPID="$(pgrep -f "$SRVPATT" | head -1)"
	[ -n "$NEWPID" ] && [ "$NEWPID" != "$SRVPID" ] && break
	size_guard
	sleep 0.5
done
if [ -z "$NEWPID" ] || [ "$NEWPID" = "$SRVPID" ]; then
	echo "FAIL: no replacement server process after kill -9 (supervisor ladder broken)"
	tail -20 "$TMP/pool.log"
	exit 1
fi
echo "PASS: supervisor restarted the server ($SRVPID -> $NEWPID)"

echo "== leg: server death MID-RUN requeues the routed task without attempt burn (V2.10)"
# pins: the severed-stream classification end-to-end — the mock scripts a
# bash tool_call for the harvested prompt (item text contains "verdict"),
# stalls the tool-result turn, and the leg kills -9 the server inside that
# window. The severed client exits 0 with the WARN line; the executor's
# DetectCrushServerDeath classifies it; the worker requeues (class
# crush-server): task PENDING, attempts 0, last error carries the marker.
# rm the marker BEFORE the item can run: with claim-wake armed the whole
# pre-stall phase (claim -> title -> tool_call -> tool result) takes under
# a second, so a marker written before a LATER rm gets erased and the poll
# then waits 60s for a second stall that never comes (run9 lesson: task
# completed exactly MOCK_STALL after enqueue = the stall DID arm; the
# marker died to the rm race, not to an inert mock).
rm -f "$TMP/stall-entered.log"
MOCK_429=0 MOCK_STALL=25 MOCK_STALL_MARKER="$TMP/stall-entered.log" \
	MOCK_REQ_LOG="$TMP/mock2-requests.log" \
	python3 "$TMP/mock_crush_pool.py" "$PORT" >"$TMP/mock2.log" 2>&1 &
sleep 1
printf -- '- [ ] smoke probe: write the verdict file now\n' >"$TMP/poolproj/repo/TODO_LIST.md"
git -C "$TMP/poolproj/repo" add -A
git -C "$TMP/poolproj/repo" commit -qm "open the mid-run kill probe item"
# taskq.py: one filter helper for both polls (running-id, then full state)
cat >"$TMP/taskq.py" <<'PYEOF'
import json
import sys

try:
    rows = json.load(sys.stdin)
except Exception:
    rows = None
rows = [r for r in (rows or []) if isinstance(r, dict)]
if sys.argv[1] == "running-id":
    running = [r for r in rows if r.get("status") == "running"]
    print(running[0]["id"] if running else "")
else:
    row = next((r for r in rows if r.get("id") == sys.argv[1]), None)
    if not row:
        print("missing")
    else:
        print(row.get("status"), row.get("attempts"))
PYEOF
# task_state: tq tasks --json filtered through taskq.py (a bare $((
# adjacency reads as arithmetic to some shells — helper function instead)
task_state() { # mode [task-id]
	local out
	out="$("$TMP/tq" tasks --db "$TMP/pool.db" --json 2>/dev/null || true)"
	[ -n "$out" ] || out='[]'
	printf '%s' "$out" | python3 "$TMP/taskq.py" "$@"
}
RUNNING_ID=""
for _ in $(seq 1 150); do
	RUNNING_ID="$(task_state running-id)"
	[ -n "$RUNNING_ID" ] && break
	size_guard
	sleep 0.3
done
[ -n "$RUNNING_ID" ] || {
	tail -25 "$TMP/pool.log"
	"$TMP/tq" tasks --db "$TMP/pool.db" --json 2>/dev/null | head -40
	fail "no task reached running — harvest never minted the probe item (or it died before this poll caught it)"
}
# Kill INSIDE the stalled tool-result turn, not at claim time: a kill
# before the first provider round lets the client respawn the server
# (ensureServer, M7) and the run completes normally — no severed stream.
# The mock marks the stall's entry (marker rm'd pre-item above); the kill
# must land after the marker and inside the MOCK_STALL window.
for _ in $(seq 1 200); do
	[ -s "$TMP/stall-entered.log" ] && break
	size_guard
	sleep 0.3
done
[ -s "$TMP/stall-entered.log" ] || {
	tail -25 "$TMP/pool.log"
	"$TMP/tq" tasks --db "$TMP/pool.db" --json 2>/dev/null | head -30
	tail -10 "$TMP/mock2.log" 2>/dev/null
	tail -10 "$TMP/mock2-requests.log" 2>/dev/null
	fail "the mock never entered the stalled tool-result turn — probe task never reached its tool call"
}
DEAD_PID="$(pgrep -f "crush server --host unix://$PSOCK" | head -1)"
[ -n "$DEAD_PID" ] || fail "no server process to kill mid-run"
kill -9 "$DEAD_PID"
wait_pattern "$TMP/pool.log" "crush server died around the routed run" 45 \
	"the severed run was never classified as crush-server death"
echo "PASS: severed mid-run turn classified (crush-server death, no burn)"
STATE=""
for _ in $(seq 1 100); do
	STATE="$(task_state "$RUNNING_ID")"
	[ "$STATE" = "pending 0" ] && break
	size_guard
	sleep 0.3
done
[ "$STATE" = "pending 0" ] || {
	tail -25 "$TMP/pool.log"
	fail "task $RUNNING_ID state after server kill = '$STATE', want pending/0-attempts (no burn)"
}
# The requeue class rides the requeued FACT's error + evidence detail —
# tq tasks --json carries no last_error field — so tq show is the operator
# surface that must render the [crush-server] marker.
SHOW_OK=0
for _ in $(seq 1 20); do
	if "$TMP/tq" show --db "$TMP/pool.db" "$RUNNING_ID" 2>/dev/null | grep -q "crush-server"; then
		SHOW_OK=1
		break
	fi
	size_guard
	sleep 0.3
done
[ "$SHOW_OK" = 1 ] || {
	"$TMP/tq" show --db "$TMP/pool.db" "$RUNNING_ID" 2>/dev/null | head -40
	fail "tq show carries no [crush-server] class for the requeued task"
}
echo "PASS: task requeued without attempt burn (pending, attempts 0, class crush-server)"
wait_pattern "$TMP/pool.log" "restart 2" 45 "supervisor did not restart the server after the mid-run kill"
echo "PASS: supervisor restart ladder rode the mid-run death too"
# Retire the probe task: its ~2m no-burn park could re-claim mid-leg and
# steal the running slot from the later probes (its job is done).
"$TMP/tq" cancel --db "$TMP/pool.db" --reason "smoke: probe retired after the kill leg" "$RUNNING_ID" 2>/dev/null || true

echo "== leg: routed verdict bridge completes end-to-end (V2.18)"
# The mock regexes the rail's per-run path out of the baked prompt and
# scripts the REAL tq verdict --file call as the bash tool: the
# SERVER-side tool executes it (tq rides the pool's PATH into the server
# env), the executor reads the file after the run, and the completion
# fact carries the verdict — the full M8 bridge, in-pool, zero spend.
printf -- '- [ ] smoke probe: record the verdict result\n' >"$TMP/poolproj/repo/TODO_LIST.md"
git -C "$TMP/poolproj/repo" add -A
git -C "$TMP/poolproj/repo" commit -qm "open the routed verdict probe item"
VERDICT_ID=""
for _ in $(seq 1 150); do
	VERDICT_ID="$(task_state running-id)"
	[ -n "$VERDICT_ID" ] && [ "$VERDICT_ID" != "$RUNNING_ID" ] && break
	size_guard
	sleep 0.3
done
[ -n "$VERDICT_ID" ] || {
	tail -25 "$TMP/pool.log"
	fail "verdict probe: no task reached running"
}
VSTATE=""
for _ in $(seq 1 100); do
	VSTATE="$(task_state "$VERDICT_ID")"
	case "$VSTATE" in
		completed* | dead* | failed* | cancelled*) break ;;
	esac
	size_guard
	sleep 0.3
done
case "$VSTATE" in
	completed*) : ;;
	*)
		tail -25 "$TMP/pool.log"
		"$TMP/tq" show --db "$TMP/pool.db" "$VERDICT_ID" 2>/dev/null | head -40
		fail "verdict probe task state = '$VSTATE', want completed"
		;;
esac
"$TMP/tq" show --db "$TMP/pool.db" "$VERDICT_ID" 2>/dev/null | grep -q "routed e2e" || {
	"$TMP/tq" show --db "$TMP/pool.db" "$VERDICT_ID" 2>/dev/null | head -50
	fail "the routed verdict never landed in the task facts (bridge broken)"
}
echo "PASS: routed run recorded its verdict via the baked rail (file -> facts)"

echo "== leg: operator cancel reaches the routed client + the server reaps (V2.15)"
cat >"$TMP/busy.py" <<'PYEOF'
import http.client, json, socket, sys

class UnixHTTPConnection(http.client.HTTPConnection):
    def __init__(self, path):
        super().__init__("localhost")
        self.unix_path = path
    def connect(self):
        s = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        s.settimeout(5)
        s.connect(self.unix_path)
        self.sock = s

def get(sock, path):
    conn = UnixHTTPConnection(sock)
    conn.request("GET", path)
    body = conn.getresponse().read()
    conn.close()
    return json.loads(body)

busy = 0
try:
    for w in get(sys.argv[1], "/v1/workspaces"):
        if sys.argv[2] not in str(w.get("path", "")):
            continue
        for sess in get(sys.argv[1], "/v1/workspaces/%s/sessions" % w.get("id")):
            if sess.get("is_busy"):
                busy += 1
except Exception as exc:
    print("probe-error %s" % exc)
else:
    print(busy)
PYEOF
printf -- '- [ ] smoke probe: cancel this verdict run\n' >"$TMP/poolproj/repo/TODO_LIST.md"
git -C "$TMP/poolproj/repo" add -A
git -C "$TMP/poolproj/repo" commit -qm "open the cancel probe item"
CANCEL_ID=""
for _ in $(seq 1 150); do
	CANCEL_ID="$(task_state running-id)"
	[ -n "$CANCEL_ID" ] && [ "$CANCEL_ID" != "$RUNNING_ID" ] && break
	size_guard
	sleep 0.3
done
[ -n "$CANCEL_ID" ] || {
	tail -25 "$TMP/pool.log"
	fail "cancel probe: no task reached running"
}
# Gate on the SECOND stall line (the kill leg wrote the first): the cancel
# must land INSIDE the held tool-result turn, same window rule as the kill.
STALL_LINES=0
for _ in $(seq 1 200); do
	STALL_LINES="$(wc -l <"$TMP/stall-entered.log" 2>/dev/null || echo 0)"
	[ "$STALL_LINES" -ge 2 ] && break
	size_guard
	sleep 0.3
done
[ "$STALL_LINES" -ge 2 ] || {
	tail -25 "$TMP/pool.log"
	tail -10 "$TMP/mock2-requests.log" 2>/dev/null
	fail "cancel probe never entered its stalled tool-result turn"
}
PRE_BUSY="$(python3 "$TMP/busy.py" "$PSOCK" "$TMP/poolproj/repo" 2>/dev/null || true)"
echo "== busy sessions over the repo before cancel: ${PRE_BUSY:-unprobed}"
"$TMP/tq" cancel --db "$TMP/pool.db" --force --reason "smoke cancel leg" "$CANCEL_ID"
wait_pattern "$TMP/pool.log" "task cancelled by operator request" 45 \
	"the cooperative cancel never stopped the routed run"
echo "PASS: cancel reached the worker and stopped the routed run"
CLIENTS_OK=0
for _ in $(seq 1 40); do
	if ! pgrep -f "crush run --verbose --cwd $TMP/poolproj/repo" >/dev/null 2>&1; then
		CLIENTS_OK=1
		break
	fi
	size_guard
	sleep 0.5
done
[ "$CLIENTS_OK" = 1 ] || fail "routed crush client still alive after cancel"
BUSY_OK=0
for _ in $(seq 1 90); do
	BUSY_NOW="$(python3 "$TMP/busy.py" "$PSOCK" "$TMP/poolproj/repo" 2>/dev/null || true)"
	if [ "$BUSY_NOW" = "0" ]; then
		BUSY_OK=1
		break
	fi
	size_guard
	sleep 0.5
done
[ "$BUSY_OK" = 1 ] || {
	python3 "$TMP/busy.py" "$PSOCK" "$TMP/poolproj/repo" 2>&1 || true
	fail "a session over the repo stayed busy after cancel (server never reaped the turn)"
}
CSTATE="$(task_state "$CANCEL_ID")"
case "$CSTATE" in
	cancelled*) : ;;
	*) fail "cancel probe task state = '$CSTATE', want cancelled" ;;
esac
echo "PASS: cancel leg — client exited, server reaped the turn, task cancelled"

pkill -TERM -f "$TMP/tq agent-pool"
for _ in $(seq 1 30); do
	pgrep -f "$TMP/tq agent-pool" >/dev/null || break
	size_guard
	sleep 0.5
done
if pgrep -f "$TMP/tq agent-pool" >/dev/null; then
	echo "FAIL: pool did not exit on SIGTERM within 15s"
	tail -20 "$TMP/pool.log"
	exit 1
fi
sleep 1
pgrep -f "$SRVPATT" >/dev/null && fail "crush server orphaned after pool exit"
echo "PASS: pool exited clean on SIGTERM, server torn down with it"

echo
echo "PASS: crush-pool-supervisor smoke — routed 429 fast-refuse + keeper/supervisor restart + mid-run kill no-burn + routed verdict bridge + operator cancel all green"
