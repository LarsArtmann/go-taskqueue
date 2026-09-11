#!/usr/bin/env bash
# Dogfood-once smoke: replicates the 2026-09-10 --once proof
# (docs/status/2026-09-10_01-55) — one pool tick: harvest → agent work
# turn (TODO item ticked, commit with Task-Queue-ID footer) → review turn
# (approve verdict) → drained, exit.
#
# Default (ungated) mode runs against a scratch repo with a stub agent —
# no network, no API spend, CI-safe.
# With TQ_DOGFOOD=1 it additionally runs the REAL proof against THIS repo
# with the real crush agent from a foreign cwd and a scratch DB — that
# spends API money, hence the gate. Never run it against the production
# journal: the script pins TQ_DB to a scratch path in both modes.
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
	GOEXPERIMENT=jsonv2 go build -o "$TMP/tq" ./cmd/tq
fi

export TQ_DB="$TMP/tasks.db"

echo "== scratch repo (clean tree, .tq-verify gate, one open item)"
REPO="$TMP/repo"
mkdir -p "$REPO"
git -C "$REPO" init -q
git -C "$REPO" config user.email smoke@tq.local
git -C "$REPO" config user.name "tq smoke"
printf '%s\n' '- [ ] stub dogfood work item. Commit with footer: Task-Queue-ID: {{TASK_ID}}' >"$REPO/TODO_LIST.md"
printf 'test -f work.txt\n' >"$REPO/.tq-verify"
git -C "$REPO" add -A
git -C "$REPO" commit -qm "seed"

echo "== stub agent (work turn vs review turn)"
STUB="$TMP/stub-agent"
cat >"$STUB" <<'EOF'
#!/bin/sh
prompt=""
prev=""
for a in "$@"; do
	[ "$prev" = "--" ] && prompt="$a"
	prev="$a"
done
case "$prompt" in
	*"senior code reviewer"*)
		printf 'looks fine\n'
		printf '%s\n' 'TQ_RESULT: {"verdict":"approve","summary":"stub review","findings":[]}'
		;;
	*)
		qid="$(printf '%s\n' "$prompt" | grep -o 'Task-Queue-ID: [0-9a-f]*' | head -1 | cut -d' ' -f2)"
		echo done >work.txt
		sed -i 's/^- \[ \] stub dogfood work item.*$/- [x] stub dogfood work item/' TODO_LIST.md
		git add -A
		git commit -qm "stub dogfood work" -m "Task-Queue-ID: $qid"
		printf 'did the work\n'
		printf '%s\n' 'TQ_RESULT: {"summary":"stub work"}'
		;;
esac
EOF
chmod +x "$STUB"
export TQ_AGENT_BIN="$STUB"

echo "== one pool tick: harvest -> work -> review (bare name + foreign cwd,"
echo "== as in the proof; --projects-dir exercises the name expansion fix)"
cd "$TMP"
timeout 120 "$TMP/tq" agent-pool --projects-dir "$TMP" --repos repo \
	--review --once --max-per-tick 1 --poll 50ms --task-timeout 30s \
	>"$TMP/pool.log" 2>&1 || {
	cat "$TMP/pool.log"
	exit 1
}
cat "$TMP/pool.log"

echo "== assert the review turn ran and the queue drained"
grep -q "review: enqueued" "$TMP/pool.log" || {
	echo "FAIL: sweeper never minted a review"
	exit 1
}
STATS="$("$TMP/tq" stats)"
echo "$STATS"
echo "$STATS" | grep -Eq '^completed\s+2$' || {
	echo "FAIL: want 2 completed (agent + review)"
	exit 1
}
if echo "$STATS" | grep -Eq '^(pending|dead)\s+[1-9]'; then
	echo "FAIL: queue did not drain cleanly"
	exit 1
fi

echo "== assert the work artifacts: ticked item + Task-Queue-ID commit"
grep -q -- '- \[x\] stub dogfood work item' "$REPO/TODO_LIST.md" || {
	echo "FAIL: TODO item not ticked"
	exit 1
}
git -C "$REPO" log -1 --format=%B | grep -q '^Task-Queue-ID: ' || {
	echo "FAIL: work commit carries no Task-Queue-ID footer"
	exit 1
}

echo "== assert the sweeper checkpoint is at head"
"$TMP/tq" doctor | tee "$TMP/doctor.out"
grep -Eq '^ok +review-sweeper' "$TMP/doctor.out" || {
	echo "FAIL: review-sweeper watermark not at head"
	exit 1
}

echo "== dogfood-once stub smoke passed"

if [ "${TQ_DOGFOOD:-}" = "1" ]; then
	echo "== TQ_DOGFOOD=1: real proof (spends API money) — THIS repo, real agent"
	echo "== scratch DB already pinned via TQ_DB; foreign cwd as in the proof"
	if [ ! -f "$HOME/projects/go-taskqueue/.crushrc" ]; then
		echo "FAIL: repo .crushrc missing — pool --yolo would fail fast"
		exit 1
	fi
	timeout 1800 "$TMP/tq" agent-pool --projects-dir "$HOME/projects" \
		--repos go-taskqueue --yolo --review --once --max-per-tick 1 \
		--allow-dirty --task-timeout 15m 2>&1 | tee "$TMP/real-pool.log"
	echo "== real dogfood once-run finished; inspect $TMP/real-pool.log and $TQ_DB"
fi
