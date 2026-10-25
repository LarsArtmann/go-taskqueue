#!/usr/bin/env bash
# Toolchain pin alignment gate (2026-10-25 TODO: the 1.27.3 sweep needed
# four hand-synced sources; this gate makes the next CVE bump one edit +
# one red gate). Checks, over the whole repo:
#   1. every go.mod `go` directive carries the SAME floor (no drift);
#   2. every setup-go `go-version:` pin in .github/workflows/*.yml equals
#      that floor (CI runners run GOTOOLCHAIN=local, so a pin below the
#      floor dies in every go step — the 2026-09-20/21 master red);
#   3. the flake's goToolchainVersion is >= the floor (the tarball-built
#     toolchain may run ahead, never behind).
#   4. every tracked go.work `go` directive equals the floor (the
#     2026-10-24 sweep missed cmd/tq/go.work at 1.27.2; gopls died on it).
# Called from scripts/ci-local.sh AND .github/workflows/ci.yml — keep
# both call sites in sync.
set -euo pipefail
cd "$(dirname "$0")/.."
self_abs="$(cd "$(dirname "$0")" && pwd)/$(basename "$0")"

# Env hooks (self-test injection, the check-go-mods TQ_GOMODS_EXTRA_MODS
# pattern): TQ_TOOLCHAIN_EXTRA_MODS (colon-separated extra go.mod files),
# TQ_TOOLCHAIN_WORKFLOWS_DIR (replaces .github/workflows), and
# TQ_TOOLCHAIN_FLAKE (replaces flake.nix), and
# TQ_TOOLCHAIN_EXTRA_GOWORKS (colon-separated extra go.work files).
extra_mods="${TQ_TOOLCHAIN_EXTRA_MODS:-}"
workflows_dir="${TQ_TOOLCHAIN_WORKFLOWS_DIR:-.github/workflows}"
flake_file="${TQ_TOOLCHAIN_FLAKE:-flake.nix}"

# --print-floor: emit ONLY the shared go.mod floor — the SAME source the
# checks below validate the setup-go pins against — so callers can pin
# GOTOOLCHAIN from one place (ci-local exports GOTOOLCHAIN=go<floor> so
# local govulncheck sees the advisories the pinned CI runner sees).
if [[ "${1:-}" == "--print-floor" ]]; then
	floors_out=""
	while IFS= read -r mod; do
		f="$(sed -n 's/^go \([0-9][0-9.]*\)$/\1/p' "$mod" | head -1)"
		if [ -z "$f" ]; then
			echo "check-toolchain-pins: $mod: no parsable 'go <version>' directive" >&2
			exit 1
		fi
		floors_out+="$f"$'\n'
	done < <(git ls-files '*go.mod' | sort)
	if [ -n "$extra_mods" ]; then
		while IFS= read -r mod; do
			f="$(sed -n 's/^go \([0-9][0-9.]*\)$/\1/p' "$mod" | head -1)"
			if [ -z "$f" ]; then
				echo "check-toolchain-pins: $mod: no parsable 'go <version>' directive" >&2
				exit 1
			fi
			floors_out+="$f"$'\n'
		done < <(tr ':' '\n' <<<"$extra_mods")
	fi
	sort -Vu <<<"$floors_out" | tail -1
	exit 0
fi

ok_count=0
fail_count=0

pass() {
	ok_count=$((ok_count + 1))
	echo "ok: $1"
}

fail() {
	fail_count=$((fail_count + 1))
	echo "FAIL: $1"
}

# version_ge a b — true when semver a >= b (three numeric components).
version_ge() {
	local a="$1" b="$2" i
	local -a pa pb
	IFS=. read -r pa0 pa1 pa2 <<<"$a"
	IFS=. read -r pb0 pb1 pb2 <<<"$b"
	pa=("${pa0:-0}" "${pa1:-0}" "${pa2:-0}")
	pb=("${pb0:-0}" "${pb1:-0}" "${pb2:-0}")
	for i in 0 1 2; do
		if [ "${pa[$i]}" -gt "${pb[$i]}" ]; then
			return 0
		fi
		if [ "${pa[$i]}" -lt "${pb[$i]}" ]; then
			return 1
		fi
	done
	return 0
}

