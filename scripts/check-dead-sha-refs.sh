#!/usr/bin/env bash
# Dead-SHA citation gate: a 7-40-hex token in the living docs that resolves
# to a REAL git commit but is UNREACHABLE from HEAD means a history rewrite
# left a stale citation behind (the dd543ec class: it survived in TODO_LIST
# for a window after the msg-filter heal before 131ea17 fixed it).
#
# Scope: docs/status/*.md (top level) + TODO_LIST.md + AGENTS.md.
# Hex tokens that are NOT commit objects (queue task IDs, CI run numbers,
# line counts) are ignored — only commits that exist but dangle fail.
#
# Escapes:
#   - fork-record arrow form: a token is escaped only when it IS a member
#     of a `<sha>→<sha>` pair (either arrow direction) on its own line
#     (per-token matching: an unrelated arrow on the same line no longer
#     whitewashes other dead tokens on that line — 2026-10-05 tightening).
#   - baseline file scripts/dead-sha-baseline.txt (one sha per line, `#`
#     comments): the legacy citations from the 2026-09-10/2026-09-19
#     rewrites and later heal masses. Shrink is advisory-only; a NEW dead
#     sha not in the file fails the gate. Baselined hits are silent unless
#     TQ_DEAD_SHA_VERBOSE=1.
#
# --emit-baseline: generator mode (2026-10-05, owner ruling O16) — runs the
# same dead-token walk and prints a sorted, deduplicated baseline addendum
# on stdout (arrow-escaped tokens excluded; baseline membership ignored so
# regens are idempotent). Fork-record-vs-baseline curation stays human.
#
# --self-test pins the gate's decision branches against a /tmp fixture repo:
# clean run rc=0, non-baselined dead SHA rc=1 with exact file:line, per-token
# arrow escape (member passes, non-member on the same line still fails),
# --emit-baseline lists the dangling token, baseline-hit silent unless
# TQ_DEAD_SHA_VERBOSE=1.
set -uo pipefail

MODE="gate" # overridden to "emit" by --emit-baseline

# token_arrow_escaped <tok> <text>: rc=0 iff the token IS a member of an
# old→new fork-record pair on that line. Guards keep the token from
# matching inside a longer hex token.
token_arrow_escaped() {
	local tok=$1 text=$2
	if grep -qE "(^|[^0-9a-f])${tok}(→|<-)[0-9a-f]{7,40}" <<<"$text"; then return 0; fi
	if grep -qE "[0-9a-f]{7,40}(→|<-)${tok}([^0-9a-f]|$)" <<<"$text"; then return 0; fi
	return 1
}

