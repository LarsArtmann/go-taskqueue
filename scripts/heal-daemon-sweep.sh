#!/usr/bin/env bash
# heal-daemon-sweep.sh: scripted heal for footer-less daemon-swept commits.
#
# The auto-commit daemon sweeps files into footer-less `chore:` commits while
# an agent works (the "daemon race", hit 2026-09-20 09-52 §d, 2026-09-21
# 00-27 §d). Without a `Task-Queue-ID:` footer the queue cannot attribute the
# work. This tool bakes the 00-27 §d heal playbook into a runnable script:
# msg-filter the footer onto every footer-less commit in an UNPUSHED range,
# then run the mandatory verification (same subjects, same per-commit stats,
# exactly one well-formed trailing footer each, byte-equal tree, no tags
# affected).
#
# Safety rails (history policy: never rewrite pushed commits):
#   - refuses a dirty worktree;
#   - refuses an empty heal range;
#   - refuses if any range commit is reachable from a remote-tracking ref
#     other than the base itself;
#   - footer-carrying commits in the range (e.g. a real task commit on top
#     of sweeps) are preserved verbatim and still footer-verified;
#   - the backup ref refs/original/heal-daemon-sweep is dropped ONLY after
#     all five verifications pass; on any failure it is kept and the script
#     exits non-zero (recovery: read the printed fork records, or
#     `git reset --soft refs/original/heal-daemon-sweep`).
#   - prints old→new fork-record lines (sanctioned by check-dead-sha-refs)
#     for pasting into the window's status report;
#   - post-heal citation check: sweeps the dead-sha gate's living-docs scope
#     (docs/status, TODO_LIST.md, AGENTS.md) for lines still citing a
#     REWRITTEN sha and prints each stale site with its paste-ready
#     fork-record arrow, so the dangling-cite residue is cured in the
#     healing window instead of a later one (2026-10-06 02-02 §e5: the
#     99458a91 class waited for a second window).
#
#   - a commit footered with a DIFFERENT Task-Queue-ID inside the range is
#     refused BY DESIGN (2026-10-13 01-54 §d2): even when that commit sits
#     ABOVE the footer-less daemon commit the caller wants healed (the
#     `--from <oldest-exactly-mine>` shape), the rewrite cascades — the
#     foreign commit's SHA changes too (filter-branch rewrites every
#     descendant), which silently rewrites another task's history. Healing
#     below a foreign-ID commit is therefore structurally impossible for
#     this tool. Escape hatches: `--from <foreign-commit>` heals only the
#     span ABOVE it (the below-daemon commit stays footer-less; manual
#     `git commit --amend` territory per the AGENTS.md Known Issues
#     fallback), or the sanctioned soft-reset fold.
#
# Usage: scripts/heal-daemon-sweep.sh [--from <ref>] [--retitle <subject>] <Task-Queue-ID>
#        scripts/heal-daemon-sweep.sh --self-test
#   --from <ref>  base of the heal range (default: origin/master, then
#                 @{upstream}, then the root commit). Everything in
#                 <ref>..HEAD is treated as unpushed-local and verified.
#   --retitle <subject>
#                 set this subject on every HEALED (footer-less) commit in
#                 the range — healed sweeps keep the daemon's lying
#                 `chore: auto-commit` subject even when they carry real
#                 work (2026-10-17 batch: 8ee9930a, aadfed9d); footer-
#                 carrying commits in the range keep their subjects.
#   <Task-Queue-ID> may be a hex task id OR a session-scope slug
#                 (e.g. 20261011-b10-alert-routing-pack): a session's own
#                 footered commits interleave with daemon sweeps, so the
#                 sweeps must heal onto the session footer.
set -uo pipefail

BACKUP_REF="refs/original/heal-daemon-sweep"
tmp=""

die() {
	echo "heal-daemon-sweep: $*" >&2
	exit 1
}

usage() {
	echo "usage: $0 [--from <ref>] [--retitle <subject>] <Task-Queue-ID>" >&2
	echo "       $0 --self-test" >&2
	exit 2
}

need_git() {
	command -v git >/dev/null 2>&1 || die "git not found in PATH"
}

footer_well_formed() {
	# A message is healed iff its LAST non-empty line is exactly the footer.
	local msg=$1 id=$2
	local last
	last=$(printf '%s\n' "$msg" | sed '/^[[:space:]]*$/d' | tail -n 1)
	[ "$last" = "Task-Queue-ID: $id" ]
}

has_footer() {
	printf '%s\n' "$1" | grep -q '^Task-Queue-ID: '
}

# token_arrow_escaped <tok> <text>: rc=0 iff the token IS a member of an
# old→new fork-record pair on that line — the same escape semantics as
# check-dead-sha-refs.sh, so a site the gate whitewashes is not reported
# (and one it would count as cured is not double-flagged here).
token_arrow_escaped() {
	local tok=$1 text=$2
	if grep -qE "(^|[^0-9a-f])${tok}(→|<-)[0-9a-f]{7,40}" <<<"$text"; then return 0; fi
	if grep -qE "[0-9a-f]{7,40}(→|<-)${tok}([^0-9a-f]|$)" <<<"$text"; then return 0; fi
	return 1
}

