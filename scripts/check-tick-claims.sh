#!/usr/bin/env bash
# Tick-claim guard (advisory WARN; TODO rows 1057/1058, 18-44 closeout §e2).
# Parses aggregate tick-count claims — "rows N-M ticked", "row N ticked",
# "rows N-M `[x]`" — from the docs/status README index rows + closeout
# reports and verifies every claimed row reads `- [x]` in TODO_LIST.md at
# the claim's own authorship tree: a filename-dated report verifies at its
# ADD commit (`git log --diff-filter=A` — the exact row numbering its
# author saw; same-day mints/prunes would skew an end-of-day snapshot), a
# README index row at its date cell's end-of-day tree, an explicitly cited
# tick commit overrides, and an undated file falls back to HEAD. The
# 2026-10-21 index row claimed "rows 776-778 ticked" while row
# 777 sat unchecked — the class passed every gate until a paid reviewer
# caught it; authorship-tree verification catches it at its own commit
# instead of re-litigating renumbered rows forever. A commit is bound to a
# claim ONLY
# when it rides the tick verb itself (`ticked (sha)`, `ticked
# in|by|at|via sha` within the claim window), so the report's own commit or
# an UNCHECKED citation never hijacks the check (annotate-in-place
# corrections stay WARN-free when they cite the tick commit). archived/
# reports are frozen annotate-never-rewrite history: not scanned. Mismatch
# = WARN pointing at the cite-the-tick-commit convention; hard-gate flip is
# a separate follow-up row, not bundled here. Live-tree baseline at ship
# time (2026-10-25): 318 ok / 10 WARN — historical claim-ahead residue,
# incl. the originally reviewer-caught 68a20cdb "row 777 `[x]`" falsehood
# the gate now catches mechanically; a baseline REGRESSION is the signal.
set -uo pipefail
export LC_ALL=C
cd "$(dirname "$0")/.." || exit 1
self_abs="$(cd "$(dirname "$0")" && pwd)/$(basename "$0")"

declare -A LIST_CACHE=()
declare -A SPAN_BASE=()

