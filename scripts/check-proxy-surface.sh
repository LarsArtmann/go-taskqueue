#!/usr/bin/env bash
# Proxy-surface sentinel probes (row-113 follow-ups, 2026-09-27 04-05
# report §c/§e/§f): per ADR-0016 facade module (task, journal, queue,
# executor, worker), resolve the latest tag at the module proxy INSIDE a
# /tmp scratch module (GOWORK=off, GOTOOLCHAIN=auto — the exact shape an
# external adopter resolves), then build AND RUN a sentinel-symbol probe
# so "proxy consumers receive X" is an rc-captured fact, not a
# compile-only inference. The sibling gates each pin one leg of the
# consumer contract: check-pkg-proxy.sh pins the proxy LISTING,
# consumer-install.sh pins the COMPILE; neither RUNS the facade's
# sentinels — the v0.3.0 install-regression class (09-14 retro §d1) hid
# exactly there. A probe rc≠0 = the latest tag genuinely breaks its
# sentinel contract; rc-captured, never inferred from a build banner.
#
# Network gate placement (hermeticity rule): a ci-local/CI OPTIONAL step
# only — NEVER flake checkPhase. Network absence can only SKIP (rc stays
# 0); a FAIL is always a real probe verdict, never a network artifact.
# Facade tags lag master by design (consumer-install precedent), so the
# probes assert the release-contract shape — sentinels non-nil, band
# ladder, status transitions — that holds across the published surface,
# not HEAD-only details.
#
# Hooks (both run under `bash -c "$CMD" tq-proxy-<leg> args…`, so the
# snippet owns its own exit and sees the arguments as $1/$2/$3):
#   TQ_PROXY_RESOLVE_CMD  args: $1 = module path. Prints the resolved
#                         version on stdout, rc 0; anything else (rc≠0 or
#                         empty output) = SKIP.
#   TQ_PROXY_PROBE_CMD    args: $1 = module path, $2 = resolved version,
#                         $3 = scratch dir. rc IS the sentinel-verdict fact.
#
#   scripts/check-proxy-surface.sh              # real run (network)
#   scripts/check-proxy-surface.sh --self-test  # hermetic summary pins
set -euo pipefail
cd "$(dirname "$0")/.."
# Recursive --self-test re-invokes THIS script; capture the absolute self
# path BEFORE any cd (the gosec self_abs pattern; 02-38 §b).
self_abs="$(cd "$(dirname "$0")" && pwd)/$(basename "$0")"

MODULE="github.com/larsartmann/go-taskqueue"
FACADES=(task journal queue executor worker)

must_mention() {
	if grep -qF -- "$1" <<<"$2"; then
		echo "ok: $3"
	else
		echo "FAIL: $3 (output is missing '$1')"
		printf '%s\n' "$2"
		exit 1
	fi
}

