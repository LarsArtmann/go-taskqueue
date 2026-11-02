#!/usr/bin/env bash
# Rewind-replay integration smoke for the status sweeper (TODO row 654;
# 02-03 §f4): enqueue → complete → `tq watermarks set status-sweeper 0`
# (the operator rewind) → re-sweep → assert EXACTLY ONE report task —
# the sweeper's dedup done-guard must turn the replayed completion into a
# skip, not a duplicate report (the unit pin is internal/status
# sweep_test.go's done-guard case; this is the E2E wire over the real
# binary + real journal). CI-safe: scratch TQ_DB, stub agent, no network.
set -euo pipefail
cd "$(dirname "$0")/../.."
REPO_ROOT="$(pwd)"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
export TQ_DB="$TMP/tasks.db"

if [ -n "${TQ_BIN:-}" ]; then
	echo "== using prebuilt tq: $TQ_BIN"
	cp "$TQ_BIN" "$TMP/tq"
	chmod +x "$TMP/tq"
else
	echo "== build tq"
	"$REPO_ROOT/scripts/build-tq.sh" "$TMP/tq"
fi

echo "== seed repo (clean tree, .tq-verify gate, one DONE item)"
REPO="$TMP/repo"
mkdir -p "$REPO"
git -C "$REPO" init -q
git -C "$REPO" config user.email smoke@tq.local
git -C "$REPO" config user.name "tq smoke"
printf -- '- [x] seeded item\n' >"$REPO/TODO_LIST.md"
printf 'test -f TODO_LIST.md\n' >"$REPO/.tq-verify"
git -C "$REPO" add -A
git -C "$REPO" commit -qm seed

echo "== stub agent (the status turn writes one report)"
REPORT_PATH="docs/status/$(date +%Y-%m-%d_%H-%M)_rewind-smoke.md"
export REPORT_PATH
STUB="$TMP/stub-agent"
# pins: the status-reporter branch (report write + footer commit) and the
# commit-free work branch (the nothing-to-commit dead-letter trap)
cat >"$STUB" <<'EOF'
#!/bin/sh
prompt=""
prev=""
for a in "$@"; do
	[ "$prev" = "--" ] && prompt="$a"
	prev="$a"
done
case "$prompt" in
	*"status reporter"*)
		mkdir -p docs/status
		printf '# rewind smoke report\n' >"$REPORT_PATH"
		git add -A
		git commit -qm "docs: rewind smoke report"
		printf '%s\n' "{\"report\":\"$REPORT_PATH\"}" > "$TQ_RESULT_FILE"
		;;
	*)
		# Work branch stays commit-free (a nothing-to-commit git exit 1
		# burns all three attempts and dead-letters the task — the exact
		# failure this smoke's first cut died to).
		printf 'did the work\n'
		;;
esac
EOF
chmod +x "$STUB"
export TQ_AGENT_BIN="$STUB"
cd "$TMP"

echo "== enqueue one agent task + drain with the status sweeper armed"
"$TMP/tq" enqueue --type agent --project smoke --payload "{\"repo\":\"$REPO\",\"prompt\":\"do the work\"}"
timeout 120 "$TMP/tq" agent-pool --repos "$REPO" --status-every 1 --once --poll 50ms --task-timeout 30s >"$TMP/pool1.log" 2>&1 || {
	cat "$TMP/pool1.log"
	exit 1
}
grep -q "status: enqueued report" "$TMP/pool1.log" || {
	cat "$TMP/pool1.log"
	echo "FAIL: sweeper never minted a report"
	exit 1
}

status_task_count() { # completed+any status-type tasks in the journal
	"$TMP/tq" tasks --json -limit 100 | python3 -c '
import json, sys
tasks = json.load(sys.stdin)
if isinstance(tasks, dict):
    tasks = tasks.get("tasks", [])
print(sum(1 for t in tasks if t.get("type") == "status"))'
}

echo "== baseline: exactly one status task exists and is drained"
# The sweeper mint fires on the NEXT tick after the completion, so poll
# until the status task itself completes (max 15s), then count.
for _ in $(seq 1 15); do
	COMPLETED="$("$TMP/tq" stats | awk '/^completed/{print $2}')"
	[ "${COMPLETED:-0}" -ge 2 ] && break
	sleep 1
done
[ "${COMPLETED:-0}" -ge 2 ] || {
	"$TMP/tq" stats
	echo "FAIL: the status task never drained (want >=2 completed)"
	exit 1
}
BEFORE="$(status_task_count)"
[ "$BEFORE" = "1" ] || {
	echo "FAIL: baseline status-task count = $BEFORE, want 1"
	exit 1
}
echo "ok: one status task minted and drained"

echo "== operator rewind: tq watermarks set status-sweeper 0"
"$TMP/tq" watermarks set status-sweeper 0
"$TMP/tq" watermarks show | grep -q 'status-sweeper.*0' || {
	"$TMP/tq" watermarks show
	echo "FAIL: rewind did not land"
	exit 1
}

echo "== re-sweep (rewound journal replays the completion)"
timeout 120 "$TMP/tq" agent-pool --repos "$REPO" --status-every 1 --once --poll 50ms --task-timeout 30s >"$TMP/pool2.log" 2>&1 || {
	cat "$TMP/pool2.log"
	exit 1
}

echo "== assert EXACTLY ONE report task (replay is a dedup skip, not a mint)"
for _ in $(seq 1 10); do
	AFTER="$(status_task_count)"
	[ "$AFTER" = "$BEFORE" ] && break
	sleep 1
done
AFTER="$(status_task_count)"
if [ "$AFTER" != "1" ]; then
	echo "FAIL: rewind replay minted a duplicate report (status tasks: $AFTER, want 1)"
	"$TMP/tq" tasks -limit 20
	exit 1
fi
echo "ok: rewound replay skipped by the done-guard (still exactly one report task)"

echo "== rewind-replay smoke passed"
