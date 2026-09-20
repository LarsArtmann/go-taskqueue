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
set -uo pipefail
cd "$(dirname "$0")/.." || exit 1

baseline="scripts/dead-sha-baseline.txt"
files=(AGENTS.md TODO_LIST.md)
for f in docs/status/*.md; do [ -f "$f" ] && files+=("$f"); done

in_baseline() {
	local tok=$1 line
	[ -f "$baseline" ] || return 1
	while IFS= read -r line; do
		[[ "$line" =~ ^# ]] && continue
		[ -z "$line" ] && continue
		[ "$line" = "$tok" ] && return 0
	done <"$baseline"
	return 1
}

fail=0
new_hits=0
baselined_hits=0
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
exit "$fail"
