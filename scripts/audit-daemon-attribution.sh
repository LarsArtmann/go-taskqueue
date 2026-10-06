#!/usr/bin/env bash
# audit-daemon-attribution.sh: doctor-style attribution audit over the
# auto-commit daemon's footer-less `chore:` sweeps (00-55 report §f2).
#
# The daemon sweeps work into commits whose subject matches
# "chore: auto-commit N changed file(s) (heuristic)" and that carry no
# Task-Queue-ID footer, so the queue cannot attribute them — every
# shipped item whose code rode such a commit leaves the cross-reference
# resting on the report commit alone (the §b2 class). This audit maps
# every such sweep in a range to task ids via the TWO established
# attribution channels:
#   1. a docs/status/**/<...>_task-<id>.md report file inside the sweep;
#   2. a later footer-carrying MARKER commit whose body cites the
#      sweep's sha (the pushed-history heal precedent: an empty
#      footered commit naming the swept shas).
# and classifies each sweep:
#   attributed             — at least one task id resolved (either channel)
#   report-only            — no id, but every touched file is under
#                            docs/status/ (window reports, index rows)
#   unattributed shipping  — no id AND at least one file outside
#                            docs/status/ (code, scripts, CHANGELOG, …)
# Exit codes (doctor semantics): 0 = healthy or warnings only, 1 = at
# least one unattributed shipping commit; every result prints first.
#
# Read-only: no history rewrite, no writes beyond stdout. Works on
# pushed and unpushed history alike — healing is a separate, reviewed
# operation (scripts/heal-daemon-sweep.sh for unpushed ranges; a
# footered empty marker commit for pushed history).
#
# Usage:
#   scripts/audit-daemon-attribution.sh [--from <ref>] [--all-chore] [--json] [-v]
#   scripts/audit-daemon-attribution.sh --self-test
#
#   --from <ref>  audit only <ref>..HEAD (default: the root commit).
#   --all-chore   widen the daemon heuristic to EVERY footer-less
#                 `chore:` commit (manual chores included).
#   --json        one JSON row per audited sweep on stdout (JSONL);
#                 the summary moves to stderr so jq sees pure rows.
#   -v|--verbose  also print attributed and report-only detail lines.
set -uo pipefail

SELF=$(cd "$(dirname "$0")" && pwd)/$(basename "$0")

die() {
	echo "audit-daemon-attribution: $*" >&2
	exit 1
}

usage() {
	echo "usage: $0 [--from <ref>] [--all-chore] [--json] [-v|--verbose]" >&2
	echo "       $0 --self-test" >&2
	exit 2
}

json_escape() {
	local s=$1
	s=${s//\\/\\\\}
	s=${s//\"/\\\"}
	s=${s//$'\t'/ }
	s=${s//$'\r'/ }
	printf '%s' "$s"
}

# ---- arguments -----------------------------------------------------------
FROM=""
ALL_CHORE=0
JSON=0
VERBOSE=0
SELF_TEST=0
while [ $# -gt 0 ]; do
	case "$1" in
	--from)
		[ $# -ge 2 ] || usage
		FROM=$2
		shift 2
		;;
	--all-chore)
		ALL_CHORE=1
		shift
		;;
	--json)
		JSON=1
		shift
		;;
	-v | --verbose)
		VERBOSE=1
		shift
		;;
	--self-test)
		SELF_TEST=1
		shift
		;;
	*)
		usage
		;;
	esac
done

