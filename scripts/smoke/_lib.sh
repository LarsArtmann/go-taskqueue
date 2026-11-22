#!/usr/bin/env bash
# scripts/smoke/_lib.sh — scratch-repo factory shared by repo-surface smokes
# (extracted from smoke/archive-evidence.sh, 23-40 report f8). Sourcing
# smoke assumes: WORK set to a mktemp dir cleaned by the caller's trap,
# and `set -euo pipefail` already active. bootstrap-install and
# release-gates deliberately do NOT use this factory: their fixtures rely
# on flag-only git identity (-c user.*) as a load-bearing self-test
# premise, which repo-local git config from new_repo() would break.
#
fail() {
	echo "FAIL: $*" >&2
	exit 1
}
pass() { echo "ok: $*"; }

new_repo() {
	local r="$WORK/$1"
	git init -q "$r"
	git -C "$r" config user.name smoke
	git -C "$r" config user.email smoke@example.invalid
	git -C "$r" commit -q --allow-empty -m init
	echo "$r"
}

run_in() { (cd "$1" && "${@:2}"); }

# Scratch-serve helpers (2026-11-13 done-prompt report §e/§f3): boot a tq
# serve dashboard over a scratch journal and wait for readiness, so new
# serve smoke legs cost only their asserts. scratch_serve starts `serve`
# with the caller's args in the background, redirecting to the given log,
# and sets SERVE_PID; wait_dashboard polls the log for the readiness line;
# stop_serve tears the server down. All three take explicit paths — no
# hidden globals beyond SERVE_PID. wait_dashboard RETURNS nonzero on
# timeout (it never exits); callers decide how to fail, so a smoke can
# print its own evidence (serve-log tail) via its wrapper first.
free_port() {
	python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1])'
}

scratch_serve() {
	local bin="$1" log="$2"
	shift 2
	"$bin" serve "$@" >"$log" 2>&1 &
	SERVE_PID=$!
}

wait_dashboard() {
	local log="$1"
	for _ in $(seq 1 50); do
		grep -q "dashboard on" "$log" 2>/dev/null && return 0
		sleep 0.1
	done
	echo "FAIL: tq serve never logged 'dashboard on' (log: $log)" >&2
	return 1
}

stop_serve() {
	if [ -n "${SERVE_PID:-}" ]; then
		python3 -c "import os, signal; os.kill($SERVE_PID, signal.SIGTERM)" 2>/dev/null || true
		SERVE_PID=""
	fi
}

# --self-test: pins the factory contract (fail exits 1, pass prints
# "ok: …", new_repo yields an init-committed repo with identity, run_in
# executes in the repo dir). Defined after the helpers it exercises.
if [ "${1:-}" = "--self-test" ]; then
	WORK=$(mktemp -d)
	trap 'rm -rf "$WORK"' EXIT
	set -euo pipefail
	r=$(new_repo selftest)
	pass "factory self-test start"
	[ -d "$r/.git" ] || fail "new_repo did not create a git repo"
	git -C "$r" log -1 --format=%s | grep -qx init || fail "new_repo repo lacks the init commit"
	git -C "$r" config user.name | grep -qx smoke || fail "new_repo repo lacks smoke identity"
	run_in "$r" git rev-parse --verify HEAD >/dev/null || fail "run_in did not execute in the repo dir"
	fake=$(mktemp "$WORK/fake-tq.XXXXXX")
	printf '#!/usr/bin/env bash\nsleep 0.3\necho "dashboard on 127.0.0.1:9"\nsleep 30\n' >"$fake"
	chmod +x "$fake"
	scratch_serve "$fake" "$WORK/serve.log" --addr "127.0.0.1:0"
	[ -n "$SERVE_PID" ] || fail "scratch_serve did not set SERVE_PID"
	wait_dashboard "$WORK/serve.log" || fail "wait_dashboard missed the readiness line"
	port=$(free_port)
	[ "$port" -ge 1024 ] && [ "$port" -le 65535 ] || fail "free_port returned $port outside the ephemeral range"
	spid=$SERVE_PID
	stop_serve
	[ -z "$SERVE_PID" ] || fail "stop_serve did not clear SERVE_PID"
	if python3 -c "import os; os.kill($spid, 0)" 2>/dev/null; then
		fail "stop_serve left the server alive"
	fi
	pass "scratch-serve helpers boot/wait/stop pinned"
	if (fail "intentional") >/dev/null 2>&1; then
		fail "fail() did not exit nonzero"
	fi
	echo "smoke _lib.sh self-test ok: fail/pass/new_repo/run_in + free_port/scratch_serve/wait_dashboard/stop_serve contract pinned"
	exit 0
fi
