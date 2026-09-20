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

# 15-39 report f9/e2: a concurrent session's mid-edit state transiently breaks
# the tree-reading Go gates (the `undefined: atomic` class) and kills
# 10-minute runs with a confusing red. On failure, poll (sleep 45s, retry) up
# to 3 times before giving up; a gate still red after that fails with explicit
# context instead of looking like a real regression. Deliberately NOT wrapped:
# the nix steps (they measure the staged tree — a foreign break there is about
# to be pushed and must fail) and the smokes (each owns its own cleanup;
# extending the wrapper there is a deliberate follow-up, not drive-by). The
# env defaults exist so the loop can be exercised without sleeping.
TRANSIENT_POLL_SECS="${TRANSIENT_POLL_SECS:-45}"
TRANSIENT_MAX_POLLS="${TRANSIENT_MAX_POLLS:-3}"

with_transient_retry() {
	local label="$1"
	shift
	local polls=0
	while ! "$@"; do
		polls=$((polls + 1))
		if [ "$polls" -gt "$TRANSIENT_MAX_POLLS" ]; then
			echo "FAIL: $label still failing after $TRANSIENT_MAX_POLLS retry polls (${TRANSIENT_POLL_SECS}s apart)."
			echo "If the errors above are in files you did not edit, a concurrent edit was likely in flight — let it land (or re-run this gate on a quiet tree) before judging the work."
			return 1
		fi
		echo "WARN: $label failed — possible concurrent edit in flight; retry poll $polls/$TRANSIENT_MAX_POLLS in ${TRANSIENT_POLL_SECS}s"
		sleep "$TRANSIENT_POLL_SECS"
	done
}

step "master CI state (check-ci; CI_CHECK=off to bypass)"
./scripts/check-ci.sh

# Pin the retry helper BEFORE any gate rides on it (01-46 report f2): the
# self-test exercises the shipped bytes of with_transient_retry, so an edit
# that breaks the poll counter, the context message or argument forwarding
# fails here instead of mid-run.
step "transient-retry self-test (with_transient_retry behavior pin)"
./scripts/check-transient-retry.sh

# --- CI test job (exact ci.yml order; lint advisory exactly like CI) -------

step "vet"
with_transient_retry "vet" go vet ./...

step "build"
with_transient_retry "build" go build ./...

step "windows cross-compile (build + vet)"
with_transient_retry "windows build (root)" env GOOS=windows go build ./...
with_transient_retry "windows vet (root)" env GOOS=windows go vet ./...
# Root ./... never descends into nested modules — every sub-module needs its
# own cross-compile gate or Windows-only code could rot invisibly. The list
# is disk-derived so newly added modules are gated without editing this
# script.
mods="$(./scripts/for-each-module.sh)"
for m in $mods; do
	with_transient_retry "windows cross-compile ($m)" bash -c 'cd "$1" && GOWORK=off GOOS=windows go build ./... && GOWORK=off GOOS=windows go vet ./...' _ "$m" || exit 1
done
# cmd/tq is its own replace-free module (ADR-0017): gated through the
# devmod shim instead of the generic per-module loops above.
with_transient_retry "cmd/tq windows gate" env CMD_TQ_OS=windows ./scripts/test-cmd-tq.sh

step "tests (-race)"
with_transient_retry "tests (-race)" go test ./... -count=1 -race -timeout 120s

step "module isolation gates (GOWORK=off per sub-module)"
for m in $mods; do
	echo "== $m"
	with_transient_retry "module gate ($m)" bash -c 'cd "$1" && GOWORK=off go build ./... && GOWORK=off go vet ./... && GOWORK=off go test ./... -count=1 -timeout 120s' _ "$m" || exit 1
done
# cmd/tq module (ADR-0017) — replace-free go.mod, devmod shim gate.
with_transient_retry "cmd/tq gate" ./scripts/test-cmd-tq.sh

step "embed example builds on facade paths (adopter on-ramp rot guard)"
with_transient_retry "embed example (facade paths)" bash -c 'cd examples/embed && GOWORK=off go build ./... && GOWORK=off go vet ./...' || exit 1

step "go.mod hygiene (replaces, pins, toolchain alignment, mod verify)"
./scripts/check-go-mods.sh

step "facade parity (ADR-0016: facades mirror internal exports)"
./scripts/check-facade-parity.sh

step "dead-export audit (advisory report)"
./scripts/check-dead-exports.sh

step "rename-hygiene scan (advisory; quoted literals shadowing removed identifiers)"
./scripts/check-rename-hygiene.sh

step "script syntax gate (bash -n + shellcheck, zero findings)"
./scripts/check-script-syntax.sh

step "gosec self-test (version branches + Files:0 parse pin, canned stubs)"
./scripts/check-gosec.sh --self-test

step "gosec post-config gate (pinned v2.29.0, triage-encoded excludes, Files>0 asserted)"
./scripts/check-gosec.sh

step "gofmt"
# vendor/ is regenerated by `go mod vendor` and upstream deps carry their
# own (non-gofmt) style — never gate, never reformat it. `|| true`: grep
# exits 1 on zero matches and set -e would kill the step on success.
unformatted="$(gofmt -l . | grep -v '^vendor/' || true)"
if [ -n "$unformatted" ]; then
	echo "$unformatted"
	echo "FAIL: run gofmt -w on the files above"
	exit 1
