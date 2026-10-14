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

# Classify a workflow conclusion into the three gate classes. A CANCELLED
# run is its own NEUTRAL class (not red): it carries no pass/fail signal
# and usually means a newer push superseded it (owner ruling O17,
# docs/planning/2026-10-05_15-00_OWNER-RULINGS-SHEET.md).
classify_conclusion() {
	case "$1" in
	success) echo "ok" ;;
	cancelled) echo "cancelled" ;;
	*) echo "red" ;;
	esac
}

# Hermetic negative test: CHECK_CI_SELF_TEST=1 pins the classification
# without network or gh (wired as a ci-local step next to the live probe).
if [ "${CHECK_CI_SELF_TEST:-}" = "1" ]; then
	[ "$(classify_conclusion success)" = "ok" ] || {
		echo "check-ci self-test: success must classify ok"
		exit 1
	}
	[ "$(classify_conclusion cancelled)" = "cancelled" ] || {
		echo "check-ci self-test: cancelled must be its own neutral class, not red"
		exit 1
	}
	[ "$(classify_conclusion failure)" = "red" ] || {
		echo "check-ci self-test: failure must stay red"
		exit 1
	}
	[ "$(classify_conclusion timed_out)" = "red" ] || {
		echo "check-ci self-test: timed_out must stay red"
		exit 1
	}
	[ "$(classify_conclusion startup_failure)" = "red" ] || {
		echo "check-ci self-test: startup_failure must stay red"
		exit 1
	}
	[ "$(classify_conclusion '')" = "red" ] || {
		echo "check-ci self-test: empty conclusion must stay red"
		exit 1
	}
	echo "check-ci: self-test ok (success=ok, cancelled=neutral, else=red)"
	exit 0
fi

# The workflow name matches .github/workflows/ci.yml. Use the branch's
# latest run, EXCLUDING runs still in progress (a red conclusion on an old
# commit is the signal; a running head says nothing yet). ONE gh call
# extracts all four fields via --jq (04-21 e4: three python3 subprocesses
# per invocation was pure overhead).
RUN="$(gh run list --workflow CI --branch "$BRANCH" --status completed --limit 1 \
	--json databaseId,conclusion,headSha,displayTitle \
	--jq '.[0] | [(.conclusion // ""), (.databaseId | tostring), .headSha, .displayTitle] | @tsv' 2>/dev/null || true)"

if [ -z "$RUN" ] || [ "$RUN" = "[]" ]; then
	echo "check-ci: SKIP (no completed CI run found for branch $BRANCH)"
	exit 0
fi

IFS=$'\t' read -r CONCLUSION RUN_ID RUN_SHA TITLE <<<"$RUN"
SHA="${RUN_SHA:0:9}"
LOCAL_SHA="$(git rev-parse HEAD)"

CLASS="$(classify_conclusion "$CONCLUSION")"
case "$CLASS" in
ok)
	echo "check-ci: ok — latest CI run on $BRANCH ($SHA) is green"
	exit 0
	;;
cancelled)
	echo "check-ci: NEUTRAL — latest CI run on $BRANCH ($SHA, \"$TITLE\") was cancelled (not red)."
	echo "  Re-run it with: gh run rerun $RUN_ID"
	exit 0
	;;
esac

if [ "$CONCLUSION" != "success" ]; then
	echo "check-ci: FAIL — latest CI run on $BRANCH ($SHA, \"$TITLE\") is $CONCLUSION (local HEAD: ${LOCAL_SHA:0:9})"
	if [ "$RUN_SHA" = "$LOCAL_SHA" ]; then
		echo "  The red run IS your tree — fix master first (gh run view --log-failed $RUN_ID)"
		echo "  or bypass consciously with CI_CHECK=off."
	elif git merge-base --is-ancestor "$RUN_SHA" "$LOCAL_SHA" 2>/dev/null; then
		# The red run tested an older commit that is an ancestor of the
		# local tree: a fix may already be in flight (push pending or CI
		# still running) — right instinct to distrust it, precise word.
		echo "  The red run predates your tree (its sha is an ancestor of local HEAD) — pushing a fix?"
		echo "  Bypass consciously with CI_CHECK=off."
	else
		# Not an ancestor: the branch history forked (force-push or
		# rebase) — the stale red is noise, but "predates" would be a lie.
		echo "  The red run's sha is NOT an ancestor of local HEAD — branch history diverged; judge that run on its own merits."
		echo "  Bypass consciously with CI_CHECK=off."
	fi
	exit 1
fi

echo "check-ci: ok — latest CI run on $BRANCH ($SHA) is green"
