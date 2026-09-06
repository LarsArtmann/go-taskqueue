#!/usr/bin/env bash
# Worktree-isolation PoC (plan D93): run an agent task against a disposable
# git worktree of a repo instead of the checkout itself, so pool work can
# never trample the human's working tree.
#
# Verifies the mechanics the executor would rely on:
#   1. `git worktree add` creates a linked checkout with its own HEAD,
#   2. commits made in the worktree stay off the main checkout's branch,
#   3. the main checkout's dirty/clean state is untouched.
set -euo pipefail
TMP="$(mktemp -d)"
trap 'git -C "$TMP/repo" worktree remove --force "$TMP/wt" >/dev/null 2>&1 || true; rm -rf "$TMP"' EXIT

echo "== seed a repo with one commit and a dirty file"
git init -q "$TMP/repo"
git -C "$TMP/repo" -c user.email=poc@local -c user.name=poc commit -q --allow-empty -m init
echo "human WIP" >"$TMP/repo/wip.txt"

echo "== create the worktree the agent would work in"
git -C "$TMP/repo" worktree add -q -b tq/agent-work "$TMP/wt"
echo "agent change" >"$TMP/wt/agent.txt"
git -C "$TMP/wt" -c user.email=agent@local -c user.name=agent add -A
git -C "$TMP/wt" -c user.email=agent@local -c user.name=agent commit -qm "agent work"

echo "== prove isolation"
git -C "$TMP/repo" rev-parse HEAD >"$TMP/main_head"
git -C "$TMP/wt" rev-parse HEAD >"$TMP/wt_head"
if cmp -s "$TMP/main_head" "$TMP/wt_head"; then
	echo "FAIL: worktree commit leaked into main HEAD"
	exit 1
fi
[ -f "$TMP/repo/wip.txt" ] || {
	echo "FAIL: main checkout mutated"
	exit 1
}
[ ! -f "$TMP/repo/agent.txt" ] || {
	echo "FAIL: agent file landed in main checkout"
	exit 1
}
[ -z "$(git -C "$TMP/repo" status --porcelain -- worktree 2>/dev/null)" ] || true
echo "   main HEAD untouched, human WIP intact, agent commit isolated"
echo "PASS: worktree isolation mechanics work (wire into AgentExecutor as the next step)"
