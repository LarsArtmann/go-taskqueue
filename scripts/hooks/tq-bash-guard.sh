#!/usr/bin/env bash
# tq-bash-guard.sh — PreToolUse hook for the bash tool (go-taskqueue).
#
# Converts this repo's recurring session-shell and gate-death classes from
# paid round-trips (DLQ autopsies, silently-wrong reasoning) into instant
# denies carrying the fix, or one-line advisories. Every rule cites a real
# incident class documented in AGENTS.md §Known Issues.
#
# DENIES (exit 2):
#   - ${PIPESTATUS[...]} usage — no usable PIPESTATUS in session shells
#   - printf %.0s — yields empty here
#   - POSIX ERE bracket classes written with \t/\d/\s/\w (2026-10-27: a
#     `[^ \t]` class silently skipped the cqrs event module; fixture
#     "passed" wrong)
#   - unscoped gofmt over directories — re-plants the vendor-gofmt death
#     class (94% of this repo's DLQ)
#   - scripts/smoke/* without an explicit TQ_DB= — agent shells inherit
#     the PRODUCTION journal
#   - git push -f/--force/--force-with-lease/+refspec — history policy:
#     an agent can never supply the required human approval (NO hatch)
#   - git filter-branch — dead-cite explosion; scripted history edits go
#     through script files (scripts/heal-daemon-sweep.sh pattern)
#   - shell redirects writing *.go — heredoc/redirect tab-glue + daemon
#     sweeps; Go source goes through the write tool
#
# ADVISORIES (context only, never blocking):
#   - inherited TQ_DB on tq invocations (prod-journal reminder)
#   - bare `unset` (leaks to children; use env -u VAR cmd)
#   - multi-file tail (fails in session shells)
#   - chained single `&` (backgrounds the wrong span)
#   - bare \d/\t in grep patterns without -P/-F (literal d/t)
#   - root go build/test/vet while internal-module dirs are dirty and
#     vendor/ is stale (root builds read vendor/)
#   - GOCACHE resolving into /nix/store (ENOSPC death)
#   - nested-module go commands missing GOEXPERIMENT=jsonv2 GOWORK=off
#   - nix build/run/flake with untracked files (flakes see tracked only)
#
# Noise policy: silent (exit 0, no stdout) unless a rule fires; git is
# consulted only when a go/nix command makes a rule relevant.
#
# Escape hatches (real env var or command prefix): ALLOW_UNSCOPED_GOFMT=1,
# ALLOW_PROD_TQ=1. Force-push deliberately has none.
#
# Hook contract: CRUSH_TOOL_INPUT_COMMAND carries the command; exit 2 =
# deny (stderr is the reason); exit 0 + JSON stdout = advisory context.
# --self-test runs the pinned fixture battery (fleet convention).
set -euo pipefail

cmd="${CRUSH_TOOL_INPUT_COMMAND:-}"
[ -n "$cmd" ] || exit 0

# Doubled backslashes are legitimate literal-backslash searches; blank
# them before any single-backslash \t/\d detection so they cannot match.
sanitized="${cmd//\\\\/x}"

has_word() {
	# $1 = word that must appear as a shell token (boundary-delimited)
	printf '%s' "$cmd" | grep -Eq "(^|[[:space:];|&])$1([[:space:]]|$)"
}

has_go_tool_cmd() {
	printf '%s' "$cmd" | grep -Eq '(^|[[:space:];&])go[[:space:]]+(build|test|vet)([[:space:]]|$)'
}

uses_posix_regex() {
	printf '%s' "$cmd" | grep -Eq '(^|[[:space:];|&])(e?grep|egrep|sed)([[:space:]]|$)'
}

# Any grep carrying -P or -F disables the ERE hazard rules for the whole
# command (conservative: a compound with one PCRE grep skips the denies).
has_pcre_or_fixed_grep() {
	printf '%s' "$cmd" | grep -Eq '[[:space:]]-[A-Za-z]*[PF]([[:space:]]|$)'
}

hatched() {
	# $1 = hatch name; real env var or command prefix both work
	[ "${!1:-0}" = "1" ] || [ "${cmd#"$1=1"}" != "$cmd" ]
}

# ---------------------------------------------------------------- denies

if printf '%s' "$cmd" | grep -Eq '[$][{]?PIPESTATUS'; then
	cat >&2 <<'EOF'
BLOCKED: session shells here have no usable PIPESTATUS (AGENTS.md §Known
Issues) — reads return garbage and gates get verified on fiction.

- battery convention: write each gate's rc to a FILE, then grep the file
  ("battery rc TO A FILE (no PIPESTATUS)")
EOF
	exit 2
fi

if printf '%s' "$cmd" | grep -Eq 'printf[[:space:]][^|;&]*%[.]0s'; then
	cat >&2 <<'EOF'
BLOCKED: printf %.0s yields an EMPTY string in these session shells
(AGENTS.md §Session shell hazards). Use `printf -v` repeated char via a
loop, head -c, or the bash tool's run_in_background instead.
EOF
	exit 2
