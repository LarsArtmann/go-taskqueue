#!/usr/bin/env bash
# Doc↔script drift smoke for docs/release/RELEASE.md (06-55 report f3/e1):
# every mechanism the release doc cites must still exist in the code it
# documents, and the core script modes must still be what the doc says.
# Cheap greps by design — this is a drift ALARM, not a parser; a failure
# means someone changed the release flow or the doc without the other.
#
#   scripts/check-release-docs.sh        # exit 0 = no drift, 1 = drift
set -euo pipefail
cd "$(dirname "$0")/.."

doc="docs/release/RELEASE.md"
script="scripts/release.sh"
lib="scripts/lib/release-gates.sh"
drift=0

miss() {
	echo "release-docs drift: $1" >&2
	drift=1
}

need_in_both() {
	local needle="$1" what="$2"
	grep -qF -- "$needle" "$doc" || miss "$what cited in RELEASE.md but the doc no longer mentions it: $needle"
	grep -qF -- "$needle" "$script" || miss "$what documented in RELEASE.md but gone from $script: $needle"
}

[ -f "$doc" ] || miss "missing $doc"
[ -f "$script" ] || miss "missing $script"
[ -f "$lib" ] || miss "missing $lib"

# Modes the doc's usage block promises vs the script's parser.
grep -qF -- '--tag | --push)' "$script" || miss "release.sh no longer parses --tag/--push as documented"
grep -qF -- 'scripts/release.sh vX.Y.Z --tag' "$doc" || miss "RELEASE.md usage block lost the --tag line"
grep -qF -- 'scripts/release.sh vX.Y.Z --push' "$doc" || miss "RELEASE.md usage block lost the --push line"

# Gate library: the allowlist section names gate_gomod and claims both callers.
grep -qF -- 'gate_gomod()' "$lib" || miss "gate_gomod no longer defined in $lib"
grep -qF -- 'gate_gomod go.mod' "$script" || miss "release.sh no longer runs gate_gomod"
grep -qF -- 'gate_gomod' "$doc" || miss "RELEASE.md allowlist section lost the gate_gomod name"

# Mechanisms the doc cites in its phase descriptions.
need_in_both 'go list -m -versions' 'module-proxy wait'
need_in_both 'find internal -name go.mod' 'disk-derived sub-tag list'
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
