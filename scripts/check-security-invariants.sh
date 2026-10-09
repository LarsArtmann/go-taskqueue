#!/usr/bin/env bash
# Security-invariant drift gate: the "a learned check may only narrow"
# promise must stay true in BOTH prose (SECURITY.md) and code
# (internal/executor/finding.go, the MergeFindings helper). A rename of the
# helper or a wording drift that drops the max+union rule would silently
# degrade the promise to stale prose — this gate catches that.
#
# --self-test mutates a temp copy and asserts the gate CATCHES each drift
# (prose needle removed, code needle removed), so the gate cannot rot into a
# no-op that always passes.
set -euo pipefail
cd "$(dirname "$0")/.."

# Needles are single-line substrings: SECURITY.md wraps prose at ~72 cols,
# so only phrases that never straddle a line break may appear here.
security_needles=(
	"MergeFindings"
	"max(deterministic, learned)"
	"never widen"
	"must never LOWER"
)
finding_needles=(
	"func MergeFindings("
	"func unionFlags("
	"never LOWER"
)

check() {
	local file="$1"
	shift
	local -a needles=("$@")
	local rc=0

	if [ ! -f "$file" ]; then
		echo "MISSING: $file"
		return 1
	fi

	for needle in "${needles[@]}"; do
		if ! grep -qF "$needle" "$file"; then
			echo "DRIFT: $file no longer contains: $needle"
			rc=1
		fi
	done

	return "$rc"
}

run_gate() {
	# Resolve the root here (not at script top) so --self-test can point the
	# gate at a mutated temp copy via TQ_SECURITY_ROOT.
	local root="${TQ_SECURITY_ROOT:-.}"
	local security="$root/SECURITY.md"
	local finding="$root/internal/executor/finding.go"
	local rc=0

	check "$security" "${security_needles[@]}" || rc=1
	check "$finding" "${finding_needles[@]}" || rc=1

	return "$rc"
}

if [ "${1:-}" = "--self-test" ]; then
	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' EXIT
	mkdir -p "$tmp/internal/executor"
	cp SECURITY.md "$tmp/SECURITY.md"
	cp internal/executor/finding.go "$tmp/internal/executor/finding.go"

	# Sanity: the pristine copy must pass before mutation.
	if ! TQ_SECURITY_ROOT="$tmp" run_gate; then
		echo "self-test: pristine copy failed the gate"
		exit 1
	fi

	# Mutation 1: drop the prose needle.
	grep -vF "must never LOWER" "$tmp/SECURITY.md" >"$tmp/SECURITY.md.mut" &&
		mv "$tmp/SECURITY.md.mut" "$tmp/SECURITY.md"
	if TQ_SECURITY_ROOT="$tmp" run_gate; then
		echo "self-test: gate missed the SECURITY.md prose drift"
		exit 1
	fi
	cp SECURITY.md "$tmp/SECURITY.md"

	# Mutation 2: rename the code helper.
	sed 's/func MergeFindings(/func MergeFindingsRenamed(/' "$tmp/internal/executor/finding.go" \
		>"$tmp/internal/executor/finding.go.mut" &&
		mv "$tmp/internal/executor/finding.go.mut" "$tmp/internal/executor/finding.go"
	if TQ_SECURITY_ROOT="$tmp" run_gate; then
		echo "self-test: gate missed the MergeFindings rename"
		exit 1
	fi

	echo "security-invariant self-test ok: both drift mutations were caught"
	exit 0
fi

if run_gate; then
	echo "security-invariant check ok: learned-checks-narrow holds in prose and code"
	exit 0
fi

echo "security-invariant check FAILED: see DRIFT lines above" >&2
exit 1
