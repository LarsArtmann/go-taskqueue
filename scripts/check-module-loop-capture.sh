#!/usr/bin/env bash
# Module-loop capture audit (TODO_LIST 2026-09-19 07:00 verify window row):
# the per-module loops in ci.yml and ci-local.sh must be fed by the
# disk-derived enumeration (scripts/for-each-module.sh). If a future edit
# reverts a loop to word-position substitution (`for m in $(...)`) or swaps
# the `mods="$(./scripts/for-each-module.sh)"` capture for a hand-rolled
# enumeration, new sub-modules silently drop out of every gate. This audit
# pins the capture shape mechanically:
#   1. every `for m in <list>` loop must iterate `$mods` (no inline $(...))
#   2. every `mods=` assignment must be the for-each-module capture
#   3. a file with loops must carry at least one such capture
set -euo pipefail
cd "$(dirname "$0")/.."

targets=(scripts/ci-local.sh .github/workflows/ci.yml)
rc=0

for f in "${targets[@]}"; do
	loops=0
	captures=0
	while IFS= read -r line; do
		ln=${line%%:*}
		text=${line#*:}
		case "$text" in
		*"for m in "*)
			case "$text" in
			*"for m in \$mods"*) ;;
			*)
				echo "BAD LOOP: $f:$ln — module loop must iterate \$mods (for-each-module capture), got: $text"
				rc=1
				;;
			esac
			loops=$((loops + 1))
			;;
		esac
		case "$text" in
		*mods=*)
			case "$text" in
			*'"$(./scripts/for-each-module.sh)"'*) captures=$((captures + 1)) ;;
			*)
				echo "BAD CAPTURE: $f:$ln — mods must be captured from scripts/for-each-module.sh, got: $text"
				rc=1
				;;
			esac
			;;
		esac
	done < <(grep -n -e 'for m in ' -e 'mods=' "$f" || true)
	if [ "$loops" -gt 0 ] && [ "$captures" -eq 0 ]; then
		echo "NO CAPTURE: $f has $loops module loop(s) but no mods=\"\$(./scripts/for-each-module.sh)\" capture"
		rc=1
	fi
	echo "module-loop-capture: $f loops=$loops captures=$captures"
done

if [ "$rc" = 0 ]; then
	echo "module-loop-capture ok: every module loop is fed by the for-each-module capture"
fi
exit "$rc"
