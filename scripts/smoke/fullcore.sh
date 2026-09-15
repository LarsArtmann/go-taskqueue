#!/usr/bin/env bash
# Hermetic smoke of examples/fullcore on the sqlite backend: builds the
# example from source, points it at a scratch DB, and asserts the queue
# drains with all four demo tasks completed (including the deliberate
# first-attempt retry). No network, no external services.
#
# The explicit TQ_DB export is the production-DB trap guard: agent shells
# inherit TQ_DB=/mnt/pool/services/tq/tq.db, and the example now CONSUMES
# TQ_DB as its --db default (the smoke asserts the scratch file materialized),
# so a stray default open cannot touch the production dogfood journal.
#
# Setting TQ_TEST_POSTGRES adds the postgres variant (drain + deadline path
# against that DSN) — the CI test-postgres job exports it; locally:
#   docker run --rm -d -p 5432:5432 -e POSTGRES_USER=tq -e POSTGRES_PASSWORD=tq \
#     -e POSTGRES_DB=tqtest postgres:16
#   TQ_TEST_POSTGRES=postgres://tq:tq@localhost:5432/tqtest ./scripts/smoke/fullcore.sh
#
# Deadline-path knobs (02-04 f2): DEADLINE_RUNS / DEADLINE_TIMEOUT_MS scale
# the determinism loop for nightly-style beefier runs without editing the
# script. Defaults stay committed.
set -euo pipefail
cd "$(dirname "$0")/../.."

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

# GOEXPERIMENT=jsonv2 is REQUIRED outside the flake devShell (go-sse imports
# encoding/json/v2 — see AGENTS.md).
export GOEXPERIMENT=jsonv2
export TQ_DB="$TMP/tasks.db"

DEADLINE_RUNS="${DEADLINE_RUNS:-3}"
DEADLINE_TIMEOUT="${DEADLINE_TIMEOUT_MS:-50}ms"

echo "== build examples/fullcore"
go build -o "$TMP/fullcore" ./examples/fullcore

echo "== run (sqlite backend, NO --db: the run must consume \$TQ_DB)"
"$TMP/fullcore" --backend sqlite >"$TMP/out.log" 2>&1 || {
	echo "FAIL: fullcore exited nonzero"
	cat "$TMP/out.log"
	exit 1
}

echo "== assert drain report"
if ! grep -q '^drained; final counts:' "$TMP/out.log"; then
	echo "FAIL: no drain report"
	cat "$TMP/out.log"
	exit 1
fi
if ! grep -q '^  completed 4$' "$TMP/out.log"; then
	echo "FAIL: expected 4 completed (sh x2, greet, flaky retry)"
	cat "$TMP/out.log"
	exit 1
fi

# TQ_DB-consumption proof: the example must have opened the scratch DB the
# env pointed at (a non-empty sqlite file), not fallen back to ./fullcore.db.
if [ ! -s "$TMP/tasks.db" ]; then
	echo "FAIL: TQ_DB scratch db was not consumed (example ignored the env?)"
	exit 1
fi
rm -f fullcore.db

# Deadline-path determinism (08-25 report b3): a too-short --timeout must
# deterministically kill the run with the drain-deadline message — 16a15d8
# verified this only by hand. 50ms is far below worker-start latency on this
# host (250ms still trips), so the failure mode is stable, not a race; the
# exact-message assertion doubles as the margin check (a drain inside the
# timeout would exit 0 and fail the run's own guard below).
echo "== deadline path x$DEADLINE_RUNS ($DEADLINE_TIMEOUT timeout must fail with drain deadline)"
for i in $(seq 1 "$DEADLINE_RUNS"); do
	if "$TMP/fullcore" --backend sqlite --db "$TMP/fullcore-deadline.db" --timeout "$DEADLINE_TIMEOUT" >"$TMP/deadline-$i.log" 2>&1; then
		echo "FAIL: deadline run $i exited zero (queue drained inside $DEADLINE_TIMEOUT?)"
		cat "$TMP/deadline-$i.log"
		exit 1
	fi
	if ! grep -q 'deadline exceeded before the queue drained' "$TMP/deadline-$i.log"; then
		echo "FAIL: deadline run $i failed for the wrong reason"
		cat "$TMP/deadline-$i.log"
		exit 1
	fi
done

if [ -n "${TQ_TEST_POSTGRES:-}" ]; then
	echo "== postgres variant (drain + deadline on $TQ_TEST_POSTGRES)"
	if "$TMP/fullcore" --backend postgres --dsn "$TQ_TEST_POSTGRES" >"$TMP/pg.log" 2>&1; then
		:
	else
		echo "FAIL: fullcore postgres run exited nonzero"
		cat "$TMP/pg.log"
		exit 1
	fi
	if ! grep -q '^  completed 4$' "$TMP/pg.log"; then
		echo "FAIL: postgres variant expected 4 completed"
		cat "$TMP/pg.log"
		exit 1
	fi
	if "$TMP/fullcore" --backend postgres --dsn "$TQ_TEST_POSTGRES" --timeout "$DEADLINE_TIMEOUT" >"$TMP/pg-deadline.log" 2>&1; then
		echo "FAIL: postgres deadline run exited zero (queue drained inside $DEADLINE_TIMEOUT?)"
		cat "$TMP/pg-deadline.log"
		exit 1
	fi
	if ! grep -q 'deadline exceeded before the queue drained' "$TMP/pg-deadline.log"; then
		echo "FAIL: postgres deadline run failed for the wrong reason"
		cat "$TMP/pg-deadline.log"
		exit 1
	fi
	echo "PASS: fullcore postgres variant drained 4/4 + deadline path"
else
	echo "SKIP: postgres variant (TQ_TEST_POSTGRES unset)"
fi

echo "PASS: fullcore drained 4/4 on sqlite (scratch TQ_DB honored)"
