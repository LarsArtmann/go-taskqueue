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
#   5. KNOWN_FLAKY ↔ AGENTS.md cross-check (03-47 §e4 + 2026-10-13 01-54
#      closeout §e, bidirectional): every "is load-flaky" Known Issues
#      mention appears in the lib's list AND every list entry has a Known
#      Issues mention — plus (§f9) a self-test that a bullet reworded
#      away from "is load-flaky" trips the empty-extraction FAIL.
#   6. scripts/verify.sh's genuine-red path in situ (04-55 report §b2):
#      under an always-failing NON-signature go shim it propagates rc=1
#      with exactly one go invocation and zero retry lines — at the BUILD
#      stage and (01-54 closeout §f7) with build+vet passing and the
#      failure at the TEST stage.
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

# Pin 3: the lib keeps the retry semantics' surface strings. The literal
# KNOWN_FLAKY names (TestExactlyOnceUnderConcurrency/TestSelfManagingLoop)
# were DROPPED here (2026-10-13 02-08 follow-up 5 audit): pin 6's AGENTS.md→lib
# extraction subsumes them — every '<Test> is load-flaky' bullet must appear
# in the lib's KNOWN_FLAKY array, and a bullet reworded away trips the
# empty-extraction self-test below — so the literal greps only duplicated
# coverage while needing manual edits on every legitimate test rename.
for marker in \
	'run_with_flake_retry()' \
	'known-flaky signature in the failure output; retrying ONCE' \
	'FLAKE-RETRY' \
	'still red after the flake retry'; do
	if ! grep -qF -- "$marker" "$lib"; then
		echo "FAIL: $lib lost marker '$marker'"
		fail=1
	fi
done

# Pin 6 (03-47 report §e4): the KNOWN_FLAKY list stays pinned to the
# AGENTS.md Known Issues flake mentions — any test named "<Test> is
# load-flaky" there must appear in the lib's list (one grep gate; the
# Known Issues list owns which signatures get the ONE retry).
while IFS= read -r known_flake; do
	[ -n "$known_flake" ] || continue
	# The name must sit INSIDE the KNOWN_FLAKY array — not merely anywhere in
	# the lib (a stale comment mention must not satisfy the pin, so dropping
	# the array entry stays a red gate).
	if ! sed -n '/^KNOWN_FLAKY=(/,/^)/p' "$lib" | grep -qF -- "$known_flake"; then
		echo "FAIL: AGENTS.md Known Issues flake '$known_flake' is missing from $lib KNOWN_FLAKY"
		fail=1
	fi
done < <(grep -oE '`?Test[A-Za-z0-9_]+`? is load-flaky' AGENTS.md | grep -oE 'Test[A-Za-z0-9_]+')
if ! grep -qE 'is load-flaky' AGENTS.md; then
	echo "FAIL: no '<Test> is load-flaky' bullet found in AGENTS.md — the KNOWN_FLAKY pin has nothing to bind to"
	fail=1
fi
# Bidirectional (2026-10-13 01-54 closeout §e): every lib KNOWN_FLAKY entry
# must also have an AGENTS.md "<Test> is load-flaky" mention — the Known
# Issues list stays the single owner of the retry ladder in both directions.
while IFS= read -r listed; do
	[ -n "$listed" ] || continue
	if ! grep -qE "[\`']?${listed}[\`']? is load-flaky" AGENTS.md; then
		echo "FAIL: $lib KNOWN_FLAKY entry '$listed' has no '<Test> is load-flaky' mention in AGENTS.md"
		fail=1
	fi
done < <(sed -n '/^KNOWN_FLAKY=(/,/^)/p' "$lib" | grep -oE '"Test[A-Za-z0-9_]+"' | tr -d '"')
if ! sed -n '/^KNOWN_FLAKY=(/,/^)/p' "$lib" | grep -qE '"Test[A-Za-z0-9_]+"'; then
	echo "FAIL: $lib KNOWN_FLAKY list is empty — the bidirectional pin has nothing to bind to"
	fail=1
fi

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

# Pin 6 self-test (2026-10-13 01-54 closeout §f9): the extraction regex's
# empty result must NOT pass silently — a Known Issues bullet reworded away
# from "is load-flaky" must trip the "nothing to bind to" FAIL. Re-run this
# script from a temp tree whose AGENTS.md copy has every "is load-flaky"
# mention reworded; the gate must exit 1 printing the empty-extraction
# failure message.
st_tree="$tmp/st-tree"
mkdir -p "$st_tree/scripts/lib"
cp "$0" "$st_tree/scripts/check-verify.sh"
cp "$lib" "$st_tree/scripts/lib/verify-retry.sh"
cp "$verify" "$st_tree/scripts/verify.sh"
cp "$root_gate" "$st_tree/scripts/root-gate.sh"
cp "$tq_verify" "$st_tree/.tq-verify"
sed 's/is load-flaky/was load-flaky (reworded)/g' AGENTS.md >"$st_tree/AGENTS.md"
st_out="$(bash "$st_tree/scripts/check-verify.sh" 2>&1)" && st_rc=0 || st_rc=$?
if [ "$st_rc" -ne 1 ]; then
	echo "FAIL: empty-extraction self-test (want rc=1, got rc=$st_rc) — a reworded Known Issues bullet would silently unbind the KNOWN_FLAKY pin"
	printf '%s\n' "$st_out"
	exit 1
fi
st_msg="no '<Test> is load-flaky' bullet found in AGENTS.md — the KNOWN_FLAKY pin has nothing to bind to"
if ! grep -qF -- "$st_msg" <<<"$st_out"; then
	echo "FAIL: empty-extraction self-test — gate exited 1 but without the expected message: $st_msg"
	printf '%s\n' "$st_out"
	exit 1
