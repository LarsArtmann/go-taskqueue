#!/usr/bin/env bash
# Live-socket lockout smoke for `tq api` (TODO row 133): spawn the real
# server on a scratch DB and walk the auth plane end-to-end —
#
#   correct token -> 200            (baseline; also proves server + token)
#   3x wrong token -> 401           (three strikes recorded)
#   next request -> 429 + Retry-After (lockout, even WITH the correct token)
#   after the Retry-After window -> 200 (expiry releases the client)
#
# The scratch TQ_DB export is the production-journal trap guard: the smoke
# must never touch the operator's journal.
set -euo pipefail
cd "$(dirname "$0")/../.."
REPO_ROOT="$(pwd)"

TMP="$(mktemp -d)"
cleanup() {
	for pid in "${API_PID:-}"; do
		[ -n "$pid" ] && kill "$pid" 2>/dev/null || true
	done
	rm -rf "$TMP"
}
trap cleanup EXIT

free_port() {
	python3 - <<'PY'
import socket
with socket.socket() as s:
    s.bind(("127.0.0.1", 0))
    print(s.getsockname()[1])
PY
}

# TQ_BIN points at a prebuilt binary (e.g. the nix-built result/bin/tq);
# unset, the script builds from source with the devmod shim.
if [ -z "${TQ_BIN:-}" ]; then
	"$REPO_ROOT/scripts/build-tq.sh" "$TMP/tq"
	TQ_BIN="$TMP/tq"
fi
echo "== tq binary: $TQ_BIN"

PORT="$(free_port)"
TOKEN="tq-api-smoke-token-$$"
export TQ_DB="$TMP/scratch.db"

"$TQ_BIN" api --addr "127.0.0.1:$PORT" --auth-token "$TOKEN" &
API_PID=$!

# probe METHOD PATH TOKEN -> "code retry_after" on stdout
probe() {
	python3 - "$1" "http://127.0.0.1:$PORT$2" "$3" <<'PY'
import sys, urllib.request, urllib.error

method, url, token = sys.argv[1], sys.argv[2], sys.argv[3]
req = urllib.request.Request(url, method=method)
if token:
    req.add_header("Authorization", "Bearer " + token)
try:
    with urllib.request.urlopen(req, timeout=5) as r:
        print(r.status, r.headers.get("Retry-After", "-"))
except urllib.error.HTTPError as e:
    print(e.code, e.headers.get("Retry-After", "-"))
PY
}

wait_up() {
	for _ in $(seq 1 50); do
		[ "$(probe GET /api/v1/healthz "$TOKEN" | cut -d' ' -f1)" = "200" ] && return 0
		sleep 0.2
	done
	echo "FAIL: tq api never came up on 127.0.0.1:$PORT"
	exit 1
}

wait_up
echo "== server up (scratch DB: $TQ_DB)"

line="$(probe GET /api/v1/healthz "$TOKEN")"
[ "${line%% *}" = "200" ] || { echo "FAIL: correct-token baseline wanted 200, got: $line"; exit 1; }
echo "== PASS: correct token -> 200 (baseline)"

for i in 1 2 3; do
	line="$(probe GET /api/v1/healthz wrong-token-$i)"
	[ "${line%% *}" = "401" ] || { echo "FAIL: strike $i wanted 401, got: $line"; exit 1; }
done
echo "== PASS: 3x wrong token -> 401 (strikes recorded)"

line="$(probe GET /api/v1/healthz "$TOKEN")"
[ "${line%% *}" = "429" ] || { echo "FAIL: locked request wanted 429, got: $line"; exit 1; }
retry="${line#* }"
case "$retry" in
'' | '-' | *[!0-9]*)
	echo "FAIL: 429 without a numeric Retry-After (got '$retry')"
	exit 1
	;;
esac
[ "$retry" -ge 1 ] || { echo "FAIL: Retry-After=$retry, want >= 1"; exit 1; }
echo "== PASS: 429 + Retry-After=$retry (lockout holds even with the correct token)"

echo "== sleeping the lockout window ($retry s + 1 margin)…"
sleep $((retry + 1))

line="$(probe GET /api/v1/healthz "$TOKEN")"
[ "${line%% *}" = "200" ] || { echo "FAIL: post-expiry correct-token request wanted 200, got: $line"; exit 1; }
echo "== PASS: lockout expired -> 200"

echo "PASS: tq api lockout smoke (200 / 3x401 / 429+Retry-After=$retry / expiry 200)"
