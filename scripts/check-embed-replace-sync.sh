#!/usr/bin/env bash
# embed replace-block sync gate (facade-retirement follow-up, batch
# 000001a2093ef4b12ceab7e50acc00000000, 2026-11-23): examples/embed/go.mod's
# KEEP-FOREVER replace block mirrors the tq module graph BY HAND, and
# scripts/smoke/consumer-store-impl.sh derives its HEAD-truth replace list
# from that block at runtime -- one hand-maintained list feeds both the
# embed example build and the smoke's Store-implementability leg. A graph
# change that lands in embed's requires but not its replace block resolves
# the new module from a stale proxy tag against HEAD facades (mixed tree),
# silently in BOTH surfaces. Three pinned relations:
#   1. coverage: every tq module embed requires (direct + indirect) has a
#      replace line in the KEEP-FOREVER block;
#   2. liveness: every replace left side resolves to a real in-repo module
#      directory whose go.mod declares that module path;
#   3. derivation: the smoke still derives its block from embed live --
#      the replace-block extraction is present, no hand-frozen tq module
#      list outside its main.go heredoc, and a silently empty derivation
#      is refused by the smoke itself.
#
# Countable-gate contract (fleet convention 01-12): the summary format is
# PINNED by --self-test including the negative battery via the
# TQ_REPLACE_SYNC_EMBED_GOMOD / TQ_REPLACE_SYNC_EMBED_DIR /
# TQ_REPLACE_SYNC_SMOKE hooks.
set -euo pipefail
export LC_ALL=C
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
self_abs="$SCRIPT_DIR/$(basename "$0")"
cd "$SCRIPT_DIR/.."

EMBED_GOMOD="${TQ_REPLACE_SYNC_EMBED_GOMOD:-examples/embed/go.mod}"
EMBED_DIR="${TQ_REPLACE_SYNC_EMBED_DIR:-examples/embed}"
SMOKE="${TQ_REPLACE_SYNC_SMOKE:-scripts/smoke/consumer-store-impl.sh}"

extract_requires() {
	awk '/^require \(/,/^\)/' "$1" \
		| grep -oE 'github\.com/larsartmann/go-taskqueue/[A-Za-z0-9._/-]+' \
		| sort -u || true
}

extract_replace_mods() {
	sed -n '/^replace (/,/^)/p' "$1" \
		| sed -e '1d' -e '$d' \
			-e 's/=> .*$//' \
			-e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//' \
		| grep -v '^$' \
		| grep '^github\.com/larsartmann/go-taskqueue/' \
		| sort -u || true
}

extract_replace_lines() {
	sed -n '/^replace (/,/^)/p' "$1" \
		| sed -e '1d' -e '$d' \
			-e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//' \
		| grep -v '^$' | grep '^[^#]' || true
}

count_lines() {
	printf '%s\n' "$1" | grep -c . || true
}

