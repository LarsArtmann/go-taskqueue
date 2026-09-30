#!/usr/bin/env bash
# Anti-ghost-archive gate: every evidence file promised by a
# docs/status/assets/*/README.md must be git-tracked. The f9 dogfood
# archive nearly shipped README-only because the global `*.log` gitignore
# (buildflow-managed block) silently excluded the evidence from the
# auto-commit daemon's add-everything sweep (06-41 report §d1) — an
# archive whose promised files are not in git is lost at the next checkout.
#
# Each archive must also carry a git-tracked SHA256SUMS manifest covering
# its whole file set; the gate re-verifies the hashes so a manifest that
# drifted from the evidence fails instead of falsely certifying it
# (06-41 report §c2/§f2).
set -uo pipefail

# Test seam: GHOST_ROOT overrides the repo root the gate scans (the
# self-test's scratch fixture repo); unset in production.
root="$(cd "$(dirname "$0")/.." && pwd)"
[ -n "${GHOST_ROOT:-}" ] && root="$GHOST_ROOT"
cd "$root" || exit 1

# A promise is a bare filename ending in an evidence extension; anything
# else in backticks (versions, commit hashes, flags, prose) is not a file.
# Extend this list when an archive starts carrying a new format.
exts='log|txt|json|db|out|csv|png|jpg|jpeg|svg|gif|md|yaml|yml|toml|xml|html|pdf|tar\.gz|tgz|zip|webm|mp4'

[ -d docs/status/assets ] || {
	echo "no asset archives"
	exit 0
}

