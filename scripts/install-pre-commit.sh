#!/usr/bin/env bash
# Installs the docs-honesty pre-commit hook (round-10 T25): every commit
# must keep docs/status/*.md fully indexed with honest DATE rows, and
# TODO_LIST.md must not gain unblocked owner-gated items. Catches the
# failure classes of 2026-09-08/09 at WRITE time instead of gate time.
# Idempotent; uninstall with: rm .git/hooks/pre-commit
set -euo pipefail
cd "$(dirname "$0")/.."

hook=".git/hooks/pre-commit"

cat >"$hook" <<'EOF'
#!/usr/bin/env bash
# Installed by scripts/install-pre-commit.sh — local docs-honesty guard.
exec ./scripts/check-status-index.sh && exec ./scripts/check-todo-list.sh
EOF
chmod +x "$hook"

echo "installed $hook (status-index + TODO gates run on every commit)"