run_checks() {
	local drift=0
	local requires replaces lines missing line left right dir
	requires="$(extract_requires "$EMBED_GOMOD")"
	replaces="$(extract_replace_mods "$EMBED_GOMOD")"
	lines="$(extract_replace_lines "$EMBED_GOMOD")"

	if [ ! -f "$EMBED_GOMOD" ]; then
		echo "REPLACE-SYNC DRIFT: $EMBED_GOMOD missing"
		drift=1
	fi

	if [ "$(count_lines "$requires")" -eq 0 ]; then
		echo "REPLACE-SYNC DRIFT: parsed ZERO tq requires from $EMBED_GOMOD -- require-block shape changed; fix extract_requires before trusting this gate"
		drift=1
	fi
	if [ "$(count_lines "$replaces")" -eq 0 ]; then
		echo "REPLACE-SYNC DRIFT: parsed ZERO tq replace lines from $EMBED_GOMOD -- the KEEP-FOREVER block vanished or its shape changed"
		drift=1
	fi

	missing="$(comm -23 <(printf '%s\n' "$requires") <(printf '%s\n' "$replaces") | grep -v '^$' || true)"
	if [ -n "$missing" ]; then
		echo "REPLACE-SYNC DRIFT: tq modules embed requires with NO replace line (these resolve from a stale proxy tag against HEAD facades):"
		printf '%s\n' "$missing" | sed 's/^/  missing: /'
		drift=1
	fi

	while IFS= read -r line; do
		[ -z "$line" ] && continue
		case "$line" in github.com/larsartmann/go-taskqueue/*) ;; *) continue ;; esac
		left="$(printf '%s' "${line%%=>*}" | sed -e 's/[[:space:]]*$//')"
		right="$(printf '%s' "${line#*=>}" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')"
		case "$right" in
		/*) dir="$right" ;;
		*) dir="$EMBED_DIR/$right" ;;
		esac
		if [ ! -f "$dir/go.mod" ]; then
			echo "REPLACE-SYNC DRIFT: stale replace line (target dir has no go.mod): $line (resolved: $dir)"
			drift=1
			continue
		fi
		if ! grep -qxF "module $left" "$dir/go.mod"; then
			echo "REPLACE-SYNC DRIFT: replace target module-path mismatch: $line (target go.mod declares: $(awk '$1 == "module" { print $2; exit }' "$dir/go.mod"))"
			drift=1
		fi
	done <<<"$lines"

	if [ ! -f "$SMOKE" ]; then
		echo "REPLACE-SYNC DRIFT: smoke $SMOKE missing -- the HEAD-truth Store-implementability leg cannot derive its replace list"
		drift=1
	else
		local smoke_body smoke_code
		smoke_body="$(sed -e "/cat >main.go <<'EOF'/,/^EOF$/d" "$SMOKE")"
		# For the frozen-list check only, also strip string literals and
		# comment lines: what remains is bare script code, where a tq module
		# path can only be a hand-frozen replace list. The derivation,
		# embed-reference and guard checks run on smoke_body (quotes
		# intact) -- stripping would erase their double-quoted occurrences.
		smoke_code="$(sed -e 's/"[^"]*"//g' <<<"$smoke_body" | grep -v '^[[:space:]]*#' || true)"
		if ! grep -qF "sed -n '/^replace (/,/^)/p'" <<<"$smoke_body"; then
			echo "REPLACE-SYNC DRIFT: smoke no longer DERIVES its replace block from examples/embed/go.mod (replace-block extraction gone) -- a frozen copy would hand-sync the graph again"
			drift=1
		fi
		if ! grep -qF "examples/embed" <<<"$smoke_body"; then
			echo "REPLACE-SYNC DRIFT: smoke derivation no longer points at examples/embed"
			drift=1
		fi
		if grep -qE 'go-taskqueue/(task|queue|journal|executor|worker|internal)/' <<<"$smoke_code"; then
			echo "REPLACE-SYNC DRIFT: smoke carries a hand-frozen tq module path outside its main.go heredoc -- the replace list must stay derived from embed, never copied:"
			grep -nE 'go-taskqueue/(task|queue|journal|executor|worker|internal)/' <<<"$smoke_code" | sed 's/^/  frozen: /'
			drift=1
		fi
		if ! grep -qF "EMPTY REPLACE LIST" <<<"$smoke_body"; then
			echo "REPLACE-SYNC DRIFT: smoke lost its empty-derivation guard (EMPTY REPLACE LIST) -- a silently empty derivation would run the HEAD leg proxy-mixed"
			drift=1
		fi
	fi

	if [ "$drift" -ne 0 ]; then
		# pins: drift-exit-recipe — the recipe + provenance block names every drift exit
		cat >&2 <<'EOF'
fix recipe: cd examples/embed && GOWORK=off go mod tidy, then add ONE replace line per new tq module in the transitive graph (see the block's KEEP-FOREVER comment); keep scripts/smoke/consumer-store-impl.sh deriving its block live -- never freeze a module list into it
provenance: embed's local build AND the smoke's TQ_STORE_IMPL_REPLACE=1 HEAD-truth leg both resolve the tq graph from this ONE hand-maintained block; a missing line silently mixed a stale proxy tag into HEAD-truth verification (facade-retirement batch 000001a2093ef4b12ceab7e50acc00000000)
EOF
		return 1
	fi

	echo "embed-replace-sync ok: $(count_lines "$requires") tq requires covered by $(count_lines "$replaces") replace lines; smoke derivation intact"
}

self_test() {
	# tmp is deliberately global: the EXIT trap reads it after self_test
	# returned, and set -u would kill the trap on a function-local name.
	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' EXIT
	local real_gomod="$SCRIPT_DIR/../examples/embed/go.mod"
	local out rc

	# pins: negative-battery-coverage -- a required tq module without a
	# replace line must fire the coverage branch naming the module.
	grep -v 'go-taskqueue/internal/task =>' "$real_gomod" >"$tmp/missing.go.mod"
	rc=0
	out="$(TQ_REPLACE_SYNC_EMBED_GOMOD="$tmp/missing.go.mod" TQ_REPLACE_SYNC_EMBED_DIR="$SCRIPT_DIR/../examples/embed" "$self_abs" 2>&1)" || rc=$?
	if [ "$rc" -ne 1 ]; then
		echo "FAIL: missing-replace battery (want rc=1, got rc=$rc)"
		printf '%s\n' "$out"
		exit 1
	fi
	grep -qF 'internal/task' <<<"$out" || {
		echo "FAIL: coverage branch does not name the missing module"
		printf '%s\n' "$out"
		exit 1
	}
	grep -qF 'REPLACE-SYNC DRIFT' <<<"$out" || {
		echo "FAIL: coverage branch missing the pinned DRIFT tag"
		printf '%s\n' "$out"
		exit 1
	}
	echo "ok: coverage branch fires on the missing-replace fixture"

	# pins: negative-battery-liveness -- a replace line whose target dir
	# is gone must fire the stale branch naming the line.
	sed 's#internal/retryx => ../../internal/retryx#internal/retryx => ../../internal/nonexistent-target#' \
		"$real_gomod" >"$tmp/stale.go.mod"
	rc=0
	out="$(TQ_REPLACE_SYNC_EMBED_GOMOD="$tmp/stale.go.mod" TQ_REPLACE_SYNC_EMBED_DIR="$SCRIPT_DIR/../examples/embed" "$self_abs" 2>&1)" || rc=$?
	if [ "$rc" -ne 1 ]; then
		echo "FAIL: stale-replace battery (want rc=1, got rc=$rc)"
		printf '%s\n' "$out"
		exit 1
	fi
	grep -qF 'stale replace line' <<<"$out" || {
		echo "FAIL: liveness branch does not name the stale line"
		printf '%s\n' "$out"
		exit 1
	}
	echo "ok: liveness branch fires on the stale-target fixture"

	# pins: negative-battery-frozen -- a smoke that freezes tq module
	# paths into its replace list (derivation + guard otherwise intact)
	# must fire the frozen-list branch.
	cat >"$tmp/frozen-smoke.sh" <<'EOT'
#!/usr/bin/env bash
# pins: the frozen line stays BARE (unquoted) -- a real frozen list is
# bare code, and the gate's quote-strip must not hide it.
derived="$(cd "$repo_root/examples/embed" && sed -n '/^replace (/,/^)/p' go.mod)"
echo "EMPTY REPLACE LIST guard marker present"
github.com/larsartmann/go-taskqueue/internal/task => hand-copied list
EOT
	rc=0
	out="$(TQ_REPLACE_SYNC_SMOKE="$tmp/frozen-smoke.sh" "$self_abs" 2>&1)" || rc=$?
	if [ "$rc" -ne 1 ]; then
		echo "FAIL: frozen-smoke battery (want rc=1, got rc=$rc)"
		printf '%s\n' "$out"
		exit 1
	fi
	grep -qF 'hand-frozen' <<<"$out" || {
		echo "FAIL: frozen-list branch does not fire on the fixture"
		printf '%s\n' "$out"
		exit 1
	}
	echo "ok: frozen-list branch fires on the frozen-smoke fixture"

	# pins: negative-battery-noderivation -- a smoke without the live
	# replace-block extraction must fire the derivation branch.
	cat >"$tmp/noderive-smoke.sh" <<'EOT'
#!/usr/bin/env bash
replace_block="hand written list"
echo "EMPTY REPLACE LIST guard marker present"
EOT
	rc=0
	out="$(TQ_REPLACE_SYNC_SMOKE="$tmp/noderive-smoke.sh" "$self_abs" 2>&1)" || rc=$?
	if [ "$rc" -ne 1 ]; then
		echo "FAIL: no-derivation battery (want rc=1, got rc=$rc)"
		printf '%s\n' "$out"
		exit 1
	fi
	grep -qF 'no longer DERIVES' <<<"$out" || {
		echo "FAIL: derivation branch does not fire on the fixture"
		printf '%s\n' "$out"
		exit 1
	}
	echo "ok: derivation branch fires on the extraction-less fixture"

	# pins: negative-battery-noguard -- a deriving smoke without the
	# empty-derivation guard must fire the guard branch.
	cat >"$tmp/noguard-smoke.sh" <<'EOT'
#!/usr/bin/env bash
derived="$(cd "$repo_root/examples/embed" && sed -n '/^replace (/,/^)/p' go.mod)"
EOT
	rc=0
	out="$(TQ_REPLACE_SYNC_SMOKE="$tmp/noguard-smoke.sh" "$self_abs" 2>&1)" || rc=$?
	if [ "$rc" -ne 1 ]; then
		echo "FAIL: no-guard battery (want rc=1, got rc=$rc)"
		printf '%s\n' "$out"
		exit 1
	fi
	grep -qF 'empty-derivation guard' <<<"$out" || {
		echo "FAIL: guard branch does not fire on the fixture"
		printf '%s\n' "$out"
		exit 1
	}
	echo "ok: guard branch fires on the guard-less fixture"

	# pins: ok-summary -- the real run must emit the PINNED summary
	# format (counts vary; the format is the contract).
	rc=0
	out="$("$self_abs" 2>&1)" || rc=$?
	if [ "$rc" -ne 0 ]; then
		echo "FAIL: real-run battery (want rc=0, got rc=$rc)"
		printf '%s\n' "$out"
		exit 1
	fi
	if grep -Eq '^embed-replace-sync ok: [0-9]+ tq requires covered by [0-9]+ replace lines; smoke derivation intact$' <<<"$out"; then
		echo "ok: real-run summary matches the pinned format"
	else
		echo "FAIL: real-run summary missing or malformed"
		printf '%s\n' "$out"
		exit 1
	fi
	echo "embed-replace-sync self-test ok (summary format pin + negative counter battery)"
}

if [ "${1:-}" = "--self-test" ]; then
	self_test
	exit 0
fi

run_checks
