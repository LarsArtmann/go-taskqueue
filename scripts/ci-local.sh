#!/usr/bin/env bash
# Local replicant of the full CI pipeline (.github/workflows/ci.yml) — the
# pre-push gate. Same steps, same order, same flags as CI, plus the nix job
# on a fully tracked tree. A green run here on the tree you are about to
# push is the contract: the 2026-09-07 red-master incident happened because
# nix was checked on a different tree than the one pushed and pushes went
# out unverified. Run it right before `git push`.
set -euo pipefail
cd "$(dirname "$0")/.."

step() { printf '\n== %s\n' "$*"; }

# --- CI test job (exact ci.yml order; lint advisory exactly like CI) -------

step "vet"
go vet ./...

step "build"
go build ./...

step "windows cross-compile (build + vet)"
GOOS=windows go build ./...
GOOS=windows go vet ./...

step "tests (-race)"
go test ./... -count=1 -race -timeout 120s

step "gofmt"
unformatted="$(gofmt -l .)"
if [ -n "$unformatted" ]; then
	echo "$unformatted"
	echo "FAIL: run gofmt -w on the files above"
	exit 1
fi

# Advisory, not a gate: the ~400-finding repo baseline (wrapcheck,
# varnamelen, paralleltest, ...) is documented in AGENTS.md. Any failure
# inside this block — including golangci-lint being missing — must not
# fail the run, mirroring CI's continue-on-error on the whole step.
step "lint (advisory — CI runs continue-on-error)"
lint() {
	if command -v golangci-lint >/dev/null 2>&1; then
		golangci-lint run ./...
	else
		echo "golangci-lint not on PATH — installing the CI-pinned version (v2.13.2)"
		go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.2
		"$(go env GOPATH)/bin/golangci-lint" run ./...
	fi
}
if lint_out="$(lint 2>&1)"; then
	echo "lint: no findings"
else
	echo "$lint_out"
	echo "lint: findings or lint failure — advisory only, continuing"
fi

# CI turns findings on lines changed since the base revision into real
# annotations (scripts/lint-annotations.sh) because golangci-lint v2 emits
# no annotation commands itself. Run the same scoping here so the pre-push
# gate sees what CI would annotate.
step "lint annotations (new findings only, advisory)"
if command -v jq >/dev/null 2>&1; then
	if ! LINT_BASE="HEAD~1" ./scripts/lint-annotations.sh; then
		echo "lint-annotations failed — advisory only, continuing"
	fi
else
	echo "jq not on PATH — skipped locally (CI runners ship jq)"
fi

step "harvest-parse guard"
go test ./internal/harvest/ -run TestRepoTodoListParses -count=1

step "web UI live smoke"
./scripts/smoke/webui.sh

step "status-loop live smoke"
./scripts/smoke/status-loop.sh

step "bootstrap --install smoke"
./scripts/smoke/bootstrap-install.sh

step "doc-reference check"
./scripts/check-doc-refs.sh

step "status-index check"
./scripts/check-status-index.sh

step "TODO_LIST honesty check"
./scripts/check-todo-list.sh

step "FEATURES/ROADMAP cross-check"
./scripts/check-features-roadmap.sh

# --- CI nix job, on a fully tracked tree ------------------------------------
# Flakes only see git-tracked files, so stage everything first and then prove
# no untracked stragglers remain BEFORE nix sees the tree. Measuring nix on a
# different tree than the one pushed was the red-master root cause; staging
# makes the local nix gates see exactly the working tree being pushed.

step "stage tree for nix (git add -A)"
git add -A
strays="$(git ls-files --others --exclude-standard)"
if [ -n "$strays" ]; then
	echo "$strays"
	echo "FAIL: untracked files remain after git add -A"
	exit 1
fi

step "nix build"
nix build

step "nix flake check"
nix flake check

# A swallowed build once produced an empty-but-"successful" output path;
# the binary existing is the cheap proof the build delivered something.
test -x result/bin/tq || {
	echo "FAIL: nix build left no binary at result/bin/tq"
	exit 1
}

echo
echo "ALL CI GATES GREEN — this exact tree is what CI will see. Safe to push."
