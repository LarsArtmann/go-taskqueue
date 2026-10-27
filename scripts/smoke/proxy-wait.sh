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
# netns_is_empty: true only when this process is inside an empty network
# namespace, i.e. /proc/self/net/dev lists ONLY the loopback interface.
# Guards the TQ_SMOKE_NETNS=1 re-exec marker: an operator-exported env var
# must never let the smoke run in a real (non-empty) net while still
# making the structural offline claim.
netns_is_empty() {
	[ -r /proc/self/net/dev ] || return 1
	local devs
	devs="$(awk 'NR > 2 { sub(/:.*/, ""); gsub(/^[ \t]+|[ \t]+$/, ""); if ($0 != "") print }' /proc/self/net/dev)"
	[ "$devs" = "lo" ]
}

if [ "${TQ_SMOKE_NETNS:-}" != "1" ]; then
	if command -v unshare >/dev/null 2>&1 && unshare -n true 2>/dev/null; then
		echo "== re-exec under unshare -n (structural offline) =="
		exec env TQ_SMOKE_NETNS=1 unshare -n "$0" "$@"
	fi
	echo "SKIP: unshare -n unavailable (or no permission) — offline claim is NOT structural; proceeding with faked seams only"
else
	if ! netns_is_empty; then
		echo "FAIL: TQ_SMOKE_NETNS=1 set but /proc/self/net/dev is not loopback-only — structural offline claim does NOT hold; refusing to run" >&2
		exit 1
	fi
	echo "== netns marker verified: loopback-only /proc/self/net/dev =="
fi

# shellcheck source=scripts/lib/proxy-wait.sh
source scripts/lib/proxy-wait.sh

# Expected die messages come from the lib's constants (single source of
# truth) — no substring greps. The driven module/version pair lives in one
# pair of exported vars shared by the die_lag derivation and drive()'s
# wait_for_proxy_version call, so the expected printf can never drift from
# the driven task.
export SMOKE_MODULE='example.com/module'
export SMOKE_VERSION='vX.Y.Z'
die_lag="$(printf "$TQ_PROXY_DIE_LAG" "$SMOKE_VERSION" "$SMOKE_MODULE" "$SMOKE_VERSION")"
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
		# Structural seam-ordering pin: the overrides must survive every lib
		# (re)source up to (and through) the driven call. If a reorder or a
		# stray re-source ever restores the real proxy seams, fail loudly
		# instead of silently hitting the network.
		assert_seams_faked() {
			local seam_body
			for seam_body in "$(declare -f tq_proxy_poke)" "$(declare -f tq_proxy_lists)"; do
				case "$seam_body" in
				*FAKE_*) ;;
				*) echo "SEAM ORDERING VIOLATION: fake override lost — a (re)source after the overrides would hit the real proxy" >&2; exit 43 ;;
				esac
			done
		}
		die() {
			assert_seams_faked
			echo "DIE: $*"
			exit 42
		}
		assert_seams_faked
		wait_for_proxy_version "$SMOKE_MODULE" "$SMOKE_VERSION" 3 0
		assert_seams_faked
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
