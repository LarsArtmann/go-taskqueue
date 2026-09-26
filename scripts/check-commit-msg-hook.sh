#!/usr/bin/env bash
# Durable behavior pin for the Task-Queue-ID commit-msg hook that
# scripts/install-pre-commit.sh writes (.git/hooks/commit-msg). The hook is
# machine-local and untracked; the installer heredoc is the shipped artifact,
# so — like check-transient-retry.sh — this pin sed-extracts those bytes and
# exercises them directly. Marker assertions guard the extraction itself.
#
# The placement rule (TODO_LIST row 112; 04-13 d3 / 04-31 f8; committed live
# by the 2026-09-26 05-47 window): a well-formed footer ABOVE an attribution
# block (Crush/Co-Authored-By) is invisible to git's trailer parser — the
# queue's commit-attribution channel — so the hook must reject it, and
# accept the sanctioned shape (footer as the FINAL line). Comment lines
# (editor COMMIT_EDITMSG tail) must stay ignored; trailing prose after the
# footer must reject (footer must be the last line).
set -euo pipefail
cd "$(dirname "$0")/.."

installer=scripts/install-pre-commit.sh
[ -f "$installer" ] || {
	echo "FAIL: $installer missing"
	exit 1
}

extracted="$(sed -n "/^cat >\"\$commit_msg\" <<'EOF'$/,/^EOF$/p" "$installer" | sed '1d;$d')"
[ -n "$extracted" ] || {
	echo "FAIL: extraction is empty — the sed range no longer matches the shipped hook heredoc"
	exit 1
}
fail=0
for marker in \
	'Task-Queue-ID trailer guard' \
	'exactly one allowed' \
	'not in the FINAL trailer block' \
	'git interpret-trailers --parse' \
	'hex, 16+ chars'; do
	if ! grep -qF -- "$marker" <<<"$extracted"; then
		echo "FAIL: extraction lost marker '$marker' — the sed range no longer matches the shipped hook"
		fail=1
	fi
done
if [ "$fail" -ne 0 ]; then
	exit 1
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

hook="$tmp/commit-msg"
printf '%s\n' "$extracted" >"$hook"
chmod +x "$hook"

id="00000000000000000000000000000000deadbeef"

# run_case <name> <want_rc> <message-file>
run_case() {
	local name="$1" want_rc="$2" file="$3"
	local rc=0
	"$hook" "$file" >/dev/null 2>&1 || rc=$?
	if [ "$rc" -ne "$want_rc" ]; then
		echo "FAIL: $name — want rc=$want_rc, got rc=$rc (hook output:)"
		"$hook" "$file" >&2 || true
		return 1
	fi
	echo "ok: $name (rc=$rc)"
}

# Sanctioned shape (05-47 g1): footer AFTER the attribution block, final line.
printf 'fix: subject\n\nGenerated with Crush\n\nAssisted-by: Crush:glm\n\nTask-Queue-ID: %s\n' "$id" >"$tmp/sanctioned.txt"
printf 'fix: subject\n\nTask-Queue-ID: %s\n\nGenerated with Crush\n\nAssisted-by: Crush:glm\n' "$id" >"$tmp/above-attribution.txt"
printf 'fix: subject\n\nTask-Queue-ID: %s\n' "$id" >"$tmp/plain.txt"
printf 'fix: subject, no footer\n' >"$tmp/no-footer.txt"
printf 'fix: subject\n\nTask-Queue-ID: %s\nTask-Queue-ID: %s\n' "$id" "$id" >"$tmp/two-footers.txt"
printf 'fix: subject\n\nTask-Queue-ID: not-a-task-id\n' >"$tmp/malformed.txt"
printf 'fix: subject\n\nTask-Queue-ID: %s\n\n# Please enter the commit message\n# comment lines\n' "$id" >"$tmp/comment-tail.txt"
printf 'fix: subject\n\nTask-Queue-ID: %s\n\nsome trailing prose\n' "$id" >"$tmp/trailing-prose.txt"

rc=0
run_case "footer as final line after attribution (sanctioned)" 0 "$tmp/sanctioned.txt" || rc=1
run_case "footer above attribution block (the 05-47 defect)" 1 "$tmp/above-attribution.txt" || rc=1
run_case "plain footer without attribution" 0 "$tmp/plain.txt" || rc=1
run_case "no footer (presence stays optional)" 0 "$tmp/no-footer.txt" || rc=1
run_case "two footers" 1 "$tmp/two-footers.txt" || rc=1
run_case "malformed footer value" 1 "$tmp/malformed.txt" || rc=1
run_case "footer with editor comment tail stays accepted" 0 "$tmp/comment-tail.txt" || rc=1
run_case "trailing prose after footer" 1 "$tmp/trailing-prose.txt" || rc=1

if [ "$rc" -ne 0 ]; then
	echo "commit-msg hook pin FAILED"
	exit 1
fi
echo "commit-msg hook pin ok (extraction guarded, 8 cases: placement, count, format, comment-tail, trailing-prose)"
