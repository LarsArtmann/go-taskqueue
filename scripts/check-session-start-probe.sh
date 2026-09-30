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
# into the close-out, never a stop condition. The pin is two-layered:
#   1. static shape assertions over the SHIPPED script bytes (the
#      rc-capture form present, a bare call absent);
#   2. an end-to-end run in a scratch git repo with a stub check-ci.sh
#      that exits 1: the ritual must still exit 0, REPORT the red master,
#      and print the completion SUMMARY.
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
out="$(bash "$tmp/repo/scripts/session-start.sh" 2>&1)" || rc=$?
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
