#!/usr/bin/env bash
# Budget-meter smoke (2026-11-13 done-prompt report §f12): boots
# `tq serve --daily-budget N`, enqueues N tasks, and asserts the rendered
# stats page carries the live budget meter — the CLI flag → webui.Config →
# BudgetView wire, end to end in a real binary (the unit table in
# internal/webui/components_test.go pins the (spent, cap) → (pct, fill,
# tone) derivations; this smoke proves the wiring boots).
#
# TQ_BIN points at a prebuilt binary; unset, the script builds from source.
set -euo pipefail
cd "$(dirname "$0")/../.."
REPO_ROOT="$(pwd)"

TMP="$(mktemp -d)"
SMOKE_FAILED=""
fail_smoke() {
	SMOKE_FAILED=1
	echo "FAIL: $*" >&2
	[ -f "$TMP/serve.log" ] && {
		echo "--- $TMP/serve.log (tail) ---" >&2
		tail -n 25 "$TMP/serve.log" >&2 || true
	}
	exit 1
}
cleanup() {
	[ -n "${SERVE_PID:-}" ] && kill "$SERVE_PID" 2>/dev/null || true
	if [ -n "$SMOKE_FAILED" ] || [ "${BUDGET_SMOKE_KEEP:-}" = "1" ]; then
		echo "TMP kept for autopsy: $TMP" >&2
		return
	fi
	rm -rf "$TMP"
}
trap cleanup EXIT

wait_dashboard() {
	for _ in $(seq 1 50); do
		grep -q "dashboard on" "$TMP/serve.log" 2>/dev/null && return 0
		sleep 0.1
	done
	fail_smoke "tq serve never logged 'dashboard on'"
}

if [ -n "${TQ_BIN:-}" ]; then
	echo "== using prebuilt tq: $TQ_BIN"
	cp "$TQ_BIN" "$TMP/tq"
	chmod +x "$TMP/tq"
else
	echo "== build tq"
	"$REPO_ROOT/scripts/build-tq.sh" "$TMP/tq"
fi

CAP=3
export TQ_DB="$TMP/tasks.db"

PORT="${BUDGET_SMOKE_PORT:-$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1])')}"

echo "== serve with --daily-budget $CAP (hand tailer: the meter reads the store either way)"
"$TMP/tq" serve --addr "127.0.0.1:$PORT" --daily-budget "$CAP" --read-model=false >"$TMP/serve.log" 2>&1 &
SERVE_PID=$!

wait_dashboard

echo "== enqueue $CAP tasks (each enqueue is one spent fact)"
for i in 1 2 3; do
	"$TMP/tq" enqueue --type sh --project budget-smoke --payload "echo $i" || fail_smoke "enqueue $i failed"
done

echo "== assert the rendered meter"
python3 - "$PORT" <<'PYEOF' || fail_smoke "meter asserts"
import sys, time, urllib.request

port = sys.argv[1]
base = f"http://127.0.0.1:{port}"

page = None
for _ in range(30):
    try:
        with urllib.request.urlopen(f"{base}/", timeout=2) as r:
            page = r.read().decode()
        if "budget today 3/3" in page:
            break
    except Exception:
        pass
    time.sleep(0.2)
else:
    print(f"FAIL: meter never showed 3/3 spend")
    sys.exit(1)

for frag in ("card-budget", "budget today 3/3", "tq-meter tq-meter-over",
             "tq-meter-fill tq-meter-fill-100",
             'aria-valuemax="3"', 'aria-valuenow="3"'):
    assert frag in page, f"rendered meter missing {frag}"
print("budget meter OK: card + 3/3 spend + over-cap tone + full fill + raw aria")
PYEOF

echo "smoke ok: budget meter renders live spend at/over the --daily-budget cap"
