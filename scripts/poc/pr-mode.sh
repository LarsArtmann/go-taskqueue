#!/usr/bin/env bash
# PR-mode PoC (plan D92): how an agent task becomes a pull request instead of
# a bare commit. Verifies the local mechanics end-to-end in a throwaway clone;
# the final `gh pr create` runs ONLY when you export OPEN_PR=1 and point
# REMOTE at a repository you own — opening PRs is an owner-visible action.
#
#   REMOTE=github.com/you/somerepo OPEN_PR=1 ./scripts/poc/pr-mode.sh
set -euo pipefail
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

: "${REMOTE:=}" "${OPEN_PR:=0}"

echo "== seed a source repo and a throwaway clone of it"
git init -q "$TMP/src"
echo "hello" > "$TMP/src/readme.md"
git -C "$TMP/src" -c user.email=poc@local -c user.name=poc add -A
git -C "$TMP/src" -c user.email=poc@local -c user.name=poc commit -qm init

git clone -q "$TMP/src" "$TMP/clone"

echo "== agent-style work: branch, change, commit (never main)"
BRANCH="tq/agent-$(date +%s)"
git -C "$TMP/clone" switch -qc "$BRANCH"
echo "agent fix" > "$TMP/clone/fix.txt"
git -C "$TMP/clone" -c user.email=agent@local -c user.name=agent add -A
git -C "$TMP/clone" -c user.email=agent@local -c user.name=agent commit -qm "fix: the thing TODO_LIST asked for"
git -C "$TMP/clone" log --oneline -1

echo "== push branch + open PR (this is the owner-visible part)"
if [ "$OPEN_PR" = "1" ] && [ -n "$REMOTE" ]; then
  git -C "$TMP/clone" remote set-url origin "https://$REMOTE"
  git -C "$TMP/clone" push -q -u origin "$BRANCH"
  (cd "$TMP/clone" && gh pr create \
    --title "tq agent: fix the TODO_LIST item" \
    --body "Automated fix for one TODO_LIST.md item, opened by the taskqueue agent pool." )
  echo "PASS: PR opened on $REMOTE"
else
  # Local-only proof of the push mechanics against a bare remote.
  git init -q --bare "$TMP/remote.git"
  git -C "$TMP/clone" push -q -u "$TMP/remote.git" "$BRANCH"
  git -C "$TMP/remote.git" branch
  echo "PASS (local mode): branch committed and pushed to a bare remote; set REMOTE=… OPEN_PR=1 to open a real PR"
fi
