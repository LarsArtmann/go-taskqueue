#!/usr/bin/env bash
# Research-note shape gate: every template-era note in docs/research/ must carry
# the two sections whose ABSENCE is what makes a research note unverifiable —
# a source-verified citation table (what the source actually says, per row) and
# a Verification trail (how the evidence was acquired, and where it was
# re-read).
#
# Why a gate and not a convention: the 2026-11-01 OKF note was written from
# `agentic_fetch` summaries while its Verification trail claimed direct reads,
# and four of its line cites were off by one or two. Both failures are SHAPE
# failures — a note with a citations table and a real acquisition trail is much
# harder to fake than a prose summary. The template
# (docs/references/research-note-template.md) states the obligation; this gate
# makes it observable.
#
# Checks per note, in order:
#   1. `## Source-verified citations` exists (FAIL when absent).
#   2. that section contains a markdown table (a `| --- |` separator row) —
#      a heading with prose under it is not a citation table (FAIL).
#   3. `## Verification trail` exists and is non-empty (FAIL).
#   4. `**stale_after:** <date>` is present (WARN — counted, not fatal, so the
#      notes written before the freshness convention keep passing while the
#      gap stays visible).
#
# Grandfathering: scripts/research-note-shape-grandfathered.txt lists notes that
# predate the template, one basename per line with a `#` reason. They are
# skipped entirely (a shape retrofit of a 2026-10-03 competitive analysis is
# archaeology, not maintenance).
#
# --self-test pins every branch against a /tmp fixture tree (fleet countable-gate
# convention): a conforming note rc=0 with the pinned summary, each missing
# section rc=1 naming the note and the section, a heading without a table rc=1,
# an empty trail rc=1, a grandfathered note skipped, and stale_after-missing
# counted as a WARN without failing.
set -uo pipefail

FAKE_DIR="${TQ_RESEARCH_DIR:-}"

