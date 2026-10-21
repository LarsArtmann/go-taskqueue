#!/usr/bin/env bash
# Fixture-attribution meta-gate (TODO row: 2026-09-20 01-36 §e1/f2, third
# strike on the mis-aimed-negative-fixture class): every fixture heredoc
# in a scripts/check-*.sh gate must carry a grep-able `pins:` attribution
# naming WHICH branch the fixture pins — on the `cat >` line, inside the
# heredoc block, or in the comment lines immediately above it (up to two,
# contiguous). Without the
# attribution a negative fixture can drift aim (edit the fixture, keep
# the rc!=0 assertion) and the gate stays green while pinning nothing.
#
# The marker is documentation, not enforcement of ACCURACY — accuracy is
# the author's job; the gate enforces that the attribution EXISTS and is
# mechanically findable next to the fixture it describes.
#
# Contract: `cat > ... <<TAG` blocks (all quote shapes, <<- included)
# inside scripts/check-*.sh must contain `pins:` in the block, on the
# cat line, or in the contiguous comment lines immediately above. This script's own file is exempt
# (its --self-test fixtures carry the marker, proving the rule bites).
set -uo pipefail
cd "$(dirname "$0")/.." || exit 1
self="$(basename "$0")"


check_file() {
	local file=$1
	# awk state machine: collect heredoc spans, report unattributed ones.
	awk -v file="$file" '
		{
			lines[NR] = $0
		}
		END {
			v = 0
			b = 0
			intag = 0
			tag = ""
			start = 0
			found = 0
			for (i = 1; i <= NR; i++) {
				line = lines[i]
				if (!intag) {
					if (line ~ /^[[:space:]]*(cat|tee)[[:space:]].*<<-?[ ]*[\x27"]?[A-Za-z_][A-Za-z0-9_]*[\x27"]?([[:space:]]|$)/) {
						# heredoc start: extract the tag (last simple word
						# after <<)
						rest = line
						sub(/.*<</, "", rest)
						sub(/^-/, "", rest)
						gsub(/[\x27"]/, "", rest)
						split(rest, w, /[[:space:]]|>/)
						tag = w[1]
						if (tag != "") {
							intag = 1
							start = i
							found = 0
							if (line ~ /pins:/) found = 1
							if (i > 1 && lines[i - 1] ~ /pins:/) found = 1
							if (i > 2 && lines[i - 2] ~ /pins:/ && lines[i - 1] ~ /^[[:space:]]*#/) found = 1
						}
					}
				} else {
					if (line ~ /pins:/) found = 1
					# end of heredoc: the tag alone on its line (allow -)
					ender = line
					sub(/^-/, "", ender)
					gsub(/[[:space:]]/, "", ender)
					if (ender == tag) {
						intag = 0
						b++
						if (!found) {
							printf "UNATTRIBUTED FIXTURE: %s:%d heredoc <<%s has no `pins:` branch attribution (cat line, block body, or the line above)\n", file, start, tag
							v++
						}
					}
				}
			}
			printf "%d %d\n", v, b
		}
	' "$file"
}

total_blocks=0
total_viol=0
for f in scripts/check-*.sh; do
	[ "$(basename "$f")" = "$self" ] && continue
	out="$(check_file "$f")"
	v="$(printf '%s\n' "$out" | tail -1 | cut -d' ' -f1)"
	b="$(printf '%s\n' "$out" | tail -1 | cut -d' ' -f2)"
	total_blocks=$((total_blocks + b))
	total_viol=$((total_viol + v))
	if [ "$v" -gt 0 ]; then
		printf '%s\n' "$out" | head -n -1
	fi
done

if [ "${1:-}" = "--self-test" ]; then
	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' EXIT
	good="$tmp/good.sh"
	bad="$tmp/bad.sh"
	cat >"$good" <<'SCRIPT'
#!/usr/bin/env bash
# pins: ok-branch — fixture that satisfies the gate (block attribution)
cat >"$tmp/f1" <<'EOF'
# pins: negative-branch — rc!=0 must come from THIS branch
echo boom
EOF
cat >"$tmp/f2" <<EOF
inline pins: stub-branch
EOF
# pins: above-line attribution — pinned-branch
# pins: two-above attribution — multi-line-branch
# (comment block directly above the fixture write)
cat >"$tmp/f3" <<'F3'
data
F3
SCRIPT
	cat >"$bad" <<'SCRIPT'
#!/usr/bin/env bash
cat >"$tmp/g1" <<'EOF'
echo no attribution here
EOF
SCRIPT
	out="$(check_file "$good")"
	gv="$(printf '%s\n' "$out" | tail -1 | cut -d' ' -f1)"
	[ "$gv" = 0 ] || { echo "FAIL: self-test good fixture reported $gv violations (expected 0)"; exit 1; }
	echo "ok: self-test good fixture passes (4 attribution shapes)"
	out="$(check_file "$bad")"
	bv="$(printf '%s\n' "$out" | tail -1 | cut -d' ' -f1)"
	if [ "$bv" != 1 ]; then
		echo "FAIL: self-test bad fixture reported '$bv' violations (expected exactly 1)"
		exit 1
	fi
	if ! printf '%s\n' "$out" | grep -q 'UNATTRIBUTED FIXTURE'; then
		echo "FAIL: self-test bad fixture did not name the violation"
		exit 1
	fi
	echo "ok: self-test bad fixture trips exactly one UNATTRIBUTED FIXTURE"
	echo "fixture-attribution self-test ok (block/inline/above-line/two-line-above attribution pinned, missing-marker negative pinned)"
	exit 0
fi

if [ "$total_viol" -gt 0 ]; then
	echo "FAIL: $total_viol unattributed fixture heredoc(s) across scripts/check-*.sh ($total_blocks blocks scanned)"
	echo "  fix: add a grep-able \`pins: <branch>\` attribution naming which branch the fixture pins"
	exit 1
fi
echo "fixture-attribution ok: $total_blocks fixture heredoc(s) across scripts/check-*.sh all carry pins: attribution"
