#!/usr/bin/env bash
# Shared boot block for the crush client/server smokes (crush-server-route.sh,
# crush-server-memory.sh, crush-pool-supervisor.sh). SOURCE this file — never
# execute it. Every function calls the sourcer's `fail` on refusal, so the
# smoke must define fail() before sourcing.
#
# What lives here: the third-duplication boot plumbing — bin checks, port
# allocation (bind 0, never a pinned port: the zombie-mock hijack class),
# the scratch crush.json with the mock as the ONLY resolvable provider, the
# crush dirs isolation gate, the seeded scratch repo, mock/server start-stop,
# and the de-chunked unix-socket GET probe.
#
# What deliberately does NOT live here: each smoke's mock-provider SCRIPTING
# (verdict tool_call, 429 mode, stall branches) and — above all — the route
# smoke's header-carrying chunk helper, kept verbatim in that smoke: the
# raw-SSE bug class (an SSE body read without the header's chunked framing)
# was only debugged to green in place; do not "simplify" it from here.
#
# Convention: CRUSH_SMOKE_* globals are set by the start helpers so a
# smoke's cleanup trap can kill exactly what was booted.

# crush_smoke_require_bins — both binaries the boot block shells out to.
crush_smoke_require_bins() {
	command -v crush >/dev/null 2>&1 ||
		fail "crush binary not on PATH — this smoke routes real crush runs through a scratch server"
	command -v python3 >/dev/null 2>&1 ||
		fail "python3 required (mock provider + HTTP probes)"
}

# crush_smoke_alloc_port — a free 127.0.0.1 port via bind(0), closed before
# the mock races for it (never a fixed port).
crush_smoke_alloc_port() {
	python3 -c 'import socket
s = socket.socket()
s.bind(("127.0.0.1", 0))
print(s.getsockname()[1])
s.close()'
}

# crush_smoke_write_config <xdg> <port> [extra-options-json]
# Writes <xdg>/crush/crush.json pointing at the mock and parse-checks it.
# The extra fragment (omit or pass "") is spliced into the options object
# after disable_default_providers (e.g. '"disable_auto_summarize": true').
crush_smoke_write_config() {
	local xdg=$1 port=$2 extra=${3:-}
	local options='{"disable_default_providers": true'
	[ -n "$extra" ] && options="$options, $extra"
	options="$options}"

	mkdir -p "$xdg/crush"

	cat >"$xdg/crush/crush.json" <<EOF
{
  "\$schema": "https://charm.land/crush.json",
  "providers": {
    "mock": {
      "api_key": "smoke-key",
      "base_url": "http://127.0.0.1:$port/v1",
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
  "options": $options
}
EOF
	python3 -c 'import json,sys; json.load(open(sys.argv[1]))' "$xdg/crush/crush.json" ||
		fail "scratch crush.json does not parse"
}

# crush_smoke_assert_isolation <xdg> <data> — crush dirs under the candidate
# env must resolve the scratch config dir FIRST LINE, else nothing may run.
# crush dirs prints its report on stderr (a 2>/dev/null swallow reads as
# "no output" — proven the hard way), and head -1 SIGPIPEs the writer under
# pipefail — the || true is load-bearing.
crush_smoke_assert_isolation() {
	local xdg=$1 data=$2 line
	line="$(XDG_CONFIG_HOME="$xdg" XDG_DATA_HOME="$data" crush dirs 2>&1 | head -1 || true)"
	[ "$line" = "$xdg/crush" ] ||
		fail "crush dirs first line is '$line', want the scratch config dir '$xdg/crush' — isolation NOT proven, refusing to run anything"
}

# crush_smoke_seed_repo <repo> — a one-commit scratch git repo.
crush_smoke_seed_repo() {
	local repo=$1
	git -C "$repo" init -q
	git -C "$repo" config user.email smoke@tq.local
	git -C "$repo" config user.name "tq smoke"
	printf 'scratch\n' >"$repo/f.txt"
	git -C "$repo" add -A
	git -C "$repo" commit -qm seed
}

# crush_smoke_start_mock <pkill-pattern> <logfile> <cmd...> — kill any
# previous instance of THIS mock, start it, prove it survived startup.
# Sets CRUSH_SMOKE_MOCK_PID.
crush_smoke_start_mock() {
	local pattern=$1 log=$2
	shift 2

	pkill -9 -f "$pattern" 2>/dev/null || true
	sleep 0.3

	"$@" >"$log" 2>&1 &
	CRUSH_SMOKE_MOCK_PID=$!
	sleep 1
	kill -0 "$CRUSH_SMOKE_MOCK_PID" 2>/dev/null || {
		cat "$log"
		fail "mock provider died on startup"
	}
}

# crush_smoke_start_server <sock> <xdg> <data> <logfile> [setsid]
# Boots one isolated crush server on the scratch unix socket. setsid (the
# pool smoke's supervisor legs need the server in its own session so a
# kill -9 keeper leg cannot take the smoke's process group down).
# Sets CRUSH_SMOKE_SERVER_PID.
crush_smoke_start_server() {
	local sock=$1 xdg=$2 data=$3 log=$4 mode=${5:-}

	crush_smoke_kill_server "$sock"

	if [ "$mode" = "setsid" ]; then
		XDG_CONFIG_HOME="$xdg" XDG_DATA_HOME="$data" \
			setsid crush server --host "unix://$sock" >"$log" 2>&1 &
	else
		XDG_CONFIG_HOME="$xdg" XDG_DATA_HOME="$data" \
			crush server --host "unix://$sock" >"$log" 2>&1 &
	fi

	CRUSH_SMOKE_SERVER_PID=$!
	sleep 2
}

# crush_smoke_kill_server <sock> — kill exactly this scratch socket's
# server (client-spawned servers are not our child; the pattern is the
# only safe handle).
crush_smoke_kill_server() {
	pkill -9 -f "crush server --host unix://$1" 2>/dev/null || true
}

# crush_smoke_probe <sock> <path> — GET on the scratch unix socket; prints
# "<status line>\n<de-chunked body>". De-chunking reads the transfer-encoded
# framing INSIDE this helper; the route smoke's SSE capture logic (which
# must see raw framing) deliberately does NOT go through here.
crush_smoke_probe() {
	python3 - "$1" "$2" <<'PYEOF'
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

# crush_smoke_health_ok <sock> — 200 check on /v1/health.
crush_smoke_health_ok() {
	crush_smoke_probe "$1" /v1/health | head -1 | grep -q 200
}
