#!/usr/bin/env bash
# Regression pin for scripts/new-module.sh (02-02 report §e4b): the
# scaffolder must keep .github/dependabot.yml at parity with the module
# tree at the SOURCE — the hand-maintained config drifted within one week
# of creation (three missing modules + one phantom, fixed 2026-10-06).
# Scratch-repo smoke: happy-path go.mod + appended entry byte-shape,
# append onto a config missing its trailing newline, existing-go.mod
# refusal, existing-entry refusal, missing-config refusal (the tree must
# stay untouched on every refusal).
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
TARGET="$SCRIPT_DIR/../new-module.sh"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

fail() {
	echo "FAIL: $*" >&2
	exit 1
}
pass() { echo "ok: $*"; }

# The scaffolder cds to its own repo root, so the fixture gets a copy of
# the script under scripts/ — the same relative layout as the real repo.
new_repo() {
local r="$WORK/$1"
	mkdir -p "$r/scripts" "$r/.github"
	cp "$TARGET" "$r/scripts/"
	# the scaffolder reads the root go directive from the repo-root go.mod
	printf 'module github.com/larsartmann/go-taskqueue\n\ngo 1.27.1\n' >"$r/go.mod"
	git init -q "$r"
	git -C "$r" config user.name smoke
	git -C "$r" config user.email smoke@example.invalid
	# host git config may force tag signing (editor launch — the heal
	# self-test disables it for the same reason)
	git -C "$r" config tag.gpgSign false
	echo "$r"
}

seed_config() {
	# Seed mirroring the repo config's shape (root entry first); $2 = keep
	# the file WITHOUT a trailing newline when set.
	{
		printf 'version: 2\nupdates:\n'
		printf '  - package-ecosystem: gomod\n'
		printf '    directory: /\n'
		printf '    schedule:\n'
		printf '      interval: weekly\n'
	} >"$1/.github/dependabot.yml"
	if [ "${2:-}" = no-newline ]; then
		printf '    open-pull-requests-limit: 5' >>"$1/.github/dependabot.yml"
	fi
}

# --- happy path: go.mod scaffold + dependabot entry appended ---------------
r=$(new_repo happy)
git -C "$r" commit -q --allow-empty -m init
git -C "$r" tag internal/task/v0.1.0
seed_config "$r"
out=$(cd "$r" && ./scripts/new-module.sh internal/foo internal/task 2>&1) || fail "happy path exited nonzero: $out"
grep -q '^module github.com/larsartmann/go-taskqueue/internal/foo$' "$r/internal/foo/go.mod" || fail "go.mod module line wrong"
grep -qF 'require github.com/larsartmann/go-taskqueue/internal/task v0.1.0' "$r/internal/foo/go.mod" || fail "go.mod require line wrong"
grep -qF 'replace github.com/larsartmann/go-taskqueue/internal/task => ../task' "$r/internal/foo/go.mod" || fail "go.mod replace line wrong"
grep -qF 'scaffolded internal/foo/go.mod (+ .github/dependabot.yml entry)' <<<"$out" || fail "success line does not mention the dependabot entry: $out"
grep -qF '    directory: /internal/foo' "$r/.github/dependabot.yml" || fail "dependabot entry not appended"
grep -qF '          - patch' "$r/.github/dependabot.yml" || fail "dependabot entry block incomplete"
grep -qF '    directory: /' "$r/.github/dependabot.yml" || fail "seed entry lost by the append"
pass "happy path: go.mod + dependabot entry, seed preserved"

# --- append onto a config missing its trailing newline ---------------------
r=$(new_repo no-newline)
git -C "$r" commit -q --allow-empty -m init
git -C "$r" tag internal/task/v0.1.0
seed_config "$r" no-newline
(cd "$r" && ./scripts/new-module.sh internal/bar internal/task) >/dev/null 2>&1 || fail "no-newline append exited nonzero"
grep -qF $'weekly\n    open-pull-requests-limit: 5\n  - package-ecosystem: gomod\n    directory: /internal/bar' "$r/.github/dependabot.yml" ||
	fail "append glued the entry onto the last seed line (newline guard rotted)"
pass "append onto newline-less config stays line-aligned"

# --- refusal: go.mod already exists (config untouched) ----------------------
r=$(new_repo existing-gomod)
git -C "$r" commit -q --allow-empty -m init
git -C "$r" tag internal/task/v0.1.0
seed_config "$r"
mkdir -p "$r/internal/baz"
echo 'module existing' >"$r/internal/baz/go.mod"
before=$(md5sum "$r/.github/dependabot.yml")
if out=$(cd "$r" && ./scripts/new-module.sh internal/baz internal/task 2>&1); then
	fail "existing go.mod was not refused: $out"
fi
grep -q "already exists" <<<"$out" || fail "existing-go.mod refusal lacks the reason: $out"
[ "$before" = "$(md5sum "$r/.github/dependabot.yml")" ] || fail "refused run mutated dependabot.yml"
pass "existing-go.mod refusal leaves the config untouched"

# --- refusal: entry already covers the directory ----------------------------
r=$(new_repo existing-entry)
git -C "$r" commit -q --allow-empty -m init
git -C "$r" tag internal/task/v0.1.0
seed_config "$r"
printf '  - package-ecosystem: gomod\n    directory: /internal/qux\n' >>"$r/.github/dependabot.yml"
if out=$(cd "$r" && ./scripts/new-module.sh internal/qux internal/task 2>&1); then
	fail "covered directory was not refused: $out"
fi
grep -q "already covers /internal/qux" <<<"$out" || fail "duplicate-entry refusal lacks the reason: $out"
[ ! -e "$r/internal/qux/go.mod" ] || fail "refused run scaffolded go.mod anyway"
pass "duplicate-entry refusal, nothing scaffolded"

# --- refusal: dependabot.yml missing ----------------------------------------
r=$(new_repo missing-config)
git -C "$r" commit -q --allow-empty -m init
git -C "$r" tag internal/task/v0.1.0
if out=$(cd "$r" && ./scripts/new-module.sh internal/waldo internal/task 2>&1); then
	fail "missing dependabot.yml was not refused: $out"
fi
grep -q "missing" <<<"$out" || fail "missing-config refusal lacks the reason: $out"
[ ! -e "$r/internal/waldo/go.mod" ] || fail "refused run scaffolded go.mod anyway"
pass "missing-config refusal, nothing scaffolded"

echo "new-module smoke ok (happy path, newline guard, 3 refusals keep the tree clean)"
