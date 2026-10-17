#!/usr/bin/env bash
# Changed-lines line-length gate (round-11 M59): every line ADDED to a *.go
# file since the base revision must be <= 120 columns — the same cap the
# golines formatter and the advisory lll linter use. Scoping to changed
# lines keeps the ~legacy long lines out of the gate (they are baseline;
# mass-rewriting them buries real signal) while stopping new ones cold.
#
# Base revision: $LINT_BASE, else $1, else HEAD~1 (same resolution order as
# scripts/lint-annotations.sh). Generated files (*_templ.go) are excluded —
# templ fmt owns their formatting.
#
# --self-test pins that exclusion mechanically (row 110): two windows had
# verified the one property by hand (11-30 brutal self-review, the row-110
# window) — the signature of a missing mechanical pin. The awk filter is
# sed-extracted from the SHIPPED file (check-transient-retry.sh pattern), so
# the pin always exercises the bytes the gate actually runs; marker
# assertions fail loudly if the sed range stops matching, and a mutated-copy
# negative control proves the pin can fail.
set -euo pipefail
cd "$(dirname "$0")/.."

if [ "${1:-}" = "--self-test" ]; then
	awk_filter="$(sed -n "/^violations=\"[$](git diff/,/^')/p" "$0" | sed '1d;$d')"
	fail=0
	for marker in 'length(line) > 120' '_templ\.go$'; do
		if ! grep -qF -- "$marker" <<<"$awk_filter"; then
			echo "FAIL: extraction lost marker '$marker' — the sed range no longer matches the shipped awk filter"
			fail=1
		fi
	done
	if [ "$fail" -ne 0 ]; then
		exit 1
	fi

	# The fixture line must itself clear the cap, or both cases would pass on
	# a broken filter and the pin would be a false green.
	long_line='return w.execute(ctx, task, heartbeat, func(t *task.Task) error { return w.reviewAndRequeue(ctx, t, w.lease, w.requeueLadder, w.budgetGate, w.worktrees) })'
	if [ "${#long_line}" -le 120 ]; then
		echo "FAIL: fixture line is ${#long_line} chars — must be >120 for the pin to bite"
		exit 1
	fi

	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' EXIT
	: >"$tmp/fixture.diff"
	for file in internal/worker/loop.go internal/webui/fragments_templ.go; do
		printf 'diff --git a/%s b/%s\nindex 0000000..0000001 100644\n--- a/%s\n+++ b/%s\n@@ -12,0 +13,1 @@\n+%s\n' \
			"$file" "$file" "$file" "$file" "$long_line" >>"$tmp/fixture.diff"
	done

	hits="$(awk "$awk_filter" "$tmp/fixture.diff")"
	if [ "$(grep -c . <<<"$hits" || true)" -eq 1 ] && grep -qF 'internal/worker/loop.go:13:' <<<"$hits"; then
		echo "ok: >120-char added line in a plain .go file FAILs ($hits)"
	else
		echo "FAIL: plain .go fixture did not trip the gate — got: '$hits'"
		exit 1
	fi
	if grep -qF 'fragments_templ.go' <<<"$hits"; then
		echo "FAIL: *_templ.go exclusion lost — templ fixture tripped the gate:"
		printf '%s\n' "$hits"
		exit 1
	fi
	echo "ok: identical line in a *_templ.go file PASSes (generated files excluded)"

	# Negative control: a copy with the exclusion clause deleted must fail its
	# own self-test (marker fire) — proves the pin can fail, not decoration.
	sed 's/ && file !~ \/_templ\\\.go\$\///' "$0" >"$tmp/mutated.sh"
	if cmp -s "$0" "$tmp/mutated.sh"; then
		echo "FAIL: negative control — mutation did not apply (sed pattern no longer matches the shipped awk filter)"
		exit 1
	fi
	if bash "$tmp/mutated.sh" --self-test >/dev/null 2>&1; then
		echo "FAIL: negative control — mutated copy (exclusion deleted) still passed its own self-test"
		exit 1
	fi

	echo "lll-changed self-test ok (extracted filter pins the 120 cap + *_templ.go exclusion; negative control fails as expected)"
	exit 0
fi

base="${LINT_BASE:-${1:-}}"
if [ -z "$base" ] || ! git rev-parse -q --verify "$base^{commit}" >/dev/null 2>&1; then
	base="HEAD~1"
fi
if ! git rev-parse -q --verify "$base^{commit}" >/dev/null 2>&1; then
	echo "lll-changed: no base revision available, skipping"
	exit 0
fi

violations="$(git diff --unified=0 "$base" -- '*.go' | awk '
/^diff --git/ { file = "" }
/^\+\+\+ b\// { file = substr($0, 7) }
/^@@/ {
	# @@ -a,b +c,d @@ — new-file side starts at c
	split($0, h, " ")
	split(h[3], r, ",")
	newline = substr(r[1], 2) + 0
}
/^\+\+\+|^---|^diff --git|^index |^new file|^deleted file|^@@/ { next }
/^\+/ {
	line = substr($0, 2)
	if (length(line) > 120 && file !~ /_templ\.go$/) {
		printf "%s:%d: %d chars\n", file, newline, length(line)
	}
	newline++
	next
}
/^ / { newline++ }
')"

if [ -z "$violations" ]; then
	echo "lll-changed: no added line exceeds 120 chars (base $(git rev-parse --short "$base"))"
	exit 0
fi

echo "$violations"
echo "lll-changed: FAIL — added lines over 120 chars (wrap them; golines owns formatting)"
exit 1
