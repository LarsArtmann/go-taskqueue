#!/usr/bin/env bash
# Gate-ordering guard (row 1121-adjacent F24 / the fleet ORDERING row):
# the fleet convention says every countable gate's --self-test invocation
# runs AHEAD of its real-gate invocation in ci-local.sh — the dce6c12b
# inverted wiring (self-tests after gates) shipped despite the convention
# and only a paid reviewer caught it. This gate pins the ordering
# mechanically: for every check-* script invoked in ci-local.sh in BOTH
# forms, the first self-test line must precede the first real line.
#
# Self-test form: a line invoking the script with `--self-test` as an
# argument, or with a `*_SELF_TEST=1` env prefix (the check-ci /
# daemon-sweep-docs injection style). Real form: any other invocation.
#
# Countable-gate convention: pinned summary + --self-test battery over
# the TQ_GATE_ORDER_FILE env hook (a fixture standing in for ci-local.sh).
set -euo pipefail
cd "$(dirname "$0")/.."

self_abs="$(cd "$(dirname "$0")" && pwd)/$(basename "$0")"

if [ "${1:-}" = "--self-test" ]; then
	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' EXIT
	okc=0
	failc=0
	check() {
		local desc="$1" want_rc="$2" needle="${3:-}" rc out
		rc=0
		out="$(TQ_GATE_ORDER_FILE="$tmp/fake-ci-local.sh" "$self_abs" 2>&1)" || rc=$?
		if [ "$rc" -ne "$want_rc" ]; then
			echo "FAIL: $desc (want rc=$want_rc, got rc=$rc)"; printf '%s\n' "$out" | sed 's/^/    /'
			failc=$((failc + 1))
			return
		fi
		if [ -n "$needle" ] && ! grep -qF -- "$needle" <<<"$out"; then
			echo "FAIL: $desc (missing: $needle)"; printf '%s\n' "$out" | sed 's/^/    /'
			failc=$((failc + 1))
			return
		fi
		echo "ok: $desc (rc=$rc)"
		okc=$((okc + 1))
	}

	cat >"$tmp/fake-ci-local.sh" <<'EOF'
step "x self-test"
./scripts/check-x.sh --self-test
step "x real"
./scripts/check-x.sh
step "y env-prefixed self-test"
Y_SELF_TEST=1 ./scripts/check-y.sh
step "y real"
./scripts/check-y.sh
EOF
	check "ordered gates pass (arg + env-prefixed forms)" 0 "gate-ordering summary: 2 gate(s) ordered, 0 violations"

	cat >"$tmp/fake-ci-local.sh" <<'EOF'
step "x real first (the dce6c12b inversion)"
./scripts/check-x.sh
step "x self-test after"
./scripts/check-x.sh --self-test
EOF
	check "inverted wiring fails naming the gate" 1 "INVERTED-ORDER: check-x.sh"

	cat >"$tmp/fake-ci-local.sh" <<'EOF'
step "z real only (no self-test yet)"
./scripts/check-z.sh
EOF
	check "real-only gate is not a violation" 0 "gate-ordering summary: 0 gate(s) ordered, 0 violations"

	echo "gate-ordering self-test ok ($okc ok / $failc fail) — ordering property + inversion refusal + real-only tolerance"
	[ "$failc" -eq 0 ] || exit 1
	exit 0
fi

target="${TQ_GATE_ORDER_FILE:-scripts/ci-local.sh}"
[ -f "$target" ] || {
	echo "MISSING: $target"
	exit 1
}

# first_st[script]=line of first self-test-ish invocation; first_real[script]=line of first real invocation.
declare -A first_st first_real
lineno=0
while IFS= read -r line; do
	lineno=$((lineno + 1))
	script="$(printf '%s' "$line" | grep -oE 'scripts/[a-z0-9-]+\.sh' | head -1)" || true
	[ -n "$script" ] || continue
	name="$(basename "$script")"
	if printf '%s' "$line" | grep -qE '(--self-test|_SELF_TEST=1)'; then
		if [ -z "${first_st[$name]:-}" ]; then
			first_st[$name]=$lineno
		fi
	elif ! printf '%s' "$line" | grep -q -- '--'; then
		# A flag-carrying invocation (--print-floor, --emit-baseline,
		# --canaries) is a QUERY against the script, not the gate run —
		# exempt so the early floor export is not misread as a real gate.
		if [ -z "${first_real[$name]:-}" ]; then
			first_real[$name]=$lineno
		fi
	fi
done <"$target"

ordered=0
violations=0
for name in "${!first_real[@]}"; do
	st="${first_st[$name]:-}"
	[ -n "$st" ] || continue
	re="${first_real[$name]}"
	if [ "$st" -lt "$re" ]; then
		ordered=$((ordered + 1))
	else
		echo "INVERTED-ORDER: $name (self-test at line $st, real gate at line $re — the fleet convention wants the self-test AHEAD; the dce6c12b class)"
		violations=$((violations + 1))
	fi
done

echo "gate-ordering summary: $ordered gate(s) ordered, $violations violations"
[ "$violations" -eq 0 ]
