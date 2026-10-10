#!/usr/bin/env bash
# Flag-count guard (E3 of the 2026-10-09 config-system plan): pins the
# number of registered flags per `tq <cmd> -h` so flag growth is a
# CONSCIOUS act — the agent-pool surface alone carries 55 registrations,
# and a new default flag silently widens every deployment's config
# surface. A legit new flag updates the baseline with `--update` and the
# commit message names it; an ACCIDENTAL registration (typo'd helper
# sharing a FlagSet, a flag added to the wrong command) fails here
# before review has to catch it.
#
# Counts lines matching '^  -' in `-h` output (flag.FlagSet.PrintDefaults
# format), so usage-text edits never trip the guard — only real
# registrations do.
#
#   ./scripts/check-flag-count.sh           # compare against baseline
#   ./scripts/check-flag-count.sh --update  # regenerate the baseline
#   FLAG_COUNT_SELF_TEST=1 ...              # verify drift is caught
set -uo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BASELINE="$REPO_ROOT/scripts/flag-count-baseline.txt"
WORK="$(mktemp -d)"
TQ="${TQ_BIN:-$WORK/tq}"
cleanup() { rm -rf "$WORK"; }
trap cleanup EXIT

if [ -z "${TQ_BIN:-}" ]; then
	echo "== building tq =="
	"$REPO_ROOT/scripts/build-tq.sh" "$TQ" || exit 1
fi

# Scratch DB: tq inherits TQ_DB from the environment and must never touch
# production.
export TQ_DB="$WORK/scratch.db"

# Kept in sync with the command list in scripts/smoke/help-text.sh plus
# the non-help-text commands (ask, crush, incidents, session, verdict,
# watermarks) so every user-visible flag surface is pinned.
cmds="enqueue worker harvest bootstrap agent-pool stats tasks audit doctor top show dlq cancel facts tail serve api version ask crush incidents session verdict watermarks"

count_flags() {
	local cmd="$1"
	"$TQ" "$cmd" -h 2>&1 | grep -c '^  -' || true
}

write_baseline() {
	{
		echo "# Per-command registered-flag counts, pinned by scripts/check-flag-count.sh"
		echo "# (E3, config-system plan 2026-10-09). Regenerate with --update; growth"
		echo "# needs the flag named in the commit. Format: <cmd> <count>."
		for cmd in $cmds; do
			echo "$cmd $(count_flags "$cmd")"
		done
	} >"$BASELINE"
	echo "baseline written: $BASELINE"
}

declare -A actual
for cmd in $cmds; do
	actual[$cmd]="$(count_flags "$cmd")"
done

if [ "${1:-}" = "--update" ]; then
	write_baseline
	exit 0
fi

if [ ! -f "$BASELINE" ]; then
	echo "FAIL: baseline missing: $BASELINE (run with --update to create)"
	exit 1
fi

fail=0
while read -r cmd expected; do
	case "$cmd" in
	'#'*) continue ;;
	esac

	got="${actual[$cmd]:-MISSING-COMMAND}"
	if [ "$got" != "$expected" ]; then
		echo "FAIL: tq $cmd registered flags: expected $expected, got $got"
		echo "  (a new/removed flag? update the baseline with --update and name the flag in the commit;"
		echo "   an unexpected count can also mean a flag landed on the wrong command)"
		fail=1
	fi
done <"$BASELINE"

for cmd in $cmds; do
	if ! grep -q "^$cmd " "$BASELINE"; then
		echo "FAIL: command $cmd missing from baseline (added to the command list? --update)"
		fail=1
	fi
done

if [ "$fail" = 0 ]; then
	echo "flag counts ok: $(grep -vc '^#' "$BASELINE") commands match the baseline"
fi

if [ "${FLAG_COUNT_SELF_TEST:-}" = "1" ]; then
	echo "== self-test: tampered baseline must FAIL =="
	sed 's/^agent-pool 55$/agent-pool 999/' "$BASELINE" >"$WORK/tampered"
	sed -i "s|$BASELINE|$WORK/tampered|" /dev/null 2>/dev/null || true
	# Re-run the comparison logic against the tampered file by pointing the
	# loop at it: simplest is a subshell re-read with BASELINE overridden.
	tampered_fail=0
	while read -r cmd expected; do
		case "$cmd" in
		'#'*) continue ;;
		esac
		got="${actual[$cmd]:-MISSING-COMMAND}"
		if [ "$got" != "$expected" ]; then
			tampered_fail=1
		fi
	done <"$WORK/tampered"
	if [ "$tampered_fail" = 1 ]; then
		echo "self-test ok: tampered baseline (agent-pool 999) detected as drift"
	else
		echo "FAIL: self-test did NOT detect the tampered baseline"
		fail=1
	fi
fi

exit "$fail"
