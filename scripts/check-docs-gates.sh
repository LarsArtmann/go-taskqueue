#!/usr/bin/env bash
# check-docs-gates.sh (2026-11-06): run the docs-gates doctrine battery
# (github.com/Artmann-Technologies/docs-gates) over this repo and fail on
# any FAIL verdict row. The battery reads .docs-gates.json at the repo root
# — this repo's tuned doctrine: audit-all + annotate are SKIPPED (corpus
# archaeology + history-locked findings; both stay sweep-only tools) and
# cadence runs as a pileup tripwire. Machine face: `docs-gates run <root>
# --json` emits exactly one JSON document on stdout; exit contract 0/1/2.
#
# Binary-version contract: the `run --json` face requires a binary NEWER
# than the released v0.1.1 tag (in-tree docs-gates; v0.1.1 lacks the JSON
# face). A stale binary fails LOUD here — its output is not a JSON verdict
# document, so it trips the non-JSON FAIL branch below — never a silent
# green.
#
# Host-tool optional: the binary lives outside the flake (go install /
# nix profile), so a missing binary is a loud SKIP, rc=0 — hermetic CI and
# fresh checkouts must not fail a docs-doctrine gate for a missing tool.
#
# TQ_DOCS_GATES_BIN  overrides the binary under test (self-test fixture hook)
# TQ_DOCS_GATES_ROOT overrides the scanned repo root (self-test fixture hook)
# Countable summary pinned via --self-test (fleet convention 01-12 §e7/§g2).
#
#   scripts/check-docs-gates.sh            # exit 0 = battery PASS (or SKIP)
#   scripts/check-docs-gates.sh --self-test
set -euo pipefail
cd "$(dirname "$0")/.."

bin="${TQ_DOCS_GATES_BIN:-docs-gates}"
root="${TQ_DOCS_GATES_ROOT:-.}"

scan() {
	if ! command -v "$bin" >/dev/null 2>&1 && [ ! -x "$bin" ]; then
		echo "docs-gates gate: SKIP — binary '$bin' not found (host-tool optional; install docs-gates or set TQ_DOCS_GATES_BIN)"
		echo "docs-gates gate: 0 gate(s), 0 PASS / 0 SKIP / 0 FAIL"
		return 0
	fi

	local rc=0 out
	out="$("$bin" run "$root" --json 2>/dev/null)" || rc=$?

	if ! jq -e '.gates' >/dev/null 2>&1 <<<"$out"; then
		echo "docs-gates gate: FAIL — battery output is not a JSON verdict document (rc=$rc; a stale binary — older than the in-tree, post-v0.1.1 'run --json' face — fails loud here, never silently green)"
		echo "docs-gates gate: 0 gate(s), 0 PASS / 0 SKIP / 1 FAIL"
		return 1
	fi

	local pass skip fail
	pass="$(jq -r '[.gates[] | select(.verdict == "PASS")] | length' <<<"$out")"
	skip="$(jq -r '[.gates[] | select(.verdict == "SKIP")] | length' <<<"$out")"
	fail="$(jq -r '[.gates[] | select(.verdict == "FAIL")] | length' <<<"$out")"

	jq -r '.gates[] | select(.verdict == "FAIL") |
		"docs-gates gate: FAIL \(.name) (rc=\(.exit_code)\(if .timed_out then ", timed_out" else "" end))"' <<<"$out"

	if [ "$rc" -ne 0 ] && [ "$fail" -eq 0 ]; then
		echo "docs-gates gate: FAIL — battery rc=$rc with 0 FAIL rows (usage/environment class; check .docs-gates.json)"
		fail=1
	fi

	echo "docs-gates gate: $((pass + skip + fail)) gate(s), $pass PASS / $skip SKIP / $fail FAIL"
	[ "$rc" -eq 0 ] && [ "$fail" -eq 0 ]
}

