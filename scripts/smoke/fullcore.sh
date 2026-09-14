#!/usr/bin/env bash
# Hermetic smoke of examples/fullcore on the sqlite backend: builds the
# example from source, points it at a scratch DB, and asserts the queue
# drains with all four demo tasks completed (including the deliberate
# first-attempt retry). No network, no external services.
#
# The explicit TQ_DB export is the production-DB trap guard: agent shells
# inherit TQ_DB=/mnt/pool/services/tq/tq.db, and sqlite.Open-style helpers
# prefer the env over ./tasks.db — without it a stray default open could
# touch the production dogfood journal. The example itself takes --db, so
# this is belt and braces.
set -euo pipefail
cd "$(dirname "$0")/../.."

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

# GOEXPERIMENT=jsonv2 is REQUIRED outside the flake devShell (go-sse imports
# encoding/json/v2 — see AGENTS.md).
export GOEXPERIMENT=jsonv2
export TQ_DB="$TMP/tasks.db"

echo "== build examples/fullcore"
go build -o "$TMP/fullcore" ./examples/fullcore

echo "== run (sqlite backend, scratch db: $TMP/fullcore.db)"
"$TMP/fullcore" --backend sqlite --db "$TMP/fullcore.db" >"$TMP/out.log" 2>&1 || {
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

# Deadline-path determinism (08-25 report b3): a too-short --timeout must
# deterministically kill the run with the drain-deadline message — 16a15d8
# verified this only by hand. 50ms is far below worker-start latency on this
# host (250ms still trips), so the failure mode is stable, not a race.
DEADLINE_RUNS=3

echo "== deadline path x$DEADLINE_RUNS (50ms timeout must fail with drain deadline)"
for i in $(seq 1 "$DEADLINE_RUNS"); do
	if "$TMP/fullcore" --backend sqlite --db "$TMP/fullcore-deadline.db" --timeout 50ms >"$TMP/deadline-$i.log" 2>&1; then
		echo "FAIL: deadline run $i exited zero (queue drained inside 50ms?)"
		cat "$TMP/deadline-$i.log"
		exit 1
	fi
	if ! grep -q 'deadline exceeded before the queue drained' "$TMP/deadline-$i.log"; then
		echo "FAIL: deadline run $i failed for the wrong reason"
		cat "$TMP/deadline-$i.log"
		exit 1
	fi
done

echo "PASS: fullcore drained 4/4 on sqlite (scratch TQ_DB honored)"
