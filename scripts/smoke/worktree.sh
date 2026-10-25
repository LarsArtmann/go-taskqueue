#!/usr/bin/env bash
# worktree-per-agent smoke (design rollout step 3): a scratch git repo, two
# agent tasks, --worktrees on. Asserts both tasks complete in linked
# checkouts (worktree_dir/branch recorded on the completion facts), the
# main checkout stays untouched (HEAD and clean status), and after the
# review approves and the merge/reap pass runs there are NO orphan tq/*
# branches or worktrees left. Scratch DB everywhere (production-journal
# trap guard); stub agent, no network.
set -euo pipefail
cd "$(dirname "$0")/../.."
REPO_ROOT="$(pwd)"

TMP="$(mktemp -d)"
cleanup() {
	# TQ_SMOKE_KEEP=1 keeps the scratch tree for forensics on a failure.
	if [ "${TQ_SMOKE_KEEP:-0}" = "1" ]; then
		echo "== TQ_SMOKE_KEEP=1: scratch tree left at $TMP"
		return
	fi
	rm -rf "$TMP"
}
trap cleanup EXIT

if [ -n "${TQ_BIN:-}" ]; then
	echo "== using prebuilt tq: $TQ_BIN"
	cp "$TQ_BIN" "$TMP/tq"
	chmod +x "$TMP/tq"
else
	echo "== build tq"
	"$REPO_ROOT/scripts/build-tq.sh" "$TMP/tq"
fi

export TQ_DB="$TMP/tasks.db"

echo "== scratch repo (rails committed: a worktree checkout must carry them)"
REPO="$TMP/repo"
mkdir -p "$REPO"
git -C "$REPO" init -q
git -C "$REPO" config user.email smoke@tq.local
git -C "$REPO" config user.name "tq smoke"
printf 'test -f .tq-verify\n' >"$REPO/.tq-verify"
git -C "$REPO" add -A
git -C "$REPO" commit -qm "seed"

echo "== stub agent: work turns commit inside THEIR checkout; review approves"
STUB="$TMP/stub-agent"
cat >"$STUB" <<'EOF'
#!/bin/sh
prompt=""
prev=""
for a in "$@"; do
	[ "$prev" = "--" ] && prompt="$a"
	prev="$a"
done
case "$prompt" in
	*"strict senior code reviewer"*)
		[ -n "${TQ_RESULT_FILE:-}" ] || exit 1
		printf '{"verdict":"approve","summary":"fine","findings":[]}' >"$TQ_RESULT_FILE"
		;;
	*)
		# Prove the run happens in a LINKED checkout: commit there. The
		# main checkout must never see this file. The name is prompt-unique
		# so two same-repo tasks never add/add-conflict on the auto-merge.
		wf="work-$(printf '%s' "$prompt" | tr -cd 'a-z0-9' | head -c 24).txt"
		printf 'work: %s\n' "$prompt" >"$wf"
		git add "$wf"
		git commit -qm "work: $prompt"
		;;
esac
EOF
chmod +x "$STUB"
export TQ_AGENT_BIN="$STUB"

echo "== enqueue two same-repo agent tasks"
"$TMP/tq" enqueue --type agent --project smoke --payload "{\"repo\":\"$REPO\",\"prompt\":\"task one\"}" >/dev/null
"$TMP/tq" enqueue --type agent --project smoke --payload "{\"repo\":\"$REPO\",\"prompt\":\"task two\"}" >/dev/null

run_pool() {
	local n="$1"
	shift
	timeout 120 "$TMP/tq" agent-pool --repos "$REPO" --worktrees "$@" --once --poll 50ms --task-timeout 30s >"$TMP/pool$n.log" 2>&1 ||
		{
			cat "$TMP/pool$n.log"
			exit 1
		}
}

# ONE run carries the whole arc: the review sweeper auto-merges approved
# branches within the same pool run that mints the reviews (designed
# behavior — a later --review run never sees earlier completions because
# the sweeper's watermark already consumed them), so the historical
# work-run-then-review-run shape is unobservable.
echo "== pool run: work completes in linked checkouts, reviews approve → auto-merge + reap"
run_pool 1 --review
grep -q "review: enqueued" "$TMP/pool1.log" || {
	cat "$TMP/pool1.log"
	echo "FAIL: sweeper never minted a review"
	exit 1
}

"$TMP/tq" facts --json >"$TMP/facts.json"
[ "$(grep -o '"worktree_dir"' "$TMP/facts.json" | wc -l)" -ge 2 ] || {
	echo "FAIL: want worktree_dir on both completions"
	grep worktree "$TMP/facts.json"
	exit 1
}
[ "$(grep -o '"branch": *"tq/' "$TMP/facts.json" | wc -l)" -ge 2 ] || {
	echo "FAIL: want tq/ branches on both completions"
	exit 1
}

echo "== deliveries merged through tq/* branches; main checkout clean"
[ "$(grep -c 'review: merged' "$TMP/pool1.log")" -ge 2 ] || {
	cat "$TMP/pool1.log"
	echo "FAIL: approve verdicts never merged both worktree branches"
	exit 1
}
[ "$(git -C "$REPO" rev-list --merges --count HEAD)" -ge 2 ] || {
	echo "FAIL: want >=2 merge commits carrying the deliveries"
	exit 1
}
[ -f "$REPO/.tq-verify" ] || {
	echo "FAIL: seed rails missing from the main checkout"
	exit 1
}
[ -z "$(ls "$REPO"/work-*.txt 2>/dev/null)" ] || {
	ls "$REPO"/work-*.txt
	echo "FAIL: work files on disk in the main checkout — merges are plumbing-side (commit-tree), the working dir must never see them directly"
	exit 1
}
for wf in work-taskone.txt work-tasktwo.txt; do
	git -C "$REPO" ls-tree --name-only HEAD | grep -qx "$wf" || {
		echo "FAIL: $wf missing from the merged HEAD tree"
		exit 1
	}
done
[ -z "$(git -C "$REPO" status --porcelain | grep -v '^D  work-')" ] || {
	git -C "$REPO" status --porcelain
	echo "FAIL: main checkout dirtied beyond the plumbing-merge index lag"
	exit 1
}

echo "== reap pass: a second pool run sweeps the completed-task remnants"
run_pool 2 --review

echo "== no orphan tq/* branches or worktrees left"
[ -z "$(git -C "$REPO" branch --list 'tq/*')" ] || {
	git -C "$REPO" branch --list 'tq/*'
	echo "FAIL: orphan tq/* branches remain"
	exit 1
}
# The reap removes the linked checkouts (git worktree remove leaves the
# empty per-repo parent behind — residue, not an orphan); only task-id
# checkout dirs count.
[ -z "$(find "$TMP/.tq-worktrees" -mindepth 2 -type d 2>/dev/null)" ] || {
	find "$TMP/.tq-worktrees" -mindepth 2 -type d
	echo "FAIL: orphan worktree checkouts remain"
	exit 1
}

echo "worktree smoke ok: 2 tasks ran in linked checkouts (worktree_dir + tq/ branch facts), review-approved branches auto-merged through tq/* merges and reaped clean"
