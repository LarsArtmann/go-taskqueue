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
#   TQ_REWRITE_REFLOG_FILE  read `date|message` lines from this file instead
#                         of the live reflog (self-test + replay fixture)
#   TQ_REWRITE_RATE_LEDGER  ledger file (one `date|events|hot` row per day,
#                         tracked in git) — each run appends/replaces TODAY's
#                         row so day-over-day trends survive across nights
#                         (TODO 02-56 §g2); idempotent: a re-run the same
#                         day rewrites that day's row, never duplicates it
#   TQ_REWRITE_RATE_ADAPTIVE  when set AND the ledger holds ≥ 10 rows, the
#                         threshold is p95-calibrated from the trailing 30
#                         ledger rows instead of the constant (min 1) — the
#                         owner call on calibration vs constant-noise is
#                         pending, so this is OPT-IN and the default stays
#                         the constant MAX
#
# --self-test pins the fixture battery: count summary, hot-day failure,
# threshold boundary (3 fires at MAX=3, passes at MAX=4), non-rewrite
# events ignored, malformed lines skipped.
set -uo pipefail

threshold="${TQ_REWRITE_RATE_MAX:-3}"
repo="${TQ_PROBE_REPO:-.}"
ledger="${TQ_REWRITE_RATE_LEDGER:-}"
today="$(date -u +%F)"

# p95-calibrate the threshold from the trailing 30 ledger rows (adaptive
# mode only; needs a real sample to beat the constant).
if [ -n "${TQ_REWRITE_RATE_ADAPTIVE:-}" ] && [ -n "$ledger" ] && [ -f "$ledger" ]; then
	cal="$(tail -n 30 "$ledger" 2>/dev/null | awk -F'|' '$1 ~ /^[0-9]{4}-/ && $2 ~ /^[0-9]+$/ {print $2}' | sort -n | awk '
		{v[NR]=$1}
		END {
			if (NR < 10) exit 1
			i = int(NR * 0.95)
			if (i < 1) i = 1
			if (i < NR && NR * 0.95 > i) i++
			print v[i]
		}')" || cal=""
	if [ -n "$cal" ] && [ "$cal" -ge 1 ] 2>/dev/null; then
		threshold="$cal"
	fi
fi

is_rewrite() {
	# One rebase logs many "(pick)" entries — count the (start) event once;
	# filter-branch lines are their own event. Bare `rebase` (no suffix)
	# counts as an event too. `commit (amend)` counts (TODO 02-56 §f23):
	# an amend orphans every sha cited before it (2 dead cites this session
	# via pre-amend shas) — a rewrite event in every way that matters.
	case "$1" in
	*filter-branch* | *rebase*\ \(start\)* | rebase | rebase\ -i* | commit\ \(amend\)*) return 0 ;;
	esac
	return 1
}

scan() {
	if [ -n "${TQ_REWRITE_REFLOG_FILE:-}" ]; then
		cat "$TQ_REWRITE_REFLOG_FILE"
	else
		# %gn|%ge = the reflog identity — WHO performed the entry (TODO
		# 02-56 §f24: repeat offenders are actors, not just days)
		git -C "$repo" log -g --date=short --format='%cd|%gs|%gn|%ge' HEAD 2>/dev/null || true
	fi
}