fi

# Gate, not advisory: a schema-invalid key in .golangci.yml silently
# disables linter settings (07-39 report f3) — config verify catches that
# before the advisory runs pretend the policy applied.
step "lint config verify (schema-invalid keys are silent)"
if command -v golangci-lint >/dev/null 2>&1; then
	golangci-lint config verify
else
	"$(go env GOPATH)/bin/golangci-lint" config verify
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
		# cmd/tq (ADR-0017) through the devmod + devwork shims: golangci-lint
		# rejects -modfile in its internal env probes, so the tooling run goes
		# through the derived go.work instead (the lib owns both shims).
		(
			source scripts/lib/cmd-tq-devmod.sh
			cmdtq_devmod
			cmdtq_devwork
			trap cmdtq_devmod_cleanup EXIT
			cd cmd/tq
			GOWORK="$CMD_TQ_WORK" golangci-lint run ./...
		)
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
with_transient_retry "harvest-parse guard" go test ./internal/harvest/ -run TestRepoTodoListParses -count=1

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

step "fullcore embed smoke (sqlite, scratch TQ_DB)"
./scripts/smoke/fullcore.sh

step "help-text smoke (no parenthesized-identifier artifacts in tq help)"
./scripts/smoke/help-text.sh

step "multi-repo live smoke (two pools, one DB, per-project exclusivity)"
./scripts/smoke/multi-repo.sh

step "papdashboard bridge e2e smoke (stub dashboard; alert raise + rescue resolve)"
./scripts/smoke/papdashboard-e2e.sh

step "papdashboard questions e2e smoke (ask -> forward -> answer -> unblock -> resume)"
./scripts/smoke/questions-e2e.sh

step "rate-limit e2e smoke (429 parks the task without burning an attempt)"
./scripts/smoke/ratelimit-e2e.sh

step "review-loop e2e smoke (stub reviewer; approve + request_changes + autofix)"
./scripts/smoke/reviews.sh

step "session-close bridge smoke (begin → footer commit → close → replay-safe second close)"
./scripts/smoke/session-close.sh

# Advisory (2026-09-14 O5 ruling): journal-drift audit smoke over a seeded
# fixture — reported, never a hard gate on task state.
step "journal-drift audit smoke (advisory)"
if ./scripts/smoke/journal-drift.sh; then
	echo "journal-drift smoke: PASS"
else
	echo "WARNING: journal-drift audit smoke failed (advisory — not gating)"
fi

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

# Dead-SHA citation gate (task 000001a0bcc86662ffff2bfdefc2ec7ee49f):
# a real-but-unreachable commit SHA cited in the living docs fails the gate
# unless baselined (scripts/dead-sha-baseline.txt) or recorded as old→new.
step "dead-sha citation check"
./scripts/check-dead-sha-refs.sh

step "dead-sha citation self-test (gate semantics pin)"
./scripts/check-dead-sha-refs.sh --self-test

# Round-13 T16: RELEASE.md's cited modes/gates/mechanisms pinned to
# scripts/release.sh reality (06-55 f3/e1 drift class).
step "release-doc drift check"
./scripts/check-release-docs.sh

step "status-index check"
./scripts/check-status-index.sh

step "status-index self-test (live-row counter pin)"
./scripts/check-status-index.sh --self-test

# Orphaned-guard audit (15-39 report c7/f5, e1): every check-*/smoke script
# must be wired (this file, ci.yml, or flake.nix) — the check-webui-css
# lesson generalized into a gate.
step "guard-wiring check (no orphaned check-*/smoke scripts)"
./scripts/check-guard-wiring.sh

step "asset-archive ghost check"
./scripts/check-ghost-archives.sh

step "TODO_LIST honesty check"
./scripts/check-todo-list.sh

step "FEATURES/ROADMAP cross-check"
./scripts/check-features-roadmap.sh

# Pareto M4: FEATURES rows citing CI run ids must cite GREEN runs — a red
# citation means the feature table claims evidence that no longer holds.
step "FEATURES CI-freshness check"
./scripts/check-features-ci.sh

# Advisory (Pareto M5): cqrs-lint over internal/journal/cqrs, the ADR-0014
# go-cqrs-lite seam (.cqrs-lint.json pins the read-only/library intent).
# Provenance: cmd/cqrs-lint in the OWNER-LOCAL go-cqrs-lite checkout — not
# a flake input, so the step SKIPs without it. NON-BLOCKING by decision
# (TODO row, 08-00 §c1 / 08-21 §c1): a hard-gate flip needs (a) a soak
# window of clean CI runs and (b) a hermetic tool source (flake input or
# nix package) — both are separate rulings, do not flip silently here.
step "cqrs-lint (advisory)"
if [ -d "${HOME}/projects/go-cqrs-lite/cmd/cqrs-lint" ]; then
	if (cd "${HOME}/projects/go-cqrs-lite/cmd/cqrs-lint" && go build -o /tmp/cqrs-lint-bin .) &&
		/tmp/cqrs-lint-bin internal/journal/cqrs; then
		echo "cqrs-lint clean"
	else
		echo "ADVISORY (non-blocking): cqrs-lint reported findings or failed to build/run"
	fi
else
	echo "SKIP: local go-cqrs-lite checkout not found (${HOME}/projects/go-cqrs-lite)"
fi

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
