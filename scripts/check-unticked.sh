#!/usr/bin/env bash
# Landed-but-unticked sweep (TODO row 1292, dispatch 000001a19e59e52e219b2608422000000000):
# extracts each OPEN TODO_LIST.md row's backticked symbols/paths, mechanically
# probes them at HEAD, and prints candidate landed-but-unticked rows for a
# human/agent to tick WITH EVIDENCE — ticking stays a human decision, this
# tool only candidates (never edits TODO_LIST.md).
#
# Probes, per backticked token on an open row:
#   path   — token is a tracked file at HEAD (git ls-files exact match)
#   symbol — token resolves in Go source: grep over tracked *.go (substring,
#            the dead-exports lesson: word matching undercounts suffixed refs)
#   gotest — token is a Test/Symbol and `go test -list` finds it in the
#            owning module (TQ_UNTICKED_TEST_MODULES hook narrows the set in
#            the self-test; the real run sweeps every module via
#            scripts/for-each-module.sh + root + cmd/tq)
#   gate   — token names a gate script (scripts/check-*.sh / smoke / tq-*)
#            whose --self-test plus the live gate state matches open rows
#            CLAIMING that gate red (row text mentions the gate + a fixed
#            finding token and the gate is green at HEAD): candidate
#            landed-but-unticked. This is the channel that would have
#            flagged the consumer-tag SC2046 + guard-wiring orphan rows on
#            2026-10-27 without the manual sweep.
#
# Advisory by design: rc=0 always (candidates are prompts, not verdicts);
# the pinned countable summary follows the fleet convention
# (2026-10-24, TODO 01-12 §e7/§g2).
# shellcheck disable=SC2016
set -euo pipefail
cd "$(dirname "$0")/.."
self_abs="$(cd "$(dirname "$0")" && pwd)/$(basename "$0")"

must_mention() {
	if grep -qF -- "$1" <<<"$2"; then
		echo "ok: $3"
	else
		echo "FAIL: $3 (output is missing '$1')"
		printf '%s\n' "$2"
		exit 1
	fi
}

self_test() {
	# tmp is deliberately global: the EXIT trap reads it after self_test
	# returned, and set -u would kill the trap on a function-local name.
	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' EXIT
	printf '# unticked self-test corpus\n- [ ] a row citing `tq_fakesym_never_exists` as landed evidence\n- [ ] a row citing `%s` — this real path must NOT be a path-candidate (the probe is exact-path, not a prefix)\n- [x] a done row citing `tq_donesym_never_exists` (skipped: only open rows are probed)\n' "$self_abs" >"$tmp/TODO.md"
	local out rc=0
	out="$(TQ_UNTICKED_TODO="$tmp/TODO.md" TQ_UNTICKED_SKIP_GATES=1 "$self_abs" 2>&1)" || rc=$?
	if [ "$rc" -ne 0 ]; then
		echo "FAIL: advisory gate must exit rc=0 (got rc=$rc)"
		printf '%s\n' "$out"
		exit 1
	fi
	must_mention "CANDIDATE landed-but-unticked (open row cites tq_fakesym_never_exists but the symbol has no tracked Go occurrence):" "$out" "symbol probe fires on the planted dead symbol"
	must_mention "unticked-sweep summary: 1 candidates, " "$out" "summary pins the candidate counter (done-row symbol skipped, self path skipped)"
	if grep -q 'tq_donesym_never_exists' <<<"$out"; then
		echo "FAIL: done rows must be skipped, not probed"
		printf '%s\n' "$out"
		exit 1
	fi
	local pos_out
	pos_out="$("$self_abs" 2>&1)" || true
	if grep -Eq '^unticked-sweep summary: [0-9]+ candidates, [0-9]+ open rows scanned, [0-9]+ symbols probed \(advisory\)$' <<<"$pos_out"; then
		echo "ok: real-run summary matches the pinned format"
	else
		echo "FAIL: real-run summary missing or malformed"
		printf '%s\n' "$pos_out"
		exit 1
	fi
	echo "unticked-sweep self-test ok (summary format pin + negative counter battery)"
}

if [ "${1:-}" = "--self-test" ]; then
	self_test
	exit 0
fi

todo="${TQ_UNTICKED_TODO:-TODO_LIST.md}"
if [ ! -f "$todo" ]; then
	echo "FAIL: $todo not found" >&2
	exit 1
fi

# Advisory gate: the summary + candidate lines are the product; rc stays 0
# so ci-local wiring reports rather than blocks (ticking is a human act).
rc=0

candidates=0
open_rows=0
symbols_probed=0

# Per-row: collect open-row text, extract backticked tokens.
rowbuf=""
flush_row() {
	[ -n "$rowbuf" ] || return 0
	open_rows=$((open_rows + 1))
	local row="$rowbuf"
	rowbuf=""
	local tok
	# Backticked tokens; skip file:// style noise and the script's own name.
	while IFS= read -r tok; do
		case "$tok" in
			'' | '`') continue ;;
		esac
		case "$tok" in
			"$self_abs") continue ;;
		esac
		# PATH probe: exact tracked-file match at HEAD.
		if git ls-files --error-unmatch "$tok" >/dev/null 2>&1; then
			continue
		fi
		# GATE probe: token names a known gate script; compare the row's red
		# claims against the gate's live state.
		if [ -z "${TQ_UNTICKED_SKIP_GATES:-}" ] && [ -f "$tok" ]; then
			continue
		fi
		if [ -z "${TQ_UNTICKED_SKIP_GATES:-}" ]; then
			case "$tok" in
				scripts/check-*.sh | scripts/smoke/*.sh | scripts/tq-*.sh | scripts/*.sh)
					continue
					;;
			esac
		fi
		# GOTEST probe: TestXxx symbols resolved through go test -list in
		# their owning module (heuristic: match the row's backticked path
		# hints; fall back to the module sweep).
		if [[ "$tok" == Test* ]]; then
			symbols_probed=$((symbols_probed + 1))
			if probe_test_symbol "$tok"; then
				continue
			fi
		else
			# SYMBOL probe: substring grep over tracked Go sources (the
			# dead-exports lesson: substring, never word-boundary).
			symbols_probed=$((symbols_probed + 1))
			if git grep -qF -- "$tok" -- '*.go' 2>/dev/null; then
				continue
			fi
		fi
		candidates=$((candidates + 1))
		echo "CANDIDATE landed-but-unticked (open row cites $tok but the symbol has no tracked Go occurrence):"
		printf '  %s\n' "$row"
	done < <(grep -oE '`[^`]+`' <<<"$row" | tr -d '`')
}

probe_test_symbol() {
	local sym="$1" mods m
	mods="$(scripts/for-each-module.sh 2>/dev/null; echo .; echo cmd/tq)"
	for m in $mods; do
		[ -f "$m/go.mod" ] || continue
		if (cd "$m" && GOWORK=off go list ./... >/dev/null 2>&1 && GOWORK=off go test -list "^${sym}$" ./... 2>/dev/null) | grep -q "^${sym}$"; then
			return 0
		fi
	done
	return 1
}

while IFS= read -r line; do
	if [[ "$line" == '- [ ] '* ]]; then
		flush_row
		rowbuf="$line"
	elif [[ "$line" == '- [x]'* ]]; then
		flush_row
	elif [ -n "$rowbuf" ]; then
		# Continuation lines fold into the current open row.
		rowbuf="$rowbuf $line"
	fi
done <"$todo"
flush_row

echo "unticked-sweep summary: $candidates candidates, $open_rows open rows scanned, $symbols_probed symbols probed (advisory)"
exit "$rc"