# resolve_base: default base ref, first existing of origin/master, @{upstream},
# or the root commit.
resolve_base() {
	local requested=$1
	if [ "$requested" != "" ]; then
		git rev-parse --verify --quiet "$requested" >/dev/null && {
			echo "$requested"
			return
		}
		die "base ref '$requested' not found"
	fi
	if git rev-parse --verify --quiet origin/master >/dev/null 2>&1; then
		echo "origin/master"
		return
	fi
	if git rev-parse --verify --quiet '@{upstream}' >/dev/null 2>&1; then
		echo '@{upstream}'
		return
	fi
	git rev-list --max-parents=0 HEAD
}

# check_rails base id: every pre-flight refusal. Dies with the reason on hit;
# echoes "noop" when every range commit already carries the target footer.
check_rails() {
	local base=$1 id=$2
	[ -z "$(git status --porcelain)" ] || die "worktree is dirty; commit or stash first"
	[ -n "$(git rev-list "$base..HEAD")" ] || die "range $base..HEAD is empty; nothing to heal"

	local pushed=""
	while IFS= read -r r; do
		local hits
		hits=$(comm -12 <(git rev-list "$r" | sort) <(git rev-list "$base..HEAD" | sort))
		pushed="$pushed$hits"
	done < <(git for-each-ref --format='%(refname)' 'refs/remotes/*')
	[ -z "$pushed" ] || die "refusing: pushed commits are inside the heal range (history policy)"

	# Tag rail: the pushed-commit rail does not catch tags — a tag on an
	# unpushed range commit would dangle off-branch after the rewrite. Refuse
	# before run_filter instead of relying on post-rewrite detection.
	local tagged=""
	while IFS= read -r t; do
		local hits
		hits=$(comm -12 <(git rev-list "$t" | sort) <(git rev-list "$base..HEAD" | sort))
		tagged="$tagged$hits"
	done < <(git tag --format='%(refname)')
	[ -z "$tagged" ] || die "refusing: tag(s) reference commits inside the heal range ($tagged); re-tag after healing"

	# NOTE: footer-carrying commits inside the range are legitimate (a task
	# commit can sit on top of daemon sweeps) — run_filter preserves their
	# messages verbatim and verify_heal demands the TARGET footer on them.
	# A commit footered with a DIFFERENT id is never rewritten (footer
	# rewrites are a manual, reviewed operation); a fully footered range is
	# a no-op.
	local c msg unfootered=0 mode=${3:-}
	local -a blockers=()
	while IFS= read -r c; do
		msg=$(git log -1 --format='%B' "$c")
		if has_footer "$msg"; then
			footer_well_formed "$msg" "$id" && continue
			blockers+=("$c")
			continue
		fi
		unfootered=$((unfootered + 1))
	done < <(git rev-list "$base..HEAD")
	if [ "${#blockers[@]}" -gt 0 ]; then
		# Row 55 span terminator: ONE different-id footer below an
		# otherwise-exactly-mine span (my footer at HEAD + my sweep + a
		# foreign footer below — the 2026-10-12 01-41 shape) is the span
		# boundary, not a refusal: with a DEFAULT base the heal narrows to
		# the exactly-mine span above it. An explicit --from is a conscious
		# choice and never auto-narrows; >1 blocker is a genuinely mixed
		# span and refuses with the exact invocation that would be accepted
		# (row 56: never make the caller eyeball git log).
		if [ "${#blockers[@]}" -eq 1 ] && [ -z "$mode" ]; then
			echo "narrow ${blockers[0]}"
			return 0
		fi
		local oldest="${blockers[${#blockers[@]} - 1]}"
		{
			echo "refusing: ${#blockers[@]} commit(s) in $base..HEAD inside the heal range:"
			for c in "${blockers[@]}"; do
				echo "  $(git log -1 --oneline "$c") carries a different Task-Queue-ID footer"
			done
			echo "Heal only your span (auto-detected oldest blocker):"
			echo "  $0 $id --from $oldest"
		} >&2
		return 1
	fi
	[ "$unfootered" -gt 0 ] || echo "noop"
}

# snapshot_subjects base id retitle: the EXPECTED subjects of base..HEAD
# oldest-first, one per line (subjects cannot contain newlines). Footer-
# carrying commits are preserved verbatim -> their subject stands; healed
# sweeps get <retitle> when given, else their own subject. verify_heal
# compares the rewrite against this list, so --retitle turns the
# subjects-must-not-move rail into subjects-match-intent.
snapshot_subjects() {
	local base=$1 id=$2 retitle=$3
	local c msg
	while IFS= read -r c; do
		msg=$(git log -1 --format='%B' "$c")
		if has_footer "$msg"; then
			git log -1 --format='%s' "$c"
		elif [ -n "$retitle" ]; then
			printf '%s\n' "$retitle"
		else
			git log -1 --format='%s' "$c"
		fi
	done < <(git rev-list --reverse "$base..HEAD")
}

