#!/usr/bin/env bash
# TODO_LIST honesty guard (round-10 T25): an UNCHECKED item whose text names
# sudo, owner decisions, or policy choices must carry "— BLOCKED:" — the
# harvester skips BLOCKED items, everything else becomes a pool task an
# agent cannot or should not execute (the 02:52 review's d2 defect class:
# a sudo-gated item and an owner-policy item sat unchecked and unblocked).
#
# Exit semantics (row 1098): only STRUCTURAL violations fail the gate —
# damaged/double checkboxes break the harvester's parse and are hard fails.
# Advisory classes (UNBLOCKED-OWNER-GATED phrasing, DONE-annotation rows)
# WARN without exiting 1 so the gate's exit code is trustworthy: rc=1
# always means TODO_LIST.md is machine-unreadable, never "someone should
# look at these".
set -uo pipefail
cd "$(dirname "$0")/.." || exit 1

# --self-test re-invokes THIS script; capture the absolute self path first.
self_abs="$(cd "$(dirname "$0")" && pwd)/$(basename "$0")"

self_test() {
	local dir fx rc out
	local okc=0 failc=0
	assert() {
		# assert <desc> <expected_rc> <needle-or-empty> <fixture-content>
		local desc="$1" want_rc="$2" needle="$3" content="$4"
		dir="$(mktemp -d)"
		fx="$dir/TODO_LIST.md"
		printf '%s\n' "$content" >"$fx"
		rc=0
		out="$(TQ_TODO_LIST_FILE="$fx" "$self_abs" 2>&1)" || rc=$?
		if [ "$rc" -ne "$want_rc" ]; then
			echo "FAIL: $desc (want rc=$want_rc, got rc=$rc)"
			printf '%s\n' "$out" | sed 's/^/    /'
			failc=$((failc + 1))
			return
		fi
		if [ -n "$needle" ] && ! printf '%s\n' "$out" | grep -qF -- "$needle"; then
			echo "FAIL: $desc (output missing: $needle)"
			printf '%s\n' "$out" | sed 's/^/    /'
			failc=$((failc + 1))
			return
		fi
		if [ -z "$needle" ] && printf '%s\n' "$out" | grep -qE 'UNBLOCKED-OWNER-GATED|DONE-ANNOTATION-UNCHECKED|DAMAGED-CHECKBOX|DOUBLE-CHECKBOX'; then
			echo "FAIL: $desc (expected quiet, got a warning)"
			printf '%s\n' "$out" | sed 's/^/    /'
			failc=$((failc + 1))
			return
		fi
		echo "ok: $desc (rc=$rc)"
		okc=$((okc + 1))
	}

	assert "clean fixture green" 0 "TODO_LIST gate ok" "- [ ] A plain agent-executable row"
	assert "DONE-annotation contradiction warns without failing (row 977 fixture 1)" 0 "DONE-ANNOTATION-UNCHECKED" "- [ ] Do the thing — DONE 2026-10-01: proof at head"
	assert "BLOCKED row stays exempt from the annotation warn (row 977 fixture 2)" 0 "TODO_LIST gate ok" "- [ ] Owner picks the theme — BLOCKED: owner call pending"
	assert "prose DONE mentions stay quiet (row 977 fixture 3)" 0 "TODO_LIST gate ok" "- [ ] Close as DONE only after gates; ARCHIVE-DONE verdicts apply"
	assert "unblocked owner-gated row warns without failing (row 1098 semantics)" 0 "UNBLOCKED-OWNER-GATED" "- [ ] Owner must run sudo deploy on the host"
	assert "damaged checkbox still fails (structural)" 1 "DAMAGED-CHECKBOX" "--- [x] broken bullet row"
	assert "double checkbox still fails (structural)" 1 "DOUBLE-CHECKBOX" "- [ ] [x] double box row"
	assert "glued two-item row fails (structural, row 1138)" 1 "GLUED-CHECKBOX" "- [ ] First row- [ ] Second row"
	assert "backticked prose checkbox mention stays quiet" 0 "TODO_LIST gate ok" "- [ ] Keep the \`- [ ]\` checkbox format, one item per line"
	assert "summary line pinned format" 0 "todo-list summary: 0 advisory warn(s), 0 structural violation class(es) — advisory warnings do not fail the gate" "- [ ] Another plain row"

	rm -rf "$dir" 2>/dev/null || true
	echo "todo-list self-test ok ($okc ok / $failc fail) — advisory/structural exit semantics + 977 fixture set"
	[ "$failc" -eq 0 ] || exit 1
}

if [ "${1:-}" = "--self-test" ]; then
	self_test
	exit 0
fi

todo="${TQ_TODO_LIST_FILE:-TODO_LIST.md}"
[ -f "$todo" ] || {
	echo "MISSING: $todo"
	exit 1
}

