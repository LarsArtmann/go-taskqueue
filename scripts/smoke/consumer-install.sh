#!/usr/bin/env bash
# T5: out-of-tree consumer smoke (09-14 retro §f5 — closes the embed
# ci.yml parity gap with a HARDER proof). Builds a scratch module OUTSIDE
# the repo that consumes the facades from the module PROXY at the latest
# released tags — the exact shape an external adopter resolves — and
# compiles an import of every facade (the store drivers retired from the
# public surface 2026-11-23, so the proof is compile-only). This is the
# proxy twin of examples/embed (which pins replaces to the local tree);
# master between releases is fine: the tags lag by design, that is the
# surface consumers actually see.
#
#   scripts/smoke/consumer-install.sh   # exit 0 = proxy consumer roundtrip ok
set -euo pipefail

MODULE="github.com/larsartmann/go-taskqueue"
verify_dir="$(mktemp -d)"
trap 'rm -rf "$verify_dir"' EXIT

(
	cd "$verify_dir"
	export GOWORK=off
	export GOTOOLCHAIN=auto
	# pins: verify-dir-imports-facades — main.go blank-imports executor+journal
	cat >main.go <<EOF
package main

import (
	_ "$MODULE/executor"
	_ "$MODULE/journal"
	_ "$MODULE/queue"
	_ "$MODULE/task"
	_ "$MODULE/worker"
)

func main() {}
EOF
	GOFLAGS=-mod=mod go mod init consumer >/dev/null
	go mod edit -go=1.27.1
	go get "$MODULE@latest"
	go get "$MODULE/executor@latest" "$MODULE/journal@latest" "$MODULE/queue@latest" \
		"$MODULE/task@latest" "$MODULE/worker@latest"
	go mod tidy
	go build -o /dev/null .
	echo "CONSUMER-OK: all 5 facades install + compile from the proxy"
)
