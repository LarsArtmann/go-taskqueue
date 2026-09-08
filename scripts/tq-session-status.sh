#!/usr/bin/env bash
# tq session status — what is running, on which DB, since when.
# Dogfood ops (round-5 M14/F74): one command answers "is the pool/serve
# still alive, which binary, which DB, how fresh is the journal".
# Read-only: scans /proc cmdlines, never touches the DBs beyond tq facts.
set -euo pipefail

TQ="${TQ_BIN:-tq}"

fmt_duration() {
	# seconds -> "1d2h3m" style
	local s=$1 out=""
	local d=$((s / 86400))
	s=$((s % 86400))
	local h=$((s / 3600))
	s=$((s % 3600))
	local m=$((s / 60))
	[ "$d" -gt 0 ] && out="${d}d"
	[ "$h" -gt 0 ] && out="${out}${h}h"
	out="${out}${m}m"
	echo "$out"
}

found=0

for proc in /proc/[0-9]*; do
	pid="${proc#/proc/}"
	cmdline="$(tr '\0' ' ' 2>/dev/null <"$proc/cmdline" || true)"
	case "$cmdline" in
	*tq\ agent-pool* | *tq\ worker* | *tq\ serve* | *"./tq agent-pool"* | *"./tq worker"* | *"./tq serve"*) ;;
	*) continue ;;
	esac

	found=1

	# uptime (seconds) straight from the scheduler
	etimes="$(ps -o etimes= -p "$pid" 2>/dev/null | tr -d ' ' || echo 0)"
	etimes="${etimes:-0}"

	# db: --db flag wins, then $TQ_DB from the process env, then cwd/tasks.db
	db="$(echo "$cmdline" | tr ' ' '\n' | grep -A1 -- '--db' | grep -v -- '--db' | head -1 || true)"
	if [ -z "$db" ]; then
		db="$(tr '\0' '\n' <"$proc/environ" 2>/dev/null | grep '^TQ_DB=' | head -1 | cut -d= -f2- || true)"
	fi
	cwd="$(readlink "$proc/cwd" 2>/dev/null || echo "?")"
	[ -z "$db" ] && db="$cwd/tasks.db"

	# serve addr if present
	addr="$(echo "$cmdline" | tr ' ' '\n' | grep -A1 -- '--addr' | grep -v -- '--addr' | head -1 || true)"

	role="$(echo "$cmdline" | awk '{print $1" "$2}' | sed 's|.*/||')"

	printf 'PID %s  %s  up %s\n' "$pid" "$role" "$(fmt_duration "${etimes:-0}")"
	printf '  cmd: %s\n' "$cmdline"
	printf '  db:  %s' "$db"
	if [ -n "$addr" ]; then printf '   addr: %s' "$addr"; fi
	echo

	# journal freshness: newest fact line (tq facts ends with a count summary)
	if command -v "$TQ" >/dev/null 2>&1 && [ -f "$db" ]; then
		last="$(TQ_DB="$db" "$TQ" facts 2>/dev/null | tail -2 | head -1 || true)"
		[ -n "$last" ] && printf '  journal: %s\n' "$last"
	fi

	echo
done

if [ "$found" -eq 0 ]; then
	echo "no tq agent-pool/worker/serve processes running"
	exit 1
fi
