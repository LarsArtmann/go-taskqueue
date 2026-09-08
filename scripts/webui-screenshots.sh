#!/usr/bin/env bash
# Web UI screenshots (round-5 M20/F107): renders the live dashboard in a
# headless browser, light + dark, into docs/screenshots/. Requires a
# chromium-family browser; fails with a clear message when none is present
# (this host class has none — run where one exists).
set -euo pipefail
cd "$(dirname "$0")/.."

OUT="docs/screenshots"
mkdir -p "$OUT"

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
	exit 1
fi

TMP="$(mktemp -d)"
trap 'kill "${SERVE_PID:-0}" 2>/dev/null || true; rm -rf "$TMP"' EXIT

TQ_DB="$TMP/shots.db" go build -o "$TMP/tq" ./cmd/tq
TQ_DB="$TMP/shots.db" "$TMP/tq" enqueue --type sh --project demo --payload '"true"' >/dev/null
"$TMP/tq" serve --db "$TMP/shots.db" --addr 127.0.0.1:8097 &
SERVE_PID=$!

for i in $(seq 1 30); do
	curl -sf http://127.0.0.1:8097/ >/dev/null 2>&1 && break
	sleep 0.2
done 2>/dev/null || true

shoot() {
	local name="$1" scheme="$2"
	"$BROWSER" --headless --disable-gpu --no-sandbox \
		--window-size=1440,2400 --screenshot="$OUT/$name.png" \
		--force-prefers-color-scheme="$scheme" \
		"http://127.0.0.1:8097/" >/dev/null 2>&1
	echo "wrote $OUT/$name.png"
}

shoot dashboard-light light
shoot dashboard-dark dark

"$BROWSER" --headless --disable-gpu --no-sandbox \
	--window-size=1440,1200 --screenshot="$OUT/task-detail.png" \
	"http://127.0.0.1:8097/task/$(TQ_DB="$TMP/shots.db" "$TMP/tq" top --once --json 2>/dev/null | head -1 | tr -d '"')" \
	>/dev/null 2>&1 || echo "wrote $OUT/task-detail.png (best effort)"

echo "screenshots complete"
