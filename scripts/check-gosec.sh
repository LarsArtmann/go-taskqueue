#!/usr/bin/env bash
# One-command post-config gosec gate (round-13 T6 triage encoded as excludes).
#
# Pinned gosec v2.29.0 with the CI-encoded exclude set — any finding is a
# NEW class needing a fresh triage note (AGENTS.md "gosec advisory
# baseline"), never a silent re-exclusion.
#
# Every scan must report Files > 0 — a scanner must prove it actually
# scanned, not trust the exit code. Two known Files:0 silent-skip shapes
# are therefore hard failures: (a) a module-context scan without
# GOWORK=off (`cd internal/session && gosec ./...` returns 0/0 silently);
# (b) a root-module package path resolved outside its module. Root-module
# packages are covered by the root `./...` scan; each sub-module is scanned
# in place with GOWORK=off.
set -euo pipefail
cd "$(dirname "$0")/.."

GOSEC_VERSION=v2.29.0
GOSEC_EXCLUDES="-exclude=G104,G115,G118,G124,G202,G204,G301,G302,G304,G306,G404,G702,G703,G710"

GOSEC_BIN="${GOSEC:-}"
if [ -z "$GOSEC_BIN" ]; then
	for candidate in "$(go env GOPATH)/bin/gosec" "$(command -v gosec || true)"; do
		if [ -n "$candidate" ] && [ -x "$candidate" ]; then
			GOSEC_BIN="$candidate"
			break
		fi
	done
fi
if [ -z "$GOSEC_BIN" ]; then
	echo "gosec not found; installing pinned $GOSEC_VERSION"
	go install "github.com/securego/gosec/v2/cmd/gosec@$GOSEC_VERSION"
	GOSEC_BIN="$(go env GOPATH)/bin/gosec"
fi

scan() {
	local bin="$1" target="$2"
	local out plain files issues
	# shellcheck disable=SC2086  # GOSEC_EXCLUDES is a flag list, word-splitting intended
	out="$(GOWORK=off "$bin" $GOSEC_EXCLUDES "$target" 2>&1 || true)"
	plain="$(printf '%s' "$out" | sed 's/\x1b\[[0-9;]*m//g')"
	files="$(printf '%s\n' "$plain" | awk '/^[[:space:]]*Files[[:space:]]*:/ { print $3 }')"
	issues="$(printf '%s\n' "$plain" | awk '/^[[:space:]]*Issues[[:space:]]*:/ { print $3 }')"
	if [ -z "$files" ] || [ "$files" -eq 0 ]; then
		echo "FAIL: $target scanned 0 files (silent skip — a scanner must prove it scanned; use GOWORK=off in-module or a root-module path)"
		return 1
	elif [ "${issues:-0}" -gt 0 ]; then
		printf '%s\n' "$plain"
		echo "FAIL: $target has $issues finding(s) — a NEW gosec class: triage it and extend the AGENTS.md baseline note, never silently re-exclude"
		return 1
	fi
	echo "ok: Files=$files Issues=0"
	return 0
}

fail=0
echo "== (root) ./..."
scan "$GOSEC_BIN" ./... || fail=1
while IFS= read -r m; do
	echo "== $m"
	(
		cd "$m" && GOWORK=off scan "$GOSEC_BIN" ./...
	) || fail=1
done < <(./scripts/for-each-module.sh)

exit "$fail"
