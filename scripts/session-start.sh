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
      # A DONE-row verdict is a stronger stop signal than a filename
      # match, so each prior report's verdict line surfaces beside it
      # (repeat-dispatch window, 2026-09-22).
      echo "verdicts:"
      while IFS= read -r f; do
        [ -f "$f" ] || continue
        v="$(rg -m1 '^\*\*Verdict' "$f" 2>/dev/null || true)"
        if [ -n "$v" ]; then
          echo "  $(basename "$f"): $v"
        fi
      done <<< "$local_reports"
      done_rows="$(rg -n "$id" docs/status/ 2>/dev/null | rg 'DONE|done row|\[x\]' || true)"
      if [ -n "$done_rows" ]; then
        echo "DONE-row state:"
        echo "$done_rows" | sed 's/^/  /'
      else
        echo "(no DONE rows mention this ID — repeat dispatch possible)"
      fi
    fi
    # The queue record (status / attempts / lastError tail / facts) — the
    # tq-show turn-1 gap recurred even after the report documenting it was
    # read, so it is mechanical now (07-12 report §d2/§f1).
    if command -v tq >/dev/null 2>&1; then
      echo "queue record (tq show $id, first 40 lines):"
      tq show "$id" 2>&1 | head -40 | sed 's/^/  /'
    else
      echo "(tq not on PATH — queue record skipped)"
    fi
  done
else
  echo "(pass task IDs: scripts/session-start.sh <task-id>…; rg docs/status/ skipped)"
fi

echo
echo "=== status index tail (docs/status/README.md, last 10 lines) ==="
# Sibling close-outs carry the stop-artifact / §d remedy patterns; the
# zero-artifact-stop class recurred because only the SAME-ID report got
# read (04-46 §e1).
tail -10 docs/status/README.md 2>/dev/null || echo "(no status index)"

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