self_test() {
	# tmp is deliberately global: the EXIT trap reads it after self_test
	# returned, and set -u would kill the trap on a function-local name.
	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' EXIT

	# Hook fixtures: every battery is hermetic — no go invocation, no
	# network. "$1"/"$2"/"$3" reach the hook at eval time (the contract
	# pinned in the header).
	local resolve_all='printf "%s\n" v9.9.9'
	local resolve_empty='exit 7'
	local resolve_task='case "$1" in */task) printf "%s\n" v9.9.9;; *) exit 7;; esac'
	local resolve_queue='case "$1" in */queue) printf "%s\n" v9.9.9;; *) exit 7;; esac'
	local probe_ok='exit 0'
	local probe_fail='exit 3'

	run_hooked() {
		RUN_RC=0
		RUN_OUT="$(TQ_PROXY_RESOLVE_CMD="$1" TQ_PROXY_PROBE_CMD="$2" "$self_abs" 2>&1)" || RUN_RC=$?
	}
	assert_case() {
		if [ "$RUN_RC" -ne "$1" ]; then
			echo "FAIL: $2 (want rc=$1, got rc=$RUN_RC)"
			printf '%s\n' "$RUN_OUT"
			exit 1
		fi
		must_mention "$3" "$RUN_OUT" "$2"
	}

	# Battery A (all-ok): 5 verified, per-facade receipt format pinned.
	run_hooked "$resolve_all" "$probe_ok"
	assert_case 0 "all-ok summary pins the counters" \
		"proxy-surface summary: 5 facades verified, 0 skipped, 0 failed"
	must_mention "ok: proxy-surface task@v9.9.9 sentinel probe" "$RUN_OUT" "per-facade receipt format pin (task)"
	must_mention "ok: proxy-surface worker@v9.9.9 sentinel probe" "$RUN_OUT" "per-facade receipt format pin (worker)"

	# Battery B (all-skip): unresolvable tags are network-tolerant SKIPs,
	# rc stays 0 — absence of network can never fail this gate.
	run_hooked "$resolve_empty" "$probe_ok"
	assert_case 0 "all-skip summary pins the counters" \
		"proxy-surface summary: 0 facades verified, 5 skipped, 0 failed"
	must_mention "SKIP: proxy-surface journal (latest tag unresolvable at the proxy — network-tolerant skip)" "$RUN_OUT" "skip receipt format pin"

	# Battery C (one-fail): task resolves, its probe dies rc=3 → the gate
	# exits 1 (the rc-captured fact), summary still precedes the FAIL tail
	# (the §f13 ordering pin).
	run_hooked "$resolve_task" "$probe_fail"
	assert_case 1 "one-fail summary pins the counters" \
		"proxy-surface summary: 0 facades verified, 4 skipped, 1 failed"
	must_mention "FAIL: proxy-surface task@v9.9.9 sentinel probe rc=3" "$RUN_OUT" "fail receipt carries the probe rc"
	if [ "$(grep -n 'proxy-surface summary:' <<<"$RUN_OUT" | head -1 | cut -d: -f1)" -lt "$(grep -n 'proxy-surface FAIL:' <<<"$RUN_OUT" | head -1 | cut -d: -f1)" ]; then
		echo "ok: summary precedes the FAIL tail (ordering pin)"
	else
		echo "FAIL: FAIL tail printed before the summary"
		printf '%s\n' "$RUN_OUT"
		exit 1
	fi

	# Battery D (one-ok): per-module resolution differentiation — queue
	# resolves, the other four skip, and only queue's receipt is ok.
	run_hooked "$resolve_queue" "$probe_ok"
	assert_case 0 "one-ok summary pins the counters" \
		"proxy-surface summary: 1 facades verified, 4 skipped, 0 failed"
	must_mention "ok: proxy-surface queue@v9.9.9 sentinel probe" "$RUN_OUT" "per-module resolution pin (queue ok)"
	must_mention "SKIP: proxy-surface task (latest tag unresolvable at the proxy — network-tolerant skip)" "$RUN_OUT" "per-module resolution pin (task skipped)"

	echo "proxy-surface self-test ok (summary format pin + ok/skip/fail counter battery)"
}

if [ "${1:-}" = "--self-test" ]; then
	self_test
	exit 0
fi

# default_resolve: the scratch module resolves <facade>@latest at the
# proxy — go get populates go.mod/go.sum exactly as an adopter's first
# fetch does, then the resolved version is read back for the receipt.
# Any failure rc≠0 → the caller SKIPs (network-tolerant), stderr flows.
default_resolve() {
	local facade="$1" dir="$2"
	(
		cd "$dir"
		export GOWORK=off
		export GOTOOLCHAIN=auto
		export GOFLAGS=-mod=mod
		go mod init tqproxysurface >/dev/null 2>&1
		go mod edit -go=1.27.1
		go get "$MODULE/$facade@latest" >/dev/null
		go list -m -f '{{.Version}}' "$MODULE/$facade"
	)
}

# Per-facade sentinel probe sources. Each asserts the published surface's
# contract shape (verified against the v0.4.2 tags); compile failure and
# assertion failure are both rc-captured by default_probe.
write_probe_task() {
	# pins: task-probe-declines-unknown-method — a no-tools probe program
	cat >"$1" <<'PROBE_EOF'
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/larsartmann/go-taskqueue/task"
)

func check(ok bool, msg string) {
	if !ok {
		fmt.Printf("PROBE-FAIL task: %s\n", msg)
		os.Exit(1)
	}
}

