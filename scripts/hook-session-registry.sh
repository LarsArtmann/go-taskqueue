#!/usr/bin/env bash
# PreToolUse hook (interactive crush sessions): record the session in the tq
# session registry (one {id, cwd, last_seen} line per tool call) so
# `tq session sweep` can mint the review + status close-out once no crush
# process owns the session id anymore. This is the interim trigger until
# crush #3146 ships SessionEnd hooks.
#
# Wiring (owner-run, in the crush config hooks section):
#   PreToolUse -> bash scripts/hook-session-registry.sh
#
# Requires CRUSH_SESSION_ID in the hook environment and tq on PATH. The ping
# touches only the registry file, never the queue database. Best-effort
# telemetry: any failure exits 0 so it can never block a tool call.
set -euo pipefail

if [ -z "${CRUSH_SESSION_ID:-}" ]; then
	exit 0
fi

tq session ping >/dev/null 2>&1 || true
exit 0
