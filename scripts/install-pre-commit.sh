#!/usr/bin/env bash
# Installs the docs-honesty pre-commit hook (round-10 T25) and the
# Task-Queue-ID commit-msg trailer guard (plan Round 11 T10): every commit
# must keep docs/status/*.md fully indexed with honest DATE rows, TODO_LIST.md
# must not gain unblocked owner-gated items, a staged app.css must be
# the minified tailwind artifact (2026-09-13 css drift guard), and a
# commit message carrying a
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
# Installed by scripts/install-pre-commit.sh — local docs-honesty guard
# + app.css drift guard (2026-09-13). Checks run sequentially: the old
# `exec A && exec B` chain never reached B (exec replaces the shell), so
# the TODO gate was dead code until now.
./scripts/check-status-index.sh || exit 1
./scripts/check-todo-list.sh || exit 1

# app.css drift guard: fires ONLY when app.css is staged. Mechanism
# decision (facade-adoption plan T6): pre-commit hook, not a daemon
# path-exclusion — the daemon's config is not repo-owned; hooks are, and
# the installer is the established write-time-guard pattern. With nix:
# the staged file must be byte-equal to what `nix run .#webui-css`
# rebuilds (on drift the rebuild lands on disk, the commit blocks —
# re-stage and retry). Without nix: a line-count heuristic (the
# 2026-09-11 incident shipped a 6,302-line unminified rebuild; the
# minified artifact is a handful of lines).
if git diff --cached --name-only --diff-filter=ACMR | grep -qx 'internal/webui/static/app.css'; then
	css=internal/webui/static/app.css
	if command -v nix >/dev/null 2>&1; then
		before="$(mktemp)"
		cp "$css" "$before"
		if ! nix run .#webui-css; then
			echo "FAIL: nix run .#webui-css failed — cannot verify the staged app.css" >&2
			exit 1
		fi
		if ! diff -q "$before" "$css" >/dev/null; then
			echo "FAIL: staged app.css is not what nix run .#webui-css produces." >&2
			echo "  The rebuilt artifact is now on disk — re-stage it and retry the commit." >&2
			exit 1
		fi
	else
		lines="$(wc -l <"$css")"
		if [ "$lines" -gt 40 ]; then
			echo "FAIL: staged app.css has $lines lines — that is an unminified rebuild." >&2
			echo "  Run nix run .#webui-css, re-stage, retry (full byte gate: scripts/check-webui-css.sh)." >&2
			exit 1
		fi
	fi
fi
exit 0
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

echo "installed $pre_commit (status-index + TODO + app.css gates) and $commit_msg (Task-Queue-ID trailer guard)"
