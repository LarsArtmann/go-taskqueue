#!/usr/bin/env bash
# T5: out-of-tree consumer smoke (09-14 retro §f5 — closes the embed
# ci.yml parity gap with a HARDER proof). Builds a scratch module OUTSIDE
# the repo that consumes the facades from the module PROXY at the latest
# released tags — the exact shape an external adopter resolves — and runs
# enqueue → claim → complete → get through queue/sqlite. This is the proxy
# twin of examples/embed (which pins replaces to the local tree); master
# between releases is fine: the tags lag by design, that is the surface
# consumers actually see.
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
	cat > main.go <<EOF
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"$MODULE/queue/sqlite"
	"$MODULE/task"
)

func main() {
	ctx := context.Background()

	dir, err := os.MkdirTemp("", "tq-consumer-*")
	if err != nil {
		panic(err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	store, err := sqlite.Open(dir + "/tq.db")
	if err != nil {
		panic(err)
	}
	defer func() { _ = store.Close() }()

	enqueued, err := store.Enqueue(ctx, task.New{Project: "consumer", Type: "sh", Payload: []byte(`"true"`)})
	if err != nil {
		panic(err)
	}

	got, err := store.Get(ctx, enqueued.ID)
	if err != nil {
		panic(err)
	}
	if got.ID != enqueued.ID {
		panic("Get returned a different task")
	}

	claimed, claim, err := store.ClaimDue(ctx, "consumer-worker", time.Minute)
	if err != nil {
		panic(err)
	}
	if claimed.ID != enqueued.ID {
		panic("claimed the wrong task")
	}

	if err := store.Complete(ctx, claimed.ID, claim, nil); err != nil {
		panic(err)
	}

	fmt.Println("CONSUMER-OK", got.ID)
}
EOF
	GOFLAGS=-mod=mod go mod init consumer >/dev/null
	go mod edit -go=1.27.1
	go get "$MODULE@latest"
	go get "$MODULE/queue/sqlite@latest"
	go mod tidy
	go run .
)
