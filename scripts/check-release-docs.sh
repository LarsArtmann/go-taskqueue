#!/usr/bin/env bash
# Doc↔script drift smoke for docs/release/RELEASE.md (06-55 report f3/e1):
# every mechanism the release doc cites must still exist in the code it
# documents, and the core script modes must still be what the doc says.
# Cheap greps by design — this is a drift ALARM, not a parser; a failure
# means someone changed the release flow or the doc without the other.
#
#   scripts/check-release-docs.sh        # exit 0 = no drift, 1 = drift
#   scripts/check-release-docs.sh --self-test   # pin: a mutated doc copy must trip the alarm
set -euo pipefail
cd "$(dirname "$0")/.."

doc="${RELEASE_DOCS_DOC:-docs/release/RELEASE.md}"
script="scripts/release.sh"
lib="scripts/lib/release-gates.sh"
drift=0

miss() {
	echo "release-docs drift: $1" >&2
	drift=1
}

need_in_both() {
	local needle="$1" what="$2" rc_script=0 rc_lib=0
	grep -qF -- "$needle" <<<"$doc_flat" || miss "$what cited in RELEASE.md but the doc no longer mentions it: $needle"
	# Per-target misses (needle may live in release.sh OR scripts/lib/ — the
	# proxy-wait loop moved to scripts/lib/proxy-wait.sh in 058a4b37): a
	# union failure names WHERE the needle died, not one combined target.
	grep -qF -- "$needle" "$script" || rc_script=1
	if [ "$rc_script" -eq 1 ]; then
		grep -qF -- "$needle" scripts/lib/*.sh || rc_lib=1
	fi
	# Union: the needle surviving in either target keeps the smoke green;
	# both captured rcs set = it died in both, name WHERE.
	if [ "$rc_script" -eq 1 ] && [ "$rc_lib" -eq 1 ]; then
		miss "$what documented in RELEASE.md but gone from $script: $needle"
		miss "$what documented in RELEASE.md but gone from scripts/lib/: $needle"
	fi
}

# --self-test (closeout 2026-10-08 §e2): a drift smoke that only ever runs
# green can rot — a reshaped needle, a lost `drift=1` or a neutered grep
# would pass silently. So re-run THIS script against a mutated COPY of the
# doc (newline-flattened so a doc reflow can't defeat the mutation, needle
# deleted) and require the exact doc-side miss. The real doc is never
# touched; RELEASE_DOCS_DOC points the checks at the copy.
if [ "${1:-}" = "--self-test" ]; then
	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' EXIT
	tr '\n' ' ' <"$doc" | sed 's/go list -m -versions/removed-by-self-test/g' >"$tmp/RELEASE.md"
	selftest_rc=0
	selftest_out="$(RELEASE_DOCS_DOC="$tmp/RELEASE.md" bash scripts/check-release-docs.sh 2>&1)" || selftest_rc=$?
	if [ "$selftest_rc" -eq 0 ]; then
		echo "release-docs self-test FAIL: mutated doc copy stayed green — drift detection can no longer fire" >&2
		printf '%s\n' "$selftest_out" >&2
		exit 1
	fi
	# Exact-miss assertion (2026-10-12 03-19 §b1): substring presence lets a
	# neutered smoke pass by firing the right message for the wrong reason.
	# Require the FULL doc-side miss line AND no unrelated drift lines (the
	# generic summary line excluded).
	expected='release-docs drift: module-proxy wait cited in RELEASE.md but the doc no longer mentions it: go list -m -versions'
	summary='release-docs drift: fix the doc or the script — whichever moved'
	miss_lines=$(printf '%s\n' "$selftest_out" | grep '^release-docs drift: ' | grep -vxF -- "$summary" || true)
	if [ "$miss_lines" = "$expected" ]; then
		echo "release-docs self-test ok: mutated doc copy trips the expected doc-side drift miss"
		exit 0
	fi
	echo "release-docs self-test FAIL: mutated run went red for the wrong reason (expected exactly: $expected)" >&2
	printf '%s\n' "$selftest_out" >&2
	exit 1
fi

[ -f "$doc" ] || miss "missing $doc"
[ -f "$script" ] || miss "missing $script"
[ -f "$lib" ] || miss "missing $lib"

# One tr-normalized doc snapshot, built once before the pins: every
# need_in_both doc-side grep reads this copy instead of re-running tr.
doc_flat="$(tr '\n' ' ' <"$doc")"

# Modes the doc's usage block promises vs the script's parser.
grep -qF -- '--tag | --push)' "$script" || miss "release.sh no longer parses --tag/--push as documented"
grep -qF -- 'scripts/release.sh vX.Y.Z --tag' "$doc" || miss "RELEASE.md usage block lost the --tag line"
grep -qF -- 'scripts/release.sh vX.Y.Z --push' "$doc" || miss "RELEASE.md usage block lost the --push line"

# Gate library: the allowlist section names gate_gomod and claims both callers.
grep -qF -- 'gate_gomod()' "$lib" || miss "gate_gomod no longer defined in $lib"
grep -qF -- 'gate_gomod "$mod_go"' "$script" || miss "release.sh no longer runs gate_gomod on every module go.mod"
grep -qF -- 'gate_gomod' "$doc" || miss "RELEASE.md allowlist section lost the gate_gomod name"

# Mechanisms the doc cites in its phase descriptions.
# Doc pins are reflow-tolerant: the doc-side grep runs on a newline-normalized copy, so a RELEASE.md reflow can't false-fail.
need_in_both 'go list -m -versions' 'module-proxy wait'
need_in_both 'poked demand-fill: https://proxy.golang.org/' 'proxy demand-fill log line'
need_in_both 'find internal task journal queue executor worker cmd/tq -name go.mod' 'disk-derived sub-tag list'
need_in_both --prerelease 'GitHub Release prerelease flag'
need_in_both 'gh run list --commit' 'tag CI poll'
need_in_both 'go install' 'clean-room install step'

# The allowlist hardcodes the real module path (fixture-proof note).
grep -qF -- 'github.com/larsartmann/go-taskqueue' "$lib" || miss "lib lost the hardcoded real module path"
grep -qF -- 'github.com/larsartmann/go-taskqueue' "$doc" || miss "RELEASE.md lost the module-path shape"

# Companion scripts the doc sends readers to.
for companion in scripts/ci-local.sh scripts/smoke/release-gates.sh scripts/check-go-mods.sh scripts/install-pre-commit.sh; do
	[ -f "$companion" ] || miss "RELEASE.md cites $companion but it is gone"
	grep -qF -- "$companion" "$doc" || miss "$companion exists but RELEASE.md stopped citing it"
done

# tq show --commits forensics view: flag in the CLI, named in the doc.
grep -qF -- '"commits"' cmd/tq/main.go || miss "tq show --commits flag gone from cmd/tq/main.go"
grep -qF -- 'tq show <id> --commits' "$doc" || miss "RELEASE.md multi-commits section lost the tq show --commits pin"

# Doctor tag-ancestry check is cited as a warning, not a gate.
grep -qF -- 'tag-ancestry' cmd/tq/doctor.go || miss "doctor tag-ancestry check gone"
grep -qF -- 'tq doctor' "$doc" || miss "RELEASE.md lost the doctor tag-ancestry mention"

if [ "$drift" -ne 0 ]; then
	echo "release-docs drift: fix the doc or the script — whichever moved" >&2
	exit 1
fi

echo "release-docs ok: RELEASE.md claims match scripts/release.sh + lib/release-gates.sh"