# run_gate / emit: cwd must be the fixture/real git repo root; caller sets
# the `baseline` path and the `files` array. gate: returns 1 on any new
# (non-baselined, non-arrow) dead SHA. emit: prints sorted dead tokens.
run_gate() {
	local f line linenum text tok
	local fail=0 new_hits=0 baselined_hits=0
	local -A emit=()
	in_baseline() {
		local tok=$1 bl
		[ -f "$baseline" ] || return 1
		while IFS= read -r bl; do
			[[ "$bl" =~ ^# ]] && continue
			[ -z "$bl" ] && continue
			[ "$bl" = "$tok" ] && return 0
		done <"$baseline"
		return 1
	}
	for f in "${files[@]}"; do
		while IFS= read -r line; do
			linenum=${line%%:*}
			text=${line#*:}
			for tok in $(grep -oE '[0-9a-f]{7,40}' <<<"$text" | sort -u); do
				[ "$(git cat-file -t "$tok" 2>/dev/null)" = commit ] || continue
				git merge-base --is-ancestor "$tok" HEAD >/dev/null 2>&1 && continue
				token_arrow_escaped "$tok" "$text" && continue
				if [ "$MODE" = emit ]; then
					emit["$tok"]=1
					continue
				fi
				if in_baseline "$tok"; then
					baselined_hits=$((baselined_hits + 1))
					[ "${TQ_DEAD_SHA_VERBOSE:-0}" = 1 ] && echo "STALE (baselined): $f:$linenum cites '$tok' (commit exists, unreachable from HEAD)"
				else
					echo "DEAD SHA: $f:$linenum cites '$tok' (commit exists, unreachable from HEAD) — re-resolve, add a fork record (old→new), or baseline"
					fail=1
					new_hits=$((new_hits + 1))
				fi
			done
		done < <(grep -nE '[0-9a-f]{7,40}' "$f")
	done

	if [ "$MODE" = emit ]; then
		local k
		for k in "${!emit[@]}"; do echo "$k"; done | sort -u
		return 0
	fi
	if [ "$fail" = 0 ]; then
		echo "dead-sha refs ok (${baselined_hits} baselined, 0 new)"
	fi
	return "$fail"
}

# --self-test: /tmp fixture git repo with one dangling commit (a commit-tree
# child of HEAD — real commit object, unreachable, no git reset needed).
if [ "${1:-}" = "--self-test" ]; then
	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' EXIT
	mkdir -p "$tmp/docs/status"
	git init -q "$tmp"
	git -C "$tmp" -c user.email=t@t -c user.name=t commit --allow-empty -q -m base
	old=$(git -C "$tmp" -c user.email=t@t -c user.name=t commit-tree 'HEAD^{tree}' -p HEAD -m dangling)
	other=$(git -C "$tmp" -c user.email=t@t -c user.name=t commit-tree 'HEAD^{tree}' -p HEAD -m dangling2)
	new=$(git -C "$tmp" rev-parse HEAD)

	cat >"$tmp/AGENTS.md" <<EOF
fork playbook: history rewrote ${old}→${new}
EOF
	cat >"$tmp/TODO_LIST.md" <<EOF
legacy citation survives in the backlog: ${old}
EOF
	cat >"$tmp/docs/status/report.md" <<EOF
cites ${old} for context (rewrite casualty)
EOF
	cat >"$tmp/docs/status/mixed.md" <<EOF
unrelated arrow on the line: ${old} plus a playbook ${other}→${new}
EOF
	cat >"$tmp/docs/status/paired.md" <<EOF
proper pair: ${old}→${new}
EOF
	printf '%s\n' "# baseline" "$old" >"$tmp/base.txt"

	baseline="$tmp/base.txt"
	files=(AGENTS.md TODO_LIST.md docs/status/report.md)

	# 1. Baselined dead SHA + arrow line: clean, rc=0, hit counted, not printed.
	out=$(cd "$tmp" && run_gate) && rc=0 || rc=$?
	if [ "$rc" != 0 ] || ! grep -q 'dead-sha refs ok (2 baselined, 0 new)' <<<"$out"; then
		echo "dead-sha self-test FAIL: clean/baselined run rc=$rc out: $out"
		exit 1
	fi

	# 2. Baselined hit silent without the verbose flag, printed with it.
	out=$(cd "$tmp" && run_gate)
	grep -q 'STALE (baselined)' <<<"$out" && {
		echo "dead-sha self-test FAIL: baselined hit printed without TQ_DEAD_SHA_VERBOSE"
		exit 1
	}
	out=$(cd "$tmp" && TQ_DEAD_SHA_VERBOSE=1 run_gate)
	grep -q "STALE (baselined): TODO_LIST.md:1 cites '${old}'" <<<"$out" || {
		echo "dead-sha self-test FAIL: verbose flag did not surface TODO_LIST.md:1"
		exit 1
	}

	# 3. Non-baselined dead SHA: rc=1 with exact file:line.
	grep -v "^${old}$" "$tmp/base.txt" >"$tmp/base2.txt" && mv "$tmp/base2.txt" "$tmp/base.txt"
	out=$(cd "$tmp" && run_gate) && rc=0 || rc=$?
	if [ "$rc" != 1 ] || ! grep -q "DEAD SHA: docs/status/report.md:1 cites '${old}'" <<<"$out"; then
		echo "dead-sha self-test FAIL: non-baselined run rc=$rc out: $out"
		exit 1
	fi

	# 4. Per-token arrow escape: a token that IS a pair member passes, but a
	# dead token merely SHARING the line with an unrelated arrow still fails.
	files=(docs/status/paired.md)
	out=$(cd "$tmp" && run_gate) && rc=0 || rc=$?
	[ "$rc" = 0 ] || { echo "dead-sha self-test FAIL: proper arrow pair whitewashed: $out"; exit 1; }
	files=(docs/status/mixed.md)
	out=$(cd "$tmp" && run_gate) && rc=0 || rc=$?
	if [ "$rc" != 1 ] || ! grep -q "DEAD SHA: docs/status/mixed.md:1 cites '${old}'" <<<"$out"; then
		echo "dead-sha self-test FAIL: line-level arrow still whitewashes unrelated dead tokens: rc=$rc out: $out"
		exit 1
	fi

	# 5. --emit-baseline: sorted dead tokens (pair members excluded), then
	# feeding the emit output into the baseline makes the gate green.
	files=(docs/status/report.md docs/status/mixed.md docs/status/paired.md)
	MODE=emit
	out=$(cd "$tmp" && run_gate) && rc=0 || rc=$?
	[ "$rc" = 0 ] || { echo "dead-sha self-test FAIL: emit mode rc=$rc"; exit 1; }
	[ "$(grep -c '^[0-9a-f]\{7,40\}$' <<<"$out")" = 1 ] || { echo "dead-sha self-test FAIL: emit expected exactly 1 token (pair member excluded): $out"; exit 1; }
	grep -q "^${old}$" <<<"$out" || { echo "dead-sha self-test FAIL: emit missed the dangling token: $out"; exit 1; }
	grep -q "^${other}$" <<<"$out" && { echo "dead-sha self-test FAIL: emit included an arrow-pair member: $out"; exit 1; }
	{ echo "# emitted"; cat <<<"$out"; } >"$tmp/base.txt"
	files=(docs/status/mixed.md)
	MODE=gate
	out=$(cd "$tmp" && run_gate) && rc=0 || rc=$?
	[ "$rc" = 0 ] || { echo "dead-sha self-test FAIL: emitted baseline did not green the gate: $out"; exit 1; }

	echo "dead-sha self-test ok (clean rc=0, exact file:line on rc=1, per-token arrow escape, emit-baseline sorted+pair-excluded, baselined silent unless TQ_DEAD_SHA_VERBOSE=1)"
	exit 0
fi

cd "$(dirname "$0")/.." || exit 1

baseline="scripts/dead-sha-baseline.txt"

# Explicit file arguments (repo-root-relative) scope the gate to exactly
# those files — the verify-battery's own-file leg (TODO row 370) passes the
# window's touched files so a NEW dead cite outside the default scope
# (docs/planning/, docs/adr/, scripts docs …) still fails loudly. No args
# keeps the default living-docs scope.
if [ "${1:-}" = "--emit-baseline" ]; then
	MODE="emit"
	shift
fi

if [ "$#" -gt 0 ]; then
	files=()
	for f in "$@"; do
		[ -f "$f" ] && files+=("$f")
	done
else
	files=(AGENTS.md TODO_LIST.md)
	for f in docs/status/*.md; do [ -f "$f" ] && files+=("$f"); done
fi

run_gate