fi

if uses_posix_regex && ! has_pcre_or_fixed_grep \
	&& printf '%s' "$sanitized" | grep -Eq '\[[^]]*\\[bdstw]'; then
	cat >&2 <<'EOF'
BLOCKED: POSIX ERE bracket class written with \t/\d/\s/\w — inside [...]
those are the LITERAL letters t/d/s/w. 2026-10-27: a `[^ \t]` class
silently skipped the cqrs event module and the fixture "passed" wrong.

- whitespace/digit classes: [[:space:]] [[:alnum:]] [[:digit:]] etc.
- real PCRE escapes: grep -P '\t' (but never inside a bracket class)
EOF
	exit 2
fi

if has_word gofmt && ! printf '%s' "$cmd" | grep -q 'check-ignore' && ! hatched ALLOW_UNSCOPED_GOFMT; then
	gofmt_unscoped=0 skipval=0 seen=0
	for tok in $cmd; do
		if [ "$seen" = 0 ]; then
			[ "$tok" = "gofmt" ] && seen=1
			continue
		fi
		case "$tok" in
			*'&'* | *';'* | *'|'*) break ;;
			-*) skipval=1 ;;
			*) ;;
		esac
		if [ "$skipval" = 1 ]; then
			skipval=0
			continue
		fi
		case "$tok" in
			. | ./ | ./... | ../) gofmt_unscoped=1 break ;;
			*.go) ;;
			*)
				if [ -n "${CRUSH_CWD:-}" ] && [ -d "$CRUSH_CWD/$tok/vendor" ]; then
					gofmt_unscoped=1
					break
				fi
				;;
		esac
	done
	if [ "$gofmt_unscoped" = 1 ]; then
		cat >&2 <<'EOF'
