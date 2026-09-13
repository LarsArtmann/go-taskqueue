#!/usr/bin/env bash
# Release-gate smoke: proves the go.mod release rules (scripts/lib/
# release-gates.sh) accept the REAL tree and reject each poison shape.
# Round-2 lesson encoded here: the allowlist regex silently stopped matching
# nested modules (internal/queue/sqlite) after the backend split — the gate
# would have false-positived the real tree while skipping the require check
# for the very module it failed to parse. Regex gates without fixtures rot.
set -euo pipefail
cd "$(dirname "$0")/../.."

source scripts/lib/release-gates.sh

fails=0
expect_pass() {
	if gate_gomod "$1" >/dev/null 2>&1; then
		echo "ok   $2"
	else
		echo "FAIL $2 (gate rejected a valid go.mod)"
		gate_gomod "$1" || true
		fails=$((fails + 1))
	fi
}

expect_fail() {
	if gate_gomod "$1" >/dev/null 2>&1; then
		echo "FAIL $2 (gate accepted a poisoned go.mod)"
		fails=$((fails + 1))
	else
		echo "ok   $2"
	fi
}

# The real tree must pass against the real tags.
expect_pass go.mod "real go.mod (incl. nested queue/sqlite replace + require)"

# Fixture repo: the require-tag existence check resolves tags here.
fixture="$(mktemp -d)"
trap 'rm -rf "$fixture"' EXIT
git -C "$fixture" init -q
git -C "$fixture" -c user.email=t@t -c user.name=t commit -q --allow-empty -m init
for tag in internal/task/v0.2.0 internal/queue/sqlite/v0.2.0; do
	# Annotated tags need a committer identity, and CI runners have none —
	# the commit above carries -c flags for the same reason.
	git -C "$fixture" -c user.email=t@t -c user.name=t tag -a "$tag" -m fixture
done

cat >"$fixture/go.mod" <<'EOF'
module github.com/larsartmann/go-taskqueue

go 1.26.7

require (
	github.com/larsartmann/go-taskqueue/internal/task v0.2.0
	github.com/larsartmann/go-taskqueue/internal/queue/sqlite v0.2.0
)

replace github.com/larsartmann/go-taskqueue/internal/task => ./internal/task

replace github.com/larsartmann/go-taskqueue/internal/queue/sqlite => ./internal/queue/sqlite
EOF
(
	cd "$fixture"
	expect_pass go.mod "fixture: nested sibling replaces + tagged requires"
)

sed 's|=> ./internal/task|=> /home/someone/internal/task|' "$fixture/go.mod" >"$fixture/abs.go.mod"
(
	cd "$fixture"
	expect_fail abs.go.mod "fixture: absolute replace path is poison"
)

sed 's|internal/task v0.2.0|internal/task v9.9.9|' "$fixture/go.mod" >"$fixture/untagged.go.mod"
(
	cd "$fixture"
	expect_fail untagged.go.mod "fixture: require without a cut sub-tag"
)

sed 's|internal/queue/sqlite v0.2.0|internal/queue/sqlite v0.0.0-00010101000000-000000000000|' "$fixture/go.mod" >"$fixture/pseudo.go.mod"
(
	cd "$fixture"
	expect_fail pseudo.go.mod "fixture: pseudo-version require (replace leak)"
)

cat >"$fixture/stranger.go.mod" <<'EOF'
module example.com/other

replace github.com/larsartmann/go-taskqueue/internal/task => ../go-taskqueue/internal/task
EOF
(
	cd "$fixture"
	expect_fail stranger.go.mod "fixture: parent-relative replace is poison"
)

# Facade shape (ADR-0016): a top-level module (task/) whose internal require
# resolves through the proxy exactly like the root's, and whose replace
# points UP into the tree (../internal/…) — repo-contained, must pass.
mkdir -p "$fixture/task"
cat >"$fixture/task/go.mod" <<'EOF'
module github.com/larsartmann/go-taskqueue/task

go 1.26.7

require github.com/larsartmann/go-taskqueue/internal/task v0.2.0

replace github.com/larsartmann/go-taskqueue/internal/task => ../internal/task
EOF
(
	cd "$fixture"
	expect_pass task/go.mod "fixture: facade go.mod (../internal replace + tagged require)"
)

# Facade poison: an untagged internal pin would publish a facade that
# cannot resolve on the proxy (the T0 flaw class).
sed 's|internal/task v0.2.0|internal/task v9.9.9|' "$fixture/task/go.mod" >"$fixture/task/untagged.go.mod"
(
	cd "$fixture"
	expect_fail task/untagged.go.mod "fixture: facade go.mod with untagged internal pin"
)

# Facade poison: a replace climbing above the repo root is the stranger
# shape even from a facade dir.
sed 's|=> ../internal/task|=> ../../outside/task|' "$fixture/task/go.mod" >"$fixture/task/escape.go.mod"
(
	cd "$fixture"
	expect_fail task/escape.go.mod "fixture: facade go.mod replace escaping the repo root"
)

if [ "$fails" -gt 0 ]; then
	echo "$fails release-gate case(s) failed"
	exit 1
fi
echo "release-gates smoke: all cases green"
