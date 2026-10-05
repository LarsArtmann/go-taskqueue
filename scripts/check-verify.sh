#!/usr/bin/env bash
# Self-test for the shared verify wrapper (TODO row 132), pinned against the
# SHIPPED bytes:
#   1. scripts/verify.sh's battery line is byte-identical to the
#      `.tq-verify` gate line — an owner flip of `.tq-verify` to the wrapper
#      (02-11 report §g2) must change only the retry semantics, never what
#      the gate runs; drift fails here instead of at the next agent gate.
#   2. scripts/lib/verify-retry.sh keeps the extracted retry semantics:
#      green-first-try single run, flaky-signature heal on the ONE retry
#      (FLAKE-RETRY marker), flaky-signature exhaustion, and non-signature
#      failures that do NOT retry — plus the KNOWN_FLAKY list.
#   3. scripts/root-gate.sh consumes the shared lib instead of a private
#      signature copy (the extraction must not silently re-duplicate).
#   4. scripts/verify.sh itself, executed in situ under a PATH-shimmed
#      toolchain, heals a scripted flake-signature first-run failure on
#      its ONE retry (03-47 report §b2): the wrapper file's own retry
#      path runs, not just the lib's behind it.
# Pattern: scripts/check-transient-retry.sh (marker-guarded, sub-second).
set -euo pipefail
cd "$(dirname "$0")/.."

lib=scripts/lib/verify-retry.sh
verify=scripts/verify.sh
tq_verify=.tq-verify
root_gate=scripts/root-gate.sh
for f in "$lib" "$verify" "$tq_verify" "$root_gate"; do
	[ -f "$f" ] || {
		echo "FAIL: $f missing"
		exit 1
	}
done

fail=0

# Pin 1: the wrapper's battery line IS the `.tq-verify` line.
battery="$(sed -n 's/^verify_gate() { //; s/; }$//p' "$verify")"
shipped="$(cat "$tq_verify")"
if [ -z "$battery" ]; then
	echo "FAIL: verify_gate battery line not found in $verify — the sed range no longer matches"
	fail=1
elif [ "$battery" = "$shipped" ]; then
	echo "ok: verify.sh battery is byte-identical to .tq-verify"
else
	echo "FAIL: verify.sh battery drifted from .tq-verify:"
	echo "  verify.sh:  $battery"
	echo "  .tq-verify: $shipped"
	fail=1
fi

# Pin 2: the wrapper rides the shared retry.
for marker in \
	'run_with_flake_retry "VERIFY" verify_gate' \
	'scripts/lib/verify-retry.sh'; do
	if ! grep -qF -- "$marker" "$verify"; then
		echo "FAIL: $verify lost marker '$marker'"
		fail=1
	fi
done

# Pin 3: the lib keeps the retry semantics' surface strings and the list.
for marker in \
	'run_with_flake_retry()' \
	'known-flaky signature in the failure output; retrying ONCE' \
	'FLAKE-RETRY' \
	'still red after the flake retry' \
	'TestExactlyOnceUnderConcurrency' \
	'TestSelfManagingLoop'; do
	if ! grep -qF -- "$marker" "$lib"; then
		echo "FAIL: $lib lost marker '$marker'"
		fail=1
	fi
done

# Pin 4: root-gate.sh consumes the lib, no private signature copy.
if grep -qE '^(known_flaky|KNOWN_FLAKY)=' "$root_gate"; then
	echo "FAIL: $root_gate still defines a private flaky-signature list — the extraction regressed"
	fail=1
fi
if ! grep -qF 'scripts/lib/verify-retry.sh' "$root_gate"; then
	echo "FAIL: $root_gate no longer sources the shared retry lib"
	fail=1
fi

if [ "$fail" -ne 0 ]; then
	exit 1
fi

# shellcheck source=lib/verify-retry.sh
. "$lib"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

# Behavior pins: run the SHIPPED lib bytes through the four contract cases
# (the lib is sourced once above; each case calls it in a fresh subshell).
# run_case <name> <want_rc> <want_runs> <wrapper invocation...>: run the
# invocation in a subshell that first sources the shipped lib, then count
# GATE-RUN lines. Full output is kept in last_out for follow-up
# must_mention / must_not_mention assertions.
last_out=""
run_case() {
	local name="$1" want_rc="$2" want_runs="$3"
	shift 3
	local rc=0 runs
	last_out="$(
		run_with_flake_retry "PIN" "$@"
	)" || rc=$?
	runs="$(grep -c '^GATE-RUN$' <<<"$last_out" || true)"
	if [ "$rc" -ne "$want_rc" ] || [ "$runs" -ne "$want_runs" ]; then
		echo "FAIL: $name — want rc=$want_rc runs=$want_runs, got rc=$rc runs=$runs"
		printf '%s\n' "$last_out"
		exit 1
	fi
	echo "ok: $name (rc=$rc, runs=$runs)"
}

