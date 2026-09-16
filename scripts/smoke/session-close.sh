#!/usr/bin/env bash
# End-to-end smoke of the session-close bridge (`tq session begin/close`):
# an interactive session opens in the journal, its commit carries the
# `Crush-Session:` footer, and close attributes the commit and mints exactly
# ONE review + ONE status task — a second close is a replay (dedup, no new
# tasks). No real agent, no network.
set -euo pipefail
cd "$(dirname "$0")/../.."
REPO_ROOT="$(pwd)"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

# TQ_BIN points at a prebuilt binary (e.g. the nix-built result/bin/tq);
# unset, the script builds from source with `go build`.
if [ -n "${TQ_BIN:-}" ]; then
	echo "== using prebuilt tq: $TQ_BIN"
	cp "$TQ_BIN" "$TMP/tq"
	chmod +x "$TMP/tq"
else
	echo "== build tq"
	"$REPO_ROOT/scripts/build-tq.sh" "$TMP/tq"
fi

export TQ_DB="$TMP/tasks.db"
TQ="$TMP/tq"
SID="smokesession0001"

echo "== seed repo (two commits, the second carrying the session footer)"
REPO="$TMP/repo"
mkdir -p "$REPO"
git -C "$REPO" init -q
git -C "$REPO" config user.email smoke@tq.local
git -C "$REPO" config user.name "tq smoke"
printf 'seed\n' >"$REPO/README.md"
git -C "$REPO" add -A
git -C "$REPO" commit -qm "seed"
printf 'work\n' >"$REPO/work.txt"
git -C "$REPO" add -A
git -C "$REPO" commit -qm "did the session work

Crush-Session: $SID"

echo "== session begin"
"$TQ" session begin --id "$SID" --repo "$REPO" | tee "$TMP/begin.out"
grep -q "session $SID opened" "$TMP/begin.out" || {
	echo "FAIL: begin did not confirm the opening"
	exit 1
}

echo "== session close (attributes the footer commit, mints review + status)"
"$TQ" session close --id "$SID" --repo "$REPO" --project smokerepo --summary "did the thing" | tee "$TMP/close.out"
grep -q "1 attributed commit" "$TMP/close.out" || {
	echo "FAIL: close did not attribute the footer commit"
	exit 1
}
grep -q "review task .* enqueued" "$TMP/close.out" || {
	echo "FAIL: close did not mint a fresh review task"
	exit 1
}
grep -q "status task .* enqueued" "$TMP/close.out" || {
	echo "FAIL: close did not mint a fresh status task"
	exit 1
}

count_type() {
	"$TQ" tasks --type "$1" --json | grep -c '"id":' || true
}

echo "== assert exactly one review and one status task"
[ "$(count_type review)" -eq 1 ] || {
	echo "FAIL: want exactly 1 review task"
	exit 1
}
[ "$(count_type status)" -eq 1 ] || {
	echo "FAIL: want exactly 1 status task"
	exit 1
}

echo "== second close is replay-safe (dedup, no new tasks)"
"$TQ" session close --id "$SID" --repo "$REPO" --project smokerepo | tee "$TMP/close2.out"
grep -q "known (dedup)" "$TMP/close2.out" || {
	echo "FAIL: replay close did not report dedup"
	exit 1
}
[ "$(count_type review)" -eq 1 ] || {
	echo "FAIL: replay minted an extra review task"
	exit 1
}
[ "$(count_type status)" -eq 1 ] || {
	echo "FAIL: replay minted an extra status task"
	exit 1
}

echo "== assert journal facts: one session.opened + one session.closed"
FACTS="$("$TQ" facts)"
[ "$(grep -c 'session.opened' <<<"$FACTS")" -eq 1 ] || {
	echo "FAIL: want exactly 1 session.opened fact"
	exit 1
}
[ "$(grep -c 'session.closed' <<<"$FACTS")" -eq 1 ] || {
	echo "FAIL: want exactly 1 session.closed fact"
	exit 1
}

echo "== session-close smoke passed"
