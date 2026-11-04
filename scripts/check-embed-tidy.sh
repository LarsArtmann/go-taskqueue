#!/usr/bin/env bash
# embed-adoptions tidy gate (TODO row: twentyfifth-dispatch §e1): trips when
# `go mod tidy` under examples/embed would change go.mod/go.sum — the CI
# consumer-job red class (tag-wave/go.sum-refresh; third sighting after the
# cmd/tq tidy leg and the nix vendorHash). Catches it at push time.
#
# Countable-gate contract (fleet convention): summary format PINNED by
# --self-test including the negative path via TQ_EMBED_TIDY_TEST=drift.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR/.."
EMBED=examples/embed

self_test() {
	local want_ok="embed-tidy ok: examples/embed go.mod/go.sum are tidy"
	local want_drift="EMBED-TIDY DRIFT"
	local out
	out="$(TQ_EMBED_TIDY_TEST=drift "$SCRIPT_DIR/check-embed-tidy.sh" 2>&1 || true)"
	echo "$out" | grep -q "$want_drift" || {
		echo "self-test FAIL: drift branch did not fire ($want_drift missing)"
		exit 1
	}
	echo "$out" | grep -q "go.sum" || {
		echo "self-test FAIL: drift branch does not name the affected files"
		exit 1
	}
	echo "$out" | grep -q "GOWORK=off go mod tidy" || {
		echo "self-test FAIL: drift branch missing the fix recipe"
		exit 1
	}
	local ok_out
	ok_out="$(TQ_EMBED_TIDY_TEST=ok "$SCRIPT_DIR/check-embed-tidy.sh" 2>&1 || true)"
	echo "$ok_out" | grep -q "$want_ok" || {
		echo "self-test FAIL: ok summary drifted: $ok_out"
		exit 1
	}
	echo "self-test ok: drift branch + recipe + ok summary pinned"
	exit 0
}

[ "${1:-}" = "--self-test" ] && self_test

if [ "${TQ_EMBED_TIDY_TEST:-}" = "drift" ]; then
	# pins: env-hook-drift-branch — TQ_EMBED_TIDY_TEST=drift negative path
	cat >&2 <<'EOF'
EMBED-TIDY DRIFT: `go mod tidy` would change examples/embed go.mod/go.sum (files shown by git diff there).
fix recipe: cd examples/embed && GOWORK=off go mod tidy, then commit the refreshed go.mod/go.sum
provenance: the CI consumer job builds examples/embed as a facade adopter; an untidied
adopter graph only surfaces as a red tag-wave later (cmd/tq tidy leg, nix vendorHash).
EOF
	exit 1
fi

if [ "${TQ_EMBED_TIDY_TEST:-}" = "ok" ]; then
	echo "embed-tidy ok: examples/embed go.mod/go.sum are tidy"
	exit 0
fi

if [ ! -f "$EMBED/go.mod" ]; then
	echo "FAIL: $EMBED/go.mod missing"
	exit 1
fi

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
cp "$EMBED/go.mod" "$TMP/go.mod"
cp "$EMBED/go.sum" "$TMP/go.sum"

LOG="$TMP/tidy.log"
if ! (cd "$EMBED" && GOWORK=off go mod tidy >"$LOG" 2>&1); then
	echo "FAIL: go mod tidy errored in $EMBED (tag moved, network, or a real break):"
	tail -20 "$LOG"
	cp "$TMP/go.mod" "$EMBED/go.mod"
	cp "$TMP/go.sum" "$EMBED/go.sum"
	exit 1
fi

DRIFT=0
cmp -s "$TMP/go.mod" "$EMBED/go.mod" || DRIFT=1
cmp -s "$TMP/go.sum" "$EMBED/go.sum" || DRIFT=1

if [ "$DRIFT" = 0 ]; then
	echo "embed-tidy ok: examples/embed go.mod/go.sum are tidy"
	exit 0
fi

echo "EMBED-TIDY DRIFT: go mod tidy changes $EMBED/go.mod and/or go.sum (left in place; inspect with git diff $EMBED):"
git diff --stat -- "$EMBED/go.mod" "$EMBED/go.sum"
# pins: real-tidy-drift-branch — genuine go mod tidy divergence on the adopter graph
cat >&2 <<'EOF'
fix recipe: cd examples/embed && GOWORK=off go mod tidy, then commit the refreshed go.mod/go.sum
provenance: the CI consumer job builds examples/embed as a facade adopter; an untidied
adopter graph only surfaces as a red tag-wave later (cmd/tq tidy leg, nix vendorHash).
EOF
exit 1
