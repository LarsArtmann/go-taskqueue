#!/usr/bin/env bash
# check-dlq-sidecar-citations.sh (TODO row 54): FAIL on pre-fix DLQ sidecar
# citations — `DLQPathFor(<x>.DBPath)` WITHOUT the PathFor wrapper. The
# corrected derivation is DLQPathFor(PathFor(d.DBPath))
# (internal/readmodel/dlq.go, the 2026-10-11 split-brain fix); a stale
# citation re-plants the split-brain, so the manual no-op sweep (row 20)
# becomes a gate.
#
# Scope: unchecked TODO_LIST.md rows (a hit on a continuation line belongs
# to the row's checkbox) + living tracked markdown. Excluded by design —
# they legitimately describe the pre-fix bug: checked rows, docs/status/**
# historical reports, docs/memos/** variant comparisons, docs/planning/**
# decision records, docs/architecture-understanding/** dated snapshots,
# CHANGELOG.md append-only history, vendor/**, and any line carrying the
# `dlq-shape-ok` marker (an unchecked row may QUOTE the wrong shape to
# specify its elimination — a mention, not a prescription).
#
# TQ_DLQ_CITE_ROOT overrides the scanned tree (self-test fixture hook).
# Countable summary pinned via --self-test (fleet convention 01-12 §e7/§g2).
#
#   scripts/check-dlq-sidecar-citations.sh           # exit 0 = clean
#   scripts/check-dlq-sidecar-citations.sh --self-test
set -euo pipefail
cd "$(dirname "$0")/.."

# Stale shape: DLQPathFor applied directly to a *.DBPath expression. The
# corrected form's argument starts with PathFor( — parens are outside the
# arg class, so it cannot match.
stale_re='DLQPathFor\([A-Za-z0-9_.]+\.DBPath\)'

