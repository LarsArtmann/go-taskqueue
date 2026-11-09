#!/usr/bin/env bash
# Regression pin for the WITH-ARGS branch of scripts/session-start.sh
# (2026-09-30 08-57 report §g1 — owner-ruled: scratch-repo e2e pins are
# the accepted bar for utility regression pins). The no-args red-master
# path already runs live at every session start; this pin covers the
# ID-args path hermetically: stub `rg` (grep-backed) and `tq` on PATH so
# the prior-reports lookup, verdict surfacing, DONE-row detection, footer
# scan and queue-record block are all exercised without docs/status/,
# the production journal or network.
#
# Hermeticity: runs in a scratch git repo (no check-ci.sh, no hooks, no
# docs/) — the ritual's degraded-path messages for all of those are part
# of what the pin asserts. Never run against the real repo state.
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
TARGET="$SCRIPT_DIR/../session-start.sh"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
. "$SCRIPT_DIR/_lib.sh"

fail() {
	echo "FAIL: $*" >&2
	exit 1
}
pass() { echo "ok: $*"; }

# grep-backed rg stub: supports exactly the -l / -m1 / -n forms
# session-start.sh uses; anything else falls through to plain grep (the
# DONE-row filter pipes into a second rg call).
stubbin="$WORK/stubbin"
mkdir -p "$stubbin"
cat >"$stubbin/rg" <<'EOF'
#!/bin/sh
case $1 in
-l) shift; pat=$1; shift; exec grep -rl -- "$pat" "$@" ;;
-m1) shift; pat=$1; shift; exec grep -rm1 -- "$pat" "$@" ;;
-n) shift; pat=$1; shift; exec grep -rn -- "$pat" "$@" ;;
*) exec grep -E "$@" ;;
esac
EOF
cat >"$stubbin/tq" <<'EOF'
#!/bin/sh
[ "$1" = show ] || exit 0
echo "stub queue record for $2 (status=PENDING attempts=0)"
EOF
chmod +x "$stubbin/rg" "$stubbin/tq"

ID=000001a1bf29b81e21c3c2e6cee500000000
r=$(new_repo session-start-pin)
mkdir -p "$r/docs/status"
export PATH="$stubbin:$PATH"

# Seed: a prior report with verdict + DONE row, a footer commit for the ID.
cat >"$r/docs/status/prior.md" <<EOF
# prior window

**Verdict: DONE — pinned by smoke**

- [x] Gate-script polish ($ID)
EOF
git -C "$r" add docs/status
git -C "$r" commit -q -m "prior report"
git -C "$r" commit -q --allow-empty -m "fix: pinned work

Task-Queue-ID: $ID"

out=$(run_in "$r" "$TARGET" "$ID")
grep -q "prior windows:" <<<"$out" || fail "with-args run did not list prior reports: $out"
grep -q "docs/status/prior.md" <<<"$out" || fail "prior report path missing from output: $out"
grep -q "prior.md: \*\*Verdict: DONE" <<<"$out" || fail "verdict line not surfaced: $out"
grep -q "DONE-row state:" <<<"$out" || fail "DONE-row block missing: $out"
grep -q "footer already carried by:" <<<"$out" || fail "footer block missing: $out"
grep -q "stub queue record for $ID" <<<"$out" || fail "tq queue-record block missing: $out"
grep -q "windows=1 done-row-hits=1" <<<"$out" || fail "summary must count the window and its DONE hit: $out"
pass "with-args branch: prior reports, verdicts, DONE rows, footer, queue record"

# Unknown ID degrades without aborting (the set -e trap seen live, 07-12).
out=$(run_in "$r" "$TARGET" 0000000000000000deadbeefcafe00000000)
grep -q "(no prior reports)" <<<"$out" || fail "unknown ID must print the no-reports line: $out"
grep -q "windows=1 done-row-hits=0" <<<"$out" || fail "unknown ID must still complete the summary: $out"
pass "unknown ID: ritual completes, no DONE-row credit"

# Degraded-environment lines: no check-ci.sh, no hooks, no CONTRIBUTING in
# the scratch repo — the ritual must name each degradation, not die.
grep -q "master was red at session START" <<<"$out" || fail "missing check-ci.sh must surface the red-at-start note: $out"
grep -q "hooks INERT" <<<"$out" || fail "hook-less scratch repo must surface hooks INERT: $out"
pass "degraded scratch env: all degradation notes printed, ritual completes"

echo "session-start smoke: PASS (3 checks)"
