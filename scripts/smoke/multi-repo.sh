#!/usr/bin/env bash
# Multi-repo live smoke: two agent-pool processes, one shared database,
# three repos — proves the per-project exclusivity and dedup guarantees
# end to end with a stub agent (zero API cost).
#
# Assertions (D24):
#   1. every open item is enqueued exactly once (no double-enqueue)
#   2. every task completes exactly once (one claim per task, no dead)
#   3. both pools actually claimed work
set -uo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
WORK="$(mktemp -d)"
TQ="$WORK/tq"
KEEP="${SMOKE_KEEP:-0}"
cleanup() { [ "$KEEP" = 1 ] && echo "workdir kept: $WORK" || rm -rf "$WORK"; }
trap cleanup EXIT

# TQ_BIN points at a prebuilt binary (e.g. the nix-built result/bin/tq);
# unset, the script builds from source with `go build`.
if [ -n "${TQ_BIN:-}" ]; then
	echo "== using prebuilt tq: $TQ_BIN"
	cp "$TQ_BIN" "$TQ"
	chmod +x "$TQ"
else
	echo "== building tq =="
	(cd "$REPO_ROOT" && go build -o "$TQ" ./cmd/tq) || exit 1
fi

STUB="$WORK/stub-agent"
printf '#!/bin/sh\nsleep 0.2\nexit 0\n' >"$STUB"
chmod +x "$STUB"
export TQ_AGENT_BIN="$STUB"

echo "== fixtures: 3 repos, 2 items each (D22) =="
DB="$WORK/shared.db"
for r in alpha beta gamma; do
	mkdir -p "$WORK/$r"
	{
		echo "## Work"
		echo
		echo "- [ ] $r item one"
		echo "- [ ] $r item two"
	} >"$WORK/$r/TODO_LIST.md"
done

echo "== two pools over the same DB, two rounds (D23) =="
# The harvester paces itself: one NEW item per repo per tick, so two items
# per repo need two rounds of --once.
DB="$WORK/shared.db"
for round in 1 2; do
	(cd "$WORK" && timeout 60 "$TQ" agent-pool --repos "$WORK/alpha,$WORK/beta,$WORK/gamma" --db "$DB" --project-exclusive --concurrency 1 --poll 20ms --once --owner pool-1 >"$WORK/pool1-$round.log" 2>&1) &
	P1=$!
	(cd "$WORK" && timeout 60 "$TQ" agent-pool --repos "$WORK/alpha,$WORK/beta,$WORK/gamma" --db "$DB" --project-exclusive --concurrency 1 --poll 20ms --once --owner pool-2 >"$WORK/pool2-$round.log" 2>&1) &
	P2=$!
	wait "$P1"
	R1=$?
	wait "$P2"
	R2=$?
	if [ "$R1" != 0 ] || [ "$R2" != 0 ]; then
		echo "FAIL: round $round exit codes: pool-1=$R1 pool-2=$R2"
		echo "--- pool1-$round.log ---"
		tail -n 8 "$WORK/pool1-$round.log"
		echo "--- pool2-$round.log ---"
		tail -n 8 "$WORK/pool2-$round.log"
		exit 1
	fi
done

fail=0
count_facts() { "$TQ" facts --db "$DB" | grep -c "$1" || true; }

ENQ=$(count_facts "task.enqueued")
DONE=$(count_facts "task.completed")
CLAIMS=$(count_facts "task.claimed")
DEADS=$(count_facts "task.dead-lettered")
P1C=$("$TQ" facts --db "$DB" | grep -c "task.claimed.*pool-1" || true)
P2C=$("$TQ" facts --db "$DB" | grep -c "task.claimed.*pool-2" || true)

echo "== assertions (D24) =="
echo "enqueued=$ENQ completed=$DONE claims=$CLAIMS dead=$DEADS (pool-1 claims=$P1C pool-2 claims=$P2C)"

[ "$ENQ" = 6 ] || {
	echo "FAIL: want 6 enqueued (one per item), got $ENQ — double-enqueue?"
	fail=1
}
[ "$DONE" = 6 ] || {
	echo "FAIL: want 6 completed, got $DONE"
	fail=1
}
[ "$DEADS" = 0 ] || {
	echo "FAIL: want 0 dead-lettered, got $DEADS"
	fail=1
}
# Exactly one claim per task: 6 tasks → 6 claims (no retry, no co-run).
[ "$CLAIMS" = 6 ] || {
	echo "FAIL: want 6 claims (one per task), got $CLAIMS — co-run or retry?"
	fail=1
}
[ "$P1C" -ge 1 ] && [ "$P2C" -ge 1 ] || {
	echo "FAIL: both pools must claim (pool-1=$P1C pool-2=$P2C)"
	fail=1
}

if [ "$fail" = 0 ]; then
	echo "MULTI-REPO SMOKE OK"
else
	echo "MULTI-REPO SMOKE FAILED"
	exit 1
fi
