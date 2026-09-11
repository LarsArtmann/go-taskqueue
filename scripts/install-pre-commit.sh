#!/usr/bin/env bash
# Installs the docs-honesty pre-commit hook (round-10 T25) and the
# Task-Queue-ID commit-msg trailer guard (plan Round 11 T10): every commit
# must keep docs/status/*.md fully indexed with honest DATE rows, TODO_LIST.md
# must not gain unblocked owner-gated items, and a commit message carrying a
# Task-Queue-ID trailer must carry exactly ONE, well-formed — a duplicate or
# malformed footer silently corrupts the queue↔git cross-reference (the f26
# three-ID cluster is the cautionary tale). Catches the failure classes of
# 2026-09-08/09/10 at WRITE time instead of gate time.
# Idempotent; uninstall with: rm .git/hooks/pre-commit .git/hooks/commit-msg
set -euo pipefail
cd "$(dirname "$0")/.."

pre_commit=".git/hooks/pre-commit"

cat >"$pre_commit" <<'EOF'
#!/usr/bin/env bash
# Installed by scripts/install-pre-commit.sh — local docs-honesty guard.
exec ./scripts/check-status-index.sh && exec ./scripts/check-todo-list.sh
EOF
chmod +x "$pre_commit"

commit_msg=".git/hooks/commit-msg"

cat >"$commit_msg" <<'EOF'
#!/usr/bin/env bash
# Installed by scripts/install-pre-commit.sh — Task-Queue-ID trailer guard.
# The footer is the queue↔git cross-reference: EXACTLY ONE per commit, and
# it must be a verbatim queue-assigned task ID (hex, 16+ chars). Multiple
# or malformed footers make `tq facts`/`tq show` commits-per-ID ambiguous.
msg="$1"

count=$(grep -ciE '^Task-Queue-ID:' "$msg" || true)

if [ "$count" -gt 1 ]; then
	echo "FAIL: $count Task-Queue-ID trailers in the commit message — exactly one allowed" >&2
	grep -inE '^Task-Queue-ID:' "$msg" >&2
	exit 1
fi

if [ "$count" -eq 1 ]; then
	value=$(grep -iE '^Task-Queue-ID:' "$msg" | head -1 | cut -d: -f2- | tr -d '[:space:]')
	if ! echo "$value" | grep -qE '^[a-f0-9]{16,}$'; then
		echo "FAIL: Task-Queue-ID trailer value '$value' is not a queue task ID (hex, 16+ chars)" >&2
		echo "  Copy the ID VERBATIM from the task prompt (never merge or pick between IDs)." >&2
		exit 1
	fi
fi

exit 0
EOF
chmod +x "$commit_msg"

echo "installed $pre_commit (status-index + TODO gates) and $commit_msg (Task-Queue-ID trailer guard)"
