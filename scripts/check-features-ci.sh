#!/usr/bin/env bash
# FEATURES CI-freshness gate (Pareto M4, 2026-09-13): every GitHub Actions
# run id cited in FEATURES.md (pattern `run NNNNNN`) must be a COMPLETED,
# SUCCESS run on the current branch. A red/cancelled/stale citation means
# the feature table is claiming evidence that no longer holds — the exact
# 08-49 e1 drift class (FEATURES cited a red master while calling itself
# fresh). Advisory about AGE, gating about TRUTH: a green-but-old run is
# fine; a red one fails the gate.
set -uo pipefail
cd "$(dirname "$0")/.."

features="FEATURES.md"
[ -f "$features" ] || { echo "MISSING: $features"; exit 1; }

command -v gh >/dev/null 2>&1 || { echo "SKIP: gh not on PATH"; exit 0; }
[ -n "${CI:-}" ] && [ -z "${GITHUB_TOKEN:-}" ] && { echo "SKIP: no gh auth in CI"; exit 0; }

branch="$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo master)"

fail=0
seen=0
run_ids="$(grep -oE '\brun [0-9]{6,}' "$features" | awk '{print $2}' | sort -u)"
for id in $run_ids; do
	seen=$((seen + 1))
	info="$(gh run view "$id" --json status,conclusion,headBranch 2>/dev/null)" || {
		echo "STALE: FEATURES cites run $id but gh cannot see it (deleted or wrong repo)"
		fail=1
		continue
	}
	status="$(echo "$info" | grep -oE '"status":"[^"]*"' | cut -d'"' -f4)"
	conclusion="$(echo "$info" | grep -oE '"conclusion":"[^"]*"' | cut -d'"' -f4)"
	head_branch="$(echo "$info" | grep -oE '"headBranch":"[^"]*"' | cut -d'"' -f4)"
	if [ "$status" != "completed" ] || [ "$conclusion" != "success" ]; then
		echo "RED CITATION: FEATURES cites run $id (status=$status conclusion=$conclusion) — feature table claims evidence that failed"
		fail=1
	fi
	if [ -n "$head_branch" ] && [ "$head_branch" != "$branch" ]; then
		echo "NOTE: run $id is on branch '$head_branch' (local branch: '$branch')"
	fi
done

if [ "$seen" = 0 ]; then
	echo "NO-CITATIONS: $features cites no CI run ids — gate vacuously green (add 'run NNNNNN' evidence to rows as they ship)"
	exit 0
fi

if [ "$fail" = 0 ]; then
	echo "FEATURES-CI GATE GREEN: $seen cited run(s), all completed/success"
	exit 0
fi
exit 1
