#!/usr/bin/env bash
# Ghost-reference check: repo paths cited in the living docs must exist.
# Doc drift caught here dies in PRs, not in audits. TODO_LIST.md and
# docs/planning/ are exempt on purpose — they name paths that are yet to be.
set -uo pipefail
cd "$(dirname "$0")/.."

docs=(README.md AGENTS.md CONTRIBUTING.md CHANGELOG.md FEATURES.md ROADMAP.md)

# Tokens that look like repo paths but are not. One regex per line; add only
# verified false positives, with the doc that produced them as a comment.
allow=(
	'^go-taskqueue/internal/' # module import path in code samples (README)
)

fail=0
for f in "${docs[@]}"; do
	[ -f "$f" ] || continue
	while IFS= read -r ref; do
		[ -n "$ref" ] || continue
		skip=0
		for pat in "${allow[@]}"; do
			[[ "$ref" =~ $pat ]] && { skip=1; break; }
		done
		[ "$skip" = 1 ] && continue
		if [ ! -e "$ref" ]; then
			echo "MISSING: $f cites '$ref' (path does not exist)"
			fail=1
		fi
	done < <(grep -oE '`[A-Za-z0-9][A-Za-z0-9_-]*(/[A-Za-z0-9._-]+)+`' "$f" | tr -d '`' | sort -u)
done

if [ "$fail" = 0 ]; then
	echo "doc refs ok"
fi
exit "$fail"
