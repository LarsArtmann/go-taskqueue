#!/usr/bin/env bash
# Dead-SHA citation gate: a 7-40-hex token in the living docs that resolves
# to a REAL git commit but is UNREACHABLE from HEAD means a history rewrite
# left a stale citation behind (the dd543ec class: it survived in TODO_LIST
# for a window after the msg-filter heal before 131ea17 fixed it).
#
# Scope: docs/status/*.md (top level + tasks/ closeout reports) +
# TODO_LIST.md + AGENTS.md.
# Hex tokens that are NOT commit objects (queue task IDs, CI run numbers,
# line counts) are ignored — only commits that exist but dangle fail.
#
# Escapes:
#   - fork-record arrow form: a token is escaped only when it IS a member
#     of a `<sha>→<sha>` pair (either arrow direction); a line may carry
#     SEVERAL comma-separated pairs (the heal tooling's paste format) —
#     per-token matching keeps a non-member dead token on such a line RED
#     (an unrelated arrow on the same line no longer whitewashes other
#     dead tokens on that line — 2026-10-05 tightening).
#   - baseline file scripts/dead-sha-baseline.txt (one sha per line, `#`
#     comments): the legacy citations from the 2026-09-10/2026-09-19
#     rewrites and later heal masses. Shrink is advisory-only; a NEW dead
#     sha not in the file fails the gate. Baselined hits are silent unless
#     TQ_DEAD_SHA_VERBOSE=1.
#
# --emit-baseline: generator mode (2026-10-05, owner ruling O16) — runs the
# same dead-token walk and prints a sorted, deduplicated baseline addendum
# on stdout (arrow-escaped tokens excluded; baseline membership ignored so
# regens are idempotent). Fork-record-vs-baseline curation stays human.
#
# --self-test pins the gate's decision branches against a /tmp fixture repo:
# clean run rc=0, non-baselined dead SHA rc=1 with exact file:line, per-token
# arrow escape (member passes, non-member on the same line still fails),
# --emit-baseline lists the dangling token, baseline-hit silent unless
# TQ_DEAD_SHA_VERBOSE=1.
set -uo pipefail

MODE="gate" # overridden to "emit" by --emit-baseline

# token_arrow_escaped <tok> <text>: rc=0 iff the token IS a member of an
# old→new fork-record pair on that line. Guards keep the token from
# matching inside a longer hex token.
token_arrow_escaped() {
	local tok=$1 text=$2
	# pure-bash ERE (no grep spawn): this runs per (line, token) pair —
	# spawned greps dominated the gate pass (§f9 perf)
	if [[ "$text" =~ (^|[^0-9a-f])${tok}(→|<-)[0-9a-f]{7,40} ]]; then return 0; fi
	if [[ "$text" =~ [0-9a-f]{7,40}(→|<-)${tok}([^0-9a-f]|$) ]]; then return 0; fi
	return 1
}