# run_filter footer base subject: the actual msg-filter rewrite. On failure the
# filter's stderr is printed (a bare rc=1 hid the cause once — 08-06 report
# §e2 polish row).
run_filter() {
	local footer=$1 base=$2 subject=$3
	local err
	err=$(FILTER_BRANCH_SQUELCH_WARNING=1 TQ_HEAL_FOOTER="$footer" TQ_HEAL_SUBJECT="$subject" git filter-branch -f --msg-filter '
		msg=$(cat)
		case $msg in
		*"Task-Queue-ID: "*)
			printf "%s\n" "$msg"
			exit 0
			;;
		esac
		if [ -n "$TQ_HEAL_SUBJECT" ]; then
			printf "%s\n\n" "$TQ_HEAL_SUBJECT"
		else
			printf "%s\n" "$msg"
		fi |
			git interpret-trailers --if-exists doNothing --trailer "$TQ_HEAL_FOOTER"
	' -- "$base..HEAD" 2>&1 >/dev/null) || {
		echo "filter-branch failed:" >&2
		printf '%s\n' "$err" >&2
		return 1
	}
	# Self-test seam: filter-branch -f wipes unrelated refs/original backups
	# on modern git, so the stale-sibling hijack state the current-branch
	# resolution (92c9193c) guards against cannot be built from outside —
	# the self-test recreates it HERE, after the wipe, before resolution.
	if [ -n "${TQ_HEAL_TEST_STALE_BACKUP:-}" ]; then
		git update-ref "refs/original/refs/heads/$TQ_HEAL_TEST_STALE_BACKUP" "$(git rev-parse HEAD~1)"
	fi
	local bref
	bref=$(git symbolic-ref -q HEAD || return 1)
	bref="refs/original/$bref"
	git show-ref --verify --quiet "$bref" || return 1
	git update-ref "$BACKUP_REF" "$(git rev-parse --verify "$bref")"
	git update-ref -d "$bref"
}

# verify_heal base id subj_old stat_old: the five playbook verifications.
# subj_old/stat_old are snapshots taken BEFORE the rewrite (over base..HEAD);
# comparing against base..HEAD AFTER works because the base ref never moves
# and preserved commits keep their SHAs. rc 0 iff all checks pass.
verify_heal() {
	local base=$1 id=$2 subj_old=$3 stat_old=$4
	local fail=0 subj_new stat_new c new msg

	# 1+2: same subjects, same per-commit change sets (oldest-first lockstep
	# is guaranteed by equal counts, which the rewrite preserves).
	subj_new=$(git log --reverse --format='%s' "$base..HEAD")
	if [ "$subj_old" != "$subj_new" ]; then
		echo "FAIL: subjects do not match the expected (post-heal) list" >&2
		fail=1
	fi
	stat_new=$(git rev-list --reverse "$base..HEAD" | while IFS= read -r c; do git diff-tree --no-commit-id --name-only -r "$c" | sort | md5sum; done)
	if [ "$stat_old" != "$stat_new" ]; then
		echo "FAIL: per-commit change sets changed across the heal" >&2
		fail=1
	fi

	# 3: exactly one well-formed trailing footer in every healed commit.
	while IFS= read -r new; do
		msg=$(git log -1 --format='%B' "$new")
		if ! footer_well_formed "$msg" "$id"; then
			echo "FAIL: commit $new lacks a well-formed trailing footer" >&2
			fail=1
		fi
		if [ "$(printf '%s\n' "$msg" | grep -c '^Task-Queue-ID: ')" != "1" ]; then
			echo "FAIL: commit $new carries more than one footer" >&2
			fail=1
		fi
		# 3b — post-heal citation check: the footer must be MACHINE-visible
		# to the queue's cross-reference (CommitsByTrailer runs
		# interpret-trailers --parse), not merely last-line-shaped. A footer
		# glued above an attribution block parses as nothing (row 276 class);
		# the msg-filter builds it parseable, this pins that it stayed so.
		if ! parsed=$(git log -1 --format='%B' "$new" | git interpret-trailers --parse 2>/dev/null); then
			echo "FAIL: interpret-trailers failed on healed commit $new" >&2
			fail=1
		elif ! printf '%s\n' "$parsed" | grep -qi "^task-queue-id: $id\$"; then
			echo "FAIL: healed footer in $new is not trailer-parseable (invisible to the queue)" >&2
			fail=1
		fi
	done < <(git rev-list --reverse "$base..HEAD")

	# 4: byte-equal tree (the green battery carries over verbatim).
	if [ -n "$(git diff "$BACKUP_REF" HEAD)" ]; then
		echo "FAIL: tree differs from the pre-heal backup" >&2
		fail=1
	fi

	# 5: no tags reference any rewritten commit — belt-and-suspenders behind
	# the pre-flight tag rail in check_rails (row 434: the comment names the
	# promise and the mechanism as one story): reaching here with a tag hit
	# means the tag appeared between the rail check and the rewrite.
	while IFS= read -r new; do
		if [ -n "$(git tag --points-at "$new")" ] || [ -n "$(git tag --contains "$new")" ]; then
			echo "FAIL: tag(s) reference rewritten commit $new" >&2
			fail=1
		fi
	done < <(git rev-list "$base..HEAD")

	return $fail
}

# print_fork_records base shas_old: old→new lines via positional map (equal
# counts and identical ordering make index alignment exact). shas_old is the
# pre-rewrite rev-list captured BEFORE the msg-filter: after it, base..HEAD
# lists the NEW shas and the backup chain is disjoint from HEAD, so deriving
# the "old" side post-hoc prints identity pairs (the 2026-10-09 04-14 and
# 02-53 §f1 false-record class, rows 69/455). Preserved footer-carrying
# commits keep their shas, so their old==new pairs are legitimate.
print_fork_records() {
	local base=$1 olds=$2
	local news
	news=$(git rev-list --reverse "$base..HEAD")
	while IFS= read -r old && IFS= read -r new <&3; do
		echo "FORK-RECORD: $old→$new"
	done <<<"$olds" 3<<<"$news"
}

