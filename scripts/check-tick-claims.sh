#!/usr/bin/env bash
# Tick-claim guard (advisory WARN; TODO rows 1057/1058, 18-44 closeout §e2).
# Parses aggregate tick-count claims — "rows N-M ticked", "row N ticked",
# "rows N-M `[x]`" — from the docs/status README index rows + closeout
# reports and verifies every claimed row reads `- [x]` in TODO_LIST.md at
# the cited tick commit (`git show <sha>:TODO_LIST.md`); HEAD-only fallback
# when no tick commit is cited. The 2026-10-21 index row claimed "rows
# 776-778 ticked" while row 777 sat unchecked — the class passed every gate
# until a paid reviewer caught it. A commit is bound to a claim ONLY when
# it rides the tick verb itself (`ticked (sha)`, `ticked in|by|at|via sha`
# within the claim window), so the report's own commit or an UNCHECKED
# citation never hijacks the check (annotate-in-place corrections stay
# WARN-free when they cite the tick commit). archived/ reports are frozen
# annotate-never-rewrite history: not scanned. Mismatch = WARN pointing at
# the cite-the-tick-commit convention; hard-gate flip is a separate
# follow-up row, not bundled here.
set -uo pipefail
export LC_ALL=C
cd "$(dirname "$0")/.." || exit 1
self_abs="$(cd "$(dirname "$0")" && pwd)/$(basename "$0")"

declare -A LIST_CACHE=()

list_at() {
	local sha="$1"
	if [[ -n "${LIST_CACHE[$sha]:-}" ]]; then
		printf '%s' "${LIST_CACHE[$sha]}"
		return 0
	fi
	local content
	if [ -n "${TQ_TICK_LIST_CMD:-}" ]; then
		content="$("$TQ_TICK_LIST_CMD" "$sha")" || return 1
	else
		content="$(git show "$sha:TODO_LIST.md")" || return 1
	fi
	LIST_CACHE[$sha]=$content
	printf '%s' "$content"
}

oks=0
warns=0
skips=0

verify_claim() {
	local file="$1" lineno="$2" span="$3" a="$4" b="$5" sha="$6"
	local at="${sha:-HEAD}" list n row
	if [ "$b" -lt "$a" ] || [ $((b - a)) -gt 50 ]; then
		skips=$((skips + 1))
		echo "TICK-CLAIM SKIP: $file:$lineno \"$span\" — degenerate range"
		return
	fi
	list="$(list_at "$at")" || {
		skips=$((skips + 1))
		echo "TICK-CLAIM SKIP: $file:$lineno \"$span\" — TODO_LIST unreadable at $at"
		return
	}
	for ((n = a; n <= b; n++)); do
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
	local file="$1" lineno="$2" line="$3"
	local apos="'"
	local claim_re="rows? [0-9]+([/-][0-9]+)?([[:space:]]+[A-Za-z${apos}]+){0,2}[[:space:]]+(ticked|\`?\[x\]\`?)"
	local off span low window sha match
	while IFS=: read -r off span; do
		[ -n "$span" ] || continue
		low="${span,,}"
		case "$low" in
		*not* | *never* | *unticked*) continue ;;
		esac
		[[ "$span" =~ ^rows?[[:space:]]([0-9]+)([/-]([0-9]+))? ]] || continue
		local a="${BASH_REMATCH[1]}" b="${BASH_REMATCH[3]:-}"
		[ -n "$b" ] || b="$a"
		sha=""
		window="${line:off+${#span}:140}"
		if [[ "$window" =~ \(([0-9a-f]{8,40})\) ]]; then
			sha="${BASH_REMATCH[1]}"
		elif [[ "$window" =~ [[:space:]](in|by|at|via)[[:space:]]+([0-9a-f]{8,40}) ]]; then
			sha="${BASH_REMATCH[2]}"
		fi
		verify_claim "$file" "$lineno" "$span" "$a" "$b" "$sha"
		# shellcheck disable=SC2034  # match is read by IFS
		match=1
	done < <(grep -obE "$claim_re" <<<"$line" || true)
}

scan_file() {
	local f="$1" lineno=0 line
	while IFS= read -r line || [ -n "$line" ]; do
		lineno=$((lineno + 1))
		scan_line "$f" "$lineno" "$line"
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
aaaaaaaaaaaaaaaa) printf '%s\n' '- [x] alpha landed' '- [x] beta landed' ;;
bbbbbbbbbbbbbbbb) printf '%s\n' '- [x] alpha landed' '- [ ] beta open' ;;
*) exit 1 ;;
esac
EOF
	chmod +x "$tmp/listcmd.sh"
	printf 'report says row 1 ticked and rows 1-3 [x]\n' >"$tmp/f1.md"
	printf 'sha-cited rows 1-2 ticked in aaaaaaaaaaaaaaaa plus row 2 ticked in bbbbbbbbbbbbbbbb\n' >"$tmp/f2.md"
	printf 'negatives: rows 1-2 unticked, row 3 not ticked, row 4 was UNCHECKED\n' >"$tmp/f3.md"
	printf 'broken row 1 ticked in deadbeef00\n' >"$tmp/f4.md"
	local files out out2
	files="$(printf '%s\n' "$tmp/f1.md" "$tmp/f2.md" "$tmp/f3.md" "$tmp/f4.md")"
	out="$(TQ_TICK_LIST_CMD="$tmp/listcmd.sh" TQ_TICK_CLAIM_FILES="$files" "$self_abs" 2>&1)"
	if ! grep -qF 'tick-claims summary: 2 ok, 2 warned, 1 skipped' <<<"$out"; then
		echo "FAIL: negative battery counters off (want 2 ok, 2 warned, 1 skipped)"
		printf '%s\n' "$out"
		exit 1
	fi
	if [ "$(grep -c 'TICK-CLAIM WARN' <<<"$out")" -ne 2 ]; then
		echo "FAIL: expected exactly 2 WARNs (bad-HEAD range + wrong-cited-sha)"
		printf '%s\n' "$out"
		exit 1
	fi
	if grep -q 'rows 1-2 ticked in aaaaaaaaaaaaaaaa' <<<"$out"; then
		echo "FAIL: sha-cited true range was flagged"
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
