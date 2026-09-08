#!/usr/bin/env bash
# Status-index check: every docs/status/*.md report must be indexed in
# docs/status/README.md. 5 of 2026-09-08's 8 reports went unindexed within a
# single day (21:40 report §d6) — the index is the only discovery surface for
# point-in-time reports, so an unindexed report is effectively lost.
set -uo pipefail
cd "$(dirname "$0")/.."

index="docs/status/README.md"
[ -f "$index" ] || {
	echo "MISSING: $index does not exist"
	exit 1
}

fail=0
while IFS= read -r report; do
	name="$(basename "$report")"
	if ! grep -qF "$name" "$index"; then
		echo "UNINDEXED: $report (add a row to $index)"
		fail=1
	fi
done < <(find docs/status -maxdepth 1 -name '*.md' ! -name 'README.md' | sort)

if [ "$fail" = 0 ]; then
	echo "status index ok"
fi
exit "$fail"