# check_citations base olds: post-heal residue sweep. A rewritten sha
# dangles the moment the heal lands, and any doc line still citing it turns
# check-dead-sha-refs red until the site carries the fork-record arrow.
# Lines whose token is already a fork-record pair member (same per-token
# semantics as the gate) are cured; preserved commits (old==new, still
# reachable) are not residue. Informational only — the heal itself stays
# rc=0; the printed arrow per site is the cure.
check_citations() {
	local base=$1 olds=$2
	local top old new f hit line text tok hits=0
	top=$(git rev-parse --show-toplevel)
	local docs=()
	for f in "$top"/docs/status/*.md "$top"/docs/status/tasks/*.md "$top"/TODO_LIST.md "$top"/AGENTS.md; do
		[ -f "$f" ] && docs+=("$f")
	done
	[ "${#docs[@]}" -gt 0 ] || return 0
	while IFS= read -r old && IFS= read -r new <&3; do
		[ "$old" != "$new" ] || continue
		for f in "${docs[@]}"; do
			while IFS= read -r hit; do
				line=${hit%%:*}
				text=${hit#*:}
				for tok in $(grep -oE '[0-9a-f]{7,40}' <<<"$text" | sort -u); do
					[[ "$old" == "$tok"* ]] || continue
					token_arrow_escaped "$tok" "$text" && continue
					if [ "$hits" = 0 ]; then
						echo "citation check: docs still cite rewritten sha(s) — paste the fork-record arrow onto each site:"
					fi
					hits=$((hits + 1))
					echo "  ${f#"$top"/}:$line cites $tok — fork record: $tok→$new"
				done
			done < <(grep -nE "${old:0:7}" "$f" 2>/dev/null)
		done
	done <<<"$olds" 3<<<"$(git rev-list --reverse "$base..HEAD")"
	[ "$hits" -gt 0 ] && echo "citation check: $hits stale site(s) — check-dead-sha-refs stays red until each carries the arrow"
	return 0
}

main() {
	local base_arg="" footer="" retitle="" base id
	while [ $# -gt 0 ]; do
		case $1 in
		--from)
			[ $# -ge 2 ] || usage
			base_arg=$2
			shift 2
			;;
		--retitle)
			[ $# -ge 2 ] || usage
			case $2 in
			"" | *$'\n'*) die "invalid --retitle subject: must be a single non-empty line" ;;
			esac
			retitle=$2
			shift 2
			;;
		-*) usage ;;
		*)
			[ -z "$footer" ] || usage
			case $1 in
			"" | *[!0-9A-Za-z._-]*) die "invalid Task-Queue-ID '$1': must be a hex task id or session-scope slug (letters, digits, '.', '_', '-')" ;;
			esac
			id=$1
			footer="Task-Queue-ID: $1"
			shift
			;;
		esac
	done
	[ -n "$footer" ] || usage
	need_git
	git rev-parse --is-inside-work-tree >/dev/null 2>&1 || die "not inside a git work tree"
	base=$(resolve_base "$base_arg")

	git update-ref -d "$BACKUP_REF" 2>/dev/null
	local rail
	rail=$(check_rails "$base" "$id" "${base_arg:+explicit}") || exit 1
	if [[ "$rail" == narrow\ * ]]; then
		local narrowed="${rail#narrow }"
		echo "span-terminator: one different-id footer at $(git log -1 --oneline "$narrowed") bounds the exactly-mine span — healing only $narrowed..HEAD (row 55)"
		base="$narrowed"
		rail=$(check_rails "$base" "$id" explicit) || exit 1
	fi
	if [ "$rail" = "noop" ]; then
		echo "nothing to heal: every commit in $base..HEAD already carries the footer"
		return 0
	fi
	echo "healing $(git rev-list --count "$base..HEAD") commit(s) in $base..HEAD with: $footer${retitle:+ (retitle: $retitle)}"

	local subj_old stat_old shas_old
	subj_old=$(snapshot_subjects "$base" "$id" "$retitle")
	stat_old=$(git rev-list --reverse "$base..HEAD" | while IFS= read -r c; do git diff-tree --no-commit-id --name-only -r "$c" | sort | md5sum; done)
	shas_old=$(git rev-list --reverse "$base..HEAD")

	run_filter "$footer" "$base" "$retitle" || die "filter-branch failed (backup kept at $BACKUP_REF if it existed)"

	if verify_heal "$base" "$id" "$subj_old" "$stat_old"; then
		print_fork_records "$base" "$shas_old"
		check_citations "$base" "$shas_old"
		git update-ref -d "$BACKUP_REF"
		echo "HEAL OK — backup ref dropped"
		return 0
	fi
	die "verification FAILED — backup kept at $BACKUP_REF; recovery: git reset --soft $BACKUP_REF"
}

# --- self-test: fixture repo exercising the heal + every refusal rail ---
self_test() {
	local self=$0
	case $self in
	/*) ;;
	*) self="$PWD/${self#./}" ;;
	esac
	local repo
	tmp=$(mktemp -d) || die "mktemp failed"
	trap 'rm -rf "$tmp"' EXIT
	local repo="$tmp/repo"
	git init -q -b master "$repo"
	git -C "$repo" config user.email t@t
	git -C "$repo" config user.name t
	git -C "$repo" config tag.gpgSign false

	local ok=0 fail=0
	# expect_refusal <label> <rc> [want-substring] [errfile]: a refusal rc
	# must be non-zero; when want+errfile are given the captured stderr must
	# ALSO contain want, so a different refusal firing first cannot mask a
	# rotted rail (the dead --sort=reverse class, 2026-10-03 01-22 report
	# §d1). Cases left bare stay bare pending the row-452 owner test-norm
	# call (retroactive tightening is BLOCKED there).
	expect_refusal() {
		local label=$1 rc=$2 want=${3:-} errfile=${4:-}
		if [ "$rc" = "0" ]; then
			fail=$((fail + 1))
			echo "SELF-TEST FAIL (expected refusal): $label" >&2
			return
		fi
		if [ -n "$want" ] && ! grep -qF -- "$want" "$errfile"; then
			fail=$((fail + 1))
			echo "SELF-TEST FAIL ($label): wrong refusal reason — stderr missing: $want; got:" >&2
			cat "$errfile" >&2
			return
		fi
		ok=$((ok + 1))
	}

	echo a >"$repo/a.txt"
	git -C "$repo" add a.txt
	git -C "$repo" commit -qm "base"
	git -C "$repo" update-ref refs/remotes/origin/master HEAD
	local old1 old2
	echo b >"$repo/b.txt"
	git -C "$repo" add b.txt
	git -C "$repo" commit -qm "chore: sweep one"
	old1=$(git -C "$repo" rev-parse HEAD)
	echo c >"$repo/c.txt"
	git -C "$repo" add c.txt
	git -C "$repo" commit -qm "chore: sweep two"
	old2=$(git -C "$repo" rev-parse HEAD)

	# Post-heal citation-check fixture (02-02 §e5): a committed docs file
	# citing the pre-rewrite shas — line 1 bare (stale site to flag), line
	# 2 a fork-record arrow pair (already cured, must stay unflagged).
	mkdir -p "$repo/docs/status"
	cat >"$repo/docs/status/cite.md" <<EOF
the sweep landed in ${old1} (heal pending)
fork playbook: ${old1}→${old2}
EOF
	git -C "$repo" add docs/status/cite.md
	git -C "$repo" commit -qm "docs: cite the sweeps"

	# Happy path: heal both sweeps. "$self" (NOT "$0"): the fixture subshell
	# cds away, so a relative $0 resolves into the temp repo and dies with
	# "No such file or directory" — the 2026-10-04 00-52 red (row 456).
	(cd "$repo" && "$self" --from origin/master deadbeef00000000000000000000000000000001) >"$tmp/out1" 2>"$tmp/err1" &&
		ok=$((ok + 1)) || {
		fail=$((fail + 1))
		echo "SELF-TEST FAIL: happy path exited non-zero" >&2
		cat "$tmp/err1" >&2
	}
	local new1 new2 m1 m2
	new1=$(git -C "$repo" rev-list --reverse origin/master..HEAD | sed -n 1p)
	new2=$(git -C "$repo" rev-list --reverse origin/master..HEAD | sed -n 2p)
	m1=$(git -C "$repo" log -1 --format='%B' "$new1")
	m2=$(git -C "$repo" log -1 --format='%B' "$new2")
	if footer_well_formed "$m1" "deadbeef00000000000000000000000000000001" &&
		footer_well_formed "$m2" "deadbeef00000000000000000000000000000001" &&
		[ "$m1" = "$(printf 'chore: sweep one\n\nTask-Queue-ID: deadbeef00000000000000000000000000000001')" ] &&
		[ -z "$(git -C "$repo" for-each-ref 'refs/original/*')" ]; then
		ok=$((ok + 1))
	else
		fail=$((fail + 1))
		echo "SELF-TEST FAIL: healed messages not as expected" >&2
	fi
	if [ "$new1" != "$old1" ] && [ "$new2" != "$old2" ]; then
		ok=$((ok + 1))
	else
		fail=$((fail + 1))
		echo "SELF-TEST FAIL: shas did not move" >&2
	fi

	# Post-heal citation check (02-02 §e5): the bare cite is flagged with
	# its paste-ready fork-record arrow; the arrowed line is cured and
	# stays unflagged; the summary counts exactly the stale sites.
	if grep -qF "docs/status/cite.md:1 cites ${old1}" "$tmp/out1" &&
		grep -qF "fork record: ${old1}→${new1}" "$tmp/out1" &&
		grep -qF "citation check: 1 stale site(s)" "$tmp/out1" &&
		[ "$(grep -cF 'cite.md:' "$tmp/out1")" = 1 ]; then
		ok=$((ok + 1))
	else
		fail=$((fail + 1))
		echo "SELF-TEST FAIL: post-heal citation check wrong" >&2
		cat "$tmp/out1" >&2
	fi

	# Fork-record honesty pin (rows 69/455): the left side must be the TRUE
	# pre-rewrite sha. Deriving "old" after the msg-filter lists post-rewrite
	# shas on BOTH sides (identity pairs), so reports paste a false old side.
	if grep -qF "FORK-RECORD: $old1→$new1" "$tmp/out1" &&
		grep -qF "FORK-RECORD: $old2→$new2" "$tmp/out1" &&
		! grep -qF "FORK-RECORD: $new1→$new1" "$tmp/out1"; then
		ok=$((ok + 1))
	else
		fail=$((fail + 1))
		echo "SELF-TEST FAIL: fork records must carry the true pre-rewrite sha on the left" >&2
		cat "$tmp/out1" >&2
	fi

	# Rail: dirty worktree refusal.
	echo dirt >>"$repo/a.txt"
	(cd "$repo" && "$self" --from origin/master deadbeef00000000000000000000000000000002) >/dev/null 2>&1
	expect_refusal "dirty worktree" $?
	git -C "$repo" checkout -q -- a.txt

	# A fully-footered range is a no-op success; a different-id footer is a
	# refusal; a mixed range heals only the sweeps and preserves the task
	# commit's message byte-identical.
	echo d >"$repo/d.txt"
	git -C "$repo" add d.txt
	git -C "$repo" commit -qm "work: real task"
	git -C "$repo" commit -q --amend -m "$(printf 'work: real task\n\nTask-Queue-ID: deadbeef00000000000000000000000000000001')"
	(cd "$repo" && "$self" --from origin/master deadbeef00000000000000000000000000000001) >/dev/null 2>&1
	local noop_rc=$?
	if [ "$noop_rc" = "0" ]; then
		ok=$((ok + 1))
	else
		fail=$((fail + 1))
		echo "SELF-TEST FAIL: fully-footered range should no-op successfully" >&2
	fi

	(cd "$repo" && "$self" --from origin/master deadbeef0000000000000000000000000000000f) >/dev/null 2>"$tmp/err3"
	expect_refusal "different-id footer" $? "carries a different Task-Queue-ID footer" "$tmp/err3"

	local mixed_rc
	(cd "$repo" && git reset -q --soft HEAD~1)
	(cd "$repo" && git commit -qm "work: real task")
	(cd "$repo" && "$self" --from origin/master deadbeef00000000000000000000000000000001) >/dev/null 2>"$tmp/err4" && mixed_rc=0 || mixed_rc=1
	if [ "$mixed_rc" = "0" ]; then
		ok=$((ok + 1))
	else
		fail=$((fail + 1))
		echo "SELF-TEST FAIL: mixed-range heal should succeed" >&2
		cat "$tmp/err4" >&2
	fi
	local footered_after
	footered_after=$(git -C "$repo" log -1 --format='%B' HEAD)
	if [ "$footered_after" = "$(printf 'work: real task\n\nTask-Queue-ID: deadbeef00000000000000000000000000000001')" ]; then
		ok=$((ok + 1))
	else
		fail=$((fail + 1))
		echo "SELF-TEST FAIL: task commit message not preserved in mixed heal" >&2
	fi

	# Rail: pushed commit inside the heal range.
	git -C "$repo" update-ref refs/remotes/origin/master HEAD
	(cd "$repo" && "$self" --from refs/remotes/origin/master~1 deadbeef00000000000000000000000000000004) >/dev/null 2>&1
	expect_refusal "pushed commit in range" $?

	# Rail: empty heal range.
	(cd "$repo" && "$self" --from origin/master deadbeef00000000000000000000000000000005) >/dev/null 2>&1
	expect_refusal "empty range" $?

	# Rail: tag on a commit inside the heal range (the pushed-commit rail
	# does not catch tags).
	echo e >"$repo/e.txt"
	git -C "$repo" add e.txt
	git -C "$repo" commit -qm "chore: sweep three"
	git -C "$repo" tag sweep-tag HEAD
	(cd "$repo" && "$self" --from origin/master deadbeef00000000000000000000000000000006) >/dev/null 2>"$tmp/err_tag"
	expect_refusal "tag in heal range" $? "tag(s) reference commits inside the heal range" "$tmp/err_tag"
	git -C "$repo" tag -d sweep-tag

	# A tag on the BASE commit is outside the heal range — heal succeeds.
	git -C "$repo" tag base-tag origin/master
	(cd "$repo" && "$self" --from origin/master deadbeef00000000000000000000000000000006) >/dev/null 2>"$tmp/err5"
	if [ "$?" = "0" ]; then
		ok=$((ok + 1))
	else
		fail=$((fail + 1))
		echo "SELF-TEST FAIL: tag on base should not block the heal" >&2
		cat "$tmp/err5" >&2
	fi
	git -C "$repo" tag -d base-tag

	# Rail: metacharacter + empty Task-Queue-ID — the hex validation
	# (3f9f497d) must refuse both with the same reason line.
	(cd "$repo" && "$self" --from origin/master 'dead;beef00000000000000000000000000') >/dev/null 2>"$tmp/err_meta"
	expect_refusal "metacharacter id" $? "invalid Task-Queue-ID" "$tmp/err_meta"
	(cd "$repo" && "$self" --from origin/master "") >/dev/null 2>"$tmp/err_empty_id"
	expect_refusal "empty id" $? "invalid Task-Queue-ID" "$tmp/err_empty_id"

	# Multi-branch backup-ref resolution (92c9193c): a stale
	# refs/original/refs/heads/<other> (sorting before the current branch's
	# backup, pointing at an older tree) must not hijack the resolution —
	# the TQ_HEAL_TEST_STALE_BACKUP seam recreates the sibling backup after
	# filter-branch's wipe, and the heal must still succeed.
	echo f >"$repo/f.txt"
	git -C "$repo" add f.txt
	git -C "$repo" commit -qm "chore: sweep four"
	(cd "$repo" && TQ_HEAL_TEST_STALE_BACKUP=aaa-stale "$self" --from origin/master deadbeef00000000000000000000000000000006) >/dev/null 2>"$tmp/err6"
	if [ "$?" = "0" ]; then
		ok=$((ok + 1))
	else
		fail=$((fail + 1))
		echo "SELF-TEST FAIL: multi-branch backup-ref resolution hijacked the heal" >&2
		cat "$tmp/err6" >&2
	fi

	# Session-scope footer ids (C12.4): heal must accept slug-shaped ids like
	# 20261011-b10-alert-routing-pack, not just hex task ids — a session's own
	# footered commits interleave with daemon sweeps, so the sweeps must heal
	# onto the session footer. Base at HEAD~1 keeps the hex-footered earlier
	# commits out of the range (a different-id footer stays a hard refusal).
	echo g >"$repo/g.txt"
	git -C "$repo" add g.txt
	git -C "$repo" commit -qm "chore: sweep five"
	local slug_old slug_new
	slug_old=$(git -C "$repo" rev-parse HEAD)
	(cd "$repo" && "$self" --from HEAD~1 20261011-b10-alert-routing-pack) >"$tmp/out_slug" 2>"$tmp/err_slug"
	if [ "$?" = "0" ] && footer_well_formed "$(git -C "$repo" log -1 --format='%B' HEAD)" "20261011-b10-alert-routing-pack"; then
		ok=$((ok + 1))
	else
		fail=$((fail + 1))
		echo "SELF-TEST FAIL: session-scope id must be accepted and baked" >&2
		cat "$tmp/err_slug" >&2
	fi
	slug_new=$(git -C "$repo" rev-parse HEAD)
	if [ "$slug_old" != "$slug_new" ] && grep -qF "FORK-RECORD: $slug_old→$slug_new" "$tmp/out_slug"; then
		ok=$((ok + 1))
	else
		fail=$((fail + 1))
		echo "SELF-TEST FAIL: session-scope heal fork record wrong" >&2
		cat "$tmp/out_slug" >&2
	fi

	# Silence when nothing cites the rewritten shas: the slug heal rewrites
	# a commit no doc cites, so no citation-check output may appear.
	if grep -q 'citation check' "$tmp/out_slug"; then
		fail=$((fail + 1))
		echo "SELF-TEST FAIL: citation check fired with no stale sites" >&2
		cat "$tmp/out_slug" >&2
	fi

	# Rail: a footer-less daemon commit BELOW a foreign-ID commit (the
	# 2026-10-13 01-54 shape) must refuse with the different-footer reason
	# — healing it would cascade the rewrite into the foreign commit's SHA.
	echo h >"$repo/h.txt"
	git -C "$repo" add h.txt
	git -C "$repo" commit -qm "chore: sweep below foreign"
	git -C "$repo" commit -q --allow-empty -m "$(printf 'work: foreign on top\n\nTask-Queue-ID: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa')"
	echo i >"$repo/i.txt"
	git -C "$repo" add i.txt
	git -C "$repo" commit -qm "chore: sweep above foreign"
	local below_foreign_base
	below_foreign_base=$(git -C "$repo" rev-parse HEAD~2)
	(cd "$repo" && "$self" --from "$below_foreign_base" deadbeef00000000000000000000000000000007) >/dev/null 2>"$tmp/err_below"
	expect_refusal "daemon commit below foreign-ID" $? "carries a different Task-Queue-ID footer" "$tmp/err_below"
	# Escape hatch: --from AT the foreign commit heals only the span above
	# it, preserving the foreign commit byte-identical.
	(cd "$repo" && "$self" --from HEAD~1 deadbeef00000000000000000000000000000007) >/dev/null 2>"$tmp/err_above"
	if [ "$?" = "0" ] && [ "$(git -C "$repo" log -1 --format='%B' HEAD~1)" = "$(printf 'work: foreign on top\n\nTask-Queue-ID: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa')" ]; then
		ok=$((ok + 1))
	else
		fail=$((fail + 1))
		echo "SELF-TEST FAIL: --from at foreign commit should heal only the span above" >&2
		cat "$tmp/err_above" >&2
	fi

	# --retitle (2026-10-17 8ee9930a class): healed sweeps take the
	# caller's true subject; footer-carrying commits in the range keep
	# theirs byte-identical.
	echo j >"$repo/j.txt"
	git -C "$repo" add j.txt
	git -C "$repo" commit -qm "chore: auto-commit"
	echo k >"$repo/k.txt"
	git -C "$repo" add k.txt
	git -C "$repo" commit -qm "work: footered stays"
	git -C "$repo" commit -q --amend -m "$(printf 'work: footered stays\n\nTask-Queue-ID: deadbeef00000000000000000000000000000001')"
	echo l >"$repo/l.txt"
	git -C "$repo" add l.txt
	git -C "$repo" commit -qm "chore: auto-commit two"
	local rbase
	rbase=$(git -C "$repo" rev-parse HEAD~3)
	(cd "$repo" && "$self" --from "$rbase" --retitle "feat: real work, retitled" deadbeef00000000000000000000000000000001) >"$tmp/out_retitle" 2>"$tmp/err_retitle"
	if [ "$?" = "0" ] &&
		[ "$(git -C "$repo" log --reverse --format='%s' "$rbase..HEAD" | sed -n 1p)" = "feat: real work, retitled" ] &&
		[ "$(git -C "$repo" log --reverse --format='%s' "$rbase..HEAD" | sed -n 2p)" = "work: footered stays" ] &&
		[ "$(git -C "$repo" log --reverse --format='%s' "$rbase..HEAD" | sed -n 3p)" = "feat: real work, retitled" ]; then
		ok=$((ok + 1))
	else
		fail=$((fail + 1))
		echo "SELF-TEST FAIL: --retitle subjects wrong" >&2
		cat "$tmp/err_retitle" >&2
	fi

	# Row 55 span-terminator: a fresh repo with [foreign footer] -> [my
	# footer-less sweep] -> [my target footer at HEAD]; the BARE form must
	# auto-narrow to the exactly-mine sweep instead of refusing on the
	# foreign footer below it (the 2026-10-12 01-41 shape).
	local repo2="$tmp/repo2"
	git init -q "$repo2"
	git -C "$repo2" config user.email t@t
	git -C "$repo2" config user.name t
	echo a >"$repo2/a.txt"
	git -C "$repo2" add a.txt
	git -C "$repo2" commit -qm "base"
	git -C "$repo2" update-ref refs/remotes/origin/master HEAD
	echo f >"$repo2/f.txt"
	git -C "$repo2" add f.txt
	git -C "$repo2" commit -qm $'chore: foreign sweep\n\nTask-Queue-ID: feedfacefeedfacefeedfeedfeedface'
	local foreign_sha
	foreign_sha=$(git -C "$repo2" rev-parse HEAD)
	echo m >"$repo2/m.txt"
	git -C "$repo2" add m.txt
	git -C "$repo2" commit -qm "chore: sweep mine"
	echo w >"$repo2/w.txt"
	git -C "$repo2" add w.txt
	git -C "$repo2" commit -qm $'fix: real work\n\nTask-Queue-ID: deadbeef00000000000000000000000000000002'
	(cd "$repo2" && "$self" deadbeef00000000000000000000000000000002) >"$tmp/out_term" 2>"$tmp/err_term" &&
		ok=$((ok + 1)) || {
		fail=$((fail + 1))
		echo "SELF-TEST FAIL: bare-form span-terminator exited non-zero" >&2
		cat "$tmp/err_term" >&2
	}
	if grep -qF "span-terminator" "$tmp/out_term" &&
		[ "$(git -C "$repo2" rev-parse HEAD~2)" = "$foreign_sha" ] &&
		footer_well_formed "$(git -C "$repo2" log -1 --format='%B' HEAD~1)" "deadbeef00000000000000000000000000000002" &&
		[ "$(git -C "$repo2" log -1 --format='%B' HEAD)" = "$(printf 'fix: real work\n\nTask-Queue-ID: deadbeef00000000000000000000000000000002')" ]; then
		ok=$((ok + 1))
	else
		fail=$((fail + 1))
		echo "SELF-TEST FAIL: span-terminator heal wrong (foreign commit must stay put; sweep footered; HEAD preserved)" >&2
		cat "$tmp/out_term" >&2
	fi

	# Row 56 refusal UX: TWO different-id footers in the range must refuse
	# AND print the exact --from invocation (the oldest blocker) so the
	# caller never has to eyeball git log.
	local repo3="$tmp/repo3"
	git init -q "$repo3"
	git -C "$repo3" config user.email t@t
	git -C "$repo3" config user.name t
	echo a >"$repo3/a.txt"
	git -C "$repo3" add a.txt
	git -C "$repo3" commit -qm "base"
	git -C "$repo3" update-ref refs/remotes/origin/master HEAD
	echo f1 >"$repo3/f1.txt"
	git -C "$repo3" add f1.txt
	git -C "$repo3" commit -qm $'chore: foreign one\n\nTask-Queue-ID: aaaa000000000000000000000000000001'
	local blocker1
	blocker1=$(git -C "$repo3" rev-parse HEAD)
	echo f2 >"$repo3/f2.txt"
	git -C "$repo3" add f2.txt
	git -C "$repo3" commit -qm $'chore: foreign two\n\nTask-Queue-ID: bbbb000000000000000000000000000002'
	echo m >"$repo3/m.txt"
	git -C "$repo3" add m.txt
	git -C "$repo3" commit -qm "chore: sweep mine"
	(cd "$repo3" && "$self" deadbeef00000000000000000000000000000003) >"$tmp/out_multi" 2>"$tmp/err_multi"
	expect_refusal "mixed span (two foreign footers)" $? "--from $blocker1" "$tmp/err_multi"

	if [ "$fail" = "0" ]; then
		echo "SELF-TEST OK ($ok checks)"
		return 0
	fi
	echo "SELF-TEST FAILED ($ok ok, $fail failed)" >&2
	return 1
}

if [ "${1:-}" = "--self-test" ]; then
	self_test
else
	main "$@"
fi
