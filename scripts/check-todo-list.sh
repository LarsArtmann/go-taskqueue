#!/usr/bin/env bash
# TODO_LIST honesty guard (round-10 T25): an UNCHECKED item whose text names
# sudo, owner decisions, or policy choices must carry "— BLOCKED:" — the
# harvester skips BLOCKED items, everything else becomes a pool task an
# agent cannot or should not execute (the 02:52 review's d2 defect class:
# a sudo-gated item and an owner-policy item sat unchecked and unblocked).
set -uo pipefail
cd "$(dirname "$0")/.." || exit 1

todo="TODO_LIST.md"
[ -f "$todo" ] || {
	echo "MISSING: $todo"
	exit 1
}

fail=0
while IFS= read -r line; do
	case "$line" in
	*"— BLOCKED:"*) continue ;;
	esac

	for marker in "sudo" "owner" "policy decision" "go/no-go" "owner-run"; do
		if [[ "$line" == *"$marker"* ]]; then
			echo "UNBLOCKED-OWNER-GATED: $line"
			echo "  -> append ' — BLOCKED: <reason>' or rephrase so an agent can execute it"
			fail=1
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
# "ARCHIVE-DONE verdicts") do not trip it. WARN only: the
# re-verification-policy ruling is rows 425/711's territory; a hard-gate
# flip is a separate follow-up row, not bundled here.
annotated="$(awk '/^[[:space:]]*- \[ \]/ && ! /— BLOCKED:/ && /— *(DONE|VERIFIED)([^A-Za-z]|$)/ { print NR ":" $0 }' "$todo")"
if [ -n "$annotated" ]; then
	echo "DONE-ANNOTATION-UNCHECKED (advisory WARN): unchecked rows whose text already carries a DONE/VERIFIED annotation:"
	printf '%s\n' "$annotated"
	echo "  -> tick with the citation (work genuinely done) or strip the annotation (row genuinely open)"
fi

if [ "$fail" = 0 ]; then
	echo "TODO_LIST gate ok (no unblocked owner-gated items)"
fi
exit "$fail"