run_probe() {
	local counts=() dates=() day msg an ae total=0 ndays=0 hot=0 today_count=0
	local actors=() acounts=() actor afound offenders=()
	while IFS='|' read -r day msg an ae; do
		[ -n "$day" ] || continue
		case "$day" in
		[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]) ;;
		*) continue ;;
		esac
		is_rewrite "$msg" || continue
		total=$((total + 1))
		actor="${an:-unknown} <${ae:-unknown}>"
		[ -n "$an" ] || actor="unknown"
		[ "$day" = "$today" ] && today_count=$((today_count + 1))
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
		# per-actor attribution (TODO 02-56 §f24): repeat offenders are
		# actors, not just days
		afound=0
		for i in "${!actors[@]}"; do
			if [ "${actors[$i]}" = "$actor" ]; then
				acounts[$i]=$((acounts[$i] + 1))
				afound=1
				break
			fi
		done
		[ "$afound" = 1 ] || {
			actors+=("$actor")
			acounts+=(1)
		}
	done < <(scan)

	for i in "${!dates[@]}"; do
		echo "${dates[$i]} ${counts[$i]}"
		ndays=$((ndays + 1))
		if [ "${counts[$i]}" -ge "$threshold" ]; then
			hot=$((hot + 1))
		fi
	done

	for i in "${!actors[@]}"; do
		echo "actor ${actors[$i]}: ${acounts[$i]}"
		[ "${acounts[$i]}" -ge "$threshold" ] && offenders+=("${actors[$i]} (${acounts[$i]})")
	done

	echo "rewrite-rate: $total events / $ndays days / ${#actors[@]} actors, threshold $threshold"
	if [ -n "$ledger" ]; then
		today_hot=0
		[ "$today_count" -ge "$threshold" ] && today_hot=1
		if [ -f "$ledger" ] && grep -q "^${today}|" "$ledger" 2>/dev/null; then
			grep -v "^${today}|" "$ledger" >"${ledger}.tmp" 2>/dev/null || : >"${ledger}.tmp"
			mv "${ledger}.tmp" "$ledger"
		fi
		printf '%s|%s|%s\n' "$today" "$today_count" "$today_hot" >>"$ledger"
	fi
	if [ "$hot" -gt 0 ]; then
		echo "REWRITE-RATE FAIL: $hot day(s) at/over $threshold rewrites — history rewrites mint dead-cite debt (check-dead-sha-refs.sh); re-run with TQ_REWRITE_REFLOG_FILE to audit the offenders" >&2
		if [ "${#offenders[@]}" -gt 0 ]; then
			echo "repeat offenders (by reflog identity): ${offenders[*]}" >&2
		fi
		return 1
	fi
	return 0
}

if [ "${1:-}" = "--self-test" ]; then
	fix="$(mktemp)"
	trap 'rm -f "$fix"' EXIT
	# canned reflog: 3 rebase STARTS + 1 amend on 2026-11-02 (hot), 1
	# filter-branch on 2026-11-03 (fine), plus pick/plain-commit/checkout
	# noise to ignore; 4-field lines carry the reflog actor (§f24), the
	# 2-field tail lines pin backward-compat (actor falls back to unknown)
	cat >"$fix" <<'EOF'
