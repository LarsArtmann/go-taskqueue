#!/usr/bin/env bash
# Shared known-flaky-signature retry (TODO row 132, extracted from
# scripts/root-gate.sh): ONE retry mechanism and ONE signature list for every
# verify surface, so a load-flake the repo already cures never burns an agent
# attempt plus a re-dispatch cycle again (02-11 report §d4). The AGENTS.md
# Known Issues list owns which tests are listed here. Sourced by
# scripts/root-gate.sh and scripts/verify.sh; behavior pinned by
# scripts/check-verify.sh.

# Failure output mentioning one of these tests is load-flaky (fails at clean
# parents too under host build storms), so one retry is cheaper than
# re-adjudicating every red window by hand.
KNOWN_FLAKY=(
	"TestExactlyOnceUnderConcurrency"
	"TestSelfManagingLoop"
)

# flaky_signature <output>: rc 0 when a known load-flaky signature appears.
flaky_signature() {
	local out=$1
	local name
	for name in "${KNOWN_FLAKY[@]}"; do
		if grep -qF -- "$name" <<<"$out"; then
			return 0
		fi
	done

	return 1
}

# run_with_flake_retry <label> <cmd...>: run the gate command with live
# output; on a first-run failure matching a known-flaky signature, retry
# ONCE. A retry pass prints the FLAKE-RETRY marker so callers can see the
# signal was not clean.
run_with_flake_retry() {
	local label="$1"
	shift
	local out
	out="$(mktemp)"
	if "$@" 2>&1 | tee "$out"; then
		rm -f "$out"
		return 0
	fi

	if flaky_signature "$(cat "$out")"; then
		echo "$label: known-flaky signature in the failure output; retrying ONCE"
		if "$@"; then
			echo "$label: FLAKE-RETRY — green on the second run (known-flaky signature)"
			rm -f "$out"
			return 0
		fi

		echo "$label: still red after the flake retry — a real failure"
	fi

	rm -f "$out"
	return 1
}
