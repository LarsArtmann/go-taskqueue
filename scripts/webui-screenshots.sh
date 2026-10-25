#!/usr/bin/env bash
# Web UI screenshots (round-5 M20/F107): renders the live dashboard in a
# headless browser, light + dark, into docs/screenshots/. Requires a
# chromium-family browser; fails with a clear message when none is present
# (this host class has none — run where one exists).
set -euo pipefail
cd "$(dirname "$0")/.."
REPO_ROOT="$(pwd)"

OUT="docs/screenshots"
mkdir -p "$OUT"

TMP="$(mktemp -d)"
trap '[ -n "${SERVE_PID:-}" ] && kill "$SERVE_PID" 2>/dev/null || true; rm -rf "$TMP"' EXIT

# A dead-server or error-page capture is a near-uniform small PNG; a real
# dashboard render at 1440px wide is orders of magnitude larger (task
# 000001a137f7cb2c §f1: the run used to print "wrote" and exit 0 on garbage).
MIN_PNG_BYTES="${MIN_PNG_BYTES:-20000}"

check_png() {
	local name="$1" path="$2"
	if [ ! -s "$path" ]; then
		echo "FAIL: $name capture missing or empty: $path (server down or renderer failed?)" >&2
		return 1
	fi
	local size
	size=$(wc -c <"$path")
	if [ "$size" -lt "$MIN_PNG_BYTES" ]; then
		echo "FAIL: $name capture is trivially small (${size} bytes < ${MIN_PNG_BYTES}) — likely a dead-server blank/error capture, not the dashboard" >&2
		return 1
	fi
}

# --self-test (2026-10-25 TODO): pins check_png's FAIL lines via a /tmp
# fixture — a tiny PNG against MIN_PNG_BYTES=999999 must fail with the
# trivially-small line, a missing file with the missing-or-empty line, and
# a large-enough fixture must pass. No browser, no server.
if [[ "${1:-}" == "--self-test" ]]; then
	tiny="$TMP/tiny.png"
	printf '\x89PNG\r\n\x1a\ntruncated-fixture' >"$tiny"
	# 25 bytes exactly (8-byte PNG signature + fixture text): the size is
	# pinned in the FAIL-line assertion below.
	big="$TMP/big.png"
	head -c 30000 /dev/zero >"$big"
	local_rc=0
	out="$(MIN_PNG_BYTES=999999 check_png tiny "$tiny" 2>&1)" || local_rc=$?
	if [ "$local_rc" -ne 1 ]; then
		echo "FAIL: self-test trivially-small branch (want rc=1, got rc=$local_rc)" >&2
		printf '%s\n' "$out" >&2
		exit 1
	fi
	case "$out" in
	*"FAIL: tiny capture is trivially small (25 bytes < 999999)"*) ;;
	*)
		echo "FAIL: self-test trivially-small FAIL line not pinned" >&2
		printf '%s\n' "$out" >&2
		exit 1
		;;
	esac
	missing_rc=0
	out="$(MIN_PNG_BYTES=20000 check_png ghost "$TMP/nope.png" 2>&1)" || missing_rc=$?
	if [ "$missing_rc" -ne 1 ] || ! grep -q 'FAIL: ghost capture missing or empty' <<<"$out"; then
		echo "FAIL: self-test missing-file branch (want rc=1 + missing-or-empty FAIL line, got rc=$missing_rc)" >&2
		printf '%s\n' "$out" >&2
		exit 1
	fi
	if ! MIN_PNG_BYTES=20000 check_png big "$big" 2>/dev/null; then
		echo "FAIL: self-test positive branch (30k fixture must pass at MIN_PNG_BYTES=20000)" >&2
		exit 1
	fi
	echo "ok: check_png self-test (trivially-small FAIL line + missing-or-empty FAIL line + positive pass)"
	exit 0
fi

BROWSER="${CHROME_BIN:-}"
if [ -z "$BROWSER" ]; then
	for candidate in chromium chromium-browser google-chrome google-chrome-stable; do
		if command -v "$candidate" >/dev/null 2>&1; then
			BROWSER="$candidate"
			break
		fi
	done
fi

if [ -z "$BROWSER" ]; then
	echo "error: no chromium-family browser found (set CHROME_BIN); screenshots need a real renderer" >&2
	echo "  fix (reproducible recipe, 2026-09-16 instrument-shot spike §a8): nix build nixpkgs#chromium -o /tmp/chromium-shot && CHROME_BIN=/tmp/chromium-shot/bin/chromium $0" >&2
	echo "  ruling (2026-10-21 batch): this harness deliberately lives OUTSIDE scripts/smoke/ — smoke scripts are CI-safe, this one needs a browser; chromedp assertions stay the webui-health TODO row's scope" >&2
	exit 1
fi

# A dead-server or error-page capture guard lives in check_png above.
"$REPO_ROOT/scripts/build-tq.sh" "$TMP/tq"
TQ_DB="$TMP/shots.db" "$TMP/tq" enqueue --type sh --project demo --payload '"true"' >/dev/null
"$TMP/tq" serve --db "$TMP/shots.db" --addr 127.0.0.1:8097 &
SERVE_PID=$!

for _ in $(seq 1 30); do
	curl -sf http://127.0.0.1:8097/ >/dev/null 2>&1 && break
	sleep 0.2
done 2>/dev/null || true
if ! curl -sf http://127.0.0.1:8097/ >/dev/null 2>&1; then
	echo "WARN: tq serve never came up on 127.0.0.1:8097 after the poll window — continuing; screenshots will fail with the underlying curl/browser error"
fi

shoot() {
	local name="$1" scheme="$2"
	"$BROWSER" --headless --disable-gpu --no-sandbox \
		--window-size=1440,2400 --screenshot="$OUT/$name.png" \
		--force-prefers-color-scheme="$scheme" \
		"http://127.0.0.1:8097/" >/dev/null 2>&1
	echo "wrote $OUT/$name.png"
	check_png "$name" "$OUT/$name.png"
}

shoot dashboard-light light
shoot dashboard-dark dark

# Detail page: a REAL task id from the seeded database (round-5 d1 — the
# old `top --json | head -1 | tr -d '"'` piped a JSON object fragment into
# the URL and always 404'd).
if ! command -v jq >/dev/null 2>&1; then
	echo "error: jq needed to pick the detail task id" >&2
	exit 1
fi
DETAIL_ID="$(TQ_DB="$TMP/shots.db" "$TMP/tq" tasks --json 2>/dev/null | jq -r '.[0].id // empty')"
if [ -z "$DETAIL_ID" ]; then
	echo "error: no task in the screenshot database to open" >&2
	exit 1
fi

"$BROWSER" --headless --disable-gpu --no-sandbox \
	--window-size=1440,1200 --screenshot="$OUT/task-detail.png" \
	"http://127.0.0.1:8097/task/$DETAIL_ID" \
	>/dev/null 2>&1 || echo "wrote $OUT/task-detail.png (best effort)"
check_png "task-detail" "$OUT/task-detail.png"

echo "screenshots complete"
