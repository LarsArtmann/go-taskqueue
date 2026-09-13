#!/usr/bin/env bash
# Local replicant of the full CI pipeline (.github/workflows/ci.yml) — the
# pre-push gate. Same steps, same order, same flags as CI, plus the nix job
# on a fully tracked tree. A green run here on the tree you are about to
# push is the contract: the 2026-09-07 red-master incident happened because
# nix was checked on a different tree than the one pushed and pushes went
# out unverified. Run it right before `git push`.
set -euo pipefail
cd "$(dirname "$0")/.."

# encoding/json/v2 needs GOEXPERIMENT=jsonv2 on go 1.26 — do NOT rely on
# ~/.config/go/env (a fresh machine, a service env or a read-only store
# symlink can drop it; that exact dependence made CI red while local was
# green, 2026-09-11). One export covers every step below, incl. the
# sub-module loops which inherit the environment.
export GOEXPERIMENT=jsonv2

step() { printf '\n== %s\n' "$*"; }

step "master CI state (check-ci; CI_CHECK=off to bypass)"
./scripts/check-ci.sh

# --- CI test job (exact ci.yml order; lint advisory exactly like CI) -------

step "vet"
go vet ./...

step "build"
go build ./...

step "windows cross-compile (build + vet)"
GOOS=windows go build ./...
GOOS=windows go vet ./...
# Root ./... never descends into nested modules — every sub-module needs its
# own cross-compile gate or Windows-only code could rot invisibly. The list
# is disk-derived so newly added modules are gated without editing this
# script.
mods="$(find internal task journal queue executor worker -name go.mod | sed 's|/go.mod$||' | sort)"
for m in $mods; do
	(cd "$m" &&
		GOWORK=off GOOS=windows go build ./... &&
		GOWORK=off GOOS=windows go vet ./...) || exit 1
done

step "tests (-race)"
go test ./... -count=1 -race -timeout 120s

step "module isolation gates (GOWORK=off per sub-module)"
for m in $mods; do
	echo "== $m"
	(cd "$m" &&
		GOWORK=off go build ./... &&
		GOWORK=off go vet ./... &&
		GOWORK=off go test ./... -count=1 -timeout 120s) || exit 1
done

step "embed example builds on facade paths (adopter on-ramp rot guard)"
(cd examples/embed && GOWORK=off go build ./... && GOWORK=off go vet ./...) || exit 1

step "go.mod hygiene (replaces, pins, toolchain alignment, mod verify)"
./scripts/check-go-mods.sh

step "facade parity (ADR-0016: facades mirror internal exports)"
./scripts/check-facade-parity.sh

step "dead-export audit (advisory report)"
./scripts/check-dead-exports.sh

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
		for m in $mods; do
			(cd "$m" && golangci-lint run ./...)
		done
	else
		# Single source for the pin: .github/workflows/ci.yml owns the
		# version; ci-local derives it so the two can never drift (M55).
		pin="$(grep -oE 'golangci-lint/v2/cmd/golangci-lint@v[0-9.]+' .github/workflows/ci.yml | head -1 | cut -d@ -f2)"
		echo "golangci-lint not on PATH — installing the CI-pinned version (${pin:-unknown})"
		go install "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@${pin}"
		"$(go env GOPATH)/bin/golangci-lint" run ./...
	fi
}
if lint_out="$(lint 2>&1)"; then
	echo "lint: no findings"
else
	echo "$lint_out"
	lint_findings="$(printf '%s\n' "$lint_out" | grep -c '\.go:[0-9][0-9]*:[0-9][0-9]*:' || true)"
	echo "lint summary: ${lint_findings:-0} findings (advisory baseline ~400, AGENTS.md) — continuing"
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

# Round-13 T5: the advisory sea is documented, but GROWTH beyond it is a
# gate failure (new findings in touched code, or a policy change that must
# regenerate the baseline deliberately). Shrink is advisory-only.
step "lint baseline gate (growth fails, shrink advisory)"
if command -v golangci-lint >/dev/null 2>&1 || [[ -x "$(go env GOPATH)/bin/golangci-lint" ]]; then
	PATH="$(go env GOPATH)/bin:$PATH" ./scripts/lint-baseline.sh --check
else
	echo "golangci-lint unavailable — baseline gate skipped (CI installs the pinned version)"
fi

step "actionlint (GitHub Actions workflows)"
if command -v actionlint >/dev/null 2>&1; then
	actionlint .github/workflows/*.yml
else
	nix shell nixpkgs#actionlint -c actionlint .github/workflows/*.yml
fi

step "line-length gate (changed lines only, 120 cols)"
./scripts/lint-lll-changed.sh

step "harvest-parse guard"
go test ./internal/harvest/ -run TestRepoTodoListParses -count=1

step "web UI live smoke"
./scripts/smoke/webui.sh

step "web UI css drift (committed app.css must equal the tailwind rebuild)"
./scripts/check-webui-css.sh

step "status-loop live smoke"
./scripts/smoke/status-loop.sh

step "dogfood-once live smoke (stub agent; TQ_DOGFOOD=1 adds the real-agent proof)"
./scripts/smoke/dogfood-once.sh

step "bootstrap --install smoke"
./scripts/smoke/bootstrap-install.sh

step "release-gates smoke (fixture go.mods, positive + negative)"
# Runner parity: no global git identity locally either. /dev/null alone is
# NOT enough — this host auto-detects identity from the passwd GECOS and
# commits anyway; user.useConfigOnly makes identity-blindness strict, so a
# dropped -c user.* flag in the smoke fails here the way it does on runners.
GIT_CONFIG_GLOBAL=/dev/null \
	GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=user.useConfigOnly GIT_CONFIG_VALUE_0=true \
	./scripts/smoke/release-gates.sh

# Round-13 T8: the version surfaces are one set (flake attr = ldflags source;
# CHANGELOG latest release never older). Red-probed 2026-09-12.
step "version-agreement gate"
./scripts/check-version-agreement.sh

step "doc-reference check"
./scripts/check-doc-refs.sh

# Round-13 T16: RELEASE.md's cited modes/gates/mechanisms pinned to
# scripts/release.sh reality (06-55 f3/e1 drift class).
step "release-doc drift check"
./scripts/check-release-docs.sh

step "status-index check"
./scripts/check-status-index.sh

step "asset-archive ghost check"
./scripts/check-ghost-archives.sh

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
