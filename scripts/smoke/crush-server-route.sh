#!/usr/bin/env bash
# End-to-end smoke of the crush client/server route (L1 seam wire legs,
# TODO row 1450 / L1.8): boots an ISOLATED `crush server` (XDG-redirected
# scratch config with a mock OpenAI-compatible provider — ZERO spend) and
# proves the routed-run contract end to end:
#
#   isolation  — crush dirs under the candidate env points at scratch only
#                and the first run's log says provider=mock (the §e1 gate:
#                no run before isolation is proven)
#   health     — GET /v1/health answers 200 on the unix socket
#   routed run — CRUSH_CLIENT_SERVER=1 + -H: rc 0, session-id line (M4),
#                provider=mock receipt, tool-call verdict write (M1)
#   live view  — /v1/workspaces lists the run's workspace DURING the run
#                (the routing discriminator — a local run never appears),
#                sessions + messages carry the M5 closeout shapes
#   env gap    — CANARY (two-sided): the routed bash tool sees the SERVER
#                process env but NOT the client's $TQ_RESULT_FILE — the
#                L1 verdict/question env channels are BROKEN under routing
#                (v0.99.0 internal/shell/background.go builds the tool
#                shell without workspace Env; shell.go falls back to the
#                server's os.Environ). When upstream fixes env
#                propagation this leg goes RED on purpose: flip the
#                asserts, drop the caveat rows, re-enable the E-gaps.
#   hooks      — repo .crushrc PreToolUse hook fires SERVER-side and
#                denies a marker command (M3: the hook fleet survives
#                routing)
#   respawn    — kill -9 the server; the next routed client respawns it
#                (M7: ensureServer) and a fresh run completes
#
# No tq binary is exercised (wire legs only — the pool-integration E-gaps
# 429/question-poll stay unit-pinned until the env gap closes); TQ_DB is
# exported anyway per the production-trap rule: a bare tq command in a
# debug session would otherwise touch the PRODUCTION journal.
#
# Portability: GNU date not required; python3 (stdlib only) drives the
# HTTP probes and the mock provider. Requires `crush` on PATH — a host
# without it fails loudly instead of skipping silently.
set -euo pipefail
cd "$(dirname "$0")/../.."
REPO_ROOT="$(pwd)"

fail() {
	echo "FAIL: $*"
	exit 1
}

command -v crush >/dev/null 2>&1 || fail "crush binary not on PATH — this smoke routes real crush runs through a scratch server"
command -v python3 >/dev/null 2>&1 || fail "python3 required (mock provider + HTTP probes)"

TMP="$(mktemp -d /tmp/tq-crush-route-smoke.XXXXXX)"
MOCK_PID=""
SERVER_PID=""
cleanup() {
	[ -n "$MOCK_PID" ] && kill "$MOCK_PID" 2>/dev/null || true
	# The respawn leg's client-spawned server is not our child; match the
	# exact scratch socket so nothing else is touched.
	pkill -9 -f "crush server --host unix://$TMP/crush.sock" 2>/dev/null || true
	[ -n "$SERVER_PID" ] && kill "$SERVER_PID" 2>/dev/null || true
	rm -rf "$TMP"
}
trap cleanup EXIT

export TQ_DB="$TMP/tasks.db" # hook contract: smokes never ride the production journal

CRUSH_BIN="$(command -v crush)"
XDG="$TMP/xdg"
DATA="$TMP/data"
REPO="$TMP/repo"
SOCK="$TMP/crush.sock"
mkdir -p "$XDG/crush" "$DATA" "$REPO"

echo "== scratch: $TMP"

# ---------------------------------------------------------------------------
# Port allocation: bind port 0, take the port, close, let the mock race for
# it (the 2026-11-01 probe session died to a zombie mock that kept a fixed
# port and hijacked every later run — never pin a fixed port).
# ---------------------------------------------------------------------------
PORT="$(python3 -c 'import socket
s = socket.socket()
s.bind(("127.0.0.1", 0))
print(s.getsockname()[1])
s.close()')"

