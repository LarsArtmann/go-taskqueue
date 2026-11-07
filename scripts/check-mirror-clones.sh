#!/usr/bin/env bash
# Cross-backend mirror-clone gate (dedup ruling 2026-09-26: accepts ≈ 0).
#
# The ADR-0019 spike adapters (internal/queue/{sqlitev4,postgresv4})
# carried near-identical companion surfaces. The shared production surface
# lives in internal/queue/companion since the 2026-09-26 extraction window;
# the gate fails on any clone group spanning more than one backend directory
# so the mirror class can never silently regrow.
#
# TWO passes (2026-11-07 widening, 2026-09-27_15-29 §f2/§f17):
#   1. canonical — art-dupl's default actionability-filtered report (the
#      historical contract);
#   2. raw — the same invocation plus --show-suppressed, so groups the
#      actionability filter hides (the design-blessed adapter delegator
#      pairs, test-scaffolding prologs) are gate-visible instead of
#      silently passing. NEW mass fails from EITHER view; known-acceptable
#      pairs are pinned in scripts/mirror-baseline.txt with recorded
#      reasons.
#
# A group resolved since the baseline is reported for a deliberate regen
# (which shrinks the baseline). Idiom noise that never reaches either view
# is handled by art-dupl's own actionability filter and art-dupl:accept
# directives with recorded reasons.
#
# Flags:
#   --show-suppressed  also print every BASELINED group line-by-line (the
#                      hidden mass gets human triage instead of a silent
#                      count in the summary)
#   --self-test        fleet-convention pin: summary format + negative-path
#                      counter battery via the TQ_MIRROR_CANON_CMD /
#                      TQ_MIRROR_RAW_CMD hooks (fixture HTML emitters)
# Env:
#   MIRROR_CLONES_STRICT=0  advisory mode (default 1)
#   TQ_MIRROR_CANON_CMD / TQ_MIRROR_RAW_CMD  self-test hooks replacing the
#                      two art-dupl invocations (must emit the HTML report
#                      to stdout)
set -euo pipefail
cd "$(dirname "$0")/.."

self_abs="$(cd "$(dirname "$0")" && pwd)/$(basename "$0")"

strict="${MIRROR_CLONES_STRICT:-1}"
show_suppressed=0
mode="${1:-}"
if [ "$mode" = "--show-suppressed" ]; then
	show_suppressed=1
fi

baseline_rows() {
	grep -v -e '^#' -e '^$' scripts/mirror-baseline.txt
}

