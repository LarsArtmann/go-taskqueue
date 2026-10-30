#!/usr/bin/env bash
# erraudit HARD gate (owner ruling 2026-10-27, wiring ruled 2026-10-28):
# `erraudit lint ./... --max --disable generic_return --disable legacy_is`
# from EVERY module root. `lint` REJECTS --modules/--violations-only — the
# root hint naming `--modules=all` is wrong for `lint`, never switch flags.
# Rule set context (AGENTS.md §Known Issues): generic_return stays disabled
# until the per-function error-type migration lands (~197 sites, ADOPT
# ruling); legacy_is stays disabled because `--max` implies --no-suppress
# and the sentinel sites legitimately keep errors.Is +
# //nolint:legacyerrors; legacy_as must stay ZERO (migrated 2026-10-27).
#
# Countable-gate contract (fleet convention 2026-10-24): the summary is
# pinned, not advisory — `--self-test` pins the format AND a negative-path
# battery via the TQ_ERRAUDIT_BIN env hook (a failing fake must FAIL the
# gate naming the module).
#
#   scripts/check-erraudit.sh            # exit 0 = all modules clean
#   scripts/check-erraudit.sh --self-test
set -euo pipefail
cd "$(dirname "$0")/.."

# check_one_module <dir>: run the ruled lint shape in one module root.
# TQ_ERRAUDIT_BIN is read per call so the self-test's fakes can hook it.
check_one_module() {
	local dir="$1"
	local bin="${TQ_ERRAUDIT_BIN:-erraudit}"
	if ! command -v "$bin" >/dev/null 2>&1; then
		echo "FAIL: $bin not on PATH — install erraudit (github.com/larsartmann/erraudit) or point TQ_ERRAUDIT_BIN at it" >&2
		return 1
	fi
	local out rc findings
	rc=0
	out="$(cd "$dir" && "$bin" lint ./... --max --disable generic_return --disable legacy_is 2>&1)" || rc=$?
	if [ "$rc" -ne 0 ]; then
		echo "FAIL: erraudit $dir" >&2
		echo "$out" | head -20 >&2
		return 1
	fi
	# rc 0 with real findings still means the rule set drifted off zero.
	# The multi-module "nested go.mod" HINT lines are not findings — a
	# finding line carries the `: [rule]` shape.
	findings="$(printf '%s\n' "$out" | grep -E ': \[[a-z_]+\]' || true)"
	if [ -n "$findings" ]; then
		echo "FAIL: erraudit $dir emitted findings under the ruled disable set:" >&2
		printf '%s\n' "$findings" | head -20 >&2
		return 1
	fi
	echo "ok: erraudit $dir clean"
}

self_test() {
	local tmp rc out
	tmp="$(mktemp -d)"
	# shellcheck disable=SC2064  # expand NOW: RETURN trap runs in caller scope
	trap "rm -rf '$tmp'" RETURN

	# Fake erraudit: rc 1 with a violation line (the negative path).
	cat >"$tmp/erraudit-fail" <<'SCRIPT'
#!/usr/bin/env bash
echo "main.go:10:5: [legacy_as] use errors.AsType instead of errors.As"
exit 1
SCRIPT
	chmod +x "$tmp/erraudit-fail"

	# expect-branch contract: the FAIL must NAME the module, not just rc.
	out=""
	rc=0
	out="$(TQ_ERRAUDIT_BIN="$tmp/erraudit-fail" check_one_module internal/task 2>&1)" || rc=$?
	if [ "$rc" -eq 0 ] || [[ "$out" != *"FAIL: erraudit internal/task"* ]]; then
		echo "SELF-TEST FAIL: violating module not detected/named (rc=$rc, out: $out)" >&2
		return 1
	fi

	# rc 0 WITH findings is also a fail (the silent-drift path).
	cat >"$tmp/erraudit-quiet" <<'SCRIPT'
#!/usr/bin/env bash
echo "main.go:12:3: [generic_return] returned error type is not named"
exit 0
SCRIPT
	chmod +x "$tmp/erraudit-quiet"
	rc=0
	out="$(TQ_ERRAUDIT_BIN="$tmp/erraudit-quiet" check_one_module internal/task 2>&1)" || rc=$?
	if [ "$rc" -eq 0 ] || [[ "$out" != *"emitted findings"* ]]; then
		echo "SELF-TEST FAIL: rc-0 findings not detected (rc=$rc, out: $out)" >&2
		return 1
	fi

	# A clean fake must pass with the pinned summary line.
	cat >"$tmp/erraudit-ok" <<'SCRIPT'
#!/usr/bin/env bash
exit 0
SCRIPT
	chmod +x "$tmp/erraudit-ok"
	out="$(TQ_ERRAUDIT_BIN="$tmp/erraudit-ok" check_one_module internal/task 2>&1)" || {
		echo "SELF-TEST FAIL: clean module failed (out: $out)" >&2
		return 1
	}
	if [[ "$out" != *"ok: erraudit internal/task clean"* ]]; then
		echo "SELF-TEST FAIL: summary line not pinned (out: $out)" >&2
		return 1
	fi

	echo "self-test ok: erraudit summary + module-named negative paths pinned (rc-1 fail + rc-0 findings)"
}

if [ "${1:-}" = "--self-test" ]; then
	self_test
	exit $?
fi

fail=0
clean=0
mods="$(./scripts/for-each-module.sh)
.
cmd/tq"
for m in $mods; do
	[ -f "$m/go.mod" ] || continue
	if check_one_module "$m"; then
		clean=$((clean + 1))
	else
		fail=1
	fi
done

if [ "$fail" -ne 0 ]; then
	echo "erraudit summary: FAILING — see module FAILs above" >&2
	exit 1
fi
echo "erraudit summary: $clean modules clean, 0 violations (generic_return + legacy_is disabled by ruling)"
