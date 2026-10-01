#!/usr/bin/env bash
# README-as-contract rot guard: the install + quickstart snippets are the
# README's contract with every new user — this smoke keeps both sides
# honest (05-29 report c3/f6): every guarded line must (a) still exist in
# README.md and (b) still run green against a scratch DB, so a CLI change
# that breaks a documented command OR a README edit that rots one fails
# CI. (The quickstart `tq worker` line runs as the same fence's documented
# `--once` variant — a blocking worker cannot run in CI; the dashboard and
# agent-pool snippets have dedicated live smokes: webui.sh, multi-repo.sh.)
#
# Assertions:
#   1. the install + quickstart lines exist in README.md as documented
#   2. enqueue (raw + JSON payload), worker --once, stats run green
#   3. tail -f streams facts and exits 0 on SIGTERM
#   4. the hello task completes (the snippet works end-to-end, not just
#      an accepted enqueue)
set -uo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
WORK="$(mktemp -d)"
TQ="${TQ_BIN:-$WORK/tq}"
cleanup() { rm -rf "$WORK"; }
trap cleanup EXIT

if [ -z "${TQ_BIN:-}" ]; then
	echo "== building tq =="
	"$REPO_ROOT/scripts/build-tq.sh" "$TQ" || exit 1
fi

README="$REPO_ROOT/README.md"
failed=0

echo "== README still documents the install + quickstart lines =="
while IFS= read -r want; do
	grep -Fq -- "$want" "$README" || {
		echo "FAIL: README no longer carries the documented line: $want"
		failed=1
	}
done <<'LINES'
git clone https://github.com/LarsArtmann/go-taskqueue && cd go-taskqueue
./scripts/build-tq.sh ~/.local/bin/tq
tq enqueue --type sh --project demo --payload 'echo hello from $(uname -s)'
tq enqueue --type sh --project demo --payload '{"cmd":"go test ./..."}'
tq worker
--once drains and exits
tq stats
tq tail -f
LINES

echo "== running the quickstart against a scratch DB =="
# Scratch DB is MANDATORY: agent shells inherit TQ_DB (the PRODUCTION
# journal) — AGENTS.md Known Issues.
RUN="$WORK/run"
mkdir -p "$RUN"
export TQ_DB="$WORK/scratch.db"
cd "$RUN" || exit 1

"$TQ" enqueue --type sh --project demo --payload 'echo hello from $(uname -s)' >/dev/null || {
	echo "FAIL: README enqueue (raw payload) exited non-zero"
	failed=1
}
"$TQ" enqueue --type sh --project demo --payload '{"cmd":"go test ./..."}' >/dev/null || {
	echo "FAIL: README enqueue (JSON payload) exited non-zero"
	failed=1
}
"$TQ" worker --once >/dev/null || {
	echo "FAIL: README worker (--once variant) exited non-zero"
	failed=1
}
"$TQ" stats >/dev/null || {
	echo "FAIL: README stats exited non-zero"
	failed=1
}

# The hello task must have COMPLETED — the snippet's promise is a task
# that actually executes. (The JSON example's payload — go test ./... —
# is payload semantics, not CLI contract: it dead-letters in a bare dir
# by design and is not asserted.)
tq_tasks="$("$TQ" tasks --status completed)"
case $tq_tasks in
*completed*demo*sh*) ;;
*)
	echo "FAIL: the quickstart hello task did not complete:"
	"$TQ" tasks
	failed=1
	;;
esac

# tail -f streams the fact feed and exits 0 on SIGTERM (the README's
# "watch it work" line); timeout bounds a regression that ignores TERM.
if ! timeout 15 bash -c '"$1" tail -f >"$2" 2>&1 & p=$!; sleep 1; kill -TERM "$p" 2>/dev/null; wait "$p"' _ "$TQ" "$WORK/tail.out"; then
	echo "FAIL: tq tail -f did not stream and exit 0 on SIGTERM"
	failed=1
fi
[ -s "$WORK/tail.out" ] || {
	echo "FAIL: tq tail -f streamed no facts"
	failed=1
}

if [ "$failed" -eq 0 ]; then
	echo "readme-install smoke: PASS (README lines intact, quickstart green on a scratch DB)"
fi
exit "$failed"