echo "== isolation gate (crush dirs under candidate env)"
DIRS_LINE1="$(XDG_CONFIG_HOME="$XDG" XDG_DATA_HOME="$DATA" crush dirs 2>/dev/null | head -1)"
[ "$DIRS_LINE1" = "$XDG/crush" ] || fail "crush dirs first line is '$DIRS_LINE1', want the scratch config dir '$XDG/crush' — isolation NOT proven, refusing to run anything"
echo "PASS: candidate config dir resolves inside the scratch"

echo "== scratch config (mock provider only, defaults disabled)"
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
  "options": {"disable_default_providers": true}
}
EOF
python3 -c 'import json,sys; json.load(open(sys.argv[1]))' "$XDG/crush/crush.json" || fail "scratch crush.json does not parse"
echo "PASS: config parses (snake_case, disable_default_providers)"

echo "== seed scratch repo"
git -C "$REPO" init -q
git -C "$REPO" config user.email smoke@tq.local
git -C "$REPO" config user.name "tq smoke"
printf 'scratch\n' >"$REPO/f.txt"
git -C "$REPO" add -A
git -C "$REPO" commit -qm seed

# ---------------------------------------------------------------------------
# Mock provider: OpenAI-compatible, scripted by request content (see the
# probe recipe it was promoted from). argv:
#   <port> <verdict-cmd> <final-text> [stall-seconds]
# The stall delays the tool-result response so the smoke can probe the live
# workspace mid-run.
# ---------------------------------------------------------------------------
cat >"$TMP/mock_provider.py" <<'PYEOF'
#!/usr/bin/env python3
import json
import sys
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

REQ_LOG = open(sys.argv[5] if len(sys.argv) > 5 else "/dev/null", "a", buffering=1)
REQ_COUNT = 0


def sse_chunk(obj):
    return f"data: {json.dumps(obj)}\n\n"


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

    def do_GET(self):
        if self.path.startswith("/v1/models"):
            self._json(200, {"object": "list", "data": [{"id": "mock-1", "object": "model", "created": 0, "owned_by": "smoke"}]})
            return
        self._json(404, {"error": "not found"})

    def do_POST(self):
        global REQ_COUNT
        if not self.path.startswith("/v1/chat/completions"):
            self._json(404, {"error": "not found"})
            return
        length = int(self.headers.get("Content-Length", "0"))
        req = json.loads(self.rfile.read(length) or b"{}")
        messages = req.get("messages", [])
        REQ_COUNT += 1
        roles = [m.get("role") for m in messages]
        last_user = next((str(m.get("content"))[:80] for m in reversed(messages) if m.get("role") == "user"), "")
        REQ_LOG.write(f"#{REQ_COUNT} stream={req.get('stream')} roles={roles} last_user={last_user!r}\n")

        # Loop guard: everything after the scripted exchange answers inertly
        # and instantly so no consumer can spin the provider forever.
        if REQ_COUNT > 8:
            delta = {"role": "assistant", "content": "done"}
        elif any(m.get("role") == "tool" for m in messages):
            stall = float(sys.argv[4]) if len(sys.argv) > 4 else 0.0
            if stall > 0:
                time.sleep(stall)
            delta = {"role": "assistant", "content": sys.argv[3] if len(sys.argv) > 3 else "mock done"}
        else:
            # The REAL task turn: the last user message is the task prompt
            # itself — NOT an internal template quoting it (title generation
            # prepends "Generate a concise title..."). Matching on template
            # heads sends the tool_call to the wrong conversation.
            is_template = last_user.startswith("Generate a concise title") or last_user.startswith("Provide a detailed summary")
            if "verdict" in last_user.lower() and not is_template:
                cmd = sys.argv[2] if len(sys.argv) > 2 else "echo verdict"
                args = json.dumps({"command": cmd, "description": "write the verdict file"})
                delta = {
                    "role": "assistant",
                    "content": None,
                    "tool_calls": [{"id": "call_1", "type": "function", "function": {"name": "bash", "arguments": args}}],
                }
            else:
                delta = {"role": "assistant", "content": "ok"}

        if req.get("stream"):
            self.send_response(200)
            self.send_header("Content-Type", "text/event-stream")
            self.send_header("Cache-Control", "no-cache")
            self.send_header("Transfer-Encoding", "chunked")
            self.end_headers()

            def chunk(payload):
                data = sse_chunk(payload).encode()
                self.wfile.write(f"{len(data):x}\r\n".encode() + data + b"\r\n")
                self.wfile.flush()

            base = {"id": "chatcmpl-mock", "object": "chat.completion.chunk", "created": 0, "model": "mock-1"}
            calls = delta.get("tool_calls")
            if calls:
                fn = calls[0]["function"]
                # OpenAI incremental tool_call streaming: id/name chunk,
                # then argument chunks, then the tool_calls finish marker.
                chunk({**base, "choices": [{"index": 0, "delta": {"role": "assistant", "tool_calls": [{"index": 0, "id": calls[0]["id"], "type": "function", "function": {"name": fn["name"], "arguments": ""}}]}, "finish_reason": None}]})
                chunk({**base, "choices": [{"index": 0, "delta": {"tool_calls": [{"index": 0, "function": {"arguments": fn["arguments"]}}]}, "finish_reason": None}]})
                chunk({**base, "choices": [{"index": 0, "delta": {}, "finish_reason": "tool_calls"}]})
            else:
                chunk({**base, "choices": [{"index": 0, "delta": delta, "finish_reason": None}]})
                chunk({**base, "choices": [{"index": 0, "delta": {}, "finish_reason": "stop"}]})
            self.wfile.write(b"0\r\n\r\n")
            self.wfile.flush()
            return

        self._json(200, {"id": "chatcmpl-mock", "object": "chat.completion", "created": 0, "model": "mock-1", "choices": [{"index": 0, "message": delta, "finish_reason": "stop"}]})


