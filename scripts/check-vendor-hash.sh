#!/usr/bin/env bash
# vendorHash drift detector (T22; TODO rows 1389/504): realizes ONLY the
# go-modules FOD via `nix build .#checks.x86_64-linux.vendor-hash` (the
# flake's fast drift gate — seconds, not a full build) and, on mismatch,
# prints the exact got→flake.nix copy recipe. Answers the row-504
# provenance question operationally: vendorHash is a function of the
# committed go.mod graph (requires + replace pins), NOTHING else — the
# go-tarball hash twin is single-sourced at flake.nix (goTarballHash =
# goToolchainHash) and never feeds it.
#
# Countable-gate contract (2026-10-24 fleet convention): the summary
# format is PINNED by --self-test, including the negative path via
# TQ_VENDOR_HASH_TEST=drift (no real nix run needed for the pin).
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR/.."

FLAKE_VENDORHASH_LINE="$(grep -n 'vendorHash = "sha256-' flake.nix | head -1)"

self_test() {
	local want_summary="vendor-hash ok: flake.nix vendorHash matches the module graph"
	local want_drift="VENDOR-HASH DRIFT"
	local out
	out="$(TQ_VENDOR_HASH_TEST=drift "$SCRIPT_DIR/check-vendor-hash.sh" 2>&1 || true)"
	echo "$out" | grep -q "$want_drift" || {
		echo "self-test FAIL: drift branch did not fire ($want_drift missing)"
		exit 1
	}
	echo "$out" | grep -q "flake.nix:${FLAKE_VENDORHASH_LINE%%:*}" || {
		echo "self-test FAIL: drift recipe does not name the flake.nix vendorHash line"
		exit 1
	}
	echo "$out" | grep -q "got:" || {
		echo "self-test FAIL: drift recipe missing the got: hash"
		exit 1
	}
	local ok_out
	ok_out="$(TQ_VENDOR_HASH_TEST=ok "$SCRIPT_DIR/check-vendor-hash.sh" 2>&1 || true)"
	echo "$ok_out" | grep -q "$want_summary" || {
		echo "self-test FAIL: ok summary drifted: $ok_out"
		exit 1
	}
	echo "self-test ok: drift branch + recipe + ok summary pinned"
	exit 0
}

[ "${1:-}" = "--self-test" ] && self_test

if [ "${TQ_VENDOR_HASH_TEST:-}" = "drift" ]; then
	# pins: self-test drift branch (recipe text + flake line + got: hash)
	cat >&2 <<EOF
VENDOR-HASH DRIFT: the committed go.mod graph hashes differently than flake.nix pins.
  specified: sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
  got:      sha256-BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB
copy recipe: replace the hash at flake.nix:${FLAKE_VENDORHASH_LINE%%:*} with the got: value above,
then re-run: ./scripts/check-vendor-hash.sh && nix build
provenance: vendorHash = f(go.mod requires + replace pins) — a drift means the graph changed
without the pin, never the tarball (goTarballHash is single-sourced at flake.nix:54).
EOF
	exit 1
fi

if [ "${TQ_VENDOR_HASH_TEST:-}" = "ok" ]; then
	echo "vendor-hash ok: flake.nix vendorHash matches the module graph"
	exit 0
fi

if ! command -v nix >/dev/null 2>&1; then
	echo "SKIP: nix not on PATH — vendor-hash drift gate is host-pinned (CI's nix job owns it remotely)"
	exit 0
fi

LOG="$(mktemp)"
trap 'rm -f "$LOG"' EXIT

if nix build .#checks.x86_64-linux.vendor-hash --no-link >"$LOG" 2>&1; then
	echo "vendor-hash ok: flake.nix vendorHash matches the module graph"
	exit 0
fi

GOT="$(grep -o 'got: *sha256-[A-Za-z0-9+/=]*' "$LOG" | head -1 | sed 's/got: *//')"
SPEC="$(grep -o 'specified: *sha256-[A-Za-z0-9+/=]*' "$LOG" | head -1 | sed 's/specified: *//')"

if [ -z "$GOT" ]; then
	echo "FAIL: vendor-hash build failed WITHOUT a hash mismatch (a different nix error):"
	tail -20 "$LOG"
	exit 1
fi

# pins: real drift branch (got/spec from the nix build log + copy recipe)
cat >&2 <<EOF
VENDOR-HASH DRIFT: the committed go.mod graph hashes differently than flake.nix pins.
  specified: $SPEC
  got:      $GOT
copy recipe: replace the hash at flake.nix:${FLAKE_VENDORHASH_LINE%%:*} with the got: value above,
then re-run: ./scripts/check-vendor-hash.sh && nix build
provenance: vendorHash = f(go.mod requires + replace pins) — a drift means the graph changed
without the pin, never the tarball (goTarballHash is single-sourced at flake.nix:54).
EOF
exit 1
