#!/usr/bin/env bash
# check-hook-fp-triage.sh — weekly hook false-positive triage ritual
# (T6/H5, plan 2026-10-30 §H).
#
# tq-bash-guard denies append `ts|rule|cmd` rows to $TQ_HOOK_DENY_LOG
# (.crushrc points it at ~/.cache/tq-hook-deny.log); advisories append the
# SAME row shape to $TQ_HOOK_ADVISORY_LOG (~/.cache/tq-hook-advisory.log).
# This check renders per-rule deny AND advisory counts + the most recent
# representative command per rule so a weekly pass can spot shapes that
# deny or advise so often they look like FPs (a rule the fleet keeps
# fighting is either miscalibrated or the corpus is missing its benign
# shape — feed the shape to
# scripts/hooks/benign-corpus-bash.txt or retire the rule).
#
# Exit code: always 0 unless the log is MALFORMED (rows not in
# ts|rule|cmd shape) — volume itself is information, not a failure.
#
#   scripts/check-hook-fp-triage.sh            # triage render (log path
#                                               # from env or default)
#   TQ_HOOK_FP_LOG=<file> scripts/check-hook-fp-triage.sh
#   scripts/check-hook-fp-triage.sh --self-test
#   TQ_HOOK_FP_DAYS=7 ...                      # only count rows newer
#                                               # than N days (0 = all)
set -euo pipefail
cd "$(dirname "$0")/.."

log="${TQ_HOOK_FP_LOG:-${TQ_HOOK_DENY_LOG:-$HOME/.cache/tq-hook-deny.log}}"
advlog="${TQ_HOOK_ADVISORY_LOG:-$HOME/.cache/tq-hook-advisory.log}"
days="${TQ_HOOK_FP_DAYS:-0}"

if [ "${1:-}" = "--self-test" ]; then
	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' EXIT

	# Fixture: well-formed rows across two rules + one foreign junk line
	# is NOT tolerated (malformed = rc 1 naming the row).
	# pins: summary+breakdown-branch — well-formed rows render the count
	# summary and per-rule breakdown; the junk-row negative path is below.
	cat >"$tmp/deny.log" <<'LOG'
2026-10-30T15:04:05Z|force-push|git push -f origin main
2026-10-30T15:05:06Z|force-push|git push --force origin x
2026-10-30T16:07:08Z|ere-bracket-class|grep -E '[^ \t]' f
LOG
	out="$(TQ_HOOK_FP_LOG="$tmp/deny.log" "$0" 2>&1)"
	printf '%s' "$out" | grep -q 'hook-deny triage: 3 deny(2 rule) since forever' ||
		{
			echo "SELF-TEST FAIL: summary line not pinned"
			printf '%s\n' "$out"
			exit 1
		}
	printf '%s' "$out" | grep -q 'force-push' ||
		{
			echo "SELF-TEST FAIL: rule breakdown missing"
			printf '%s\n' "$out"
			exit 1
		}

	printf 'junk-not-a-row\n' >"$tmp/bad.log"
	rc=0
	TQ_HOOK_FP_LOG="$tmp/bad.log" "$0" >/dev/null 2>&1 || rc=$?
	[ "$rc" = 1 ] || {
		echo "SELF-TEST FAIL: malformed row must rc=1"
		exit 1
	}

	# pins: advisory-mirror-branch — a TQ_HOOK_ADVISORY_LOG in the same
	# shape renders its own count summary + rule breakdown; a malformed
	# advisory row is a failure too (row 23, 02-56 §e2).
	# pins: advisory-mirror-branch — 3-row advisory log renders count + breakdown
	cat >"$tmp/adv.log" <<'LOG'
