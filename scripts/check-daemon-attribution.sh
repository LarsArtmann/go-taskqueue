#!/usr/bin/env bash
# Daemon-commit attribution gate (task 000001a109757cf3d260a0de4e2700000000;
# 00-55 report §f2): freezes the full-history findings of
# scripts/audit-daemon-attribution.sh into a baseline and fails only on NEW
# unattributed-shipping sweeps — the check-dead-sha-refs pattern (grandfather
# the legacy debt, gate the growth).
#
# The audit classifies every footer-less `chore: auto-commit …` sweep:
#   attributed / report-only  — never failures, not recorded in the baseline
#   unattributed-shipping     — failure UNLESS the commit sha is baselined
#
# Baseline: scripts/daemon-sweep-baseline.txt (one full sha per line, `#`
# comments). Shrink is advisory-only (a swept sha that later gains a marker
# or footer just stops being counted); a NEW unattributed-shipping sha not in
# the file fails the gate. Baselined hits are silent unless
# TQ_DAEMON_SWEEP_VERBOSE=1. A missing baseline fails closed.
#
# --self-test pins the decision branches against a /tmp fixture repo: baselined
# sweep rc=0 (silent unless verbose), shrink advisory (marker-attributed sweep
# stops counting), new shipping sweep rc=1 naming the sha, missing baseline
# fails closed.
set -uo pipefail

SELFDIR=$(cd "$(dirname "$0")" && pwd)
AUDIT="$SELFDIR/audit-daemon-attribution.sh"

die() {
	echo "check-daemon-attribution: $*" >&2
	exit 1
}

[ -f "$AUDIT" ] || die "audit script missing: $AUDIT"

