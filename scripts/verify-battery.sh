#!/usr/bin/env bash
# Verify-window battery as ONE script (TODO row 370). Spec: the AGENTS.md
# verify-window minimum battery + the row-346 recipe — cheap gates at HEAD
# (check-doc-refs, root build+vet, check-dead-sha-refs), the window's
# own-file dead-SHA grep, the date-measured report filename check, and the
# fresh delta as TARGETED test legs that must prove a PASS COUNT (-v), so a
# window cannot cite a zero-test green ("ok … [no tests to run]") or a
# bare ok. Per-leg output goes to a FILE and the rc is captured from the
# command, never pipe-captured (no PIPESTATUS — it does not survive the
# session shell). Ends by printing the a)-g) close-out skeleton with the
# leg table pre-filled for report windows.
#
# Usage:
#   scripts/verify-battery.sh                                  # cheap battery only
#   scripts/verify-battery.sh -m internal/harvest              # + full module gate
#   scripts/verify-battery.sh -t internal/executor:TestFoo     # + targeted leg (-v, count enforced)
#   scripts/verify-battery.sh -t internal/queue/sqlitev4:'TestStoreConformance/TestCountTasksMatchesList'
#   scripts/verify-battery.sh -r docs/status/2026-10-01_12-34_slug.md
#
# Exit rc: 0 only if every leg passed.
set -uo pipefail
cd "$(dirname "$0")/.."

export GOEXPERIMENT=jsonv2
export GOTOOLCHAIN=auto

# Pool-session env scrub, same list and reason as ci-local (TODO row 333):
# a battery run from an agent-session shell must not read the pool's
# TQ_* channel vars.
for tq_env in TQ_QUESTION_FILE TQ_RESULT_FILE TQ_DB TQ_REDACT TQ_LOG_DIR TQ_PAP_API_KEY TQ_PAP_URL; do
	unset "$tq_env"
done

usage() {
	sed -n '2,20p' "$0" | sed 's/^# \{0,1\}//'
}

BATTERY_DIR="${TQ_BATTERY_DIR:-/tmp/tq-battery}"
mkdir -p "$BATTERY_DIR" || exit 1

MODULES=""
TARGETS=""
REPORT=""
SELFTEST=0

while [ "$#" -gt 0 ]; do
	case "$1" in
	--self-test)
		SELFTEST=1
		shift
		;;
	-m)
		MODULES="$MODULES $2"
		shift 2
		;;
	-t)
		TARGETS="$TARGETS $2"
		shift 2
		;;
	-r)
		REPORT="$2"
		shift 2
		;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		echo "unknown argument: $1 (see --help)"
		exit 2
		;;
	esac
done

# --- helpers (pinned by --self-test) --------------------------------------

# pass_count FILE — number of `--- PASS` rows in a -v go test log.
pass_count() {
	grep -c '^--- PASS' "$1" 2>/dev/null || true
}

# zero_green FILE — rc-0 targeted output that ran ZERO tests: the bare-ok
# trap (row 346).
zero_green() {
	if grep -q 'no tests to run' "$1" 2>/dev/null; then
		return 0
	fi
	[ "$(pass_count "$1")" -eq 0 ]
}

# report_name_ok PATH — the report filename is date-MEASURED: prefixed with
# today's date and the HH-MM stamp shape (AGENTS.md battery rule).
report_name_ok() {
	local base today
	base=$(basename "$1")
	today=$(date +%Y-%m-%d)
	[[ "$base" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}_[0-9]{2}-[0-9]{2}_ ]] || return 1
	[ "${base:0:10}" = "$today" ]
}

if [ "$SELFTEST" = 1 ]; then
	set -e
	tmp=$(mktemp -d)
	trap 'rm -rf "$tmp"' EXIT

	printf '%s\n' '=== RUN   TestA' '--- PASS: TestA' '=== RUN   TestA/sub' '--- PASS: TestA/sub' 'PASS' 'ok  	x	0.1s' >"$tmp/v.log"
	[ "$(pass_count "$tmp/v.log")" = 2 ] || {
		echo "self-test FAIL: pass_count"
		exit 1
	}

	printf '%s\n' '--- PASS: TestReal' 'ok  	x	0.1s' >"$tmp/good.log"
	zero_green "$tmp/good.log" && {
		echo "self-test FAIL: a real pass flagged zero-green"
		exit 1
	}

	printf '%s\n' 'ok  	x	0.5s	[no tests to run]' >"$tmp/notests.log"
	zero_green "$tmp/notests.log" || {
		echo "self-test FAIL: no-tests-to-run not detected as zero-green"
		exit 1
	}

	printf '%s\n' 'ok  	x	0.5s' >"$tmp/bare.log"
	zero_green "$tmp/bare.log" || {
		echo "self-test FAIL: bare ok not detected as zero-green"
		exit 1
	}

	report_name_ok "docs/status/$(date +%Y-%m-%d_%H-%M)_slug.md" || {
		echo "self-test FAIL: today-dated name rejected"
		exit 1
	}
	report_name_ok "docs/status/2026-09-01_08-15_slug.md" && {
		echo "self-test FAIL: stale-dated name accepted"
		exit 1
	}
	report_name_ok "docs/status/no-timestamp.md" && {
		echo "self-test FAIL: undated name accepted"
		exit 1
	}

	echo "verify-battery self-test ok (pass_count, zero-green trap incl. [no tests to run], date-measured filename)"
	exit 0
