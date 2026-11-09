#!/usr/bin/env bash
# Regression pin for scripts/archive-evidence.sh (23-28 report f1/f4):
# scratch-repo smoke exercising every branch — ignored-dest refusal,
# negation-rule acceptance, f9-shape FAIL, PENDING, SUCCESS,
# dir-source/outside-repo/dup-basename refusals, --help,
# ignored-but-tracked re-run — plus the
# git-check-ignore untracked-only semantics probe the ghost refusal
# relies on (a git upgrade that starts reporting tracked paths would
# hard-FAIL every legitimate archive re-run).
#
# SUCCESS/f9 use future-dated committer timestamps on pre-seeded
# "chore: auto-commit" commits: the helper treats any strictly
# post-copy commit (%ct > copy wall clock) as the daemon's sweep, so
# both verdicts are deterministic without racing a --wait poll.
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
TARGET="$SCRIPT_DIR/../archive-evidence.sh"
# POSIX guard (23-40 f9): the helper's manifest regen leans on GNU find's
# -printf (the only `find -printf` use in the repo's scripts; audited
# 2026-11-09). Skip cleanly where it is absent instead of failing every
# branch for a platform limitation.
if ! find . -maxdepth 0 -printf x >/dev/null 2>&1; then
	echo "archive-evidence smoke: SKIP (GNU find -printf required)"
	exit 0
fi
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
. "$SCRIPT_DIR/_lib.sh"

# --- --help ---------------------------------------------------------------
out=$(run_in "$WORK" "$TARGET" --help) || fail "--help exited nonzero"
grep -q "^Usage:" <<<"$out" || fail "--help did not print usage"
pass "--help prints usage, exit 0"

# --- ignored-dest refusal (nothing copied) --------------------------------
r=$(new_repo ignored-dest)
printf '*.log\n' >"$r/.gitignore"
mkdir -p "$r/ev" "$r/docs/status/assets"
echo x >"$r/ev/run.log"
if out=$(run_in "$r" "$TARGET" ev/run.log docs/status/assets/arch 2>&1); then
	fail "ignored destination was not refused: $out"
fi
grep -q "gitignore rule" <<<"$out" || fail "ignored-dest refusal lacks the ghost explanation: $out"
[ ! -e "$r/docs/status/assets/arch" ] || fail "refused run created the archive dir"
pass "ignored-dest refusal, nothing copied"

# --- negation-rule dest proceeds (deliberate content, not ghost-risk) ------
r=$(new_repo negation-dest)
printf '*.log\n!docs/status/assets/**/*.log\n' >"$r/.gitignore"
mkdir -p "$r/ev"
echo x >"$r/ev/run.log"
out=$(run_in "$r" "$TARGET" ev/run.log docs/status/assets/arch) || fail "negation-rule dest was refused: $out"
grep -q "negation rule" <<<"$out" || fail "negation acceptance missing the ok line: $out"
grep -q "^PENDING:" <<<"$out" || fail "negation-rule run should proceed to PENDING: $out"
[ -f "$r/docs/status/assets/arch/run.log" ] || fail "copy missing after negation-rule run"
pass "negation-rule dest proceeds (! match is deliberate content, 03-47 b5)"

# --- dir-source refusal ----------------------------------------------------
r=$(new_repo dir-src)
mkdir -p "$r/ev" "$r/docs/status/assets/arch"
echo x >"$r/ev/a.txt"
if out=$(run_in "$r" "$TARGET" ev docs/status/assets/arch 2>&1); then
	fail "directory source was not refused: $out"
fi
grep -q "not a regular file" <<<"$out" || fail "dir-source refusal lacks the branch's own line: $out"
pass "dir-source refusal"

# --- outside-repo dest refusal --------------------------------------------
r=$(new_repo outside-dest)
mkdir -p "$r/ev"
echo x >"$r/ev/a.txt"
if out=$(run_in "$r" "$TARGET" ev/a.txt "$WORK/outside" 2>&1); then
	fail "outside-repo destination was not refused: $out"
fi
grep -q "resolves outside the repository" <<<"$out" || fail "outside-dest refusal lacks the branch's own line: $out"
[ ! -e "$WORK/outside" ] || fail "refused run created the outside dir"
pass "outside-repo dest refusal"

# --- dup-basename refusal --------------------------------------------------
r=$(new_repo dup-basename)
mkdir -p "$r/ev1" "$r/ev2" "$r/docs/status/assets"
echo x >"$r/ev1/a.txt"
echo y >"$r/ev2/a.txt"
if out=$(run_in "$r" "$TARGET" ev1/a.txt ev2/a.txt docs/status/assets 2>&1); then
	fail "duplicate basenames were not refused: $out"
fi
grep -q "duplicate basename among sources" <<<"$out" || fail "dup-basename refusal lacks the branch's own line: $out"
pass "dup-basename refusal"

# --- PENDING (fresh archive, no daemon commit yet) -------------------------
r=$(new_repo pending)
mkdir -p "$r/ev" "$r/docs/status/assets/arch"
echo x >"$r/ev/a.txt"
out=$(run_in "$r" "$TARGET" ev/a.txt docs/status/assets/arch) || fail "PENDING run exited nonzero: $out"
grep -q "^PENDING:" <<<"$out" || fail "expected PENDING banner: $out"
[ -f "$r/docs/status/assets/arch/a.txt" ] || fail "copy missing after PENDING run"
[ -f "$r/docs/status/assets/arch/SHA256SUMS" ] || fail "SHA256SUMS missing after PENDING run"
[ "$(wc -l <"$r/docs/status/assets/arch/SHA256SUMS")" -eq 1 ] || fail "manifest must cover exactly the non-manifest archive files"
pass "PENDING verdict, copy + manifest landed"

