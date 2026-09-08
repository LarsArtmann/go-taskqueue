#!/usr/bin/env bash
# 60s fuzz campaign rotation over every committed fuzz target; syncs
# coverage-interesting inputs into each target's committed seed corpus under
# <pkg>/testdata/fuzz/<FuzzTarget>. Every committed seed also runs as a
# regular test case on each `go test`, so corpus growth does not depend on
# session memory.
#
# Campaigns (pkg|target), rotated nightly so each target gets full fuzztime:
#   internal/harvest|FuzzParseRepo           — TODO_LIST.md parsing
#   internal/executor|FuzzExtractResultPayload — agent TQ_RESULT output parsing
#
# The nightly CI job (.github/workflows/fuzz.yml) runs this and commits any
# new seeds back to master; locally, the auto-commit daemon picks the files
# up (or commit them yourself).
#
# Usage: scripts/fuzz/nightly.sh [fuzztime] [target]   (default 60s, all; also FUZZTIME=)
# Env:   MAX_SEEDS cap on committed corpus size per target (default 1000).
set -euo pipefail

cd "$(dirname "$0")/../.."

fuzztime="${1:-${FUZZTIME:-60s}}"
only="${2:-}"
max_seeds="${MAX_SEEDS:-1000}"

campaigns=(
	"internal/harvest|FuzzParseRepo"
	"internal/executor|FuzzExtractResultPayload"
)

cache="$(mktemp -d)"
trap 'rm -rf "$cache"' EXIT

for campaign in "${campaigns[@]}"; do
	pkg_dir="${campaign%%|*}"
	target="${campaign#*|}"
	seed_dir="$pkg_dir/testdata/fuzz/$target"

	if [ -n "$only" ] && [ "$only" != "$target" ]; then
		continue
	fi

	# Hermetic GOCACHE: the ambient cache can be a shared mount where the fuzz
	# corpus never lands, and a private cache keeps the campaign independent of
	# whatever else is building concurrently (agent pools run go test here).
	echo "fuzzing $target in $pkg_dir for $fuzztime (private GOCACHE=$cache)"
	if ! GOCACHE="$cache" go test "./$pkg_dir" -run '^$' -fuzz "^$target\$" -fuzztime "$fuzztime"; then
		echo "FUZZ FAILURE: go test wrote the crash input to $seed_dir" >&2
		echo "Reproduce with: go test ./$pkg_dir -run '^$target\$'" >&2
		exit 1
	fi

	# Interesting inputs land in the cache only when the campaign finishes.
	corpus_dir="$cache/fuzz/$(go list -f '{{.ImportPath}}' "./$pkg_dir")/$target"
	mkdir -p "$seed_dir"

	seeds=$(find "$seed_dir" -type f | wc -l)
	added=0
	for input in "$corpus_dir"/*; do
		[ -f "$input" ] || continue
		if [ $((seeds + added)) -ge "$max_seeds" ]; then
			echo "seed cap ($max_seeds) reached for $target; pruning needed before more sync" >&2
			break
		fi
		# Corpus file names are content hashes: same name means same input.
		if [ ! -e "$seed_dir/$(basename "$input")" ]; then
			cp "$input" "$seed_dir/"
			added=$((added + 1))
		fi
	done

	echo "$target seed corpus: $((seeds + added)) files (+$added new)"
	if [ "$added" -gt 0 ]; then
		echo "new seeds staged in $seed_dir — commit them so future campaigns start richer"
	fi
done