if __name__ == "__main__":
    ThreadingHTTPServer(("127.0.0.1", int(sys.argv[1])), Handler).serve_forever()
PYEOF

start_mock() { # <verdict-cmd> <final-text> [stall]
	pkill -9 -f "mock_provider.py $PORT " 2>/dev/null || true
	python3 "$TMP/mock_provider.py" "$PORT" "$1" "$2" "${3:-0}" "$TMP/mock-requests.log" >"$TMP/mock.log" 2>&1 &
	MOCK_PID=$!
	sleep 1
	kill -0 "$MOCK_PID" 2>/dev/null || { cat "$TMP/mock.log"; fail "mock provider died on startup (port $PORT busy?)"; }
}

probe() { # <path> — GET on the scratch socket, de-chunked body on stdout
	python3 - "$SOCK" "$1" <<'PYEOF'
import re
import socket
import sys

sock, path = sys.argv[1], sys.argv[2]
try:
    s = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
    s.settimeout(5)
    s.connect(sock)
    s.sendall(f"GET {path} HTTP/1.1\r\nHost: localhost\r\nAccept: application/json\r\nConnection: close\r\n\r\n".encode())
    buf = b""
    while True:
        d = s.recv(65536)
        if not d:
            break
        buf += d
    s.close()
except OSError as e:
    print(f"PROBE_ERROR {e}")
    sys.exit(0)
head, _, body = buf.partition(b"\r\n\r\n")
status = head.split(b"\r\n")[0].decode()
txt = body.decode(errors="replace")
m = re.match(r"^[0-9a-f]+\r\n", txt)
if m:  # de-chunk transfer-encoded bodies
    out, rest = [], txt
    while True:
        cm = re.match(r"^([0-9a-f]+)\r\n", rest)
        if not cm:
            break
        n = int(cm.group(1), 16)
        if n == 0:
            break
        out.append(rest[cm.end():cm.end() + n])
        rest = rest[cm.end() + n + 2:]
    txt = "".join(out)
print(f"{status}\n{txt}")
PYEOF
}

