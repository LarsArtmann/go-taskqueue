#!/usr/bin/env bash
# Orphaned-guard audit (TODO_LIST 2026-09-11 evening row; 15-39 report c7/f5, e1):
# every scripts/check-*.sh and scripts/smoke/*.sh must be referenced by
# ci-local.sh, ci.yml, or flake.nix. A guard nothing runs is the
# check-webui-css failure mode (unminified/hand-edited css shipped to
# master twice in 2026-09-11 behind an orphaned guard).
set -euo pipefail
cd "$(dirname "$0")/.."
# Recursive --self-test re-invokes THIS script; capture the absolute self
# path BEFORE any cd (the gosec self_abs pattern; 02-38 §b).
self_abs="$(cd "$(dirname "$0")" && pwd)/$(basename "$0")"

must_mention() {
	if grep -qF -- "$1" <<<"$2"; then
		echo "ok: $3"
	else
		echo "FAIL: $3 (output is missing '$1')"
		printf '%s\n' "$2"
		exit 1
	fi
}

# --self-test pins the end-of-run summary line (fleet convention: countable
# gate summaries are PINNED, not advisory — 01-12 §e7/§g2) and runs a
# negative-path counter battery over the orphan branch via the
# TQ_GUARD_FILES hook (scan-list override).
self_test() {
	# tmp is deliberately global: the EXIT trap reads it after self_test
	# returned, and set -u would kill the trap on a function-local name.
	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' EXIT
	local orphan="$tmp/fake-orphan.sh"
	: >"$orphan"
	local out rc=0
	out="$(TQ_GUARD_FILES="$orphan" "$self_abs" 2>&1)" || rc=$?
	if [ "$rc" -ne 1 ]; then
		echo "FAIL: negative battery (want rc=1, got rc=$rc)"
		printf '%s\n' "$out"
		exit 1
	fi
	must_mention "ORPHANED GUARD: $orphan" "$out" "orphan branch fires on the unwired fixture"
	must_mention "guard-wiring summary: 1 scripts wired, 1 orphaned" "$out" "summary pins exact counters (ritual wired + fixture orphaned)"
	# Sibling-sourcing rule (2026-11-17): a smoke library a sibling sources
	# counts as wired; a lib nothing sources stays an orphan. Fixtures are a
	# fake smoke dir (TQ_GUARD_SMOKE_DIR) + TQ_GUARD_FILES pair — hermetic,
	# the real scripts/smoke tree stays untouched.
	mkdir -p "$tmp/smoke"
	: >"$tmp/smoke/fakelib.sh"
	printf '. "$(dirname "$0")/fakelib.sh"\n' >"$tmp/smoke/consumer.sh"
	: >"$tmp/smoke/unsourced.sh"
	local src_out src_rc=0
	src_out="$(TQ_GUARD_FILES="$tmp/smoke/fakelib.sh $tmp/smoke/unsourced.sh" TQ_GUARD_SMOKE_DIR="$tmp/smoke" "$self_abs" 2>&1)" || src_rc=$?
	if [ "$src_rc" -ne 1 ]; then
		echo "FAIL: sibling-sourcing battery (want rc=1, got rc=$src_rc)"
		printf '%s\n' "$src_out"
		exit 1
	fi
	if grep -qF "ORPHANED GUARD: $tmp/smoke/fakelib.sh" "$src_out"; then
		echo "FAIL: sourced lib must count as wired"
		printf '%s\n' "$src_out"
		exit 1
	fi
	must_mention "ORPHANED GUARD: $tmp/smoke/unsourced.sh" "$src_out" "unsourced lib stays an orphan"
	must_mention "guard-wiring summary: 1 scripts wired, 1 orphaned" "$src_out" "sibling-sourcing summary pins exact counters (sourced lib wired)"
	local pos_out pos_wired pos_orphaned
	pos_out="$("$self_abs" 2>&1)" || true
	pos_wired=$(printf '%s\n' scripts/check-*.sh scripts/smoke/*.sh | wc -l)
	pos_orphaned=$(grep -c '^ORPHANED GUARD:' <<<"$pos_out" || true)
	pos_wired=$((pos_wired - pos_orphaned))
	# rc is NOT pinned: a pre-existing orphan elsewhere in the fleet keeps
	# the real run red (counter still exact) — the pin targets the summary,
	# not the fleet's orphan hygiene.
	must_mention "guard-wiring summary: $pos_wired scripts wired, $pos_orphaned orphaned" "$pos_out" "summary pins exact positive counters"
	echo "guard-wiring self-test ok (summary format pin + negative counter battery)"
}

if [ "${1:-}" = "--self-test" ]; then
	self_test
	exit 0
fi

# release.sh counts as a wiring surface: check-consumer-tag.sh is the
# release-flow gate (release.sh --push runs it at the tag; needs network +
# the module proxy — never a per-push ci-local step).
refs=(scripts/ci-local.sh .github/workflows/ci.yml flake.nix scripts/release.sh)
rc=0
wired_ok=0
wired_failed=0
if [ -n "${TQ_GUARD_FILES:-}" ]; then
	# word splitting intended: the hook carries a whitespace-separated list
	# shellcheck disable=SC2086
	set -- $TQ_GUARD_FILES
else
	set -- scripts/check-*.sh scripts/smoke/*.sh
fi
for f in "$@"; do
	wired=0
	for r in "${refs[@]}"; do
		if grep -qF "$f" "$r"; then
			wired=1
			break
		fi
	done
	if [ "$wired" = 0 ]; then
		case "$f" in
		"${TQ_GUARD_SMOKE_DIR:-scripts/smoke}"/*)
			# Shared smoke libraries (_lib.sh, the scratch-repo factory)
			# are wired by their CONSUMERS: a sibling smoke script sourcing
			# the basename counts as a wiring surface — a lib nothing
			# sources is dead code the orphan branch should still catch,
			# but a sourced lib runs on every consumer run (2026-11-17:
			# _lib.sh orphaned by the literal-path grep while every
			# hermetic smoke sourced it).
			base=${f##*/}
			for g in "${TQ_GUARD_SMOKE_DIR:-scripts/smoke}"/*.sh; do
				[ "$g" = "$f" ] && continue
				if grep -qE "^[[:space:]]*(source|[.])[[:space:]].*${base}" "$g" 2>/dev/null; then
					wired=1
					break
				fi
			done
			;;
		esac
	fi
	if [ "$wired" = 0 ]; then
		echo "ORPHANED GUARD: $f is not referenced by ci-local.sh, ci.yml, or flake.nix — wire it or delete it"
		rc=1
		wired_failed=$((wired_failed + 1))
	else
		wired_ok=$((wired_ok + 1))
	fi
done

# session-start.sh is convention-invoked (agent turn-1 ritual, AGENTS.md),
# never referenced by ci-local/ci.yml/flake — the loop above is blind to it.
# Assert existence so deletion/renaming fails the gate (08-57 report §f3).
ritual=scripts/session-start.sh
if [ ! -f "$ritual" ]; then
	echo "ORPHANED GUARD: $ritual missing — the AGENTS.md turn-1 ritual script is convention-invoked, not gate-referenced; deletion would be invisible"
	rc=1
	wired_failed=$((wired_failed + 1))
else
	wired_ok=$((wired_ok + 1))
fi

if [ "$rc" = 0 ]; then
	echo "guard wiring ok: every check-*/smoke script is referenced"
fi
echo "guard-wiring summary: $wired_ok scripts wired, $wired_failed orphaned"
exit "$rc"
