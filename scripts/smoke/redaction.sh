#!/usr/bin/env bash
# Secrets-pass smoke (TODO row 134): a stub agent leaks fake provider
# tokens on BOTH outcomes and the stored forensics must be REDACTED —
#
#   success turn: the sidecar log (TQ_LOG_DIR) carries [REDACTED], never
#   the raw token shapes
#   failure turn: the task.failed evidence tail renders [REDACTED] on the
#   task surface (tq show lastError/facts)
#   `tq audit --journal` reports ZERO SECRET EVIDENCE after the pass
#
# Scratch DB everywhere (production-journal trap guard); scratch repo; no
# network, no API spend. Four --once drains with backoff-sized sleeps:
# complete the ok turn and burn the failing task's three attempts
# (ExpBackoff 2s/4s/… between them).
set -euo pipefail
cd "$(dirname "$0")/../.."
REPO_ROOT="$(pwd)"

TMP="$(mktemp -d)"
cleanup() {
	# TQ_SMOKE_KEEP=1 keeps the scratch tree for forensics on a failure.
	if [ "${TQ_SMOKE_KEEP:-0}" = "1" ]; then
		echo "== TQ_SMOKE_KEEP=1: scratch tree left at $TMP"
		return
	fi
	rm -rf "$TMP"
}
trap cleanup EXIT

if [ -n "${TQ_BIN:-}" ]; then
	echo "== using prebuilt tq: $TQ_BIN"
	cp "$TQ_BIN" "$TMP/tq"
	chmod +x "$TMP/tq"
else
	echo "== build tq"
	"$REPO_ROOT/scripts/build-tq.sh" "$TMP/tq"
fi

export TQ_DB="$TMP/tasks.db"
export TQ_LOG_DIR="$TMP/logs"
mkdir -p "$TQ_LOG_DIR"

# Clearly-fake token shapes that MATCH the redaction patterns (16+ chars
# after sk-ant-, AKIA+16) — never real credentials.
FAKE_ANTHROPIC="sk-ant-SMOKETEST-0000000000000000"
FAKE_AWS="AKIAIOSFODNN7EXAMPLE"

echo "== scratch repo (one completable item, one doomed item)"
REPO="$TMP/repo"
mkdir -p "$REPO"
git -C "$REPO" init -q
git -C "$REPO" config user.email smoke@tq.local
git -C "$REPO" config user.name "tq smoke"
printf '%s\n' \
	'- [ ] stub redaction-ok item. Commit with footer: Task-Queue-ID: {{TASK_ID}}' \
	'- [ ] stub redaction-fail item. Commit with footer: Task-Queue-ID: {{TASK_ID}}' \
	>"$REPO/TODO_LIST.md"
printf 'test -f work.txt\n' >"$REPO/.tq-verify"
git -C "$REPO" add -A
git -C "$REPO" commit -qm "seed"

echo "== stub agent: leaks fake tokens on both outcomes"
STUB="$TMP/stub-agent"
cat >"$STUB" <<EOF
#!/bin/sh
prompt=""
prev=""
for a in "\$@"; do
	[ "\$prev" = "--" ] && prompt="\$a"
	prev="\$a"
done
case "\$prompt" in
	*"redaction-fail"*)
		echo "fail turn leaked $FAKE_ANTHROPIC and $FAKE_AWS"
		exit 1
		;;
	*)
		qid="\$(printf '%s\n' "\$prompt" | grep -o 'Task-Queue-ID: [0-9a-f]*' | head -1 | cut -d' ' -f2)"
		echo "ok turn authenticated with $FAKE_ANTHROPIC / $FAKE_AWS"
		echo done >work.txt
		sed -i 's/^- \[ \] stub redaction-ok item.*/- [x] stub redaction-ok item/' TODO_LIST.md
		git add -A
		git commit -qm "stub redaction work" -m "Task-Queue-ID: \$qid"
		printf '%s\n' 'TQ_RESULT: {"summary":"stub ok"}'
		;;
esac
EOF
chmod +x "$STUB"
export TQ_AGENT_BIN="$STUB"

cd "$TMP"

echo "== four --once drains: complete the ok turn, exhaust the doomed one"
for i in 1 2 3 4; do
	timeout 120 "$TMP/tq" agent-pool --projects-dir "$TMP" --repos repo \
		--once --max-per-tick 1 --poll 50ms --task-timeout 30s \
		>"$TMP/pool-$i.log" 2>&1 || {
		cat "$TMP/pool-$i.log"
		exit 1
	}
	sleep 5
done

echo "== locate the two agent tasks"
IDS="$("$TMP/tq" tasks -json -limit 10 | python3 -c '
import json, sys
tasks = json.load(sys.stdin)
if isinstance(tasks, dict):
    tasks = tasks.get("tasks", [])
agent = [t for t in tasks if t.get("type") == "agent"]
ok = [t["id"] for t in agent if "redaction-ok" in t.get("payload", {}).get("item", "")]
fail = [t["id"] for t in agent if "redaction-fail" in t.get("payload", {}).get("item", "")]
if not ok or not fail:
    sys.exit("wanted one ok and one fail agent task, got %d/%d" % (len(ok), len(fail)))
print(ok[0], fail[0])
')"
OK_ID=${IDS%% *}
FAIL_ID=${IDS##* }
echo "== ok task: $OK_ID, fail task: $FAIL_ID"

echo "== assert the ok turn completed and the fail turn is dead"
"$TMP/tq" stats | tee "$TMP/stats.out"
grep -Eq '^completed\s+1$' "$TMP/stats.out" || {
	echo "FAIL: ok turn did not complete"
	exit 1
}
grep -Eq '^dead\s+1$' "$TMP/stats.out" || {
	echo "FAIL: fail turn is not dead"
	exit 1
}

echo "== assert the sidecar is redacted"
SIDECAR="$TQ_LOG_DIR/$OK_ID.log"
[ -f "$SIDECAR" ] || {
	echo "FAIL: sidecar $SIDECAR missing (TQ_LOG_DIR ignored?)"
	exit 1
}
if grep -qE "$FAKE_ANTHROPIC|$FAKE_AWS" "$SIDECAR"; then
	echo "FAIL: sidecar carries a RAW token"
	exit 1
fi
grep -q '\[REDACTED\]' "$SIDECAR" || {
	echo "FAIL: sidecar has no [REDACTED] marker — the pass never ran"
	exit 1
}
echo "== PASS: sidecar redacted"

echo "== assert the failing turn's evidence tail is redacted on the task surface"
FAIL_SHOW="$("$TMP/tq" show "$FAIL_ID")"
if printf '%s' "$FAIL_SHOW" | grep -qE "$FAKE_ANTHROPIC|$FAKE_AWS"; then
	echo "FAIL: failing task surface carries a RAW token"
	exit 1
fi
printf '%s' "$FAIL_SHOW" | grep -q '\[REDACTED\]' || {
	echo "FAIL: failing task surface has no [REDACTED] marker"
	exit 1
}
echo "== PASS: failure evidence redacted"

echo "== assert tq audit --journal reports ZERO SECRET EVIDENCE"
AUDIT="$("$TMP/tq" audit --journal 2>&1)"
if printf '%s\n' "$AUDIT" | grep -q "SECRET EVIDENCE"; then
	echo "FAIL: audit still reports SECRET EVIDENCE after the redaction pass:"
	printf '%s\n' "$AUDIT" | grep -A5 "SECRET EVIDENCE"
	exit 1
fi
echo "== PASS: zero SECRET EVIDENCE"

echo "PASS: redaction smoke (sidecar + failure evidence redacted, audit clean)"
