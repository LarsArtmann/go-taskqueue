#!/usr/bin/env bash
# Release gate library — the go.mod checks scripts/release.sh blocks on.
# Sourced by scripts/release.sh (real tree) and scripts/smoke/release-gates.sh
# (fixtures), so the rules can never drift from their tests.
#
# gate_gomod <path-to-go.mod>: runs from a repo root whose tags the internal
# requires resolve against. Dies on the first rule violation.
#
# Paths allow ONE nesting level (internal/queue/sqlite): round-2 split the
# store backends into nested modules and the original single-segment
# character class silently stopped matching them — the allowlist read the
# legit replace as poison and the require-tag check skipped it entirely.
#
# Facade modules (ADR-0016, top-level task/, queue/, …) replace their
# internal counterparts with ../internal/… or ../../internal/… paths, so
# the replace rule is CONTAINMENT-based: the right side must be relative
# and resolve inside the repo root from the go.mod's directory (a
# ../go-taskqueue/… climb above the root is the stranger shape and stays
# poison). The left side stays internal-only — that is the ADR-0011
# sibling-replace pattern; nothing else may be replaced in published tags.

gate_gomod() {
	local gomod="${1:-go.mod}"
	local dir
	dir="$(dirname "$gomod")"

	local bad_replaces=""
	local line target resolved
	while IFS= read -r line; do
		[ -n "$line" ] || continue

		if ! printf '%s' "$line" | grep -qE '^replace github\.com/larsartmann/go-taskqueue/internal/[a-z0-9-]+(/[a-z0-9-]+)? => '; then
			bad_replaces+="$line"$'\n'
			continue
		fi

		target="${line##* => }"

		case "$target" in
		. | .. | ./* | ../*) ;;
		*)
			bad_replaces+="$line"$'\n'
			continue
			;;
		esac

		resolved="$(realpath -m --relative-to=. "$dir/$target" 2>/dev/null || echo ESCAPED)"

		case "$resolved" in
		.. | ../* | */../* | ESCAPED)
			bad_replaces+="$line"$'\n'
			;;
		esac
	done < <(grep '^replace' "$gomod" || true)

	if [ -n "$bad_replaces" ]; then
		printf '%s' "$bad_replaces"
		echo "FAIL: $gomod has non-repo-relative replace directives — poison in published tags"
		return 1
	fi

	if grep '00010101' "$gomod"; then
		echo "FAIL: $gomod has a pseudo-version (replace-directive leak)"
		return 1
	fi

	# go install of the published module resolves the internal sub-modules
	# through the module proxy: every internal require must be a real version
	# whose subdirectory tag exists BEFORE the release tag is cut. Applies to
	# facade go.mod files the same way — their internal requires are what a
	# consumer's `go get` resolves through the proxy.
	local mod ver sub_tag
	while read -r mod ver; do
		[ -n "$mod" ] || continue
		sub_tag="${mod#github.com/larsartmann/go-taskqueue/}/$ver"
		if ! git rev-parse -q --verify "refs/tags/$sub_tag" >/dev/null; then
			echo "FAIL: $mod requires $ver but tag $sub_tag does not exist — cut it (git tag -a $sub_tag) before releasing"
			return 1
		fi
	done < <(sed 's/^require[[:space:]]\+//' "$gomod" | grep -E '^[[:space:]]*github\.com/larsartmann/go-taskqueue/internal/[a-z0-9-]+(/[a-z0-9-]+)? v[0-9]+' | awk '{print $1, $2}')
}
