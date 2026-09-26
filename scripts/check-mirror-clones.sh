#!/usr/bin/env bash
# Cross-backend mirror-clone gate (dedup ruling 2026-09-26: accepts ≈ 0).
#
# The ADR-0019 spike adapters (internal/queue/{sqlitev4,postgresv4,cqrsqlite})
# carried near-identical companion surfaces. The shared production surface
# lives in internal/queue/companion since the 2026-09-26 extraction window;
# the gate fails on any clone group spanning more than one backend directory
# so the mirror class can never silently regrow.
#
# Remaining KNOWN mirrors are pinned in scripts/mirror-baseline.txt (the
# three mirrored conformance suites — their consolidation into
# companion/conform is tracked in TODO_LIST). Strict mode (default ON for
# ci-local) fails on any group NOT in the baseline; groups RESOLVED since
# the baseline are reported and shrink the baseline on the next deliberate
# regen. Idiom noise (defer/flag-parse prologs, interface asserts,
# shared-seam call pairs) is handled by art-dupl's own actionability filter
# and art-dupl:accept directives with recorded reasons.
set -euo pipefail
cd "$(dirname "$0")/.."

strict="${MIRROR_CLONES_STRICT:-1}"

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
baseline_path = "scripts/mirror-baseline.txt"

baseline = set()
try:
    with open(baseline_path) as fh:
        for line in fh:
            line = line.strip()
            if line and not line.startswith("#"):
                baseline.add(line)
except FileNotFoundError:
    pass  # no baseline yet: every mirror group is new

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
        cat = cat.group(1) if cat else "?"
        # Identity strips line numbers: groups drift as files are edited,
        # the mirrored FILE PAIR is the stable fact.
        paths = [h.split(":")[0] for h in hits]
        mirrors.append((cat, hits, cat + " " + " ".join(sorted(paths))))

seen = {m[2] for m in mirrors}
new_groups = [m for m in mirrors if m[2] not in baseline]
resolved = sorted(baseline - seen)

for cat, files, ident in mirrors:
    tag = "BASELINED" if ident in baseline else "NEW MIRROR"
    print(tag + " [" + cat + "]: " + ", ".join(files))

if resolved:
    for row in resolved:
        print("RESOLVED (regen baseline to shrink): " + row)

if not mirrors:
    print("mirror-clones: 0 cross-backend clone groups")
    sys.exit(0)

summary = str(len(mirrors)) + " cross-backend group(s): " + str(len(new_groups)) + " new, " + str(len(mirrors) - len(new_groups)) + " baselined"
print("mirror-clones: " + summary)

if new_groups and strict == "1":
    print("FAIL: new cross-backend mirror group(s) — home the surface in internal/queue/companion, or amend scripts/mirror-baseline.txt with a recorded reason", file=sys.stderr)
    sys.exit(1)

sys.exit(0)
PYEOF

if [ "$rc" -ne 0 ] && [ "$strict" != "1" ]; then
	echo "mirror-clones: advisory (MIRROR_CLONES_STRICT=0) — new mirror group(s) above"
	rc=0
fi
exit "$rc"
