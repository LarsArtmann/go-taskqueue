#!/usr/bin/env bash
# Push-timing guard (TODO row 949; 2026-10-15 00-44 report §d1/§g1): the
# auto-commit daemon's footer-less `chore:` sweeps commit continuously, and
# an in-flight task heals them into attributed commits via
# scripts/heal-daemon-sweep.sh — but only while they are UNPUSHED (heal
# refuses pushed commits under the history policy). c7e95833 was pushed
# mid-fold in 2026-10-15 and the frozen-date sweep's substance stayed
# unattributed forever. Installed as the pre-push hook by
# scripts/install-pre-commit.sh: refuse pushing a footer-less daemon
# commit younger than TQ_PUSH_MIN_AGE seconds (default 600 = 10 minutes —
# longer than a sweep-to-heal window, shorter than a session).
#
# --self-test pins the summary format and runs a negative/positive battery
# over a fixture repo with backdated commit timestamps (fleet convention:
# countable gate summaries are PINNED).
set -euo pipefail
cd "$(dirname "$0")/.."

min_age="${TQ_PUSH_MIN_AGE:-600}"

# check_commit <sha> <now>: rc 0 = allowed, 1 = refuse (too young).
check_commit() {
	local sha="$1" now="$2" subject trailers commit_time age
	subject="$(git log -1 --format=%s "$sha")"
	case "$subject" in
	"chore: auto-commit"*) ;;
	*) return 0 ;;
	esac
	trailers="$(git log -1 --format='%(trailers)' "$sha")"
	if printf '%s\n' "$trailers" | grep -qi '^Task-Queue-ID:'; then
		return 0
	fi
	commit_time="$(git log -1 --format=%ct "$sha")"
	age=$((now - commit_time))
	if [ "$age" -lt "$min_age" ]; then
		printf 'PUSH-TIMING REFUSAL: %s (%s) is %ss old, younger than TQ_PUSH_MIN_AGE=%ss\n' "$sha" "$subject" "$age" "$min_age" >&2
		printf '  a footer-less daemon sweep this fresh may be mid-fold: an in-flight task can still heal it into an attributed commit via scripts/heal-daemon-sweep.sh — pushing now unattributes its substance permanently (c7e95833 class, 2026-10-15)\n' >&2
		printf '  escape: wait %ss and retry, or push explicitly bypassing the hook (git push --no-verify) after confirming no task is folding it\n' "$((min_age - age))" >&2
		return 1
	fi
	return 0
}

# run_hook <now> on pre-push stdin lines "<local-ref> <local-sha> <remote-ref> <remote-sha>".
run_hook() {
	local now="$1" refused=0 total=0 line lsha
	refused_summary="push-timing summary: 0 commits checked, 0 refused"
	while read -r _ lsha _ _; do
		case "$lsha" in
		0000000000000000000000000000000000000000) continue ;;
		esac
		total=$((total + 1))
		if ! check_commit "$lsha" "$now"; then
			refused=$((refused + 1))
		fi
	done
	refused_summary="push-timing summary: $total commits checked, $refused refused"
	if [ "$refused" -gt 0 ]; then
		echo "$refused_summary" >&2
		return 1
	fi
	echo "$refused_summary"
}

if [ "${1:-}" = "--self-test" ]; then
	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' EXIT
	git init -q "$tmp"
	git -C "$tmp" config user.email gate@tq.local
	git -C "$tmp" config user.name gate
	base=1767225600
	now=$((base + 700))
	printf seed >"$tmp/f"
	git -C "$tmp" add -A
	git -C "$tmp" commit -qm 'seed (non-daemon)'
	# Backdated footer-less daemon commit (old → allowed) and a fresh one (refused).
	printf x >"$tmp/f"
	git -C "$tmp" add -A
	GIT_AUTHOR_DATE='2026-01-01T00:00:00 +0000' GIT_COMMITTER_DATE='2026-01-01T00:00:00 +0000' \
		git -C "$tmp" commit -qm 'chore: auto-commit 1 changed file(s) (heuristic)'
	old_sha="$(git -C "$tmp" rev-parse HEAD)"
	printf y >"$tmp/f"
	git -C "$tmp" add -A
	GIT_AUTHOR_DATE="@$((base + 400)) +0000" GIT_COMMITTER_DATE="@$((base + 400)) +0000" \
		git -C "$tmp" commit -qm 'chore: auto-commit 2 changed file(s) (heuristic)'
	fresh_sha="$(git -C "$tmp" rev-parse HEAD)"
	# Attributed fresh daemon commit (allowed — a healed sweep pushes fine).
	printf z >"$tmp/f"
	git -C "$tmp" add -A
	GIT_AUTHOR_DATE="@$((base + 400)) +0000" GIT_COMMITTER_DATE="@$((base + 400)) +0000" \
		git -C "$tmp" commit -qm 'chore: auto-commit 1 changed file(s) (heuristic)

Task-Queue-ID: 000001a175f4de403dc14ed9579100000000'
	healed_sha="$(git -C "$tmp" rev-parse HEAD)"
	non_daemon_sha="$(git -C "$tmp" rev-parse HEAD~3)"

	cd "$tmp"
	rc=0
	check_commit "$old_sha" "$now" || rc=1
	[ "$rc" -eq 0 ] || echo "FAIL: backdated daemon commit should be allowed"
	check_commit "$healed_sha" "$now" || rc=1
	[ "$rc" -eq 0 ] || echo "FAIL: attributed (healed) daemon commit should be allowed"
	check_commit "$non_daemon_sha" "$now" || {
		echo "FAIL: non-daemon commit should be allowed regardless of age"
		rc=1
	}
	if check_commit "$fresh_sha" "$now" 2>/dev/null; then
		echo "FAIL: fresh footer-less daemon commit should be refused"
		rc=1
	fi
	out="$(printf 'refs/heads/master %s refs/heads/master %s\n' "$fresh_sha" "$fresh_sha" | run_hook "$now" 2>&1)" && hook_rc=0 || hook_rc=$?
	[ "$hook_rc" -eq 1 ] || {
		echo "FAIL: hook should refuse the fresh commit batch (rc=$hook_rc)"
		rc=1
	}
	case "$out" in
	*"push-timing summary: 1 commits checked, 1 refused"*) ;;
	*)
		echo "FAIL: summary pin mismatch:"
		printf '%s\n' "$out"
		rc=1
		;;
	esac
	out="$(printf 'refs/heads/master %s refs/heads/master %s\n' "$old_sha" "$old_sha" | run_hook "$now")"
	case "$out" in
	*"push-timing summary: 1 commits checked, 0 refused"*) ;;
	*)
		echo "FAIL: allowed-batch summary pin mismatch:"
		printf '%s\n' "$out"
		rc=1
		;;
	esac
	# Non-daemon subjects bypass the age check entirely.
	subject_override_check="$(git log -1 --format=%s "$non_daemon_sha")"
	case "$subject_override_check" in
	"chore: auto-commit"*) echo "FAIL: fixture hygiene"; rc=1 ;;
	esac
	[ "$rc" -eq 0 ] || exit 1
	echo "push-timing self-test ok (summary format pin + 3-way fixture battery: backdated allowed, fresh refused, healed allowed)"
	exit 0
fi

# Hook mode: pre-push stdin (no args = not a hook invocation).
if [ "${1:-}" = "" ]; then
	echo "usage: check-push-timing.sh (pre-push stdin) | --self-test" >&2
	exit 2
fi

now="$(date +%s)"
run_hook "$now"
