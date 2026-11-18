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

# --self-test re-invokes THIS script (absolute path captured before any cd)
# and pins: the derived-budget fixture path (Go const → gate budget), the
# red path with the over-by delta, the AGENTS_BUDGET_BYTES override, the
# loud parse-failure (exit 2), the underscore-literal const shape, and the
# real-file summary format (budget pinned to the live size so the green
# shape is deterministic whatever AGENTS.md weighs).
if [ "${1:-}" = "--self-test" ]; then
	self_abs="$(cd "$(dirname "$0")" && pwd)/$(basename "$0")"
	repo_root="$(cd "$(dirname "$self_abs")/.." && pwd)"
	. "$(dirname "$self_abs")/lib/gate-fixture.sh"
	tmp="$(gate_fixture_tree)"
	trap 'rm -rf "$tmp"' EXIT
	gate_fixture_write "$tmp" "const.go" 'const agentsDocMaxBytes = 10'
	gate_fixture_write "$tmp" "const_underscore.go" 'const agentsDocMaxBytes = 1_2'
	gate_fixture_write "$tmp" "doc.md" '0123456789A
'
	gate_fixture_expect "derived budget trips with over-by delta" 1 "budget 10 B, over by 2 B" TQ_AGENTS_CONST_FILE="$tmp/const.go" "$self_abs" "$tmp/doc.md"
	gate_fixture_expect "underscore-literal const parses" 0 "12/12 B (headroom 0 B)" TQ_AGENTS_CONST_FILE="$tmp/const_underscore.go" "$self_abs" "$tmp/doc.md"
	gate_fixture_expect "AGENTS_BUDGET_BYTES override beats derived const + headroom delta" 0 "12/20 B (headroom 8 B)" TQ_AGENTS_CONST_FILE="$tmp/const.go" AGENTS_BUDGET_BYTES=20 "$self_abs" "$tmp/doc.md"
	gate_fixture_expect "missing const fails loud at exit 2" 2 "cannot parse agentsDocMaxBytes" TQ_AGENTS_CONST_FILE="$tmp/absent.go" "$self_abs" "$tmp/doc.md"
	live_size="$(wc -c <"$repo_root/AGENTS.md")"
	gate_fixture_expect "real-file OK summary format (live-size budget)" 0 "agents-size OK: $repo_root/AGENTS.md is $live_size/$live_size B (headroom 0 B)" AGENTS_BUDGET_BYTES="$live_size" "$self_abs" "$repo_root/AGENTS.md"
	gate_fixture_summary "agents-size self-test" "derived-budget pin + negative battery"
	exit 0
fi

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
	echo "agents-size OK: $file is $size/${budget} B (headroom $((budget - size)) B)"
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