self_test() {
	local self=$0
	case $self in
	/*) ;;
	*) self="$PWD/${self#./}" ;;
	esac
	tmp="$(mktemp -d)"
	trap 'rm -rf "${tmp:-}"' EXIT

	# Fake binary: emits the canned battery document for `run <root> --json`
	# and exits with the canned rc file's value (0 when absent).
	mkdir -p "$tmp/bin"
	# pins: docs-gates-fake-bin-branch — stub runner feeds canned battery.json + rc
	cat >"$tmp/bin/docs-gates" <<'FAKE'
#!/usr/bin/env bash
set -euo pipefail
case "${1:-}" in
run)
	cat "$2/battery.json"
	if [ -f "$2/rc" ]; then exit "$(cat "$2/rc")"; fi
	exit 0
	;;
*)
	echo "usage: docs-gates run <root> --json" >&2
	exit 2
	;;
esac
FAKE
	chmod +x "$tmp/bin/docs-gates"

	local ok=0 failc=0
	expect() {
		local label=$1 want_rc=$2 needle=$3 rc out
		rc=0
		out="$(TQ_DOCS_GATES_BIN="$tmp/bin/docs-gates" TQ_DOCS_GATES_ROOT="$tmp/case" "$self" 2>&1)" || rc=$?
		if [ "$rc" -ne "$want_rc" ]; then
			echo "FAIL: $label (want rc=$want_rc, got rc=$rc)"
			printf '%s\n' "$out" | sed 's/^/    /'
			failc=$((failc + 1))
			return
		fi
		if [ -n "$needle" ] && ! grep -qF -- "$needle" <<<"$out"; then
			echo "FAIL: $label (missing: $needle)"
			printf '%s\n' "$out" | sed 's/^/    /'
			failc=$((failc + 1))
			return
		fi
		echo "ok: $label (rc=$rc)"
		ok=$((ok + 1))
	}

	# green battery: 2 PASS rows, rc 0 → gate green, pinned summary
	mkdir -p "$tmp/case"
	# pins: docs-gates-green-branch — all-PASS battery is green with the pinned summary
	cat >"$tmp/case/battery.json" <<'JSON'
{"root":".","pass":true,"gates":[
 {"name":"freshness","verdict":"PASS","exit_code":0,"skipped":false,"timed_out":false,"duration_ms":9,"stdout":"","stderr":""},
 {"name":"routing","verdict":"PASS","exit_code":0,"skipped":false,"timed_out":false,"duration_ms":3,"stdout":"","stderr":""}]}
JSON
	expect "green battery passes with pinned summary" 0 "docs-gates gate: 2 gate(s), 2 PASS / 0 SKIP / 0 FAIL"

	# red battery: FAIL row (timed_out) + rc 1 → gate red naming the gate
	# pins: docs-gates-red-branch — timed_out FAIL row names the gate and rc=1
	cat >"$tmp/case/battery.json" <<'JSON'
{"root":".","pass":false,"gates":[
 {"name":"freshness","verdict":"PASS","exit_code":0,"skipped":false,"timed_out":false,"duration_ms":9,"stdout":"","stderr":""},
 {"name":"cadence","verdict":"FAIL","exit_code":1,"skipped":false,"timed_out":true,"duration_ms":300000,"stdout":"","stderr":""}]}
JSON
	expect "red battery fails naming gate + timeout" 1 "docs-gates gate: FAIL cadence (rc=1, timed_out)"
	expect "red battery counts the FAIL row" 1 "docs-gates gate: 2 gate(s), 1 PASS / 0 SKIP / 1 FAIL"

	# usage/environment class: valid JSON, 0 FAIL rows, rc 2 → loud FAIL
	# pins: docs-gates-usage-branch — rc=2 with zero FAIL rows is the loud usage class
	cat >"$tmp/case/battery.json" <<'JSON'
{"root":".","pass":false,"gates":[]}
JSON
	echo 2 >"$tmp/case/rc"
	expect "battery rc=2 with 0 FAIL rows fails loud" 1 "usage/environment class"

	# malformed output → FAIL, never a silent green
	echo "docs-gates: PANIC 9" >"$tmp/case/battery.json"
	rm -f "$tmp/case/rc"
	expect "non-JSON battery output fails loud" 1 "not a JSON verdict document"

	# missing binary → loud SKIP, rc 0 (host-tool optional)
	rc=0
	out="$(TQ_DOCS_GATES_BIN="$tmp/definitely-missing" "$self" 2>&1)" || rc=$?
	if [ "$rc" -ne 0 ] || ! grep -qF "SKIP — binary" <<<"$out"; then
		echo "FAIL: missing binary must SKIP with rc=0"
		printf '%s\n' "$out" | sed 's/^/    /'
		failc=$((failc + 1))
	else
		echo "ok: missing binary SKIPs (rc=0)"
		ok=$((ok + 1))
	fi

	echo "docs-gates self-test: $ok ok / $failc fail"
	[ "$failc" -eq 0 ]
}

if [ "${1:-}" = "--self-test" ]; then
	self_test
else
	scan
fi
