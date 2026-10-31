#!/usr/bin/env bash
# doctor --hygiene smoke (06-43 §f4, hardened row 609): scratch DBs seeded
# with pin tasks must surface BOTH verify-pins firing classes through the
# operator-facing `tq doctor --hygiene --json` output — the override WARN
# (.tq-verify overrides an old pin, named by task id, remedy pointed at)
# AND the WILL-FIRE class (no .tq-verify, today's auto-detected go gate
# differs) — plus a positive control (current-pin repo → verify-pins ok,
# "all matching") so the smoke cannot pass vacuously. Scratch DB everywhere
# (production-journal trap guard); scratch repos; no network, no agent
# run — doctor is read-only.
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

seed_task() {
	local repo=$1 pin=$2
	"$TMP/tq" enqueue --type agent --payload "{\"repo\":\"$repo\",\"verify\":\"$pin\"}"
}

echo "== fixture A: repo whose .tq-verify moved on after enqueue (override WARN)"
REPO_A="$TMP/repo-a"
mkdir -p "$REPO_A"
printf 'echo current-gate\n' >"$REPO_A/.tq-verify"
TASK_A="$(seed_task "$REPO_A" 'echo old-gate')"
case "$TASK_A" in
[0-9a-f]*) ;;
*)
	echo "FAIL: enqueue did not print a task id, got: $TASK_A"
	exit 1
	;;
esac
echo "== override-warn task: $TASK_A"

echo "== fixture B: go repo with NO .tq-verify, pin predates detection (WILL FIRE)"
REPO_B="$TMP/repo-b"
mkdir -p "$REPO_B"
printf 'module example.com/b\n\ngo 1.27\n' >"$REPO_B/go.mod"
TASK_B="$(seed_task "$REPO_B" 'echo old-gate')"
case "$TASK_B" in
[0-9a-f]*) ;;
*)
	echo "FAIL: enqueue did not print a task id, got: $TASK_B"
	exit 1
	;;
esac
echo "== will-fire task: $TASK_B"

echo "== fixture C: current-pin repo (positive control — must stay unflagged)"
REPO_C="$TMP/repo-c"
mkdir -p "$REPO_C"
printf 'echo matched-gate\n' >"$REPO_C/.tq-verify"
TASK_C="$(seed_task "$REPO_C" 'echo matched-gate')"
case "$TASK_C" in
[0-9a-f]*) ;;
*)
	echo "FAIL: enqueue did not print a task id, got: $TASK_C"
	exit 1
	;;
esac
echo "== positive-control task: $TASK_C"

echo "== run tq doctor --hygiene --json against the mixed scratch DB"
# The exit code is NOT asserted: the seeded PENDING tasks with no worker
# alive correctly drive the liveness check (and the overall worst) to FAIL
# on this fixture — the smoke owns the verify-pins check, not the rest.
"$TMP/tq" doctor --hygiene --json --projects-dir "$TMP" >"$TMP/doctor.json" ||
	echo "== doctor exited nonzero (expected: liveness fails on workerless pending work)"

python3 - "$TASK_A" "$TASK_B" "$TASK_C" "$TMP/doctor.json" <<'EOF'
import json, sys

task_a, task_b, task_c, path = sys.argv[1], sys.argv[2], sys.argv[3], sys.argv[4]
with open(path) as f:
    report = json.load(f)

pins = [c for c in report.get("checks", []) if c.get("name") == "verify-pins"]
if not pins:
    sys.exit("FAIL: no verify-pins check in the report (was --hygiene honored?)")
check = pins[0]

if check.get("status") != "warn":
    sys.exit("FAIL: verify-pins status = %r, want warn (stale pins must surface)" % check.get("status"))

detail = check.get("detail", "")
for needle in (
    "2 of 3 pinned task(s)",
    task_a,
    ".tq-verify currently overrides it",
    task_b,
    "STALE PIN WILL FIRE",
    "--reresolve-verify",
):
    if needle not in detail:
        sys.exit("FAIL: verify-pins detail lacks %r:\n%s" % (needle, detail))
if task_c in detail:
    sys.exit("FAIL: positive-control task must stay unflagged:\n%s" % detail)

print("== verify-pins detail: " + detail)
EOF

echo "== positive control: a clean DB whose only pin matches its repo gate"
export TQ_DB="$TMP/clean.db"
TASK_OK="$(seed_task "$REPO_C" 'echo matched-gate')"
case "$TASK_OK" in
[0-9a-f]*) ;;
*)
	echo "FAIL: enqueue did not print a task id, got: $TASK_OK"
	exit 1
	;;
esac
"$TMP/tq" doctor --hygiene --json --projects-dir "$TMP" >"$TMP/doctor-clean.json" ||
	echo "== doctor exited nonzero (expected: liveness fails on workerless pending work)"

python3 - "$TMP/doctor-clean.json" <<'EOF'
import json, sys

with open(sys.argv[1]) as f:
    report = json.load(f)

pins = [c for c in report.get("checks", []) if c.get("name") == "verify-pins"]
if not pins:
    sys.exit("FAIL: no verify-pins check in the clean report")
check = pins[0]
if check.get("status") != "ok":
    sys.exit("FAIL: clean verify-pins status = %r, want ok (matching pin must not warn): %s"
		% (check.get("status"), check.get("detail")))
if "all matching" not in check.get("detail", ""):
    sys.exit("FAIL: clean verify-pins detail lacks 'all matching': %s" % check.get("detail"))

print("== clean verify-pins: " + check.get("detail", ""))
EOF

echo "PASS: doctor --hygiene smoke (override WARN + STALE PIN WILL FIRE surfaced via --json; positive control ok)"
