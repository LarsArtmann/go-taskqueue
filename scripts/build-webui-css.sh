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

TC_DIR="$(go list -m -f '{{.Dir}}' github.com/larsartmann/templ-components)"
REPO="$(pwd)"
OUT="internal/webui/static/app.css"
ENTRY="$(mktemp /tmp/tq-webui-app-XXXXXX.css)"
trap 'rm -f "$ENTRY"' EXIT

{
  echo '@import "tailwindcss" source(none);'
  echo "@source \"$REPO/internal/webui/*.templ\";"
  echo "@source \"$REPO/internal/webui/*.go\";"
  echo "@source \"$TC_DIR\";"
  echo "@import \"$REPO/internal/webui/theme.css\";"
} > "$ENTRY"

: "${TAILWINDCSS:=tailwindcss}"
"$TAILWINDCSS" --input "$ENTRY" --output "$OUT" --minify
echo "CSS compiled: $OUT"
