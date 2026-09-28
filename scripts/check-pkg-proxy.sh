#!/usr/bin/env bash
# Publishability audit for the module proxy + pkg.go.dev (09-06/09-14/10-27
# reports §f: the pkg.go.dev 404 window where the proxy listed v0.3.0 for
# every facade while the docs site still 404'd for hours). Checks, for the
# root module and the seven ADR-0016 facades, that the latest root tag is:
#   1. listed by proxy.golang.org (`go list -m -versions`), and
#   2. rendered by pkg.go.dev (HTTP 200).
# Network-tolerant by design (advisory in ci-local): a proxy/curl failure
# SKIPs instead of failing — publishability is only provable where the
# network is. Exit 0 = verified or skipped, 1 = a module is missing.
#
#   scripts/check-pkg-proxy.sh [vX.Y.Z]   # default: latest vX.Y.Z tag
set -euo pipefail
cd "$(dirname "$0")/.."

MODULE="github.com/larsartmann/go-taskqueue"
version="${1:-}"
if [ -z "$version" ]; then
	version="$(git tag --list 'v[0-9]*' | sort -V | tail -1)"
fi
[ -n "$version" ] || {
	echo "pkg-proxy SKIP: no vX.Y.Z tag found" >&2
	exit 0
}

command -v go >/dev/null || {
	echo "pkg-proxy SKIP: no go on PATH" >&2
	exit 0
}

mods=("$MODULE" \
	"$MODULE/task" "$MODULE/journal" "$MODULE/queue" \
	"$MODULE/queue/sqlite" "$MODULE/queue/postgres" \
	"$MODULE/executor" "$MODULE/worker")

missing=0
skipped=0

for mod in "${mods[@]}"; do
	label="$mod@$version"
	# -mod=readonly: the root vendor/ dir would otherwise make `go list -m`
	# refuse to hit the network ("can't determine available versions").
	if ! go list -m -versions -mod=readonly "$mod" 2>/dev/null | tr ' ' '\n' | grep -qx "${version#v}" \
		&& ! go list -m -versions -mod=readonly "$mod" 2>/dev/null | tr ' ' '\n' | grep -qx "$version"; then
		# go list prints bare versions (0.3.0); some toolchains print v-prefixed.
		echo "pkg-proxy MISSING proxy listing: $label" >&2
		missing=1
		continue
	fi
	if command -v curl >/dev/null; then
		code="$(curl -sfL -o /dev/null -w '%{http_code}' "https://pkg.go.dev/$mod" 2>/dev/null || echo 000)"
		case "$code" in
		200) echo "pkg-proxy ok: $label (proxy + pkg.go.dev)" ;;
		000)
			echo "pkg-proxy SKIP pkg.go.dev (network unreachable): $label" >&2
			skipped=1
			;;
		*)
			echo "pkg-proxy MISSING pkg.go.dev render (HTTP $code): $label" >&2
			missing=1
			;;
		esac
	else
		echo "pkg-proxy ok (proxy only, curl absent): $label"
		skipped=1
	fi
done

if [ "$missing" = 1 ]; then
	echo "pkg-proxy FAIL: missing proxy/pkg.go.dev entries above (persistent 404 = investigate crawler vs proxy, 10-27 report §b1)" >&2
	exit 1
fi
[ "$skipped" = 1 ] && echo "pkg-proxy: verified with skips (see above)"
exit 0
