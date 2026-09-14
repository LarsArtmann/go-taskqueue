#!/usr/bin/env bash
# Help-text smoke: run every tq subcommand's help (minimum tq dlq -h) and
# assert no parenthesized-identifier artifacts in flag strings — the class
# where a variable-rename regex leaked into a help literal and printed
# "rescued task(store)" instead of "rescued task(s)" (08:42 report e3/f2).
#
# Artifact pattern: an identifier attached DIRECTLY to a preceding word,
# e.g. task(store) — the intended plural "(s)" and flag-package
# "(default 3)" both have a space or single letter and are exempt.
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

# Scratch DB: tq inherits TQ_DB from the environment and must never touch
# production. (Help output does not need the DB, but the binary is run.)
export TQ_DB="$WORK/scratch.db"

failed=0
check_help() {
	local label="$1" out="$2"
	if grep -Eq '[A-Za-z0-9]\([a-zA-Z][a-zA-Z0-9]+\)' <<<"$out"; then
		echo "FAIL: $label help carries a parenthesized-identifier artifact:"
		grep -En '[A-Za-z0-9]\([a-zA-Z][a-zA-Z0-9]+\)' <<<"$out"
		failed=1
	fi
}

root_help="$("$TQ" -h 2>&1)" || { echo "FAIL: tq -h failed"; exit 1; }
check_help "tq" "$root_help"

# Subcommands (kept in sync with the Usage block of `tq -h`; the explicit
# dlq line below is the work item's minimum coverage regardless).
cmds="enqueue worker harvest bootstrap agent-pool stats tasks audit doctor top show dlq cancel facts tail serve api version"

for cmd in $cmds; do
	out="$("$TQ" "$cmd" -h 2>&1)" || { echo "FAIL: tq $cmd -h failed"; failed=1; continue; }
	check_help "tq $cmd" "$out"
done

# Grouped subcommands reject a bare -h; probe the leaf commands instead.
for cmd in "watermarks show" "watermarks set" "session begin" "session close"; do
	out="$("$TQ" $cmd -h 2>&1)" || { echo "FAIL: tq $cmd -h failed"; failed=1; continue; }
	check_help "tq $cmd" "$out"
done

# verdict prints its usage but exits 1 on -h; check the output anyway.
out="$("$TQ" verdict -h 2>&1)"
check_help "tq verdict" "$out"

# Explicit minimum coverage per the work item, independent of parsing.
out="$("$TQ" dlq -h 2>&1)" || { echo "FAIL: tq dlq -h failed"; exit 1; }
check_help "tq dlq" "$out"

if [ "$failed" -ne 0 ]; then
	echo "help-text smoke: FAIL"
	exit 1
fi
echo "help-text smoke: PASS ($(wc -w <<<"$cmds") subcommands checked)"