if [ "$mode" = "--self-test" ]; then
	# --self-test pins the end-of-run summary format on a REAL double pass
	# and runs a negative-path counter battery over fixtures. The battery
	# derives its fixture groups FROM scripts/mirror-baseline.txt itself, so
	# a parser/baseline format drift (identity strings stop matching) fails
	# here instead of silently turning every pinned pair "new" — the
	# 2026-09-27 blind-spot class: a gate that trusts filtered output
	# inherits the filter's blind spots.
	tmpd="$(mktemp -d)"
	trap 'rm -rf "$tmpd"' EXIT

	must_mention() {
		if grep -qF -- "$1" <<<"$2"; then
			echo "ok: $3"
		else
			echo "FAIL: $3 (output is missing '$1')"
			printf '%s\n' "$2"
			exit 1
		fi
	}

	empty="$tmpd/empty.html"
	printf '<html><body></body></html>' >"$empty"

	# Fixture emitter: prints one clone-group div per baseline row in the
	# exact shape the gate parses (vscode://file hrefs, repo-relative paths
	# + line numbers, data-category attribute).
	emit_fixture() {
		printf '<html><body>'
		local n=0 row cat paths p i
		while read -r row; do
			n=$((n + 1))
			cat="${row%% *}"
			paths="${row#* }"
			printf '<div class="clone-group" id="group-fx%d" data-category="%s">' "$n" "$cat"
			i=0
			for p in $paths; do
				i=$((i + 1))
				printf '<a href="vscode://file/%s:%d">x</a>' "$p" "$i"
			done
			printf '</div>'
		done < <(baseline_rows)
		printf '</body></html>'
	}

	# 1) Hidden-new mass fails strict: the raw view carries a group the
	#    canonical view never sees and the baseline does not pin.
	newfix="$tmpd/new.html"
	{
		printf '<html><body>'
		printf '<div class="clone-group" id="group-new1" data-category="assignment">'
		printf '<a href="vscode://file/internal/queue/postgresv4/ghost.go:1">x</a>'
		printf '<a href="vscode://file/internal/queue/sqlitev4/ghost.go:1">x</a>'
		printf '</div></body></html>'
	} >"$newfix"
	strict_out="$(MIRROR_CLONES_STRICT=1 \
		TQ_MIRROR_CANON_CMD="cat $empty" \
		TQ_MIRROR_RAW_CMD="cat $newfix" \
		"$self_abs" 2>&1)" && rc=0 || rc=$?
	if [ "$rc" -ne 1 ]; then
		echo "FAIL: hidden-new group must exit 1 under strict (got rc=$rc)"
		printf '%s\n' "$strict_out"
		exit 1
	fi
	must_mention "NEW HIDDEN MIRROR" "$strict_out" "hidden-new group is tagged NEW HIDDEN MIRROR"
	must_mention "1 new (1 hidden)" "$strict_out" "summary counts the hidden-new group"
	echo "ok: hidden-new group fails strict"

	# 2) Advisory mode downgrades the same fixture to rc=0.
	adv_out="$(MIRROR_CLONES_STRICT=0 \
		TQ_MIRROR_CANON_CMD="cat $empty" \
		TQ_MIRROR_RAW_CMD="cat $newfix" \
		"$self_abs" 2>&1)" && rc=0 || rc=$?
	if [ "$rc" -ne 0 ]; then
		echo "FAIL: MIRROR_CLONES_STRICT=0 must stay advisory rc=0 (got rc=$rc)"
		printf '%s\n' "$adv_out"
		exit 1
	fi
	must_mention "advisory (MIRROR_CLONES_STRICT=0)" "$adv_out" "advisory downgrade line printed"
	echo "ok: strict=0 stays advisory"

	# 3) Baseline rows match the parser: every pinned row shows up as
	#    BASELINED (not NEW), rc 0. The fixture is DERIVED from the baseline
	#    file, so a format drift fails here.
	fix="$tmpd/baseline.html"
	emit_fixture >"$fix"
	base_out="$(TQ_MIRROR_CANON_CMD="cat $empty" \
		TQ_MIRROR_RAW_CMD="cat $fix" \
		"$self_abs" 2>&1)" && rc=0 || rc=$?
	if [ "$rc" -ne 0 ]; then
		echo "FAIL: baseline-pinned fixture must pass (got rc=$rc) — parser/baseline identity drift"
		printf '%s\n' "$base_out"
		exit 1
	fi
	rows="$(baseline_rows | wc -l)"
	if [ "$rows" -eq 0 ]; then
		echo "FAIL: mirror-baseline.txt has zero rows — the widened gate requires the known-acceptable pairs pinned with recorded reasons (an empty baseline re-opens the blind spot this gate exists to close)"
		exit 1
	fi
	must_mention "$rows baselined" "$base_out" "summary counts every pinned row as baselined"
	if grep -q 'NEW' <<<"$base_out"; then
		echo "FAIL: baseline-pinned fixture tagged NEW — identity strings drifted"
		printf '%s\n' "$base_out"
		exit 1
	fi
	echo "ok: baseline rows match the parser (identity pin)"

	# 4) --show-suppressed prints the baselined rows line-by-line; the
	#    default pass must NOT.
	sup_out="$(TQ_MIRROR_CANON_CMD="cat $empty" \
		TQ_MIRROR_RAW_CMD="cat $fix" \
		"$self_abs" --show-suppressed 2>&1)" && rc=0 || rc=$?
	if [ "$rc" -ne 0 ]; then
		echo "FAIL: --show-suppressed pass must stay rc=0 (got rc=$rc)"
		printf '%s\n' "$sup_out"
		exit 1
	fi
	sup_lines="$(grep -c 'BASELINED \[hidden\]' <<<"$sup_out")"
	if [ "$sup_lines" -ne "$rows" ]; then
		echo "FAIL: --show-suppressed must list all $rows baselined rows line-by-line (got $sup_lines)"
		printf '%s\n' "$sup_out"
		exit 1
	fi
	if grep -q 'BASELINED' <<<"$base_out"; then
		echo "FAIL: default pass must NOT print baselined rows line-by-line (only the summary)"
		printf '%s\n' "$base_out"
		exit 1
	fi
	echo "ok: --show-suppressed lists the hidden baselined rows"

	# 5) Baseline rows unseen by either view are reported RESOLVED.
	first="$(baseline_rows | head -1)"
	resfix="$tmpd/one-row.html"
	{
		printf '<html><body>'
		printf '<div class="clone-group" id="group-fx1" data-category="%s">' "${first%% *}"
		for p in ${first#* }; do
			printf '<a href="vscode://file/%s:1">x</a>' "$p"
		done
		printf '</div></body></html>'
	} >"$resfix"
	res_out="$(TQ_MIRROR_CANON_CMD="cat $empty" \
		TQ_MIRROR_RAW_CMD="cat $resfix" \
		"$self_abs" 2>&1)" && rc=0 || rc=$?
	if [ "$rc" -ne 0 ]; then
		echo "FAIL: resolved-row pass must stay rc=0 (got rc=$rc)"
		printf '%s\n' "$res_out"
		exit 1
	fi
	must_mention "RESOLVED (regen baseline to shrink)" "$res_out" "unseen baseline rows are reported RESOLVED"
	must_mention "1 resolved" "$res_out" "summary counts the resolved row"
	echo "ok: resolved rows reported"

	# 6) Zero groups keeps the pinned zero line.
	zero_out="$(TQ_MIRROR_CANON_CMD="cat $empty" \
		TQ_MIRROR_RAW_CMD="cat $empty" \
		"$self_abs" 2>&1)" && rc=0 || rc=$?
	must_mention "mirror-clones: 0 cross-backend clone groups" "$zero_out" "zero line pinned"

	# 7) REAL double pass: the shipped summary format is pinned by regex
	#    (counters drift with the tree; the FORMAT is the contract).
	real_out="$("$self_abs" 2>&1)" && rc=0 || rc=$?
	if [ "$rc" -ne 0 ]; then
		echo "FAIL: real mirror-gate pass must be green at HEAD (got rc=$rc)"
		printf '%s\n' "$real_out"
		exit 1
	fi
	if grep -Eq '^mirror-clones summary: [0-9]+ cross-backend group\(s\), [0-9]+ new \([0-9]+ hidden\), [0-9]+ baselined, [0-9]+ resolved$' <<<"$real_out"; then
		echo "ok: real-run summary matches the pinned format"
	else
		echo "FAIL: real-run summary missing or malformed"
		printf '%s\n' "$real_out"
		exit 1
	fi

	echo "mirror-clones self-test ok (summary format pin + negative counter battery)"
	exit 0
fi

canon_tmp="$(mktemp /tmp/mirror-canon.XXXXXX.html)"
raw_tmp="$(mktemp /tmp/mirror-raw.XXXXXX.html)"
trap 'rm -f "$canon_tmp" "$raw_tmp"' EXIT

# The canonical invocation lives in scripts/art-dupl.json (-c config,
# 2026-10-05 §f10: threshold 3, total-tokens sort, type-aware, rich-text,
# explain) so ad-hoc runs match the gate flags by default. STDOUT is a
# single-line HTML document. The raw pass adds --show-suppressed so the
# actionability-hidden groups are gate-visible.
run_pass() {
	local out="$1" hook="$2"
	shift 2
	if [ -n "$hook" ]; then
		eval "$hook" >"$out"
	else
		if ! art-dupl -c scripts/art-dupl.json "$@" --html >"$out" 2>/dev/null; then
			echo "FAIL: art-dupl could not produce a report" >&2
			exit 1
		fi
	fi
}
run_pass "$canon_tmp" "${TQ_MIRROR_CANON_CMD:-}"
run_pass "$raw_tmp" "${TQ_MIRROR_RAW_CMD:-}" --show-suppressed

rc=0
python3 - "$canon_tmp" "$raw_tmp" "$strict" "$show_suppressed" <<'PYEOF' || rc=$?
import re
import sys

canon_path, raw_path, strict, show_suppressed = sys.argv[1:5]
patterns = [
    r"internal/queue/sqlite$",
    r"internal/queue/sqlite/",
    r"internal/queue/sqlitev4",
    r"internal/queue/postgres$",
    r"internal/queue/postgres/",
    r"internal/queue/postgresv4",
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


def parse(path):
    raw = open(path).read()
    groups = re.split(r'<div class="clone-group" id="group-', raw)[1:]
    out = {}
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
            # Identity strips line numbers: groups drift as files are
            # edited, the mirrored FILE PAIR is the stable fact.
            paths = sorted({h.split(":")[0] for h in hits})
            ident = cat + " " + " ".join(paths)
            out[ident] = paths
    return out


canon = parse(canon_path)
rawv = parse(raw_path)
union = dict(rawv)
union.update(canon)
union_ids = set(union)
new_ids = union_ids - baseline
hidden_new_ids = {i for i in new_ids if i not in canon}
baselined_ids = sorted(union_ids & baseline)
resolved = sorted(baseline - union_ids)

for ident in sorted(new_ids):
    tag = "NEW MIRROR" if ident in canon else "NEW HIDDEN MIRROR"
    cat = ident.split(" ")[0]
    print(tag + " [" + cat + "]: " + ", ".join(union[ident]))

if show_suppressed == "1":
    for ident in baselined_ids:
        cat = ident.split(" ")[0]
        marker = " [hidden]" if ident not in canon else ""
        print("BASELINED" + marker + " [" + cat + "]: " + ", ".join(union[ident]))

for row in resolved:
    print("RESOLVED (regen baseline to shrink): " + row)

if not union_ids:
    print("mirror-clones: 0 cross-backend clone groups")
    sys.exit(0)

summary = (
    str(len(union_ids))
    + " cross-backend group(s), "
    + str(len(new_ids))
    + " new ("
    + str(len(hidden_new_ids))
    + " hidden), "
    + str(len(baselined_ids))
    + " baselined, "
    + str(len(resolved))
    + " resolved"
)
print("mirror-clones summary: " + summary)

if new_ids and strict == "1":
    print(
        "FAIL: new cross-backend mirror group(s) ("
        + str(len(hidden_new_ids))
        + " visible only in the --show-suppressed view) — home the surface in internal/queue/companion, or amend scripts/mirror-baseline.txt with a recorded reason",
        file=sys.stderr,
    )
    sys.exit(1)

if new_ids and strict != "1":
    print("mirror-clones: advisory (MIRROR_CLONES_STRICT=0) — new mirror group(s) above")

sys.exit(0)
PYEOF

exit "$rc"
