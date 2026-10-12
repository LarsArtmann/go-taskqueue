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
BASE_HEAD="$(git -C "$REPO" rev-parse HEAD)"

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
		# main checkout must never see this file.
		printf 'work: %s\n' "$prompt" >work.txt
		git add work.txt
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
	timeout 120 "$TMP/tq" agent-pool --repos "$REPO" --worktrees --review --once --poll 50ms --task-timeout 30s >"$TMP/pool$1.log" 2>&1 ||
		{
			cat "$TMP/pool$1.log"
			exit 1
		}
}

echo "== pool run 1: both work tasks complete in linked checkouts"
run_pool 1
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

echo "== main checkout untouched: same HEAD, clean status, no work.txt"
[ "$(git -C "$REPO" rev-parse HEAD)" = "$BASE_HEAD" ] || {
	echo "FAIL: main checkout HEAD moved"
	exit 1
}
[ -z "$(git -C "$REPO" status --porcelain)" ] || {
	git -C "$REPO" status --porcelain
	echo "FAIL: main checkout is dirty"
	exit 1
}
[ ! -e "$REPO/work.txt" ] || {
	echo "FAIL: work.txt leaked into the main checkout"
	exit 1
}

echo "== pool run 2: reviews approve → auto-merge + reap"
run_pool 2
grep -q "review: merged" "$TMP/pool2.log" || {
	cat "$TMP/pool2.log"
	echo "FAIL: approve verdict never merged a worktree branch"
	exit 1
}

echo "== no orphan tq/* branches or worktrees left"
[ -z "$(git -C "$REPO" branch --list 'tq/*')" ] || {
	git -C "$REPO" branch --list 'tq/*'
	echo "FAIL: orphan tq/* branches remain"
	exit 1
}
[ ! -d "$TMP/.tq-worktrees" ] || [ -z "$(ls -A "$TMP/.tq-worktrees" 2>/dev/null)" ] || {
	ls -A "$TMP/.tq-worktrees"
	echo "FAIL: orphan worktree checkouts remain"
	exit 1
}

echo "worktree smoke ok: 2 tasks ran in linked checkouts, main checkout untouched, reviews approved, branches merged and reaped"