# run_gate / emit: cwd must be the fixture/real git repo root; caller sets
# the `baseline` path and the `files` array. gate: returns 1 on any new
# (non-baselined, non-arrow) dead SHA. emit: prints sorted dead tokens.
# hex_tokens TEXT: successive leftmost [0-9a-f]{7,40} matches, pure-bash
# regex (no grep spawn — this runs once per hex-bearing line, ~5.7k lines
# in the default scope; spawned greps dominated the pass, §f9 perf).
hex_tokens() {
	local t=$1
	local re='[0-9a-f]{7,40}'
	while [[ "$t" =~ $re ]]; do
		printf '%s\n' "${BASH_REMATCH[0]}"
		t=${t#*"${BASH_REMATCH[0]}"}
	done
}

run_gate() {
	local f line linenum text tok
	local fail=0 new_hits=0 baselined_hits=0
	local -A emit=() alltok=() oid=() anc=()
	# Phase 1: collect the unique hex tokens across the whole scope.
	for f in "${files[@]}"; do
		while IFS= read -r line; do
			for tok in $(hex_tokens "${line#*:}"); do alltok["$tok"]=1; done
		done < <(grep -nE '[0-9a-f]{7,40}' "$f")
	done
	# Phase 2 (BATCHED, 2026-11-03 §f9): ONE `git cat-file --batch-check`
	# (existence + resolved oid, order-preserving) plus ONE `git rev-list
	# HEAD` (ancestor set) replace the per-token `cat-file -t` +
	# `merge-base --is-ancestor` spawns — the gate pass itself was ~54 s
	# over the living-docs scope, now two invocations total.
	local -a toks=() chk=()
	local k i o typ
	for k in "${!alltok[@]}"; do toks+=("$k"); done
	if [ ${#toks[@]} -gt 0 ]; then
		mapfile -t chk < <(printf '%s\n' "${toks[@]}" | git cat-file --batch-check 2>/dev/null)
		for i in "${!toks[@]}"; do
			[ -n "${chk[$i]:-}" ] || continue
			o=${chk[$i]%% *}
			typ=${chk[$i]#* }
			typ=${typ%% *}
			[ "$typ" = commit ] && oid["${toks[$i]}"]=$o
		done
		while read -r o; do anc["$o"]=1; done < <(git rev-list HEAD 2>/dev/null)
	fi
	local -A bl_set=()
	if [ -f "$baseline" ]; then
		while IFS= read -r bl; do
			[[ "$bl" =~ ^# ]] && continue
			[ -z "$bl" ] && continue
			bl_set["$bl"]=1
		done <"$baseline"
	fi
	in_baseline() {
		[ -n "${bl_set[$1]:-}" ]
	}
	for f in "${files[@]}"; do
		while IFS= read -r line; do
			linenum=${line%%:*}
			text=${line#*:}
			local -A seen=()
		# per-OCCURRENCE scan (TODO §f12): the same token may appear twice
		# on a line — first as a fork-record member (healthy), then bare
		# (dangling). Arrow-escape is tested against a WINDOW around the
		# occurrence (±45 chars), so a pair elsewhere on the line cannot
		# whitewash a second bare cite; the dedup marks a token seen only
		# once CLASSIFIED (dead/baselined), never on a mere escape.
		local t=$text tok pos pre win
		while [[ "$t" =~ [0-9a-f]{7,40} ]]; do
			tok=${BASH_REMATCH[0]}
			t=${t#*"$tok"}
			pre=${text:0:$(( ${#text} - ${#t} ))}  # text up to and incl. tok
			pos=$((${#pre} - ${#tok}))
			win=${text:$((pos > 45 ? pos - 45 : 0)):$(( ${#tok} + 90 ))}
			[ -n "${seen[$tok]:-}" ] && continue
			[ -n "${oid[$tok]:-}" ] || continue
			[ -n "${anc[${oid[$tok]}]:-}" ] && continue
			token_arrow_escaped "$tok" "$win" && continue
			seen["$tok"]=1
				if [ "$MODE" = emit ]; then
					emit["$tok"]=1
					continue
				fi
				if in_baseline "$tok"; then
					baselined_hits=$((baselined_hits + 1))
					[ "${TQ_DEAD_SHA_VERBOSE:-0}" = 1 ] && echo "STALE (baselined): $f:$linenum cites '$tok' (commit exists, unreachable from HEAD)"
				else
					echo "DEAD SHA: $f:$linenum cites '$tok' (commit exists, unreachable from HEAD) — re-resolve, add a fork record (old→new), or baseline"
					fail=1
					new_hits=$((new_hits + 1))
				fi
			done
		done < <(grep -nE '[0-9a-f]{7,40}' "$f")
	done

	if [ "$MODE" = emit ]; then
		local k
		for k in "${!emit[@]}"; do echo "$k"; done | sort -u
		return 0
	fi
	if [ "$fail" = 0 ]; then
		echo "dead-sha refs ok (${baselined_hits} baselined, 0 new)"
	fi
	return "$fail"
}

# --shrink: baseline shrink pass (2026-11-03, TODO §f9 — advisory-only
# today, baseline only grows). For each baselined token, TREE-VERIFY whether
# a NOW-reachable counterpart exists (same tree hash + same subject) — i.e.
# the cited content survived the rewrite at a commit reachable from HEAD.
# Print KEEP/DROP verdicts + a summary. With --write, rewrite the baseline
# dropping only rows that BOTH have a reachable twin AND are no longer a
# needed escape (the token no longer appears in the gate's own dead-cite
# emit — arrow-escaped cites need no baseline row). A twinned token still
# cited bare is KEPT with a fork-record hint: dropping it alone would turn
# the gate RED (the cite points at the dead token, not the twin).
# --check: diff this pass's verdicts against the stored last-shrink snapshot
# (${baseline}.last-shrink, git-tracked) so silent drift/regression between
# runs is visible (2026-11-03, TODO 02-56 §b4: live shrink went 17→2 drops
# across runs and only manual accounting explained it). No snapshot yet →
# write one, rc=0. Drift (any token's verdict/twin changed) → print each
# changed row as NEWLY-TWINED / NO-LONGER-TWINED / VERDICT-CHANGED, rc=1.
# --write also refreshes the snapshot so the check re-arms green.
# --json: suppress the human verdict lines + summary and emit ONE
# machine-parseable JSON object instead (TODO 02-56 §f10):
#   {"shrink":{"total":N,"drops":D,"verdicts":[{"token":…,"verdict":"DROP|KEEP|TWINED","twin":…}]}}
# Verdict order = baseline order; KEEP rows carry "twin":null.
shrink_pass() {
	local tok tok_tree tok_subj twin total=0 drops=0 cand
	# needed escapes: dead non-arrow tokens currently cited in scope —
	# exactly the set the gate's emit mode reports.
	local -A needed=()
	local k
	MODE=emit
	while IFS= read -r k; do [ -n "$k" ] && needed["$k"]=1; done < <(run_gate)
	MODE=gate
	# reachable twins: tree|subject -> candidate shas (newline-separated).
	# Tiebreak, documented (TODO 02-56 §f6): commits with IDENTICAL tree and
	# subject (daemon re-sweeps, squash duplicates) collapse to ONE key; on
	# lookup the FIRST candidate wins, and git log is newest-first, so the
	# NEWEST reachable commit wins. For daemon tokens that is purely
	# cosmetic (identical tree = identical content); non-daemon tokens take
	# the body-verified branch below instead of trusting the order.
	local -A twins=()
	local sha tree subj
	while read -r sha tree subj; do
		twins["${tree}|${subj}"]="${twins[${tree}|${subj}]:-}${sha}"$'\n'
	done < <(git log --format='%H %T %s' HEAD 2>/dev/null)
	[ -f "$baseline" ] || {
		echo "shrink: no baseline at $baseline"
		return 0
	}
	local kept=()
	local -a verdicts=()
	local -a toks=()
	while IFS= read -r tok; do
		[[ "$tok" =~ ^# || -z "$tok" ]] && {
			[ -n "${write_mode:-}" ] && kept+=("$tok")
			continue
		}
		toks+=("$tok")
	done <"$baseline"
	# BATCHED metadata (2026-11-03 §f9): the per-token calls (tree, subject,
	# body) cost 3 process spawns × ~670 tokens ≈ 25 s per pass; ONE
	# `git cat-file --batch-check` (existence + resolved oid, order-preserving,
	# missing lines echo the input) plus ONE `git log --stdin --no-walk`
	# (NUL-framed %H/%T/%s/%B records) replace them with TWO invocations
	# total. Nonexistent tokens keep the old KEEP fallthrough (no meta).
	local -A full=() meta=()
	local -a valid=() chk=()
	if [ ${#toks[@]} -gt 0 ]; then
		mapfile -t chk < <(printf '%s\n' "${toks[@]}" | git cat-file --batch-check 2>/dev/null)
		local i oid typ
		for i in "${!toks[@]}"; do
			[ -n "${chk[$i]:-}" ] || continue
			oid=${chk[$i]%% *}
			typ=${chk[$i]#* }
			typ=${typ%% *}
			if [ "$typ" = commit ]; then
				valid+=("${toks[$i]}")
				full["${toks[$i]}"]=$oid
			fi
		done
		if [ ${#valid[@]} -gt 0 ]; then
			local osha otree osubj obody
			while read -r -d '' osha && read -r -d '' otree && read -r -d '' osubj && read -r -d '' obody; do
				meta["$osha"]="$otree"$'\n'"$osubj"$'\n'"$obody"
			done < <(printf '%s\n' "${valid[@]}" | git log --no-walk=unsorted --format='%H%x00%T%x00%s%x00%B%x00' --stdin 2>/dev/null)
		fi
	fi
	local -a json=()
	local m rest tok tok_tree tok_subj tok_body fsha
	for tok in "${toks[@]}"; do
		total=$((total + 1))
		tok_tree="" tok_subj="" tok_body=""
		fsha=${full[$tok]:-}
		if [ -n "$fsha" ] && [ -n "${meta[$fsha]:-}" ]; then
			m=${meta[$fsha]}
			tok_tree=${m%%$'\n'*}
			rest=${m#*$'\n'}
			tok_subj=${rest%%$'\n'*}
			tok_body=${rest#*$'\n'}
		fi
		twin=""
		if [ -n "$tok_tree" ] && [ -n "${twins[${tok_tree}|${tok_subj}]:-}" ]; then
			if grep -q '^Task-Queue-ID:' <<<"$tok_body"; then
				# daemon commit: the body is queue boilerplate keyed by the
				# task id, so tree+subject suffices; newest candidate wins
				# (tiebreak above)
				twin="${twins[${tok_tree}|${tok_subj}]%%$'\n'*}"
			else
				# non-daemon (TODO 02-56 §f21): verify the message BODY too —
				# a tree+subject twin with a different body is a DIFFERENT
				# commit; only a body-exact candidate counts as the twin
				for cand in ${twins[${tok_tree}|${tok_subj}]}; do
					[ -n "$cand" ] || continue
					if [ "$(git log -1 --format=%B "$cand" 2>/dev/null)" = "$tok_body" ]; then
						twin="$cand"
						break
					fi
				done
			fi
		fi
		if [ -n "$twin" ] && [ "$twin" != "$tok" ]; then
			if [ -n "${needed[$tok]:-}" ]; then
				[ -z "${json_mode:-}" ] && echo "TWINED (still cited — fork-record ${tok}→${twin} then re-shrink): $tok"
				verdicts+=("TWINED $tok $twin")
				json+=("{\"token\":\"$tok\",\"verdict\":\"TWINED\",\"twin\":\"$twin\"}")
				[ -n "${write_mode:-}" ] && kept+=("$tok")
			else
				[ -z "${json_mode:-}" ] && echo "DROP: $tok → $twin (twin-verified: tree + subject + body for non-daemon, reachable from HEAD, no longer cited)"
				verdicts+=("DROP $tok $twin")
				json+=("{\"token\":\"$tok\",\"verdict\":\"DROP\",\"twin\":\"$twin\"}")
				drops=$((drops + 1))
			fi
		else
			[ -z "${json_mode:-}" ] && echo "KEEP: $tok"
			verdicts+=("KEEP $tok")
			json+=("{\"token\":\"$tok\",\"verdict\":\"KEEP\",\"twin\":null}")
			[ -n "${write_mode:-}" ] && kept+=("$tok")
		fi
	done
	if [ -n "${json_mode:-}" ]; then
		printf '{"shrink":{"total":%d,"drops":%d,"verdicts":[%s]}}\n' "$total" "$drops" "$(IFS=,; [ ${#json[@]} -gt 0 ] && echo "${json[*]}")"
		return 0
	fi
	echo "shrink: $drops of $total baselined tokens have a now-reachable twin (advisory; gate stays green throughout)"
	# --write lands FIRST: --check then verifies the (possibly just-refreshed)
	# state, so --shrink --check --write re-arms green even on drift.
	if [ -n "${write_mode:-}" ] && [ "$drops" -gt 0 ]; then
		printf '%s\n' "${kept[@]}" >"$baseline"
		echo "shrink: rewrote $baseline ($total → $((${total} - drops)) lines)"
	fi
	if [ -n "${write_mode:-}" ] && [ -n "${check_mode:-}" ]; then
		printf '%s\n' "$(printf '%s\n' "${verdicts[@]}" | sort)" >"${baseline}.last-shrink"
		echo "shrink-check: refreshed the last-shrink snapshot"
	fi
	if [ -n "${check_mode:-}" ]; then
		snapshot="${baseline}.last-shrink"
		current=$(printf '%s\n' "${verdicts[@]}" | sort)
		if [ ! -f "$snapshot" ]; then
			printf '%s\n' "$current" >"$snapshot"
			echo "shrink-check: no last-shrink snapshot — wrote first one ($total rows)"
			return 0
		fi
		stored=$(grep -vE '^#|^$' "$snapshot" | sort)
		drift=$(diff <(printf '%s\n' "$stored") <(printf '%s\n' "$current") | grep -E '^[<>]' || true)
		if [ -z "$drift" ]; then
			echo "shrink-check: ok — verdicts match the last-shrink snapshot ($total rows)"
			return 0
		fi
		echo "shrink-check: DRIFT vs $(basename "$snapshot") — shrink verdicts changed between runs:"
		printf '%s\n' "$drift"
		return 1
	fi
	return 0
}

# --self-test: /tmp fixture git repo with one dangling commit (a commit-tree
# child of HEAD — real commit object, unreachable, no git reset needed).
if [ "${1:-}" = "--self-test" ]; then
	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' EXIT
	mkdir -p "$tmp/docs/status"
	git init -q "$tmp"
	git -C "$tmp" -c user.email=t@t -c user.name=t commit --allow-empty -q -m base
	old=$(git -C "$tmp" -c user.email=t@t -c user.name=t commit-tree 'HEAD^{tree}' -p HEAD -m dangling)
	other=$(git -C "$tmp" -c user.email=t@t -c user.name=t commit-tree 'HEAD^{tree}' -p HEAD -m dangling2)
	third=$(git -C "$tmp" -c user.email=t@t -c user.name=t commit-tree 'HEAD^{tree}' -p HEAD -m dangling3)
	new=$(git -C "$tmp" rev-parse HEAD)

	# pins: baselined-doc-branch — fork-playbook line counts as baseline hit 1, rc=0
	cat >"$tmp/AGENTS.md" <<EOF
fork playbook: history rewrote ${old}→${new}
EOF
	# pins: baselined-verbose-branch — backlog citation is baseline hit 2; printed only under TQ_DEAD_SHA_VERBOSE
	cat >"$tmp/TODO_LIST.md" <<EOF
legacy citation survives in the backlog: ${old}
EOF
	# pins: new-dead-sha-branch — non-baselined citation must rc=1 with exact file:line (also the --emit-baseline token source)
	cat >"$tmp/docs/status/report.md" <<EOF
cites ${old} for context (rewrite casualty)
EOF
	# pins: line-level-arrow-branch — dead token sharing a line with an unrelated arrow must still rc=1 (no whitewash)
	cat >"$tmp/docs/status/mixed.md" <<EOF
unrelated arrow on the line: ${old} plus a playbook ${other}→${new}
EOF
	# pins: arrow-pair-branch — proper old→new fork-record pair is exempt, rc=0
	cat >"$tmp/docs/status/paired.md" <<EOF
proper pair: ${old}→${new}
EOF
	# pins: multipair-branch — comma-separated multi-pair fork-record lines
	# (the heal tooling's paste format) exempt every MEMBER token, rc=0;
	# a NON-member dead token sharing such a line still fails (per-token).
	second_new=$(git -C "$tmp" rev-parse HEAD)
	# pins: multipair-member-exempt — every pair MEMBER token on a heal line is exempt
	cat >"$tmp/docs/status/multipair.md" <<EOF
heal chain: \`${old}→${new}\`, \`${other}→${second_new}\`. HEAL OK, zero data loss.
EOF
	printf '%s\n' "# baseline" "$old" >"$tmp/base.txt"

	baseline="$tmp/base.txt"
	files=(AGENTS.md TODO_LIST.md docs/status/report.md)

	# 1. Baselined dead SHA + arrow line: clean, rc=0, hit counted, not printed.
	out=$(cd "$tmp" && run_gate) && rc=0 || rc=$?
	if [ "$rc" != 0 ] || ! grep -q 'dead-sha refs ok (2 baselined, 0 new)' <<<"$out"; then
		echo "dead-sha self-test FAIL: clean/baselined run rc=$rc out: $out"
		exit 1
	fi

	# 2. Baselined hit silent without the verbose flag, printed with it.
	out=$(cd "$tmp" && run_gate)
	grep -q 'STALE (baselined)' <<<"$out" && {
		echo "dead-sha self-test FAIL: baselined hit printed without TQ_DEAD_SHA_VERBOSE"
		exit 1
	}
	out=$(cd "$tmp" && TQ_DEAD_SHA_VERBOSE=1 run_gate)
	grep -q "STALE (baselined): TODO_LIST.md:1 cites '${old}'" <<<"$out" || {
		echo "dead-sha self-test FAIL: verbose flag did not surface TODO_LIST.md:1"
		exit 1
	}

	# 3. Non-baselined dead SHA: rc=1 with exact file:line.
	grep -v "^${old}$" "$tmp/base.txt" >"$tmp/base2.txt" && mv "$tmp/base2.txt" "$tmp/base.txt"
	out=$(cd "$tmp" && run_gate) && rc=0 || rc=$?
	if [ "$rc" != 1 ] || ! grep -q "DEAD SHA: docs/status/report.md:1 cites '${old}'" <<<"$out"; then
		echo "dead-sha self-test FAIL: non-baselined run rc=$rc out: $out"
		exit 1
	fi

	# 4. Per-token arrow escape: a token that IS a pair member passes, but a
	# dead token merely SHARING the line with an unrelated arrow still fails.
	files=(docs/status/paired.md)
	out=$(cd "$tmp" && run_gate) && rc=0 || rc=$?
	[ "$rc" = 0 ] || {
		echo "dead-sha self-test FAIL: proper arrow pair whitewashed: $out"
		exit 1
	}
	files=(docs/status/mixed.md)
	out=$(cd "$tmp" && run_gate) && rc=0 || rc=$?
	if [ "$rc" != 1 ] || ! grep -q "DEAD SHA: docs/status/mixed.md:1 cites '${old}'" <<<"$out"; then
		echo "dead-sha self-test FAIL: line-level arrow still whitewashes unrelated dead tokens: rc=$rc out: $out"
		exit 1
	fi

	# 4b. Comma-separated multi-pair fork-record lines: every MEMBER token
	# passes (rc=0); a NON-member dead token on the same line still fails.
	files=(docs/status/multipair.md)
	out=$(cd "$tmp" && run_gate) && rc=0 || rc=$?
	[ "$rc" = 0 ] || {
		echo "dead-sha self-test FAIL: comma-separated multi-pair fork-record line flagged: $out"
		exit 1
	}
	# pins: stray-member-fails — member token outside any pair still DEAD SHA
	cat >>"$tmp/docs/status/multipair.md" <<EOF
stray member on the record line: ${third} not in any pair
EOF
	out=$(cd "$tmp" && run_gate) && rc=0 || rc=$?
	if [ "$rc" != 1 ] || ! grep -q "DEAD SHA: docs/status/multipair.md:2 cites '${third}'" <<<"$out"; then
		echo "dead-sha self-test FAIL: multi-pair line whitewashed a non-member dead token: rc=$rc out: $out"
		exit 1
	fi

	# 5. --emit-baseline: sorted dead tokens (pair members excluded), then
	# feeding the emit output into the baseline makes the gate green.
	files=(docs/status/report.md docs/status/mixed.md docs/status/paired.md)
	MODE=emit
	out=$(cd "$tmp" && run_gate) && rc=0 || rc=$?
	[ "$rc" = 0 ] || {
		echo "dead-sha self-test FAIL: emit mode rc=$rc"
		exit 1
	}
	[ "$(grep -c '^[0-9a-f]\{7,40\}$' <<<"$out")" = 1 ] || {
		echo "dead-sha self-test FAIL: emit expected exactly 1 token (pair member excluded): $out"
		exit 1
	}
	grep -q "^${old}$" <<<"$out" || {
		echo "dead-sha self-test FAIL: emit missed the dangling token: $out"
		exit 1
	}
	grep -q "^${other}$" <<<"$out" && {
		echo "dead-sha self-test FAIL: emit included an arrow-pair member: $out"
		exit 1
	}
	{
		echo "# emitted"
		cat <<<"$out"
	} >"$tmp/base.txt"
	files=(docs/status/mixed.md)
	MODE=gate
	out=$(cd "$tmp" && run_gate) && rc=0 || rc=$?
	[ "$rc" = 0 ] || {
		echo "dead-sha self-test FAIL: emitted baseline did not green the gate: $out"
		exit 1
	}

	# 6. --shrink: a baselined dangling token WITH a reachable twin (same
	# tree + subject) drops; a token without one keeps; --write rewrites the
	# baseline dropping only DROP rows and the gate stays green.
	baseline="$tmp/base.txt"
	old_tree=$(git -C "$tmp" log -1 --format='%T' "$old")
	printf '%s\n' "$third" >>"$tmp/base.txt"
	twin=$(GIT_AUTHOR_DATE='2020-01-01T00:00:00' GIT_COMMITTER_DATE='2020-01-01T00:00:00' \
		git -C "$tmp" -c user.email=t@t -c user.name=t commit-tree "$old_tree" -p HEAD -m dangling)
	git -C "$tmp" merge -q --no-ff -m "merge twin for shrink self-test" "$twin"
	# scope: paired.md carries old only as an arrow MEMBER (no bare cite),
	# so old is not a needed escape and the twin lets its row drop; third
	# has no twin and keeps. A still-bare-cited twinned token (mixed.md)
	# is covered by the TWINED branch pinned right after.
	files=(docs/status/paired.md)
	out=$(cd "$tmp" && shrink_pass)
	grep -q "^DROP: ${old} → " <<<"$out" || {
		echo "dead-sha self-test FAIL: shrink did not drop the twinned token: $out"
		exit 1
	}
	grep -q "^KEEP: ${third}$" <<<"$out" || {
		echo "dead-sha self-test FAIL: shrink kept-row verdict missing: $out"
		exit 1
	}
	grep -qF 'shrink: 1 of 2 baselined tokens' <<<"$out" || {
		echo "dead-sha self-test FAIL: shrink summary wrong: $out"
		exit 1
	}
	files=(docs/status/mixed.md)
	out=$(cd "$tmp" && shrink_pass)
	grep -q "^TWINED (still cited" <<<"$out" || {
		echo "dead-sha self-test FAIL: bare-cited twinned token not held as TWINED: $out"
		exit 1
	}
	write_mode=1
	files=(docs/status/paired.md)
	out=$(cd "$tmp" && shrink_pass)
	[ "$(grep -c '^[0-9a-f]' "$tmp/base.txt")" = 1 ] || {
		echo "dead-sha self-test FAIL: --write left wrong baseline: $(cat "$tmp/base.txt")"
		exit 1
	}
	write_mode=
	files=(AGENTS.md)
	out=$(cd "$tmp" && run_gate) && rc=0 || rc=$?
	[ "$rc" = 0 ] || {
		echo "dead-sha self-test FAIL: gate red after shrink --write: $out"
		exit 1
	}

	# 7. twin-map collision + body tiebreak (TODO 02-56 §f6/§f21): two
	# reachable commits with IDENTICAL tree+subject collapse to one twins
	# key — a non-daemon token must pick the BODY-exact twin (not the
	# last-wins order winner), a body-less-matching token keeps, and a
	# daemon token still twins on tree+subject alone (body is boilerplate).
	twin2=$(GIT_AUTHOR_DATE='2020-01-02T00:00:00' GIT_COMMITTER_DATE='2020-01-02T00:00:00' \
		git -C "$tmp" -c user.email=t@t -c user.name=t commit-tree "$old_tree" -p HEAD -m dangling -m "twin2 body variant")
	git -C "$tmp" merge -q --no-ff -m "merge twin2 (same tree+subject, different body)" "$twin2"
	dtok=$(git -C "$tmp" -c user.email=t@t -c user.name=t commit-tree "$old_tree" -p HEAD -m "dangling daemon" -m "Task-Queue-ID: 0000000000000000000000000000000000000000")
	dtwin=$(GIT_AUTHOR_DATE='2020-01-03T00:00:00' GIT_COMMITTER_DATE='2020-01-03T00:00:00' \
		git -C "$tmp" -c user.email=t@t -c user.name=t commit-tree "$old_tree" -p HEAD -m "dangling daemon" -m "DIFFERENT daemon body")
	git -C "$tmp" merge -q --no-ff -m "merge daemon twin" "$dtwin"
	tok3=$(git -C "$tmp" -c user.email=t@t -c user.name=t commit-tree "$old_tree" -p HEAD -m dangling -m "unrelated body no twin")
	printf '%s\n' "$old" "$dtok" "$tok3" >"$tmp/base.txt"
	files=(docs/status/paired.md)
	out=$(cd "$tmp" && shrink_pass)
	grep -qF "DROP: ${old} → ${twin}" <<<"$out" || {
		echo "dead-sha self-test FAIL: collision did not pick the body-exact twin (want ${twin}): $out"
		exit 1
	}
	grep -qF "DROP: ${dtok} → ${dtwin}" <<<"$out" || {
		echo "dead-sha self-test FAIL: daemon token did not twin on tree+subject alone (want ${dtwin}): $out"
		exit 1
	}
	grep -q "^KEEP: ${tok3}$" <<<"$out" || {
		echo "dead-sha self-test FAIL: body-less token wrongly twinned: $out"
		exit 1
	}
	grep -qF 'shrink: 2 of 3 baselined tokens' <<<"$out" || {
		echo "dead-sha self-test FAIL: collision summary wrong: $out"
		exit 1
	}

	# 8. --shrink --check: last-shrink snapshot diff (TODO 02-56 §b4) —
	# first run writes the snapshot rc=0; a verdict change (KEEP→DROP) is
	# DRIFT rc=1 naming the token; --write refreshes the snapshot and the
	# check re-arms green.
	check_mode=1
	files=(docs/status/paired.md)
	out=$(cd "$tmp" && shrink_pass) && rc=0 || rc=$?
	if [ "$rc" != 0 ] || ! grep -q 'shrink-check: no last-shrink snapshot' <<<"$out"; then
		echo "dead-sha self-test FAIL: shrink --check first run did not seed the snapshot: rc=$rc out: $out"
		exit 1
	fi
	tok3twin=$(GIT_AUTHOR_DATE='2020-01-04T00:00:00' GIT_COMMITTER_DATE='2020-01-04T00:00:00' \
		git -C "$tmp" -c user.email=t@t -c user.name=t commit-tree "$old_tree" -p HEAD -m dangling -m "unrelated body no twin")
	git -C "$tmp" merge -q --no-ff -m "merge tok3 twin" "$tok3twin"
	out=$(cd "$tmp" && shrink_pass) && rc=0 || rc=$?
	if [ "$rc" != 1 ] || ! grep -q "^> DROP ${tok3} " <<<"$out"; then
		echo "dead-sha self-test FAIL: shrink --check missed the verdict drift on ${tok3}: rc=$rc out: $out"
		exit 1
	fi
	write_mode=1
	out=$(cd "$tmp" && shrink_pass) && rc=0 || rc=$?
	out=$(cd "$tmp" && shrink_pass) && rc=0 || rc=$?
	if [ "$rc" != 0 ] || ! grep -q 'shrink-check: ok' <<<"$out"; then
		echo "dead-sha self-test FAIL: shrink --check not green after --write refresh: rc=$rc out: $out"
		exit 1
	fi
	check_mode=
	write_mode=

	# 9. --shrink --json (TODO 02-56 §f10): machine output only — one JSON
	# object, verdicts in baseline order, DROP carries the twin, KEEP
	# carries "twin":null, no human lines.
	printf '%s\n' "$tok3" "$third" >"$tmp/base.txt"
	json_mode=1
	files=(docs/status/paired.md)
	out=$(cd "$tmp" && shrink_pass)
	json_ok=1
	grep -qF "{\"shrink\":{\"total\":2,\"drops\":1,\"verdicts\":" <<<"$out" || json_ok=0
	grep -qF "{\"token\":\"$tok3\",\"verdict\":\"DROP\",\"twin\":\"$tok3twin\"}" <<<"$out" || json_ok=0
	grep -qF "{\"token\":\"$third\",\"verdict\":\"KEEP\",\"twin\":null}" <<<"$out" || json_ok=0
	[ "$(grep -cE '^(DROP|KEEP|TWINED|shrink):' <<<"$out")" = 0 ] || json_ok=0
	if command -v jq >/dev/null 2>&1; then
		jq -e '.shrink.total == 2 and .shrink.drops == 1' <<<"$out" >/dev/null || json_ok=0
	fi
	[ "$json_ok" = 1 ] || {
		echo "dead-sha self-test FAIL: --json output wrong: $out"
		exit 1
	}
	json_mode=

	# 10. twice-on-a-line dangling token (TODO §f12): the token is a
	# fork-record MEMBER at its first occurrence (healthy) and cited BARE at
	# its second (dangling) — the per-line dedup must not swallow the bare
	# cite just because the first occurrence escaped via the arrow.
	cat >"$tmp/docs/status/twice.md" <<EOF
first as a pair member: ${old}→${new} — then cited bare again: ${old}
EOF
	files=(docs/status/twice.md)
	out=$(cd "$tmp" && run_gate) && rc=0 || rc=$?
	if [ "$rc" != 1 ] || ! grep -q "DEAD SHA: docs/status/twice.md:1 cites '${old}'" <<<"$out"; then
		echo "dead-sha self-test FAIL: twice-on-a-line dangling cite swallowed by the first-occurrence arrow escape: rc=$rc out: $out"
		exit 1
	fi
	# healthy-side inverse: both occurrences clean → green
	cat >"$tmp/docs/status/twice-ok.md" <<EOF
pair first: ${old}→${new} — bare but reachable: ${new}
EOF
	files=(docs/status/twice-ok.md)
	out=$(cd "$tmp" && run_gate) && rc=0 || rc=$?
	[ "$rc" = 0 ] || {
		echo "dead-sha self-test FAIL: twice-on-a-line healthy pair flagged: rc=$rc out: $out"
		exit 1
	}

	# 11. baselined-token-on-arrow-line precedence (TODO §f13): a baselined
	# token on a line ONLY as a fork-record member is escaped by the arrow
	# BEFORE the baseline branch — no baselined hit, gate green (arrow
	# escape outranks baseline membership); the BARE baselined cite still
	# rides the baseline branch and counts a hit.
	printf '%s\n' "$old" >"$tmp/base.txt"
	cat >"$tmp/AGENTS.md" <<EOF
fork playbook: history rewrote ${old}→${new}
EOF
	files=(AGENTS.md)
	out=$(cd "$tmp" && run_gate) && rc=0 || rc=$?
	if [ "$rc" != 0 ] || ! grep -q 'dead-sha refs ok (0 baselined, 0 new)' <<<"$out"; then
		echo "dead-sha self-test FAIL: baselined token on an arrow line must be arrow-escaped, not a baselined hit: rc=$rc out: $out"
		exit 1
	fi
	cat >"$tmp/AGENTS.md" <<EOF
legacy bare cite survives: ${old}
EOF
	out=$(cd "$tmp" && run_gate) && rc=0 || rc=$?
	if [ "$rc" != 0 ] || ! grep -q 'dead-sha refs ok (1 baselined, 0 new)' <<<"$out"; then
		echo "dead-sha self-test FAIL: bare baselined cite did not count a baselined hit: rc=$rc out: $out"
		exit 1
	fi

	echo "dead-sha self-test ok (clean rc=0, exact file:line on rc=1, per-token arrow escape, comma-separated multi-pair fork-record lines, emit-baseline sorted+pair-excluded, shrink drop/keep/--write, twin-map collision body tiebreak + daemon tree+subject path, shrink --check snapshot drift, shrink --json machine output, twice-on-a-line dangling token, baselined-on-arrow-line precedence, baselined silent unless TQ_DEAD_SHA_VERBOSE=1)"
	exit 0
fi

if [ "${1:-}" = "--shrink" ]; then
	shift
	while [ "$#" -gt 0 ]; do
		case "$1" in
		--write) write_mode=1 ;;
		--check) check_mode=1 ;;
		--json) json_mode=1 ;;
		esac
		shift
	done
	SHRINK=1
fi

cd "$(dirname "$0")/.." || exit 1

# TQ_DEAD_SHA_BASELINE overrides the curated baseline path (the env var keeps
# the curated file untouched; the hook serves fixture self-tests and a
# LOCAL receipt run against a scratch baseline — never a default change).
baseline="${TQ_DEAD_SHA_BASELINE:-scripts/dead-sha-baseline.txt}"

# Explicit file arguments (repo-root-relative) scope the gate to exactly
# those files — the verify-battery's own-file leg (scripts/verify-battery.sh) passes the
# window's touched files so a NEW dead cite outside the default scope
# (docs/planning/, docs/adr/, scripts docs …) still fails loudly. No args
# keeps the default living-docs scope.
if [ "${1:-}" = "--emit-baseline" ]; then
	MODE="emit"
	shift
fi
if [ "${1:-}" = "--shrink" ]; then
	shift
	while [ "$#" -gt 0 ]; do
		case "$1" in
		--write) write_mode=1 ;;
		--check) check_mode=1 ;;
		--json) json_mode=1 ;;
		esac
		shift
	done
fi

if [ "$#" -gt 0 ]; then
	files=()
	for f in "$@"; do
		[ -f "$f" ] && files+=("$f")
	done
else
	files=(AGENTS.md TODO_LIST.md)
	for f in docs/status/*.md docs/status/tasks/*.md; do [ -f "$f" ] && files+=("$f"); done
fi

if [ -n "${SHRINK:-}" ]; then
	shrink_pass
	exit $?
fi

run_gate