historical() {
	case $1 in
	vendor/* | docs/status/* | docs/memos/* | docs/planning/* | docs/architecture-understanding/* | CHANGELOG.md) return 0 ;;
	esac
	return 1
}

scan() {
	local root=${TQ_DLQ_CITE_ROOT:-$PWD}
	local f files=0 stale=0 hit
	while IFS= read -r f; do
		historical "$f" && continue
		files=$((files + 1))
		if [ "$f" = "TODO_LIST.md" ]; then
			# Only UNCHECKED rows are in scope; a hit on a wrapped
			# continuation line inherits its row's checkbox state.
			local line_no=0 row_checked=0
			while IFS= read -r hit || [ -n "$hit" ]; do
				line_no=$((line_no + 1))
				case $hit in
				'- ['*']'*) row_checked=0 ;;
				esac
				case $hit in
				'- [x]'*) row_checked=1 ;;
				'- [ ]'*) row_checked=0 ;;
				esac
				if [ "$row_checked" = "0" ] && printf '%s\n' "$hit" | grep -qE "$stale_re" &&
					! printf '%s\n' "$hit" | grep -qF 'dlq-shape-ok'; then
					echo "TODO_LIST.md:$line_no: $hit"
					stale=$((stale + 1))
				fi
			done <"$root/TODO_LIST.md"
		else
			local out
			out=$(grep -nE "$stale_re" "$root/$f" || true)
			if [ -n "$out" ]; then
				printf '%s\n' "$out" | sed "s|^|$f:|"
				stale=$((stale + 1))
			fi
		fi
	done < <(git -C "$root" ls-files '*.md')
	echo "dlq-sidecar-citations: $files living md file(s) scanned, $stale stale pre-fix citation(s)"
	[ "$stale" = "0" ]
}

self_test() {
	local self=$0
	case $self in
	/*) ;;
	*) self="$PWD/${self#./}" ;;
	esac
	tmp="$(mktemp -d)"
	trap 'rm -rf "${tmp:-}"' EXIT
	git init -q -b master "$tmp/fix"
	git -C "$tmp/fix" config user.email t@t
	git -C "$tmp/fix" config user.name t

	local ok=0 fail=0
	expect() {
		local label=$1 want=$2 got=$3
		if [ "$got" = "$want" ]; then
			ok=$((ok + 1))
		else
			fail=$((fail + 1))
			echo "SELF-TEST FAIL ($label): want rc=$want got rc=$got" >&2
		fi
	}

	# Green corpus: corrected form everywhere + stale shapes ONLY in the
	# excluded-by-design locations.
	mkdir -p "$tmp/fix/docs/status" "$tmp/fix/docs/memos" "$tmp/fix/docs/planning"
	{
		echo '- [ ] fresh row cites DLQPathFor(PathFor(d.DBPath)) correctly'
		echo '- [x] checked row may describe the pre-fix DLQPathFor(d.DBPath) shape'
	} >"$tmp/fix/TODO_LIST.md"
	echo 'correct: DLQPathFor(PathFor(d.DBPath))' >"$tmp/fix/docs/live.md"
	echo 'historical: DLQPathFor(d.DBPath)' >"$tmp/fix/docs/status/report.md"
	echo 'comparison: DLQPathFor(d.DBPath)' >"$tmp/fix/docs/memos/memo.md"
	echo 'decision: DLQPathFor(d.DBPath)' >"$tmp/fix/docs/planning/sheet.md"
	echo 'append-only: DLQPathFor(d.DBPath)' >"$tmp/fix/CHANGELOG.md"
	git -C "$tmp/fix" add -A
	git -C "$tmp/fix" commit -qm base
	local rc out
	out=$(TQ_DLQ_CITE_ROOT="$tmp/fix" "$self" 2>&1) && rc=0 || rc=$?
	expect "clean corpus" 0 "$rc"
	printf '%s\n' "$out" | grep -qE 'dlq-sidecar-citations: [0-9]+ living md file\(s\) scanned, 0 stale' &&
		ok=$((ok + 1)) || {
		fail=$((fail + 1))
		echo "SELF-TEST FAIL: green summary not pinned" >&2
		printf '%s\n' "$out" >&2
	}

	# Negative: a stale cite on an UNCHECKED row's continuation line fails.
	{
		echo '- [ ] fresh row wraps:'
		echo '  sidecar via DLQPathFor(d.DBPath)'
	} >>"$tmp/fix/TODO_LIST.md"
	out=$(TQ_DLQ_CITE_ROOT="$tmp/fix" "$self" 2>&1) && rc=0 || rc=$?
	expect "unchecked-row continuation cite" 1 "$rc"
	printf '%s\n' "$out" | grep -q 'TODO_LIST.md:' &&
		ok=$((ok + 1)) || {
		fail=$((fail + 1))
		echo "SELF-TEST FAIL: hit must name TODO_LIST.md row" >&2
		printf '%s\n' "$out" >&2
	}
	git -C "$tmp/fix" checkout -q -- TODO_LIST.md

	# Mention-marker: an unchecked row quoting the shape to eliminate it
	# (dlq-shape-ok on the citing line) is a spec mention, not a citation.
	echo '- [ ] eliminate the wrong shape DLQPathFor(d.DBPath) everywhere (dlq-shape-ok)' >>"$tmp/fix/TODO_LIST.md"
	out=$(TQ_DLQ_CITE_ROOT="$tmp/fix" "$self" 2>&1) && rc=0 || rc=$?
	expect "dlq-shape-ok mention ignored" 0 "$rc"
	git -C "$tmp/fix" checkout -q -- TODO_LIST.md
	echo 'stale: DLQPathFor(d.DBPath)' >>"$tmp/fix/docs/live.md"
	out=$(TQ_DLQ_CITE_ROOT="$tmp/fix" "$self" 2>&1) && rc=0 || rc=$?
	expect "living-doc cite" 1 "$rc"
	printf '%s\n' "$out" | grep -q 'docs/live.md:' &&
		ok=$((ok + 1)) || {
		fail=$((fail + 1))
		echo "SELF-TEST FAIL: hit must name the living doc" >&2
		printf '%s\n' "$out" >&2
	}
	git -C "$tmp/fix" checkout -q -- docs/live.md

	if [ "$fail" = "0" ]; then
		echo "SELF-TEST OK ($ok checks)"
		return 0
	fi
	echo "SELF-TEST FAILED ($ok ok, $fail failed)" >&2
	return 1
}

if [ "${1:-}" = "--self-test" ]; then
	self_test
	exit $?
fi

scan || {
	echo "FAIL: stale pre-fix DLQ sidecar citation(s) above — correct to DLQPathFor(PathFor(d.DBPath)) (internal/readmodel/dlq.go)" >&2
	exit 1
}
