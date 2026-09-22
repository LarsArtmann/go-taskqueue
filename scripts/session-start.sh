#!/usr/bin/env bash
# Session-start ritual (AGENTS.md "Session-start ritual"): run this as the
# FIRST action of every agent session in this repo. Makes the documented
# skip/late class (00-52 d4, 02-17 d3, 00-22 d1, 00-32 d2) structurally
# impossible. Read-only — no state is mutated.
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

echo "=== git log (whole repo, concurrent agents land everywhere) ==="
git log --oneline -5

echo
echo "=== git status ==="
git status --short || true

echo
echo "=== git stash list ==="
git stash list || true

echo
echo "=== prior reports for current task ID(s) ==="
# Pass task IDs as arguments: scripts/session-start.sh <id> [<id>...]
# With no arguments, prints the hint instead of guessing.
if [ "$#" -gt 0 ]; then
  for id in "$@"; do
    echo "--- $id ---"
    local_reports="$(rg -l "$id" docs/status/ 2>/dev/null || true)"
    if [ -z "$local_reports" ]; then
      echo "(no prior reports)"
    else
      echo "prior windows:"
      echo "$local_reports" | sed 's/^/  /'
      done_rows="$(rg -n "$id" docs/status/ 2>/dev/null | rg 'DONE|done row|\[x\]' || true)"
      if [ -n "$done_rows" ]; then
        echo "DONE-row state:"
        echo "$done_rows" | sed 's/^/  /'
      else
        echo "(no DONE rows mention this ID — repeat dispatch possible)"
      fi
    fi
  done
else
  echo "(pass task IDs: scripts/session-start.sh <task-id>…; rg docs/status/ skipped)"
fi

echo
echo "=== CONTRIBUTING.md head ==="
if [ -f CONTRIBUTING.md ]; then
  head -40 CONTRIBUTING.md
else
  echo "(no CONTRIBUTING.md)"
fi
if [ -f CLAUDE.md ]; then
  echo
  echo "=== CLAUDE.md head ==="
  head -40 CLAUDE.md
fi

echo
echo "Session-start ritual complete. Read the output before editing anything."