# run_gate: cwd must be the git repo under audit; caller sets `baseline`.
# Returns 1 on any unattributed-shipping sweep sha not in the baseline.
run_gate() {
	local rows err summary row sha subject
	local fail=0 new_hits=0 baselined_hits=0
	if [ ! -f "$baseline" ]; then
		echo "daemon-sweep attribution: baseline missing: $baseline — restore it or regenerate from scripts/audit-daemon-attribution.sh --json"
		return 1
	fi
	err=$(mktemp) || die "mktemp failed"
	rows=$("$AUDIT" --json 2>"$err")
	summary=$(grep '^summary: scanned=' "$err" || true)
	rm -f "$err"
	if [ -z "$summary" ]; then
		echo "daemon-sweep attribution: audit produced no summary line — audit rot, failing closed"
		return 1
	fi
	while IFS= read -r row; do
		[ -n "$row" ] || continue
		case $row in
		*'"class":"unattributed-shipping"'*)
			sha=${row#\"commit\":\"}
			sha=${sha%%\"*}
			if [[ ! $sha =~ ^[0-9a-f]{40}$ ]]; then
				echo "daemon-sweep attribution: malformed audit row (sha parse rot): $row"
				fail=1
				continue
			fi
			if grep -qxF "$sha" "$baseline"; then
				baselined_hits=$((baselined_hits + 1))
				if [ "${TQ_DAEMON_SWEEP_VERBOSE:-0}" = 1 ]; then
					echo "BASELINED sweep: ${sha:0:7} (grandfathered; shrink is advisory-only)"
				fi
			else
				subject=${row#*\"subject\":\"}
				subject=${subject%%\",\"class\":*}
				fail=1
				new_hits=$((new_hits + 1))
				echo "NEW UNATTRIBUTED SWEEP: $sha \"$subject\""
				echo "  heal: scripts/heal-daemon-sweep.sh when unpushed; a footered empty marker commit citing the sha when pushed"
				echo "  deliberate grandfathering: append the full sha to $baseline"
			fi
			;;
		esac
	done <<<"$rows"
	if [ "$fail" = 0 ]; then
		echo "daemon-sweep attribution ok ($baselined_hits baselined, 0 new)"
		return 0
	fi
	echo "daemon-sweep attribution FAIL: $new_hits new unattributed-shipping sweep(s) not in $baseline ($baselined_hits baselined; audit $summary)"
	return 1
}

if [ "${1:-}" = "--self-test" ]; then
	tmp=$(mktemp -d) || die "mktemp failed"
	trap 'rm -rf "$tmp"' EXIT

	repo="$tmp/repo"
	mkdir -p "$repo/docs/status" || die "mkdir failed"
	git -C "$repo" init -q
	git -C "$repo" config user.email gate-selftest@example.invalid
	git -C "$repo" config user.name "gate self-test"
	git -C "$repo" config commit.gpgsign false

	daemon_subj="chore: auto-commit 1 changed file(s) (heuristic)"
	fixture_commit() {
		git -C "$repo" add -A
		git -C "$repo" commit -q "$@" || die "fixture commit failed"
	}

	printf 'base\n' >"$repo/README.md"
	fixture_commit -m init

	printf 'package old\n' >"$repo/old.go"
	fixture_commit -m "$daemon_subj"
	old_sweep=$(git -C "$repo" rev-parse HEAD)

	printf 'closeout\n' >"$repo/docs/status/2026-01-01_00-00_task-aaaabbbb111122223333444455556666.md"
	fixture_commit -m "$daemon_subj"

	printf '# baseline fixture\n%s\n' "$old_sweep" >"$tmp/base.txt"
	baseline="$tmp/base.txt"

	local pass=0
	local fail=0
	check_eq() {
		local label=$1 want=$2 got=$3
		if [ "$want" = "$got" ]; then
			pass=$((pass + 1))
		else
			fail=$((fail + 1))
			echo "FAIL: $label: want [$want] got [$got]"
		fi
	}

	# 1. Baselined shipping sweep: rc=0, counted, silent without the flag.
	out=$(cd "$repo" && run_gate)
	rc=$?
	check_eq "baselined run exit" "0" "$rc"
	check_eq "baselined run message" "daemon-sweep attribution ok (1 baselined, 0 new)" "$out"
	out=$(cd "$repo" && run_gate)
	if grep -q 'BASELINED sweep' <<<"$out"; then
		fail=$((fail + 1))
		echo "FAIL: baselined hit printed without TQ_DAEMON_SWEEP_VERBOSE"
	else
		pass=$((pass + 1))
	fi
	out=$(cd "$repo" && TQ_DAEMON_SWEEP_VERBOSE=1 run_gate)
	check_eq "verbose flag surfaces the baselined sha" "1" \
		"$(grep -c "BASELINED sweep: ${old_sweep:0:7}" <<<"$out")"

	# 2. Shrink advisory: a footered marker citing the sweep attributes it;
	# the now-stale baseline row just stops counting (rc stays 0).
	git -C "$repo" commit -q --allow-empty -m "$(printf '%s\n\n%s' \
		"The work files landed in the footer-less sweep ${old_sweep}; this marker carries the queue attribution without rewriting shared history." \
		"Task-Queue-ID: bbbbcccc222233334444555566667777")"
	out=$(cd "$repo" && run_gate)
	rc=$?
	check_eq "shrink run exit" "0" "$rc"
	check_eq "stale baseline row no longer counted" \
		"daemon-sweep attribution ok (0 baselined, 0 new)" "$out"

	# 3. NEW unattributed-shipping sweep: rc=1 naming exactly that sha.
	printf 'package new\n' >"$repo/new.go"
	fixture_commit -m "$daemon_subj"
	new_sweep=$(git -C "$repo" rev-parse HEAD)
	out=$(cd "$repo" && run_gate)
	rc=$?
	check_eq "new sweep run exit" "1" "$rc"
	check_eq "new sweep named with sha" "1" \
		"$(grep -c "NEW UNATTRIBUTED SWEEP: $new_sweep" <<<"$out")"
	check_eq "attributed old sweep not flagged" "0" \
		"$(grep -c "$old_sweep" <<<"$out")"

	# 4. Missing baseline fails closed.
	mv "$tmp/base.txt" "$tmp/base.bak"
	out=$(cd "$repo" && run_gate)
	rc=$?
	check_eq "missing baseline exit" "1" "$rc"
	check_eq "missing baseline message" "1" \
		"$(grep -c 'baseline missing' <<<"$out")"

	echo "daemon-attribution self-test: $pass passed, $fail failed"
	[ "$fail" -eq 0 ]
	exit $?
fi

cd "$SELFDIR/.." || exit 1

baseline="scripts/daemon-sweep-baseline.txt"

run_gate