fi

LEG_NAMES=()
LEG_RCS=()
LEG_LOGS=()

leg() {
	local name="$1"
	shift
	local idx=${#LEG_NAMES[@]}
	local safe=${name//\//_}
	local log
	log=$(printf '%s/%02d-%s.log' "$BATTERY_DIR" "$idx" "$safe")
	"$@" >"$log" 2>&1
	local rc=$?
	printf '%s\n' "$rc" >"${log%.log}.rc"
	LEG_NAMES+=("$name")
	LEG_RCS+=("$rc")
	LEG_LOGS+=("$log")
	if [ "$rc" -eq 0 ]; then
		echo "PASS: $name"
	else
		echo "FAIL: $name (rc=$rc, log: $log)"
	fi
}

# targeted_leg MODULE PATTERN — the fresh delta. Runs go test -run … -v and
# then enforces the row-346 recipe: a PASS count > 0, never a bare ok.
targeted_leg() {
	local m="$1" pattern="$2"
	local idx=${#LEG_NAMES[@]}
	local name="target:$m:$pattern"
	local log
	log=$(printf '%s/%02d-target.log' "$BATTERY_DIR" "$idx")
	if [ "$m" = cmd/tq ]; then
		./scripts/test-cmd-tq.sh -run "$pattern" -count=1 -v >"$log" 2>&1
	else
		bash -c 'cd "$1" && GOWORK=off go test -run "$2" -count=1 -v -timeout 300s' _ "$m" "$pattern" >"$log" 2>&1
	fi
	local rc=$?
	printf '%s\n' "$rc" >"${log%.log}.rc"
	LEG_NAMES+=("$name")
	LEG_RCS+=("$rc")
	LEG_LOGS+=("$log")
	if [ "$rc" -ne 0 ]; then
		echo "FAIL: $name (rc=$rc, log: $log)"
		return 0
	fi

	if zero_green "$log"; then
		LEG_RCS[$idx]=1
		printf '%s\n' 1 >"${log%.log}.rc"
		echo "FAIL: $name — ZERO-TEST GREEN: '-run $pattern' matched nothing (row 346: conform pins register as subtests — use -run 'TestStoreConformance/<name>'; cite -v plus a PASS count, never the bare ok)"
		return 0
	fi

	local n
	n=$(pass_count "$log")
	echo "PASS: $name (PASS rows: $n)"
}

# --- battery ---------------------------------------------------------------

echo "== verify battery $(date +%Y-%m-%d\ %H:%M:%S) at $(git rev-parse --short HEAD) (logs: $BATTERY_DIR)"

leg "doc-refs" ./scripts/check-doc-refs.sh
leg "root-build" go build ./...
leg "root-vet" go vet ./...
leg "dead-sha-default-scope" ./scripts/check-dead-sha-refs.sh

# Own-file dead-SHA grep (row 370): the window's touched files, including
# paths outside the gate's default living-docs scope.
changed="$( {
	git diff --name-only HEAD
	git diff --cached --name-only
} | sort -u)"
if [ -n "$changed" ]; then
	# shellcheck disable=SC2086
	leg "dead-sha-own-files" ./scripts/check-dead-sha-refs.sh $changed
else
	echo "SKIP: dead-sha-own-files (clean tree — no touched files)"
fi

if [ -n "$REPORT" ]; then
	if report_name_ok "$REPORT"; then
		leg "report-filename" true
	else
		echo "FAIL: report-filename '$REPORT' is not date-measured (want docs/status/$(date +%Y-%m-%d_%H-%M)_<slug>.md — a projected or stale timestamp is the 06-12 defect class)"
		leg "report-filename" false
	fi
else
	echo "date-measured report filename for this battery: docs/status/$(date +%Y-%m-%d_%H-%M)_<slug>.md"
fi

for m in $MODULES; do
	if [ "$m" = cmd/tq ]; then
		leg "module:cmd/tq" ./scripts/test-cmd-tq.sh
	else
		leg "module:$m" bash -c 'cd "$1" && GOWORK=off go build ./... && GOWORK=off go vet ./... && GOWORK=off go test ./... -count=1 -timeout 300s' _ "$m"
	fi
done

for t in $TARGETS; do
	m=${t%%:*}
	pattern=${t#*:}
	if [ "$m" = "$pattern" ]; then
		echo "BAD -t leg '$t' (want module:pattern)"
		exit 2
	fi
	targeted_leg "$m" "$pattern"
done

# --- summary + a)-g) skeleton ----------------------------------------------

failures=0
for rc in "${LEG_RCS[@]}"; do
	[ "$rc" -ne 0 ] && failures=$((failures + 1))
done

echo
echo "== BATTERY: $((${#LEG_NAMES[@]} - failures))/${#LEG_NAMES[@]} legs green"
for i in "${!LEG_NAMES[@]}"; do
	state=PASS
	[ "${LEG_RCS[$i]}" -ne 0 ] && state=FAIL
	printf '  %-4s %s\n' "$state" "${LEG_NAMES[$i]}"
done

cat <<'EOF'

a) FULLY DONE:
b) PARTIALLY DONE:
c) NOT STARTED:
d) TOTALLY FUCKED UP:
e) IMPROVE:
f) next:
g) questions:
(paste the leg table above + cite log paths and PASS counts; every targeted
claim needs -v and a count, never a bare ok)
EOF

[ "$failures" -eq 0 ]
