#!/usr/bin/env bash
# Smoke-coverage freshness gate (row 1115): every smoke leg in
# scripts/smoke/*.sh must be named in scripts/smoke/COVERAGE.md — the
# leg → assertion index cannot rot silently when a leg is added, renamed,
# or moved without its row. The fleet table is the "which surface has no
# E2E" answer (minted 2026-09-20_06-45 §f20); an unindexed leg answers
# that question wrong.
#
# Countable-gate convention (01-12 §e7/§g2): the end-of-run summary is
# PINNED and the negative battery runs through --self-test with the
# TQ_SMOKE_COVERAGE_DIR (smoke scripts dir) and TQ_SMOKE_COVERAGE_DOC
# (index doc) env hooks.
set -euo pipefail
cd "$(dirname "$0")/.."

self_abs="$(cd "$(dirname "$0")" && pwd)/$(basename "$0")"

if [ "${1:-}" = "--self-test" ]; then
	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' EXIT
	mkdir -p "$tmp/smoke"
	okc=0
	failc=0
	check() {
		# check <desc> <want_rc> <needle-or-empty>
		local desc="$1" want_rc="$2" needle="${3:-}" rc out
		rc=0
		out="$(TQ_SMOKE_COVERAGE_DIR="$tmp/smoke" TQ_SMOKE_COVERAGE_DOC="$tmp/COVERAGE.md" "$self_abs" 2>&1)" || rc=$?
		if [ "$rc" -ne "$want_rc" ]; then
			echo "FAIL: $desc (want rc=$want_rc, got rc=$rc)"
			printf '%s\n' "$out" | sed 's/^/    /'
			failc=$((failc + 1))
			return
		fi
		if [ -n "$needle" ] && ! grep -qF -- "$needle" <<<"$out"; then
			echo "FAIL: $desc (missing: $needle)"
			printf '%s\n' "$out" | sed 's/^/    /'
			failc=$((failc + 1))
			return
		fi
		echo "ok: $desc (rc=$rc)"
		okc=$((okc + 1))
	}

	printf '# Smoke coverage index\n\n| webui.sh | dashboard |\n' >"$tmp/COVERAGE.md"
	printf '#!/usr/bin/env bash\n# A smoke leg.\n' >"$tmp/smoke/webui.sh"
	check "indexed leg passes" 0 "smoke-coverage summary: 1 leg(s) indexed, 0 unindexed"

	printf '#!/usr/bin/env bash\n# An unindexed smoke leg.\n' >"$tmp/smoke/newleg.sh"
	check "unindexed leg fails" 1 "UNINDEXED-SMOKE: newleg.sh"

	rm -f "$tmp/smoke/newleg.sh"
	rm -f "$tmp/smoke/webui.sh"
	check "empty smoke dir is a no-op green" 0 "smoke-coverage summary: 0 leg(s) indexed, 0 unindexed"

	echo "smoke-coverage self-test ok ($okc ok / $failc fail) — name-coverage floor + pinned summary"
	[ "$failc" -eq 0 ] || exit 1
	exit 0
fi

smoke_dir="${TQ_SMOKE_COVERAGE_DIR:-scripts/smoke}"
doc="${TQ_SMOKE_COVERAGE_DOC:-$smoke_dir/COVERAGE.md}"

[ -f "$doc" ] || {
	echo "MISSING: $doc — the leg→assertion index is gone"
	exit 1
}

unindexed=0
indexed=0
for f in "$smoke_dir"/*.sh; do
	[ -f "$f" ] || continue
	name="$(basename "$f")"
	if grep -qF "$name" "$doc"; then
		indexed=$((indexed + 1))
	else
		echo "UNINDEXED-SMOKE: $name (add its leg row to $doc)"
		unindexed=$((unindexed + 1))
	fi
done

echo "smoke-coverage summary: $indexed leg(s) indexed, $unindexed unindexed"
[ "$unindexed" -eq 0 ]