2026-11-02|rebase (start): checkout master|Lars|lars@x
2026-11-02|rebase (pick): docs: x|Lars|lars@x
2026-11-02|rebase (start): checkout master|Peer|peer@y
2026-11-02|pull --rebase origin master (start): fast-forward|Lars|lars@x
2026-11-02|commit (amend): docs: tick rows|Lars|lars@x
2026-11-03|filter-branch: msg-filter heal|Peer|peer@y
2026-11-03|rebase (pick): docs: y|Peer|peer@y
2026-11-03|commit: docs(status): closeout
2026-11-03|checkout: moving from master to wip
not-a-date|rebase (start): malformed line skipped
EOF
	st_fail=0
	out="$(TQ_REWRITE_REFLOG_FILE="$fix" TQ_REWRITE_RATE_MAX=5 bash "$0" 2>&1)" || st_fail=$((st_fail + 1))
	printf '%s' "$out" | grep -q '^2026-11-02 4$' || {
		echo "FAIL: day count row missing (amend must count as a rewrite event): $out"
		st_fail=$((st_fail + 1))
	}
	printf '%s' "$out" | grep -qF 'rewrite-rate: 5 events / 2 days / 2 actors, threshold 5' || {
		echo "FAIL: summary wrong: $out"
		st_fail=$((st_fail + 1))
	}
	printf '%s' "$out" | grep -qF 'actor Lars <lars@x>: 3' || {
		echo "FAIL: per-actor attribution row missing/wrong: $out"
		st_fail=$((st_fail + 1))
	}
	printf '%s' "$out" | grep -qF 'actor Peer <peer@y>: 2' || {
		echo "FAIL: per-actor attribution row missing/wrong: $out"
		st_fail=$((st_fail + 1))
	}
	out="$(TQ_REWRITE_REFLOG_FILE="$fix" TQ_REWRITE_RATE_MAX=3 bash "$0" 2>&1)" && {
		echo "FAIL: threshold 3 should fail on the 4-event (3 rebase + 1 amend) day: $out"
		st_fail=$((st_fail + 1))
	}
	printf '%s' "$out" | grep -q 'REWRITE-RATE FAIL' || {
		echo "FAIL: hot-day message missing: $out"
		st_fail=$((st_fail + 1))
	}
	printf '%s' "$out" | grep -qF 'repeat offenders (by reflog identity): Lars <lars@x> (3)' || {
		echo "FAIL: repeat-offender attribution missing: $out"
		st_fail=$((st_fail + 1))
	}
	if [ "$st_fail" -ne 0 ]; then
		echo "rewrite-rate-probe self-test FAILED ($st_fail cases)"
		exit 1
	fi

	# pins: ledger-branch (02-56 §g2) — a run with TQ_REWRITE_RATE_LEDGER
	# appends exactly one `date|events|hot` row for today; a re-run the same
	# day REPLACES it (idempotent, never duplicates).
	led2dir="$(mktemp -d)"
	trap 'rm -rf "$led2dir"' EXIT
	led="$led2dir/ledger.csv"
	fix2="$led2dir/reflog2"
	printf '%s\n' '2026-11-03|rebase (start): checkout master' '2026-11-03|rebase (start): checkout main' >"$fix2"
	TQ_REWRITE_REFLOG_FILE="$fix2" TQ_REWRITE_RATE_MAX=1 TQ_REWRITE_RATE_LEDGER="$led" bash "$0" >/dev/null 2>&1
	TQ_REWRITE_REFLOG_FILE="$fix2" TQ_REWRITE_RATE_MAX=1 TQ_REWRITE_RATE_LEDGER="$led" bash "$0" >/dev/null 2>&1
	[ "$(wc -l <"$led")" = 1 ] && grep -qE "^${today}\|2\|1$" "$led" || {
		echo "FAIL: ledger row not appended/deduped in date|events|hot shape: $(cat "$led")"
		st_fail=$((st_fail + 1))
	}
	# pins: adaptive-threshold-branch — with ≥ 10 trailing ledger rows the
	# p95 of the events column replaces the constant threshold. Fixture:
	# 20 rows of 1 event (p95 = 1); a 1-event day must then be hot.
	led2="$led2dir/ledger2.csv"
	for _ in $(seq 20); do printf '2026-10-01|1|0\n' >>"$led2"; done
	fix3="$led2dir/reflog3"
	printf '%s\n' '2026-11-03|filter-branch: x' >"$fix3"
	TQ_REWRITE_REFLOG_FILE="$fix3" TQ_REWRITE_RATE_LEDGER="$led2" TQ_REWRITE_RATE_ADAPTIVE=1 \
		bash "$0" >"$led2dir/adaptive.out" 2>&1 && {
		echo "FAIL: adaptive threshold p95=1 should fire on a 1-event day: $(cat "$led2dir/adaptive.out")"
		st_fail=$((st_fail + 1))
	}
	grep -qF 'threshold 1' "$led2dir/adaptive.out" || {
		echo "FAIL: adaptive run did not use the p95 threshold: $(cat "$led2dir/adaptive.out")"
		st_fail=$((st_fail + 1))
	}
	# same ledger WITHOUT the adaptive flag keeps the constant (3) → pass
	TQ_REWRITE_REFLOG_FILE="$fix3" TQ_REWRITE_RATE_LEDGER="$led2" bash "$0" >/dev/null 2>&1 || {
		echo "FAIL: non-adaptive run must keep the constant threshold"
		st_fail=$((st_fail + 1))
	}
	if [ "$st_fail" -ne 0 ]; then
		echo "rewrite-rate-probe self-test FAILED ($st_fail cases)"
		exit 1
	fi
	echo "rewrite-rate-probe self-test ok (9 checks: count row incl. amend events, summary with actor count, per-actor attribution rows, threshold boundary, repeat-offender attribution, ledger append+dedupe, adaptive p95 threshold, constant default, 2-field backward compat)"
	exit 0
fi

run_probe