2026-10-30T15:04:05Z|tq-db-inherit|tq tasks
2026-10-31T09:00:00Z|tq-db-inherit|tq show
2026-10-31T10:00:00Z|multi-tail|tail -n 5 a.log b.log
LOG
	out="$(TQ_HOOK_ADVISORY_LOG="$tmp/adv.log" "$0" 2>&1)"
	printf '%s' "$out" | grep -q 'hook-advisory triage: 3 advise(2 rule)' ||
		{
			echo "SELF-TEST FAIL: advisory summary line not pinned"
			printf '%s\n' "$out"
			exit 1
		}
	printf '%s' "$out" | grep -q 'tq-db-inherit' ||
		{
			echo "SELF-TEST FAIL: advisory rule breakdown missing"
			printf '%s\n' "$out"
			exit 1
		}
	printf 'junk\n' >"$tmp/badadv.log"
	rc=0
	TQ_HOOK_ADVISORY_LOG="$tmp/badadv.log" "$0" >/dev/null 2>&1 || rc=$?
	[ "$rc" = 1 ] || {
		echo "SELF-TEST FAIL: malformed advisory row must rc=1"
		exit 1
	}
	echo "self-test ok: hook-fp-triage summary + rule breakdown + malformed-row negative path pinned (deny + advisory mirror)"
	exit 0
fi

window="since forever"
[ "$days" -gt 0 ] && window="in the last ${days}d"

# Advisory mirror (TQ_HOOK_ADVISORY_LOG): same shape validation, rendered
# before the deny section so it survives the deny-log-missing early exit —
# advisory FP pressure is as actionable as deny pressure but was previously
# untraceable.
if [ -f "$advlog" ]; then
	arows="$(cat "$advlog")"
	if [ -n "${cutoff:-}" ]; then
		arows="$(printf '%s\n' "$arows" | awk -F'|' -v c="$cutoff" '$1 >= c || $0 !~ /[|]/ {print}')"
	fi
	abad=0
	while IFS= read -r row; do
		[ -n "$row" ] || continue
		if ! printf '%s' "$row" | grep -qE '^[0-9]{4}-[0-9]{2}-[0-9]{2}T[^|]+\|[a-z0-9-]+\|.'; then
			echo "FAIL: malformed advisory-log row (want ts|rule|cmd): $row" >&2
			abad=1
		fi
	done <<<"$arows"
	[ "$abad" = 0 ] || exit 1
	atotal="$(printf '%s\n' "$arows" | grep -c '|' || true)"
	arules="$(printf '%s\n' "$arows" | awk -F'|' 'NF >= 2 && $2 != "" {print $2}' | sort -u | wc -l)"
	echo "hook-advisory triage: ${atotal} advise(${arules} rule) $window"
	printf '%s\n' "$arows" | awk -F'|' 'NF >= 3 {count[$2]++; last[$2]=$0} END {for (r in count) printf "  %-20s %d×  last: %s\n", r, count[r], last[r]}' | sort
fi

# Deny section (TQ_HOOK_DENY_LOG): the original triage surface.
if [ ! -f "$log" ]; then
	echo "hook-deny triage: no deny log yet ($log) — nothing to triage, hooks not denying or TQ_HOOK_DENY_LOG unset"
	exit 0
fi

# Filter by age when TQ_HOOK_FP_DAYS > 0 (ts is ISO-8601 UTC from date -u).
if [ "$days" -gt 0 ]; then
	cutoff="$(date -u -d "-${days} days" +%FT%TZ 2>/dev/null || date -u -v-"${days}"d +%FT%TZ 2>/dev/null || true)"
fi
rows="$(cat "$log")"
if [ -n "${cutoff:-}" ]; then
	rows="$(printf '%s\n' "$rows" | awk -F'|' -v c="$cutoff" '$1 >= c || $0 !~ /[|]/ {print}')"
fi

bad=0
while IFS= read -r row; do
	[ -n "$row" ] || continue
	if ! printf '%s' "$row" | grep -qE '^[0-9]{4}-[0-9]{2}-[0-9]{2}T[^|]+\|[a-z0-9-]+\|.'; then
		echo "FAIL: malformed deny-log row (want ts|rule|cmd): $row" >&2
		bad=1
	fi
done <<<"$rows"
[ "$bad" = 0 ] || exit 1

total="$(printf '%s\n' "$rows" | grep -c '|' || true)"
rules="$(printf '%s\n' "$rows" | awk -F'|' 'NF >= 2 && $2 != "" {print $2}' | sort -u | wc -l)"
echo "hook-deny triage: ${total} deny($rules rule) $window"
printf '%s\n' "$rows" | awk -F'|' 'NF >= 3 {count[$2]++; last[$2]=$0} END {for (r in count) printf "  %-20s %d×  last: %s\n", r, count[r], last[r]}' | sort
echo "review: a rule denying constantly is either miscalibrated or missing its benign shape in scripts/hooks/benign-corpus-bash.txt"