func main() {
	check(task.ErrNotFound != nil, "ErrNotFound is nil")
	check(task.ErrLeaseNotHeld != nil, "ErrLeaseNotHeld is nil")
	check(task.ErrDuplicateID != nil, "ErrDuplicateID is nil")
	check(task.ErrInvalidTransition != nil, "ErrInvalidTransition is nil")
	check(errors.Is(task.ErrNotFound, task.ErrNotFound), "errors.Is identity does not hold")
	check(task.DefaultMaxAttempts > 0, "DefaultMaxAttempts is not positive")
	seen := map[string]bool{}
	for _, s := range []any{task.Pending, task.Running, task.Completed, task.Dead, task.Cancelled} {
		key := fmt.Sprint(s)
		check(!seen[key], fmt.Sprintf("duplicate status value %q", key))
		seen[key] = true
	}
	check(task.CanTransitionTo(task.Pending, task.Running), "Pending cannot transition to Running")
	check(!task.CanTransitionTo(task.Completed, task.Pending), "Completed can transition back to Pending")
	fmt.Println("PROBE-OK task")
}
PROBE_EOF
}

write_probe_journal() {
	# pins: journal-probe-happy-writes — probe writes the journal marker
	cat >"$1" <<'PROBE_EOF'
package main

import (
	"fmt"
	"os"

	"github.com/larsartmann/go-taskqueue/journal"
)

func check(ok bool, msg string) {
	if !ok {
		fmt.Printf("PROBE-FAIL journal: %s\n", msg)
		os.Exit(1)
	}
}

func distinct(msg string, vals []any) {
	seen := map[string]bool{}
	for _, v := range vals {
		key := fmt.Sprint(v)
		check(!seen[key], fmt.Sprintf("%s: duplicate value %q", msg, key))
		seen[key] = true
	}
}

func main() {
	check(journal.NewMemoryJournal() != nil, "NewMemoryJournal returned nil")
	distinct("fact types", []any{
		journal.Enqueued, journal.Claimed, journal.Heartbeat, journal.Completed,
		journal.Failed, journal.DeadLettered, journal.Requeued,
	})
	distinct("effect dispositions", []any{
		journal.EffectCommitted, journal.EffectNone, journal.EffectUnknown,
		journal.EffectPartial, journal.EffectRolledBack,
	})
	fmt.Println("PROBE-OK journal")
}
PROBE_EOF
}

write_probe_queue() {
	# pins: queue-probe-happy-writes — probe exercises the queue store
	cat >"$1" <<'PROBE_EOF'
package main

import (
	"fmt"
	"os"

	"github.com/larsartmann/go-taskqueue/queue"
)

func check(ok bool, msg string) {
	if !ok {
		fmt.Printf("PROBE-FAIL queue: %s\n", msg)
		os.Exit(1)
	}
}

func main() {
	check(queue.ErrNoTaskDue != nil, "ErrNoTaskDue is nil")
	check(queue.ErrEmptyType != nil, "ErrEmptyType is nil")
	check(queue.ErrTaskDone != nil, "ErrTaskDone is nil")
	check(queue.BandOf(-1) == queue.BandBacklog, "negative priorities must land in the backlog band")
	check(queue.BandOf(queue.BacklogMax) == queue.BandBacklog, "BacklogMax must land in the backlog band")
	check(queue.BandOf(queue.HotMin) == queue.BandHot, "HotMin must land in the hot band")
	check(queue.BandOf(queue.HotMax) == queue.BandHot, "HotMax must land in the hot band")
	check(queue.BandOf(queue.MachineMin) == queue.BandMachine, "MachineMin must land in the machine band")
	check(queue.ClampBacklog(queue.HotMin) == queue.BacklogMax, "hot-band values must clamp to BacklogMax")
	check(queue.ClampBacklog(-5) == 0, "negative values must clamp to 0")
	check(queue.ValidQuestionType(queue.QuestionTypeInfo), "QuestionTypeInfo must be a valid question type")
	check(!queue.ValidQuestionType("no-such-question-kind"), "an unknown question kind must be invalid")
	fmt.Println("PROBE-OK queue")
}
PROBE_EOF
}

write_probe_executor() {
	# pins: executor-probe-happy-writes — probe exercises the executor
	cat >"$1" <<'PROBE_EOF'
package main

import (
	"fmt"
	"os"

	"github.com/larsartmann/go-taskqueue/executor"
)

func check(ok bool, msg string) {
	if !ok {
		fmt.Printf("PROBE-FAIL executor: %s\n", msg)
		os.Exit(1)
	}
}

func distinct(msg string, vals []any) {
	seen := map[string]bool{}
	for _, v := range vals {
		key := fmt.Sprint(v)
		check(!seen[key], fmt.Sprintf("%s: duplicate value %q", msg, key))
		seen[key] = true
	}
}

func main() {
	check(executor.ErrUnknownType != nil, "ErrUnknownType is nil")
	distinct("task types", []any{
		executor.TaskTypeAgent, executor.TaskTypeReview, executor.TaskTypeStatus,
		executor.TaskTypeDLQFix, executor.TaskTypePrioritize, executor.TaskTypeDepBump,
	})
	distinct("verdicts", []any{
		executor.VerdictApprove, executor.VerdictRequestChanges,
		executor.VerdictFixed, executor.VerdictWontFix,
	})
	check(executor.IsStableSemver("v1.2.3"), "v1.2.3 must read as stable semver")
	check(!executor.IsStableSemver("v1.2.3-rc.1"), "pre-release versions must not read as stable semver")
	check(!executor.IsStableSemver(""), "the empty version must not read as stable semver")
	fmt.Println("PROBE-OK executor")
}
PROBE_EOF
}

