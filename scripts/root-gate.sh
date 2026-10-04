#!/usr/bin/env bash
# Root-module verify gate with ONE flake-aware retry: build + vet + test
# -race at the repo root, re-running the test battery once when (and only
# when) the failure output matches a known load-flaky signature. The
# AGENTS.md Known Issues list owns that list — a test failing there fails
# at clean parents too under host build storms (TestExactlyOnceUnderConcurrency
# dropped to 19/20; TestSelfManagingLoop), so one retry is cheaper than
# re-adjudicating every red window by hand. A retry pass prints the
# FLAKE-RETRY marker so callers can see the signal was not clean.
#
# Not a ci-local replacement: the pre-push gate stays the full battery
# (scripts/ci-local.sh); this is the fast loop for one green signal.
set -euo pipefail
cd "$(dirname "$0")/.."

export GOEXPERIMENT=jsonv2
export GOTOOLCHAIN=auto

known_flaky=(
	"TestExactlyOnceUnderConcurrency"
	"TestSelfManagingLoop"
)

run_gate() {
	go build ./... && go vet ./... && go test ./... -race
}

flaky_signature() {
	local out=$1
	local name
	for name in "${known_flaky[@]}"; do
		if grep -qF -- "$name" <<<"$out"; then
			return 0
		fi
	done

	return 1
}

if out="$(mktemp)" && run_gate 2>&1 | tee "$out"; then
	rm -f "$out"
	exit 0
fi

if flaky_signature "$(cat "$out")"; then
	echo "ROOT-GATE: known-flaky signature in the failure output; retrying ONCE"
	if run_gate; then
		echo "ROOT-GATE: FLAKE-RETRY — green on the second run (known-flaky signature)"
		rm -f "$out"
		exit 0
	fi

	echo "ROOT-GATE: still red after the flake retry — a real failure"
fi

rm -f "$out"
exit 1