check_notes() { # check_notes <dir> <grandfather-file>
	local dir=$1 grand=$2
	local fail=0 notes=0 conforming=0 skipped=0 warns=0

	local f base
	for f in "$dir"/*.md; do
		[ -f "$f" ] || continue
		base="$(basename "$f")"
		[ "$base" = "README.md" ] && continue

		if [ -f "$grand" ] && grep -qxF -- "$base" <(grep -v '^#' "$grand" | sed '/^$/d'); then
			skipped=$((skipped + 1))
			continue
		fi

		notes=$((notes + 1))
		local before=$fail

		# 1 + 2: the citations heading, and a real table under it.
		if ! grep -qE '^## +Source-verified citations' "$f"; then
			echo "MISSING SECTION: $base has no '## Source-verified citations' heading"
			fail=$((fail + 1))
		else
			local body
			body=$(awk '
				/^## +Source-verified citations/ { inside = 1; next }
				/^## / { inside = 0 }
				inside { print }
			' "$f")
			if ! grep -qE '^\|[[:space:]]*:?-+:?[[:space:]]*\|' <<<"$body"; then
				echo "NO CITATION TABLE: $base's citations section has no markdown table (need a '| --- |' separator row)"
				fail=$((fail + 1))
			fi
		fi

		# 3: the verification trail, non-empty.
		if ! grep -qE '^## +Verification trail' "$f"; then
			echo "MISSING SECTION: $base has no '## Verification trail' heading"
			fail=$((fail + 1))
		else
			local trail
			trail=$(awk '
				/^## +Verification trail/ { inside = 1; next }
				/^## / { inside = 0 }
				inside { print }
			' "$f" | tr -d '[:space:]')
			if [ -z "$trail" ]; then
				echo "EMPTY TRAIL: $base's Verification trail section is empty"
				fail=$((fail + 1))
			fi
		fi

		# 4: the freshness convention (advisory).
		if ! grep -qE '^\*\*stale_after:\*\*' "$f"; then
			echo "WARN: $base carries no '**stale_after:** <absolute date>' line (freshness convention)"
			warns=$((warns + 1))
		fi

		[ "$fail" = "$before" ] && conforming=$((conforming + 1))
	done

	echo "research-note shape: $conforming/$notes conforming, $skipped grandfathered, $warns stale_after warn(s), $fail finding(s)"

	[ "$fail" = 0 ] && echo "research-note shapes ok"

	[ "$fail" = 0 ]
}

if [ "${1:-}" = "--self-test" ]; then
	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' EXIT
	mkdir -p "$tmp/notes"
	local_dir="$tmp/notes"

	# pins: the positive control (full shape: heading + citations table + trail)
	cat >"$local_dir/good.md" <<'EOF'
# Thing lessons for go-taskqueue

## Source-verified citations

| # | Path | What it says |
| - | ---- | ------------ |
| 1 | a.go | says a thing |

## Verification trail

- downloaded the sources directly; no intermediary summary
EOF

	# pins: the missing-citations-heading negative path
	cat >"$local_dir/no-heading.md" <<'EOF'
# Thing lessons

## Verification trail

- read it
EOF

	# pins: the prose-instead-of-table negative path
	cat >"$local_dir/no-table.md" <<'EOF'
# Thing lessons

## Source-verified citations

Prose instead of a table.

## Verification trail

- read it
EOF

	# pins: the empty-verification-trail negative path
	cat >"$local_dir/empty-trail.md" <<'EOF'
# Thing lessons

## Source-verified citations

| # | Path |
| - | ---- |
| 1 | a.go |

## Verification trail

EOF

	# pins: the legacy-shape grandfathering path (listed in grand.txt)
	cat >"$local_dir/legacy.md" <<'EOF'
# An old analysis with a different shape
EOF

	printf '%s\n' "# grandfathered" "" "legacy.md" >"$tmp/grand.txt"

	out=$(check_notes "$local_dir" "$tmp/grand.txt") && rc=0 || rc=$?
	[ "$rc" = 1 ] || {
		echo "research-note self-test FAIL: broken fixture tree rc=$rc out: $out"
		exit 1
	}
	grep -q "research-note shape: 1/4 conforming, 1 grandfathered, 4 stale_after warn(s), 3 finding(s)" <<<"$out" || {
		echo "research-note self-test FAIL: summary shape drifted: $out"
		exit 1
	}
	grep -q "MISSING SECTION: no-heading.md has no '## Source-verified citations' heading" <<<"$out" || {
		echo "research-note self-test FAIL: missing-heading branch: $out"
		exit 1
	}
	grep -q "NO CITATION TABLE: no-table.md" <<<"$out" || {
		echo "research-note self-test FAIL: no-table branch: $out"
		exit 1
	}
	grep -q "EMPTY TRAIL: empty-trail.md" <<<"$out" || {
		echo "research-note self-test FAIL: empty-trail branch: $out"
		exit 1
	}
	grep -q "legacy.md" <<<"$out" && {
		echo "research-note self-test FAIL: grandfathered note was judged: $out"
		exit 1
	}
	grep -q "stale_after" <<<"$out" || {
		echo "research-note self-test FAIL: stale_after warn not reported: $out"
		exit 1
	}

	# A clean tree is green, and stale_after presence silences the WARN.
	rm -f "$local_dir/no-heading.md" "$local_dir/no-table.md" "$local_dir/empty-trail.md"
	printf '%s\n' "**stale_after:** 2027-05-01" >>"$local_dir/good.md"
	out=$(check_notes "$local_dir" "$tmp/grand.txt") && rc=0 || rc=$?
	if [ "$rc" != 0 ] || ! grep -q "research-note shape: 1/1 conforming, 1 grandfathered, 0 stale_after warn(s), 0 finding(s)" <<<"$out"; then
		echo "research-note self-test FAIL: clean tree rc=$rc out: $out"
		exit 1
	fi

	echo "research-note self-test ok (conforming rc=0 with pinned summary, missing heading / no table / empty trail all rc=1, grandfathered skipped, stale_after advisory)"
	exit 0
fi

cd "$(dirname "$0")/.." || exit 1

dir="${FAKE_DIR:-docs/research}"
check_notes "$dir" "scripts/research-note-shape-grandfathered.txt"
