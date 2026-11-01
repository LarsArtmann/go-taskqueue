#!/usr/bin/env bash
# Dead-export audit: exported package-level identifiers with zero
# references outside their declaring package. Advisory report — exit 0
# unless --strict, because "kept on purpose" exports (documented domain
# API like task.CanTransitionTo) legitimately have zero in-repo importers.
#
# Methodology note (2026-09-10 re-derivation): matching is SUBSTRING, not
# word-boundary. `rg -w Sink` misses suffixed references like NewSink and
# NewCommandExecutor and undercounts — most of the arch-review's "~15 dead
# exports" were alive once matching was corrected. Substring matching can
# only overcount liveness (false survivors), never flag live symbols as
# dead, so the report is a conservative floor for the true dead set.
# shellcheck disable=SC2016
set -euo pipefail
cd "$(dirname "$0")/.."
# Recursive --self-test re-invokes THIS script; capture the absolute self
# path BEFORE any cd (the gosec self_abs pattern; 02-38 §b).
self_abs="$(cd "$(dirname "$0")" && pwd)/$(basename "$0")"

must_mention() {
	if grep -qF -- "$1" <<<"$2"; then
		echo "ok: $3"
	else
		echo "FAIL: $3 (output is missing '$1')"
		printf '%s\n' "$2"
		exit 1
	fi
}

# --self-test pins the end-of-run summary line (fleet convention: countable
# gate summaries are PINNED, not advisory — 01-12 §e7/§g2) and runs a
# negative-path counter battery over the dead/strict branches via the
# TQ_DEAD_FILES_CMD hook (declaration-source override): a planted fixture
# symbol with zero importers must (a) count dead=1, (b) exit 0 non-strict
# WITH the advisory line printed AFTER the summary, and (c) exit 1 under
# --strict with the summary still printed BEFORE the exit (01-12 §f13
# ordering pin).
self_test() {
	# tmp is deliberately global: the EXIT trap reads it after self_test
	# returned, and set -u would kill the trap on a function-local name.
	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' EXIT
	printf 'package fix\n\n// DeadVar is exported with zero importers.\nvar DeadVar = 1\n' >"$tmp/fix.go"
	local hook="printf '%s\\n' '$tmp/fix.go'"
	local out rc=0
	out="$(TQ_DEAD_FILES_CMD="$hook" "$self_abs" 2>&1)" || rc=$?
	if [ "$rc" -ne 0 ]; then
		echo "FAIL: non-strict dead>0 must stay advisory rc=0 (got rc=$rc) — the strict-exit if/else leaks its status under set -e"
		printf '%s\n' "$out"
		exit 1
	fi
	must_mention "dead-exports summary: 0 symbols ok, 1 dead, 0 structurally alive, 0 annotated keep (advisory)" "$out" "summary pins exact counters on the planted dead symbol"
	must_mention "advisory — audit before pruning" "$out" "non-strict dead>0 branch prints the advisory"
	if [ "$(grep -n 'dead-exports summary:' <<<"$out" | head -1 | cut -d: -f1)" -lt "$(grep -n 'advisory — audit' <<<"$out" | head -1 | cut -d: -f1)" ]; then
		echo "ok: summary precedes the advisory tail (ordering pin)"
	else
		echo "FAIL: advisory tail printed before the summary"
		printf '%s\n' "$out"
		exit 1
	fi
	local s_out s_rc=0
	s_out="$(TQ_DEAD_FILES_CMD="$hook" "$self_abs" --strict 2>&1)" || s_rc=$?
	if [ "$s_rc" -ne 1 ]; then
		echo "FAIL: strict dead>0 must exit 1 (got rc=$s_rc)"
		printf '%s\n' "$s_out"
		exit 1
	fi
	must_mention "dead-exports summary: 0 symbols ok, 1 dead, 0 structurally alive, 0 annotated keep (advisory)" "$s_out" "strict path still prints the summary before exiting"
	local pos_out pos_rc=0
	pos_out="$("$self_abs" 2>&1)" || pos_rc=$?
	if grep -Eq '^dead-exports summary: [0-9]+ symbols ok, [0-9]+ dead, [0-9]+ structurally alive, [0-9]+ annotated keep \(advisory\)$' <<<"$pos_out"; then
		echo "ok: real-run summary matches the pinned format (rc=$pos_rc)"
	else
		echo "FAIL: real-run summary missing or malformed"
		printf '%s\n' "$pos_out"
		exit 1
	fi
	echo "dead-exports self-test ok (summary format pin + negative counter battery)"
}

