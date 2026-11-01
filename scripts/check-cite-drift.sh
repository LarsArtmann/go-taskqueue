#!/usr/bin/env bash
# Line-cite drift gate: a living doc that writes `path/to/file.go:123` is
# making a CHECKABLE claim about where something lives. This gate re-reads
# that line and fails when the claim no longer holds.
#
# Why it exists: citation rot is this repo's highest-frequency documentation
# defect. `check-dead-sha-refs.sh` covers the commit-SHA half; line numbers are
# the other half and nothing guarded them. The 2026-11-01 OKF research note was
# committed with FOUR wrong line numbers in a single document (queue.go:444 vs
# 443, events.go:271 vs 273 — twice each, plus result.go:64/105 vs 65/109) by
# the very session that wrote the analysis. A mechanical re-read prevents the
# whole class, not the instance.
#
# Three findings, in increasing precision:
#
#   1. MISSING FILE  — the cited path does not exist. A repo-relative path in
#                      a living doc that resolves to nothing is a stale cite.
#   2. LINE PAST EOF — the cited line number is beyond the file's length.
#   3. ANCHOR DRIFT  — the citing line names a backticked identifier that DOES
#                      occur in the cited file, but not within +/-TQ_CITE_WINDOW
#                      lines of the cite. This is the "points at the wrong
#                      function" class: a cite that lands in an unrelated
#                      region is worse than no cite, because readers trust it.
#
# Deliberate non-findings (why this is not a false-positive machine):
#   - a +/-1 line slip passes. It is cosmetic, and a window-based gate that
#     fails on it would be enforced by disabling it. TQ_CITE_WINDOW (default
#     10) is the knob; the value is a judgement, documented here.
#   - an identifier the doc names but the file never mentions is SKIPPED:
#     living docs legitimately name concepts, not just symbols
#     (`executor.AgentResult` vs the file's `type AgentResult`).
#   - cites into paths that are obviously foreign (absolute, `../`, or an
#     `http(s)://` URL fragment) are skipped, not judged.
#
# Escapes:
#   - baseline file scripts/cite-drift-baseline.txt (`path:lineno` per line,
#     `#` comments): grandfathers pre-existing findings. A NEW finding not in
#     the file fails the gate. Baselined hits are silent unless
#     TQ_CITE_VERBOSE=1.
#   - explicit file arguments (repo-root-relative) scope the gate to exactly
#     those files — the verify-battery's own-file leg passes the window's
#     touched docs so a NEW drifted cite outside the default scope still fails.
#     No args keeps the living-docs default scope.
#
# --emit-baseline prints a sorted, deduplicated addendum for the baseline file
# (baseline membership ignored, so regens are idempotent). Curation stays human.
#
# --self-test pins every decision branch against a /tmp fixture tree per the
# fleet countable-gate convention: clean rc=0, missing file rc=1, past-EOF
# rc=1, anchor drift rc=1, in-window anchor rc=0, non-mentioned anchor skipped,
# baseline membership silent unless TQ_CITE_VERBOSE=1, --emit-baseline output
# greens the gate it was generated from.
set -uo pipefail

MODE="gate"
WINDOW="${TQ_CITE_WINDOW:-10}"

# cite_lines_of <file> <path> <lineno>: print "<path>:<from>-<to>" for the
# window, clamped to the file's real length.
in_window() { # in_window <file> <lineno> <token>
	local file=$1 lineno=$2 token=$3
	local from=$((lineno - WINDOW)) to=$((lineno + WINDOW))
	[ "$from" -lt 1 ] && from=1
	sed -n "${from},${to}p" "$file" | grep -qF -- "$token"
}

