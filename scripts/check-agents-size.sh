#!/usr/bin/env bash
# AGENTS.md byte-budget gate (row 425): trips at gate time with a readable
# message instead of as a cmd/tq TestAgentsDocSizeGuard failure two windows
# later. Mirrors agentsDocMaxBytes in cmd/tq/facts_json_test.go (16,900 B
# since the 2026-10-07 fifth conscious reset; the reset protocol lives in
# that test's comment — CHANGE BOTH TWINS TOGETHER: the 2026-10-05 ci-local
# red was exactly this drift) and prints the top sections by size
# so a prune is a 2-minute targeted fix (byte semantics via LC_ALL=C).
#
# DOCTRINE (hit twice on 2026-10-07): formatter table padding (dprint-style
# column alignment adding ~1.4KB of spaces) is PRUNED via
# `git restore --source=<pre-padding-commit> AGENTS.md` after confirming
# with a whitespace-normalized diff that no real content changed — padding
# is NEVER a reason to reset the budget.
set -euo pipefail
cd "$(dirname "$0")/.."

file="${1:-AGENTS.md}"
budget="${AGENTS_BUDGET_BYTES:-16900}"

if [ ! -f "$file" ]; then
	echo "FAIL: $file not found (cwd: $(pwd))" >&2
	exit 1
fi

size=$(wc -c <"$file")
if [ "$size" -le "$budget" ]; then
	echo "agents-size OK: $file is $size/${budget} B"
	exit 0
fi

echo "FAIL: $file is $size B (budget ${budget} B, over by $((size - budget)) B) — prune the file or consciously reset the budget (AGENTS.md size guard, cmd/tq facts_json_test.go agentsDocMaxBytes)" >&2
echo "Top sections by byte size — prune targets:" >&2
LC_ALL=C awk '
	BEGIN { name = "(preamble)" }
	/^## /{ print len "\t" name; name = $0; len = length($0) + 1; next }
	{ len += length($0) + 1 }
	END { print len "\t" name }
' "$file" | sort -rn | head -3 | awk -F'\t' '{printf "  %6d B  %s\n", $1, $2}' >&2
exit 1