fi
echo "ok: pin 6 empty-extraction failure message fires on a reworded Known Issues bullet"

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
must_mention "non-signature case" 'FAIL: undefined: atomic'

# Pin 5: run the SHIPPED scripts/verify.sh itself through a scripted
# flake-heal. The battery is hardcoded, so the sandbox is PATH: a shimmed
# go fails the FIRST invocation with a known-flaky signature and passes
# every later one; a shimmed gofmt keeps the green retry hermetic and
# sub-second (no repo-wide gofmt walk, no real toolchain). This executes
# the actual wrapper file's ONE-retry path: rc=0 plus the first-run
# signature, the label-prefixed retry notice, and the FLAKE-RETRY line
# must all appear in verify.sh's own output.

# make_shim_pair <dir> <<'EOF' <go-body> EOF (row 216): the per-pin go/gofmt
# pair was hand-rolled three times; the go body stays per-pin (heredoc on
# the call site), the always-green gofmt is shared boilerplate.
make_shim_pair() {
	local dir=$1
	mkdir -p "$dir"
	cat >"$dir/go"
	printf '#!/usr/bin/env bash\nexit 0\n' >"$dir/gofmt"
	chmod +x "$dir/go" "$dir/gofmt"
}

shim_bin="$tmp/shim-bin"
# pins: retry-once-branch — first red run then stamp-file green; rc=0 + retry notice + FLAKE-RETRY line in verify.sh output
make_shim_pair "$shim_bin" <<'EOF'
#!/usr/bin/env bash
stamp="${GO_SHIM_STAMP:-}"
if [ -n "$stamp" ] && [ -e "$stamp" ]; then
	exit 0
fi
printf 'FAIL: TestSelfManagingLoop 19/20\n' >&2
if [ -n "$stamp" ]; then : >"$stamp"; fi
exit 1
EOF

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

# Pin 7 (04-55 report §b2): the wrapper's GENUINE-red path in situ —
# verify.sh under an always-failing NON-signature go shim must propagate
# rc=1 with exactly ONE go invocation (build fails, no retry, no second
# run) and zero retry lines. The lib-level non-signature case above pins
# the lib; this executes the actual wrapper's rc=1 propagation.
red_bin="$tmp/red-bin"
red_count="$tmp/go-invocations"
: >"$red_count"
# pins: hard-red-branch — persistently failing go must rc=1 with NO retry lines (red is not flake)
make_shim_pair "$red_bin" <<EOF
#!/usr/bin/env bash
printf '%s\n' 'FAIL: undefined: atomic.Lane' >&2
echo go >>"$red_count"
exit 1
EOF

last_out="$(PATH="$red_bin:$PATH" bash "$verify" 2>&1)" && red_rc=0 || red_rc=$?
if [ "$red_rc" -ne 1 ]; then
	echo "FAIL: in-situ verify.sh genuine-red (want rc=1, got rc=$red_rc)"
	printf '%s\n' "$last_out"
	exit 1
fi
red_invocations="$(wc -l <"$red_count")"
if [ "$red_invocations" -ne 1 ]; then
	echo "FAIL: in-situ genuine-red go invocations (want 1, got $red_invocations)"
	exit 1
fi
must_not_mention "in-situ genuine-red case" 'retrying ONCE'
must_not_mention "in-situ genuine-red case" 'FLAKE-RETRY'
must_mention "in-situ genuine-red case" 'FAIL: undefined: atomic.Lane'
echo "ok: in-situ verify.sh genuine-red (rc=1, one go invocation, zero retry lines)"

# Pin 7 variant (2026-10-13 01-54 closeout §f7): the same genuine-red path
# with the failure at the TEST stage — the shim lets build+vet pass (exit 0
# for every non-test invocation) and fails `go test` with a NON-signature
# line. rc=1 must propagate with exactly one go-test invocation and zero
# retry lines, so the battery's later legs are covered by the pin too.
test_bin="$tmp/test-bin"
test_count="$tmp/go-test-invocations"
: >"$test_count"
# pins: flaky-test-branch — go test FAIL retried once per the KNOWN_FLAKY signature, rc=0 on second pass
make_shim_pair "$test_bin" <<EOF
#!/usr/bin/env bash
if [ "\$1" = "test" ]; then
	printf '%s\n' 'FAIL: TestSomethingReal 3/17' >&2
	echo test >>"$test_count"
	exit 1
fi
exit 0
EOF

last_out="$(PATH="$test_bin:$PATH" bash "$verify" 2>&1)" && test_rc=0 || test_rc=$?
if [ "$test_rc" -ne 1 ]; then
	echo "FAIL: in-situ verify.sh test-stage genuine-red (want rc=1, got rc=$test_rc)"
	printf '%s\n' "$last_out"
	exit 1
fi
test_invocations="$(wc -l <"$test_count")"
if [ "$test_invocations" -ne 1 ]; then
	echo "FAIL: in-situ test-stage genuine-red go test invocations (want 1, got $test_invocations)"
	exit 1
fi
must_mention "in-situ test-stage case" 'FAIL: TestSomethingReal 3/17'
must_not_mention "in-situ test-stage case" 'retrying ONCE'
must_not_mention "in-situ test-stage case" 'FLAKE-RETRY'
echo "ok: in-situ verify.sh genuine-red at the TEST stage (rc=1, one go test invocation, zero retry lines)"

echo "verify self-test ok (battery parity with .tq-verify, wrapper + lib markers, root-gate extraction, green/heal/exhaust/non-signature semantics, in-situ verify.sh flake-heal verified, KNOWN_FLAKY↔AGENTS.md bidirectional cross-check (+ reworded-bullet empty-extraction self-test), in-situ genuine-red rc=1 no-retry verified (build-stage + test-stage))"