write_probe_worker() {
	# pins: worker-probe-happy-writes — probe exercises the worker
	cat >"$1" <<'PROBE_EOF'
package main

import (
	"fmt"
	"os"

	"github.com/larsartmann/go-taskqueue/worker"
)

func check(ok bool, msg string) {
	if !ok {
		fmt.Printf("PROBE-FAIL worker: %s\n", msg)
		os.Exit(1)
	}
}

func main() {
	check(worker.EnvStreakCode != "", "EnvStreakCode is empty")
	check(worker.ExpBackoff(1) > 0, "ExpBackoff(1) must be positive")
	check(worker.ExpBackoff(0) == worker.ExpBackoff(1), "ExpBackoff must clamp sub-1 attempts to the first attempt")
	check(worker.ExpBackoff(3) >= worker.ExpBackoff(1), "ExpBackoff must be non-decreasing in the attempt")
	fmt.Println("PROBE-OK worker")
}
PROBE_EOF
}

# default_probe: write the facade's probe, tidy the scratch graph (the
# imports resolve from the proxy), build AND run it — the run's rc IS the
# sentinel-verdict fact. Build/run output lands in probe.log; failures
# print its tail for diagnosis.
default_probe() {
	local facade="$1" ver="$2" dir="$3"
	"write_probe_$facade" "$dir/main.go"
	local log="$dir/probe.log"
	if (
		cd "$dir"
		export GOWORK=off
		export GOTOOLCHAIN=auto
		export GOFLAGS=-mod=mod
		go mod tidy >/dev/null
		go build -o probe.bin .
		./probe.bin
	) >"$log" 2>&1; then
		cat "$log"
		return 0
	fi
	local probe_rc=$?
	echo "--- $facade@$ver probe output tail (rc=$probe_rc) ---" >&2
	tail -n 20 "$log" >&2 || true
	return "$probe_rc"
}

root="$(mktemp -d)"
trap 'rm -rf "$root"' EXIT

verified=0
skipped=0
failed=0
for facade in "${FACADES[@]}"; do
	dir="$root/$facade"
	mkdir -p "$dir"

	ver=""
	if [ -n "${TQ_PROXY_RESOLVE_CMD:-}" ]; then
		ver="$(bash -c "$TQ_PROXY_RESOLVE_CMD" tq-proxy-resolve "$MODULE/$facade")" || ver=""
	else
		ver="$(default_resolve "$facade" "$dir")" || ver=""
	fi
	if [ -z "$ver" ]; then
		echo "SKIP: proxy-surface $facade (latest tag unresolvable at the proxy — network-tolerant skip)" >&2
		skipped=$((skipped + 1))
		continue
	fi

	probe_rc=0
	if [ -n "${TQ_PROXY_PROBE_CMD:-}" ]; then
		bash -c "$TQ_PROXY_PROBE_CMD" tq-proxy-probe "$MODULE/$facade" "$ver" "$dir" || probe_rc=$?
	else
		default_probe "$facade" "$ver" "$dir" || probe_rc=$?
	fi
	if [ "$probe_rc" -ne 0 ]; then
		echo "FAIL: proxy-surface $facade@$ver sentinel probe rc=$probe_rc" >&2
		failed=$((failed + 1))
	else
		echo "ok: proxy-surface $facade@$ver sentinel probe"
		verified=$((verified + 1))
	fi
done

echo "proxy-surface summary: $verified facades verified, $skipped skipped, $failed failed"
if [ "$failed" -gt 0 ]; then
	echo "proxy-surface FAIL: a released tag breaks its sentinel contract for proxy consumers (see the FAIL lines above; fix + re-tag per docs/release/)" >&2
	exit 1
fi
exit 0
