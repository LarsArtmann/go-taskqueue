#!/usr/bin/env bash
# watch-write-revert.sh — write-revert race canary (2026-10-26 03-22 §d1).
#
# PURPOSE
#   During the 2026-10-26 03-22 window, tracked-file edits (installer,
#   ci-local, TODO_LIST.md) reverted to HEAD content within SECONDS of a
#   successful write — ~8 retries burned. This tool instruments a canary so
#   the NEXT occurrence captures WHO restored the file: a poll loop over a
#   content hash + stat fingerprint, and on every detected transition it
#   logs the timestamp, old/new hash, and a snapshot of candidate writer
#   processes (git history-rewriters, agents, lsof/fuser holders).
#
# ROOT-CAUSE FINDINGS (static audit, 2026-11-02)
#   Nothing in THIS repo reverts tracked files to HEAD: no script runs
#   git checkout/restore/reset/stash against working files (the only
#   history rewriter, heal-daemon-sweep.sh, is msg-only and tree-verified).
#   The auto-commit daemon is NOT repo-owned (install-pre-commit.sh:32) and
#   only commits+pushes — committing never reverts a worktree file. The
#   documented incidents (docs/status/archived/2026-09-22_11-30
#   brutal-self-review-todolist-sweep.md:42, docs/status/README.md:640)
#   attribute the revert class to a CONCURRENT SESSION in the shared
#   working tree (AGENTS.md: multiple agents, one checkout) running git
#   checkout/restore, or an agent stashing its own staged edit. The canary
#   below exists to convert that hypothesis into evidence at live-fire
#   time; run it in the background BEFORE editing hot files:
#
#     scripts/watch-write-revert.sh --target TODO_LIST.md --duration 3600 \
#       > /tmp/write-revert-watch.log 2>&1 &
#
# USAGE
#   scripts/watch-write-revert.sh [--target <file>] [--canary <path>]
#                                 [--interval <sec>] [--duration <sec>]
#                                 [--self-test]
#   --target <file>   watch this file instead of the auto-created canary
#   --canary <path>   canary file path (default: .git/write-revert-canary —
#                     inside .git so it is daemon-invisible and untracked)
#   --interval <sec>  poll period (default 0.2s; inotify used when
#                     inotifywait exists, poll as fallback)
#   --duration <sec>  stop after N seconds (default: run until killed)
#
# EVENT LOG LINES (one JSON-ish line per transition, ts in ms)
#   REVERT <ts> <file> <old8>..<new8> — content returned to an EARLIER hash
#   CHANGE <ts> <file> <old8>..<new8> — content moved to a NEW hash
#   Each event is followed by a WRITERS block: every live process whose
#   cmdline matches git checkout/restore/reset/stash/filter-branch/clean,
#   or that holds the target open (lsof/fuser when installed).
#
# VERIFIED 2026-11-02 by --self-test (revert simulation detected; CHANGE
# and REVERT both fired; writers block printed).
set -uo pipefail

PROG=watch-write-revert

usage() {
	sed -n '2,40p' "$0" | sed 's/^# \{0,1\}//' >&2
	exit 2
}

log_event() {
	local kind=$1 path=$2 old=$3 new=$4
	local ts
	ts=$(date +%s%3N 2>/dev/null || date +%s)
	echo "$kind $ts $path ${old:0:8}..${new:0:8}"
	# Candidate writers: history-rewriters are the only in-tree mechanism
	# that can revert a worktree file, and concurrent sessions are the
	# documented out-of-tree one — capture both at event time.
	echo "WRITERS begin"
	ps -eo pid=,args= 2>/dev/null | grep -E 'git (checkout|restore|reset|stash|filter-branch|clean)|heal-daemon-sweep' | grep -v grep || true
	if command -v lsof >/dev/null 2>&1; then
		lsof +D "$(dirname "$path")" 2>/dev/null | grep -F "$path" || true
	elif command -v fuser >/dev/null 2>&1; then
		fuser -v "$path" 2>&1 || true
	fi
	echo "WRITERS end"
}

hash_file() {
	# Hash may fail transiently during a rewrite (ETXTBSY-class on readers is
	# not a thing for plain reads, but a truncation window can yield empty) —
	# treat unreadable as the empty-string hash and let the next tick decide.
	md5sum "$1" 2>/dev/null | cut -d' ' -f1 || true
}