warns=0
fail=0
while IFS= read -r line; do
	case "$line" in
	*"— BLOCKED:"*) continue ;;
	esac

	for marker in "sudo" "owner" "policy decision" "go/no-go" "owner-run"; do
		if [[ "$line" == *"$marker"* ]]; then
			echo "UNBLOCKED-OWNER-GATED: $line"
			echo "  -> append ' — BLOCKED: <reason>' or rephrase so an agent can execute it"
			warns=$((warns + 1))
			break
		fi
	done
done < <(grep -E '^\s*- \[ \]' "$todo")

# Damaged checkbox shapes must fail the gate (04-46 §d4/§f1): the 04-40
# close-out shipped a backlog row as `--- [ ] …` — it matched neither the
# unchecked-grep above nor the harvester and was invisible to the pool
# until caught by eye. Any bullet run that runs into a checkbox bracket
# without the exact `- [ ] `/`- [x] ` (or `*`) prefix is a FAIL; the
# well-formed single-bullet forms and the space-less `-[ ]` shape the
# harvester tolerates are excluded.
damaged="$(grep -nE '^[[:space:]]*[-*][-*[:space:]]*\[[xX ]\]' "$todo" | grep -vE '^[0-9]+:[[:space:]]*[-*][ ]?\[[xX ]\]' || true)"
if [ -n "$damaged" ]; then
	echo "DAMAGED-CHECKBOX: checkbox line with a malformed bullet (must be exactly '- [ ] ' or '- [x] '):"
	printf '%s\n' "$damaged"
	fail=1
fi

# Double-checkbox shape must fail the gate (TODO row 206): the 2026-10-01
# tick batch shipped four rows as `- [ ] [x] …` — the well-formed prefix
# passes the damaged-bullet grep above and checkboxOf parses the line as
# an OPEN row, so the human-ticked state stayed invisible to both
# consumers until hand-caught. Any checkbox line whose text opens with a
# SECOND bracket is a FAIL.
doubled="$(grep -nE '^[[:space:]]*[-*][[:space:]]*\[[xX ]\][[:space:]]*\[[xX ]\]' "$todo" || true)"
if [ -n "$doubled" ]; then
	echo "DOUBLE-CHECKBOX: checkbox line with a second bracket after the prefix (exactly ONE '- [ ] '/'- [x] ' checkbox per line):"
	printf '%s\n' "$doubled"
	fail=1
fi

# Advisory WARN (row-110/111 closure-gap class, 05-19 §e4): an UNCHECKED row
# whose text already carries a trailing " — DONE/VERIFIED" annotation means
# the verdict was recorded but the checkbox never flipped (the 2026-09-22
# sweep windows verified rows 110 and 111 in prose and left both unchecked,
# each buying a full re-dispatch battery four days later), or the annotation
# is stale. Both are cheaper to resolve at dispatch than after a battery.
# BLOCKED rows are exempt (the harvester never dispatches them, same as the
# owner-marker loop above; their embedded dated sub-notes are legitimate
# accretion, e.g. the SystemNix cutover row). Anchored on the em-dash
# annotation shape so prose DONE mentions ("close as DONE",
# "ARCHIVE-DONE verdicts") do not trip it. WARN only (row 1098): the
# re-verification-policy ruling is rows 425/711's territory; a hard-gate
# flip is a separate follow-up row, not bundled here.
annotated="$(awk '/^[[:space:]]*- \[ \]/ && ! /— BLOCKED:/ && /— *(DONE|VERIFIED)([^A-Za-z]|$)/ { print NR ":" $0 }' "$todo")"
if [ -n "$annotated" ]; then
	echo "DONE-ANNOTATION-UNCHECKED (advisory WARN): unchecked rows whose text already carries a DONE/VERIFIED annotation:"
	printf '%s\n' "$annotated"
	echo "  -> tick with the citation (work genuinely done) or strip the annotation (row genuinely open)"
	while IFS= read -r _; do warns=$((warns + 1)); done <<<"$annotated"
fi

# Glued-row shape must fail the gate (row 1138): a `- [ ]` NOT at line
# start (the efbc8710 class — a second row glued onto the previous line
# was invisible to the harvester AND to the checkbox greps above, so the
# css-drift row shipped silently) is a hard structural violation,
# independent of the advisory-vs-structural ruling. A backtick before the
# bracket is the sanctioned prose mention of the format and stays quiet.
glued="$(grep -nE '[^`]- \[[xX ]\]' "$todo" || true)"
if [ -n "$glued" ]; then
	echo "GLUED-CHECKBOX: checkbox NOT at line start (a second row is glued onto the previous line; split it into its own '- [ ] ' line):"
	printf '%s\n' "$glued"
	fail=1
fi

echo "todo-list summary: $warns advisory warn(s), $fail structural violation class(es) — advisory warnings do not fail the gate"
if [ "$fail" = 0 ]; then
	echo "TODO_LIST gate ok"
fi
exit "$fail"
