#!/usr/bin/env bash
# doctor --hygiene smoke (06-43 §f4): a scratch DB seeded with a stale-pin
# agent task must surface the verify-pins WARN through the operator-facing
# `tq doctor --hygiene --json` output — the stale task named by id, the
# .tq-verify-override verdict stated, and the --reresolve-verify remedy
# pointed at. Scratch DB everywhere (production-journal trap guard); scratch
# repo; no network, no agent run — doctor is read-only.
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

echo "== scratch repo whose gate moved on after enqueue"
REPO="$TMP/repo"
mkdir -p "$REPO"
printf 'echo current-gate\n' >"$REPO/.tq-verify"

echo "== seed a stale-pin agent task (pinned the OLD gate; the repo now carries a NEW one)"
TASK_ID="$("$TMP/tq" enqueue --type agent \
	--payload "{\"repo\":\"$REPO\",\"verify\":\"echo old-gate\"}")"
case "$TASK_ID" in
[0-9a-f]*) ;;
*)
	echo "FAIL: enqueue did not print a task id, got: $TASK_ID"
	exit 1
	;;
esac
echo "== stale task: $TASK_ID"

echo "== run tq doctor --hygiene --json against the scratch DB"
"$TMP/tq" doctor --hygiene --json --projects-dir "$TMP" >"$TMP/doctor.json" || {
	echo "FAIL: doctor exited nonzero (a FAIL check masks the expected WARN?)"
	cat "$TMP/doctor.json"
	exit 1
}

python3 - "$TASK_ID" "$TMP/doctor.json" <<'EOF'
import json, sys

task_id, path = sys.argv[1], sys.argv[2]
with open(path) as f:
    report = json.load(f)

if report.get("status") != "warn":
    sys.exit("FAIL: doctor status = %r, want warn (the stale pin must surface as WARN)" % report.get("status"))

pins = [c for c in report.get("checks", []) if c.get("name") == "verify-pins"]
if not pins:
    sys.exit("FAIL: no verify-pins check in the report (was --hygiene honored?)")
check = pins[0]

if check.get("status") != "warn":
    sys.exit("FAIL: verify-pins status = %r, want warn" % check.get("status"))

detail = check.get("detail", "")
for needle in (
    task_id,
    "echo old-gate",
    ".tq-verify currently overrides it",
    "--reresolve-verify",
):
    if needle not in detail:
        sys.exit("FAIL: verify-pins detail lacks %r:\n%s" % (needle, detail))

print("== verify-pins detail: " + detail)
EOF

echo "PASS: doctor --hygiene smoke (stale pin surfaced as WARN via --json, remedy pointed at)"
