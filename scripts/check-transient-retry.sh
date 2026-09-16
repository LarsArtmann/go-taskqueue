#!/usr/bin/env bash
# Durable regression pin for with_transient_retry in scripts/ci-local.sh
# (01-46 report f2: the authoring-time functional test was a throwaway /tmp
# exercise that verified the shipped bytes once and protected nothing going
# forward — an edit breaking the poll counter or the context message had no
# gate to catch it). The helper is sed-extracted from the SHIPPED file, so
# the pin always exercises the bytes the pre-push gate actually runs; a copy
# here would drift silently. Marker assertions guard the extraction itself:
# if the sed range stops matching (helper renamed or reshaped), the pin
# fails loudly instead of testing the wrong text.
#
# Wired as a ci-local step before the Go gates — a broken retry loop must
# fail here, not mid-run. TRANSIENT_POLL_SECS=0 keeps the whole pin
# sub-second. The smoke-side wrapper extension stays a separate TODO row
# (runtime-budget ruling pending, 01-46 g3).
set -euo pipefail
cd "$(dirname "$0")/.."

ci_local=scripts/ci-local.sh
[ -f "$ci_local" ] || {
	echo "FAIL: $ci_local missing"
	exit 1
}

extracted="$(sed -n '/^TRANSIENT_POLL_SECS=/,/^}$/p' "$ci_local")"
fail=0
for marker in \
	'with_transient_retry()' \
	'retry poll' \
	'still failing after' \
	'let it land (or re-run this gate on a quiet tree)' \
	'concurrent edit in flight'; do
	if ! grep -qF -- "$marker" <<<"$extracted"; then
		echo "FAIL: extraction lost marker '$marker' — the sed range no longer matches the shipped helper"
		fail=1
	fi
done
if [ "$fail" -ne 0 ]; then
	exit 1
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

# Shipped budget stays 45s x 3 (the 15-39 f9/e2 numbers) — the defaults are
# part of the contract, not an implementation detail.
defaults="$(eval "$extracted" && printf '%s/%s' "$TRANSIENT_POLL_SECS" "$TRANSIENT_MAX_POLLS")"
if [ "$defaults" = "45/3" ]; then
	echo "ok: shipped defaults are 45s x 3 polls"
else
	echo "FAIL: shipped defaults drifted: '$defaults' (want 45/3)"
	exit 1
fi

# run_case <name> <want_rc> <want_warns> <want_runs> <wrapper invocation...>:
# run the invocation in a subshell that first evals the extracted helper,
# then count WARN and GATE-RUN lines. Full output is kept in last_out for
# follow-up must_mention assertions.
last_out=""
run_case() {
	local name="$1" want_rc="$2" want_warns="$3" want_runs="$4"
	shift 4
	local rc=0 warns runs
	last_out="$(
		export TRANSIENT_POLL_SECS=0 TRANSIENT_MAX_POLLS=3
		eval "$extracted"
		"$@"
	)" || rc=$?
	warns="$(grep -c '^WARN:' <<<"$last_out" || true)"
	runs="$(grep -c '^GATE-RUN$' <<<"$last_out" || true)"
	if [ "$rc" -ne "$want_rc" ] || [ "$warns" -ne "$want_warns" ] || [ "$runs" -ne "$want_runs" ]; then
		echo "FAIL: $name — want rc=$want_rc warns=$want_warns runs=$want_runs, got rc=$rc warns=$warns runs=$runs"
		printf '%s\n' "$last_out"
		return 1
	fi
	echo "ok: $name (rc=$rc, warns=$warns, runs=$runs)"
}

must_mention() {
	local name="$1" pattern="$2"
	if grep -qF -- "$pattern" <<<"$last_out"; then
		echo "ok: $name mentions the expected text"
	else
		echo "FAIL: $name — output is missing '$pattern'"
		printf '%s\n' "$last_out"
		return 1
	fi
}

run_case "plain gate passes first try (no WARN, no poll)" 0 0 1 \
	with_transient_retry "pin plain-success" printf 'GATE-RUN\n'

heal_stamp="$tmp/heal-stamp"
run_case "compound gate heals on first retry (real module-loop shape)" 0 1 2 \
	with_transient_retry "pin heal" bash -c 'printf "GATE-RUN\n"; if [ -e "$1" ]; then exit 0; fi; touch "$1"; exit 1' _ "$heal_stamp"
must_mention "heal case" 'possible concurrent edit in flight'

run_case "always-red gate exhausts 3 polls then fails" 1 3 4 \
	with_transient_retry "pin exhaust" bash -c 'printf "GATE-RUN\n"; exit 1'
must_mention "exhaust case" 'still failing after 3 retry polls'
must_mention "exhaust case" 'let it land (or re-run this gate on a quiet tree) before judging the work'

# TRANSIENT_MAX_POLLS=0 escape hatch: immediate fail, zero polls — the
# pre-wrapper behavior must stay reachable (01-46 f14).
rc_max0=0
max0_out="$(
	export TRANSIENT_POLL_SECS=0 TRANSIENT_MAX_POLLS=0
	eval "$extracted"
	with_transient_retry "pin max0" bash -c 'printf "GATE-RUN\n"; exit 1'
)" || rc_max0=$?
max0_runs="$(grep -c '^GATE-RUN$' <<<"$max0_out" || true)"
if [ "$rc_max0" -eq 1 ] && [ "$max0_runs" -eq 1 ] && ! grep -q '^WARN:' <<<"$max0_out"; then
	echo "ok: TRANSIENT_MAX_POLLS=0 escape hatch fails immediately (rc=1, runs=1, no polls)"
else
	echo "FAIL: TRANSIENT_MAX_POLLS=0 escape hatch — want rc=1, 1 run, 0 WARNs; got rc=$rc_max0 runs=$max0_runs"
	printf '%s\n' "$max0_out"
	exit 1
fi

echo "transient-retry self-test ok (extraction pinned, defaults 45/3, heal/exhaust/escape-hatch semantics verified)"
