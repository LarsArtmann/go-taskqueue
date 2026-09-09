#!/usr/bin/env bash
# CSS drift guard (round-5 M20): the committed internal/webui/static/app.css
# must be exactly what `nix run .#webui-css` produces from the current
# templ sources + templ-components version. Run inside the flake (needs
# tailwindcss); CI-equivalent is the nix check treefmt gate for .templ.
set -euo pipefail
cd "$(dirname "$0")/.."

if ! command -v tailwindcss >/dev/null 2>&1 && [ -z "${IN_NIX_SHELL:-}" ]; then
	echo "tailwindcss not on PATH — enter the devShell (nix develop) or run: nix run .#webui-css" >&2
	exit 1
fi

before="$(mktemp)"
cp internal/webui/static/app.css "$before"

nix run .#webui-css

if ! diff -u "$before" internal/webui/static/app.css; then
	echo "FAIL: committed app.css drifted from the tailwind rebuild — commit the regenerated file" >&2
	exit 1
fi

echo "app.css is in sync with the templates"
