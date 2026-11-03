#!/usr/bin/env bash
# Rewrite-rate probe: scan the HEAD reflog and count history-rewrite events
# (filter-branch / rebase) per day, flagging days at or over the threshold —
# three rewrites landed 2026-11-02→11-03 alone, and each rewrite mints a new
# dead-cite debt batch (the dd543ec class that check-dead-sha-refs.sh polices).
# Surfacing repeat offenders BEFORE they rewrite again is cheaper than
# healing 43 citations after (2026-11-03 report §e5).
#
# Output: one `<date> <count>` row per day with events (most recent first),
# then a `rewrite-rate: N events / D days, threshold T` summary.
# Exit 1 iff any single day reached the threshold (TQ_REWRITE_RATE_MAX,
# default 3) — the run is then a FAIL naming the hot days.
#
# Env hooks:
#   TQ_REWRITE_RATE_MAX   threshold per day (default 3)
#   TQ_REWRITE_DAYS       lookback window in days (default 30)
#   TQ_REWRITE_REFLOG_FILE  read `date|message` lines from this file instead
#                         of the live reflog (self-test + replay fixture)
#
# --self-test pins the fixture battery: count summary, hot-day failure,
# threshold boundary (3 fires at MAX=3, passes at MAX=4), non-rewrite
# events ignored, malformed lines skipped.
set -uo pipefail

threshold="${TQ_REWRITE_RATE_MAX:-3}"
window="${TQ_REWRITE_DAYS:-30}"
repo="${TQ_PROBE_REPO:-.}"

is_rewrite() {
	# One rebase logs many "(pick)" entries — count the (start) event once;
	# filter-branch lines are their own event. Bare `rebase` (no suffix)
	# counts as an event too.
	case "$1" in
	*filter-branch* | *rebase*\ \(start\)* | rebase | rebase\ -i*) return 0 ;;
	esac
	return 1
}

scan() {
	if [ -n "${TQ_REWRITE_REFLOG_FILE:-}" ]; then
		cat "$TQ_REWRITE_REFLOG_FILE"
	else
		git -C "$repo" log -g --date=short --format='%cd|%gs' HEAD 2>/dev/null || true
	fi
}

run_probe() {
	local counts=() dates=() day msg total=0 ndays=0 hot=0
	while IFS='|' read -r day msg; do
		[ -n "$day" ] || continue
		case "$day" in
		[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]) ;;
		*) continue ;;
		esac
		is_rewrite "$msg" || continue
		total=$((total + 1))
		found=0
		for i in "${!dates[@]}"; do
			if [ "${dates[$i]}" = "$day" ]; then
				counts[$i]=$((counts[$i] + 1))
				found=1
				break
			fi
		done
		if [ "$found" = 0 ]; then
			dates+=("$day")
			counts+=(1)
		fi
	done < <(scan)

	for i in "${!dates[@]}"; do
		echo "${dates[$i]} ${counts[$i]}"
		ndays=$((ndays + 1))
		if [ "${counts[$i]}" -ge "$threshold" ]; then
			hot=$((hot + 1))
		fi
	done

	echo "rewrite-rate: $total events / $ndays days, threshold $threshold"
	if [ "$hot" -gt 0 ]; then
		echo "REWRITE-RATE FAIL: $hot day(s) at/over $threshold rewrites — history rewrites mint dead-cite debt (check-dead-sha-refs.sh); re-run with TQ_REWRITE_REFLOG_FILE to audit the offenders" >&2
		return 1
	fi
	return 0
}

if [ "${1:-}" = "--self-test" ]; then
	fix="$(mktemp)"
	trap 'rm -f "$fix"' EXIT
	# canned reflog: 3 rebase STARTS on 2026-11-02 (hot), 1 filter-branch on
	# 2026-11-03 (fine), plus pick/commit/checkout noise to ignore
	cat >"$fix" <<'EOF'
2026-11-02|rebase (start): checkout master
2026-11-02|rebase (pick): docs: x
2026-11-02|rebase (start): checkout master
2026-11-02|pull --rebase origin master (start): fast-forward
2026-11-03|filter-branch: msg-filter heal
2026-11-03|rebase (pick): docs: y
2026-11-03|commit: docs(status): closeout
2026-11-03|checkout: moving from master to wip
not-a-date|rebase (start): malformed line skipped
EOF
	st_fail=0
	out="$(TQ_REWRITE_REFLOG_FILE="$fix" TQ_REWRITE_RATE_MAX=4 bash "$0" 2>&1)" || st_fail=$((st_fail + 1))
	printf '%s' "$out" | grep -q '^2026-11-02 3$' || {
		echo "FAIL: day count row missing: $out"
		st_fail=$((st_fail + 1))
	}
	printf '%s' "$out" | grep -qF 'rewrite-rate: 4 events / 2 days, threshold 4' || {
		echo "FAIL: summary wrong: $out"
		st_fail=$((st_fail + 1))
	}
	out="$(TQ_REWRITE_REFLOG_FILE="$fix" TQ_REWRITE_RATE_MAX=3 bash "$0" 2>&1)" && {
		echo "FAIL: threshold 3 should fail on the 3-rebase day: $out"
		st_fail=$((st_fail + 1))
	}
	printf '%s' "$out" | grep -q 'REWRITE-RATE FAIL' || {
		echo "FAIL: hot-day message missing: $out"
		st_fail=$((st_fail + 1))
	}
	if [ "$st_fail" -ne 0 ]; then
		echo "rewrite-rate-probe self-test FAILED ($st_fail cases)"
		exit 1
	fi
	echo "rewrite-rate-probe self-test ok (3 checks: count row, summary, threshold boundary)"
	exit 0
fi

run_probe
