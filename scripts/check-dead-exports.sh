#!/usr/bin/env bash
# Dead-export audit: exported package-level identifiers with zero
# references outside their declaring package. Advisory report — exit 0
# unless --strict, because "kept on purpose" exports (documented domain
# API like task.CanTransitionTo) legitimately have zero in-repo importers.
#
# Methodology note (2026-09-10 re-derivation): matching is SUBSTRING, not
# word-boundary. `rg -w Sink` misses suffixed references like NewSink and
# NewCommandExecutor and undercounts — most of the arch-review's "~15 dead
# exports" were alive once matching was corrected. Substring matching can
# only overcount liveness (false survivors), never flag live symbols as
# dead, so the report is a conservative floor for the true dead set.
set -euo pipefail
cd "$(dirname "$0")/.."

strict=0
[ "${1:-}" = "--strict" ] && strict=1

# Declare files live in every internal package; usage search covers the
# whole repo (all modules import from each other + root app layer).
declare_rx='^(func|type|var|const) ([A-Z][A-Za-z0-9_]*)[( ]'
block_rx='^\t([A-Z][A-Za-z0-9_]*)[ =]'

dead=0
while IFS=$'\t' read -r file name; do
	[ -n "$name" ] || continue
	pkg_dir="$(dirname "$file")"
	# Substring search across all Go files, excluding the declaring
	# package's own directory. Exclude _test.go for declarations above
	# already, but usages IN tests of other packages count as importers.
	hits="$(rg -l --no-ignore -g '*.go' -g "!${pkg_dir}/**" -F "$name" . || true)"
	if [ -z "$hits" ]; then
		echo "$file: exported symbol with zero importers: $name"
		dead=$((dead + 1))
	fi
done < <(
	git ls-files 'internal/*.go' | grep -v '_test.go' | grep -v '_templ.go' |
		xargs awk -v d="$declare_rx" -v b="$block_rx" '
			/^(var|const) \(/ { inblock = 1; next }
			inblock && /^\)/ { inblock = 0; next }
			inblock && match($0, b) { print FILENAME "\t" substr($0, RSTART + 1, RLENGTH - 2); next }
			match($0, d) {
				s = substr($0, RSTART, RLENGTH)
				sub(/^(func|type|var|const) /, "", s)
				sub(/[( ]$/, "", s)
				print FILENAME "\t" s
			}
		'
)

echo
if [ "$dead" -eq 0 ]; then
	echo "dead-exports: clean — every exported internal symbol has at least one importer"
else
	echo "dead-exports: $dead symbol(s) with zero importers"
	[ "$strict" -eq 1 ] && exit 1
	echo "advisory — audit before pruning: internal/ layout is deliberate until the API stabilizes"
fi
