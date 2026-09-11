#!/usr/bin/env bash
# Hermetic end-to-end smoke of the rate-limit handling (13:29 report f32):
# a stub agent that fails with the EXACT Z.ai 429 wall output must be
# requeued WITHOUT burning an attempt — through the real CLI binary against
# a scratch DB (TQ_DB exported per the production-trap rule; a bare tq
# command would otherwise touch the PRODUCTION dogfood journal).
set -euo pipefail
cd "$(dirname "$0")/../.."

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

if [ -n "${TQ_BIN:-}" ]; then
	echo "== using prebuilt tq: $TQ_BIN"
	cp "$TQ_BIN" "$TMP/tq"
	chmod +x "$TMP/tq"
else
	echo "== build tq"
	go build -o "$TMP/tq" ./cmd/tq
fi

export TQ_DB="$TMP/tasks.db"

echo "== seed repo (clean tree, .tq-verify gate)"
REPO="$TMP/repo"
mkdir -p "$REPO"
git -C "$REPO" init -q
git -C "$REPO" config user.email smoke@tq.local
git -C "$REPO" config user.name "tq smoke"
printf -- '- [x] seeded item\n' >"$REPO/TODO_LIST.md"
printf 'test -f TODO_LIST.md\n' >"$REPO/.tq-verify"
git -C "$REPO" add -A
git -C "$REPO" commit -qm "seed"

echo "== stub agent: prints the incident's 429 wall and exits 1"
STUB="$TMP/stub-agent"
cat >"$STUB" <<'EOF'
#!/bin/sh
cat >&2 <<'OUT'
INFO Running in non-interactive mode
INFO ModelProvider called provider=zai model=glm-5.3-flash
WARN Provider request failed, retrying retry_delay=5s status_code=429 title="too many requests" message="Usage limit reached for 5 hour. Your limit will reset at 2026-09-11 19:40:34"
OUT
exit 1
EOF
chmod +x "$STUB"
export TQ_AGENT_BIN="$STUB"

echo "== enqueue one agent task"
"$TMP/tq" enqueue --type agent --project smoke --payload "{\"repo\":\"$REPO\",\"prompt\":\"do work\"}"

echo "== run the worker --once (task must park, not die)"
timeout 60 "$TMP/tq" worker --agents --once --poll 50ms --lease 5s --task-timeout 30s >"$TMP/worker.log" 2>&1 || {
	cat "$TMP/worker.log"
	exit 1
}

echo "== assert the task parked: pending, attempts 0, requeued fact carries retry_in_ms"
TASK_ID="$("$TMP/tq" facts --json 2>/dev/null | grep -oE '"task_id":"[a-f0-9]+"' | head -1 | cut -d'"' -f4)"
if [ -z "$TASK_ID" ]; then
	echo "FAIL: could not resolve task id from facts"
	cat "$TMP/worker.log"
	exit 1
fi

"$TMP/tq" show "$TASK_ID" >"$TMP/show.json"
grep -q '"status": *"pending"\|"status":"pending"' "$TMP/show.json" || {
	echo "FAIL: task is not pending after the 429"
	cat "$TMP/show.json"
	exit 1
}
grep -q '"attempts": *0\|"attempts":0' "$TMP/show.json" || {
	echo "FAIL: the 429 burned an attempt"
	cat "$TMP/show.json"
	exit 1
}

"$TMP/tq" facts --json >"$TMP/facts.json" || "$TMP/tq" facts >"$TMP/facts.json"
grep -q 'task.requeued' "$TMP/facts.json" || {
	echo "FAIL: no task.requeued fact"
	cat "$TMP/facts.json"
	exit 1
}
grep -Eq '"retry_in_ms": *"?[1-9]' "$TMP/facts.json" || {
	echo "FAIL: requeued fact lost retry_in_ms"
	cat "$TMP/facts.json"
	exit 1
}

echo "PASS: 429 agent failure parked the task (pending, attempts 0, retry_in_ms > 0)"