mapfile -t mods < <(git ls-files '*go.mod' | sort)
if [ -n "$extra_mods" ]; then
	while IFS= read -r m; do
		mods+=("$m")
	done < <(tr ':' '\n' <<<"$extra_mods")
fi
[ "${#mods[@]}" -gt 0 ] || {
	fail "no go.mod files found"
	exit 1
}

declare -A floors
floor_list=""
for mod in "${mods[@]}"; do
	f="$(sed -n 's/^go \([0-9][0-9.]*\)$/\1/p' "$mod" | head -1)"
	if [ -z "$f" ]; then
		fail "$mod: no parsable 'go <version>' directive"
		continue
	fi
	floors["$f"]=1
	floor_list+=" $f"
done
if [ "${#floors[@]}" -eq 1 ]; then
	pass "all ${#mods[@]} go.mod files share one toolchain floor (${floor_list# })"
else
	fail "go.mod floors diverge:${floor_list} — every module must carry the SAME floor"
fi

floor="$(printf '%s\n' "${!floors[@]}" | sort -V | tail -1)"

wf_total=0
for wf in "$workflows_dir"/*.yml; do
	[ -f "$wf" ] || continue
	wf_total=$((wf_total + 1))
	while IFS= read -r pin; do
		wf_total=$((wf_total + 1))
		if [ "$pin" = "$floor" ]; then
			pass "setup-go pin $pin == go.mod floor ($wf)"
		else
			fail "setup-go pin $pin != go.mod floor $floor ($wf) — runners are GOTOOLCHAIN=local; a stale pin dies in every go step"
		fi
	done < <(sed -n 's/^[[:space:]]*go-version:[[:space:]]*\([0-9][0-9.]*\)[[:space:]]*$/\1/p' "$wf")
done
if [ "$wf_total" -eq 0 ]; then
	fail "no workflow files or go-version pins found under $workflows_dir"
fi

flake_version="$(sed -n 's/^[[:space:]]*goToolchainVersion = "\([0-9][0-9.]*\)";$/\1/p' "$flake_file" | head -1)"
if [ -z "$flake_version" ]; then
	fail "$flake_file: no parsable goToolchainVersion"
elif version_ge "$flake_version" "$floor"; then
	pass "flake goToolchainVersion $flake_version >= go.mod floor $floor"
else
	fail "flake goToolchainVersion $flake_version < go.mod floor $floor — the nix toolchain must never run behind the modules"
fi

mapfile -t goworks < <(git ls-files '*go.work' | sort)
if [ -n "${TQ_TOOLCHAIN_EXTRA_GOWORKS:-}" ]; then
	while IFS= read -r w; do
		goworks+=("$w")
	done < <(tr ':' '\n' <<<"${TQ_TOOLCHAIN_EXTRA_GOWORKS}")
fi
gw_total=0
for gw in "${goworks[@]}"; do
	gw_total=$((gw_total + 1))
	g="$(sed -n 's/^go \([0-9][0-9.]*\)$/\1/p' "$gw" | head -1)"
	if [ -z "$g" ]; then
		fail "$gw: no parsable 'go <version>' directive"
	elif [ "$g" = "$floor" ]; then
		pass "go.work go $g == go.mod floor ($gw)"
	else
		fail "go.work go $g != go.mod floor $floor ($gw) — the tracked dev workspace must carry the shared floor (gopls package loads die below it)"
	fi
done
if [ "$gw_total" -eq 0 ]; then
	pass "no tracked go.work files (dev-workspace surface absent)"
fi

echo "toolchain-pins summary: $ok_count checks ok, $fail_count failed"
[ "$fail_count" -eq 0 ]

# --self-test (fleet convention: countable gate summaries are PINNED, not
# advisory — 01-12 §e7/§g2): pins the summary line format and runs a
# negative-path battery over the divergence branches via /tmp fixtures
# injected through the env hooks.
if [[ "${1:-}" == "--self-test" ]]; then
	self_test() {
		local out rc
		# tmp is deliberately global: the EXIT trap reads it after self_test
		# returned, and set -u would kill the trap on a function-local name.
		tmp="$(mktemp -d)"
		trap 'rm -rf "$tmp"' EXIT

		local fxd="$tmp/fixmod"
		mkdir -p "$fxd" "$tmp/wf"
		printf 'module fixmod\n\ngo 1.20\n' >"$fxd/go.mod"
		printf 'jobs:\n  x:\n    steps:\n      - uses: setup-go\n        with:\n          go-version: 1.19.9\n' >"$tmp/wf/bad.yml"

		# Branch 1: go.mod floor divergence (extra fixture module below floor).
		out="$(TQ_TOOLCHAIN_EXTRA_MODS="$fxd/go.mod" "$self_abs" 2>&1)" && rc=0 || rc=$?
		[ "$rc" -eq 1 ] || {
			echo "FAIL: self-test floor-divergence branch (want rc=1, got rc=$rc)"
			printf '%s\n' "$out"
			exit 1
		}
		grep -q 'go.mod floors diverge' <<<"$out" || {
			echo "FAIL: self-test floor-divergence message missing"
			printf '%s\n' "$out"
			exit 1
		}

		# Branch 2: workflow pin below the floor (isolated fixture workflows +
		# fixture flake so the real repo's pins cannot mask the branch).
		printf 'toolchain:\n  goToolchainVersion = "9.9.9";\n' >"$tmp/fake-flake"
		out="$(TQ_TOOLCHAIN_EXTRA_MODS="$fxd/go.mod" TQ_TOOLCHAIN_WORKFLOWS_DIR="$tmp/wf" TQ_TOOLCHAIN_FLAKE="$tmp/fake-flake" "$self_abs" 2>&1)" && rc=0 || rc=$?
		[ "$rc" -eq 1 ] || {
			echo "FAIL: self-test workflow-pin branch (want rc=1, got rc=$rc)"
			printf '%s\n' "$out"
			exit 1
		}
		grep -q 'setup-go pin 1.19.9 != go.mod floor' <<<"$out" || {
			echo "FAIL: self-test workflow-pin message missing"
			printf '%s\n' "$out"
			exit 1
		}

		# Branch 3: flake toolchain behind the floor.
		printf 'x:\n  goToolchainVersion = "1.0.0";\n' >"$tmp/fake-flake"
		out="$(TQ_TOOLCHAIN_FLAKE="$tmp/fake-flake" "$self_abs" 2>&1)" && rc=0 || rc=$?
		[ "$rc" -eq 1 ] || {
			echo "FAIL: self-test flake-behind branch (want rc=1, got rc=$rc)"
			printf '%s\n' "$out"
			exit 1
		}
		grep -q 'goToolchainVersion 1.0.0 < go.mod floor' <<<"$out" || {
			echo "FAIL: self-test flake-behind message missing"
			printf '%s\n' "$out"
			exit 1
		}

		# Branch 4: go.work below the floor (the 2026-10-25 cmd/tq miss
		# class) via the extra-goworks hook.
		printf 'go 1.19\n\nuse .\n' >"$tmp/bad-go.work"
		out="$(TQ_TOOLCHAIN_EXTRA_GOWORKS="$tmp/bad-go.work" "$self_abs" 2>&1)" && rc=0 || rc=$?
		[ "$rc" -eq 1 ] || {
			echo "FAIL: self-test go.work-floor branch (want rc=1, got rc=$rc)"
			printf '%s\n' "$out"
			exit 1
		}
		grep -q 'go.work go 1.19 != go.mod floor' <<<"$out" || {
			echo "FAIL: self-test go.work-floor message missing"
			printf '%s\n' "$out"
			exit 1
		}

		# Positive: the unpinned real run passes and its summary line matches
		# the pinned format.
		out="$("$self_abs" 2>&1)" && rc=0 || rc=$?
		[ "$rc" -eq 0 ] || {
			echo "FAIL: self-test real run red"
			printf '%s\n' "$out"
			exit 1
		}
		grep -Eq '^toolchain-pins summary: [0-9]+ checks ok, 0 failed$' <<<"$out" || {
			echo "FAIL: self-test real-run summary missing or malformed"
			printf '%s\n' "$out"
			exit 1
		}
		echo "ok: self-test battery (4 negative branches + pinned real-run summary)"
	}
	self_test
fi