routed_run() { # <log-name> — one CRUSH_CLIENT_SERVER run; rc on stdout
	(
		cd "$REPO"
		XDG_CONFIG_HOME="$XDG" XDG_DATA_HOME="$DATA" \
			TQ_RESULT_FILE="$TMP/result.json" \
			CRUSH_CLIENT_SERVER=1 \
			timeout 90 "$CRUSH_BIN" run -H "unix://$SOCK" --verbose \
			"please write the verdict file now" >"$TMP/$1" 2>&1
	)
}

echo "== boot isolated server"
start_mock 'printf %s "{\"verdict\":\"ok-routed\"}" > smoke-verdict.json' 'mock done' 0
XDG_CONFIG_HOME="$XDG" XDG_DATA_HOME="$DATA" "$CRUSH_BIN" server --host "unix://$SOCK" >"$TMP/server.log" 2>&1 &
SERVER_PID=$!
sleep 2
HEALTH="$(probe /v1/health | head -1)"
echo "$HEALTH" | grep -q "200" || fail "server health probe: '$HEALTH'"
echo "PASS: /v1/health 200 on the scratch socket"

echo "== leg: routed verdict round-trip (M1/M4)"
rm -f "$REPO/smoke-verdict.json"
routed_run run1.log
RC=$?
[ "$RC" -eq 0 ] || { cat "$TMP/run1.log"; fail "routed run rc=$RC" ; }
grep -q "Created session for non-interactive run session_id=" "$TMP/run1.log" || fail "no session-id line in routed output (M4)"
grep -Eq 'provider=mock' "$TMP/run1.log" || fail "run log does not name provider=mock — isolation was NEVER proven, abort"
echo "PASS: rc 0 + session-id line + provider=mock receipt"
grep -q '"verdict":"ok-routed"' "$REPO/smoke-verdict.json" || { cat "$TMP/run1.log"; fail "verdict file missing/wrong after routed run"; }
echo "PASS: routed tool-call wrote the verdict file"

echo "== leg: live workspace + M5 shapes during the run"
start_mock 'printf %s "{\"verdict\":\"ok-routed\"}" > smoke-verdict.json' 'mock done' 20
rm -f "$REPO/smoke-verdict.json" "$TMP/shapes.json"
routed_run run2.log &
RUN2_PID=$!
WS_JSON=""
for _ in $(seq 1 20); do
	BODY="$(probe /v1/workspaces | tail -n +2)"
	if echo "$BODY" | python3 -c 'import json,sys
ws = json.load(sys.stdin)
sys.exit(0 if ws else 1)' 2>/dev/null; then
		WS_JSON="$BODY"
		break
	fi
	sleep 1
done
[ -n "$WS_JSON" ] || { kill "$RUN2_PID" 2>/dev/null || true; fail "no live workspace observed during the routed run — the run never reached the server (routing broken?)"; }
echo "$WS_JSON" | python3 -c 'import json,sys
ws = json.load(sys.stdin)
w = ws[0]
assert w["path"].endswith("repo"), w
assert w["version"], w
print("PASS: live workspace listed mid-run (id=%s path=%s)" % (w["id"][:8], w["path"]))' || fail "workspace shape unexpected: $WS_JSON"
WID="$(echo "$WS_JSON" | python3 -c 'import json,sys; print(json.load(sys.stdin)[0]["id"])')"

SESSIONS="$(probe "/v1/workspaces/$WID/sessions" | tail -n +2)"
echo "$SESSIONS" | python3 -c 'import json,sys
ss = json.load(sys.stdin)
busy = [s for s in ss if s.get("is_busy")]
assert busy, "no busy session mid-run"
s = busy[0]
for k in ("id", "title", "message_count", "prompt_tokens", "is_busy"):
    assert k in s, (k, s)