BLOCKED: unscoped gofmt over a directory walks vendor/ and re-plants the
vendor-gofmt death class (94% of this repo's DLQ).

- scoped form: gofmt -l . | git check-ignore --stdin -v --non-matching | grep '^::'
- specific files: gofmt -l file1.go file2.go
- repo doctor (already scoped): tq doctor gofmt:<repo>
- deliberate unscoped run: prefix ALLOW_UNSCOPED_GOFMT=1
EOF
		exit 2
	fi
fi

if printf '%s' "$cmd" | grep -Eq '(^|[[:space:];&])(\./)?(scripts/)?smoke/[A-Za-z0-9_.-]+' \
	&& ! printf '%s' "$cmd" | grep -q 'TQ_DB=' && ! hatched ALLOW_PROD_TQ; then
	cat >&2 <<'EOF'
BLOCKED: scripts/smoke/* without an explicit TQ_DB — agent shells inherit
the PRODUCTION journal (AGENTS.md §Known Issues).

- scratch smokes MUST export their own DB:
    TQ_DB=/tmp/tq-scratch-$(date +%s).db scripts/smoke/<suite>.sh
- genuinely operating on production: prefix ALLOW_PROD_TQ=1
EOF
	exit 2
fi

git_hit() {
	printf '%s' "$cmd" | awk '
		function reset() { seen_git = 0; subcmd = ""; skipval = 0; in_push = 0 }
		{
			n = split($0, segs, /[;|&]+/)
			for (s = 1; s <= n; s++) {
				m = split(segs[s], w, /[[:space:]]+/)
				reset()
				for (i = 1; i <= m; i++) {
					t = w[i]
					if (t == "") continue
					if (!seen_git) { if (t == "git") seen_git = 1; continue }
					if (skipval) { skipval = 0; continue }
					if (subcmd == "") {
						if (t == "-C" || t == "--git-dir" || t == "--work-tree" || t == "--namespace") { skipval = 1; continue }
						if (substr(t, 1, 1) == "-") continue
						subcmd = t
						if (subcmd == "filter-branch") { print "filter-branch"; exit }
						if (subcmd == "push") { in_push = 1; continue }
						continue
					}
					if (in_push && (t ~ /^(-f|--force)/ || t ~ /^\+/)) { print "force-push"; exit }
				}
			}
		}'
}

git_bad="$(git_hit)"
if [ "$git_bad" = "filter-branch" ]; then
	cat >&2 <<'EOF'
BLOCKED: git filter-branch rewrites history wholesale — the 2026-10
dead-cite explosion came from exactly this. History policy: scripted
edits go through script files (see scripts/heal-daemon-sweep.sh and its
--from/--self-test rails), never an inline filter-branch.
EOF
	exit 2
fi
if [ "$git_bad" = "force-push" ]; then
	cat >&2 <<'EOF'
BLOCKED: force push is NEVER allowed from an agent (history policy: no
force push, no rewriting pushed commits — an agent cannot supply the
required human approval). If it is genuinely necessary, the HUMAN runs
it outside Crush. Unpushed-span heals: scripts/heal-daemon-sweep.sh.
EOF
	exit 2
fi

if printf '%s' "$cmd" | grep -Eq '(>>?|[[:space:]]tee[[:space:]]+)[^|;&[:space:]]*\.go([[:space:];|&]|$)'; then
	cat >&2 <<'EOF'
BLOCKED: shell redirect/tee writing a *.go file — free-text heredocs glue
tabs in as literal \t (2026-10-13) and daemon sweeps eat unstaged
fixtures. Generate Go source through the write tool (or a committed
generator), never echo/cat/tee redirects.
EOF
	exit 2
fi

# ------------------------------------------------------------- advisories

notes=()

if [ -n "${TQ_DB:-}" ] \
	&& printf '%s' "$cmd" | grep -Eq '(^|[[:space:];&])(tq|build-tq\.sh)([[:space:]]|$)|cmd/tq' \
	&& ! printf '%s' "$cmd" | grep -q 'TQ_DB='; then
	notes+=("inherited TQ_DB=$TQ_DB: tq in this call hits THAT journal (production?). Scratch/verify work needs an explicit TQ_DB=<scratch>; smokes without one are denied outright.")
fi

if has_word unset; then
	notes+=("bare 'unset VAR' leaks to children in session shells — prefer 'env -u VAR cmd' (AGENTS.md §Session shell hazards).")
fi

if has_word tail; then
	tail_files=0 tail_seen=0 tail_skip=0
	for tok in $cmd; do
		if [ "$tail_seen" = 0 ]; then
			[ "$tok" = "tail" ] && tail_seen=1
			continue
		fi
		case "$tok" in
			*'&'* | *';'* | *'|'*) break ;;
			-n | -c | --lines | --bytes) tail_skip=1 ;;
			-*) ;;
			*) tail_files=$((tail_files + 1)) ;;
		esac
		if [ "$tail_skip" = 1 ]; then
			tail_skip=0
			continue
		fi
		[ "$tail_files" -ge 2 ] && break
	done
	[ "$tail_files" -ge 2 ] && notes+=("multi-file tail fails in session shells — one tail per file (or awk).")
fi

chain_count="$(printf '%s' "$cmd" | grep -o '[^&]&[^&]' | wc -l)"
if [ "$chain_count" -ge 2 ]; then
	notes+=("chained single & backgrounds the wrong span in session shells — use the bash tool's run_in_background instead.")
fi

if uses_posix_regex && ! has_pcre_or_fixed_grep && printf '%s' "$sanitized" | grep -Eq '\\[dt]'; then
	notes+=("POSIX ERE: \\d and \\t match literal 'd'/'t' — use [[:digit:]]/[[:space:]] classes or grep -P (2026-10-27 silent-skip incident).")
fi

if has_go_tool_cmd && [ -n "${CRUSH_PROJECT_DIR:-}" ] && [ "${CRUSH_CWD:-}" = "$CRUSH_PROJECT_DIR" ] \
	&& ! printf '%s' "$cmd" | grep -q 'go mod vendor' \
	&& [ -n "$(git -C "$CRUSH_PROJECT_DIR" status --porcelain -- internal task journal queue executor worker cmd 2>/dev/null | head -1)" ]; then
	notes+=("root go build/test reads vendor/ and internal-module dirs have uncommitted changes — run 'go mod vendor' first (check-gomod-vendor-sync.sh gates this).")
fi

go_cache_hazard() {
	local eff real
	eff="${GOCACHE:-$HOME/.cache/go-build}"
	real="$(readlink -f "$eff" 2>/dev/null || printf '%s' "$eff")"
	case "$real" in /nix/store/*) return 0 ;; esac
	return 1
}
if has_go_tool_cmd && go_cache_hazard; then
	notes+=("host GOCACHE resolves into /nix/store (ENOSPC death class): prefix GOCACHE=/tmp/go-build-cache (AGENTS.md §Host GOCACHE hazards).")
fi

if [ -n "${CRUSH_CWD:-}" ] && [ -f "$CRUSH_CWD/go.mod" ] && [ -f "$CRUSH_CWD/../go.mod" ] \
	&& has_go_tool_cmd && ! printf '%s' "$cmd" | grep -q 'GOWORK=off'; then
	notes+=("nested module: run gates as GOEXPERIMENT=jsonv2 GOWORK=off go ... (AGENTS.md §Commands, per-module gate).")
fi

if printf '%s' "$cmd" | grep -Eq '(^|[[:space:];&])nix[[:space:]]+(build|run|flake)([[:space:]]|$)' \
	&& [ -n "${CRUSH_PROJECT_DIR:-}" ] \
	&& git -C "$CRUSH_PROJECT_DIR" status --porcelain 2>/dev/null | grep -q '^??'; then
	notes+=("flakes see only git-tracked files and untracked files exist — 'git add' them before nix build (AGENTS.md §Known Issues).")
fi

if [ "${#notes[@]}" -gt 0 ]; then
	printf '%s\n' "${notes[@]}" | jq -R -s '{context: (split("\n") | map(select(length > 0)))}'
fi
exit 0
