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

if [ "$fail" = 0 ]; then
	echo "TODO_LIST gate ok (no unblocked owner-gated items)"
fi
exit "$fail"
