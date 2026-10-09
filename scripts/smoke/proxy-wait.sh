#!/usr/bin/env bash
# Proxy-wait smoke: offline behavior test for wait_for_proxy_version
# (scripts/lib/proxy-wait.sh, extracted from release.sh). bash -n covers
# syntax only; this drives BOTH die branches — network-dead (every .info
# poke fails) vs proxy lag (pokes succeed, @v/list never lists) — plus the
# recovery path, with the probe seams faked and zero network access.
set -euo pipefail
cd "$(dirname "$0")/../.."

# Structural offline claim: re-exec the whole smoke inside an empty network
# namespace (unshare -n) where available, so "zero network access" holds
# even if a fake regresses into real curl/go-list. Skip with a loud line
# where unshare is unavailable (fakes still keep the smoke offline).
if [ "${TQ_SMOKE_NETNS:-}" != "1" ]; then
	if command -v unshare >/dev/null 2>&1 && unshare -n true 2>/dev/null; then
		echo "== re-exec under unshare -n (structural offline) =="
		exec env TQ_SMOKE_NETNS=1 unshare -n "$0" "$@"
	fi
	echo "SKIP: unshare -n unavailable (or no permission) — offline claim is NOT structural; proceeding with faked seams only"
fi

# shellcheck source=scripts/lib/proxy-wait.sh
source scripts/lib/proxy-wait.sh

# Expected die messages come from the lib's constants (single source of
# truth) — no substring greps.
die_lag="$(printf "$TQ_PROXY_DIE_LAG" vX.Y.Z example.com/module vX.Y.Z)"
die_dead="$TQ_PROXY_DIE_DEAD"

fails=0

# drive <poke-prog> <list-prog> — runs the loop in a subshell with faked
# seams (3 attempts, no sleep); die is overridden to print "DIE: ..." and
# exit 42. Prints combined output; rc is the loop's outcome.
drive() { # <poke-prog> <list-prog>
	FAKE_POKE="$1" FAKE_LIST="$2" bash -c '
		set -euo pipefail
		source scripts/lib/proxy-wait.sh
		tq_proxy_poke() { eval "$FAKE_POKE"; }
		tq_proxy_lists() { eval "$FAKE_LIST"; }
		die() {
			echo "DIE: $*"
			exit 42
		}
		wait_for_proxy_version example.com/module vX.Y.Z 3 0
	' 2>&1
}

expect_die() { # <name> <msg-substring> <poke-prog> <list-prog>
	local out rc
	out="$(drive "$3" "$4")" && rc=0 || rc=$?
	if [ "$rc" = 42 ] && grep -qF "$2" <<<"$out"; then
		echo "ok   $1"
	else
		echo "FAIL $1 (rc=$rc, wanted die with: $2)"
		echo "$out"
		fails=$((fails + 1))
	fi
}

expect_ok() { # <name> <poke-prog> <list-prog>
	local out rc
	out="$(drive "$2" "$3")" && rc=0 || rc=$?
	if [ "$rc" = 0 ] && ! grep -q '^DIE:' <<<"$out" && grep -q "proxy serves" <<<"$out"; then
		echo "ok   $1"
	else
		echo "FAIL $1 (rc=$rc)"
		echo "$out"
		fails=$((fails + 1))
	fi
}

expect_die "network-dead: transport failure (code 0)" "$die_dead" \
	'PROXY_POKE_HTTP_CODE=0; return 1' 'return 1'
expect_die "network-dead: 5xx pokes" "$die_dead" \
	'PROXY_POKE_HTTP_CODE=503; return 1' 'return 1'
expect_die "proxy lag: 404 pokes (reachable, not ingested)" "$die_lag" \
	'PROXY_POKE_HTTP_CODE=404; return 1' 'return 1'
expect_die "proxy lag: pokes ok, never listed" "$die_lag" \
	'PROXY_POKE_HTTP_CODE=200; return 0' 'return 1'
expect_die "lag wins when the LAST poke ok: 404,404,200" \
	"$die_lag" \
	'poke_n=$(( ${poke_n:-0} + 1 )); if [ "$poke_n" -ge 2 ]; then PROXY_POKE_HTTP_CODE=200; return 0; else PROXY_POKE_HTTP_CODE=404; return 1; fi' 'return 1'
expect_die "network-dead wins on LAST attempt: 404 then transport-dead" "$die_dead" \
	'poke_n=$(( ${poke_n:-0} + 1 )); if [ "$poke_n" -eq 1 ]; then PROXY_POKE_HTTP_CODE=404; return 1; else PROXY_POKE_HTTP_CODE=0; return 1; fi' 'return 1'
expect_die "proxy lag wins on LAST attempt: transport-dead then 404" "$die_lag" \
	'poke_n=$(( ${poke_n:-0} + 1 )); if [ "$poke_n" -eq 1 ]; then PROXY_POKE_HTTP_CODE=0; return 1; else PROXY_POKE_HTTP_CODE=404; return 1; fi' 'return 1'
expect_ok "recovery: poke fails then fills, lists" \
	'poke_n=$(( ${poke_n:-0} + 1 )); if [ "$poke_n" -ge 2 ]; then PROXY_POKE_HTTP_CODE=200; return 0; else PROXY_POKE_HTTP_CODE=404; return 1; fi' \
	'poke_n2=$(( ${poke_n2:-0} + 1 )); [ "$poke_n2" -ge 2 ]'

if [ "$fails" -eq 0 ]; then
	echo "PASS: proxy-wait smoke"
else
	echo "FAIL: $fails proxy-wait case(s)"
	exit 1
fi
