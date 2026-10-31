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
# time (2026-10-25; T14 widening 2026-10-31: 352 ok / 22 WARN — the
# {0,4} window + no-flip + BLOCKED rails surfaced 12 more true residue
# sites, incl. the row-1114 "row 143 was already DONE and ticked"
# falsehood): historical claim-ahead residue,
# incl. the originally reviewer-caught 68a20cdb "row 777 `[x]`" falsehood
# the gate now catches mechanically; a baseline REGRESSION is the signal.
#
# T14 extensions (2026-10-31):
#   - claim_re window {0,2}→{0,3} (row 1114: "row 143 was already DONE and
#     ticked" class);
#   - an EXPLICITLY cited tick commit must itself flip a `- [x]` row
#     (row 22: the 02-34 fixture cited a commit whose diffstat ticked the
#     adjacent anchor) — TQ_TICK_DIFF_CMD overrides the diff fetch;
#   - BLOCKED-claims ("BLOCKED rows written/annotated (sha)", "rows N
#     annotated (sha)") verify the sha's TODO_LIST diff actually adds a
#     `— BLOCKED:` line (row 1405: the 1b2fee1d class claimed both rows
#     while touching one);
#   - COMMIT-MESSAGE claims (row 1405 open half, 2026-10-31): messages
#     asserting the blocking action ("mark … rows blocked", "both rows
#     annotated (sha)", "BLOCKED rows written") verify the target diff —
#     the cited sha's, else the claiming commit's own — adds `— BLOCKED:`
#     lines, and an explicit count ("both"/"two"/"N rows") must be covered
#     by that many added lines; the original 1b2fee1d subject claimed both
#     rows while its diff touched one;
#   - `--batch-anchors <file>` (row 1403): every anchor's TODO row must be
#     `[x]` or carry `— BLOCKED:` — the pre-final-commit tick-audit rail,
#     FAILing (not WARN) because the caller names its own batch.
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

diff_at() {
	# diff_at SHA: the sha's TODO_LIST.md diff (TQ_TICK_DIFF_CMD overrides
	# for the self-test negative battery).
	if [ -n "${TQ_TICK_DIFF_CMD:-}" ]; then
		"$TQ_TICK_DIFF_CMD" "$1"
	else
		git show "$1" -- TODO_LIST.md 2>/dev/null || true
	fi
}

verify_claim() {
	local file="$1" lineno="$2" span="$3" rows="$4" key="$5" explicit="${6:-0}"
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
	# Row 22: an EXPLICITLY cited tick commit must itself flip a `[x]` row —
	# a sha that rides the tick verb but ticks nothing (the 02-34 adjacent-
	# anchor class) is a bad citation even when the row reads `[x]` today.
	if [ "$explicit" = "1" ] && [[ "$key" =~ ^[0-9a-f]{8,40}$ ]] && [ "$key" != "HEAD" ]; then
		if ! diff_at "$key" | grep -qE '^\+.*- \[[xX]\]'; then
			warns=$((warns + 1))
			echo "TICK-CLAIM WARN: $file:$lineno \"$span\" — cited tick commit $key flips no '- [x]' row in its TODO_LIST diff"
			return
		fi
	fi
	oks=$((oks + 1))
}

verify_blocked_claim() {
	# Row 1405: "BLOCKED rows written/annotated" and "rows N … annotated"
	# claims carrying an explicit sha must have that sha's TODO_LIST diff
	# actually add a `— BLOCKED:` line (the 1b2fee1d claimed-both-touched-one
	# class).
	local file="$1" lineno="$2" text="$3" sha="$4"
	if diff_at "$sha" | grep -qE '^\+.*— BLOCKED:'; then
		oks=$((oks + 1))
		return
	fi
	warns=$((warns + 1))
	echo "TICK-CLAIM WARN: $file:$lineno \"$text\" — cited commit $sha adds no '— BLOCKED:' line"
}

