#!/usr/bin/env bash
# Pre-tag clean-room consumer compile (the v0.3.0 install-regression class:
# release.sh §d1 of the 09-14 retro — the go-install breakage was caught by
# the clean-room step AFTER the tags were immutable). Builds the CLI through
# the module graph exactly the way a consumer resolves it: a scratch module
# with NO workspace, NO vendor, requiring the root module and cmd/tq, all
# repo-owned modules replaced to this checkout. Green = the consumer graph
# CLOSES and COMPILES — a missing require, a broken module split, or a
# replace-violating module fails here. Proxy listing is check-pkg-proxy.sh's
# job; the true `go install …/cmd/tq@vX.Y.Z` proxy proof stays in release.sh
# --push mode — pre-tag, the new version does not exist on the proxy yet.
#
#   scripts/check-cleanroom-install.sh   # exit 0 = consumer graph compiles
set -euo pipefail
cd "$(dirname "$0")/.."

MODULE="github.com/larsartmann/go-taskqueue"
repo="$(pwd -P)"
verify_dir="$(mktemp -d)"
trap 'rm -rf "$verify_dir"' EXIT

(
	cd "$verify_dir"
	export GOWORK=off
	export GOTOOLCHAIN=auto
	cat >main.go <<EOF
package main

import (
	"fmt"

	"$MODULE/queue/sqlite"
	"$MODULE/task"
)

func main() {
	fmt.Println(task.Pending, sqlite.Open)
}
EOF
	GOFLAGS=-mod=mod go mod init cleanroom >/dev/null
	go mod edit -go=1.27.1
	go mod edit -replace "$MODULE=$repo"
	# Replace EVERY repo-owned module locally. Mid-cycle the go.mod pins are
	# stale relative to the tree (the version sweep rides the release), so
	# mixing proxy-resolved old tags with local modules is guaranteed API
	# skew, not a defect — the pre-tag proof is the all-local graph.
	while IFS= read -r moddir; do
		sub="${moddir%/go.mod}"
		go mod edit -replace "$MODULE/$sub=$repo/$sub"
	done < <(cd "$repo" && find internal task journal queue executor worker cmd/tq -name go.mod 2>/dev/null | sort)
	go mod tidy >/dev/null
	go mod edit -require "$MODULE/cmd/tq@v0.0.0"
	go build -o /dev/null .
	GOFLAGS=-mod=mod go build -o /dev/null "$MODULE/cmd/tq"
)
echo "cleanroom: consumer graph compiles (all repo modules local; no workspace, no vendor)"
