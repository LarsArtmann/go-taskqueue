#!/usr/bin/env bash
# Hermetic end-to-end smoke of the rate-limit handling (13:29 report f32):
# a stub agent that fails with the EXACT Z.ai 429 wall output must be
# requeued WITHOUT burning an attempt — through the real CLI binary against
# a scratch DB (TQ_DB exported per the production-trap rule; a bare tq
# command would otherwise touch the PRODUCTION dogfood journal).
#
# 16-00 report f37: the SECOND claim must not happen until not_before.
# The stub's reset timestamp is computed ~5s into the future so the park
# window (reset + 30s claim grace) is assertable in smoke time: a claim
# issued inside the window reaches no agent, and the re-claim fires only
# after the window expires. A past reset would fall back to the 15min
# default backoff — unpinnable in a smoke.
set -euo pipefail
cd "$(dirname "$0")/../.."
REPO_ROOT="$(pwd)"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

if [ -n "${TQ_BIN:-}" ]; then
	echo "== using prebuilt tq: $TQ_BIN"
	cp "$TQ_BIN" "$TMP/tq"
	chmod +x "$TMP/tq"
else
	echo "== build tq"
	"$REPO_ROOT/scripts/build-tq.sh" "$TMP/tq"
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

RESET_AT="$(date -d '+5 seconds' '+%Y-%m-%d %H:%M:%S')"

echo "== stub agent: prints the incident's 429 wall (reset +5s), counts invocations"
STUB="$TMP/stub-agent"
INVOCATIONS="$TMP/invocations"
: >"$INVOCATIONS"
cat >"$STUB" <<EOF
#!/bin/sh
echo run >>"$INVOCATIONS"
cat >&2 <<'OUT'
INFO Running in non-interactive mode
INFO ModelProvider called provider=zai model=glm-5.3-flash
WARN Provider request failed, retrying retry_delay=5s status_code=429 title="too many requests" message="Usage limit reached for 5 hour. Your limit will reset at $RESET_AT"
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
TASK_ID="$("$TMP/tq" facts --json 2>/dev/null | grep -oE '"taskId": ?"[a-f0-9]+"' | head -1 | grep -oE '[a-f0-9]{16,}' || true)"
if [ -z "$TASK_ID" ]; then
	echo "FAIL: could not resolve task id from facts"
	cat "$TMP/worker.log"
	exit 1
fi

"$TMP/tq" show "$TASK_ID" >"$TMP/show.json"
grep -Eq '"status": ?"pending"' "$TMP/show.json" || {
	echo "FAIL: task is not pending after the 429"
	cat "$TMP/show.json"
	exit 1
}
grep -Eq '"attempts": ?0[,}]' "$TMP/show.json" || {
	echo "FAIL: the 429 burned an attempt"
	cat "$TMP/show.json"
	exit 1
}

"$TMP/tq" facts --json >"$TMP/facts.json"
grep -q 'task.requeued' "$TMP/facts.json" || {
	echo "FAIL: no task.requeued fact"
	cat "$TMP/facts.json"
	exit 1
}
grep -Eq '"retry_in_ms": ?"?[1-9]' "$TMP/facts.json" || {
	echo "FAIL: requeued fact lost retry_in_ms"
	grep 'task.requeued' -A1 "$TMP/facts.json"
	exit 1
}

echo "== second claim INSIDE the park window must not reach the agent (16-00 f37)"
timeout 60 "$TMP/tq" worker --agents --once --poll 50ms --lease 5s --task-timeout 30s >"$TMP/worker2.log" 2>&1 || {
	cat "$TMP/worker2.log"
	exit 1
}

RUNS="$(wc -l <"$INVOCATIONS")"
if [ "$RUNS" -ne 1 ]; then
	echo "FAIL: agent ran $RUNS time(s); a claim issued before not_before reached the agent"
	cat "$TMP/worker2.log"
	exit 1
fi

"$TMP/tq" show "$TASK_ID" >"$TMP/show2.json"
grep -Eq '"status": ?"pending"' "$TMP/show2.json" || {
	echo "FAIL: the refused claim mutated the parked task"
	cat "$TMP/show2.json"
	exit 1
}
grep -Eq '"attempts": ?0[,}]' "$TMP/show2.json" || {
	echo "FAIL: the refused claim burned an attempt"
	cat "$TMP/show2.json"
	exit 1
}

echo "== read the park window from the store and wait it out"
NOT_BEFORE="$("$TMP/tq" show "$TASK_ID" | grep -oE '"notBefore": ?"[^"]+"' | head -1 | sed -E 's/.*"notBefore": *"//; s/\.[0-9]+//; s/"$//')"
if [ -z "$NOT_BEFORE" ]; then
	echo "FAIL: could not read notBefore from tq show"
	cat "$TMP/show2.json"
	exit 1
fi

TARGET="$(($(date -d "$NOT_BEFORE" +%s) + 2))"
if [ "$(date +%s)" -ge "$TARGET" ]; then
	echo "FAIL: park window already expired before the refusal could be asserted ($NOT_BEFORE) — window too short to pin f37"
	exit 1
fi

WAITED=0
while [ "$(date +%s)" -lt "$TARGET" ]; do
	sleep 1
	WAITED=$((WAITED + 1))
	if [ "$WAITED" -gt 120 ]; then
		echo "FAIL: not_before ($NOT_BEFORE) not reached after 120s of waiting"
		exit 1
	fi
done

echo "== claim AFTER the window: the agent must run exactly once more"
timeout 60 "$TMP/tq" worker --agents --once --poll 50ms --lease 5s --task-timeout 30s >"$TMP/worker3.log" 2>&1 || {
	cat "$TMP/worker3.log"
	exit 1
}

RUNS="$(wc -l <"$INVOCATIONS")"
if [ "$RUNS" -ne 2 ]; then
	echo "FAIL: agent ran $RUNS time(s) after the window expired, want exactly 2 (the post-window re-claim)"
	cat "$TMP/worker3.log"
	exit 1
fi

"$TMP/tq" show "$TASK_ID" >"$TMP/show3.json"
grep -Eq '"status": ?"pending"' "$TMP/show3.json" || {
	echo "FAIL: task is not pending after the second 429"
	cat "$TMP/show3.json"
	exit 1
}
grep -Eq '"attempts": ?0[,}]' "$TMP/show3.json" || {
	echo "FAIL: the second 429 burned an attempt"
	cat "$TMP/show3.json"
	exit 1
}

echo "PASS: 429 parked the task (pending, attempts 0); a claim inside the window reached no agent; the re-claim fired after not_before"