must_mention() {
	local name="$1" pattern="$2"
	if grep -qF -- "$pattern" <<<"$last_out"; then
		echo "ok: $name mentions the expected text"
	else
		echo "FAIL: $name — output is missing '$pattern'"
		printf '%s\n' "$last_out"
		exit 1
	fi
}

must_not_mention() {
	local name="$1" pattern="$2"
	if grep -qF -- "$pattern" <<<"$last_out"; then
		echo "FAIL: $name — output unexpectedly contains '$pattern'"
		printf '%s\n' "$last_out"
		exit 1
	fi
	echo "ok: $name does not mention '$pattern'"
}

run_case "green gate passes first try (single run, no retry)" 0 1 \
	bash -c 'printf "GATE-RUN\n"'
must_not_mention "green case" 'retrying ONCE'
must_not_mention "green case" 'FLAKE-RETRY'

run_case "flaky signature heals on the ONE retry" 0 2 \
	bash -c 'printf "GATE-RUN\n"; if [ -e "$1" ]; then exit 0; fi; printf "FAIL: TestSelfManagingLoop 19/20\n"; touch "$1"; exit 1' _ "$tmp/heal-stamp"
must_mention "heal case" 'known-flaky signature in the failure output; retrying ONCE'
must_mention "heal case" 'FLAKE-RETRY — green on the second run (known-flaky signature)'

run_case "flaky signature still red after the retry" 1 2 \
	bash -c 'printf "GATE-RUN\nFAIL: TestExactlyOnceUnderConcurrency\n"; exit 1'
must_mention "exhaust case" 'still red after the flake retry — a real failure'

run_case "non-signature failure does not retry" 1 1 \
	bash -c 'printf "GATE-RUN\nFAIL: undefined: atomic\n"; exit 1'
must_not_mention "non-signature case" 'retrying ONCE'
must_not_mention "non-signature case" 'FLAKE-RETRY'

# Pin 5: run the SHIPPED scripts/verify.sh itself through a scripted
# flake-heal. The battery is hardcoded, so the sandbox is PATH: a shimmed
# go fails the FIRST invocation with a known-flaky signature and passes
# every later one; a shimmed gofmt keeps the green retry hermetic and
# sub-second (no repo-wide gofmt walk, no real toolchain). This executes
# the actual wrapper file's ONE-retry path: rc=0 plus the first-run
# signature, the label-prefixed retry notice, and the FLAKE-RETRY line
# must all appear in verify.sh's own output.
shim_bin="$tmp/shim-bin"
mkdir -p "$shim_bin"
cat >"$shim_bin/go" <<'EOF'
#!/usr/bin/env bash
stamp="${GO_SHIM_STAMP:-}"
if [ -n "$stamp" ] && [ -e "$stamp" ]; then
	exit 0
fi
printf 'FAIL: TestSelfManagingLoop 19/20\n' >&2
if [ -n "$stamp" ]; then : >"$stamp"; fi
exit 1
EOF
printf '#!/usr/bin/env bash\nexit 0\n' >"$shim_bin/gofmt"
chmod +x "$shim_bin/go" "$shim_bin/gofmt"

last_out="$(GO_SHIM_STAMP="$tmp/verify-shim-stamp" PATH="$shim_bin:$PATH" bash "$verify" 2>&1)" && heal_rc=0 || heal_rc=$?
if [ "$heal_rc" -ne 0 ]; then
	echo "FAIL: in-situ verify.sh flake-heal (want rc=0, got rc=$heal_rc)"
	printf '%s\n' "$last_out"
	exit 1
fi
echo "ok: in-situ verify.sh flake-heal (rc=0)"
must_mention "in-situ heal case" 'FAIL: TestSelfManagingLoop 19/20'
must_mention "in-situ heal case" 'VERIFY: known-flaky signature in the failure output; retrying ONCE'
must_mention "in-situ heal case" 'VERIFY: FLAKE-RETRY — green on the second run (known-flaky signature)'

echo "verify self-test ok (battery parity with .tq-verify, wrapper + lib markers, root-gate extraction, green/heal/exhaust/non-signature semantics, in-situ verify.sh flake-heal verified)"