# --- --wait N branch (23-40 f4): a PRE-DATED (past) daemon commit is not
# counted as the sweep (%ct > copy wall clock is required), so --wait 2
# polls, finds nothing new, and ends at PENDING with exit 0.
r=$(new_repo wait-predated)
mkdir -p "$r/ev" "$r/docs/status/assets/arch"
echo x >"$r/ev/a.txt"
echo old >"$r/docs/status/assets/arch/a.txt"
git -C "$r" add docs/status/assets/arch
past="@$(($(date +%s) - 3600))"
GIT_AUTHOR_DATE="$past" GIT_COMMITTER_DATE="$past" \
	git -C "$r" commit -q -m "chore: auto-commit 1 changed file(s) (heuristic)"
out=$(run_in "$r" "$TARGET" --wait 2 ev/a.txt docs/status/assets/arch) || fail "--wait with pre-dated daemon commit exited nonzero: $out"
grep -q "^PENDING:" <<<"$out" || fail "--wait branch should end at PENDING on a pre-dated commit: $out"
pass "--wait 2 with pre-dated (past) daemon commit: PENDING, exit 0"

# --- SUCCESS (pre-seeded post-copy daemon commit, full set) ----------------
r=$(new_repo success)
mkdir -p "$r/ev" "$r/docs/status/assets/arch"
echo x >"$r/ev/a.txt"
echo stale >"$r/docs/status/assets/arch/a.txt"
echo stale >"$r/docs/status/assets/arch/SHA256SUMS"
git -C "$r" add docs/status/assets/arch
future="@$(($(date +%s) + 3600))"
GIT_AUTHOR_DATE="$future" GIT_COMMITTER_DATE="$future" \
	git -C "$r" commit -q -m "chore: auto-commit 2 changed file(s) (heuristic)"
out=$(run_in "$r" "$TARGET" ev/a.txt docs/status/assets/arch) || fail "SUCCESS run exited nonzero: $out"
grep -q "^SUCCESS:" <<<"$out" || fail "expected SUCCESS verdict: $out"
grep -q "stale" "$r/docs/status/assets/arch/a.txt" && fail "source not recopied over stale content"
pass "SUCCESS verdict on full-set daemon commit"

# --- f9-shape FAIL (post-copy daemon commit missing a file) ----------------
r=$(new_repo f9)
mkdir -p "$r/ev" "$r/docs/status/assets/arch"
echo x >"$r/ev/a.txt"
echo only-one >"$r/docs/status/assets/arch/a.txt"
git -C "$r" add docs/status/assets/arch/a.txt
future="@$(($(date +%s) + 3600))"
GIT_AUTHOR_DATE="$future" GIT_COMMITTER_DATE="$future" \
	git -C "$r" commit -q -m "chore: auto-commit 1 changed file(s) (heuristic)"
if out=$(run_in "$r" "$TARGET" ev/a.txt docs/status/assets/arch 2>&1); then
	fail "f9-shape loss was not flagged: $out"
fi
grep -q "f9 silent-loss shape" <<<"$out" || fail "f9 failure lacks the loss-shape explanation: $out"
pass "f9-shape FAIL on partial daemon commit"

# --- ignored-but-tracked re-run (check-ignore must NOT flag tracked paths) -
r=$(new_repo tracked-rerun)
printf '*.log\n' >"$r/.gitignore"
mkdir -p "$r/ev" "$r/docs/status/assets/arch"
echo x >"$r/ev/run.log"
echo x >"$r/docs/status/assets/arch/run.log"
git -C "$r" add .gitignore
git -C "$r" commit -q -m init
git -C "$r" add -f docs/status/assets/arch/run.log
git -C "$r" commit -q -m "track the ignored-path archive file"
out=$(run_in "$r" "$TARGET" ev/run.log docs/status/assets/arch) || fail "tracked-ignored re-run was refused: $out"
grep -q "gitignore rule" <<<"$out" && fail "tracked file flagged as ghost: $out"
grep -q "^PENDING:" <<<"$out" || fail "tracked-ignored re-run should proceed to PENDING: $out"
pass "ignored-but-tracked re-run proceeds (no ghost refusal)"

# --- git check-ignore untracked-only semantics probe -----------------------
r=$(new_repo check-ignore-semantics)
printf '*.log\n' >"$r/.gitignore"
echo x >"$r/a.log"
echo x >"$r/b.txt"
git -C "$r" add .gitignore b.txt
git -C "$r" commit -q -m init
if out=$(git -C "$r" check-ignore -v -- b.txt 2>&1); then
	fail "check-ignore reported a tracked, unignored path: $out"
fi
if out=$(git -C "$r" check-ignore -v -- a.log 2>&1); then
	: # untracked ignored path must be reported
	grep -q "a.log" <<<"$out" || fail "check-ignore output missing the path: $out"
else
	fail "check-ignore did not report an untracked ignored path"
fi
git -C "$r" add -f a.log
git -C "$r" commit -q -m "track a.log"
rc=0
out=$(git -C "$r" check-ignore -v -- a.log 2>&1) || rc=$?
if [ "$rc" -eq 0 ]; then
	fail "check-ignore reported a TRACKED path (untracked-only semantics broken; archive re-runs would hard-FAIL): $out"
elif [ "$rc" -ne 1 ]; then
	fail "check-ignore unexpected rc=$rc on tracked path: $out"
fi
pass "git check-ignore untracked-only semantics hold (tracked-ignored invisible)"

echo "archive-evidence smoke: PASS (12 checks)"
