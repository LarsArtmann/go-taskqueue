#!/usr/bin/env bash
# Root-module verify gate with ONE flake-aware retry: build + vet + test
# -race at the repo root, re-running the test battery once when (and only
# when) the failure output matches a known load-flaky signature. The
# signature list and the retry mechanism live in scripts/lib/verify-retry.sh
# (extracted for the agent verify surface, TODO row 132; the AGENTS.md
# Known Issues list owns that list) — a test failing there fails
# at clean parents too under host build storms (TestExactlyOnceUnderConcurrency
# dropped to 19/20; TestSelfManagingLoop), so one retry is cheaper than
# re-adjudicating every red window by hand. A retry pass prints the
# FLAKE-RETRY marker so callers can see the signal was not clean.
#
# Not a ci-local replacement: the pre-push gate stays the full battery
# (scripts/ci-local.sh); this is the fast loop for one green signal.
#
# Receipt discipline: the gate's FULL output is teed to a dated receipt
# file (/var/tmp/tq-root-gate-<ts>.log; override with TQ_ROOT_GATE_RECEIPT)
# so closeouts cite a captured file instead of session scrollback. The
# receipt records failure output too — a red run leaves the same artifact.
# Retention (row 1729): the newest KEEP_RECEIPTS receipts are kept; older
# tq-root-gate-*.log siblings in the receipt's directory are deleted on
# every run so receipts cannot accumulate unboundedly across windows.
# TQ_ROOT_GATE_ROTATE_ONLY=1 runs ONLY the rotation then exits (the
# --self-test hook; also a manual sweep without running the gate).
set -euo pipefail
cd "$(dirname "$0")/.."

KEEP_RECEIPTS="${TQ_ROOT_GATE_KEEP_RECEIPTS:-20}"
self_abs="$(cd "$(dirname "$0")" && pwd)/$(basename "$0")"

rotate_receipts() {
	local current="$1" f
	local old
	# shellcheck disable=SC2012 # receipt filenames are machine-generated, never adversarial
	old="$(ls -1t "$(dirname "$current")"/tq-root-gate-*.log 2>/dev/null | tail -n +"$((KEEP_RECEIPTS + 1))" || true)"
	for f in $old; do
		[ "$f" = "$current" ] && continue
		rm -f "$f"
		echo "root-gate receipt rotated out: $f"
	done
}

self_test() {
	local dir out rc
	dir="$(mktemp -d)"
	touch -d '1 hour ago' "$dir/tq-root-gate-old1.log"
	touch -d '30 minutes ago' "$dir/tq-root-gate-old2.log"
	: >"$dir/tq-root-gate-now.log"
	# keep=2 over 3 receipts: oldest goes, newest old + current survive
	out="$(TQ_ROOT_GATE_ROTATE_ONLY=1 TQ_ROOT_GATE_KEEP_RECEIPTS=2 TQ_ROOT_GATE_RECEIPT="$dir/tq-root-gate-now.log" "$self_abs" 2>&1)" && rc=0 || rc=$?
	[ "$rc" -eq 0 ] || {
		echo "root-gate self-test FAIL: rotation rc=$rc: $out"
		rm -rf "$dir"
		exit 1
	}
	[ -f "$dir/tq-root-gate-now.log" ] || {
		echo "root-gate self-test FAIL: current receipt must survive rotation"
		rm -rf "$dir"
		exit 1
	}
	if [ -f "$dir/tq-root-gate-old1.log" ]; then
		echo "root-gate self-test FAIL: oldest receipt should be rotated out (keep=2 over 3 receipts)"
		rm -rf "$dir"
		exit 1
	fi
	[ -f "$dir/tq-root-gate-old2.log" ] || {
		echo "root-gate self-test FAIL: newest old receipt must survive (keep=2)"
		rm -rf "$dir"
		exit 1
	}
	# keep=1: everything but the current receipt goes
	TQ_ROOT_GATE_ROTATE_ONLY=1 TQ_ROOT_GATE_KEEP_RECEIPTS=1 TQ_ROOT_GATE_RECEIPT="$dir/tq-root-gate-now.log" "$self_abs" >/dev/null 2>&1
	if [ -f "$dir/tq-root-gate-old2.log" ]; then
		echo "root-gate self-test FAIL: only the current receipt should survive keep=1"
		rm -rf "$dir"
		exit 1
	fi
	[ -f "$dir/tq-root-gate-now.log" ] || {
		echo "root-gate self-test FAIL: current receipt lost at keep=1"
		rm -rf "$dir"
		exit 1
	}
	rm -rf "$dir"
	echo "root-gate self-test ok (receipt rotation keeps newest N, current survives)"
}

if [ "${1:-}" = "--self-test" ]; then
	self_test
	exit 0
fi

receipt="${TQ_ROOT_GATE_RECEIPT:-/var/tmp/tq-root-gate-$(date +%Y-%m-%d_%H-%M).log}"
rotate_receipts "$receipt"
if [ "${TQ_ROOT_GATE_ROTATE_ONLY:-}" = "1" ]; then
	: >"$receipt"
	exit 0
fi
exec > >(tee "$receipt") 2>&1
echo "root-gate receipt: $receipt"

export GOEXPERIMENT=jsonv2
export GOTOOLCHAIN=auto

# Cheap script-syntax guard (closeout 2026-10-08 §e3): bash -n over the
# release path's scripts, which the push-gate battery only syntax-checks at
# its very END (check-script-syntax.sh sits late in ci-local) — a slip in
# release.sh/lib otherwise costs a full battery run before it is caught.
for f in scripts/release.sh scripts/lib/*.sh "$0"; do
	bash -n "$f" || {
		echo "FAIL: bash -n $f" >&2
		exit 1
	}
done

. scripts/lib/verify-retry.sh

run_gate() {
	go build ./... && go vet ./... && go test ./... -race
}

run_with_flake_retry "ROOT-GATE" run_gate

echo "root-gate receipt: $receipt"
