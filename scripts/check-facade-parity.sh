#!/usr/bin/env bash
# ADR-0016 facade parity gate: every exported declaration of each facaded
# internal package must carry a kind-compatible re-export in its public
# facade (and facades must not dangle). The rules live in
# scripts/facadeparity/main.go (go/parser, stdlib-only). Called from
# scripts/ci-local.sh AND .github/workflows/ci.yml — keep both call sites
# in sync. Pairs are pinned in the checker; adding a facade means
# adding its pair there in the same change (the queue/sqlite + queue/postgres
# facade pairs were retired 2026-11-23 — the drivers stay internal-path).
set -euo pipefail
cd "$(dirname "$0")/.."
go run ./scripts/facadeparity .
