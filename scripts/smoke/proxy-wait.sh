#!/usr/bin/env bash
# Proxy-wait smoke: offline behavior test for wait_for_proxy_version
# (scripts/lib/proxy-wait.sh, extracted from release.sh). bash -n covers
# syntax only; this drives BOTH die branches — network-dead (every .info
# poke fails) vs proxy lag (pokes succeed, @v/list never lists) — plus the
# recovery path, with the probe seams faked and zero network access.
set -euo pipefail
cd "$(dirname "$0")/../.."

fails=0

# Structural offline claim: re-exec the whole smoke inside an empty network
# namespace (unshare -n) where available, so "zero network access" holds
# even if a fake regresses into real curl/go-list. Skip with a loud line
# where unshare is unavailable (fakes still keep the smoke offline).
# netns_is_empty: tri-state detector for "inside an empty network
# namespace" (i.e. the dev file lists ONLY the loopback interface):
#   rc 0 — loopback-only (empty netns)
#   rc 1 — non-loopback interfaces present (a real net)
#   rc 2 — dev file UNREADABLE — a technical failure (hermetic sandboxes
#          can mask /proc), NOT evidence of a live net; the gate reports
#          it as such instead of claiming a non-empty namespace.
# TQ_NETNS_DEV_FILE overrides the dev path so the self-test below drives
# the REAL detector logic against fixture files (an override stub would
# leave the detector's own branches unguarded).
netns_is_empty() {
	local dev_file="${TQ_NETNS_DEV_FILE:-/proc/self/net/dev}"
	[ -r "$dev_file" ] || return 2
	local devs
	devs="$(awk 'NR > 2 { sub(/:.*/, ""); gsub(/^[ \t]+|[ \t]+$/, ""); if ($0 != "") print }' "$dev_file")"
	[ "$devs" = "lo" ]
}

netns_refusal_msg='FAIL: TQ_SMOKE_NETNS=1 set but /proc/self/net/dev is not loopback-only — structural offline claim does NOT hold; refusing to run'
netns_unreadable_msg='FAIL: TQ_SMOKE_NETNS=1 set but /proc/self/net/dev is UNREADABLE (hermetic sandbox?) — technical failure, NOT a non-empty namespace; cannot verify the structural offline claim; refusing to run'
netns_verified_msg='== netns marker verified: loopback-only /proc/self/net/dev =='

# netns_marker_gate is the whole marker block, factored so the refusal
# self-test below drives it verbatim: the re-exec'd inner run and the
# nested self-test children all execute THIS function, never a copy.
netns_marker_gate() {
	if [ "${TQ_SMOKE_NETNS:-}" != "1" ]; then
		if command -v unshare >/dev/null 2>&1 && unshare -n true 2>/dev/null; then
			echo "== re-exec under unshare -n (structural offline) =="
			exec env TQ_SMOKE_NETNS=1 unshare -n "$0" "$@"
		fi
		echo "SKIP: unshare -n unavailable (or no permission) — offline claim is NOT structural; proceeding with faked seams only"
	else
		local empty_rc=0
		netns_is_empty || empty_rc=$?
		case "$empty_rc" in
		0) echo "$netns_verified_msg" ;;
		2)
			echo "$netns_unreadable_msg" >&2
			exit 1
			;;
		*)
			echo "$netns_refusal_msg" >&2
			exit 1
			;;
		esac
	fi
}

expect_gate() { # <name> <want-rc> <expected-msg> <child-script>
	local out rc
	out="$(TQ_SMOKE_NETNS=1 bash -c "$4" netns-self-test 2>&1)" && rc=0 || rc=$?
	if [ "$rc" = "$2" ] && grep -qF "$3" <<<"$out"; then
		echo "ok   $1"
	else
		echo "FAIL $1 (rc=$rc, wanted exit $2 with: $3)"
		echo "$out"
		fails=$((fails + 1))
	fi
}