scan_commit_messages() {
	# Row 1405 open half: COMMIT-MESSAGE action claims. A message asserting
	# the blocking action ("mark X rows blocked", "both rows annotated",
	# "BLOCKED rows written") is a claim about a DIFF: the cited sha's when
	# one rides the claim, else the claiming commit's own. The target diff
	# must add `— BLOCKED:` lines; an explicit count (both/two/N rows) must
	# be covered by that many additions. Messages are frozen history, so a
	# WARN here is permanent residue — the same posture as the file-claim
	# side (baseline regressions are the signal). Scans message SUBJECTS
	# (where the 1b2fee1d class lives; bodies stay unscanned — prose).
	# TQ_TICK_MSG_CMD overrides the git log fetch for the self-test
	# (prints 'sha<TAB>subject' lines).
	local win="${TQ_TICK_MSG_WINDOW:-300}" sha msg low claim target need got rec
	local msg_claim_re='(mark(ed)?|writ(e|ten)|annotat(e|ed)|record(ed)?|land(ed)?)[^.]{0,80}rows?[^.]{0,40}blocked|blocked[^.]{0,40}rows?[^.]{0,80}(written|annotated|landed|marked)|((both|two|[0-9]+)[^.]{0,20}rows?[^.]{0,30}(annotated|written|blocked))'
	while IFS=$'\t' read -r sha msg; do
		[ -n "${sha:-}" ] || continue
		low="${msg,,}"
		case "$low" in
		'*not blocked*' | *unblock* | '*never blocked*') continue ;;
		esac
		while IFS= read -r claim; do
			[ -n "$claim" ] || continue
			target="$sha"
			if [[ "$claim" =~ \(([0-9a-f]{8,40})\) ]]; then
				target="${BASH_REMATCH[1]}"
			elif [[ "$claim" =~ [[:space:]](in|by|at|via)[[:space:]]+([0-9a-f]{8,40}) ]]; then
				target="${BASH_REMATCH[2]}"
			fi
			need=1
			if [[ "$claim" =~ (both|two)[[:space:]]+(rows?|items?|annotations?) ]]; then
				need=2
			elif [[ "$claim" =~ ([0-9]+)[[:space:]]+rows ]]; then
				need="${BASH_REMATCH[1]}"
			fi
			got="$(diff_at "$target" | grep -cE '^\+.*— BLOCKED:' || true)"
			if [ "${got:-0}" -ge "$need" ]; then
				oks=$((oks + 1))
			else
				warns=$((warns + 1))
				echo "TICK-CLAIM WARN: commit-message claim \"${claim:0:90}\" ($sha) — target $target adds $got '— BLOCKED:' line(s), claim needs $need"
			fi
		done < <(grep -oiE "$msg_claim_re" <<<"$msg" || true)
	done < <(msg_records "$win")
}

msg_records() {
	# One 'sha<TAB>subject' record per commit touching TODO_LIST.md,
	# newest first, bounded by the window.
	local win="$1"
	if [ -n "${TQ_TICK_MSG_CMD:-}" ]; then
		"$TQ_TICK_MSG_CMD"
	else
		git log -n "$win" --format='%H%x09%s' -- TODO_LIST.md 2>/dev/null || true
	fi
}

scan_line() {
	local file="$1" lineno="$2" line="$3" fallback="$4" readme="$5"
	local apos="'"
	local claim_re="rows? [0-9]+([/-][0-9]+)?([[:space:]]+[A-Za-z${apos}]+){0,4}[[:space:]]+(ticked|\`?\[x\]\`?)"
	local off span low window key sep rows sc cachekey n explicit bsha
	# Row 1405: BLOCKED/annotated-claims with an explicit sha.
	if [[ "$line" =~ (BLOCKED[[:space:]]+(rows?|annotations?)[[:space:]]+(written|landed|annotated)|rows?[[:space:]]+[0-9][0-9 /,-]*[[:space:]]+annotated) ]]; then
		local btext="${BASH_REMATCH[0]}" bwin="${line:${#BASH_REMATCH[0]}:80}" bsha=""
		if [[ "$bwin" =~ \(([0-9a-f]{8,40})\) ]]; then
			bsha="${BASH_REMATCH[1]}"
		elif [[ "$bwin" =~ [[:space:]](in|by|at|via)[[:space:]]+([0-9a-f]{8,40}) ]]; then
			bsha="${BASH_REMATCH[2]}"
		fi
		if [ -n "$bsha" ]; then
			verify_blocked_claim "$file" "$lineno" "$btext" "$bsha"
		fi
	fi
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
		explicit=0
		window="${line:off+${#span}:140}"
		if [[ "$window" =~ \(([0-9a-f]{8,40})\) ]]; then
			key="${BASH_REMATCH[1]}"
			explicit=1
		elif [[ "$window" =~ [[:space:]](in|by|at|via)[[:space:]]+([0-9a-f]{8,40}) ]]; then
			key="${BASH_REMATCH[2]}"
			explicit=1
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
		verify_claim "$file" "$lineno" "$span" "$rows" "$key" "$explicit"
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

batch_anchors() {
	# Row 1403: the pre-final-commit tick-audit rail. Every anchor's TODO
	# row must be [x] or carry an explicit '— BLOCKED: <reason>' — an open
	# unannotated row at closeout is the paid-round-trip class. FAILs (not
	# WARN): the caller names its own batch, so there is no historical
	# residue to grandfather. TQ_TICK_TODO_FILE overrides the list for the
	# self-test.
	local anchors="$1" todo="${TQ_TICK_TODO_FILE:-TODO_LIST.md}" fail=0 a row n
	while IFS= read -r a; do
		[ -n "$a" ] || continue
		n=0
		while IFS= read -r row; do
			n=$((n + 1))
			case "$row" in
			*'- [x]'*)
				echo "ok: anchor ticked — ${a:0:60}"
				;;
			*'— BLOCKED:'*)
				echo "ok: anchor blocked-annotated — ${a:0:60}"
				;;
			*)
				echo "BATCH-ANCHOR FAIL: TODO_LIST.md line ${row%%:*} is neither [x] nor '— BLOCKED:' — tick the row or append ' — BLOCKED: <reason>'" >&2
				fail=1
				;;
			esac
		done < <(grep -nF -- "$a" "$todo" 2>/dev/null | grep -E '^[0-9]+:- \[.\]' || true)
		if [ "$n" = "0" ]; then
			echo "BATCH-ANCHOR FAIL: no TODO row carries anchor: $a" >&2
			fail=1
		fi
	done <"$anchors"
	echo "batch-anchors summary: checked $(grep -c . "$anchors" || true) anchor(s)"
	[ "$fail" = "0" ]
}

