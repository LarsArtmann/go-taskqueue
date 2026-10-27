#!/usr/bin/env bash
# Mechanical archive bar (TODO rows 281/441/549): a status report may only
# move into docs/status/archived/ when every numbered / checkbox item in it
# is RESOLVED — struck (`~~…~~`) or marked (`- [x]`). Before this gate the
# every-item-resolved rule lived purely in agent judgment; judgment sweeps
# kept re-deriving it and unresolved rows silently died in the archive
# (2026-09-14 14-17 §e1; 2026-09-17 15-15 §e2/§f7).
#
# Usage: check-archive-eligibility.sh <candidate-report.md>…
#        (the agent underplies `git mv docs/status/x.md docs/status/archived/`
#        with this gate; ci-local runs the --self-test pin)
#
# Resolution contract (pinned by --self-test):
#   checkbox  `- [ ]` = UNRESOLVED, `- [x]`/`- [X]` = resolved (marked)
#   numbered  `1.`    = resolved ONLY when the line carries a `~~` strike
#   any line containing `~~` is treated as struck regardless of shape
set -uo pipefail
cd "$(dirname "$0")/.." || exit 1

self_abs="$(cd "$(dirname "$0")" && pwd)/$(basename "$0")"
tmp="" # set by self_test; the EXIT trap reads it (set -u safe)

# unresolved_lines <file>: print line numbers of unresolved items
unresolved_lines() {
	local file="$1" n=0 line trim
	while IFS= read -r line; do
		n=$((n + 1))
		case "$line" in
		*~~*) continue ;;
		esac
		trim="${line#"${line%%[![:space:]]*}"}" # strip leading whitespace (portable: no leading-class glob)
		case "$trim" in
		# checkbox: unresolved only when the box is empty
		*'- [ ]'*) echo "$n" ;;
		# numbered item without a strike: unresolved (strikes matched above)
		[0-9]*'.'*) echo "$n" ;;
		esac
	done <"$file"
}

run_gate() {
	local candidates=() arg
	# TQ_ARCHIVE_FILES env hook (fleet negative-battery convention) overrides argv
	if [ -n "${TQ_ARCHIVE_FILES:-}" ]; then
		while IFS= read -r arg; do [ -n "$arg" ] && candidates+=("$arg"); done <<<"$TQ_ARCHIVE_FILES"
	else
		candidates=("$@")
	fi
	if [ "${#candidates[@]}" -eq 0 ]; then
		echo "no archive candidates"
		return 0
	fi
	local ok=0 failed=0 file lines f
	for f in "${candidates[@]}"; do
		if [ ! -f "$f" ]; then
			echo "FAIL: $f is not a file"
			failed=$((failed + 1))
			continue
		fi
		lines="$(unresolved_lines "$f")"
		if [ -z "$lines" ]; then
			ok=$((ok + 1))
			continue
		fi
		failed=$((failed + 1))
		echo "NOT ELIGIBLE: $f has unresolved items (strike or mark before archiving):"
		while IFS= read -r n; do
			[ -n "$n" ] && sed -n "${n}p" "$f" | sed "s/^/  L$n: /"
		done <<<"$lines"
	done
	echo "archive-eligibility summary: $ok candidates ok, $failed failed"
	[ "$failed" -eq 0 ]
}

self_test() {
	local tmp out rc=0
	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' EXIT
	# fixture A: fully resolved (struck numbered items, marked checkboxes)
	# pins: resolved-archive-pass — struck+checked items never block, rc=0
	cat >"$tmp/resolved.md" <<'EOF'
## e1 findings
1. ~~landed the gate skeleton~~
2. ~~wired into ci-local~~
- [x] tick the row
plain prose is not an item and never blocks
EOF
	# fixture B: unresolved shapes that MUST each be caught
	# pins: unresolved-shapes-fail — open numbered/checkbox/indented items each fail
	cat >"$tmp/unresolved.md" <<'EOF'
## f7 findings
1. open numbered item
- [ ] open checkbox
  2. indented numbered item
- [x] resolved checkbox must NOT be listed
EOF
	local files
	files="$(printf '%s\n' "$tmp/resolved.md" "$tmp/unresolved.md")"
	out="$(TQ_ARCHIVE_FILES="$files" "$self_abs" 2>&1)" && rc=0 || rc=$?
	if [ "$rc" -ne 1 ]; then
		echo "FAIL: negative battery (want rc=1, got rc=$rc)"
		printf '%s\n' "$out"
		exit 1
	fi
	must_mention() {
		if grep -qF -- "$1" <<<"$2"; then
			echo "ok: $3"
		else
			echo "FAIL: $3 (output is missing '$1')"
			printf '%s\n' "$2"
			exit 1
		fi
	}
	must_mention 'archive-eligibility summary: 1 candidates ok, 1 failed' "$out" "summary pins exact counters"
	must_mention 'unresolved.md has unresolved items' "$out" "failure branch names the ineligible candidate"
	must_mention 'L2: 1. open numbered item' "$out" "numbered unresolved line reported"
	must_mention 'L3: - [ ] open checkbox' "$out" "unresolved checkbox reported"
	must_mention 'L4:   2. indented numbered item' "$out" "indented numbered item reported"
	if grep -qF 'resolved checkbox must NOT be listed' <<<"$out"; then
		echo "FAIL: resolved checkbox was flagged as unresolved"
		exit 1
	fi
	echo "ok: resolved checkbox not flagged"
	# positive battery: only the resolved candidate passes
	out="$(TQ_ARCHIVE_FILES="$tmp/resolved.md" "$self_abs" 2>&1)" && rc=0 || rc=$?
	must_mention 'archive-eligibility summary: 1 candidates ok, 0 failed' "$out" "positive battery passes a fully resolved candidate"
	# empty-candidates branch
	out="$(TQ_ARCHIVE_FILES='' "$self_abs" 2>&1)" && rc=0 || rc=$?
	must_mention 'no archive candidates' "$out" "empty-candidates branch exits 0 with the pinned line"
	echo "archive-eligibility self-test ok (summary pin + negative/positive/empty battery)"
}

if [ "${1:-}" = "--self-test" ]; then
	self_test
	exit 0
fi

run_gate "$@"
