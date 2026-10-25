#!/usr/bin/env bash
# Worktree-consumer allowlist guard (row 1091): WorktreeInUse and
# closeoutPending are executor/agent-pool surfaces whose OR-over-store
# semantics carry a documented audit (docs/memos/2026-10-24_executor-
# store-or-audit.md). A NEW consumer landing silently would re-open the
# stale-parked-entry class the audit closed, so the sanctioned caller set
# is PINNED here: any occurrence outside the allowlist fails the gate and
# demands a conscious allowlist edit + audit re-run.
#
# Countable-gate convention: pinned summary + --self-test battery over
# the TQ_WORKTREE_FILES env hook (colon-separated fixture files).
set -euo pipefail
cd "$(dirname "$0")/.."

self_abs="$(cd "$(dirname "$0")" && pwd)/$(basename "$0")"

# The sanctioned consumers: file:minimum-expected-count. A file listed
# here may carry the symbols; the count floor catches accidental mass
# deletions too. Everything else is a violation.
#   cmd/tq/agentpool.go   — the OR-over-store probe (audit sole caller)
#   internal/executor/agent.go — the executor surface itself (map + methods)
allowlist=(
	"cmd/tq/agentpool.go:2"
	"internal/executor/agent.go:1"
)

if [ "${1:-}" = "--self-test" ]; then
	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' EXIT
	okc=0
	failc=0
	check() {
		local desc="$1" want_rc="$2" needle="${3:-}" rc out files="$4"
		rc=0
		out="$(TQ_WORKTREE_FILES="$files" "$self_abs" 2>&1)" || rc=$?
		if [ "$rc" -ne "$want_rc" ]; then
			echo "FAIL: $desc (want rc=$want_rc, got rc=$rc)"; printf '%s\n' "$out" | sed 's/^/    /'
			failc=$((failc + 1))
			return
		fi
		if [ -n "$needle" ] && ! grep -qF -- "$needle" <<<"$out"; then
			echo "FAIL: $desc (missing: $needle)"; printf '%s\n' "$out" | sed 's/^/    /'
			failc=$((failc + 1))
			return
		fi
		echo "ok: $desc (rc=$rc)"
		okc=$((okc + 1))
	}

	check "real tree green" 0 "worktree-consumers summary: 2 sanctioned file(s), 0 violations" ""

	printf 'package x\n\nvar _ = WorktreeInUse("t-1")\n' >"$tmp/stranger.go"
	check "new consumer fails" 1 "UNPINNED-CONSUMER: $tmp/stranger.go" "$tmp/stranger.go"

	printf 'package x\n' >"$tmp/clean.go"
	check "clean non-consumer file passes" 0 "worktree-consumers summary: 0 sanctioned file(s), 0 violations" "$tmp/clean.go"

	echo "worktree-consumers self-test ok ($okc ok / $failc fail) — allowlist pin + new-consumer refusal"
	[ "$failc" -eq 0 ] || exit 1
	exit 0
fi

violations=0
sanctioned=0
declare -A hits

if [ -n "${TQ_WORKTREE_FILES:-}" ]; then
	IFS=':' read -ra files <<<"${TQ_WORKTREE_FILES:-}"
else
	mapfile -t files < <(git ls-files '*.go' | grep -v '_test\.go$')
fi

for f in "${files[@]}"; do
	[ -f "$f" ] || continue
	n=$(grep -cE 'WorktreeInUse|closeoutPending' "$f" || true)
	[ "$n" -gt 0 ] || continue
	hits["$f"]=$n
done

for f in "${!hits[@]}"; do
	ok=0
	for a in "${allowlist[@]}"; do
		af="${a%%:*}"
		floor="${a##*:}"
		if [ "$f" = "$af" ]; then
			ok=1
			sanctioned=$((sanctioned + 1))
			if [ "${hits[$f]}" -lt "$floor" ]; then
				echo "UNDER-FLOOR: $f carries ${hits[$f]} hit(s), allowlist floor is $floor — the sanctioned surface shrank; re-run the audit before shrinking the allowlist"
				violations=$((violations + 1))
			fi
			break
		fi
	done
	if [ "$ok" = "0" ]; then
		echo "UNPINNED-CONSUMER: $f (${hits[$f]} hit(s)) — add it to the allowlist ONLY after re-running the OR-over-store audit (docs/memos/2026-10-24_executor-store-or-audit.md)"
		violations=$((violations + 1))
	fi
done

echo "worktree-consumers summary: $sanctioned sanctioned file(s), $violations violations"
[ "$violations" -eq 0 ]