self_test() {
	# tmp is deliberately global: the EXIT trap reads it after self_test
	# returned, and set -u would kill the trap on a function-local name.
	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' EXIT
	# pins: list-resolution-branches — HEAD/day/sha tree commands return the row states each authorship-tree resolver path consumes
	cat >"$tmp/listcmd.sh" <<'EOF'
#!/bin/sh
case "$1" in
HEAD) printf '%s\n' '- [x] alpha landed' '- [ ] beta open' '- [x] gamma landed' ;;
day:2026-01-01) printf '%s\n' '- [x] alpha landed' '- [ ] beta pending' ;;
aaaaaaaaaaaaaaaa) printf '%s\n' '- [x] alpha landed' '- [x] beta landed' ;;
dddddddddddddddd) printf '%s\n' '- [x] alpha landed' '- [x] beta landed' ;;
bbbbbbbbbbbbbbbb) printf '%s\n' '- [x] alpha landed' '- [ ] beta open' ;;
*) exit 1 ;;
esac
EOF
	chmod +x "$tmp/listcmd.sh"
	cat >"$tmp/diffcmd.sh" <<'EOF'
#!/bin/sh
case "$1" in
aaaaaaaaaaaaaaaa) printf '%s\n' '+- [x] alpha landed' ;;
cccccccccccccccc) printf '%s\n' '+- [x] alpha landed' '+- [ ] beta open — BLOCKED: fixture' ;;
eeeeeeeeeeeeeeee) printf '%s\n' '+- [ ] one row — BLOCKED: fixture' ;;
*) exit 0 ;;
esac
EOF
	chmod +x "$tmp/diffcmd.sh"
	cat >"$tmp/msgcmd.sh" <<'EOF'
#!/bin/sh
printf '%s\t%s\n' \
	eeeeeeeeeeeeeeee 'docs: mark todo rows blocked on owner rulings' \
	ffffffffffffffff 'docs: both rows annotated (cccccccccccccccc)' \
	9999999999999999 'docs: mark todo rows blocked but touching nothing' \
	8888888888888888 'chore: unrelated subject with no claim'
