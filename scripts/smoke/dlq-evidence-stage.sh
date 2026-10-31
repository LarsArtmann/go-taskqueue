#!/usr/bin/env bash
# End-to-end smoke of `tq dlq --evidence-stage` (2026-10-26 01-40 §f6):
# the filter was unit-tested only before this — here the REAL CLI binary
# runs against a scratch DB (TQ_DB exported per the production-trap rule;
# a bare tq command would otherwise touch the PRODUCTION dogfood journal),
# seeds dead tasks through the actual sh-executor failure path, and walks
# the evidence-stage filter incl. the comma-separated term spec with
# `!`-negation (2026-10-30: gate-env alias matching through the recorded
# error text rides the same matcher).
set -euo pipefail
cd "$(dirname "$0")/../.."
REPO_ROOT="$(pwd)"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

if [ -n "${TQ_BIN:-}" ]; then
	echo "== using prebuilt tq: $TQ_BIN"
	cp "$TQ_BIN" "$TMP/tq"
	chmod +x "$TMP/tq"
else
	echo "== build tq"
	"$REPO_ROOT/scripts/build-tq.sh" "$TMP/tq"
fi

export TQ_DB="$TMP/tasks.db"

# json_count FILTER-ARGS… — dead-row count read from the `tq dlq --json`
# summary lead object's `count` field (TODO row 1416: project-name greps
# break on name collisions and summary lines; the summary count cannot).
# sed reads the WHOLE stream (no early exit) so the pipeline cannot
# SIGPIPE the tq binary under pipefail.
json_count() {
	local lead
	lead="$("$TMP/tq" dlq --json "$@" 2>/dev/null | sed -n '1p')"
	printf '%s' "$lead" | sed -n 's/.*"count":\([0-9][0-9]*\).*/\1/p'
}

# expect_count LABEL EXPECTED_MIN FILTER-ARGS… — assert the filtered dead
# population reaches EXPECTED_MIN rows, counted via the JSON summary.
expect_count() { # expect_count LABEL EXPECTED_MIN [FILTER-ARGS…]
	local label="$1" want="$2" got
	shift 2
	got="$(json_count "$@")"
	if [ "${got:-0}" -lt "$want" ]; then
		echo "FAIL: $label — expected >=$want dead rows, saw ${got:-none}"
		"$TMP/tq" dlq "$@" || true
		exit 1
	fi
}

echo "== seed two failing sh tasks (max-attempts 1)"
# Task A: plain command failure — the sh executor stamps FailureEvidence
# Stage "command" on the task.failed fact.
"$TMP/tq" enqueue --type sh --project smoke --payload '"exit 3"' --max-attempts 1 >/dev/null
# Task B: failure whose recorded error text carries the gate-env wording —
# the alias lives ONLY in the error text, so the filter must match it
# through MatchesEvidenceStage's error-text leg.
"$TMP/tq" enqueue --type sh --project smoke \
	--payload '"echo agent verify gate environmental signature [vendor-gofmt]: boom >&2; exit 7"' \
	--max-attempts 1 >/dev/null

echo "== run the sh worker until both tasks are dead"
for _ in 1 2 3 4 5 6; do
	"$TMP/tq" worker --once --poll 50ms --lease 5s --task-timeout 30s >>"$TMP/worker.log" 2>&1 || true
	DEAD="$(json_count --db "$TQ_DB" || true)"
	[ "${DEAD:-0}" -ge 2 ] && break
done

"$TMP/tq" dlq >"$TMP/dlq-all.txt"
grep -q 'evidence=command' "$TMP/dlq-all.txt" || {
	echo "FAIL: the plain command death lost its evidence=command suffix"
	cat "$TMP/dlq-all.txt"
	exit 1
}
grep -q 'exit status 7' "$TMP/dlq-all.txt" || {
	echo "FAIL: the gate-env death (exit 7) is not on the DLQ listing"
	cat "$TMP/dlq-all.txt"
	exit 1
}

echo "== single-term filter: command"
"$TMP/tq" dlq --evidence-stage command >"$TMP/dlq-command.txt"
expect_count "evidence-stage command" 1 --evidence-stage command
grep -q 'gate-env' "$TMP/dlq-command.txt" && {
	echo "FAIL: command filter leaked the gate-env death"
	cat "$TMP/dlq-command.txt"
	exit 1
} || true

echo "== alias filter: gate-env (matched through the recorded error text)"
"$TMP/tq" dlq --evidence-stage gate-env >"$TMP/dlq-gateenv.txt"
expect_count "evidence-stage gate-env" 1 --evidence-stage gate-env
grep -q 'exit status 7' "$TMP/dlq-gateenv.txt" || {
	echo "FAIL: gate-env filter picked the wrong death"
	cat "$TMP/dlq-gateenv.txt"
	exit 1
}
grep -q 'exit status 3' "$TMP/dlq-gateenv.txt" && {
	echo "FAIL: gate-env filter leaked the command death"
	cat "$TMP/dlq-gateenv.txt"
	exit 1
}

echo "== set spec with negation: gate-env,!gate-slow"
"$TMP/tq" dlq --evidence-stage 'gate-env,!gate-slow' >"$TMP/dlq-set.txt"
expect_count "evidence-stage gate-env,!gate-slow" 1 --evidence-stage 'gate-env,!gate-slow'

echo "== negation-only spec: !gate-slow keeps both"
"$TMP/tq" dlq --evidence-stage '!gate-slow' >"$TMP/dlq-negonly.txt"
expect_count "evidence-stage !gate-slow" 2 --evidence-stage '!gate-slow'

echo "== contradictory spec: command,!command is empty"
"$TMP/tq" dlq --evidence-stage 'command,!command' >"$TMP/dlq-empty.txt"
EMPTY="$(json_count --evidence-stage 'command,!command')"
if [ "${EMPTY:-1}" -ne 0 ]; then
	echo "FAIL: command,!command must exclude both deaths (count=$EMPTY)"
	cat "$TMP/dlq-empty.txt"
	exit 1
fi

echo "PASS: dlq-evidence-stage smoke (single term, gate-env alias, set spec with negation, negation-only, empty set)"
