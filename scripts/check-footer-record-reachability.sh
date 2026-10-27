#!/usr/bin/env bash
# Footer-record reachability pin (dispatch 000001a15bcff9bb §e2 / CV row
# "footer-record ancestry/reachability pin"): every commit SHA cited by a
# dispatch attribution record MUST stay reachable from master — the
# b1706caf reset-away proved any later `git reset` can orphan an
# attributable record to reflog-only, and reflog expiry then GCs it.
#
# docs/release/protected-records.txt is the roster: one `sha # reason`
# (or bare sha) per line; `#` starts a comment. Maintenance rule: when a
# dispatch record (CV-side attribution record or task report) cites a
# go-taskqueue SHA, add it here in the same change.
#
# Exit 0 = all roster SHAs exist and are ancestors of HEAD.
# Exit 1 = orphaned/GC'd entries (named, with the recovery order).
# Exit 2 = usage/corrupt roster.
set -euo pipefail
cd "$(dirname "$0")/.."

roster="docs/release/protected-records.txt"

if [ ! -f "$roster" ]; then
	echo "footer-record-reachability: roster missing: $roster" >&2
	exit 2
fi

orphans=""
missing=""
while IFS= read -r line; do
	line="${line%%#*}"
	sha="$(echo "$line" | tr -d '[:space:]')"
	[ -n "$sha" ] || continue
	if ! git cat-file -e "${sha}^{commit}" 2>/dev/null; then
		missing="$missing $sha"
		continue
	fi
	if ! git merge-base --is-ancestor "$sha" HEAD 2>/dev/null; then
		orphans="$orphans $sha"
	fi
done <"$roster"

rc=0
if [ -n "$missing" ]; then
	echo "footer-record-reachability: roster commits MISSING from the object store (GC'd — restore from reflog/backup NOW):$missing" >&2
	rc=1
fi
if [ -n "$orphans" ]; then
	echo "footer-record-reachability: roster commits NOT reachable from HEAD (reset-away class — cherry-pick back or land a reachable citation commit):$orphans" >&2
	rc=1
fi
[ "$rc" -eq 0 ] && echo "footer-record-reachability: $(grep -cEv '^[[:space:]]*(#|$)' "$roster") protected records reachable from HEAD"
exit "$rc"
