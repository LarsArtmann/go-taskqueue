#!/usr/bin/env bash
# Regression pin for the master-CI probe section of scripts/session-start.sh
# (23-28 report b2/f2; TODO row "session-start.sh regression pin"). The
# load-bearing shape is the rc-capture form:
#
#	ci_rc=0
#	bash scripts/check-ci.sh || ci_rc=$?
#
# A regression to a bare `bash scripts/check-ci.sh` call aborts the whole
# ritual on a red master under `set -euo pipefail` — red is signal to carry
# into the close-out, never a stop condition. The pin is four-layered:
#   1. static shape assertions over the SHIPPED script bytes (the
#      rc-capture form present, a bare call absent);
#   2. an end-to-end run in a scratch git repo with a stub check-ci.sh
#      that exits 1: the ritual must still exit 0, REPORT the red master,
#      and print the completion SUMMARY;
#   3. a static assertion that the queue-record probe stays guarded by
#      `command -v tq` (the TQ_DB hermeticity gate — a bare `tq show`
#      would touch the production journal from any environment);
#   4. end-to-end WITH args: a fixture report (verdict line, DONE-row
#      hit, footer-carrying commit) must surface, an unknown id must be
#      tolerated, and both runs must take the tq-ABSENT path. The ritual
#      executes under a symlink-farm PATH containing no tq, so no queue
#      binary and no database can be reached regardless of the host env.
#      The tq-present shape (stubbed `tq` output) stays unpinned pending
#      an owner ruling (08-57 §g1).
set -euo pipefail
cd "$(dirname "$0")/.."

script=scripts/session-start.sh
[ -f "$script" ] || {
	echo "FAIL: $script missing"
	exit 1
}

# 1. Static shape assertions on the shipped bytes.
if ! grep -qF 'ci_rc=0' "$script" || ! grep -qF 'bash scripts/check-ci.sh || ci_rc=$?' "$script"; then
	echo "FAIL: rc-capture shape missing from $script — the master-CI probe no longer tolerates a red master"
	exit 1
fi
if grep -Eq '^[[:space:]]*bash scripts/check-ci\.sh[[:space:]]*$' "$script"; then
	echo "FAIL: bare 'bash scripts/check-ci.sh' call found in $script — a red master would abort the ritual under set -e"
	exit 1
fi
echo "ok: rc-capture shape present, no bare probe call"

# 2. End-to-end: red master, ritual still completes.
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
git init -q "$tmp/repo"
mkdir -p "$tmp/repo/scripts" "$tmp/repo/docs/status"
cp "$script" "$tmp/repo/scripts/session-start.sh"
cat >"$tmp/repo/scripts/check-ci.sh" <<'EOF'
#!/usr/bin/env bash
echo "mock: master CI is RED"
exit 1
EOF
chmod +x "$tmp/repo/scripts/check-ci.sh"
printf 'index\n' >"$tmp/repo/docs/status/README.md"
printf 'contributing\n' >"$tmp/repo/CONTRIBUTING.md"
git -C "$tmp/repo" add -A
git -C "$tmp/repo" -c user.email=t@t -c user.name=t commit -qm init

rc=0
out="$(cd "$tmp/repo" && bash scripts/session-start.sh 2>&1)" || rc=$?
if [ "$rc" -ne 0 ]; then
	echo "FAIL: session-start.sh exited $rc on a red master — the ritual must always complete"
	echo "$out"
	exit 1
fi
if ! grep -q 'master was red at session START' <<<"$out"; then
	echo "FAIL: red master was not reported in the ritual output"
	echo "$out"
	exit 1
fi
if ! grep -q 'Session-start ritual complete' <<<"$out"; then
	echo "FAIL: ritual did not reach its completion line"
	echo "$out"
	exit 1
fi
echo "ok: red master reported, ritual still completed (rc=0)"

# 3. Static tq-hermeticity shape: the queue-record probe must stay guarded
# by `command -v tq` so that the only form able to reach the journal is an
# explicitly present tq; the absent branch prints a skip note.
if ! grep -qF 'if command -v tq >/dev/null 2>&1; then' "$script"; then
	echo "FAIL: queue-record probe is not guarded by 'command -v tq' in $script — a bare tq show would touch the production journal (TQ_DB hazard)"
	exit 1
fi
echo "ok: tq probe guarded by command -v"

# 4. End-to-end with args: fixture report, verdict line, DONE-row hit and
# footer commit must surface; an unknown id must be tolerated; both runs
# must take the tq-absent path (PATH scrubbed to a symlink farm without
# tq — no host queue binary or TQ_DB can be contacted).
fixid="00000000feedface0000000000000042"
report="$tmp/repo/docs/status/2026-09-30_10-00_task-$fixid.md"
cat >"$report" <<EOF
# Task $fixid — args-branch fixture

**Verdict: FULLY DONE — fixture verdict surfaced beside the filename.**

TODO row [x] for $fixid is DONE (fixture body line for the DONE-row grep).
EOF
git -C "$tmp/repo" add -A
git -C "$tmp/repo" -c user.email=t@t -c user.name=t commit -qm "fixture: pin commit for $fixid" -m "Task-Queue-ID: $fixid"

scratchbin="$tmp/bin"
mkdir -p "$scratchbin"
for tool in bash sh git rg sed head tail basename; do
	ln -s "$(command -v "$tool")" "$scratchbin/$tool"
done

rc=0
out="$(cd "$tmp/repo" && PATH="$scratchbin" bash scripts/session-start.sh "$fixid" 2>&1)" || rc=$?
if [ "$rc" -ne 0 ]; then
	echo "FAIL: session-start.sh exited $rc with args on the tq-absent PATH"
	echo "$out"
	exit 1
fi
for want in \
	"--- $fixid ---" \
	"prior windows:" \
	"2026-09-30_10-00_task-$fixid.md" \
	"verdicts:" \
	"**Verdict: FULLY DONE" \
	"DONE-row state:" \
	"footer already carried by:" \
	"fixture: pin commit for $fixid" \
	"tq not on PATH" \
	"Session-start ritual complete (SUMMARY: windows=1 done-row-hits=1)"; do
	if ! grep -qF -- "$want" <<<"$out"; then
		echo "FAIL: args-branch output missing: $want"
		echo "$out"
		exit 1
	fi
done
echo "ok: args branch surfaced report, verdict, DONE-row, footer (tq absent)"

rc=0
out="$(cd "$tmp/repo" && PATH="$scratchbin" bash scripts/session-start.sh "00000000feedface00000000000000ff" 2>&1)" || rc=$?
if [ "$rc" -ne 0 ]; then
	echo "FAIL: session-start.sh exited $rc for an unknown task id"
	echo "$out"
	exit 1
fi
for want in \
	"(no prior reports)" \
	"tq not on PATH" \
	"Session-start ritual complete (SUMMARY: windows=1 done-row-hits=0)"; do
	if ! grep -qF -- "$want" <<<"$out"; then
		echo "FAIL: unknown-id output missing: $want"
		echo "$out"
		exit 1
	fi
done
echo "ok: unknown id tolerated, counters windows=1 done-row-hits=0"
