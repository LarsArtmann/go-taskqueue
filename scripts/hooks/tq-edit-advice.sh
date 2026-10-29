#!/usr/bin/env bash
# tq-edit-advice.sh — PreToolUse hook for edit/write/multiedit (go-taskqueue).
#
# Injects the repo's per-path conventions exactly when a matching file is
# touched, so the rule survives context pressure without living in every
# prompt. Advisory only (context injection, never a decision) — the normal
# permission flow still runs.
#
# Noise policy: silent (exit 0, no stdout) unless the path matches a rule.
#
# Hook contract: CRUSH_TOOL_INPUT_FILE_PATH carries the target path; exit
# 0 + JSON stdout = advisory context. --self-test runs the pinned fixture
# battery (fleet convention).
set -euo pipefail

if [ "${1:-}" = "--self-test" ]; then
	self_abs="$(cd "$(dirname "$0")" && pwd)/$(basename "$0")"
	# tmp is deliberately global: the EXIT trap fires after this block's
	# function calls returned, and set -u would kill it on a local name.
	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' EXIT
	mkdir -p "$tmp/docs/status"
	: >"$tmp/docs/status/2026-10-01_00-00_exists.md"

	st_pass=0 st_fail=0 n_adv=0 n_sil=0
	rcase() {
		# $1 expect: ctx:SUBSTR | silent; $2 desc; $3 file path
		local expect="$1" desc="$2" out rc=0
		shift 2
		out="$(CRUSH_TOOL_INPUT_FILE_PATH="$1" "$self_abs" 2>&1)" || rc=$?
		case "$expect" in
			ctx:*)
				n_adv=$((n_adv + 1))
				local want="${expect#ctx:}"
				if [ "$rc" = 0 ] && printf '%s' "$out" | grep -qF -- "$want"; then
					st_pass=$((st_pass + 1))
				else
					st_fail=$((st_fail + 1))
					echo "FAIL: $desc (want advisory containing '$want'; rc=$rc)"
					printf '%s\n' "$out"
				fi
				;;
			silent)
				n_sil=$((n_sil + 1))
				if [ "$rc" = 0 ] && [ -z "$out" ]; then
					st_pass=$((st_pass + 1))
				else
					st_fail=$((st_fail + 1))
					echo "FAIL: $desc (want silence, rc=$rc)"
					printf '%s\n' "$out"
				fi
				;;
		esac
	}

	rcase ctx:'templ generate' 'templ edit advice' "$tmp/layout.templ"
	rcase ctx:'machine-consumed' 'TODO_LIST edit advice' "$tmp/TODO_LIST.md"
	rcase ctx:'size-guarded' 'AGENTS edit advice' "$tmp/AGENTS.md"
	rcase ctx:'### Internal' 'CHANGELOG edit advice' "$tmp/CHANGELOG.md"
	rcase ctx:'check-go-mods.sh' 'go.mod edit advice' "$tmp/go.mod"
	rcase ctx:'date +%Y-%m-%d_%H-%M' 'docs/status creation advice' "$tmp/docs/status/2026-10-29_10-00_task-1.md"
	rcase silent 'existing docs/status edit stays silent' "$tmp/docs/status/2026-10-01_00-00_exists.md"
	rcase silent 'README edit stays silent' "$tmp/README.md"
	rcase silent 'Go file edit stays silent' "$tmp/internal/foo.go"

	if [ "$st_fail" -ne 0 ]; then
		echo "tq-edit-advice self-test FAILED ($st_fail of $((st_pass + st_fail)) cases)"
		exit 1
	fi
	echo "tq-edit-advice self-test ok ($st_pass cases: advise=$n_adv silent=$n_sil)"
	exit 0
fi

fp="${CRUSH_TOOL_INPUT_FILE_PATH:-}"
[ -n "$fp" ] || exit 0
base="$(basename "$fp")"

note=""

case "$base" in
	*.templ)
		note="after saving: run 'templ generate' from the REPO ROOT and 'nix run .#webui-css', then COMMIT the generated *_templ.go and minified app.css (both are committed artifacts; AGENTS.md §Conventions)."
		;;
	TODO_LIST.md)
		note="TODO_LIST.md is machine-consumed: '- [ ]' one per line, never tables; blocked items use '— BLOCKED: <reason>'; items must be agent-executable (check-todo-list.sh gates this)."
		;;
	AGENTS.md)
		note="AGENTS.md is size-guarded (TestAgentsDocSizeGuard) and dense by design: prefer replacing stale bullets over appending; keep the reader-route lanes intact."
		;;
	CHANGELOG.md)
		note="CHANGELOG is append-only; [Unreleased] carries OPERATOR-VISIBLE changes only — repo-internal rows go under '### Internal'; behavior changes get an explicit '(BEHAVIOR CHANGE ...)' flag (O14, check-release-docs.sh)."
		;;
	go.mod)
		note="go.mod: NEVER lower the 'go' directive (check-go-mods.sh); a bumped directive requires 'go work edit -go=<ver>' on cmd/tq/go.work in the same change; per-module tidy + vendor sync follow (check-gomod-vendor-sync.sh)."
		;;
esac

if [ -z "$note" ]; then
	case "$fp" in
		*/docs/status/*)
			if [ ! -f "$fp" ]; then
				note="new docs/status report: run 'date +%Y-%m-%d_%H-%M' BEFORE composing the filename (a guessed <ts> lies); follow the a)-g) skeleton; index on creation (check-status-index.sh); daemon-folded reports need the AMEND MANEUVER."
			fi
			;;
	esac
fi

if [ -n "$note" ]; then
	jq -n --arg c "$note" '{context: $c}'
fi
exit 0
