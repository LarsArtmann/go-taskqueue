#!/usr/bin/env bash
# Master-CI state gate (plan Round 11 T6): a local gate is worthless if
# master CI is red — five DONE verdicts shipped on a 3h-red master before
# this existed. Fails (exit 1) when the latest CI run on the current branch
# is red, so ci-local.sh and any DONE verdict inherit the check.
#
# Requires `gh` and network. Skippable: CI_CHECK=off (e.g. offline work).
set -euo pipefail
cd "$(dirname "$0")/.."

if [ "${CI_CHECK:-}" = "off" ]; then
	echo "check-ci: skipped (CI_CHECK=off)"
	exit 0
fi

if ! command -v gh >/dev/null 2>&1; then
	echo "check-ci: SKIP (gh not on PATH)"
	exit 0
fi

BRANCH="$(git rev-parse --abbrev-ref HEAD)"

# The workflow name matches .github/workflows/ci.yml. Use the branch's
# latest run, EXCLUDING runs still in progress (a red conclusion on an old
# commit is the signal; a running head says nothing yet).
RUN="$(gh run list --workflow CI --branch "$BRANCH" --status completed --limit 1 \
	--json conclusion,headSha,displayTitle,createdAt 2>/dev/null || true)"

if [ -z "$RUN" ] || [ "$RUN" = "[]" ]; then
	echo "check-ci: SKIP (no completed CI run found for branch $BRANCH)"
	exit 0
fi

CONCLUSION="$(echo "$RUN" | python3 -c 'import json,sys; print(json.load(sys.stdin)[0]["conclusion"])')"
SHA="$(echo "$RUN" | python3 -c 'import json,sys; print(json.load(sys.stdin)[0]["headSha"][:9])')"
TITLE="$(echo "$RUN" | python3 -c 'import json,sys; print(json.load(sys.stdin)[0]["displayTitle"])')"

if [ "$CONCLUSION" != "success" ]; then
	echo "check-ci: FAIL — latest CI run on $BRANCH ($SHA, \"$TITLE\") is $CONCLUSION"
	echo "  Fix master first (gh run list --branch $BRANCH; gh run view --log-failed <id>)"
	echo "  or bypass consciously with CI_CHECK=off."
	exit 1
fi

echo "check-ci: ok — latest CI run on $BRANCH ($SHA) is green"
