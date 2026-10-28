#!/usr/bin/env bash
# Report-body secret gate (closeout x security grid cell, GAP 2026-10-23).
# a)-g) closeout reports and status reports are agent-authored prose that
# quotes process output, but unlike tails/evidence/sidecars (redacted in
# internal/executor/redact.go before persistence) no check ever looked at
# the reports themselves. This gate scans every docs/status/**/*.md (live
# AND archived — a token in a frozen snapshot is still a token in the
# repo) with the SAME pattern classes as redact.go so a token that leaks
# into prose fails here instead of persisting in git history forever.
# Fork protection is mechanical: the pattern-class count is pinned against
# redact.go's secretPatterns table, and the SMOKETEST fake probe is
# grep-extracted from redact_test.go (same contract as
# scripts/smoke/journal-drift.sh) so quotes of the documented fake are
# allowlisted while anything else matching a class is a hard FAIL —
# file:line + class only, matched text is never echoed. 2026-10-28
# pre-gate sweep of all 1023 live+archived reports: zero live hits (only
# SMOKETEST quotes and task-ID false positives on the sk- classic shape,
# excluded by the non-word-prefix guard below).
set -uo pipefail
export LC_ALL=C
cd "$(dirname "$0")/.." || exit 1
self_abs="$(cd "$(dirname "$0")" && pwd)/$(basename "$0")"

REDACT_GO="internal/executor/redact.go"
REDACT_TEST="internal/executor/redact_test.go"

# POSIX ERE mirrors of redact.go's secretPatterns, in table order.
# Prefixed token shapes stay case-sensitive (their specs fix the case);
# word-based shapes run -i like the Go table's (?i) flags. The
# (^|[^A-Za-z0-9_-]) guard is a prose-only refinement: report text is
# full of task IDs ("task-0000…" ends in "sk-…"), which the Go table's
# tail-scanning context tolerates but a report scanner must not flag.
PASS_NAMES=()
PASS_RES=()
PASS_NOCASE=()

add_pass() {
	PASS_NAMES+=("$1")
	PASS_RES+=("$2")
	PASS_NOCASE+=("$3")
}

add_pass anthropic '(^|[^A-Za-z0-9_-])sk-ant-[A-Za-z0-9_-]{16,}' ''
add_pass openai-project '(^|[^A-Za-z0-9_-])sk-(proj|svcacct)-[A-Za-z0-9_-]{20,}' ''
add_pass openai-classic '(^|[^A-Za-z0-9_-])sk-[A-Za-z0-9]{20,}' ''
add_pass github 'gh[pousr]_[A-Za-z0-9]{20,}' ''
add_pass github-fine 'github_pat_[A-Za-z0-9_]{20,}' ''
add_pass aws '(AKIA|ASIA)[A-Z0-9]{16}' ''
add_pass google 'AIza[0-9A-Za-z_-]{35}' ''
add_pass slack 'xox[baprs]-[A-Za-z0-9-]{10,}' ''
add_pass bearer '(^|[^A-Za-z0-9_-])bearer[[:space:]]+[A-Za-z0-9._~+/=-]{16,}' i
add_pass authorization '(^|[^A-Za-z0-9_-])authorization["'"'"']?[[:space:]]*[:=][[:space:]]*["'"'"']?(bearer[[:space:]]+)?[A-Za-z0-9._~+/=-]{16,}' i
add_pass assignment '(^|[^A-Za-z0-9_-])(api[_-]?key|apikey|secret|access[_-]?token|auth[_-]?token|password|passwd)["'"'"']?[[:space:]]*[:=][[:space:]]*["'"'"']?[A-Za-z0-9+/_-]{12,}' i

fail_hard() {
	echo "report-secrets FAIL: $1"
	exit 1
}

# ONE-table pin: redact.go's secretPatterns table grew (or shrank) without
# this gate — the author must extend the passes above in the same change.
pin_pattern_count() {
	awk '/^var secretPatterns = \[\]\*regexp.Regexp\{/{on=1; next} /^\}/{on=0} on' "$REDACT_GO" |
		grep -c 'regexp.MustCompile'
}

# The documented fake probe, extracted so its quotes are allowlisted by
# data rather than a hardcoded copy (journal-drift.sh contract).
extract_probe() {
	sed -n 's/^const fakeAuditProbe = "\(.*\)"$/\1/p' "$REDACT_TEST"
}

collect_files() {
	if [ -n "${TQ_REPORT_SECRET_FILES:-}" ]; then
		while IFS= read -r f; do
			[ -n "$f" ] && printf '%s\n' "$f"
		done <<<"$TQ_REPORT_SECRET_FILES"
	else
		find docs/status -type f -name '*.md' | sort
	fi
}

scan_file() {
	local f="$1" i line ln name
	local file_hits=0 file_allowed=0
	for i in "${!PASS_NAMES[@]}"; do
		name="${PASS_NAMES[$i]}"
		local matches
		if [ "${PASS_NOCASE[$i]}" = i ]; then
			matches="$(grep -Ein -- "${PASS_RES[$i]}" "$f" 2>/dev/null || true)"
		else
			matches="$(grep -En -- "${PASS_RES[$i]}" "$f" 2>/dev/null || true)"
		fi
		[ -n "$matches" ] || continue
		while IFS= read -r line; do
			[ -n "$line" ] || continue
			ln="${line%%:*}"
			if [ -n "$PROBE" ] && grep -qF -- "$PROBE" <<<"$line"; then
				file_allowed=$((file_allowed + 1))
				continue
			fi
			file_hits=$((file_hits + 1))
			echo "report-secrets FAIL: $f:$ln matches secret class '$name' — redact or remove the token (never echo it here)"
		done <<<"$matches"
	done
	HITS=$((HITS + file_hits))
	ALLOWED=$((ALLOWED + file_allowed))
}

