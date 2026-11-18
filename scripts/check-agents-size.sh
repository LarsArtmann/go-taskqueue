#!/usr/bin/env bash
# AGENTS.md byte-budget gate (agentsDocMaxBytes twin): trips at gate time with a readable
# message instead of as a cmd/tq TestAgentsDocSizeGuard failure two windows
# later. The budget DERIVES from agentsDocMaxBytes in cmd/tq/facts_json_test.go
# (single source of truth — the generator form of the budget-twin row: the
# 2026-10-05 ci-local red and the 2026-11-03 re-sync were two sightings of
# the same hand-sync pair this kills). A failed parse FAILS LOUDLY (exit 2)
# instead of silently guessing; AGENTS_BUDGET_BYTES overrides for
# negative-path tests. Reset history + reset RULES live in the const's
# comment in cmd/tq/facts_json_test.go.
#
# DOCTRINE (hit twice on 2026-10-07): formatter table padding (dprint-style
# column alignment adding ~1.4KB of spaces) is PRUNED via
# `git restore --source=<pre-padding-commit> AGENTS.md` after confirming
# with a whitespace-normalized diff that no real content changed — padding
# is NEVER a reason to reset the budget. Prints the top sections by size
# so a prune is a 2-minute targeted fix (byte semantics via LC_ALL=C).
set -euo pipefail
cd "$(dirname "$0")/.."

file="${1:-AGENTS.md}"
const_src="${TQ_AGENTS_CONST_FILE:-cmd/tq/facts_json_test.go}"
derived="$(sed -n 's/^const agentsDocMaxBytes = \([0-9_]*\).*$/\1/p' "$const_src" 2>/dev/null | tr -d '_')" || true
case "$derived" in
'' | *[!0-9]*)
	echo "FAIL: cannot parse agentsDocMaxBytes from $const_src — the twin budget source moved; fix this script's sed pattern (the Go const is the single source of truth; this gate must never carry its own budget)" >&2
	exit 2
	;;
esac
budget="${AGENTS_BUDGET_BYTES:-$derived}"

if [ ! -f "$file" ]; then
	echo "FAIL: $file not found (cwd: $(pwd))" >&2
	exit 1
fi

size=$(wc -c <"$file")
if [ "$size" -le "$budget" ]; then
	echo "agents-size OK: $file is $size/${budget} B"
	exit 0
fi

echo "FAIL: $file is $size B (budget ${budget} B, over by $((size - budget)) B) — prune the file or consciously reset agentsDocMaxBytes in $const_src (this script derives its budget from that const, so one reset re-syncs both twins)" >&2
echo "Top sections by byte size — prune targets:" >&2
LC_ALL=C awk '
	BEGIN { name = "(preamble)" }
	/^## /{ print len "\t" name; name = $0; len = length($0) + 1; next }
	{ len += length($0) + 1 }
	END { print len "\t" name }
' "$file" | sort -rn | head -3 | awk -F'\t' '{printf "  %6d B  %s\n", $1, $2}' >&2
exit 1
