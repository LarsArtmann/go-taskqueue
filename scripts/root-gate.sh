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
set -euo pipefail
cd "$(dirname "$0")/.."

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
