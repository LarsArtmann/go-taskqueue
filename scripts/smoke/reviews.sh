#!/usr/bin/env bash
# End-to-end smoke of the review loop (the "second opinion"): completed agent
# tasks mint ONE review task each via the sweeper, the (stub) reviewer returns
# structured verdicts, and --review-autofix turns a request_changes finding
# into a fix task (whose own completion is reviewed and approved, proving the
# loop terminates). No real agent, no network; TQ_AGENT_BIN points at a stub.
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
export GOEXPERIMENT=jsonv2 # tq doctor's go-env check fails the bare-shell env otherwise
export TQ_TQ="$TMP/tq"     # the stub reviewer records verdicts through the real `tq verdict` channel

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

echo "== stub agent binary (reviewer keyed off the review prompt + item text)"
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
	*"strict senior code reviewer"*)
		case "$prompt" in
			# A fix task's review quotes the original item, so "bad work item"
			# alone cannot discriminate — only the top-level work review
			# (item == the bare work prompt) requests changes.
			*"good work item"*|*"A code reviewer rejected"*)
				"$TQ_TQ" verdict '{"verdict":"approve","summary":"looks fine","findings":[]}'
				;;
			*)
				"$TQ_TQ" verdict '{"verdict":"request_changes","summary":"broken","findings":[{"title":"fix the thing","severity":"medium","detail":"it is broken","anchor":"seeded item"}]}'
				;;
		esac
		;;
	*)
		printf 'did the work\n'
		;;
esac
EOF
chmod +x "$STUB"
export TQ_AGENT_BIN="$STUB"

echo "== enqueue two agent tasks (one reviewable-approve, one reviewable-request_changes)"
"$TMP/tq" enqueue --type agent --project smoke --payload "{\"repo\":\"$REPO\",\"prompt\":\"good work item\"}" >/dev/null
"$TMP/tq" enqueue --type agent --project smoke --payload "{\"repo\":\"$REPO\",\"prompt\":\"bad work item\"}" >/dev/null

run_pool() {
	timeout 120 "$TMP/tq" agent-pool --repos "$REPO" --review --review-autofix --once --poll 50ms --task-timeout 30s >"$TMP/pool$1.log" 2>&1 ||
		{
			cat "$TMP/pool$1.log"
			exit 1
		}
}

echo "== pool run 1: work tasks complete, sweeper mints reviews"
run_pool 1
grep -q "review: enqueued" "$TMP/pool1.log" || {
	cat "$TMP/pool1.log"
	echo "FAIL: sweeper never minted a review"
	exit 1
}

echo "== pool runs 2-3: reviews run; request_changes mints a fix (autofix); the fix's own review approves"
run_pool 2
run_pool 3

echo "== assert the queue drained (loop terminated: fix review approved, nothing re-minted)"
if "$TMP/tq" tasks --status pending --json | grep -q '"id"'; then
	"$TMP/tq" tasks --status pending --json
	echo "FAIL: queue did not drain"
	exit 1
fi

echo "== assert verdicts in the journal: 2 approvals, 1 request_changes"
"$TMP/tq" facts --json >"$TMP/facts.json"
[ "$(grep -o '"verdict": *"approve"' "$TMP/facts.json" | wc -l)" -eq 2 ] || {
	echo "FAIL: want exactly 2 approve verdicts"
	exit 1
}
[ "$(grep -o '"verdict": *"request_changes"' "$TMP/facts.json" | wc -l)" -eq 1 ] || {
	echo "FAIL: want exactly 1 request_changes verdict"
	exit 1
}
grep -q '"title": *"fix the thing"' "$TMP/facts.json" || {
	echo "FAIL: finding detail not journaled"
	exit 1
}

echo "== assert the autofix fan-out ran: exactly one fix task minted from the finding"
"$TMP/tq" tasks --json >"$TMP/tasks.json"
[ "$(grep -o 'reviewfix:' "$TMP/tasks.json" | wc -l)" -eq 1 ] || {
	echo "FAIL: want exactly 1 reviewfix task"
	exit 1
}

echo "== assert review dedup: each reviewed task has exactly one review"
[ "$(grep -o 'review:' "$TMP/tasks.json" | wc -l)" -eq 3 ] || {
	echo "FAIL: want exactly 3 review tasks (work x2 + the fix)"
	exit 1
}

echo "== assert counts: 3 agent + 3 review, all completed"
STATS="$("$TMP/tq" stats)"
echo "$STATS"
echo "$STATS" | grep -Eq '^completed\s+6$' || {
	echo "FAIL: want 6 completed"
	exit 1
}

echo "== assert the sweeper checkpoint is at head (doctor liveness)"
"$TMP/tq" doctor | tee "$TMP/doctor.out"
grep -Eq '^ok +review-sweeper' "$TMP/doctor.out" || {
	echo "FAIL: review-sweeper watermark not at head"
	exit 1
}

echo "== review-loop smoke passed"
