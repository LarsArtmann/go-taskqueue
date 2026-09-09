#!/usr/bin/env bash
# FEATURES <-> ROADMAP contradiction guard (round-10 T25): a bundle seed
# (D8x/D9x/D10x) claimed SHIPPED in FEATURES.md must not also live in
# ROADMAP.md — the exact 02:52 defect (D83 "shipped" in FEATURES while
# ROADMAP listed cron recurring as future). Scoped to seed IDs because
# free-text feature matching is too fuzzy to gate on.
set -uo pipefail
cd "$(dirname "$0")/.."

features="FEATURES.md"
roadmap="ROADMAP.md"
[ -f "$features" ] && [ -f "$roadmap" ] || {
	echo "MISSING: $features or $roadmap"
	exit 1
}

fail=0
# Seed IDs on FEATURES lines that claim shipped status.
shipped_ids="$(
	grep -iE 'D(8[0-9]|9[0-9]|10[0-9])' "$features" |
		grep -iE 'shipped|🟢|FULLY_FUNCTIONAL' |
		grep -oE 'D(8[0-9]|9[0-9]|10[0-9])' |
		sort -u
)"

if [ -n "$shipped_ids" ]; then
	for id in $shipped_ids; do
		if grep -qE "\b$id\b" "$roadmap"; then
			echo "CONTRADICTION: seed $id is shipped in $features but still listed in $roadmap"
			fail=1
		fi
	done
fi

if [ "$fail" = 0 ]; then
	echo "FEATURES/ROADMAP seed cross-check ok"
fi
exit "$fail"
