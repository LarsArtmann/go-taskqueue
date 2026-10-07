#!/usr/bin/env bash
# Derivation-blind commit census (TODO row: one-shot historical census; the
# doctor-surface row turns the same classes into a live WARN).
#
# A commit is DERIVATION-BLIND when the queue cannot derive it:
#   invisible  - the message carries a verbatim `Task-Queue-ID: <36hex>`
#                footer but git %(trailers) cannot see it (footer sits above
#                the trailing attribution block — the final-paragraph gap,
#                pinned by TestGitLogScannerFinalParagraphGapCharacterized);
#                CommitsByTrailer attributes nothing → the task re-dispatches.
#   nofooter   - no Task-Queue-ID footer at all (daemon sweeps, session
#                work, human commits); only a cost when it bears code.
#
# Usage: scripts/census-derivation-blind.sh [git-range] [--ids]
#   git-range   default: v0.1.0..HEAD (falls back to HEAD~400..HEAD when
#               the tag is absent)
#   --ids       also list every invisible commit (sha, task id, subject)
set -euo pipefail
cd "$(dirname "$0")/.."

range="${1:-}"
ids=0
for arg in "$@"; do
	if [ "$arg" = "--ids" ]; then
		ids=1
	fi
done

if [ -z "$range" ] || [ "$range" = "--ids" ]; then
	if git rev-parse -q --verify v0.1.0 >/dev/null; then
		range="v0.1.0..HEAD"
	else
		range="HEAD~400..HEAD"
	fi
fi

total=0
visible=0
invisible=0
nofooter=0
invisible_list=""

for sha in $(git rev-list "$range"); do
	total=$((total + 1))

	# Raw body carries the footer verbatim…
	raw=$(git log -1 --format=%B "$sha" | grep -c '^Task-Queue-ID: [0-9a-f]\{36\}$' || true)
	# …and the trailer parser sees it (valueonly output is value + newline).
	trailer=$(git log -1 --format='%(trailers:key=Task-Queue-ID,valueonly)' "$sha" | grep -c '[0-9a-f]\{36\}' || true)

	if [ "$raw" -gt 0 ] && [ "$trailer" -gt 0 ]; then
		visible=$((visible + 1))
	elif [ "$raw" -gt 0 ]; then
		invisible=$((invisible + 1))
		invisible_list="$invisible_list $sha"
	else
		nofooter=$((nofooter + 1))
	fi
done

footer=$((visible + invisible))

echo "derivation-blind census over $range:"
echo "  commits:        $total"
echo "  footer commits: $footer (visible $visible / INVISIBLE $invisible)"
if [ "$footer" -gt 0 ]; then
	echo "  invisible rate: $((invisible * 100 / footer))% of footer commits"
fi
echo "  no-footer:      $nofooter (daemon sweeps / session work / human commits)"

if [ "$ids" = 1 ]; then
	echo
	echo "invisible commits (sha, task id, subject):"
	for sha in $invisible_list; do
		task=$(git log -1 --format=%B "$sha" | grep '^Task-Queue-ID: ' | head -1 | cut -d' ' -f2)
		subject=$(git log -1 --format=%s "$sha")
		echo "  $sha $task $subject"
	done
fi
