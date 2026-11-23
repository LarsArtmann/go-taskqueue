#!/usr/bin/env bash
# ADR-0016 facade parity gate: every exported declaration of each facaded
# internal package must carry a kind-compatible re-export in its public
# facade (and facades must not dangle). The rules live in
# scripts/facadeparity/main.go (go/parser, stdlib-only). Called from
# scripts/ci-local.sh AND .github/workflows/ci.yml — keep both call sites
# in sync. Pairs are pinned in the checker; adding a facade means
# adding its pair there in the same change (the queue/sqlite + queue/postgres
# facade pairs were retired 2026-11-23 — the drivers stay internal-path).
#
# Flags:
#   --self-test  fleet-convention rail (countable gate summaries are
#                PINNED, not advisory): pins the real-run summary format
#                and runs a negative-path battery (MISSING-ALIAS /
#                KIND-MISMATCH / DANGLING-ALIAS / clean) over a scratch
#                five-pair fixture root. The checker's own go test pins
#                the DETECTOR; this pins the GATE — exit codes and the
#                finding-line shapes a closeout receipt greps.
set -euo pipefail
cd "$(dirname "$0")/.."

self_abs="$(cd "$(dirname "$0")" && pwd)/$(basename "$0")"

if [ "${1:-}" = "--self-test" ]; then
	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' EXIT
	okc=0
	failc=0

	# check runs the checker against a fixture root and asserts rc + a
	# needle in the combined output.
	check() {
		local desc="$1" want_rc="$2" needle="$3" fixture="$4" rc out
		rc=0
		out="$(go run ./scripts/facadeparity "$fixture" 2>&1)" || rc=$?
		if [ "$rc" -ne "$want_rc" ]; then
			echo "FAIL: $desc (want rc=$want_rc, got rc=$rc)"
			printf '%s\n' "$out" | sed 's/^/    /'
			failc=$((failc + 1))
			return
		fi
		if [ -n "$needle" ] && ! grep -qF -- "$needle" <<<"$out"; then
			echo "FAIL: $desc (missing: $needle)"
			printf '%s\n' "$out" | sed 's/^/    /'
			failc=$((failc + 1))
			return
		fi
		echo "ok: $desc (rc=$rc)"
		okc=$((okc + 1))
	}

	# Fixture builder: a full five-pair root (the checker's pair list is
	# pinned, so the battery needs every pair present or run() errors on
	# a missing dir before any skew is reported). emit_pair <root> <pkg>
	# <facade-body-file> writes the internal side plus the given facade
	# side; fixture bodies use space indentation (go/parser does not
	# care, and tabs in shell heredocs are the glue hazard).
	internal_body() {
		printf 'package %s\n\ntype Status int\n\nfunc Valid() bool { return true }\n\nconst MaxAttempts = 3\n\nvar Sentinel = "x"\n' "$1"
	}
	emit_pair() {
		local root="$1" pkg="$2" facade_file="$3"
		mkdir -p "$root/internal/$pkg" "$root/$pkg"
		internal_body "$pkg" >"$root/internal/$pkg/$pkg.go"
		cat "$facade_file" >"$root/$pkg/$pkg.go"
	}
	facade_ok() {
		printf 'package %s\n\ntype Status = int\n\nvar Valid = func() bool { return true }\n\nconst MaxAttempts = 3\n\nvar Sentinel = "x"\n' "$1"
	}

	std_facade="$tmp/facade-standard.go.txt"
	facade_ok std >"$std_facade"

	root="$tmp/clean"
	for pkg in task journal queue executor worker; do
		emit_pair "$root" "$pkg" "$std_facade"
	done

	# 1) Clean fixture passes with the pinned summary line.
	check "clean fixture passes" 0 "facade parity OK: 5 facades mirror their internal packages" "$root"

	# 2) MISSING-ALIAS: the facade drops Status.
	root="$tmp/missing"
	for pkg in task journal queue executor worker; do
		emit_pair "$root" "$pkg" "$std_facade"
	done
	printf 'package task\n\nvar Valid = func() bool { return true }\n\nconst MaxAttempts = 3\n\nvar Sentinel = "x"\n' >"$root/task/task.go"
	check "missing alias fails naming the decl" 1 "MISSING-ALIAS task: internal/task.Status" "$root"

	# 3) KIND-MISMATCH: the facade re-exports the type as a var.
	root="$tmp/kindmismatch"
	for pkg in task journal queue executor worker; do
		emit_pair "$root" "$pkg" "$std_facade"
	done
	printf 'package task\n\nvar Status int\n\nvar Valid = func() bool { return true }\n\nconst MaxAttempts = 3\n\nvar Sentinel = "x"\n' >"$root/task/task.go"
	check "kind mismatch fails naming both kinds" 1 "KIND-MISMATCH" "$root"

	# 4) DANGLING-ALIAS: the facade carries a decl with no counterpart.
	root="$tmp/dangling"
	for pkg in task journal queue executor worker; do
		emit_pair "$root" "$pkg" "$std_facade"
	done
	printf 'package task\n\ntype Status = int\n\ntype Ghost = int\n\nvar Valid = func() bool { return true }\n\nconst MaxAttempts = 3\n\nvar Sentinel = "x"\n' >"$root/task/task.go"
	check "dangling alias fails naming the facade" 1 "DANGLING-ALIAS task: Ghost" "$root"

	# 5) REAL gate run: the shipped summary format is pinned by regex
	#    (the facade count drifts with the pair list; the FORMAT is the
	#    contract a receipt greps).
	real_out="$("$self_abs" 2>&1)" && rc=0 || rc=$?
	if [ "$rc" -ne 0 ]; then
		echo "FAIL: real facade-parity gate must be green at HEAD (got rc=$rc)"
		printf '%s\n' "$real_out"
		failc=$((failc + 1))
	else
		if grep -Eq '^facade parity OK: [0-9]+ facades? mirror their internal packages$' <<<"$real_out"; then
			echo "ok: real-run summary matches the pinned format (rc=0)"
			okc=$((okc + 1))
		else
			echo "FAIL: real-run summary missing or malformed"
			printf '%s\n' "$real_out"
			failc=$((failc + 1))
		fi
	fi

	echo "facade-parity self-test ok ($okc ok / $failc fail) — real-run format pin + fixture skew battery"
	[ "$failc" -eq 0 ] || exit 1
	exit 0
fi

go run ./scripts/facadeparity .
