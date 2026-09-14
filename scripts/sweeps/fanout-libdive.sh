#!/usr/bin/env bash
# fanout-libdive.sh — mint templ-components library-maximization tasks for a
# consumer wave, dedup-keyed so re-runs never duplicate work (G1 default home:
# zero-dep script inside tq; the pdg-bridge option stays available via owner
# override, see docs/planning/2026-09-14_12-34_...execution-plan.md §8.1).
#
# SAFETY: --db is REQUIRED. The production dogfood journal is reachable via
# TQ_DB env inheritance; a bare run must never mint there.
#
# Usage:
#   fanout-libdive.sh --wave 1 --target v1.17.0 --db /path/sweep.db \
#     [--dry-run] [--delay-step 7m] [--start-delay 0m] \
#     [--projects-dir ~/projects] [--cohort <tsv>] [--template <tmpl>] \
#     [--import <who-uses-tree.txt>]      # parse a pdg who-uses tree to TSV
#   fanout-libdive.sh --self-test         # glyph-robust parse pin
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cohort_default="$script_dir/templ-components-cohort.tsv"
template_default="$script_dir/templ-components-prompt.tmpl"

wave="" target="" db="" dry_run=false delay_step="7m" start_delay="0m"
projects_dir="${HOME}/projects" cohort="" template="" import_file="" self_test=false

while [ $# -gt 0 ]; do
	case "$1" in
	--wave) wave="$2"; shift 2 ;;
	--target) target="$2"; shift 2 ;;
	--db) db="$2"; shift 2 ;;
	--dry-run) dry_run=true; shift ;;
	--delay-step) delay_step="$2"; shift 2 ;;
	--start-delay) start_delay="$2"; shift 2 ;;
	--projects-dir) projects_dir="$2"; shift 2 ;;
	--cohort) cohort="$2"; shift 2 ;;
	--template) template="$2"; shift 2 ;;
	--import) import_file="$2"; shift 2 ;;
	--self-test) self_test=true; shift ;;
	*) echo "unknown flag: $1" >&2; exit 2 ;;
	esac
done

# parse_who_uses <tree-file> — extract "key<TAB>version" rows from a pdg
# who-uses tree. Glyph-robust: tree connectors (│ ├ ─ └) and box-drawing
# prefixes are stripped before matching; version sections set the context.
# The target module line (contains "templ-components") and blank lines are
# skipped. Pinned by the --self-test below.
parse_who_uses() {
	awk '
	{
		line = $0
		gsub(/[│|├└─┬+]/, " ", line)
		sub(/^[[:space:]]+/, "", line)
		sub(/[[:space:]]+$/, "", line)
		if (line == "") next
		if (line ~ /^\(/) next
		if (index(line, "templ-components") > 0) next
		if (line ~ /^v[0-9]+\.[0-9]+\.[0-9]+$/) { version = line; next }
		if (version != "") print line "\t" version
	}' "$1"
}

if $self_test; then
	tmp="$(mktemp)"
	cat >"$tmp" <<'TREE'
templ-components (github.com/larsartmann/templ-components)
├── v1.17.0
│   ├── DiscordSync
│   ├── bank-sync
│   └── zlota44
├── v1.8.3
│   └── browser-history
└── (2 more versions)
TREE
	got="$(parse_who_uses "$tmp")"
	rm -f "$tmp"
	expected="$(printf 'DiscordSync\tv1.17.0\nbank-sync\tv1.17.0\nzlota44\tv1.17.0\nbrowser-history\tv1.8.3')"
	if [ "$got" != "$expected" ]; then
		echo "self-test FAILED:" >&2
		diff <(printf '%s\n' "$expected") <(printf '%s\n' "$got") >&2 || true
		exit 1
	fi
	echo "self-test ok (glyph-robust tree parse)"
	exit 0
fi

if [ -n "$import_file" ]; then
	parse_who_uses "$import_file"
	exit 0
fi

[ -n "$wave" ] || { echo "--wave is required" >&2; exit 2; }
[ -n "$target" ] || { echo "--target is required (e.g. v1.17.0)" >&2; exit 2; }
[ -n "$db" ] || {
	echo "--db is REQUIRED (never mint into an inherited TQ_DB / production journal)" >&2
	exit 2
}
cohort="${cohort:-$cohort_default}"
template="${template:-$template_default}"
[ -f "$cohort" ] || { echo "cohort fixture not found: $cohort" >&2; exit 2; }
[ -f "$template" ] || { echo "prompt template not found: $template" >&2; exit 2; }

command -v jq >/dev/null || { echo "jq is required" >&2; exit 2; }
tq_bin="${TQ_BIN:-tq}"

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

i=0
minted=0
while IFS=$'\t' read -r key dir pin_gomod pin_whouses repo_wave note; do
	case "$key" in ""|"#"*) continue ;; esac
	[ "$repo_wave" = "$wave" ] || continue

	repo_abs="$projects_dir/$dir"
	[ -d "$repo_abs" ] || { echo "SKIP $key: dir missing: $repo_abs" >&2; continue; }
	[ -f "$repo_abs/go.mod" ] || { echo "SKIP $key: no go.mod in $repo_abs" >&2; continue; }

	delay="$(awk -v i="$i" -v step="$delay_step" 'BEGIN {
		num = step + 0
		unit = substr(step, length(step), 1)
		secs = (unit == "m") ? num * 60 : num
		printf "%dm", int(secs * i / 60)
	}')"

	sed -e "s|{{REPO_ABS}}|$repo_abs|g" \
		-e "s|{{REPO}}|$key|g" \
		-e "s|{{CURRENT}}|$pin_gomod|g" \
		-e "s|{{TARGET}}|$target|g" \
		"$template" >"$tmpdir/prompt.txt"

	jq -n --arg repo "$key" --arg prompt "$(cat "$tmpdir/prompt.txt")" \
		'{repo: $repo, prompt: $prompt, yolo: true, timeout_minutes: 60}' >"$tmpdir/payload.json"

	dedup="libdive:templ-components:$key@$target"

	if $dry_run; then
		echo "DRY-RUN enqueue --project $key --type agent --dedup-key $dedup --delay $delay (wave $repo_wave, pin $pin_gomod -> $target)"
	else
		tid="$("$tq_bin" enqueue --project "$key" --type agent \
			--payload @"$tmpdir/payload.json" \
			--dedup-key "$dedup" \
			--delay "$delay" \
			--db "$db")"
		echo "minted $key -> $tid (dedup $dedup, delay $delay)"
	fi

	minted=$((minted + 1))
	i=$((i + 1))
done <"$cohort"

echo "wave $wave -> $target: $minted task(s) $([ "$dry_run" = true ] && echo "(dry-run)")"
