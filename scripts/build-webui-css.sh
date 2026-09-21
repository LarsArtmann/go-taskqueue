#!/usr/bin/env bash
# Compile the web UI's Tailwind v4 stylesheet into the committed static asset.
#
# The entry point is generated into a temp file because the @source lines
# must point at the templ-components module inside the local Go module
# cache (machine-specific), while everything committed under internal/webui
# stays portable. The compiled output (internal/webui/static/app.css) is
# git-tracked and go:embed'ed — same policy as the generated *_templ.go
# files. This script is a DEV step; `nix build` never runs it.
#
# Usage:
#   scripts/build-webui-css.sh
#
# The tailwindcss CLI must be on PATH (flake devShell provides it), or set
# TAILWINDCSS to its binary: TAILWINDCSS=$(nix build nixpkgs#tailwindcss_4 ...)
set -euo pipefail
cd "$(dirname "$0")/.."

# go.mod requires go >= 1.27.1; hosts pinning GOTOOLCHAIN=local on an older
# toolchain (a nix-run env) would silently fail the go list resolution below
# and fall into the vendor fallback, which never carries templates/. Honor
# an explicit override, otherwise let go pick the module's toolchain.
export GOTOOLCHAIN="${GOTOOLCHAIN:-auto}"

# Module dir resolution: plain `go list -m` resolves Dir as EMPTY when a
# (git-ignored) vendor/ directory flips go into vendor mode — pin -mod=mod
# for the cache copy, and fall back to the vendored source when the cache
# is unavailable (offline sandbox).
module_dir() {
	local dir
	dir="$(go list -m -mod=mod -f '{{.Dir}}' "$1" 2>/dev/null || true)"

	if [ -z "$dir" ] && [ -d "vendor/$1" ]; then
		dir="$(pwd)/vendor/$1"
	fi

	if [ -z "$dir" ]; then
		echo "cannot resolve module dir for $1 (no cache, no vendor/)" >&2
		exit 1
	fi

	printf '%s' "$dir"
}

TC_DIR="$(module_dir github.com/larsartmann/templ-components)"
GHD_DIR="$(module_dir github.com/larsartmann/go-health-dashboard)"
REPO="$(pwd)"
OUT="internal/webui/static/app.css"
ENTRY="$(mktemp /tmp/tq-webui-app-XXXXXX.css)"
trap 'rm -f "$ENTRY"' EXIT

{
	echo '@import "tailwindcss" source(none);'
	echo "@source \"$REPO/internal/webui/*.templ\";"
	echo "@source \"$REPO/internal/webui/*.go\";"
	echo "@source \"$TC_DIR\";"
	# The /health dashboard (go-health-dashboard, mounted 2026-09-16) renders
	# from its module-cache copy — its templ output carries the tailwind
	# utility classes the page needs.
	echo "@source \"$GHD_DIR\";"
	echo "@import \"$REPO/internal/webui/theme.css\";"
	# The library's tc-* utility classes (terminal log lines, dialog
	# animations, …) live outside Tailwind and must be imported explicitly.
	echo "@import \"$TC_DIR/templates/custom.css\";"
} >"$ENTRY"

: "${TAILWINDCSS:=tailwindcss}"
"$TAILWINDCSS" --input "$ENTRY" --output "$OUT" --minify
echo "CSS compiled: $OUT"
