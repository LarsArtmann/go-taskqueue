#!/usr/bin/env bash
# Index-row phrase-drift guard (advisory; row-110 batch 2026-10-17).
# For each docs/status/README.md index row, extract the distinctive quoted
# spans ("...") from the summary cell and grep them against the cited
# report's body. An index row crediting a window with work its report body
# never mentions (the 03-37 "lint-lll verified already-excluded" class)
# passed every gate for four days and cost a review rejection + two
# re-verify windows. Mismatch = WARN pointing at the annotate-never-rewrite
# convention (TODO row 313); a hard-gate flip is a separate follow-up row.
set -uo pipefail
cd "$(dirname "$0")/.." || exit 1

min_len=12

run_check() {
	local readme="$1" base="$2"
	[ -f "$readme" ] || {
		echo "MISSING: $readme"
		return 1
	}
	local warn=0 rows=0 spans=0 missing=0
	local line report path text span
	while IFS= read -r line; do
		rows=$((rows + 1))
		report="$(awk -F'`' '{print $2}' <<<"$line")"
		case "$report" in
		*.md) ;;
		*) continue ;;
		esac
		path="$base/$report"
		[ -f "$path" ] || path="$(find "docs/status" -name "$(basename "$report")" -print -quit)"
		[ -f "$path" ] || {
			missing=$((missing + 1))
			continue
		}
		# Annotated spans inside a bracketed [YYYY-MM-DD ANNOTATION: ...] are
		# the annotate-never-rewrite carriers: they quote the drifted text ON
		# PURPOSE, so strip the brackets before extracting quotes.
		text="$(sed -E 's/\[[0-9]{4}-[0-9]{2}-[0-9]{2} ANNOTATION:[^]]*\]//g' <<<"$line")"
		body="$(tr -s '[:space:]' ' ' <"$path")"
		while IFS= read -r span; do
			spans=$((spans + 1))
			q="$(tr -s '[:space:]' ' ' <<<"${span:1:${#span}-2}")"
			# A trailing ellipsis marks a deliberately truncated excerpt: match
			# the prefix.
			q="${q%…}"
			q="${q%…}"
			read -r q <<<"$q"
			if [[ "$body" != *"$q"* ]]; then
				warn=$((warn + 1))
				echo "PHRASE-DRIFT WARN: quoted span not found in $report:"
				echo "  $span"
			fi
		done < <(grep -oE '"[^"]+"' <<<"$text" | awk -v m="$min_len" 'length($0) - 2 >= m')
	done < <(grep -E '^\| 2026-[0-9]{2}-[0-9]{2}[[:space:]]*\|`[^`]+\.md`' "$readme")
	echo "index phrase-drift: $rows rows, $spans spans, $warn WARN(s), $missing missing report(s) — advisory (annotate-never-rewrite; TODO row 313), continuing"
	# Advisory by design: drift is a WARN, never a gate failure.
	return 0
}

self_test() {
	local tmp
	tmp="$(mktemp -d)" || exit 1
	trap 'rm -rf "$tmp"' EXIT
	local body="$tmp/2026-01-01_selftest.md"
	printf 'body mentions the frobnicator settle ritual only here\n' >"$body"
	local idx="$tmp/README.md"
	{
		printf '| 2026-01-01 |`2026-01-01_selftest.md`| good row says "frobnicator settle ritual" and more |\n'
		printf '| 2026-01-02 |`2026-01-01_selftest.md`| drifted row says "quantum flux capacitor recalibrated" |\n'
	} >"$idx"
	local out
	out="$(run_check "$idx" "$tmp" 2>&1)"
	if grep -q "frobnicator settle ritual" <<<"$out" ||
		! grep -q "quantum flux capacitor recalibrated" <<<"$out"; then
		echo "SELF-TEST FAIL: matching span flagged or drift span not WARNed"
		echo "$out"
		exit 1
	fi
	if ! grep -q "1 WARN" <<<"$out"; then
		echo "SELF-TEST FAIL: expected exactly 1 WARN"
		echo "$out"
		exit 1
	fi
	echo "self-test ok (match quiet, drift WARNed)"
	exit 0
}

case "${1:-}" in
--self-test) self_test ;;
*) run_check "docs/status/README.md" "docs/status" ;;
esac
