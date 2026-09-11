#!/usr/bin/env bash
# Changed-lines line-length gate (round-11 M59): every line ADDED to a *.go
# file since the base revision must be <= 120 columns — the same cap the
# golines formatter and the advisory lll linter use. Scoping to changed
# lines keeps the ~legacy long lines out of the gate (they are baseline;
# mass-rewriting them buries real signal) while stopping new ones cold.
#
# Base revision: $LINT_BASE, else $1, else HEAD~1 (same resolution order as
# scripts/lint-annotations.sh). Generated files (*_templ.go) are excluded —
# templ fmt owns their formatting.
set -euo pipefail
cd "$(dirname "$0")/.."

base="${LINT_BASE:-${1:-}}"
if [ -z "$base" ] || ! git rev-parse -q --verify "$base^{commit}" >/dev/null 2>&1; then
	base="HEAD~1"
fi
if ! git rev-parse -q --verify "$base^{commit}" >/dev/null 2>&1; then
	echo "lll-changed: no base revision available, skipping"
	exit 0
fi

violations="$(git diff --unified=0 "$base" -- '*.go' | awk '
/^diff --git/ { file = "" }
/^\+\+\+ b\// { file = substr($0, 7) }
/^@@/ {
	# @@ -a,b +c,d @@ — new-file side starts at c
	split($0, h, " ")
	split(h[3], r, ",")
	newline = substr(r[1], 2) + 0
}
/^\+\+\+|^---|^diff --git|^index |^new file|^deleted file|^@@/ { next }
/^\+/ {
	line = substr($0, 2)
	if (length(line) > 120 && file !~ /_templ\.go$/) {
		printf "%s:%d: %d chars\n", file, newline, length(line)
	}
	newline++
	next
}
/^ / { newline++ }
')"

if [ -z "$violations" ]; then
	echo "lll-changed: no added line exceeds 120 chars (base $(git rev-parse --short "$base"))"
	exit 0
fi

echo "$violations"
echo "lll-changed: FAIL — added lines over 120 chars (wrap them; golines owns formatting)"
exit 1