# normalize_token: strip a leading package qualifier (`executor.VerifyReceipt`
# -> `VerifyReceipt`) and surrounding punctuation; print "" when the token is
# not identifier-shaped or too short to be evidence.
normalize_token() {
	local tok=$1
	tok=${tok%.}
	tok=${tok%,}
	tok=${tok##*/}
	tok=${tok##*.}
	[ "${#tok}" -lt 4 ] && return 0
	[[ "$tok" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]] || return 0
	printf '%s' "$tok"
}

# resolve_cite <path>: print the tracked file the cite refers to, or "" when
# the path is not resolvable. Living docs cite paths in two honest styles —
# repo-relative ("internal/queue/queue.go") and suffix-shorthand
# ("sqlitev4/adapter.go", meaning the one file by that suffix) — so resolution
# is a UNIQUE-SUFFIX match over `git ls-files` rather than a plain stat. An
# ambiguous suffix (many `adapter.go`s) resolves to nothing and the cite is
# skipped: judging it would be guessing which file the author meant.
resolve_cite() {
	local p=$1
	[ -n "${tracked_index:-}" ] && [ -f "$tracked_index" ] || return 1
	local hits
	hits=$(grep -cE "(^|/)${p//./\\.}$" "$tracked_index" || true)
	[ "${hits:-0}" = 1 ] || return 1
	grep -E "(^|/)${p//./\\.}$" "$tracked_index"
}

# foreign_path <path>: rc=0 when the cited path is not ours to judge.
foreign_path() {
	local p=$1
	case "$p" in
	/* | ../* | http://* | https://* | *://*) return 0 ;;
	esac
	return 1
}

in_baseline() { # in_baseline <key> ; $baseline must be set
	local key=$1 bl
	[ -f "$baseline" ] || return 1
	while IFS= read -r bl; do
		[[ "$bl" =~ ^# ]] && continue
		[ -z "$bl" ] && continue
		[ "$bl" = "$key" ] && return 0
	done <"$baseline"
	return 1
}

# run_gate: cwd is the repo root; caller sets `files` and `baseline`.
run_gate() {
	local f line linenum text cite path lineno tok found=0
	local fail=0 new_hits=0 baselined_hits=0
	local -A emit=()
	local -A seen=()

	tracked_index=$(mktemp)
	trap 'rm -f "$tracked_index"' RETURN
	git ls-files >"$tracked_index" 2>/dev/null || : >"$tracked_index"
	if [ ! -s "$tracked_index" ]; then
		# Not a git checkout (the self-test fixture, a tarball export): walk
		# the tree instead so the gate still resolves cites.
		find . -path ./.git -prune -o -type f -print 2>/dev/null | sed 's|^\./||' >"$tracked_index"
	fi

	for f in "${files[@]}"; do
		[ -f "$f" ] || continue
		while IFS= read -r line; do
			linenum=${line%%:*}
			text=${line#*:}
			# one doc line may carry several cites
			cites=$(grep -oE '[A-Za-z0-9_.][A-Za-z0-9_./-]*\.(go|md|sh|nix|templ|json|ya?ml|toml):[0-9]+' <<<"$text" | sort -u)
			# The ANCHOR rule only applies when the line makes ONE claim: with
			# several cites on a line, the identifiers cannot be attributed to a
			# file, and judging them turns the gate into a false-positive
			# machine. Multi-cite lines still get existence + bounds checks.
			single_cite=0
			[ "$(grep -c . <<<"$cites")" = 1 ] && single_cite=1
			for cite in $cites; do
				path=${cite%:*}
				lineno=${cite##*:}
				foreign_path "$path" && continue

				local key="$f:$linenum:$cite"
				[ -n "${seen[$key]:-}" ] && continue
				seen[$key]=1

				local finding="" resolved=""
				resolved=$(resolve_cite "$path" || true)
				if [ -z "$resolved" ]; then
					# Unresolvable. Only a PATH-SHAPED cite (it carries a
					# directory) is judged: a bare basename is shorthand whose
					# home we cannot know, and docs legitimately cite other
					# projects' files by basename.
					case "$path" in
					*/*) finding="MISSING FILE: $f:$linenum cites '$cite' — no tracked file matches that path" ;;
					esac
				else
					local total
					total=$(wc -l <"$resolved")
					if [ "$lineno" -gt "$total" ]; then
						finding="LINE PAST EOF: $f:$linenum cites '$cite' — $resolved has $total line(s)"
					else
						# anchor: the first identifier-shaped backticked token on
						# the citing line that actually occurs in the cited file
						for raw in $( [ "$single_cite" = 1 ] && grep -oE '`[^`]+`' <<<"$text" | tr -d '`' ); do
							tok=$(normalize_token "$raw")
							[ -z "$tok" ] && continue
							grep -qF -- "$tok" "$resolved" 2>/dev/null || continue
							found=1
							in_window "$resolved" "$lineno" "$tok" && { found=2; break; }
						done
						if [ "$found" = 1 ]; then
							finding="ANCHOR DRIFT: $f:$linenum cites '$cite' — the identifier it names is not within +/-$WINDOW line(s) of $lineno"
						fi
						found=0
					fi
				fi

				[ -z "$finding" ] && continue

				if [ "$MODE" = emit ]; then
					emit["${resolved:-$path}:$lineno"]=1
					continue
				fi

				if in_baseline "${resolved:-$path}:$lineno"; then
					baselined_hits=$((baselined_hits + 1))
					[ "${TQ_CITE_VERBOSE:-0}" = 1 ] && echo "BASELINED: $finding"
				else
					echo "$finding"
					fail=1
					new_hits=$((new_hits + 1))
				fi
			done
		done < <(grep -nE '\.[A-Za-z0-9]+:[0-9]+' "$f" || true)
	done

	if [ "$MODE" = emit ]; then
		local k
		for k in "${!emit[@]}"; do echo "$k"; done | sort -u
		return 0
	fi

	echo "cite-drift summary: ${new_hits} new, ${baselined_hits} baselined, ${#files[@]} file(s) scanned"
	[ "$fail" = 0 ] && echo "cite refs ok"

	return "$fail"
}

if [ "${1:-}" = "--self-test" ]; then
	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' EXIT
	mkdir -p "$tmp/pkg"
	# The anchor lives on line 5; cite 5 (in window) and 30 (drift).
	# pins: the in-window cite (line 5) vs the drifted cite (line 30) branches
	cat >"$tmp/pkg/anchor.go" <<'EOF'
package pkg

// Doc comment.

func RealAnchor() {}
EOF
	# pad to 40 lines so a past-EOF and an in-range-but-drifting cite coexist
	for i in $(seq 1 35); do echo "// filler line $i" >>"$tmp/pkg/anchor.go"; done

	mkdir -p "$tmp/docs"
	# pins: clean-branch — cite inside the window, anchor present (rc=0)
	cat >"$tmp/docs/clean.md" <<'EOF'
`RealAnchor` lives at pkg/anchor.go:6.
EOF
	# pins: drift-branch — anchor named but nowhere near the cite (rc=1)
	cat >"$tmp/docs/drift.md" <<'EOF'
`RealAnchor` lives at pkg/anchor.go:30.
EOF
	# pins: past-eof-branch — line number beyond the file length (rc=1)
	cat >"$tmp/docs/pasteof.md" <<'EOF'
see pkg/anchor.go:900
EOF
	# pins: missing-file-branch — repo-relative path that does not exist (rc=1)
	cat >"$tmp/docs/missing.md" <<'EOF'
see pkg/gone.go:12
EOF
	# pins: skip-branch — the identifier is absent from the file, so no anchor
	# claim is made and the cite is not judged on it (rc=0)
	cat >"$tmp/docs/skip.md" <<'EOF'
`SomeConceptFromElsewhere` at pkg/anchor.go:30.
EOF

	cd "$tmp" || exit 1

	baseline="$tmp/base.txt"
	printf '%s\n' "# baseline" >"$baseline"

	files=(docs/clean.md)
	out=$(run_gate) && rc=0 || rc=$?
	[ "$rc" = 0 ] || {
		echo "cite-drift self-test FAIL: clean cite rc=$rc out: $out"
		exit 1
	}
	grep -q 'cite-drift summary: 0 new, 0 baselined, 1 file(s) scanned' <<<"$out" || {
		echo "cite-drift self-test FAIL: summary shape drifted: $out"
		exit 1
	}

	files=(docs/drift.md)
	out=$(run_gate) && rc=0 || rc=$?
	if [ "$rc" != 1 ] || ! grep -q "ANCHOR DRIFT: docs/drift.md:1 cites 'pkg/anchor.go:30'" <<<"$out"; then
		echo "cite-drift self-test FAIL: anchor drift rc=$rc out: $out"
		exit 1
	fi

	files=(docs/pasteof.md)
	out=$(run_gate) && rc=0 || rc=$?
	if [ "$rc" != 1 ] || ! grep -q "LINE PAST EOF: docs/pasteof.md:1 cites 'pkg/anchor.go:900'" <<<"$out"; then
		echo "cite-drift self-test FAIL: past-EOF rc=$rc out: $out"
		exit 1
	fi

	files=(docs/missing.md)
	out=$(run_gate) && rc=0 || rc=$?
	if [ "$rc" != 1 ] || ! grep -q "MISSING FILE: docs/missing.md:1 cites 'pkg/gone.go:12'" <<<"$out"; then
		echo "cite-drift self-test FAIL: missing file rc=$rc out: $out"
		exit 1
	fi

	# skip-branch: an identifier the cited file never mentions is not an
	# anchor claim, so a far cite stays green.
	files=(docs/skip.md)
	out=$(run_gate) && rc=0 || rc=$?
	[ "$rc" = 0 ] || {
		echo "cite-drift self-test FAIL: non-mentioned anchor judged rc=$rc out: $out"
		exit 1
	}

	# baseline: membership silences a finding, and only under the verbose flag
	# does it print.
	printf '%s\n' "pkg/anchor.go:30" >>"$baseline"
	files=(docs/drift.md)
	out=$(run_gate) && rc=0 || rc=$?
	[ "$rc" = 0 ] || {
		echo "cite-drift self-test FAIL: baseline did not green the gate rc=$rc out: $out"
		exit 1
	}
	grep -q 'BASELINED' <<<"$out" && {
		echo "cite-drift self-test FAIL: baselined finding printed without TQ_CITE_VERBOSE"
		exit 1
	}
	out=$(TQ_CITE_VERBOSE=1 run_gate)
	grep -q 'BASELINED: ANCHOR DRIFT' <<<"$out" || {
		echo "cite-drift self-test FAIL: verbose flag did not surface the baselined hit: $out"
		exit 1
	}

	# --emit-baseline: prints the keys an operator would paste in, and
	# feeding them back greens the gate.
	: >"$baseline"
	files=(docs/drift.md docs/pasteof.md docs/missing.md)
	MODE=emit
	out=$(run_gate) && rc=0 || rc=$?
	[ "$rc" = 0 ] || {
		echo "cite-drift self-test FAIL: emit mode rc=$rc"
		exit 1
	}
	[ "$(grep -c . <<<"$out")" = 3 ] || {
		echo "cite-drift self-test FAIL: emit expected 3 keys: $out"
		exit 1
	}
	{ echo "# emitted"; cat <<<"$out"; } >"$baseline"
	MODE=gate
	files=(docs/drift.md docs/pasteof.md docs/missing.md)
	out=$(run_gate) && rc=0 || rc=$?
	[ "$rc" = 0 ] || {
		echo "cite-drift self-test FAIL: emitted baseline did not green the gate: $out"
		exit 1
	}

	echo "cite-drift self-test ok (clean rc=0, missing file, past-EOF, anchor drift, non-mentioned anchor skipped, baseline silent unless TQ_CITE_VERBOSE=1, emit-baseline greens its own gate)"
	exit 0
fi

cd "$(dirname "$0")/.." || exit 1

baseline="${TQ_CITE_BASELINE:-scripts/cite-drift-baseline.txt}"

if [ "${1:-}" = "--emit-baseline" ]; then
	MODE="emit"
	shift
fi

if [ "$#" -gt 0 ]; then
	files=()
	for f in "$@"; do
		[ -f "$f" ] && files+=("$f")
	done
else
	# Living docs only. Historical report trees (docs/status/**, docs/planning/**)
	# are snapshots: their cites were true when written and rewriting them is
	# not ours to do (same posture as check-dead-sha-refs.sh's default scope).
	# docs/adr/** is out too: ADRs describe designs across projects, so their
	# paths point at code that was never in this repo.
	files=(AGENTS.md TODO_LIST.md README.md FEATURES.md CHANGELOG.md)
	for f in docs/research/*.md docs/DOMAIN_LANGUAGE.md; do
		[ -f "$f" ] && files+=("$f")
	done
fi

run_gate