# host_netns_raw_empty is the self-test's independent oracle: a deliberate
# copy of the /proc/self/net/dev read that — unlike the detector — does NOT
# honor TQ_NETNS_DEV_FILE, so it always reports the host's real netns and
# stays usable as the truth the detector is checked against. Same tri-state
# rc contract as netns_is_empty (0 empty / 1 non-empty / 2 unreadable). If
# it and netns_is_empty ever disagree, the real-detector leg below fails —
# that disagreement IS the regression the self-test exists to catch (the
# guard must not trust the function under test to decide whether the
# function is testable).
host_netns_raw_empty() {
	[ -r /proc/self/net/dev ] || return 2
	local devs
	devs="$(awk 'NR > 2 { sub(/:.*/, ""); gsub(/^[ \t]+|[ \t]+$/, ""); if ($0 != "") print }' /proc/self/net/dev)"
	[ "$devs" = "lo" ]
}

if [ "${TQ_SMOKE_NETNS:-}" != "1" ]; then
	# Refusal self-test (host entry only: inside the netns the gate
	# legitimately passes). The wiring leg drives the REAL gate function
	# via export -f with a stubbed detector; the fixture legs drive the
	# REAL detector via TQ_NETNS_DEV_FILE against fixture dev files — so a
	# refactor of netns_is_empty or the gate cannot silently regress any
	# branch (the SKIP path otherwise hides the whole marker block):
	#   - wiring: detector stubbed non-empty — the gate must refuse.
	#   - fixture unreadable: real detector hits the rc-2 branch — the
	#     gate must print the UNREADABLE diagnostic, never the refusal.
	#   - fixture loopback-only: real detector hits the rc-0 branch — the
	#     gate must pass (positive-path pin for the tri-state detector).
	#   - real detector vs host netns: must refuse — skipped loudly where
	#     the RAW oracle says the host is already loopback-only or its
	#     dev file is unreadable (the truth is indistinguishable there).
	fixture_dir="$(mktemp -d)"
	trap 'rm -rf "$fixture_dir"' EXIT
	printf 'Inter-|   Receive                                                 |  Transmit\n face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed\n    lo: 1000    10   0    0    0     0          0         0      500       5    0    0    0     0       0          0\n' >"$fixture_dir/lo-only-dev"
	export -f netns_is_empty netns_marker_gate
	export netns_refusal_msg netns_unreadable_msg netns_verified_msg
	expect_gate "netns refusal self-test: gate wiring (non-empty)" 1 "$netns_refusal_msg" \
		'netns_is_empty() { return 1; }; netns_marker_gate'
	expect_gate "netns unreadable self-test: real detector, unreadable dev file" 1 "$netns_unreadable_msg" \
		"TQ_NETNS_DEV_FILE='$fixture_dir/absent/dev' netns_marker_gate"
	expect_gate "netns self-test: real detector, loopback-only dev file passes" 0 "$netns_verified_msg" \
		"TQ_NETNS_DEV_FILE='$fixture_dir/lo-only-dev' netns_marker_gate"
	host_rc=0
	host_netns_raw_empty || host_rc=$?
	if [ "$host_rc" = 0 ]; then
		echo "SKIP netns refusal self-test: host netns already loopback-only — real-detector refusal not exercisable here"
	elif [ "$host_rc" = 2 ]; then
		echo "SKIP netns refusal self-test: host /proc/self/net/dev unreadable — real-detector refusal not exercisable here"
	else
		expect_gate "netns refusal self-test: real detector vs host netns" 1 "$netns_refusal_msg" \
			'netns_marker_gate'
	fi
fi

netns_marker_gate

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
# Success-path prose comes from the lib too (same constant the loop prints).
serves_line="$(printf "$TQ_PROXY_SERVES_FMT" "$SMOKE_VERSION")"

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
	if [ "$rc" = 0 ] && ! grep -q '^DIE:' <<<"$out" && grep -qF "$serves_line" <<<"$out"; then
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
