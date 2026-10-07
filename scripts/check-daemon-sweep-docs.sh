#!/usr/bin/env bash
# Daemon-sweep doc gate: run the cheap doc/config gates whenever a
# footer-less daemon sweep touched a doc-gated surface. Both multi-hour
# red windows in the 10-07 family trace to daemon `chore:` commits
# bypassing every hook (doc-refs ~18h red; e2e ~26h red) — the daemon is
# shared infra an in-repo hook cannot gate, so this script is the
# detector half: point it at a range and it decides whether the doc
# gates must run, then runs them. Wire it from the host cron/systemd
# timer per the ruling artifact
# (docs/planning/2026-10-07_04-20_daemon-doc-gate-wiring-ruling.md) and
# from ci-local (start-of-run: catches sweeps that landed mid-window).
#
# Usage: scripts/check-daemon-sweep-docs.sh [--since <ref>]
#        DAEMON_DOC_GATE_SELF_TEST=1 scripts/check-daemon-sweep-docs.sh
#   --since <ref>   base of the sweep scan (default origin/master)
#
# Exit: 0 gates ran green or no gated sweep found; 1 a gate failed.
set -uo pipefail
cd "$(dirname "$0")/.." || exit 1

# Doc-gated surfaces (the files the three cheap gates reason about).
gated() {
	case "$1" in
	AGENTS.md | README.md | TODO_LIST.md | CHANGELOG.md | FEATURES.md | ROADMAP.md) return 0 ;;
	docs/*) return 0 ;;
	*) return 1 ;;
	esac
}

# The daemon's subject template — mirrors the queue's folded-here regex
# (cmd/tq daemonCommitSubject; keep both in sync, row 243).
is_daemon_subject() {
	printf '%s' "$1" | grep -Eq '^chore: auto-commit [0-9]+ changed file\(s\) \(heuristic\)$'
}

has_footer() {
	printf '%s' "$1" | grep -q '^Task-Queue-ID: '
}

# scan <base>: prints the daemon-swept, footer-less, doc-gated commits
# (one sha per line). Empty output = nothing to gate.
scan() {
	local base=$1 c subj msg hit
	while IFS= read -r c; do
		subj=$(git log -1 --format='%s' "$c")
		is_daemon_subject "$subj" || continue
		msg=$(git log -1 --format='%B' "$c")
		has_footer "$msg" && continue
		hit=""
		while IFS= read -r f; do
			if gated "$f"; then
				hit=1
				break
			fi
		done < <(git diff-tree --no-commit-id --name-only -r "$c")
		[ -n "$hit" ] && printf '%s\n' "$c"
	done < <(git rev-list --reverse "$base..HEAD")
}

run_gates() {
	local rc=0 g
	for g in check-doc-refs.sh check-todo-list.sh check-status-index.sh; do
		echo "-- $g"
		# Self-test seam: dry mode pins the roster without running the
		# real gates (they are repo-bound and would gate the live tree).
		if [ "${DAEMON_DOC_GATE_DRY:-0}" = 1 ]; then
			continue
		fi
		./scripts/"$g" || rc=1
	done
	return $rc
}

self_test() {
	local tmp base rc
	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' RETURN
	git init -q "$tmp/repo"
	git -C "$tmp/repo" -c user.email=t@t -c user.name=t commit -qm init --allow-empty
	base=$(git -C "$tmp/repo" rev-parse HEAD)

	# Case 1: daemon-shaped footer-less sweep touching TODO_LIST.md → gated.
	printf -- '- [ ] probe row\n' >"$tmp/repo/TODO_LIST.md"
	git -C "$tmp/repo" add TODO_LIST.md
	git -C "$tmp/repo" -c user.email=t@t -c user.name=t commit -qm 'chore: auto-commit 1 changed file(s) (heuristic)'
	if [ -z "$(cd "$tmp/repo" && scan "$base")" ]; then
		echo "SELF-TEST FAIL: doc-gated daemon sweep not detected" >&2
		return 1
	fi

	# Case 2: same sweep shape but ONLY a .go file → not doc-gated.
	printf 'package x\n' >"$tmp/repo/x.go"
	git -C "$tmp/repo" add x.go
	git -C "$tmp/repo" -c user.email=t@t -c user.name=t commit -qm 'chore: auto-commit 1 changed file(s) (heuristic)'
	if [ "$(cd "$tmp/repo" && scan "$base" | wc -l)" != 1 ]; then
		echo "SELF-TEST FAIL: source-only daemon sweep wrongly gated" >&2
		return 1
	fi

	# Case 3: footer-carrying commit touching docs → never gated (healed
	# already; the queue can attribute it).
	printf 'note\n' >>"$tmp/repo/TODO_LIST.md"
	git -C "$tmp/repo" add TODO_LIST.md
	git -C "$tmp/repo" -c user.email=t@t -c user.name=t commit -qm 'task work

Task-Queue-ID: 000001a0c698deadbeef000000000000'
	if [ "$(cd "$tmp/repo" && scan "$base" | wc -l)" != 1 ]; then
		echo "SELF-TEST FAIL: footer-carrying commit wrongly gated" >&2
		return 1
	fi

	# Case 4: non-daemon subject touching docs → not this gate's class.
	printf 'note2\n' >>"$tmp/repo/TODO_LIST.md"
	git -C "$tmp/repo" add TODO_LIST.md
	git -C "$tmp/repo" -c user.email=t@t -c user.name=t commit -qm 'docs: manual edit'
	if [ "$(cd "$tmp/repo" && scan "$base" | wc -l)" != 1 ]; then
		echo "SELF-TEST FAIL: non-daemon commit wrongly gated" >&2
		return 1
	fi

	# Gate runner pin: dry mode replaces execution but keeps the roster.
	local roster
	roster="$(DAEMON_DOC_GATE_DRY=1 run_gates 2>/dev/null | sed 's/^-- //')"
	[ "$roster" = "check-doc-refs.sh
check-todo-list.sh
check-status-index.sh" ] || {
		echo "SELF-TEST FAIL: gate roster changed: $roster" >&2
		return 1
	}

	echo "self-test ok: sweep selection (daemon shape, footer-less, doc-gated) + gate roster pinned"
	return 0
}

if [ "${DAEMON_DOC_GATE_SELF_TEST:-0}" = 1 ]; then
	self_test
	exit $?
fi

since="origin/master"
if [ "${1:-}" = "--since" ] && [ $# -ge 2 ]; then
	since=$2
elif [ $# -ge 1 ]; then
	echo "usage: $0 [--since <ref>]" >&2
	exit 2
fi
git rev-parse --verify --quiet "$since" >/dev/null || {
	echo "FAIL: base ref '$since' not found" >&2
	exit 1
}

sweeps="$(scan "$since")"
if [ -z "$sweeps" ]; then
	echo "ok: no footer-less daemon sweeps touched doc-gated files since $since"
	exit 0
fi

echo "footer-less daemon sweeps touched doc-gated files:"
printf '%s\n' "$sweeps" | sed 's/^/  /'
echo "running cheap doc gates:"
run_gates
