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

gate_gomod() {
	local gomod="${1:-go.mod}"

	local bad_replaces
	bad_replaces="$(grep '^replace' "$gomod" | grep -vE '^replace github\.com/larsartmann/go-taskqueue/internal/[a-z0-9-]+(/[a-z0-9-]+)? => \./internal/[a-z0-9-]+(/[a-z0-9-]+)?$' || true)"
	if [ -n "$bad_replaces" ]; then
		echo "$bad_replaces"
		echo "FAIL: go.mod has non-sibling replace directives — poison in published tags"
		return 1
	fi

	if grep '00010101' "$gomod"; then
		echo "FAIL: go.mod has a pseudo-version (replace-directive leak)"
		return 1
	fi

	# go install of the published module resolves the internal sub-modules
	# through the module proxy: every internal require must be a real version
	# whose subdirectory tag exists BEFORE the release tag is cut.
	local mod ver sub_tag
	while read -r mod ver; do
		[ -n "$mod" ] || continue
		sub_tag="${mod#github.com/larsartmann/go-taskqueue/}/$ver"
		if ! git rev-parse -q --verify "refs/tags/$sub_tag" >/dev/null; then
			echo "FAIL: $mod requires $ver but tag $sub_tag does not exist — cut it (git tag -a $sub_tag) before releasing"
			return 1
		fi
	done < <(grep -E '^[[:space:]]*github\.com/larsartmann/go-taskqueue/internal/[a-z0-9-]+(/[a-z0-9-]+)? v[0-9]+' "$gomod" | awk '{print $1, $2}')
}