# list_at KEY: KEY is "HEAD", "day:YYYY-MM-DD" (that day's end-of-day
# tree), or a raw commit-ish. TQ_TICK_LIST_CMD overrides the fetch for the
# self-test negative battery (same KEY contract).
list_at() {
	local key="$1"
	if [[ -n "${LIST_CACHE[$key]:-}" ]]; then
		printf '%s' "${LIST_CACHE[$key]}"
		return 0
	fi
	local content rev
	if [ -n "${TQ_TICK_LIST_CMD:-}" ]; then
		content="$("$TQ_TICK_LIST_CMD" "$key")" || return 1
	elif [[ "$key" == "HEAD" ]]; then
		content="$(git show "HEAD:TODO_LIST.md")" || return 1
	elif [[ "$key" == day:* ]]; then
		rev="$(git rev-list -1 --before="${key#day:} 23:59:59" HEAD)" || return 1
		[ -n "$rev" ] || return 1
		content="$(git show "$rev:TODO_LIST.md")" || return 1
	else
		content="$(git show "$key:TODO_LIST.md")" || return 1
	fi
	LIST_CACHE[$key]=$content
	printf '%s' "$content"
}

oks=0
warns=0
skips=0

verify_claim() {
	local file="$1" lineno="$2" span="$3" rows="$4" key="$5"
	local at="$key" list n row
	if [ "$(wc -w <<<"$rows")" -gt 50 ]; then
		skips=$((skips + 1))
		echo "TICK-CLAIM SKIP: $file:$lineno \"$span\" — degenerate range"
		return
	fi
	list="$(list_at "$at")" || {
		skips=$((skips + 1))
		echo "TICK-CLAIM SKIP: $file:$lineno \"$span\" — TODO_LIST unreadable at $at"
		return
	}
	for n in $rows; do
		row="$(sed -n "${n}p" <<<"$list")"
		if [[ "$row" == "- [x]"* || "$row" == "- [X]"* ]]; then
			continue
		fi
		warns=$((warns + 1))
		echo "TICK-CLAIM WARN: $file:$lineno \"$span\" — row $n is not '- [x]' at $at"
		echo "  row $n reads: ${row:0:120}"
		echo "  cite the tick commit (AGENTS.md: tick-claims cite the tick commit) or fix the claim"
		return
	done
	oks=$((oks + 1))
}

scan_line() {
	local file="$1" lineno="$2" line="$3" fallback="$4" readme="$5"
	local apos="'"
	local claim_re="rows? [0-9]+([/-][0-9]+)?([[:space:]]+[A-Za-z${apos}]+){0,2}[[:space:]]+(ticked|\`?\[x\]\`?)"
	local off span low window key sep rows sc cachekey n
	while IFS=: read -r off span; do
		[ -n "$span" ] || continue
		low="${span,,}"
		case "$low" in
		*not* | *never* | *unticked*) continue ;;
		esac
		[[ "$span" =~ ^rows?[[:space:]]([0-9]+)([/-]([0-9]+))? ]] || continue
		local a="${BASH_REMATCH[1]}" b="${BASH_REMATCH[3]:-}" sep="${BASH_REMATCH[2]:-}"
		# "rows 32-34" spans the range; "rows 32/34" names two endpoints —
		# the slash list must not sweep the rows in between.
		rows="$a"
		if [ -n "$b" ]; then
			if [[ "$sep" == /* ]]; then
				rows="$a $b"
			else
				for ((n = a + 1; n <= b; n++)); do rows="$rows $n"; done
			fi
		fi
		key=""
		window="${line:off+${#span}:140}"
		if [[ "$window" =~ \(([0-9a-f]{8,40})\) ]]; then
			key="${BASH_REMATCH[1]}"
		elif [[ "$window" =~ [[:space:]](in|by|at|via)[[:space:]]+([0-9a-f]{8,40}) ]]; then
			key="${BASH_REMATCH[2]}"
		elif [ "$readme" = 1 ]; then
			# README.md has no file-level authorship commit; the pickaxe
			# commit that last changed this exact claim span is the closest
			# authorship-time numbering context (falls back to the date cell).
			cachekey="$file|$span"
			if [[ -n "${SPAN_BASE[$cachekey]:-}" ]]; then
				key="${SPAN_BASE[$cachekey]}"
			else
				sc="$(git log -1 --format=%h -S"$span" -- "$file" 2>/dev/null || true)"
				if [ -n "$sc" ]; then
					key="$sc"
					SPAN_BASE[$cachekey]="$sc"
				fi
			fi
		fi
		[ -n "$key" ] || key="$fallback"
		verify_claim "$file" "$lineno" "$span" "$rows" "$key"
	done < <(grep -obE "$claim_re" <<<"$line" || true)
}

scan_file() {
	local f="$1" lineno=0 line fallback add readme=0
	[[ "$(basename "$f")" == "README.md" ]] && readme=1
	if [[ "$f" =~ (^|/)([0-9]{4}-[0-9]{2}-[0-9]{2})_ ]]; then
		# The report's own ADD commit is the tree its row numbers were true
		# against (same-day mints/prunes would skew an end-of-day snapshot);
		# unresolvable (fixture/untracked) falls back to the filename day.
		add="$(git log --diff-filter=A --format=%h -- "$f" 2>/dev/null | tail -1 || true)"
		if [ -n "$add" ]; then
			fallback="$add"
		else
			fallback="day:${BASH_REMATCH[2]}"
		fi
	else
		fallback="HEAD"
	fi
	while IFS= read -r line || [ -n "$line" ]; do
		lineno=$((lineno + 1))
		# README index rows carry their own date cell — a fresher fallback
		# than any file-level date, since README.md is continuously edited.
		local rowfallback="$fallback"
		if [[ "$line" =~ ^\|[[:space:]]*([0-9]{4}-[0-9]{2}-[0-9]{2}) ]]; then
			rowfallback="day:${BASH_REMATCH[1]}"
		fi
		scan_line "$f" "$lineno" "$line" "$rowfallback" "$readme"
	done <"$f"
}

self_test() {
	# tmp is deliberately global: the EXIT trap reads it after self_test
	# returned, and set -u would kill the trap on a function-local name.
	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' EXIT
	cat >"$tmp/listcmd.sh" <<'EOF'
#!/bin/sh
case "$1" in
HEAD) printf '%s\n' '- [x] alpha landed' '- [ ] beta open' '- [x] gamma landed' ;;
day:2026-01-01) printf '%s\n' '- [x] alpha landed' '- [ ] beta pending' ;;
aaaaaaaaaaaaaaaa) printf '%s\n' '- [x] alpha landed' '- [x] beta landed' ;;
bbbbbbbbbbbbbbbb) printf '%s\n' '- [x] alpha landed' '- [ ] beta open' ;;
*) exit 1 ;;
esac
EOF
	chmod +x "$tmp/listcmd.sh"
	printf 'report says row 1 ticked, rows 1/3 [x], and rows 1-3 [x]\n' >"$tmp/f1.md"
	printf 'sha-cited rows 1-2 ticked in aaaaaaaaaaaaaaaa plus row 2 ticked in bbbbbbbbbbbbbbbb\n' >"$tmp/f2.md"
	printf 'negatives: rows 1-2 unticked, row 3 not ticked, row 4 was UNCHECKED\n' >"$tmp/f3.md"
	printf 'broken row 1 ticked in deadbeef00\n' >"$tmp/f4.md"
	printf 'dated claim: row 2 ticked\n' >"$tmp/2026-01-01_00-00_selftest.md"
	local files out out2
	files="$(printf '%s\n' "$tmp/f1.md" "$tmp/f2.md" "$tmp/f3.md" "$tmp/f4.md" "$tmp/2026-01-01_00-00_selftest.md")"
	out="$(TQ_TICK_LIST_CMD="$tmp/listcmd.sh" TQ_TICK_CLAIM_FILES="$files" "$self_abs" 2>&1)"
	if ! grep -qF 'tick-claims summary: 3 ok, 3 warned, 1 skipped' <<<"$out"; then
		echo "FAIL: negative battery counters off (want 3 ok, 3 warned, 1 skipped)"
		printf '%s\n' "$out"
		exit 1
	fi
	if [ "$(grep -c 'TICK-CLAIM WARN' <<<"$out")" -ne 3 ]; then
		echo "FAIL: expected exactly 3 WARNs (bad-HEAD range + wrong-cited-sha + stale-day)"
		printf '%s\n' "$out"
		exit 1
	fi
	if grep -q 'rows 1-2 ticked in aaaaaaaaaaaaaaaa' <<<"$out"; then
		echo "FAIL: sha-cited true range was flagged"
		printf '%s\n' "$out"
		exit 1
	fi
	if grep -q 'rows 1/3' <<<"$out"; then
		echo "FAIL: slash list swept between endpoints (rows 1/3 must check 1 and 3 only)"
		printf '%s\n' "$out"
		exit 1
	fi
	if grep -Eq 'not ticked|unticked|UNCHECKED' <<<"$out"; then
		echo "FAIL: negative-statement guard leaked a false claim"
		printf '%s\n' "$out"
		exit 1
	fi
	if ! grep -q 'at bbbbbbbbbbbbbbbb' <<<"$out"; then
		echo "FAIL: WARN does not name the cited (wrong) tick commit"
		printf '%s\n' "$out"
		exit 1
	fi
	if ! grep -q 'at day:2026-01-01' <<<"$out"; then
		echo "FAIL: day-tree fallback key not exercised"
		printf '%s\n' "$out"
		exit 1
	fi
	out2="$("$self_abs" 2>&1)"
	if ! grep -Eq '^tick-claims summary: [0-9]+ ok, [0-9]+ warned, [0-9]+ skipped$' <<<"$out2"; then
		echo "FAIL: real-run summary missing or malformed"
		printf '%s\n' "$out2"
		exit 1
	fi
	echo "tick-claims self-test ok (summary format pin + negative counter battery)"
	exit 0
}

if [ "${1:-}" = "--self-test" ]; then
	self_test
	exit 0
fi

files=()
if [ -n "${TQ_TICK_CLAIM_FILES:-}" ]; then
	while IFS= read -r f; do
		[ -n "$f" ] && files+=("$f")
	done <<<"$TQ_TICK_CLAIM_FILES"
else
	while IFS= read -r f; do
		files+=("$f")
	done < <(find docs/status -maxdepth 2 -name '*.md' ! -path '*archived*' -type f | sort)
fi

for f in "${files[@]}"; do
	if [ -f "$f" ]; then
		scan_file "$f"
	else
		skips=$((skips + 1))
		echo "TICK-CLAIM SKIP: missing claim file $f"
	fi
done

echo "tick-claims summary: $oks ok, $warns warned, $skips skipped"
echo "advisory (WARN-only; cite-the-tick-commit convention, TODO row 1058) — continuing"
exit 0
