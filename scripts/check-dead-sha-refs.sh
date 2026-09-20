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
#   - fork-record arrow form: a line containing `<sha>→<sha>` (either arrow
#     direction) is a sanctioned old→new fork record; both tokens on that
#     line are allowed (e.g. the AGENTS.md filter-branch playbook).
#   - baseline file scripts/dead-sha-baseline.txt (one sha per line, `#`
#     comments): the legacy citations from the 2026-09-10/2026-09-19 rewrites.
#     Shrink is advisory-only; a NEW dead sha not in the file fails the gate.
#     Baselined hits are silent unless TQ_DEAD_SHA_VERBOSE=1.
#
# --self-test pins the gate's decision branches against a /tmp fixture repo:
# clean run rc=0, non-baselined dead SHA rc=1 with exact file:line, arrow
# old→new line passes, baseline-hit silent unless TQ_DEAD_SHA_VERBOSE=1.
set -uo pipefail

# run_gate: cwd must be the fixture/real git repo root; caller sets the
# `baseline` path and the `files` array. Returns 1 on any new (non-baselined,
# non-arrow) dead SHA.
run_gate() {
	local f line linenum text tok
	local fail=0 new_hits=0 baselined_hits=0
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
				grep -qE "[0-9a-f]{7,40}(→|<-)[0-9a-f]{7,40}" <<<"$text" && continue
				if in_baseline "$tok"; then
					baselined_hits=$((baselined_hits + 1))
					[ "${TQ_DEAD_SHA_VERBOSE:-0}" = 1 ] && echo "STALE (baselined): $f:$linenum cites '$tok' (commit exists, unreachable from HEAD)"
				else
					echo "DEAD SHA: $f:$linenum cites '$tok' (commit exists, unreachable from HEAD) — re-resolve or add a fork record (old→new)"
					fail=1
					new_hits=$((new_hits + 1))
				fi
			done
		done < <(grep -nE '[0-9a-f]{7,40}' "$f")
	done

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

	echo "dead-sha self-test ok (clean rc=0, exact file:line on rc=1, arrow old→new passes, baselined silent unless TQ_DEAD_SHA_VERBOSE=1)"
	exit 0
fi

cd "$(dirname "$0")/.." || exit 1

baseline="scripts/dead-sha-baseline.txt"
files=(AGENTS.md TODO_LIST.md)
for f in docs/status/*.md; do [ -f "$f" ] && files+=("$f"); done

run_gate