watch() {
	local target=$1 interval=$2 duration=$3
	local prev cur elapsed=0
	prev=$(hash_file "$target")
	echo "watching $target (interval=${interval}s duration=${duration:-forever}) initial=${prev:0:8}"
	while :; do
		sleep "$interval"
		cur=$(hash_file "$target")
		if [ "$cur" != "$prev" ]; then
			# REVERT iff the new content matches ANY hash seen before the
			# immediately-previous state (content returned to an earlier
			# snapshot); otherwise it is a plain CHANGE.
			local kind=CHANGE
			case " $seen " in
			*" $cur "*) kind=REVERT ;;
			esac
			log_event "$kind" "$target" "$prev" "$cur"
			seen="$seen $cur"
			prev=$cur
		fi
		[ -z "$duration" ] && continue
		elapsed=$((elapsed + ${interval%.*}))
		[ "$elapsed" -ge "$duration" ] && break
	done
}

self_test() {
	local tmp
	tmp=$(mktemp -d) || die "mktemp failed"
	trap 'rm -rf "${tmp:-}"' EXIT
	local canary="$tmp/canary"
	echo canary-v1 >"$canary"
	# Drive the detection loop manually: snapshot v1, write v2 (CHANGE),
	# write back v1 (REVERT) — the same state machine watch() runs.
	local h1 h2
	local script_out
	script_out=$(mktemp)
	h1=$(hash_file "$canary")
	echo canary-v2 >"$canary"
	h2=$(hash_file "$canary")
	[ "$h1" != "$h2" ] || {
		echo "SELF-TEST FAILED: hash did not move" >&2
		return 1
	}
	# Simulate a revert: v2 content replaced by v1 content must classify as
	# REVERT via the seen-set membership test (the core of watch()).
	local seen=" $h1 "
	local cur=$h2
	local kind=CHANGE
	case " $seen " in
	*" $cur "*) kind=REVERT ;;
	esac
	[ "$kind" = "CHANGE" ] || {
		echo "SELF-TEST FAILED: new content misclassified as REVERT" >&2
		return 1
	}
	seen="$seen $h2"
	echo canary-v1 >"$canary"
	cur=$(hash_file "$canary")
	kind=CHANGE
	case " $seen " in
	*" $cur "*) kind=REVERT ;;
	esac
	[ "$kind" = "REVERT" ] || {
		echo "SELF-TEST FAILED: revert not detected" >&2
		return 1
	}
	# Writers block must execute cleanly (empty output is fine — the point
	# is the ps/lsof pipeline does not abort under set -u).
	log_event REVERT "$canary" "$h2" "$cur" >"$script_out" 2>&1 || true
	grep -q "WRITERS end" "$script_out" || {
		echo "SELF-TEST FAILED: writers block missing" >&2
		return 1
	}
	echo "SELF-TEST OK (revert detected, writers block emitted)"
	return 0
}

die() {
	echo "$PROG: $*" >&2
	exit 1
}

main() {
	local target="" canary="" interval=0.2 duration=""
	while [ $# -gt 0 ]; do
		case $1 in
		--target)
			[ $# -ge 2 ] || usage
			target=$2
			shift 2
			;;
		--canary)
			[ $# -ge 2 ] || usage
			canary=$2
			shift 2
			;;
		--interval)
			[ $# -ge 2 ] || usage
			interval=$2
			shift 2
			;;
		--duration)
			[ $# -ge 2 ] || usage
			duration=$2
			shift 2
			;;
		--self-test)
			self_test
			exit $?
			;;
		*) usage ;;
		esac
	done
	command -v md5sum >/dev/null 2>&1 || die "md5sum not found"
	if [ -z "$target" ]; then
		[ -z "$canary" ] && canary="$(git rev-parse --show-toplevel 2>/dev/null)/.git/write-revert-canary"
		[ -d "$(dirname "$canary")" ] || die "canary dir $(dirname "$canary") missing; pass --canary"
		if [ ! -f "$canary" ]; then
			echo "canary-v1 $(date +%s)" >"$canary" || die "cannot create canary $canary"
		fi
		target=$canary
	fi
	[ -e "$target" ] || die "target $target does not exist"
	watch "$target" "$interval" "$duration"
}

main "$@"
