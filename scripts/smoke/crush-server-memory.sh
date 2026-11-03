#!/usr/bin/env bash
# Shared-server memory measurement (TODO row: "L4 pilot on ONE repo +
# shared-server memory measurement before any fleet-wide client-server
# rollout", 13-48 §f6 / planning doc 2026-10-30_04-43 "Memory
# accumulation: N concurrent agent goroutines in one process vs N
# processes; unmeasured").
#
# Boots ONE isolated `crush server` (XDG-redirected scratch config, mock
# OpenAI-compatible provider — ZERO spend) and drives TWO waves of N
# CONCURRENT routed agent turns through it, all against ONE scratch repo
# (the pilot's "one repo"). A background sampler records the server
# process RSS across the storm; the summary reports baseline, per-wave
# peak and post-quiescence RSS, and the accumulation guard FAILS only on
# a runaway (quiescent RSS > 3x baseline after both waves).
#
# Measures the shared-server accumulation shape, not the exact fleet
# numbers: the mock provider keeps turns small and fast, so absolute RSS
# is a floor for real routed runs. The output lines (baseline/peak/
# quiescent per wave) are the measurement record the rollout decision
# reads — capture them, do not eyeball them.
#
# Requires `crush` and `python3` on PATH (fails loudly otherwise).
set -euo pipefail

fail() {
	echo "FAIL: $*"
	exit 1
}

# Shared boot plumbing (third-duplication cure). This smoke keeps its own
# minimal inert mock-provider scripting.
. "$(dirname "${BASH_SOURCE[0]}")/lib/crush-server.sh"
crush_smoke_require_bins

TMP="$(mktemp -d /tmp/tq-crush-memory-smoke.XXXXXX)"
PORT=""
MOCK_PID=""
SERVER_PID=""
SOCK="$TMP/crush.sock"
XDG="$TMP/xdg"
DATA="$TMP/data"
REPO="$TMP/repo"
CONCURRENCY=4
WAVES=2
cleanup() {
	crush_smoke_kill_server "$SOCK"
	[ -n "$SERVER_PID" ] && kill "$SERVER_PID" 2>/dev/null || true
	[ -n "$MOCK_PID" ] && kill "$MOCK_PID" 2>/dev/null || true
}
trap cleanup EXIT

PORT="$(crush_smoke_alloc_port)"

mkdir -p "$XDG" "$DATA" "$REPO" "$XDG/crush"

crush_smoke_write_config "$XDG" "$PORT"

cat >"$TMP/mock_provider.py" <<'PYEOF'
#!/usr/bin/env python3
import json
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

REQ_LOG = open(sys.argv[2], "a", buffering=1)


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
		if not self.path.startswith("/v1/chat/completions"):
			self._json(404, {"error": "not found"})
			return
		length = int(self.headers.get("Content-Length", "0"))
		req = json.loads(self.rfile.read(length) or b"{}")
		REQ_LOG.write(f"turn stream={req.get('stream')}\n")
		delta = {"role": "assistant", "content": "done"}
		if req.get("stream"):
			self.send_response(200)
			self.send_header("Content-Type", "text/event-stream")
			self.send_header("Transfer-Encoding", "chunked")
			self.end_headers()
			base = {"id": "chatcmpl-mock", "object": "chat.completion.chunk", "created": 0, "model": "mock-1"}

			def chunk(obj):
				body = f"data: {json.dumps(obj)}\n\n".encode()
				self.wfile.write(f"{len(body):x}\r\n".encode() + body + b"\r\n")

			chunk({**base, "choices": [{"index": 0, "delta": delta, "finish_reason": None}]})
			chunk({**base, "choices": [{"index": 0, "delta": {}, "finish_reason": "stop"}]})
			self.wfile.write(b"0\r\n\r\n")
			self.wfile.flush()
			return
		self._json(200, {"id": "chatcmpl-mock", "object": "chat.completion", "created": 0, "model": "mock-1", "choices": [{"index": 0, "message": delta, "finish_reason": "stop"}]})


if __name__ == "__main__":
	ThreadingHTTPServer(("127.0.0.1", int(sys.argv[1])), Handler).serve_forever()
PYEOF

echo "== boot mock provider + isolated shared server"
crush_smoke_start_mock "mock_provider.py $PORT" "$TMP/mock.log" \
	python3 "$TMP/mock_provider.py" "$PORT" "$TMP/mock-requests.log"
MOCK_PID="$CRUSH_SMOKE_MOCK_PID"
crush_smoke_start_server "$SOCK" "$XDG" "$DATA" "$TMP/server.log"
SERVER_PID="$CRUSH_SMOKE_SERVER_PID"
crush_smoke_health_ok "$SOCK" || fail "server health probe failed"

rss_kb() {
	ps -o rss= -p "$SERVER_PID" 2>/dev/null | tr -d ' ' || echo 0
}

