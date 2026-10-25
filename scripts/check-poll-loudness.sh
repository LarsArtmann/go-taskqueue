#!/usr/bin/env bash
# Poll-loudness gate: every poll/absorption loop in scripts/ must carry a
# FAIL/WARN within its block or immediately after it (the 5d7e2dc0 audit
# found three serve-wait loops that broke out silently after exhausting
# their window; this gate makes the next silent clone a gate event instead
# of a point-in-time grep — 2026-10-14_03-46 §f4/e4).
#
# A "poll loop" is a for/while/until whose body contains `sleep` or whose
# header iterates `$(seq …)` (bounded retry windows). The loudness window
# is the loop body PLUS the 6 lines following the matching `done` (the
# canonical shapes put the exhaust message right after the loop:
# `if ! probe; then echo "WARN: …"` / `[ -n "$x" ] || { echo "FAIL: …" }`).
# `return N` alone is NOT loudness — a bare return is exactly the silent
# absorption this gate exists to prevent.
set -uo pipefail
cd "$(dirname "$0")/.." || exit 1
self_abs="$(cd "$(dirname "$0")" && pwd)/$(basename "$0")"

window_after_done=6
# `die` counts as loudness: it prints the exhaust reason and exits — the
# opposite of silent absorption (release.sh's CI-green poll loops end in die).
loud_pattern='FAIL|WARN|die '

# awk: emit "<file>:<loopstart>:<loud 0|1>" for every poll loop found.
scan_file() {
	awk -v win="$window_after_done" '
		/^[[:space:]]*(for|while|until)([[:space:]]|$)/ {
			if (depth == 0) {
				start = NR
				header = $0
				depth = 1
				body_loud = 0
				next
			}
			depth++
			if (depth > 0 && body && $0 ~ /(FAIL|WARN|die )/) body_loud = 1
			next
		}
		depth > 0 {
			if ($0 ~ /(^|[;&|[:space:]])(done)([[:space:]]|$)/) {
				depth--
				if (depth == 0) {
					is_poll = (body ~ /sleep/ || header ~ /\$\(seq/)
					if (is_poll) print FILENAME ":" start ":" NR ":" ((body_loud) ? 1 : 0)
					body = ""
					next
				}
			}
			if (body != "") body = body "\n" $0
			else body = $0
			if ($0 ~ /(FAIL|WARN|die )/) body_loud = 1
		}
	' "$1"
}

run_gate() {
	files=()
	if [ -n "${TQ_POLL_FILES:-}" ]; then
		while IFS= read -r f; do
			[ -n "$f" ] && files+=("$f")
		done <<<"$TQ_POLL_FILES"
	else
		while IFS= read -r f; do
			files+=("$f")
		done < <(git ls-files 'scripts/*.sh')
	fi

	if [ "${#files[@]}" -eq 0 ]; then
		echo "poll-loudness summary: 0 loops ok, 0 failed"
		return 0
	fi

	# Pass 1 (awk): body-loudness flags. Pass 2 (bash): extend the loudness
	# window `win` lines past the matching done — POSIX awk cannot unread.
	fail=0
	loops_ok=0
	loops_failed=0
	for f in "${files[@]}"; do
		[ -f "$f" ] || continue
		[ "$(basename "$f")" = "$(basename "$self_abs")" ] && continue # own negative fixtures are intentional
		while IFS=: read -r path start done_line body_loud; do
			[ -n "$path" ] || continue
			if [ "$body_loud" = 1 ]; then
				loops_ok=$((loops_ok + 1))
				continue
			fi
			tail="$(awk -v s="$done_line" -v win="$window_after_done" '
				NR > s && NR <= s + win { print }
			' "$f")"
			if printf '%s\n' "$tail" | grep -Eq "$loud_pattern"; then
				loops_ok=$((loops_ok + 1))
			else
				loops_failed=$((loops_failed + 1))
				fail=1
				echo "FAIL: $f:$start poll loop has no FAIL/WARN within its block or $window_after_done lines after done — silent-retry class (5d7e2dc0 audit; add an exhaust message)"
			fi
		done < <(scan_file "$f")
	done

	echo "poll-loudness summary: $loops_ok loops ok, $loops_failed failed"
	return "$fail"
}

self_test() {
	tmp="$(mktemp -d)"
	# NO EXIT trap here: command-substitution subshells inherit it and would
	# delete the fixtures after the first invocation.
	# good: loudness right after done
	cat >"$tmp/good.sh" <<'EOF'
for _ in $(seq 1 5); do
	probe && break
	sleep 0.1
done
[ -n "$x" ] || { echo "FAIL: never came up"; exit 1; }
EOF
	# bad: silent window exhaust
	cat >"$tmp/bad.sh" <<'EOF'
for _ in $(seq 1 5); do
	probe && break
	sleep 0.1
done
echo "moving on"
EOF
	# non-poll loop: no sleep, no seq — exempt
	cat >"$tmp/nonpoll.sh" <<'EOF'
for f in a b c; do
	echo "$f"
done
EOF
	local files out rc=0
	files="$(printf '%s\n' "$tmp/good.sh" "$tmp/bad.sh" "$tmp/nonpoll.sh")"
	out="$(TQ_POLL_FILES="$files" "$self_abs" 2>&1)" || rc=$?
	if [ "$rc" -ne 1 ]; then
		echo "FAIL: negative battery (want rc=1, got rc=$rc)"
		printf '%s\n' "$out"
		exit 1
	fi
	# nonpoll.sh is exempt (no sleep, no seq) and therefore absent from the
	# counters entirely — its non-detection is pinned by the exact counters.
	must_mention 'poll-loudness summary: 1 loops ok, 1 failed' "$out" "summary pins exact counters (good ok; bad failed; nonpoll exempt and uncounted)"
	must_mention "bad.sh:1 poll loop has no FAIL/WARN" "$out" "silent fixture is named with file:line"
	# positive-path: only the good fixture
	rc=0
	out="$(TQ_POLL_FILES="$tmp/good.sh" "$self_abs" 2>&1)" || rc=$?
	[ "$rc" -eq 0 ] || {
		echo "FAIL: positive fixture must pass (got rc=$rc)"
		printf '%s\n' "$out"
		exit 1
	}
	must_mention 'poll-loudness summary: 1 loops ok, 0 failed' "$out" "positive fixture summary"
	rm -rf "$tmp"
	echo "poll-loudness self-test ok (summary format pin + negative counter battery)"
}

must_mention() {
	if grep -qF -- "$1" <<<"$2"; then
		echo "ok: $3"
	else
		echo "FAIL: $3 (output is missing '$1')"
		printf '%s\n' "$2"
		exit 1
	fi
}

if [ "${1:-}" = "--self-test" ]; then
	self_test
	exit 0
fi

run_gate
