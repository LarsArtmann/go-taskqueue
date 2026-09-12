#!/usr/bin/env bash
# Version-agreement gate (round-13 T8): the version surfaces must agree as
# ONE set — flake.nix's version attr (the single source the ldflags derive
# from) and the latest released CHANGELOG heading — closing
# docs/release/VERSION-SURFACES.md's declared coverage gap (the flake's
# version-sync check only covers attr↔binary at build time, and nothing
# covered CHANGELOG↔flake).
#
# Semantics follow the documented bump order (CHANGELOG first, then the
# flake attr): mid-cycle, the CHANGELOG's latest release section MAY be
# NEWER than the flake attr (warn), but never OLDER (fail). The
# attr↔ldflags pair must match exactly (fail).
# Exit 0 = agree, 1 = drift, 2 = usage/unparseable surfaces.
set -euo pipefail
cd "$(dirname "$0")/.."

flake_version="$(sed -n 's/^[[:space:]]*version = "\(.*\)";$/\1/p' flake.nix | head -1)"
if [[ -z "$flake_version" ]]; then
	echo "version-agreement: cannot parse flake.nix version attr" >&2
	exit 2
fi

if grep -q 'main.version=\${version}' flake.nix; then
	ldflags_version="$flake_version"
else
	ldflags_version="$(sed -n 's/.*-X main\.version=\([0-9][0-9.]*\).*/\1/p' flake.nix | head -1)"
	if [[ -z "$ldflags_version" ]]; then
		echo "version-agreement: flake.nix has no main.version ldflags literal and does not derive from \${version} — restore the derivation" >&2
		exit 2
	fi
fi

changelog_version="$(grep -m1 -E '^## \[v?[0-9]+\.[0-9]+\.[0-9]+\]' CHANGELOG.md | sed -E 's/^## \[v?([0-9.]+)\].*/\1/')"
if [[ -z "$changelog_version" ]]; then
	echo "version-agreement: cannot parse the latest release heading in CHANGELOG.md" >&2
	exit 2
fi

status=0

if [[ "$flake_version" != "$ldflags_version" ]]; then
	echo "version-agreement: FAIL flake.nix attr ($flake_version) != ldflags version ($ldflags_version)" >&2
	status=1
fi

older="$(printf '%s\n%s\n' "$changelog_version" "$flake_version" | sort -V | head -1)"
if [[ "$older" == "$changelog_version" && "$changelog_version" != "$flake_version" ]]; then
	echo "version-agreement: FAIL CHANGELOG latest release ($changelog_version) is OLDER than the flake attr ($flake_version) — the CHANGELOG entry was never written" >&2
	status=1
elif [[ "$changelog_version" != "$flake_version" ]]; then
	echo "version-agreement: warn CHANGELOG latest release ($changelog_version) is newer than the flake attr ($flake_version) — mid-cycle bump; the flake attr moves at release time" >&2
fi

if [[ "$status" -eq 0 ]]; then
	echo "version-agreement: ok (flake $flake_version = ldflags $ldflags_version; CHANGELOG latest release $changelog_version)"
fi

exit "$status"
