#!/usr/bin/env bash
# One-command post-config gosec gate (round-13 T6 triage encoded as excludes).
#
# Pinned gosec v2.29.0 with the CI-encoded exclude set — any finding is a
# NEW class needing a fresh triage note (AGENTS.md "gosec advisory
# baseline"), never a silent re-exclusion.
#
# Every scan must report Files > 0 — a scanner must prove it actually
# scanned, not trust the exit code. Two known Files:0 silent-skip shapes
# are therefore hard failures: (a) a module-context scan without
# GOWORK=off (`cd internal/session && gosec ./...` returns 0/0 silently);
# (b) a root-module package path resolved outside its module. Root-module
# packages are covered by the root `./...` scan; each sub-module is scanned
# in place with GOWORK=off. Silent-skip shapes live a level above the
# scanner: a broken or empty module enumeration would silently narrow the
# gate to the root scan and exit 0 (process-substitution exit status is
# ignored), so a failing or zero-target enumeration hard-fails before any
# scan runs (02-18 report §e6). Zero targets is not the only blind spot:
# a non-zero-but-SHORT enumeration (a renamed sub-module dropping out of
# find) scans the survivors and still exits 0, so the enumeration must
# also contain the pinned canary sub-modules or the gate fails fast.
set -euo pipefail
# gate_run re-invokes THIS script via "$0" after the cd below has moved the
# working directory to the repo root; a CWD-relative $0 (e.g.
# ./check-gosec.sh from inside scripts/) stops resolving there — the child
# dies rc=127 under stock bash (mvdan/sh masks it). Capture the absolute
# self path BEFORE moving (02-38 report §b).
self_abs="$(cd "$(dirname "$0")" && pwd)/$(basename "$0")"
cd "$(dirname "$0")/.."

GOSEC_VERSION=v2.29.0
GOSEC_EXCLUDES="-exclude=G104,G115,G118,G124,G202,G204,G301,G302,G304,G306,G404,G702,G703,G710"

# --self-test pins the gate's decision branches with canned stub binaries
# (02-52 gosec-gate report f3; the 2026-09-19 verify run's /tmp stubs are
# the design): the three version branches (stamped pin ok / stamped
# mismatch FAIL / unstamped WARN), the Files:0 silent-skip parse, the
# Issues>0 parse, and the module-enumeration failures (zero-target list /
# non-zero enumerator exit, via the TQ_GOSEC_ENUM stub hook), plus the
# foreign-CWD re-entry branch (the mode re-invoked from / through the
# absolute self path; TQ_GOSEC_SELFTEST_REENTRY=1 guards the child against
# infinite recursion — externally settable, a conscious escape hatch), plus
# the ci.yml derivation-drift guard: the workflow's exact sed lines are
# extracted from .github/workflows/ci.yml, run against THIS script, and
# byte-compared with the pinned constants — the derivation runs only on
# runners inside a continue-on-error job, so without this case a format
# drift fails advisory-only in CI (harvested 2026-09-19, derive window).
# The proof lives in a runnable gate instead of
# report prose. Each case runs THIS script recursively with GOSEC pointed
# at a stub, exercising the shipped bytes end to end. Stubs are created in
# a mktemp dir outside the gated tree and removed on exit; stub outputs
# auto-adapt to a future GOSEC_VERSION bump (only the stale and dev stamps
# are hardcoded, and both stay correct under any pin).
stub_write() {
	local path="$1" version="$2" files="$3" issues="$4"
	cat >"$path" <<EOF
#!/bin/sh
if [ "\$1" = "-version" ]; then
	printf 'Version: $version\n'
	exit 0
fi
printf '   Files : $files\n'
printf '   Issues : $issues\n'
exit 0
EOF
	chmod +x "$path"
}

last_out=""
gate_run() {
	local name="$1" want_rc="$2" stub="$3" enum="${4:-}"
	local rc=0
	if [ -n "$enum" ]; then
		last_out="$(GOSEC="$stub" TQ_GOSEC_ENUM="$enum" "$self_abs" 2>&1)" || rc=$?
	else
		last_out="$(GOSEC="$stub" "$self_abs" 2>&1)" || rc=$?
	fi
	if [ "$rc" -ne "$want_rc" ]; then
		echo "FAIL: $name (want rc=$want_rc, got rc=$rc)"
		printf '%s\n' "$last_out"
		exit 1
	fi
	echo "ok: $name"
}

