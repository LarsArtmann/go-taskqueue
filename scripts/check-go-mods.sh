#!/usr/bin/env bash
# Multi-module go.mod health gate: portable replaces, pinned internal
# requires, toolchain alignment across every module, and dependency
# verification. Called from scripts/ci-local.sh AND
# .github/workflows/ci.yml — keep both call sites in sync.
set -euo pipefail
cd "$(dirname "$0")/.."
# Recursive --self-test re-invokes THIS script; capture the absolute self
# path BEFORE any cd (the gosec self_abs pattern; 02-38 §b).
self_abs="$(cd "$(dirname "$0")" && pwd)/$(basename "$0")"

must_mention() {
	if grep -qF -- "$1" <<<"$2"; then
		echo "ok: $3"
	else
		echo "FAIL: $3 (output is missing '$1')"
		printf '%s\n' "$2"
		exit 1
	fi
}

# --self-test pins the end-of-run summary line (fleet convention: countable
# gate summaries are PINNED, not advisory — 01-12 §e7/§g2) and runs a
# negative-path counter battery over the if/else branches via a /tmp
# fixture module (absolute replace + unpinned internal require + go
# directive downgrade) injected through the TQ_GOMODS_EXTRA_MODS hook.
self_test() {
	# tmp is deliberately global: the EXIT trap reads it after self_test
	# returned, and set -u would kill the trap on a function-local name.
	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' EXIT
	local fx="$tmp/fixmod"
	mkdir -p "$fx"
	cat >"$fx/go.mod" <<EOF
module fixmod

go 1.20

require (
	github.com/larsartmann/go-taskqueue/internal/task v0.3.4-rc1
)

replace github.com/larsartmann/go-taskqueue/internal/task => /nonexistent/abs/path
EOF
	local out rc=0
	out="$(TQ_GOMODS_EXTRA_MODS="$fx" "$self_abs" 2>&1)" || rc=$?
	if [ "$rc" -ne 1 ]; then
		echo "FAIL: negative battery (want rc=1, got rc=$rc)"
		printf '%s\n' "$out"
		exit 1
	fi
	must_mention "absolute replace paths are not portable" "$out" "portable-replace failure branch fires on the fixture"
	must_mention "internal requires must be real tagged versions" "$out" "pinned-require failure branch fires on the fixture (prerelease pseudo-pin)"
	must_mention "a module may only RAISE the go directive" "$out" "go-directive downgrade branch fires on the fixture"
	local pos_out pos_rc=0
	pos_out="$("$self_abs" 2>&1)" || pos_rc=$?
	if grep -Eq '^go-mods summary: [0-9]+ checks ok, [0-9]+ failed$' <<<"$pos_out"; then
		echo "ok: real-run summary matches the pinned format (rc=$pos_rc)"
	else
		echo "FAIL: real-run summary missing or malformed"
		printf '%s\n' "$pos_out"
		exit 1
	fi
	echo "go-mods self-test ok (summary format pin + negative counter battery)"
}

if [ "${1:-}" = "--self-test" ]; then
	self_test
	exit 0
fi

mods="$(
	scripts/for-each-module.sh
	echo cmd/tq
)"
# TQ_GOMODS_EXTRA_MODS: self-test hook appending fixture module dirs to the
# enumerated set (each dir must contain a go.mod).
if [ -n "${TQ_GOMODS_EXTRA_MODS:-}" ]; then
	mods="$mods
$TQ_GOMODS_EXTRA_MODS"
fi
[[ -f cmd/tq/go.mod ]] # cmd/tq sits outside for-each-module's set (ADR-0017) — fail fast if it vanished
mapfile -t modfiles < <(printf '%s\n' "$mods" | sed 's|$|/go.mod|')
fail=0
checks_ok=0
checks_failed=0

bad="$(grep -hE '^replace ' "${modfiles[@]}" | grep -E '=> */' || true)"
if [ -n "$bad" ]; then
	echo "$bad"
	echo "FAIL: absolute replace paths are not portable"
	checks_failed=$((checks_failed + 1))
	fail=1
else
	checks_ok=$((checks_ok + 1))
fi

bad="$({ grep -hE '^[[:space:]]*github.com/larsartmann/go-taskqueue/internal/' "${modfiles[@]}" go.mod | sed 's|//.*||'; } | grep -vE ' v[0-9]+\.[0-9]+\.[0-9]+[[:space:]]*$' || true)"
if [ -n "$bad" ]; then
	echo "$bad"
	echo "FAIL: internal requires must be real tagged versions (vX.Y.Z) —"
	echo "go install of the published module resolves sub-modules through the"
	echo "proxy, where v0.0.0 never exists (ADR-0011; local replaces make the"
	echo "version cosmetic in-repo, which is why a wrong pin stays invisible)"
	checks_failed=$((checks_failed + 1))
	fail=1
else
	checks_ok=$((checks_ok + 1))
fi

want="$(awk '$1 == "go" { print $2; exit }' go.mod)"
for m in $mods; do
	got="$(awk '$1 == "go" { print $2; exit }' "$m/go.mod")"
	# A module may declare a NEWER go directive than root when a dependency
	# requires it (ADR-0019 S1: go-cqrs-lite queue/v4 needs go 1.27.1 while
	# root still declares 1.27) — a raise is additive and never runs a
	# module on an older language version. A DOWNGRADE stays a hard FAIL:
	# zero-dep leaves have no dependency floor, so `go mod tidy` can
	# silently revert them (2026-09-12 red master, run 34726154600).
	if [ "$(printf '%s\n' "$want" "$got" | sort -V | head -1)" != "$want" ]; then
		echo "FAIL: $m/go.mod declares go $got, root declares go $want — a module may only RAISE the go directive (downgrades killed master: 2026-09-12)"
		checks_failed=$((checks_failed + 1))
		fail=1
	elif [ "$got" != "$want" ]; then
		echo "WARN: $m/go.mod declares go $got > root $want (dependency-required raise)"
		checks_ok=$((checks_ok + 1))
	else
		checks_ok=$((checks_ok + 1))
	fi
done

for m in . $mods; do
	# go mod verify flakes when the shared module cache is written
	# concurrently (16-00 report f41): retry once before failing, and the
	# FAIL line carries the underlying stderr (04-21 §f14) — a bare
	# "FAIL: <module>" named the victim but not the symptom.
	if ! err="$(cd "$m" && GOWORK=off go mod verify 2>&1 >/dev/null)"; then
		echo "WARN: go mod verify flaked in $m — retrying once"
		if ! err="$(cd "$m" && GOWORK=off go mod verify 2>&1 >/dev/null)"; then
			echo "FAIL: go mod verify in $m (retry also failed):"
			printf '%s\n' "$err" | sed 's/^/  /'
			checks_failed=$((checks_failed + 1))
			fail=1
		else
			checks_ok=$((checks_ok + 1))
		fi
	else
		checks_ok=$((checks_ok + 1))
	fi
done

echo "go-mods summary: $checks_ok checks ok, $checks_failed failed"
exit $fail
