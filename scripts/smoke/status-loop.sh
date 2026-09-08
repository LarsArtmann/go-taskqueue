#!/usr/bin/env bash
# End-to-end smoke of the automated status loop (the "done prompt"): agent
# completions mint a status task via the sweeper, the (stub) status agent
# writes a docs/status report and appends a fresh TODO_LIST.md item, and the
# next harvest tick re-arms the loop by enqueueing that item — which the pool
# then runs. No real agent, no network; TQ_AGENT_BIN points at a shell stub.
set -euo pipefail
cd "$(dirname "$0")/../.."

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
	go build -o "$TMP/tq" ./cmd/tq
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
	*"status reporter"*)
		mkdir -p docs/status
		printf '# smoke status report\n' > docs/status/2026-09-08_00-00_smoke.md
		printf -- '- [ ] freshly minted status-loop item\n' >> TODO_LIST.md
		git add -A
		git commit -qm "docs: smoke status report"
		printf '%s\n' 'TQ_RESULT: {"report":"docs/status/2026-09-08_00-00_smoke.md","next_items":1}'
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
test -f "$REPO/docs/status/2026-09-08_00-00_smoke.md"
grep -q 'freshly minted status-loop item' "$REPO/TODO_LIST.md"

echo "== assert counts: 2 agent + 1 status completed, plus the pool harvest's"
echo "== own re-arm run of the appended item (agent-pool harvests every tick)"
STATS="$("$TMP/tq" stats)"
echo "$STATS"
echo "$STATS" | grep -Eq '^completed\s+4$' || {
	echo "FAIL: want 4 completed"
	exit 1
}

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
STATS="$("$TMP/tq" stats)"
echo "$STATS"
echo "$STATS" | grep -Eq '^completed\s+4$' || {
	echo "FAIL: completed count changed"
	exit 1
}
if echo "$STATS" | grep -Eq '^pending\s+[1-9]'; then
	echo "FAIL: dedup failed - item enqueued twice"
	exit 1
fi

echo "== status-loop smoke passed"
