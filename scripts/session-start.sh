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
echo "=== master CI state (scripts/check-ci.sh) ==="
# Turn-1 master-CI probe: five consecutive windows confessed the
# close-out-only skip, leaving every master-RED claim unverified (23-28
# report b2/f2). Red is signal to record in the close-out, never a stop
# condition: the ritual always completes.
ci_rc=0
bash scripts/check-ci.sh || ci_rc=$?
if [ "$ci_rc" -ne 0 ]; then
	echo "  -> carry this into the close-out: master was red at session START (predates this tree)"
fi

echo
echo "=== prior reports for current task ID(s) ==="
# Pass task IDs as arguments: scripts/session-start.sh <id> [<id>...]
# With no arguments, prints the hint instead of guessing.
if [ "$#" -gt 0 ]; then
	windows_total=0
	done_hits=0
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
			done <<<"$local_reports"
			done_rows="$(rg -n "$id" docs/status/ 2>/dev/null | rg 'DONE|done row|\[x\]' || true)"
			if [ -n "$done_rows" ]; then
				echo "DONE-row state:"
				echo "$done_rows" | sed 's/^/  /'
				done_hits=$((done_hits + 1))
			else
				echo "(no DONE rows mention this ID — repeat dispatch possible)"
			fi
			# A repeat delivery of an already-completed ticket is discoverable
			# from the footer the fix commit carried (03-14 §e3) — surface it
			# without anchor archaeology.
			footer_commits="$(git log --grep="Task-Queue-ID: $id" --oneline -5 2>/dev/null || true)"
			if [ -n "$footer_commits" ]; then
				echo "footer already carried by:"
				echo "$footer_commits" | sed 's/^/  /'
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
		windows_total=$((windows_total + 1))
	done
else
	echo "(pass task IDs: scripts/session-start.sh <task-id>…; rg docs/status/ skipped)"
fi

echo
echo "=== git-hook liveness (core.hooksPath trap) ==="
# Host-global core.hooksPath pointing at a missing dir silently disables
# every .git/hooks guard (01-26 report §a5; inert 12+ days). Print the
# verdict so windows know whether write-time guards are live.
hooks_dir="$(git config core.hooksPath || true)"
if [ -z "$hooks_dir" ]; then
	hooks_dir=".git/hooks"
fi
if [ -d "$hooks_dir" ] && [ -e "$hooks_dir/commit-msg" ] && [ -e "$hooks_dir/pre-commit" ]; then
	echo "hooks LIVE: $hooks_dir (pre-commit + commit-msg present)"
else
	echo "hooks INERT: core.hooksPath=${hooks_dir} (resolved dir, commit-msg or pre-commit missing)"
	echo "  -> every installer-written .git/hooks guard is silently disabled; re-run scripts/install-pre-commit.sh or fix core.hooksPath"
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

# Tail-truncated invocations (`| tail -20`) still surface duplicates: the
# counts ride the FINAL completion line (2026-09-22 03-05 §f).
summary="windows=$windows_total done-row-hits=$done_hits"
[ "$#" -eq 0 ] && summary="windows=0 (no IDs passed) done-row-hits=0"
echo
echo "Session-start ritual complete (SUMMARY: $summary). Read the output before editing anything."
echo "REMINDER: after every green gate re-run 'git status' — the auto-commit daemon sweeps unstaged edits footer-less mid-window."
