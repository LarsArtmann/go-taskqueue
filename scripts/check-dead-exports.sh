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
# shellcheck disable=SC2016
set -euo pipefail
cd "$(dirname "$0")/.."

strict=0
[ "${1:-}" = "--strict" ] && strict=1

# Declare files live in every internal package; usage search covers the
# whole repo (all modules import from each other + root app layer).
declare_rx='^(func|type|var|const) ([A-Z][A-Za-z0-9_]*)[( ]'
block_rx='^\t([A-Z][A-Za-z0-9_]*)[ =]'

dead=0
alive=0
structalive=0

# Signature table for the structural-liveness annotation: every exported
# func's full declaration text (accumulated across continuation lines up to
# the body's opening brace). A dead-flagged type that appears only as a
# return/parameter type of a LIVE exported func is annotated instead of
# costing manual triage (the 2026-09-21 false-positive class: RegistryEntry,
# SweepOutcome, CloseResult, ... all structurally alive behind Sweep/Close).
sigfile="$(mktemp)"
declfile="$(mktemp)"
deadfile="$(mktemp)"
trap 'rm -f "$sigfile" "$declfile" "$deadfile"' EXIT

funcsig_rx='^(func|type|var|const) ([A-Z][A-Za-z0-9_]*)[( ]'
blocksig_rx='^\t([A-Z][A-Za-z0-9_]*)[ =]'
git ls-files 'internal/*.go' | grep -v '_test.go' | grep -v '_templ.go' |
	xargs awk -v d="$funcsig_rx" -v b="$blocksig_rx" '
		/^(var|const) \(/ { inblock = 1; next }
		inblock && /^\)/ { inblock = 0; next }
		inblock && match($0, b) { print substr($0, RSTART + 1, RLENGTH - 2); next }
		match($0, d) {
			s = substr($0, RSTART, RLENGTH)
			sub(/^(func|type|var|const) /, "", s)
			sub(/[( ]$/, "", s)
			print s
		}
	' > "$declfile"

git ls-files 'internal/*.go' | grep -v '_test.go' | grep -v '_templ.go' |
	xargs awk '
		infunc {
			sig = sig $0 " "
			if (index($0, "{") > 0) { print FILENAME "\t" fname "\t" sig; infunc = 0 }
			next
		}
		/^func / {
			infunc = 1
			sig = $0 " "
			s = $0
			sub(/^func /, "", s)
			if (s ~ /^\(/) { sub(/^[^)]*\)[[:space:]]*/, "", s) }
			sub(/\(.*/, "", s)
			fname = s
			if (index($0, "{") > 0) { print FILENAME "\t" fname "\t" sig; infunc = 0 }
			next
		}
	' > "$sigfile"

while IFS=$'\t' read -r file name; do
	[ -n "$name" ] || continue
	pkg_dir="$(dirname "$file")"
	# Substring search across all Go files, excluding the declaring
	# package's own directory. Exclude _test.go for declarations above
	# already, but usages IN tests of other packages count as importers.
	hits="$(rg -l --no-ignore -g '*.go' -g "!${pkg_dir}/**" -F "$name" . || true)"
	if [ -z "$hits" ]; then
		printf '%s\t%s\n' "$file" "$name" >> "$deadfile"
	else
		alive=$((alive + 1))
	fi
done < "$declfile"

# Second pass: annotate the structurally-alive class. A flagged name that
# appears (word-delimited) in the signature of a DIFFERENT exported func of
# the same package — one that is itself NOT flagged — is alive behind that
# func's exported contract.
while IFS=$'\t' read -r file name; do
	note=""
	while IFS=$'\t' read -r sfile sname ssig; do
		[ "$(dirname "$sfile")" = "$(dirname "$file")" ] || continue
		[ "$sname" != "$name" ] || continue
		grep -Eq "(^|[^A-Za-z0-9_])${name}([^A-Za-z0-9_]|$)" <<<"$ssig" || continue
		if grep -qF "$(printf '%s\t%s' "$sfile" "$sname")" "$deadfile"; then
			continue
		fi
		note="structurally alive: return/parameter type of ${sname}()"
		break
	done < "$sigfile"
	if [ -n "$note" ]; then
		echo "$file: exported symbol with zero direct importers, $note: $name"
		structalive=$((structalive + 1))
	else
		echo "$file: exported symbol with zero importers: $name"
		dead=$((dead + 1))
	fi
done < "$deadfile"

echo
echo "dead-exports summary: $alive symbols ok, $dead dead, $structalive structurally alive (advisory)"
if [ "$dead" -eq 0 ]; then
	echo "dead-exports: clean — every exported internal symbol has at least one importer"
else
	echo "dead-exports: $dead symbol(s) with zero importers"
	[ "$strict" -eq 1 ] && exit 1
	echo "advisory — audit before pruning: internal/ layout is deliberate until the API stabilizes"
fi