if [ "${1:-}" = "--self-test" ]; then
	self_test
	exit 0
fi

strict=0
[ "${1:-}" = "--strict" ] && strict=1

# Declare files live in every internal package; usage search covers the
# whole repo (all modules import from each other + root app layer).

dead=0
alive=0
structalive=0
kept=0

# Signature table for the structural-liveness annotation: every exported
# func's full declaration text (accumulated across continuation lines up to
# the body's opening brace). A dead-flagged type that appears only as a
# return/parameter type of a LIVE exported func is annotated instead of
# costing manual triage (the 2026-09-21 false-positive class: RegistryEntry,
# SweepOutcome, CloseResult, ... all structurally alive behind Sweep/Close).
sigfile="$(mktemp)"
declfile="$(mktemp)"
deadfile="$(mktemp)"
namesfile="$(mktemp)"
hitsfile="$(mktemp)"
trap 'rm -f "$sigfile" "$declfile" "$deadfile" "$namesfile" "$hitsfile"' EXIT

funcsig_rx='^(func|type|var|const) ([A-Z][A-Za-z0-9_]*)[( ]'
blocksig_rx='^\t([A-Z][A-Za-z0-9_]*)[ =]'

# Annotated-keep list: verified-live symbols whose only call sites sit in
# their DECLARING package, so the outside-package search flags them every
# run. Not dead, not structurally alive — deliberately exported (sentinel
# error contract / route-path constants shared across the package's files).
# Each entry: <file><TAB><name>; substring-verified 2026-09-22:
#   ErrTokenRequiredOnLAN  internal/webui/auth.go:39 (Validate), auth_test.go
#   HealthDashboardPath    internal/webui/webui.go:169, health.go:298
#   HealthSSEPath          internal/webui/webui.go:170, health.go:299
annotated=(
	"internal/webui/auth.go	ErrTokenRequiredOnLAN"
	"internal/webui/health.go	HealthDashboardPath"
	"internal/webui/health.go	HealthSSEPath"
)
# TQ_DEAD_FILES_CMD: self-test hook replacing the declaration-source
# pipeline (must be a shell PIPELINE string, eval'd; default lists the
# repo's internal Go sources minus tests and generated templ output).
files_cmd="${TQ_DEAD_FILES_CMD:-git ls-files 'internal/*.go' | grep -v '_test.go' | grep -v '_templ.go'}"
eval "$files_cmd" |
	xargs awk -v d="$funcsig_rx" -v b="$blocksig_rx" '
		/^(var|const) \(/ { inblock = 1; next }
		inblock && /^\)/ { inblock = 0; next }
		inblock && match($0, b) { print FILENAME "\t" substr($0, RSTART + 1, RLENGTH - 2); next }
		match($0, d) {
			s = substr($0, RSTART, RLENGTH)
			sub(/^(func|type|var|const) /, "", s)
			sub(/[( ]$/, "", s)
			print FILENAME "\t" s
		}
	' >"$declfile"

eval "$files_cmd" |
	xargs awk '
		infunc {
			sig = sig $0 " "
			if (index($0, "{") > 0) { print FILENAME "\t" fname "\t" sig; infunc = 0 }
			next
		}
		/^func / {
			infunc = 1
			sig = $0 " "
			s = $0
			sub(/^func /, "", s)
			if (s ~ /^\(/) { sub(/^[^)]*\)[[:space:]]*/, "", s) }
			sub(/\(.*/, "", s)
			fname = s
			if (index($0, "{") > 0) { print FILENAME "\t" fname "\t" sig; infunc = 0 }
			next
		}
	' >"$sigfile"

# One-sweep importer map (2026-11-01 perf fix): the old per-symbol
# whole-tree `rg` rescan was O(symbols × tree) and, once vendor/ grew
# past a hundred MB of vendored Go (--no-ignore pulled it in), turned
# this advisory gate from seconds into hours (battery-15 stall). ONE rg
# sweep now emits every fixed-string occurrence as "path:match"; awk
# folds them into unique name<TAB>path pairs and ONE join pass decides
# alive/dead per declaration. vendor/, .git/, .crush/ and .tq-verify/
# are excluded — vendored copies duplicate in-repo sources and scratch
# trees are not importers of internal symbols — so the exclusion can
# only surface genuinely dead symbols, never flag live ones. Substring
# semantics unchanged (methodology note above).
cut -f2 "$declfile" | sort -u >"$namesfile"
: >"$hitsfile"
if [ -s "$namesfile" ]; then
	# Identifier tokens, not a -F pattern set: rg -o with hundreds of -f
	# patterns shadows overlapping names (leftmost match wins, so e.g.
	# NewAnswerPoller vanished behind shorter set members — battery-15
	# debug). One token sweep + substring containment in awk reproduces
	# the ORIGINAL per-symbol substring semantics (a name counts when ANY
	# identifier token in a file contains it, suffixed refs included).
	rg -o --no-ignore -g '*.go' -g '!vendor/**' -g '!.git/**' -g '!.crush/**' -g '!.tq-verify/**' \
		'[A-Za-z_][A-Za-z0-9_]*' . 2>/dev/null |
		awk -F: -v namesfile="$namesfile" '
			FILENAME == namesfile { names[$0]; next }
			/^[^:]+\.go:[^:]+$/ {
				path = $1
				sub(/^\.\//, "", path)
				tok = $2
				if ((tok SUBSEP path) in tokseen) next
				tokseen[tok SUBSEP path]
				for (n in names)
					if (index(tok, n) > 0 && !((n SUBSEP path) in seen)) {
						seen[n SUBSEP path]
						print n "\t" path
					}
			}
		' "$namesfile" - >>"$hitsfile" || true
fi

# Alive = at least one occurrence OUTSIDE the declaring directory (usages
# in tests of other packages count; declarations were test-excluded).
decl_total="$(awk -F'\t' 'NF >= 2 && $2 != "" { n++ } END { print n + 0 }' "$declfile")"
awk -F'\t' -v OFS='\t' -v hitsfile="$hitsfile" '
	FILENAME == hitsfile { hits[$1] = hits[$1] $2 "\n"; next }
	{
		if ($2 == "") next
		dir = $1
		sub(/\/[^\/]*$/, "", dir)
		n = split(hits[$2], occ, "\n")
		for (i = 1; i <= n; i++)
			if (occ[i] != "" && index(occ[i], dir "/") != 1) next
		print $1, $2
	}
' "$hitsfile" "$declfile" >"$deadfile"
alive=$((decl_total - $(grep -c . "$deadfile" || true)))

# Second pass: annotate the structurally-alive class. A flagged name that
# appears (word-delimited) in the signature of a DIFFERENT EXPORTED func of
# the same package is alive behind that func's exported contract — even when
# the func itself is flagged (LoadRegistry/RewriteRegistry are zero-importer
# too, yet RegistryEntry is only reachable through them; Sweep/Close carry
# SweepOutcome/CloseResult directly). Unexported helpers do NOT annotate.
while IFS=$'\t' read -r file name; do
	note=""
	for entry in "${annotated[@]}"; do
		[ "$entry" = "$file	$name" ] || continue
		note="annotated keep: verified live via in-package call sites (see list in this script)"
		break
	done
	if [ -z "$note" ]; then
		while IFS=$'\t' read -r sfile sname ssig; do
			[ "${sfile%/*}" = "${file%/*}" ] || continue
			[ "$sname" != "$name" ] || continue
			case "$sname" in [A-Z]*) ;; *) continue ;; esac
			grep -Eq "(^|[^A-Za-z0-9_])${name}([^A-Za-z0-9_]|$)" <<<"$ssig" || continue
			note="structurally alive: return/parameter type of ${sname}()"
			break
		done <"$sigfile"
	fi
	if [ -n "$note" ]; then
		echo "$file: exported symbol with zero direct importers, $note: $name"
		case "$note" in annotated*) kept=$((kept + 1)) ;; *) structalive=$((structalive + 1)) ;; esac
	else
		echo "$file: exported symbol with zero importers: $name"
		dead=$((dead + 1))
	fi
done <"$deadfile"

echo
echo "dead-exports summary: $alive symbols ok, $dead dead, $structalive structurally alive, $kept annotated keep (advisory)"
if [ "$dead" -eq 0 ]; then
	echo "dead-exports: clean — every exported internal symbol has at least one importer"
else
	echo "dead-exports: $dead symbol(s) with zero importers"
	if [ "$strict" -eq 1 ]; then
		exit 1
	fi
	echo "advisory — audit before pruning: internal/ layout is deliberate until the API stabilizes"
fi