fail=0
while IFS= read -r readme; do
	dir="$(dirname "$readme")"

	# The README is part of the archive: an untracked one means the whole
	# directory is a lost tree the moment the working tree goes away.
	if ! git ls-files --error-unmatch "$readme" >/dev/null 2>&1; then
		echo "UNTRACKED: $readme (the archive index itself must be committed)"
		fail=1
	fi

	# Completeness + tamper-evidence: the manifest must exist, be tracked,
	# verify against the archived bytes, and list every file in the dir.
	# Independent ifs, not an elif chain (00-10 report §f1): an elif
	# short-circuits and the first applicable class masks the rest — an
	# untracked manifest used to hide a STALE/INCOMPLETE behind it.
	if [ ! -f "$dir/SHA256SUMS" ]; then
		echo "NO MANIFEST: $dir/SHA256SUMS is absent (every archive ships one)"
		fail=1
	fi
	if [ -f "$dir/SHA256SUMS" ] && ! git ls-files --error-unmatch "$dir/SHA256SUMS" >/dev/null 2>&1; then
		echo "UNTRACKED: $dir/SHA256SUMS (the manifest must be committed)"
		fail=1
	fi
	if [ -f "$dir/SHA256SUMS" ] && ! (cd "$dir" && sha256sum --check --quiet SHA256SUMS >/dev/null 2>&1); then
		echo "STALE: $dir/SHA256SUMS does not match the archived files (regenerate it)"
		fail=1
	fi
	if [ -f "$dir/SHA256SUMS" ] && ! diff <(cd "$dir" && for f in *; do [ "$f" = SHA256SUMS ] || printf '%s\n' "$f"; done) <(cd "$dir" && awk '{print $2}' SHA256SUMS) >/dev/null; then
		echo "INCOMPLETE: $dir/SHA256SUMS does not cover every file in $dir"
		fail=1
	fi

	while IFS= read -r token; do
		[ -n "$token" ] || continue
		# Not promises: paths outside the archive dir, placeholders
		# (`<id>.log`), globs (`*.log`) and dotfiles (`.tq-verify`).
		case "$token" in
		*/* | *\<* | *\>* | *\** | *\?* | *\[* | *\]* | .*) continue ;;
		esac
		promised="$dir/$token"
		if git ls-files --error-unmatch "$promised" >/dev/null 2>&1; then
			continue
		fi
		if [ -e "$promised" ]; then
			echo "GHOST: $promised exists on disk but is NOT git-tracked (a .gitignore rule is eating it — git add -f)"
		else
			echo "MISSING: $promised is promised by $readme but absent from the working tree and the index"
		fi
		fail=1
	done < <(grep -oE '`[^`]+`' "$readme" | tr -d '`' | sort -u | grep -E "\.($exts)\$")
done < <(find docs/status/assets -mindepth 2 -maxdepth 2 -name README.md | sort)

if [ "${1:-}" = "--self-test" ]; then
	# Negative-test fixtures (00-10 report §f1/§b1): a scratch repo proves
	# every failure class fires THROUGH the gate and that compound failures
	# are ALL reported — INCOMPLETE/STALE in particular survive the old
	# elif precedence (an untracked manifest used to mask them). Create +
	# assert + trash per fixture in ONE command, rc captured from a file
	# (never a pipeline).
	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' EXIT
	git -c user.email=t@t -c user.name=t init -q "$tmp"
	archive="$tmp/docs/status/assets/fix"

	stamp_manifest() { # $1=dir, then filename/hash pairs via args a=<hash>
		(
			cd "$1" || exit 1
			: >SHA256SUMS
			for pair in "${@:2}"; do
				printf '%s  %s\n' "${pair#*=}" "${pair%%=*}" >>SHA256SUMS
			done
		)
	}

	# Fixture A: STALE + INCOMPLETE in one manifest (a.log with a WRONG
	# hash, b.log uncovered) — both classes must surface.
	mkdir -p "$archive"
	printf 'evidence for the fixture archive\n' >"$archive/README.md"
	printf 'aaa\n' >"$archive/a.log"
	printf 'bbb\n' >"$archive/b.log"
	git -C "$tmp" add -f "$archive/README.md" "docs/status/assets/fix/a.log" "docs/status/assets/fix/b.log"
	git -C "$tmp" -c user.email=t@t -c user.name=t commit -qm t
	stamp_manifest "$archive" "a.log=deadbeef"
	out="$tmp/outA"
	GHOST_ROOT="$tmp" "$0" >"$out" 2>&1
	rc=$?
	[ "$rc" -ne 0 ] || {
		echo "self-test A: gate exited 0 on a bad manifest"
		exit 1
	}
	grep -q '^STALE:' "$out" || {
		echo "self-test A: STALE not reported"
		exit 1
	}
	grep -q '^INCOMPLETE:' "$out" || {
		echo "self-test A: INCOMPLETE masked (precedence bug is back)"
		exit 1
	}

	# Fixture B: untracked README + complete-but-untracked manifest missing
	# a file — UNTRACKED and INCOMPLETE must BOTH report.
	rm -rf "$archive"
	mkdir -p "$archive"
	printf 'evidence for the fixture archive\n' >"$archive/README.md"
	printf 'aaa\n' >"$archive/a.log"
	printf 'bbb\n' >"$archive/b.log"
	ha="$(sha256sum "$archive/a.log" | cut -d' ' -f1)"
	stamp_manifest "$archive" "a.log=$ha"
	out="$tmp/outB"
	GHOST_ROOT="$tmp" "$0" >"$out" 2>&1
	rc=$?
	[ "$rc" -ne 0 ] || {
		echo "self-test B: gate exited 0 on an untracked archive"
		exit 1
	}
	grep -q '^UNTRACKED:' "$out" || {
		echo "self-test B: UNTRACKED not reported"
		exit 1
	}
	grep -q '^INCOMPLETE:' "$out" || {
		echo "self-test B: INCOMPLETE masked by UNTRACKED (precedence bug is back)"
		exit 1
	}

	echo "ghost-archives self-test ok"
	exit 0
fi

if [ "$fail" = 0 ]; then
	echo "asset archives ok"
fi
exit "$fail"
