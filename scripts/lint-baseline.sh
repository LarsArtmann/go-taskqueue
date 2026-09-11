#!/usr/bin/env bash
# Regenerate the advisory-lint baseline (.golangci-baseline.txt): one line
# per sub-module (root included) and linter with the current finding count.
# The baseline is checked in so the pre-push gate can distinguish the
# documented advisory sea (AGENTS.md "golangci-lint is advisory") from
# unbounded growth. Regenerate deliberately after policy changes:
#   scripts/lint-baseline.sh
# Takes ~3 minutes (full root + sub-module loop).
set -euo pipefail
cd "$(dirname "$0")/.."
export GOEXPERIMENT=jsonv2

out="$(mktemp "${TMPDIR:-/tmp}/tq-baseline.XXXXXX")"
trap 'rm -f "$out"' EXIT

record() {
	local module="$1" lint_out="$2"
	printf '%s\n' "$lint_out" | awk -v mod="$module" '/^\* /{gsub(/^\* /,""); gsub(/^[ ]+/,""); print mod "\t" $1 "\t" $2}' >>"$out"
}

echo "== root" >&2
record root "$(golangci-lint run ./... 2>&1 || true)"
for m in $(find internal -name go.mod | sed 's|/go.mod$||' | sort); do
	echo "== $m" >&2
	record "$m" "$(cd "$m" && GOWORK=off golangci-lint run ./... 2>&1 || true)"
done

sort -u "$out" -o .golangci-baseline.txt
total="$(awk -F'\t' '{s+=$3} END{print s+0}' .golangci-baseline.txt)"
echo "baseline written: $(grep -c . .golangci-baseline.txt) module/linter rows, $total findings total"
