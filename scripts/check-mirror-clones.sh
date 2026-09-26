#!/usr/bin/env bash
# Cross-backend mirror-clone gate (dedup ruling 2026-09-26: accepts ≈ 0).
#
# The ADR-0019 spike adapters (internal/queue/{sqlitev4,postgresv4,cqrsqlite})
# carry near-identical companion surfaces. The ruling: no LIVING acceptances —
# the shared surface moves to internal/queue/companion (its S4-surviving home),
# and this gate fails on any clone group whose occurrences span more than one
# backend directory, so the mirror class can never silently regrow.
#
# Until the companion extraction lands, mirror groups are EXPECTED; the gate
# runs advisory (default) and prints them. The extraction window flips it to
# strict via MIRROR_CLONES_STRICT=1 in ci-local.
#
# Idiom noise (defer/flag-parse prologs, mutex pairs, shared-seam call pairs)
# stays WITHIN one package and never spans backends — out of scope by design.
set -euo pipefail
cd "$(dirname "$0")/.."

strict="${MIRROR_CLONES_STRICT:-0}"

tmp="$(mktemp /tmp/mirror-clones.XXXXXX.html)"
trap 'rm -f "$tmp"' EXIT

# The exact canonical invocation (dedup skill): type-aware, threshold 3,
# total-tokens sort. STDOUT is a single-line HTML document.
if ! art-dupl --sort total-tokens -t 3 --type-aware --rich-text --explain --html >"$tmp" 2>/dev/null; then
	echo "FAIL: art-dupl could not produce a report" >&2
	exit 1
fi

rc=0
python3 - "$tmp" "$strict" <<'PYEOF' || rc=$?
import re
import sys

path, strict = sys.argv[1], sys.argv[2]
patterns = [
    r"internal/queue/sqlite$",
    r"internal/queue/sqlite/",
    r"internal/queue/sqlitev4",
    r"internal/queue/postgres$",
    r"internal/queue/postgres/",
    r"internal/queue/postgresv4",
    r"internal/queue/cqrsqlite",
    r"queue/sqlite/",
    r"queue/postgres/",
]
backend_res = [re.compile(p) for p in patterns]

raw = open(path).read()
groups = re.split(r'<div class="clone-group" id="group-', raw)[1:]

mirrors = []
for g in groups:
    files = re.findall(r'href="vscode://file/([^"]+)"', g)
    hits = sorted({f for f in files if any(b.search(f) for b in backend_res)})
    if len(hits) < 2:
        continue
    roots = set()
    for h in hits:
        parts = h.split("/")
        roots.add("/".join(parts[:3]) if h.startswith("internal/") else parts[0])
    if len(roots) > 1:
        cat = re.search(r'data-category="([^"]*)"', g)
        mirrors.append((cat.group(1) if cat else "?", hits))

if not mirrors:
    print("mirror-clones: 0 cross-backend clone groups")
    sys.exit(0)

for cat, files in mirrors:
    print("MIRROR [" + cat + "]: " + ", ".join(files))
print("mirror-clones: " + str(len(mirrors)) + " cross-backend clone group(s)")
sys.exit(1 if strict == "1" else 0)
PYEOF

if [ "$rc" -ne 0 ] && [ "$strict" != "1" ]; then
	echo "mirror-clones: advisory (MIRROR_CLONES_STRICT unset) — companion extraction tracked in TODO_LIST"
	rc=0
fi
exit "$rc"
