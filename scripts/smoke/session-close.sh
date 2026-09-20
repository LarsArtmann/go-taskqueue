#!/usr/bin/env bash
# End-to-end smoke of the session-close bridge (`tq session begin/close`)
# plus the live `tq crush` wrapper legs: an interactive session opens in the
# journal, its commit carries the `Crush-Session:` footer, and close
# attributes the commit and mints exactly ONE review + ONE status task — a
# second close is a replay (dedup, no new tasks). The no-id wrapper leg pins
# exit-code passthrough, live stdout forwarding, the honest no-id note, and
# a DB that is never touched. No real agent, no network.
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

echo "== assert journal facts: one session.opened + one session.closed per close (replay appends a fact, mints nothing)"
FACTS="$("$TQ" facts)"
[ "$(grep -c 'session.opened' <<<"$FACTS")" -eq 1 ] || {
	echo "FAIL: want exactly 1 session.opened fact"
	exit 1
}
[ "$(grep -c 'session.closed' <<<"$FACTS")" -eq 2 ] || {
	echo "FAIL: want exactly 2 session.closed facts (one per close)"
	exit 1
}

echo "== tq crush, live no-id path: child code preserved, nothing closed, no DB touched"
cat >"$TMP/no-id-stub" <<'EOF'
#!/bin/sh
echo "plain child output, no session marker anywhere"
exit 7
EOF
chmod +x "$TMP/no-id-stub"
NOID_DB="$TMP/no-id.db"
NOID_CODE=0
"$TQ" crush --bin "$TMP/no-id-stub" --repo "$REPO" --project smokerepo --db "$NOID_DB" \
	>"$TMP/noid.out" 2>"$TMP/noid.err" || NOID_CODE=$?
[ "$NOID_CODE" -eq 7 ] || {
	echo "FAIL: wrapper exit code $NOID_CODE, want the child's 7"
	exit 1
}
grep -q "plain child output" "$TMP/noid.out" || {
	echo "FAIL: child stdout not forwarded live"
	exit 1
}
grep -q "no session id" "$TMP/noid.err" || {
	echo "FAIL: wrapper did not report the missing session id"
	exit 1
}
[ ! -e "$NOID_DB" ] || {
	echo "FAIL: wrapper touched the DB despite having nothing to close"
	exit 1
}

echo "== tq crush, committed path: stub child commits with a footer, prints its session id; wrapper scans it, closes, mints review + status, passes the exit through"
WWRAP_SID="wrapsession0001"
WRAP_DB="$TMP/wrap.db"
cat >"$TMP/wrap-stub" <<EOF
#!/bin/sh
printf 'wrapper work\n' >"$REPO/wrap.txt"
git -C "$REPO" add -A
git -C "$REPO" commit -qm "wrapped session work

Crush-Session: $WWRAP_SID"
echo "session_id: $WWRAP_SID"
exit 5
EOF
chmod +x "$TMP/wrap-stub"
WRAP_CODE=0
"$TQ" crush --bin "$TMP/wrap-stub" --repo "$REPO" --project smokerepo --db "$WRAP_DB" \
	>"$TMP/wrap.out" 2>"$TMP/wrap.err" || WRAP_CODE=$?
[ "$WRAP_CODE" -eq 5 ] || {
	echo "FAIL: wrapper exit code $WRAP_CODE, want the child's 5"
	exit 1
}
grep -q "session_id: $WWRAP_SID" "$TMP/wrap.out" || {
	echo "FAIL: child stdout not forwarded live"
	exit 1
}
grep -q "1 attributed commit" "$TMP/wrap.out" || {
	echo "FAIL: wrapper did not attribute the footer commit (id scan)"
	exit 1
}
grep -q "review task .* enqueued" "$TMP/wrap.out" || {
	echo "FAIL: wrapper did not mint a review task"
	exit 1
}
grep -q "status task .* enqueued" "$TMP/wrap.out" || {
	echo "FAIL: wrapper did not mint a status task"
	exit 1
}
TQ_DB="$WRAP_DB" "$TQ" facts >"$TMP/wrap-facts.out"
[ "$(grep -c 'session.closed' "$TMP/wrap-facts.out")" -eq 1 ] || {
	echo "FAIL: want exactly 1 session.closed fact from the wrapper close"
	exit 1
}
for t in review status; do
	[ "$(TQ_DB="$WRAP_DB" "$TQ" tasks --type "$t" --json | grep -c '"id":')" -eq 1 ] || {
		echo "FAIL: want exactly 1 $t task from the wrapper close"
		exit 1
	}
done

echo "== session-close smoke passed"
