#!/usr/bin/env bash
# Receipt helpers shared by the repo-scope gates (root-gate, ci-local): a
# gate's FULL output is teed to a dated /var/tmp receipt so closeouts cite
# a captured file instead of session scrollback (root-gate 2026-11-20, row
# 1730 parity). Retention: receipt_rotate keeps the newest KEEP receipts in
# the glob's directory and deletes older siblings — receipts must not
# accumulate unboundedly across windows (row 1729).
set -uo pipefail

receipt_rotate() {
	# receipt_rotate <current-receipt> <keep> <glob...> — the glob arrives
	# pre-expanded (unquoted at the call site), so collect all remaining args
	local current="$1" keep="$2" f
	shift 2
	local pattern="$*"
	local old
	# shellcheck disable=SC2012 # receipt filenames are machine-generated, never adversarial
	# shellcheck disable=SC2086 # $pattern is the pre-expanded glob words, split intentionally
	old="$(ls -1t $pattern 2>/dev/null | tail -n +"$((keep + 1))" || true)"
	for f in $old; do
		[ "$f" = "$current" ] && continue
		rm -f "$f"
		echo "receipt rotated out: $f"
	done
}

if [ "${1:-}" = "--self-test" ]; then
	dir="$(mktemp -d)"
	trap 'rm -rf "$dir"' EXIT
	: >"$dir/gate-old1.log"
	: >"$dir/gate-old2.log"
	touch -d '1 hour ago' "$dir/gate-old1.log"
	touch -d '30 minutes ago' "$dir/gate-old2.log"
	: >"$dir/gate-now.log"
	# keep=2 over 3 receipts: oldest goes, newest old + current survive
	receipt_rotate "$dir/gate-now.log" 2 "$dir"/gate-*.log
	[ -f "$dir/gate-now.log" ] || {
		echo "receipt self-test FAIL: current receipt must survive rotation"
		exit 1
	}
	if [ -f "$dir/gate-old1.log" ]; then
		echo "receipt self-test FAIL: oldest receipt should be rotated out (keep=2 over 3 receipts)"
		exit 1
	fi
	[ -f "$dir/gate-old2.log" ] || {
		echo "receipt self-test FAIL: newest old receipt must survive (keep=2)"
		exit 1
	}
	# keep=1: everything but the current receipt goes
	receipt_rotate "$dir/gate-now.log" 1 "$dir"/gate-*.log
	if [ -f "$dir/gate-old2.log" ]; then
		echo "receipt self-test FAIL: only the current receipt should survive keep=1"
		exit 1
	fi
	# current is never its own victim even when it matches the keep boundary
	receipt_rotate "$dir/gate-now.log" 1 "$dir"/gate-*.log
	[ -f "$dir/gate-now.log" ] || {
		echo "receipt self-test FAIL: current receipt lost at keep boundary"
		exit 1
	}
	echo "receipt self-test ok (rotation keeps newest N, current survives, keep boundary safe)"
	exit 0
fi
