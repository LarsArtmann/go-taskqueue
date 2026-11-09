# scripts/smoke/_lib.sh — scratch-repo factory shared by repo-surface smokes
# (extracted from smoke/archive-evidence.sh, 23-40 report f8). Sourcing
# smoke assumes: WORK set to a mktemp dir cleaned by the caller's trap,
# and `set -euo pipefail` already active. bootstrap-install and
# release-gates deliberately do NOT use this factory: their fixtures rely
# on flag-only git identity (-c user.*) as a load-bearing self-test
# premise, which repo-local git config from new_repo() would break.
#
fail() {
	echo "FAIL: $*" >&2
	exit 1
}
pass() { echo "ok: $*"; }

new_repo() {
	local r="$WORK/$1"
	git init -q "$r"
	git -C "$r" config user.name smoke
	git -C "$r" config user.email smoke@example.invalid
	git -C "$r" commit -q --allow-empty -m init
	echo "$r"
}

run_in() { (cd "$1" && "${@:2}"); }

# --self-test: pins the factory contract (fail exits 1, pass prints
# "ok: …", new_repo yields an init-committed repo with identity, run_in
# executes in the repo dir). Defined after the helpers it exercises.
if [ "${1:-}" = "--self-test" ]; then
	WORK=$(mktemp -d)
	trap 'rm -rf "$WORK"' EXIT
	set -euo pipefail
	r=$(new_repo selftest)
	pass "factory self-test start"
	[ -d "$r/.git" ] || fail "new_repo did not create a git repo"
	git -C "$r" log -1 --format=%s | grep -qx init || fail "new_repo repo lacks the init commit"
	git -C "$r" config user.name | grep -qx smoke || fail "new_repo repo lacks smoke identity"
	run_in "$r" git rev-parse --verify HEAD >/dev/null || fail "run_in did not execute in the repo dir"
	if fail "intentional" >/dev/null 2>&1; then
		fail "fail() did not exit nonzero"
	fi
	echo "smoke _lib.sh self-test ok: fail/pass/new_repo/run_in contract pinned"
	exit 0
fi
