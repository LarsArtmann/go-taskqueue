#!/usr/bin/env bash
# End-to-end smoke of the automated status loop (the "done prompt"): agent
# completions mint a status task via the sweeper, the (stub) status agent
# writes a docs/status report and appends a fresh TODO_LIST.md item, and the
# next harvest tick re-arms the loop by enqueueing that item — which the pool
# then runs. No real agent, no network; TQ_AGENT_BIN points at a shell stub.
set -euo pipefail
cd "$(dirname "$0")/../.."
REPO_ROOT="$(pwd)"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

# TQ_BIN points at a prebuilt binary (e.g. the nix-built result/bin/tq);
# unset, the script builds from source with `go build`.
if [ -n "${TQ_BIN:-}" ]; then
	echo "== using prebuilt tq: $TQ_BIN"
	cp "$TQ_BIN" "$TMP/tq"
	chmod +x "$TMP/tq"
else
	echo "== build tq"
	"$REPO_ROOT/scripts/build-tq.sh" "$TMP/tq"
fi

export TQ_DB="$TMP/tasks.db"

echo "== seed repo (clean tree, .tq-verify gate, one DONE item)"
REPO="$TMP/repo"
mkdir -p "$REPO"
git -C "$REPO" init -q
git -C "$REPO" config user.email smoke@tq.local
git -C "$REPO" config user.name "tq smoke"
printf -- '- [x] seeded item\n' >"$REPO/TODO_LIST.md"
printf 'test -f TODO_LIST.md\n' >"$REPO/.tq-verify"
git -C "$REPO" add -A
git -C "$REPO" commit -qm "seed"

echo "== stub agent binary (status runs keyed off the done prompt)"
# Report path computed at run time — a frozen date literal rots silently
# (ratelimit-e2e frozen-date class; check-frozen-dates.sh enforces).
REPORT_PATH="docs/status/$(date +%Y-%m-%d_%H-%M)_smoke.md"
export REPORT_PATH
STUB="$TMP/stub-agent"
# pins: stub-agent-loop-turns — prompt-keyed work/review turn dispatch
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
		printf '# smoke status report\n' > "$REPORT_PATH"
		printf -- '- [ ] freshly minted status-loop item\n' >> TODO_LIST.md
		git add -A
		git commit -qm "docs: smoke status report"
		printf '%s\n' "TQ_RESULT: {\"report\":\"$REPORT_PATH\",\"next_items\":1}"
		;;
	*)
		printf 'did the work\n'
		;;
esac
EOF
chmod +x "$STUB"
export TQ_AGENT_BIN="$STUB"

echo "== enqueue two agent tasks (the window)"
"$TMP/tq" enqueue --type agent --project smoke --payload "{\"repo\":\"$REPO\",\"prompt\":\"do work one\"}"
"$TMP/tq" enqueue --type agent --project smoke --payload "{\"repo\":\"$REPO\",\"prompt\":\"do work two\"}"

echo "== run the pool --once with --status-every 2"
timeout 120 "$TMP/tq" agent-pool --repos "$REPO" --status-every 2 --once --poll 50ms --task-timeout 30s >"$TMP/pool1.log" 2>&1 ||
	{
		cat "$TMP/pool1.log"
		exit 1
	}
grep -q "status: enqueued report" "$TMP/pool1.log" || {
	cat "$TMP/pool1.log"
	echo "FAIL: sweeper never minted a report"
	exit 1
}

echo "== run the pool again (no-op if the status task already drained)"
timeout 120 "$TMP/tq" agent-pool --repos "$REPO" --status-every 2 --once --poll 50ms --task-timeout 30s >"$TMP/pool2.log" 2>&1 ||
	{
		cat "$TMP/pool2.log"
		exit 1
	}

echo "== assert the report landed and TODO_LIST.md grew"
test -f "$REPO/$REPORT_PATH"
grep -q 'freshly minted status-loop item' "$REPO/TODO_LIST.md"

echo "== assert counts: 2 agent + 1 status completed, plus the pool harvest's"
echo "== own re-arm run of the appended item (agent-pool harvests every tick)"
# wait_completed N: the re-arm harvest's 4th task completes on its own tick,
# which can lag the pool's --once exit under a concurrent tq build storm
# (observed 3-vs-4 race, 2026-10-15 00-44 report d3) - poll instead of a
# single immediate read (KNOWN_FLAKY-class pin, cf. scripts/lib/verify-retry.sh).
wait_completed() {
	local want="$1" attempt=1 stats
	while :; do
		stats="$($TMP/tq stats)"
		if echo "$stats" | grep -Eq "^completed\s+$want$"; then
			echo "$stats"
			return 0
		fi
		if [ "$attempt" -ge 15 ]; then
			echo "$stats"
			return 1
		fi
		attempt=$((attempt + 1))
		sleep 1
	done
}

STATS="$(wait_completed 4)" || {
	echo "FAIL: want 4 completed (polled 15s)"
	exit 1
}
echo "$STATS"

echo "== assert the sweeper checkpoint is at head (doctor liveness)"
"$TMP/tq" doctor | tee "$TMP/doctor.out"
grep -Eq '^ok +status-sweeper' "$TMP/doctor.out" || {
	echo "FAIL: status-sweeper watermark not at head"
	exit 1
}

echo "== assert a batch harvest re-arm is deduped (never double-enqueued)"
"$TMP/tq" harvest --repos "$REPO" --json >"$TMP/harvest.json"
grep -q 'freshly minted status-loop item' "$TMP/harvest.json" || {
	cat "$TMP/harvest.json"
	echo "FAIL: harvest did not account for the appended item"
	exit 1
}
STATS="$(wait_completed 4)" || {
	echo "FAIL: completed count changed"
	exit 1
}
echo "$STATS"
if echo "$STATS" | grep -Eq '^pending\s+[1-9]'; then
	echo "FAIL: dedup failed - item enqueued twice"
	exit 1
fi

echo "== status-loop smoke passed"
