#!/usr/bin/env bash
# One-command post-config gosec gate (round-13 T6 triage encoded as excludes).
#
# Pinned gosec v2.29.0 with the CI-encoded exclude set — any finding is a
# NEW class needing a fresh triage note (AGENTS.md "gosec advisory
# baseline"), never a silent re-exclusion.
#
# Every scan runs FROM THE REPO ROOT via explicit package paths
# (`./internal/session/...`), never `cd <module> && gosec ./...`: the
# in-module form silently returns exit 0 with `Files: 0` (scanned nothing),
# which this gate treats as a hard failure — a scanner must prove it
# actually scanned (Files > 0), not trust the exit code.
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

targets=("./...")
while IFS= read -r m; do
	targets+=("./$m/...")
done < <(./scripts/for-each-module.sh)

fail=0
for target in "${targets[@]}"; do
	echo "== $target"
	out="$("$GOSEC_BIN" $GOSEC_EXCLUDES "$target" 2>&1 || true)"
	plain="$(printf '%s' "$out" | sed 's/\x1b\[[0-9;]*m//g')"
	files="$(printf '%s\n' "$plain" | awk '/^[[:space:]]*Files[[:space:]]*:/ { print $3 }')"
	issues="$(printf '%s\n' "$plain" | awk '/^[[:space:]]*Issues[[:space:]]*:/ { print $3 }')"
	if [ -z "$files" ] || [ "$files" -eq 0 ]; then
		echo "FAIL: $target scanned 0 files (silent skip — check the scan path, never cd into the module)"
		fail=1
	elif [ "${issues:-0}" -gt 0 ]; then
		printf '%s\n' "$plain"
		echo "FAIL: $target has $issues finding(s) — a NEW gosec class: triage it and extend the AGENTS.md baseline note, never silently re-exclude"
		fail=1
	else
		echo "ok: Files=$files Issues=0"
	fi
done

exit "$fail"
