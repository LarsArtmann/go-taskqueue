#!/usr/bin/env bash
# Scaffold a new Go module go.mod in this multi-module repo (plan T12,
# ADR-0011/0016 wiring rules): the module path, the root's go directive,
# a require at the LATEST CUT TAG per named internal dependency, and the
# correct relative replace — the exact set a hand-written go.mod got wrong
# in the 2026-09-13 facade session (missing journal replace made the
# worker facade resolve a stale proxy tag and lose journal.Reprioritized).
#
#   scripts/new-module.sh internal/foo internal/task internal/journal
#   scripts/new-module.sh queue/foo internal/queue internal/task
#
# Then: cd <dir> && go mod tidy, add code, and re-run the module gates
# (the disk-derived enumerations pick the module up automatically).
# Refuses to overwrite an existing go.mod.
#
# Also appends the module's .github/dependabot.yml entry at creation —
# parity at the source, pairing with the coverage guard: the config
# drifted within one week of hand-creation (three missing modules, one
# phantom; fixed 2026-10-06), so clerical parity is not trusted. Refuses
# when an entry for the directory already exists or the config is gone.
set -euo pipefail
cd "$(dirname "$0")/.."

dir="${1:-}"
[ -n "$dir" ] || {
	echo "usage: scripts/new-module.sh <module-dir> [internal-dep...]" >&2
	exit 2
}

case "$dir" in
internal/* | task | journal | queue | queue/* | executor | worker) ;;
*)
	echo "FAIL: '$dir' is not a repo module path (internal/… or a facade dir)" >&2
	exit 1
	;;
esac

[ -f "$dir/go.mod" ] && {
	echo "FAIL: $dir/go.mod already exists" >&2
	exit 1
}

dep_file=.github/dependabot.yml
[ -f "$dep_file" ] || {
	echo "FAIL: $dep_file missing — cannot keep dependabot parity; restore it first" >&2
	exit 1
}
grep -qF "directory: /$dir" "$dep_file" && {
	echo "FAIL: $dep_file already covers /$dir — no duplicate entry" >&2
	exit 1
}

go_ver="$(awk '$1 == "go" { print $2; exit }' go.mod)"

mkdir -p "$dir"

{
	echo "module github.com/larsartmann/go-taskqueue/${dir#./}"
	echo
	echo "go $go_ver"
	shift || true
	if [ "$#" -gt 0 ]; then
		echo
		if [ "$#" -eq 1 ]; then
			dep="$1"
			latest="$(git tag --list "$dep/v*" | sort -V | tail -1)"
			[ -n "$latest" ] || {
				echo "FAIL: no cut tag for $dep — require a real version manually" >&2
				exit 1
			}
			echo "require github.com/larsartmann/go-taskqueue/$dep ${latest##*/}"
		else
			echo "require ("
			for dep in "$@"; do
				latest="$(git tag --list "$dep/v*" | sort -V | tail -1)"
				[ -n "$latest" ] || {
					echo "FAIL: no cut tag for $dep — require a real version manually" >&2
					exit 1
				}
				echo "	github.com/larsartmann/go-taskqueue/$dep ${latest##*/}"
			done
			echo ")"
		fi
		echo
		for dep in "$@"; do
			rel="$(realpath -m --relative-to="$dir" "$dep")"
			echo "replace github.com/larsartmann/go-taskqueue/$dep => $rel"
		done
	fi
} >"$dir/go.mod"

# Dependabot entry, byte-identical in shape to the hand-maintained ones
# (GitHub's parser rejects YAML anchors, so each module carries its own
# block — keep this literal in lockstep with the file's existing rows).
[ "$(tail -c1 "$dep_file" | wc -l)" = 1 ] || printf '\n' >>"$dep_file"
{
	echo "  - package-ecosystem: gomod"
	echo "    directory: /$dir"
	echo "    schedule:"
	echo "      interval: weekly"
	echo "    open-pull-requests-limit: 5"
	echo "    groups:"
	echo "      minor-and-patch:"
	echo "        update-types:"
	echo "          - minor"
	echo "          - patch"
} >>"$dep_file"

echo "scaffolded $dir/go.mod (+ $dep_file entry) — now: cd $dir && go mod tidy, then run the module gates"