self_test() {
	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' EXIT
	# Clean prose: negative shapes from redact_test.go + the task-ID FP.
	printf '%s\n' \
		'password: (none)' \
		'asking for bearer token required for endpoint' \
		'AKIA too short' \
		'task-000001a180325195b42bf158d5b000000000 landed via footer' \
		'the api_key flag is unset and secret rotation is documented' \
		>'$tmp/clean.md'
	# Dirty prose: one hit per representative class (5 expected).
	printf '%s\n' \
		'failed with: sk-ant-api03-abcdefghijklmnopqrst' \
		'token ghp_abcdefghijklmnopqrstuvwxyz0123456789 rejected' \
		'creds AKIAIOSFODNN7EXAMPLE in env dump' \
		'set password = a1b2c3d4e5f6g7h8i9j0 here' \
		'sent Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ4In0.sig' \
		>'$tmp/dirty.md'
	# Documented fake: allowlisted by the extracted probe.
	printf '%s\n' 'probe quote: sk-ant-SMOKETEST-0000000000000000 (documented fake fixture)' >'$tmp/fake.md'

	local out rc
	out="$(TQ_REPORT_SECRET_FILES="$tmp/clean.md" "$self_abs" 2>&1)"
	rc=$?
	if [ "$rc" -ne 0 ] || ! grep -qF 'report-secrets summary: 1 files scanned, 0 hits, 0 fake-probe allowlisted' <<<"$out"; then
		echo "SELF-TEST FAIL: clean fixture flagged (rc=$rc)"
		printf '%s\n' "$out"
		exit 1
	fi

	out="$(TQ_REPORT_SECRET_FILES="$tmp/dirty.md" "$self_abs" 2>&1)"
	rc=$?
	if [ "$rc" -ne 1 ]; then
		echo "SELF-TEST FAIL: dirty fixture did not fail the gate (rc=$rc)"
		printf '%s\n' "$out"
		exit 1
	fi
	if ! grep -qF 'report-secrets summary: 1 files scanned, 5 hits, 0 fake-probe allowlisted' <<<"$out"; then
		echo "SELF-TEST FAIL: dirty-fixture counters off (want 5 hits)"
		printf '%s\n' "$out"
		exit 1
	fi
	if [ "$(grep -c 'report-secrets FAIL' <<<"$out")" -ne 5 ]; then
		echo "SELF-TEST FAIL: expected 5 FAIL lines for the dirty fixture"
		printf '%s\n' "$out"
		exit 1
	fi
	local secret
	while IFS= read -r secret; do
		if grep -qF "$secret" <<<"$out"; then
			echo "SELF-TEST FAIL: gate echoed matched secret text"
			exit 1
		fi
	done <<'SECRETS'
sk-ant-api03-abcdefghijklmnopqrst
ghp_abcdefghijklmnopqrstuvwxyz0123456789
AKIAIOSFODNN7EXAMPLE
a1b2c3d4e5f6g7h8i9j0
eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9
SECRETS

	out="$(TQ_REPORT_SECRET_FILES="$tmp/fake.md" "$self_abs" 2>&1)"
	rc=$?
	if [ "$rc" -ne 0 ] || ! grep -qF 'report-secrets summary: 1 files scanned, 0 hits, 1 fake-probe allowlisted' <<<"$out"; then
		echo "SELF-TEST FAIL: documented fake probe was flagged (rc=$rc)"
		printf '%s\n' "$out"
		exit 1
	fi

	echo "report-secrets self-test ok (summary format pin + clean/dirty/fake battery + no-echo pin)"
	exit 0
}

if [ "${1:-}" = "--self-test" ]; then
	self_test
	exit 0
fi

[ -f "$REDACT_GO" ] || fail_hard "$REDACT_GO missing — the secretPatterns table moved; repoint this gate"
[ -f "$REDACT_TEST" ] || fail_hard "$REDACT_TEST missing — the fakeAuditProbe fixture moved; repoint this gate"

go_count="$(pin_pattern_count)"
[ -n "$go_count" ] || fail_hard "could not count secretPatterns in $REDACT_GO — extraction broke"
if [ "$go_count" != "${#PASS_NAMES[@]}" ]; then
	fail_hard "redact.go has $go_count secretPatterns but this gate pins ${#PASS_NAMES[@]} passes — extend check-report-secrets.sh in the same change as secretPatterns (ONE table: redact.go + redact_test.go samples + this gate)"
fi

PROBE="$(extract_probe)"
[ -n "$PROBE" ] || fail_hard "fakeAuditProbe missing from $REDACT_TEST — the shared SECRET EVIDENCE fixture moved; update this extraction"

HITS=0
ALLOWED=0
FILES=0
while IFS= read -r f; do
	[ -f "$f" ] || continue
	FILES=$((FILES + 1))
	scan_file "$f"
done < <(collect_files)

echo "report-secrets summary: $FILES files scanned, $HITS hits, $ALLOWED fake-probe allowlisted"
if [ "$HITS" -gt 0 ]; then
	echo "secret-shaped text persists in agent-authored report prose — redact the token(s) (history rewrite is NOT required if the token is dead/rotated; rotate first)"
	exit 1
fi
exit 0
