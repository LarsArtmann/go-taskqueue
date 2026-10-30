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
source scripts/lib/expect-branch.sh

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

# expect_fail: fixture-attribution v2 accuracy layer — rc!=0 alone
# false-greens when the fixture no-ops or aims at the wrong branch, so each
# negative case must ALSO print its branch canary (a distinctive substring
# of that branch's FAIL line) before the harness accepts it. Subshell
# failures propagate via rc so `fails` counts outside the subshell too.
expect_fail() {
	local token=$1 label=$2 gomod=$3
	expect_branch_fail "$token" "$label" gate_gomod "$gomod"
}

# The real tree must pass against the real tags.
expect_pass go.mod "real go.mod (incl. nested queue/sqlite replace + require)"

# Fixture repo: the require-tag existence check resolves tags here.
fixture="$(mktemp -d)"
trap 'rm -rf "$fixture"' EXIT
git -C "$fixture" init -q
git -C "$fixture" -c user.email=t@t -c user.name=t commit -q --allow-empty -m init

# Self-test (03-46 §f3): with the -c user.* flags stripped and the ci-local
# sanitizer active (GIT_CONFIG_GLOBAL=/dev/null + user.useConfigOnly), the
# commit must fail with "Author identity unknown" — the fixture's identity
# comes ONLY from its flags, so dropping one must break the smoke.
stripped_out="$(mktemp)"
stripped_rc=0
env -u GIT_AUTHOR_NAME -u GIT_AUTHOR_EMAIL -u GIT_COMMITTER_NAME -u GIT_COMMITTER_EMAIL \
	GIT_CONFIG_GLOBAL=/dev/null \
	GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=user.useConfigOnly GIT_CONFIG_VALUE_0=true \
	git -C "$fixture" commit -q --allow-empty -m identity-stripped >"$stripped_out" 2>&1 || stripped_rc=$?
if [ "$stripped_rc" -eq 0 ] || ! grep -q "identity unknown" "$stripped_out"; then
	echo "FAIL fixture: commit without -c user.* flags did not fail with identity unknown"
	fails=$((fails + 1))
else
	echo "ok   fixture: stripped -c user.* commit fails (identity unknown)"
fi
rm -f "$stripped_out"
for tag in internal/task/v0.2.0 internal/queue/sqlite/v0.2.0; do
	# Annotated tags need a committer identity, and CI runners have none —
	# the commit above carries -c flags for the same reason.
	git -C "$fixture" -c user.email=t@t -c user.name=t tag -a "$tag" -m fixture
done

# pins: canonical-root-gomod — tagged root go.mod must pass
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
	cd "$fixture" && expect_pass go.mod "fixture: nested sibling replaces + tagged requires"
) || fails=$((fails + 1))

sed 's|=> ./internal/task|=> /home/someone/internal/task|' "$fixture/go.mod" >"$fixture/abs.go.mod"
(
	cd "$fixture" && expect_fail "non-repo-relative replace directives" "fixture: absolute replace path is poison" abs.go.mod
) || fails=$((fails + 1))

sed 's|internal/task v0.2.0|internal/task v9.9.9|' "$fixture/go.mod" >"$fixture/untagged.go.mod"
(
	cd "$fixture" && expect_fail "does not exist" "fixture: require without a cut sub-tag" untagged.go.mod
) || fails=$((fails + 1))

sed 's|internal/queue/sqlite v0.2.0|internal/queue/sqlite v0.0.0-00010101000000-000000000000|' "$fixture/go.mod" >"$fixture/pseudo.go.mod"
(
	cd "$fixture" && expect_fail "pseudo-version" "fixture: pseudo-version require (replace leak)" pseudo.go.mod
) || fails=$((fails + 1))

# pins: reject-cross-tree-replace — stranger replace pointing at ../go-taskqueue must fail
cat >"$fixture/stranger.go.mod" <<'EOF'
module example.com/other

replace github.com/larsartmann/go-taskqueue/internal/task => ../go-taskqueue/internal/task
EOF
(
	cd "$fixture" && expect_fail "non-repo-relative replace directives" "fixture: parent-relative replace is poison" stranger.go.mod
) || fails=$((fails + 1))

# Facade shape (ADR-0016): a top-level module (task/) whose internal require
# resolves through the proxy exactly like the root's, and whose replace
# points UP into the tree (../internal/…) — repo-contained, must pass.
mkdir -p "$fixture/task"
# pins: accept-facade-module — facade go.mod with UP replace must pass
cat >"$fixture/task/go.mod" <<'EOF'
module github.com/larsartmann/go-taskqueue/task

go 1.26.7

require github.com/larsartmann/go-taskqueue/internal/task v0.2.0

replace github.com/larsartmann/go-taskqueue/internal/task => ../internal/task
EOF
(
	cd "$fixture" && expect_pass task/go.mod "fixture: facade go.mod (../internal replace + tagged require)"
) || fails=$((fails + 1))

# Facade poison: an untagged internal pin would publish a facade that
# cannot resolve on the proxy (the T0 flaw class).
sed 's|internal/task v0.2.0|internal/task v9.9.9|' "$fixture/task/go.mod" >"$fixture/task/untagged.go.mod"
(
	cd "$fixture" && expect_fail "does not exist" "fixture: facade go.mod with untagged internal pin" task/untagged.go.mod
) || fails=$((fails + 1))

# Facade poison: a replace climbing above the repo root is the stranger
# shape even from a facade dir.
sed 's|=> ../internal/task|=> ../../outside/task|' "$fixture/task/go.mod" >"$fixture/task/escape.go.mod"
(
	cd "$fixture" && expect_fail "non-repo-relative replace directives" "fixture: facade go.mod replace escaping the repo root" task/escape.go.mod
) || fails=$((fails + 1))

# CLI module shape (ADR-0017): cmd/tq is its own REPLACE-FREE module whose
# requires pin the root module AND internal sub-modules at tagged versions —
# the widened require-tag check must accept it and reject a poisoned pin.
git -C "$fixture" -c user.email=t@t -c user.name=t tag -a v0.2.0 -m fixture
mkdir -p "$fixture/cmd/tq"
# pins: accept-cmd-tq-module — replace-free cmd/tq go.mod with tagged requires must pass
cat >"$fixture/cmd/tq/go.mod" <<'GOMOD'
module github.com/larsartmann/go-taskqueue/cmd/tq

go 1.26.7

require (
	github.com/larsartmann/go-taskqueue v0.2.0
	github.com/larsartmann/go-taskqueue/internal/task v0.2.0
)
GOMOD
(
	cd "$fixture" && expect_pass cmd/tq/go.mod "fixture: CLI module go.mod (replace-free, root + internal tagged requires)"
) || fails=$((fails + 1))
sed 's|go-taskqueue v0.2.0|go-taskqueue v9.9.9|' "$fixture/cmd/tq/go.mod" >"$fixture/cmd/tq/untagged.go.mod"
(
	cd "$fixture" && expect_fail "does not exist" "fixture: CLI module with untagged root-module pin" cmd/tq/untagged.go.mod
) || fails=$((fails + 1))

if [ "$fails" -gt 0 ]; then
	echo "$fails release-gate case(s) failed"
	exit 1
fi
echo "release-gates smoke: all cases green"
