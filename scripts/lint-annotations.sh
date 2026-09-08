#!/usr/bin/env bash
# Emit GitHub annotations (::warning) for golangci-lint findings on lines
# changed since the base revision ($LINT_BASE env, else $1, else HEAD~1).
#
# Why this exists: golangci-lint v2 has no github-actions output format
# (v2.13.2 rejects --output.github-actions.path as an unknown flag) and its
# default text output never emits ::warning/::error commands. Its raw
# `file.go:line:col: msg` text lines WOULD still become [failure] annotations
# via the "go" problem matcher that actions/setup-go registers (verified on
# the 2026-09-07 green run: 10 stale baseline findings leaked that way), so
# ci.yml removes that matcher in the advisory step. GitHub also caps
# annotations at 10 warnings per step / 50 per job, so the ~400-finding
# baseline could never surface usefully anyway. Scoping to --new-from-rev
# keeps the count near zero, which is exactly the policy that matters: new
# findings on changed lines surface on green runs, the accepted baseline
# stays log-only.
set -euo pipefail
cd "$(dirname "$0")/.."

base="${LINT_BASE:-${1:-}}"
if [ -z "$base" ] || ! git rev-parse -q --verify "$base^{commit}" >/dev/null 2>&1; then
	base="HEAD~1"
fi
if ! git rev-parse -q --verify "$base^{commit}" >/dev/null 2>&1; then
	echo "lint-annotations: no base revision available, skipping"
	exit 0
fi

lintbin="$(command -v golangci-lint || true)"
if [ -z "$lintbin" ]; then
	lintbin="$(go env GOPATH)/bin/golangci-lint"
fi

tmp="$(mktemp /tmp/lint-annotations.XXXXXX.json)"
trap 'rm -f "$tmp"' EXIT

rc=0
"$lintbin" run --new-from-rev="$base" --output.json.path="$tmp" ./... || rc=$?
# Exit 1 means findings were found (issues-exit-code: 1) — expected and fine.
# Any other nonzero code is a lint execution failure, not a finding.
if [ "$rc" -ne 0 ] && [ "$rc" -ne 1 ]; then
	echo "lint-annotations: golangci-lint execution failed (exit $rc)" >&2
	exit "$rc"
fi
if [ ! -s "$tmp" ]; then
	echo "lint-annotations: golangci-lint produced no JSON (exit $rc), skipping"
	exit 0
fi

count="$(jq '.Issues // [] | length' "$tmp")"
if [ "$count" -eq 0 ]; then
	echo "lint-annotations: 0 new findings on lines changed since $base"
	exit 0
fi

echo "lint-annotations: $count new finding(s) on lines changed since $base"
# GitHub caps displayed annotations at 10 warnings per step and drops the
# rest silently; the first $cap become annotations, the rest stay log-only.
cap=10
jq -r --argjson cap "$cap" '
    def enc: gsub("%"; "%25") | gsub("\r"; "%0D") | gsub("\n"; "%0A");
    (.Issues // [])
    | sort_by(.Pos.Filename, .Pos.Line, .FromLinter)
    | to_entries[]
    | if .key < $cap then
        "::warning file=\(.value.Pos.Filename),line=\(.value.Pos.Line),col=\(.value.Pos.Column)::[\(.value.FromLinter)] \(.value.Text | enc)"
      else
        "lint-annotations: over the 10-annotation cap, log only: \(.value.Pos.Filename):\(.value.Pos.Line):\(.value.Pos.Column) [\(.value.FromLinter)] \(.value.Text | enc)"
      end' "$tmp"