must_mention() {
	if grep -qF -- "$1" <<<"$last_out"; then
		echo "ok: $2"
	else
		echo "FAIL: $2 (output is missing '$1')"
		printf '%s\n' "$last_out"
		exit 1
	fi
}

must_not_mention() {
	if grep -qF -- "$1" <<<"$last_out"; then
		echo "FAIL: $2 (output must not contain '$1')"
		printf '%s\n' "$last_out"
		exit 1
	fi
	echo "ok: $2"
}

self_test() {
	local ok stale dev files0 findings
	# tmp is deliberately global: the EXIT trap reads it after self_test
	# returned, and set -u would kill the trap on a function-local name.
	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' EXIT
	ok="$tmp/gosec-ok-stub"
	stale="$tmp/gosec-stale-stub"
	dev="$tmp/gosec-dev-stub"
	files0="$tmp/gosec-files0-stub"
	findings="$tmp/gosec-findings-stub"
	stub_write "$ok" "$GOSEC_VERSION" 5 0
	stub_write "$stale" v2.28.1 5 0
	stub_write "$dev" dev 5 0
	stub_write "$files0" "$GOSEC_VERSION" 0 0
	stub_write "$findings" "$GOSEC_VERSION" 3 2
	emptyenum="$tmp/enum-empty"
	failenum="$tmp/enum-fail"
	shortenum="$tmp/enum-short"
	cat >"$emptyenum" <<'EOF'
#!/bin/sh
exit 0
EOF
	chmod +x "$emptyenum"
	cat >"$failenum" <<'EOF'
#!/bin/sh
echo "find: internal: No such file or directory" >&2
exit 1
EOF
	chmod +x "$failenum"
	# Non-zero-but-short: a renamed sub-module dropping out of the
	# enumeration leaves survivors behind — the shape both the 0-target
	# guard and the per-scan Files>0 guard are blind to.
	cat >"$shortenum" <<'EOF'
#!/bin/sh
printf '%s\n' internal/task internal/journal
exit 0
EOF
	chmod +x "$shortenum"

	gate_run "stamped pin ok: full gate passes on a $GOSEC_VERSION stub" 0 "$ok"
	must_mention "ok: $ok is $GOSEC_VERSION" "stamped ok states the pin"
	must_mention "ok: Files=" "Files>0 Issues=0 summary parses green"
	must_mention "scans ok, 0 failed" "end-of-run summary counts scans (the version-ok line is not a scan)"
	must_not_mention "WARN:" "stamped ok emits no WARN"
	must_not_mention "FAIL:" "stamped ok emits no FAIL"

	gate_run "stamped mismatch: stale stub exits 1" 1 "$stale"
	must_mention "reports 'Version: v2.28.1', not the pinned $GOSEC_VERSION" "mismatch names the detected version"
	must_mention "go install github.com/securego/gosec/v2/cmd/gosec@$GOSEC_VERSION" "mismatch carries the install fix line"
	must_mention "nothing was scanned" "mismatch fails fast before the first scan"
	must_not_mention "== (root)" "mismatch runs zero scans"

	gate_run "unstamped WARN: dev-stamped stub warns and continues" 0 "$dev"
	must_mention "is UNSTAMPED ('Version: dev')" "unstamped names the provenance gap"
	must_mention "== (root)" "unstamped gate proceeds to scans"
	must_mention "ok: Files=" "unstamped scans still parse green"

	gate_run "Files:0 parse: silent-skip summary hard-fails the gate" 1 "$files0"
	must_mention "scanned 0 files" "Files:0 reports the silent-skip failure"
	must_mention "0 scans ok," "Files:0 run's summary counts every scan as failed"

	gate_run "Issues parse: findings hard-fail the gate" 1 "$findings"
	must_mention "finding(s)" "Issues>0 reports the new-class failure"

	gate_run "empty enumeration: 0 module targets hard-fail" 1 "$ok" "$emptyenum"
	must_mention "0 module targets" "empty enumeration reports the silent-narrow failure"
	must_not_mention "== (root)" "empty enumeration fails before any scan"

	gate_run "broken enumeration: non-zero exit hard-fail" 1 "$ok" "$failenum"
	must_mention "enumeration exited non-zero" "broken enumeration reports the enumerator failure"
	must_not_mention "== (root)" "broken enumeration fails before any scan"

	gate_run "short enumeration: missing canary member hard-fail" 1 "$ok" "$shortenum"
	must_mention "missing canary module(s): internal/queue internal/executor internal/worker" "short enumeration names every missing canary"
	must_mention "renamed or removed sub-module" "short enumeration carries the conscious-update fix hint"
	must_not_mention "== (root)" "short enumeration fails before any scan"

	# ci.yml derivation drift: run the workflow's EXACT sed lines (lifted
	# from ci.yml itself, so any edit there is what gets tested) against
	# this script and byte-compare with the pinned constants above.
	pin_version="$GOSEC_VERSION"
	pin_excludes="$GOSEC_EXCLUDES"
	ci_yml=".github/workflows/ci.yml"
	ci_v_line="$(grep -F 'GOSEC_VERSION="$(sed' "$ci_yml" | head -n1)"
	ci_e_line="$(grep -F 'GOSEC_EXCLUDES="$(sed' "$ci_yml" | head -n1)"
	if [ -z "$ci_v_line" ] || [ -z "$ci_e_line" ]; then
		echo "FAIL: ci.yml derivation drift: sed line(s) not found in $ci_yml"
		exit 1
	fi
	eval "$ci_v_line"
	eval "$ci_e_line"
	if [ -z "${GOSEC_VERSION:-}" ] || [ -z "${GOSEC_EXCLUDES:-}" ]; then
		echo "FAIL: ci.yml derivation drift: workflow sed lines extract empty values"
		exit 1
	fi
	if [ "$GOSEC_VERSION" != "$pin_version" ] || [ "$GOSEC_EXCLUDES" != "$pin_excludes" ]; then
		echo "FAIL: ci.yml derivation drift: workflow derives version '$GOSEC_VERSION' / excludes '$GOSEC_EXCLUDES', but the pinned constants are '$pin_version' / '$pin_excludes'"
		exit 1
	fi
	echo "ok: ci.yml derivation matches the pinned constants byte-for-byte"

	# .golangci.yml scanner parity: the linter's gosec.excludes list must
	# equal the pinned GOSEC_EXCLUDES (set-compare; the linter uses a YAML
	# list, the gate a comma flag string). Without this a triage edit in
	# this script would silently skew the two scanner surfaces apart.
	lc_yml=".golangci.yml"
	lc_excludes="$(awk '/^    gosec:/{f=1; next} f && /^    [^ ]/{f=0} f && /^ +- /{sub(/^ +- /, ""); print}' "$lc_yml" | sort)"
	pin_excludes_sorted="$(printf '%s' "$pin_excludes" | sed 's/^-exclude=//' | tr ',' '\n' | sort)"
	if [ -z "$lc_excludes" ]; then
		echo "FAIL: .golangci.yml parity: gosec excludes list not found or empty in $lc_yml"
		exit 1
	fi
	if [ "$lc_excludes" != "$pin_excludes_sorted" ]; then
		echo "FAIL: .golangci.yml parity: linter gosec.excludes $(printf '%s' "$lc_excludes" | tr '\n' ' ')differ from GOSEC_EXCLUDES $(printf '%s' "$pin_excludes_sorted" | tr '\n' ' ')"
		echo "  fix: update .golangci.yml gosec.excludes to match GOSEC_EXCLUDES in this script (the single triage source)"
		exit 1
	fi
	echo "ok: .golangci.yml gosec.excludes matches the pinned GOSEC_EXCLUDES"

	# Foreign-CWD re-entry: a caller parked anywhere must be able to run
	# the mode — the guard var stops the child from re-running this
	# assertion (it would recurse forever), one re-entry level total.
	if [ -z "${TQ_GOSEC_SELFTEST_REENTRY:-}" ]; then
		if (cd / && TQ_GOSEC_SELFTEST_REENTRY=1 "$self_abs" --self-test >/dev/null 2>&1); then
			echo "ok: --self-test re-runs from a foreign CWD"
		else
			echo "FAIL: --self-test invoked from a foreign CWD"
			exit 1
		fi
	fi

	# Assertion-liveness negative control, daemon-proof (06-22 §f10):
	# prove the foreign-CWD assertion can actually FIRE, without mutating
	# the gated tree — the old control copied the script in-tree with
	# "$0" restored and trashed it (daemon-food per AGENTS.md). A /tmp
	# extract of THIS script with the self_abs line sed-reverted to the
	# raw "$0" (the pre-fix shape) is invoked CWD-relative from the
	# extract's scripts/ dir; the top-level cd then strands the recursive
	# child, and the first gate_run reports got rc=127. The extract lives
	# under $tmp and dies with the EXIT trap.
	if [ -z "${TQ_GOSEC_SELFTEST_REENTRY:-}" ]; then
		ctl_dir="$tmp/negctl/scripts"
		mkdir -p "$ctl_dir"
		sed 's|^self_abs=.*|self_abs="$0"|' "$self_abs" >"$ctl_dir/check-gosec.sh"
		chmod +x "$ctl_dir/check-gosec.sh"
		ctl_rc=0
		ctl_out="$(cd "$ctl_dir" && TQ_GOSEC_SELFTEST_REENTRY=1 bash ./check-gosec.sh --self-test 2>&1)" || ctl_rc=$?
		if [ "$ctl_rc" -ne 0 ] && grep -qF 'rc=127' <<<"$ctl_out"; then
			echo "ok: negative control trips rc=127 on the reverted self_abs extract (assertion is live)"
		else
			echo "FAIL: negative control did not trip rc=127 (rc=$ctl_rc) — the foreign-CWD assertion may be dead"
			printf '%s\n' "$ctl_out"
			exit 1
		fi
	fi

	echo "gosec self-test ok (three version branches, Files:0 and Issues>0 parses, empty, broken, and short module enumeration pinned via stub gates, ci.yml derivation-drift byte-match, .golangci.yml gosec-excludes parity, foreign-CWD re-entry, daemon-proof rc=127 negative control)"
}

if [ "${1:-}" = "--self-test" ]; then
	self_test
	exit 0
fi

GOSEC_BIN="${GOSEC:-}"
gosec_preexisting=0
if [ -n "$GOSEC_BIN" ]; then
	gosec_preexisting=1
else
	for candidate in "$(go env GOPATH)/bin/gosec" "$(command -v gosec || true)"; do
		if [ -n "$candidate" ] && [ -x "$candidate" ]; then
			GOSEC_BIN="$candidate"
			gosec_preexisting=1
			break
		fi
	done
fi
if [ -z "$GOSEC_BIN" ]; then
	echo "gosec not found; installing pinned $GOSEC_VERSION"
	go install "github.com/securego/gosec/v2/cmd/gosec@$GOSEC_VERSION"
	GOSEC_BIN="$(go env GOPATH)/bin/gosec"
fi

# The pin must hold for a pre-existing binary too (GOSEC override or
# PATH/GOPATH discovery): a stale gosec silently changes what green means.
# Only this script's own install path is exempt (v2.29.0 by construction).
# An UNSTAMPED binary (Version: dev, e.g. a scratch-module build) cannot be
# verified: it warns instead of failing because go install is
# sandbox-blocked for agent sessions on this host and binary provenance is
# an open owner ruling (2026-09-18_02-52 gosec-gate report §g q2).
if [ "$gosec_preexisting" -eq 1 ]; then
	gosec_version="$("$GOSEC_BIN" -version 2>&1 || true)"
	gosec_version_first="${gosec_version%%$'\n'*}"
	case "$gosec_version" in
	*"$GOSEC_VERSION"*)
		echo "ok: $GOSEC_BIN is $GOSEC_VERSION"
		;;
	*"Version: dev"* | *"Version: (devel)"*)
		echo "WARN: $GOSEC_BIN is UNSTAMPED ('$gosec_version_first'), pin $GOSEC_VERSION UNVERIFIED (provenance ruling pending: 02-52 report §g q2); green stays unproven until the binary states its version"
		;;
	*)
		echo "FAIL: $GOSEC_BIN reports '$gosec_version_first', not the pinned $GOSEC_VERSION. A stale gosec silently changes what green means; nothing was scanned"
		echo "  fix: go install github.com/securego/gosec/v2/cmd/gosec@$GOSEC_VERSION (or point GOSEC at a $GOSEC_VERSION binary), then rerun"
		exit 1
		;;
	esac
