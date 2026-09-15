#!/usr/bin/env bash
# Rename-hygiene scanner (TODO_LIST 2026-09-11 evening row; 07-39 report f4):
# sweep quoted string literals in a diff for identifier-shaped corruptions —
# the class where a variable rename leaks into a nearby string literal
# (`q.Get("query")` deadened the webui filter, `task(store)` mangled the
# `tq dlq --max-attempts` help; both caught by hand, da8f331/9b7c46b).
#
# Method: collect identifier-shaped tokens from REMOVED diff lines (the old
# names of a rename), then flag any double-quoted string literal on an ADDED
# line whose content is exactly such a token. Advisory — exit 0 unless
# --strict; a hit is a hypothesis a human confirms, not a verdict.
#
# Usage: check-rename-hygiene.sh [--strict] [git diff args...]
#   default diff: working tree vs HEAD (git diff HEAD)
# Operates on the repository of the INVOKING directory (ci-local runs from
# the repo root; point the invocation elsewhere to scan another checkout).
set -euo pipefail

diff_cmd=(git diff)
strict=0
args=()
for a in "$@"; do
	case "$a" in
	--strict) strict=1 ;;
	*) args+=("$a") ;;
	esac
done
if [ "${#args[@]}" -gt 0 ]; then
	diff_cmd+=("${args[@]}")
else
	diff_cmd+=(HEAD)
fi

added="$(mktemp)"
removed="$(mktemp)"
trap 'rm -f "$added" "$removed"' EXIT

# Split the unified diff into +/- lines (skip +++/--- file headers).
"${diff_cmd[@]}" | awk '
	/^\+\+\+|^---/ { next }
	/^\+/ { print substr($0, 2) > "'"$added"'" }
	/^-/ { print substr($0, 2) > "'"$removed"'" }
'

# Identifier-shaped tokens from removed lines: the rename's old names.
old_names="$(grep -ohE '[A-Za-z_][A-Za-z0-9_]{2,}' "$removed" | sort -u || true)"
[ -n "$old_names" ] || { echo "rename-hygiene: no removed identifiers in diff — clean"; exit 0; }

# Double-quoted literals on added lines whose content is EXACTLY
# identifier-shaped; flag when the literal equals an old name.
# Shell-string literals in single quotes are out of scope (Go strings are
# double-quoted). Line numbers are approximate (added-line ordinal) — the
# report is a pointer, not a coordinate.
hits=0
while IFS= read -r line; do
	case "$line" in
	*'"'*'"'*) ;;
	*) continue ;;
	esac
	while IFS= read -r lit; do
		case "$lit" in
		'' | *[!A-Za-z0-9_]*) continue ;;
		esac
		if printf '%s\n' "$old_names" | grep -qxF "$lit"; then
			echo "rename-hygiene: quoted literal \"$lit\" on added line matches an identifier removed by this diff:"
			echo "  + $line"
			hits=$((hits + 1))
		fi
	done < <(printf '%s' "$line" | grep -oE '"[^"]*"' | sed 's/^"\(.*\)"$/\1/')
done < "$added"

if [ "$hits" -eq 0 ]; then
	echo "rename-hygiene: clean — no identifier-shaped literal shadows a removed identifier"
	exit 0
fi
echo
echo "rename-hygiene: $hits hit(s)"
if [ "$strict" -eq 1 ]; then
	echo "FAIL: fix the literal(s) or suppress consciously (string literals must never carry a renamed identifier — AGENTS.md)"
	exit 1
fi
echo "advisory — confirm each hit: a rename that leaked into a string literal silently breaks filters, help text, and JSON keys"
