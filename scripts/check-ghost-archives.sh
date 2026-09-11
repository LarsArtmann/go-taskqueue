#!/usr/bin/env bash
# Anti-ghost-archive gate: every evidence file promised by a
# docs/status/assets/*/README.md must be git-tracked. The f9 dogfood
# archive nearly shipped README-only because the global `*.log` gitignore
# (buildflow-managed block) silently excluded the evidence from the
# auto-commit daemon's add-everything sweep (06-41 report §d1) — an
# archive whose promised files are not in git is lost at the next checkout.
set -uo pipefail
cd "$(dirname "$0")/.."

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

if [ "$fail" = 0 ]; then
	echo "asset archives ok"
fi
exit "$fail"