fi

scan() {
	local bin="$1" target="$2"
	local out plain files issues
	# shellcheck disable=SC2086  # GOSEC_EXCLUDES is a flag list, word-splitting intended
	out="$(GOWORK=off "$bin" $GOSEC_EXCLUDES "$target" 2>&1 || true)"
	plain="$(printf '%s' "$out" | sed 's/\x1b\[[0-9;]*m//g')"
	files="$(printf '%s\n' "$plain" | awk '/^[[:space:]]*Files[[:space:]]*:/ { print $3 }')"
	issues="$(printf '%s\n' "$plain" | awk '/^[[:space:]]*Issues[[:space:]]*:/ { print $3 }')"
	if [ -z "$files" ] || [ "$files" -eq 0 ]; then
		echo "FAIL: $target scanned 0 files (silent skip — a scanner must prove it scanned; use GOWORK=off in-module or a root-module path)"
		return 1
	elif [ "${issues:-0}" -gt 0 ]; then
		printf '%s\n' "$plain"
		echo "FAIL: $target has $issues finding(s) — a NEW gosec class: triage it and extend the AGENTS.md baseline note, never silently re-exclude"
		return 1
	fi
	echo "ok: Files=$files Issues=0"
	return 0
}

fail=0
scans_ok=0
scans_failed=0
enum_cmd="${TQ_GOSEC_ENUM:-./scripts/for-each-module.sh}"
modules="$("$enum_cmd")" || {
	echo "FAIL: module enumeration exited non-zero ($enum_cmd) — a broken enumerator would silently narrow the gate to the root scan only"
	exit 1
}
if [ -z "$modules" ]; then
	echo "FAIL: module enumeration returned 0 module targets (silent skip one level up — the gate would narrow to the root scan only; fix $enum_cmd)"
	exit 1
