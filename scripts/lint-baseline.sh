#!/usr/bin/env bash
# Regenerate or verify the advisory-lint baseline (.golangci-baseline.txt):
# one line per sub-module (root included) and linter with the current
# finding count. The baseline is checked in so the pre-push gate can
# distinguish the documented advisory sea (AGENTS.md "golangci-lint is
# advisory") from unbounded growth.
#
#   scripts/lint-baseline.sh            regenerate (deliberate, after policy changes)
#   scripts/lint-baseline.sh --check    gate: exit 1 on baseline GROWTH
#                                       (new finding classes included); shrink
#                                       is reported advisory-only, never fails
#
# Takes ~3 minutes (full root + sub-module loop; golangci-lint's cache makes
# a post-lint --check pass cheap).
set -euo pipefail
cd "$(dirname "$0")/.."
export GOEXPERIMENT=jsonv2

mode="${1:---regen}"
if [[ "$mode" != "--check" && "$mode" != "--regen" ]]; then
	echo "usage: $0 [--check|--regen]" >&2
	exit 2
fi

out="$(mktemp "${TMPDIR:-/tmp}/tq-baseline.XXXXXX")"
trap 'rm -f "$out"' EXIT

record() {
	local module="$1" lint_out="$2"
	printf '%s\n' "$lint_out" | awk -v mod="$module" '/^\* /{gsub(/^\* /,""); gsub(/^[ ]+/,""); print mod "\t" $1 "\t" $2}' >>"$out"
}

echo "== root" >&2
record root "$(golangci-lint run ./... 2>&1 || true)"
for m in $(find internal task journal queue executor worker -name go.mod | sed 's|/go.mod$||' | sort); do
	echo "== $m" >&2
	record "$m" "$(cd "$m" && GOWORK=off golangci-lint run ./... 2>&1 || true)"
done

sort -u "$out" -o "$out"

if [[ "$mode" == "--check" ]]; then
	baseline=".golangci-baseline.txt"
	if [[ ! -f "$baseline" ]]; then
		echo "lint-baseline: $baseline missing — run scripts/lint-baseline.sh to seed it" >&2
		exit 1
	fi

	violations="$(mktemp "${TMPDIR:-/tmp}/tq-baseline-viol.XXXXXX")"
	trap 'rm -f "$out" "$violations"' EXIT

	# Growth = fresh count strictly above the baseline row.
	awk -F'\t' 'NR==FNR { base[$1 FS $2] = $3; next }
		($1 FS $2) in base && $3 > base[$1 FS $2] { print $1 "\t" $2 "\t" base[$1 FS $2] "\t" $3 }' \
		"$baseline" "$out" >>"$violations"

	# New finding classes: (module, linter) pairs the baseline does not know.
	comm -13 <(cut -f1,2 "$baseline" | sort -s) <(cut -f1,2 "$out" | sort -s) |
		awk -F'\t' -v v="$violations" '{ print $1 "\t" $2 "\tNEW\t(new class)" > v }'

	if [[ -s "$violations" ]]; then
		echo "lint-baseline: GROWTH beyond the committed baseline (gate failure):" >&2
		echo "module	linter	baseline	now" >&2
		cat "$violations" >&2
		# Attribution aid (04-31 §f15): name the files carrying each growing
		# linter's findings so triage starts informed. One lint run per
		# affected module, reused across that module's rows.
		modules="$(cut -f1 "$violations" | sort -u)"
		for module in $modules; do
			if [[ "$module" == "root" ]]; then
				lint_out="$(golangci-lint run ./... 2>/dev/null || true)"
			else
				lint_out="$(cd "$module" && GOWORK=off golangci-lint run ./... 2>/dev/null || true)"
			fi
			while IFS=$'\t' read -r m linter _; do
				[[ "$m" == "$module" ]] || continue
				linter="${linter%:}"
				files="$(printf '%s\n' "$lint_out" | grep -F "($linter)" | cut -d: -f1 | sort -u || true)"
				if [[ -n "$files" ]]; then
					echo "  $m/$linter carries findings in:" >&2
					printf '%s\n' "$files" | sed 's/^/    /' >&2
				else
					echo "  $m/$linter: no per-file output (re-run golangci-lint manually)" >&2
				fi
			done <"$violations"
		done
		echo "fix the new findings, or regenerate deliberately (scripts/lint-baseline.sh) if a policy change owns them" >&2
		exit 1
	fi

	shrink="$(awk -F'\t' 'NR==FNR { base[$1 FS $2] = $3; next }
		($1 FS $2) in base && $3 < base[$1 FS $2] { print "  " $1 " " $2 ": " base[$1 FS $2] " -> " $3 }' \
		"$baseline" "$out")"
	if [[ -n "$shrink" ]]; then
		echo "lint-baseline: shrunk (advisory — consider regenerating):" >&2
		printf '%s\n' "$shrink" >&2
	fi

	echo "lint-baseline: within baseline ($(awk -F'\t' '{s+=$3} END{print s+0}' "$out") findings vs baseline $(awk -F'\t' '{s+=$3} END{print s+0}' "$baseline"))"
	exit 0
fi

sort -u "$out" -o .golangci-baseline.txt
total="$(awk -F'\t' '{s+=$3} END{print s+0}' .golangci-baseline.txt)"
echo "baseline written: $(grep -c . .golangci-baseline.txt) module/linter rows, $total findings total"