print("PASS: sessions shape (busy session %s, title=%r)" % (s["id"][:8], s["title"]))' || fail "sessions shape unexpected: $SESSIONS"
SID="$(echo "$SESSIONS" | python3 -c 'import json,sys
ss = json.load(sys.stdin)
print(([s for s in ss if s.get("is_busy")] or ss)[0]["id"])')"

MSGS="$(probe "/v1/workspaces/$WID/sessions/$SID/messages" | tail -n +2)"
echo "$MSGS" | python3 -c 'import json,sys
ms = json.load(sys.stdin)
types = {p["type"] for m in ms for p in m["parts"]}
assert "tool_call" in types, types
for k in ("id", "role", "session_id", "parts"):
    assert k in ms[0], (k, ms[0])
print("PASS: messages shape (parts types: %s)" % sorted(types))' || fail "messages shape unexpected: ${MSGS:0:400}"
wait "$RUN2_PID"
echo "PASS: stalled routed run completed after the probes"

echo "== leg: env-gap canary (two-sided, v0.99.0 — see header)"
start_mock 'printf %s server-env > "$XDG_CONFIG_HOME/env-proof.txt"; printf %s "[$TQ_RESULT_FILE]" > "$TMP/tqrf-seen.txt"' 'mock done' 0
rm -f "$XDG/env-proof.txt" "$TMP/tqrf-seen.txt" "$REPO/smoke-verdict.json"
routed_run run3.log
[ "$(cat "$XDG/env-proof.txt" 2>/dev/null)" = "server-env" ] || fail "server-env side of the canary broke: bash tool no longer sees the server env (behavior changed — investigate!)"
SEEN="$(cat "$TMP/tqrf-seen.txt" 2>/dev/null || echo MISSING)"
if [ "$SEEN" = "[]" ]; then
	echo "PASS (canary): client TQ_RESULT_FILE invisible server-side — the documented v0.99 env gap holds; L1 verdict/question env channels stay caveated (TODO row 1450 note)"
else
	fail "canary flipped: bash tool now sees \$TQ_RESULT_FILE='$SEEN' — upstream fixed env propagation! Flip this leg, drop the L1 caveat rows, re-enable the E-gap legs"
fi

echo "== leg: hooks fire server-side and deny (M3)"
cat >"$TMP/deny-hook.sh" <<'EOF'
#!/bin/sh
input="$(cat)"
case "$input" in
	*hook-deny-probe*)
		echo "BLOCKED: hook-deny-probe is forbidden by the scratch hook fleet" >&2
		exit 2
		;;
esac
exit 0
EOF
chmod +x "$TMP/deny-hook.sh"
cat >"$REPO/.crushrc" <<EOF
permissions allow bash view read write edit
hook add PreToolUse --matcher '^bash\$' --command 'sh $TMP/deny-hook.sh' --name smoke-deny --timeout 5
EOF
start_mock 'echo hook-deny-probe' 'mock done' 0
routed_run run4.log
grep -q "hook-deny-probe is forbidden" "$TMP/run4.log" && echo "PASS: scratch hook denied the routed bash call server-side (M3)" || { cat "$TMP/run4.log"; fail "hook deny marker absent — hooks did not fire server-side"; }
rm -f "$REPO/.crushrc"

echo "== leg: server-kill respawn (M7)"
start_mock 'printf %s "{\"verdict\":\"ok-routed\"}" > smoke-verdict.json' 'mock done' 0
rm -f "$REPO/smoke-verdict.json"
kill -9 "$SERVER_PID" 2>/dev/null || true
SERVER_PID=""
sleep 1
routed_run run5.log
RC=$?
[ "$RC" -eq 0 ] || { cat "$TMP/run5.log"; fail "run after server kill rc=$RC — client did not respawn the server"; }
HEALTH="$(probe /v1/health | head -1)"
echo "$HEALTH" | grep -q "200" || fail "respawned server health: '$HEALTH'"
echo "PASS: killed server respawned by the next client; run + health green"

echo
echo "PASS: crush-server-route smoke — all legs green (env-gap canary holding at the documented v0.99.0 behavior)"