# ---- self-test: scratch-repo fixture; the real repo is never touched -----
run_self_test() {
	tmp=$(mktemp -d) || die "mktemp failed"
	trap 'rm -rf "$tmp"' EXIT

	local repo="$tmp/repo"
	mkdir -p "$repo/docs/status" || die "mkdir failed"
	git -C "$repo" init -q
	git -C "$repo" config user.email audit-selftest@example.invalid
	git -C "$repo" config user.name "audit self-test"
	git -C "$repo" config commit.gpgsign false

	local id_report=aaaabbbb111122223333444455556666
	local id_marker=bbbbcccc222233334444555566667777
	local id_foot=ccccdddd333344445555666677778888
	local daemon_subj="chore: auto-commit 2 changed file(s) (heuristic)"

	git_commit() {
		git -C "$repo" add -A
		git -C "$repo" commit -q "$@" || die "fixture commit failed"
	}

	printf 'index\n' >"$repo/README.md"
	git_commit -m init

	printf 'closeout\n' >"$repo/docs/status/2026-01-01_00-00_task-${id_report}.md"
	printf 'index row\n' >>"$repo/docs/status/README.md"
	git_commit -m "$daemon_subj"
	local sha_att
	sha_att=$(git -C "$repo" rev-parse HEAD)

	printf 'window report\n' >"$repo/docs/status/2026-01-01_00-01_window.md"
	git_commit -m "$daemon_subj"

	printf 'package main\n' >"$repo/flagged.go"
	git_commit -m "chore: auto-commit 1 changed file(s) (heuristic)"
	local sha_flag
	sha_flag=$(git -C "$repo" rev-parse HEAD)

	printf 'package healed\n' >"$repo/healed.go"
	git_commit -m "$daemon_subj"
	local sha_heal
	sha_heal=$(git -C "$repo" rev-parse HEAD)

	git -C "$repo" commit -q --allow-empty -m "$(printf '%s\n\n%s' \
		"The work files landed in the auto-commit daemon's footer-less sweep ${sha_heal:0:7}; this marker carries the queue attribution without rewriting shared history." \
		"Task-Queue-ID: $id_marker")"

	printf 'done\n' >"$repo/done.go"
	git_commit -m "$(printf '%s\n\nTask-Queue-ID: %s' "footed task commit" "$id_foot")"

	printf 'tidy\n' >"$repo/tidy.txt"
	git_commit -m "chore: manual tidy"

	local pass=0
	local fail=0
	check_eq() {
		local label=$1 want=$2 got=$3
		if [ "$want" = "$got" ]; then
			pass=$((pass + 1))
		else
			fail=$((fail + 1))
			echo "FAIL: $label: want [$want] got [$got]"
		fi
	}

	local out rc
	out=$(
		cd "$repo" && "$SELF"
		echo "rc=$?"
	)
	rc=${out##*rc=}
	out=${out%rc=*}
	check_eq "default run exit (one unattributed shipping sweep)" "1" "$rc"
	check_eq "default summary counts" \
		"summary: scanned=4 attributed=2 report-only=1 unattributed-shipping=1" \
		"$(printf '%s\n' "$out" | grep '^summary:')"
	check_eq "unhealed shipping sweep flagged" "1" \
		"$(printf '%s\n' "$out" | grep -c "FAIL ${sha_flag:0:7}")"
	check_eq "marker-healed sweep not flagged" "0" \
		"$(printf '%s\n' "$out" | grep -c "FAIL ${sha_heal:0:7}")"
	check_eq "manual chore outside daemon heuristic (not scanned)" "0" \
		"$(printf '%s\n' "$out" | grep -c "FAIL .* manual tidy")"

	out=$(
		cd "$repo" && "$SELF" --json 2>/dev/null
		echo "rc=$?"
	)
	rc=${out##*rc=}
	out=${out%rc=*}
	check_eq "json run exit" "1" "$rc"
	check_eq "json rows" "4" "$(printf '%s\n' "$out" | grep -c '"class":')"
	check_eq "json attributed rows" "2" "$(printf '%s\n' "$out" | grep -c '"class":"attributed"')"
	check_eq "marker channel in json" "1" \
		"$(printf '%s\n' "$out" | grep "$sha_heal" | grep -cF "\"task_ids\":[\"$id_marker\"]")"
	check_eq "report channel in json" "1" \
		"$(printf '%s\n' "$out" | grep "$sha_att" | grep -cF "\"task_ids\":[\"$id_report\"]")"

	out=$(
		cd "$repo" && "$SELF" --from "$(git -C "$repo" rev-parse HEAD~2)"
		echo "rc=$?"
	)
	rc=${out##*rc=}
	out=${out%rc=*}
	check_eq "range --from exit (no daemon sweeps in range)" "0" "$rc"
	check_eq "range summary counts" \
		"summary: scanned=0 attributed=0 report-only=0 unattributed-shipping=0" \
		"$(printf '%s\n' "$out" | grep '^summary:')"

	out=$(
		cd "$repo" && "$SELF" --all-chore
		echo "rc=$?"
	)
	rc=${out##*rc=}
	out=${out%rc=*}
	check_eq "all-chore summary (manual chore joins the audit)" \
		"summary: scanned=5 attributed=2 report-only=1 unattributed-shipping=2" \
		"$(printf '%s\n' "$out" | grep '^summary:')"

	echo "self-test: $pass passed, $fail failed"
	[ "$fail" -eq 0 ]
}

# ---- main ------------------------------------------------------------------
command -v git >/dev/null 2>&1 || die "git not found in PATH"
if [ "$SELF_TEST" -eq 1 ]; then
	run_self_test
	exit $?
fi

git rev-parse --verify HEAD >/dev/null 2>&1 || die "not a git repository (or no commits)"

if [ -n "$FROM" ]; then
	git rev-parse --verify --quiet "$FROM" >/dev/null || die "base ref '$FROM' not found"
	RANGE="$FROM..HEAD"
	RANGE_LABEL="$FROM..HEAD"
else
	RANGE="HEAD"
	RANGE_LABEL="full history"
fi

# ---- pass 1: sha | subject | body ------------------------------------------
declare -a order=()
declare -A subj_of=()
declare -A is_daemon=()
declare -A footer_body_of=()

mapfile -d '' -t meta_records < <(git log --no-merges --format='%x00%H%x02%B' "$RANGE")

for rec in "${meta_records[@]}"; do
	sha=${rec%%$'\x02'*}
	[[ $sha =~ ^[0-9a-f]{40}$ ]] || continue
	message=${rec#*$'\x02'}
	subject=${message%%$'\n'*}
	order+=("$sha")
	subj_of[$sha]=$subject

	footer_id=""
	while IFS= read -r line; do
		case $line in
		'Task-Queue-ID:'\ *)
			cand=${line#Task-Queue-ID: }
			cand=${cand//[[:space:]]/}
			if [[ $cand =~ ^[0-9a-f]+$ ]]; then
				footer_id=$cand
			fi
			;;
		esac
	done <<<"$message"

	daemon=0
	if [[ $subject =~ ^chore:\ auto-commit\ [0-9]+\ changed\ file ]]; then
		daemon=1
	elif [ "$ALL_CHORE" -eq 1 ] && [[ $subject == 'chore:'* ]]; then
		daemon=1
	fi
	[ -n "$footer_id" ] && daemon=0
	is_daemon[$sha]=$daemon
	if [ -n "$footer_id" ]; then
		footer_body_of[$sha]=$footer_id$'\x02'$message
	fi
done

# ---- pass 2: files per commit + report-file task ids ------------------------
declare -A files_of=()
declare -A report_ids=()

mapfile -t file_lines < <(git log --no-merges --format='%H' --name-only "$RANGE")

cur=""
seen_blank=0
for line in "${file_lines[@]}"; do
	if [[ $line =~ ^[0-9a-f]{40}$ ]]; then
		cur=$line
		seen_blank=0
		continue
	fi
	if [ -z "$line" ]; then
		seen_blank=1
		continue
	fi
	if [ -n "$cur" ] && [ "$seen_blank" -eq 1 ]; then
		files_of[$cur]=${files_of[$cur]:-}$'\n'$line
		if [[ $line == docs/status/*_task-*.md || $line == docs/status/tasks/*_task-*.md ]]; then
			rid=${line##*_task-}
			rid=${rid%.md}
			if [[ $rid =~ ^[0-9a-f]+$ ]]; then
				report_ids[$cur]=${report_ids[$cur]:-}" $rid"
			fi
		fi
	fi
done

# ---- marker channel: footer-carrying commits citing sweep shas --------------
# Sweep shas indexed by their 7-char prefix (git's minimum abbreviation);
# a token matches when its own leading 7 chars hit the index.
declare -A daemon_by_prefix=()
for sha in "${order[@]}"; do
	[ "${is_daemon[$sha]:-0}" -eq 1 ] || continue
	daemon_by_prefix[${sha:0:7}]=${daemon_by_prefix[${sha:0:7}]:-}"$sha"
done

declare -A marker_ids=()
for sha in "${!footer_body_of[@]}"; do
	entry=${footer_body_of[$sha]}
	fid=${entry%%$'\x02'*}
	message=${entry#*$'\x02'}
	# Tokenize the whole message in-process (subject citations count too):
	# every non-hex run becomes a separator.
	raw=${message//[^0-9a-f]/ }
	read -r -a toks <<<"$raw"
	for tok in "${toks[@]}"; do
		[ ${#tok} -ge 7 ] || continue
		tok=${tok:0:7}
		hit=${daemon_by_prefix[$tok]:-}
		[ -n "$hit" ] || continue
		read -r -a hits <<<"$hit"
		for d in "${hits[@]}"; do
			marker_ids[$d]=${marker_ids[$d]:-}" $fid"
		done
	done
done

# ---- classify ----------------------------------------------------------------
scanned=0
attributed=0
report_only=0
shipping=0
declare -A class_of=()
declare -A ids_of=()
declare -a shipping_rows=()
declare -a attributed_rows=()
declare -a report_rows=()

for sha in "${order[@]}"; do
	[ "${is_daemon[$sha]:-0}" -eq 1 ] || continue
	scanned=$((scanned + 1))

	ids=${report_ids[$sha]:-}${marker_ids[$sha]:-}
	ids_of[$sha]=$ids

	class=attributed
	if [ -z "${ids//[[:space:]]/}" ]; then
		class=report-only
		while IFS= read -r f; do
			[ -n "$f" ] || continue
			case $f in
			docs/status/*) ;;
			*)
				class=unattributed-shipping
				break
				;;
			esac
		done <<<"${files_of[$sha]:-}"
	fi
	class_of[$sha]=$class

	case $class in
	attributed)
		attributed=$((attributed + 1))
		trimmed=$(printf '%s' "$ids" | tr -s ' ')
		attributed_rows+=("$sha${trimmed} ${subj_of[$sha]}")
		;;
	report-only)
		report_only=$((report_only + 1))
		report_rows+=("$sha ${subj_of[$sha]}")
		;;
	*)
		shipping=$((shipping + 1))
		shipping_rows+=("$sha ${subj_of[$sha]}")
		;;
	esac
done

# ---- report -------------------------------------------------------------------
if [ "$JSON" -eq 1 ]; then
	for sha in "${order[@]}"; do
		[ "${is_daemon[$sha]:-0}" -eq 1 ] || continue
		id_json=""
		read -r -a id_arr <<<"${ids_of[$sha]}"
		for id in "${id_arr[@]}"; do
			[ -n "$id" ] || continue
			id_json+=",\"$(json_escape "$id")\""
		done
		id_json="[${id_json#,}]"

		file_json=""
		while IFS= read -r f; do
			[ -n "$f" ] || continue
			file_json+=",\"$(json_escape "$f")\""
		done <<<"${files_of[$sha]:-}"
		file_json="[${file_json#,}]"

		printf '{"commit":"%s","subject":"%s","class":"%s","task_ids":%s,"files":%s}\n' \
			"$sha" "$(json_escape "${subj_of[$sha]}")" "${class_of[$sha]}" "$id_json" "$file_json"
	done
	printf 'summary: scanned=%d attributed=%d report-only=%d unattributed-shipping=%d\n' \
		"$scanned" "$attributed" "$report_only" "$shipping" >&2
else
	echo "== daemon-commit attribution audit ($RANGE_LABEL)"
	printf 'summary: scanned=%d attributed=%d report-only=%d unattributed-shipping=%d\n' \
		"$scanned" "$attributed" "$report_only" "$shipping"
	if [ "$VERBOSE" -eq 1 ]; then
		for row in "${attributed_rows[@]}"; do
			printf 'ok   %s\n' "$row"
		done
		for row in "${report_rows[@]}"; do
			printf 'warn %s (docs/status only)\n' "$row"
		done
	fi
	for row in "${shipping_rows[@]}"; do
		printf 'FAIL %s\n' "$row"
	done
	if [ "$shipping" -gt 0 ]; then
		echo "heal: unpushed -> scripts/heal-daemon-sweep.sh [--from <ref>] <Task-Queue-ID>; pushed -> empty footered marker commit citing the sha(s)"
	fi
fi

[ "$shipping" -eq 0 ]
