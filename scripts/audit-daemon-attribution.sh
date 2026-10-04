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

FROM=""
ALL_CHORE=0
JSON=0
VERBOSE=0
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
		exec bash "$SELF" --run-self-test
		;;
	--run-self-test)
		# Internal entry reached via the re-exec above so the fixture
		# always runs the on-disk script, not a stale copy.
		shift
		;;
	*)
		usage
		;;
	esac
done

command -v git >/dev/null 2>&1 || die "git not found in PATH"
git rev-parse --verify HEAD >/dev/null 2>&1 || die "not a git repository (or no commits)"

if [ -n "$FROM" ]; then
	git rev-parse --verify --quiet "$FROM" >/dev/null || die "base ref '$FROM' not found"
	RANGE="$FROM..HEAD"
	RANGE_LABEL="$FROM..HEAD"
else
	RANGE="HEAD"
	RANGE_LABEL="full history"
fi

# ---- pass 1: sha | subject | body --------------------------------------
declare -a order=()
declare -A subj_of=()
declare -A footer_of=()
declare -A is_daemon=()

mapfile -d '' -t meta_records < <(git log --no-merges --format='%H%x02%s%x02%b%x00' "$RANGE")

for rec in "${meta_records[@]}"; do
	[ -n "$rec" ] || continue
	sha=${rec%%$'\x02'*}
	rest=${rec#*$'\x02'}
	subject=${rest%%$'\x02'*}
	body=${rest#*$'\x02'}
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
	done <<<"$body"
	footer_of[$sha]=$footer_id

	daemon=0
	if [[ $subject =~ ^chore:\ auto-commit\ [0-9]+\ changed\ file ]]; then
		daemon=1
	elif [ "$ALL_CHORE" -eq 1 ] && [[ $subject == 'chore:'* ]]; then
		daemon=1
	fi
	[ -n "$footer_id" ] && daemon=0
	is_daemon[$sha]=$daemon
done

# ---- pass 2: files per commit + report-file task ids -------------------
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
	[ -n "$cur" ] && [ "$seen_blank" -eq 1 ] || continue
	files_of[$cur]=("${files_of[$cur]:-}" "$line")
	# shellcheck disable=SC2181 # the [ ] guard above already paired with &&
	if true; then :; fi
done