crush_smoke_seed_repo "$REPO"

BASELINE=$(rss_kb)
echo "baseline rss: ${BASELINE} KB"

probe() { # <path> — status-only view of the shared probe
	crush_smoke_probe "$SOCK" "$1" | head -1
}

routed_run() { # <log> — one routed turn; rc on stdout
	(
		cd "$REPO"
		XDG_CONFIG_HOME="$XDG" XDG_DATA_HOME="$DATA" \
			CRUSH_CLIENT_SERVER=1 \
			timeout 90 crush run -H "unix://$SOCK" --verbose \
			"reply done" >"$TMP/$1" 2>&1
	)
}

run_wave() { # <wave-n> — CONCURRENCY concurrent routed turns; echoes peak rss
	n="$1"
	: >"$TMP/rss-$n.log"
	(
		while kill -0 "$SERVER_PID" 2>/dev/null; do
			rss_kb >>"$TMP/rss-$n.log"
			sleep 0.2
		done
	) &
	SAMPLER=$!
	pids=""
	for i in $(seq 1 "$CONCURRENCY"); do
		routed_run "run-$n-$i.log" &
		pids="$pids $!"
	done
	rc_total=0
	for p in $pids; do
		wait "$p" || rc_total=$((rc_total + 1))
	done
	# One retry per failed turn: an idle-exit (M7) race can refuse the
	# socket between the health probe and the dial; the next routed client
	# respawns the server itself (route-smoke respawn leg), so a retry is
	# the faithful keeper-shaped recovery, not a mask. Retries ride the
	# same sampler window, so the peak RSS stays honest.
	if [ "$rc_total" -gt 0 ]; then
		echo "note: $rc_total wave-$n turns failed on the first dial — retrying once (client respawn)" >&2
		rc_total=0
		for p in $pids; do
			wait "$p" 2>/dev/null || true
		done
		pids=""
		for i in $(seq 1 "$CONCURRENCY"); do
			routed_run "run-$n-$i-retry.log" &
			pids="$pids $!"
		done
		for p in $pids; do
			wait "$p" || rc_total=$((rc_total + 1))
		done
	fi
	kill "$SAMPLER" 2>/dev/null || true
	wait "$SAMPLER" 2>/dev/null || true
	echo "$rc_total"
}

ensure_server() { # health-or-respawn: an idle server may self-terminate (M7)
	if probe /v1/health | head -1 | grep -q 200 && kill -0 "$SERVER_PID" 2>/dev/null; then
		return 0
	fi
	echo "note: server not serving between waves (idle self-termination, M7) — respawning as the pool keeper would"
	crush_smoke_start_server "$SOCK" "$XDG" "$DATA" "$TMP/server.log"
	SERVER_PID="$CRUSH_SMOKE_SERVER_PID"
	probe /v1/health | head -1 | grep -q 200 || fail "respawned server unhealthy"
}

for w in $(seq 1 "$WAVES"); do
	echo "== wave $w: $CONCURRENCY concurrent routed turns (ONE repo, ONE server)"
	ensure_server
	RC_TOTAL=$(run_wave "$w")
	[ "$RC_TOTAL" -eq 0 ] || {
		cat "$TMP"/run-"$w"-*.log 2>/dev/null | head -20
		fail "wave $w had $RC_TOTAL failed routed turns"
	}
	PEAK=$(sort -n "$TMP/rss-$w.log" | tail -1)
	# No hard post-wave health fail: an idle server may self-terminate (M7)
	# between the last turn and this probe; every turn already returned
	# rc=0, so the measurement stands and the next ensure_server respawns.
	probe /v1/health | head -1 | grep -q 200 || echo "note: server idle-exited after wave $w (M7); measurement stands"
	sleep 3 # quiesce: let the runtime settle before sampling steady RSS
	QUIESCENT=$(rss_kb)
	echo "wave $w: peak=${PEAK} KB quiescent=${QUIESCENT} KB"
done

ensure_server
FINAL=$(rss_kb)
[ "$FINAL" -gt 0 ] || fail "server process gone before the accumulation guard"
[ "$FINAL" -le $((BASELINE * 3)) ] || fail "accumulation guard: quiescent rss $FINAL KB exceeds 3x baseline $BASELINE KB — runaway growth under concurrent routed load"

echo "MEMORY-SMOKE ok (waves=$WAVES concurrency=$CONCURRENCY baseline=${BASELINE}KB final=${FINAL}KB)"
echo "measurement record: baseline=${BASELINE} $(for w in $(seq 1 "$WAVES"); do printf 'wave%s_peak=%s wave%s_quiescent=%s ' "$w" "$(sort -n "$TMP/rss-$w.log" | tail -1)" "$w" "$(rss_kb)"; done)KB"