EOF
	chmod +x "$tmp/msgcmd.sh"
	printf 'report says row 1 ticked, rows 1/3 [x], and rows 1-3 [x]\n' >"$tmp/f1.md"
	printf 'sha-cited rows 1-2 ticked in aaaaaaaaaaaaaaaa plus row 2 ticked in bbbbbbbbbbbbbbbb\n' >"$tmp/f2.md"
	printf 'negatives: rows 1-2 unticked, row 3 not ticked, row 4 was UNCHECKED\n' >"$tmp/f3.md"
	printf 'broken row 1 ticked in deadbeef00\n' >"$tmp/f4.md"
	printf 'dated claim: row 2 ticked\n' >"$tmp/2026-01-01_00-00_selftest.md"
	# T14 shapes: wide-window claim (row 1114), explicit-sha no-flip
	# (row 22), BLOCKED-claim diff verification (row 1405).
	printf 'wide window: row 1 was already DONE and ticked\n' >"$tmp/f5.md"
	printf 'then row 1 ticked (dddddddddddddddd)\n' >>"$tmp/f5.md"
	printf 'blocked claim: BLOCKED rows written (dddddddddddddddd)\nBLOCKED rows written (cccccccccccccccc)\n' >"$tmp/f6.md"
	local files out out2
	files="$(printf '%s\n' "$tmp/f1.md" "$tmp/f2.md" "$tmp/f3.md" "$tmp/f4.md" "$tmp/2026-01-01_00-00_selftest.md" "$tmp/f5.md" "$tmp/f6.md")"
	out="$(TQ_TICK_LIST_CMD="$tmp/listcmd.sh" TQ_TICK_DIFF_CMD="$tmp/diffcmd.sh" TQ_TICK_MSG_CMD="$tmp/msgcmd.sh" TQ_TICK_CLAIM_FILES="$files" "$self_abs" 2>&1)"
	# file-claims: 5 ok / 5 warned / 1 skipped (as before); message-claims:
	# eeee bare-claim ok (+1), ffff "both … (cccc)" needs 2 but cccc adds 1
	# (+1 warn), 9999 bare claim with an empty diff (+1 warn) → 6/7/1.
	if ! grep -qF 'tick-claims summary: 6 ok, 7 warned, 1 skipped' <<<"$out"; then
		echo "FAIL: negative battery counters off (want 6 ok, 7 warned, 1 skipped)"
		printf '%s\n' "$out"
		exit 1
	fi
	if [ "$(grep -c 'TICK-CLAIM WARN' <<<"$out")" -ne 7 ]; then
		echo "FAIL: expected exactly 7 WARNs (bad-HEAD range + wrong-cited-sha + stale-day + explicit-no-flip + blocked-no-evidence + msg-count + msg-no-evidence)"
		printf '%s\n' "$out"
		exit 1
	fi
	if ! grep -q 'flips no' <<<"$out"; then
		echo "FAIL: explicit-sha no-flip WARN missing (row 22 class)"
		printf '%s\n' "$out"
		exit 1
	fi
	if ! grep -q "adds no '— BLOCKED:'" <<<"$out"; then
		echo "FAIL: BLOCKED-claim WARN missing (row 1405 class)"
		printf '%s\n' "$out"
		exit 1
	fi
	if ! grep -qF 'commit-message claim' <<<"$out"; then
		echo "FAIL: commit-message WARN missing (row 1405 open half)"
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
	# Row 1403 batch-anchors rail: ticked + blocked anchors pass, a bare
	# open row FAILs, a missing anchor FAILs.
	printf '%s\n' '- [x] alpha landed' '- [ ] beta open' '- [ ] gamma open — BLOCKED: fixture reason' >"$tmp/todo.md"
	printf '%s\n' 'alpha landed' 'gamma open' >"$tmp/anchors-ok.txt"
	printf '%s\n' 'beta open' 'no such anchor anywhere' >"$tmp/anchors-bad.txt"
	local rc2=0 rc3=0
	TQ_TICK_TODO_FILE="$tmp/todo.md" "$self_abs" --batch-anchors "$tmp/anchors-ok.txt" >/dev/null 2>&1 || rc2=$?
	TQ_TICK_TODO_FILE="$tmp/todo.md" "$self_abs" --batch-anchors "$tmp/anchors-bad.txt" >/dev/null 2>&1 || rc3=$?
	if [ "$rc2" != "0" ] || [ "$rc3" != "1" ]; then
		echo "FAIL: batch-anchors rail (want clean=0 bad=1, got $rc2/$rc3)"
		exit 1
	fi
	echo "tick-claims self-test ok (summary format pin + negative counter battery + no-flip/blocked/wide-window/batch-anchors/commit-message rails)"
	exit 0
}

if [ "${1:-}" = "--self-test" ]; then
	self_test
	exit 0
fi

if [ "${1:-}" = "--batch-anchors" ]; then
	[ -n "${2:-}" ] || {
		echo "usage: $0 --batch-anchors <anchors-file>" >&2
		exit 2
	}
	batch_anchors "$2"
	exit $?
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

# TQ_TICK_CLAIM_FILES overrides the FILE set for the self-test; message
# scanning stays live there via TQ_TICK_MSG_CMD so both surfaces are
# exercised in one run.
scan_commit_messages

echo "tick-claims summary: $oks ok, $warns warned, $skips skipped"
echo "advisory (WARN-only; cite-the-tick-commit convention, TODO row 1058) — continuing"
exit 0