fi
# Canary members: the 0-target guard cannot see a MISSING member, so pin
# the five core sub-modules. A rename or removal of any of them must
# consciously update this list instead of silently narrowing the gate to
# the surviving modules (which would still scan and exit 0).
canary_modules=(
	internal/task
	internal/journal
	internal/queue
	internal/executor
	internal/worker
)
missing_canaries=""
for canary in "${canary_modules[@]}"; do
	if ! grep -qxF -- "$canary" <<<"$modules"; then
		missing_canaries+=" $canary"
	fi
done
if [ -n "$missing_canaries" ]; then
	echo "FAIL: module enumeration is missing canary module(s):$missing_canaries — a short enumeration silently narrows the gate to the survivors (renamed or removed sub-module? update the canary list in $self_abs consciously; enumerator: $enum_cmd)"
	exit 1
fi
echo "== (root) ./..."
if scan "$GOSEC_BIN" ./...; then
	scans_ok=$((scans_ok + 1))
else
	scans_failed=$((scans_failed + 1))
	fail=1
fi
while IFS= read -r m; do
	echo "== $m"
	if (cd "$m" && GOWORK=off scan "$GOSEC_BIN" ./...); then
		scans_ok=$((scans_ok + 1))
	else
		scans_failed=$((scans_failed + 1))
		fail=1
	fi
done <<<"$modules"

echo "gosec summary: $scans_ok scans ok, $scans_failed failed"
exit "$fail"
