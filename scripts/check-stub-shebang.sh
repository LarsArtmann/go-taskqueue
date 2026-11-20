#!/usr/bin/env bash
# scripts/check-stub-shebang.sh — every heredoc in the smoke corpus that
# embeds a shell shebang must carry an `#!/bin/sh` shebang as the FIRST
# heredoc body line (flags after the interpreter, e.g. `#!/bin/sh -e`, are
# LEGAL: the kernel passes the remainder as one interpreter argument — the
# invariant is first-line POSITION, not exact-match). Born from the 2026-11-18 sixth-window incident (report §e2): a
# repair left a BLANK FIRST HEREDOC LINE, so the generated stub began with
# a newline before `#!/bin/sh` and every run died with
# `fork/exec: exec format error` while the script looked right.
#
# Heredoc classification: a heredoc whose body contains a `#!/bin/sh` line
# is an sh stub and MUST have it as the FIRST body line; a heredoc with no
# shebang at all (prompts, configs, JSON) or a NON-sh first-line shebang
# (#!/usr/bin/env python3 stubs) is out of scope. A BLANK first body line
# before any shebang is also a violation — the exact incident shape.
#
# Scope: every tracked *.sh under scripts/smoke/ (the stub generators,
# including the shared scripts/smoke/lib/stub-agent.sh).
#
# --self-test pins the summary line + a negative-path battery via the
# TQ_STUB_SHEBANG_SCOPE hook (fleet convention).
set -euo pipefail
cd "$(dirname "$0")/.."

self_abs="$(cd "$(dirname "$0")" && pwd)/$(basename "$0")"

if [ "${1:-}" = "--self-test" ]; then
	set -euo pipefail
	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' EXIT
	# positive: #!/bin/sh IS the first body line (counted, ok)
	printf 'cat >"$STUB" <<EOF\n#!/bin/sh\nexit 0\nEOF\n' >"$tmp/good.sh"
	# negative 1: the incident shape — blank first body line before #!/bin/sh
	printf 'cat >"$STUB" <<EOF\n\n#!/bin/sh\nexit 0\nEOF\n' >"$tmp/blank.sh"
	# negative 2: shebang present but NOT the first line
	printf 'cat >"$STUB" <<EOF\nset -eu\n#!/bin/sh\nexit 0\nEOF\n' >"$tmp/misplaced.sh"
	# negative 3: blank first body line before a NON-sh shebang (python stub of the same class)
	printf 'cat >"$STUB" <<EOF\n\n#!/usr/bin/env python3\nexit 0\nEOF\n' >"$tmp/blankpy.sh"
	# out of scope: no shebang anywhere (prompt/config heredoc)
	printf 'cat >"$CFG" <<EOF\nkey=value\nEOF\n' >"$tmp/nonstub.sh"
	# out of scope: non-sh first-line shebang (python stub, correct form)
	printf 'cat >"$PYSTUB" <<EOF\n#!/usr/bin/env python3\nprint("hi")\nEOF\n' >"$tmp/pystub.sh"
	# out of scope: a COMMENT line carrying a heredoc opener (a usage example
	# like `# cat <<EOF` must not parse as a heredoc — f9, 2026-11-19)
	printf '# usage: cat >"$STUB" <<EOF\n# #!/bin/sh\n# EOF\ncat >"$STUB" <<EOF\n#!/bin/sh\nexit 0\nEOF\n' >"$tmp/comment.sh"
	# f45 policy pin: flags after the interpreter are a LEGAL first line
	# (kernel passes the remainder as one interpreter argument)
	printf 'cat >"$STUB" <<EOF\n#!/bin/sh -e\nexit 0\nEOF\n' >"$tmp/flagenok.sh"
	# row 1728: the flags POSITIVE fixture's paired REJECT shape — a flags
	# shebang misplaced into body[1:] must still VIOLATE via is_sh_shebang
	# (the classifier applies to both the first-line OK and misplaced paths)
	printf 'cat >"$STUB" <<EOF\nset -eu\n#!/bin/sh -e\nexit 0\nEOF\n' >"$tmp/misplacedflags.sh"
	# row 1737: a MULTI-CLUSTER flags line ("-e -x" — one kernel arg with a
	# space, execs as "invalid option" — verified live) must REJECT on the
	# first-line path too
	printf 'cat >"$STUB" <<EOF\n#!/bin/sh -e -x\nexit 0\nEOF\n' >"$tmp/multicluster.sh"
	printf 'cat >"$STUB" <<EOF\nset -eu\n#!/bin/sh -e -x\nexit 0\nEOF\n' >"$tmp/misplacedmulticluster.sh"
	# <<- heredoc: TAB-stripped body + TAB-indented closing delimiter must
	# still terminate and count the tab-stripped #!/bin/sh as first line (f23)
	printf 'main() {\n\tcat >"$STUB" <<-EOF\n\t#!/bin/sh\n\texit 0\n\tEOF\n}\n' >"$tmp/dash.sh"

	out="$(TQ_STUB_SHEBANG_SCOPE="$tmp/*.sh" "$self_abs" 2>&1)" && rc=0 || rc=$?
	[ "$rc" -eq 1 ] || {
		echo "stub-shebang self-test FAIL: negative battery rc=$rc (want 1): $out"
		exit 1
	}
	for shape in "$tmp/blank.sh" "$tmp/misplaced.sh" "$tmp/blankpy.sh" "$tmp/misplacedflags.sh" "$tmp/multicluster.sh" "$tmp/misplacedmulticluster.sh"; do
		grep -qF "$shape" <<<"$out" || {
			echo "stub-shebang self-test FAIL: FAIL line does not name $shape: $out"
			exit 1
		}
	done
	grep -qF "4 stub heredoc ok / 10 checked" <<<"$out" || {
		echo "stub-shebang self-test FAIL: summary shape wrong: $out"
		exit 1
	}
	out="$(TQ_STUB_SHEBANG_SCOPE="$tmp/good.sh $tmp/nonstub.sh $tmp/pystub.sh $tmp/comment.sh $tmp/dash.sh $tmp/flagenok.sh" "$self_abs" 2>&1)" && rc=0 || rc=$?
	[ "$rc" -eq 0 ] || {
		echo "stub-shebang self-test FAIL: positive battery rc=$rc (want 0): $out"
		exit 1
	}
	grep -qF "4 stub heredoc ok / 4 checked" <<<"$out" || {
		echo "stub-shebang self-test FAIL: green summary wrong: $out"
		exit 1
	}
	echo "stub-shebang self-test ok (summary pin + blank-first-line x2 + misplaced-shebang negatives incl. flags-form + multi-cluster flags-form reject x2 (row 1737) + non-stub/non-sh exclusions + comment-line opener exclusion + <<- tab-stripped delimiter + flags-after-interpreter legal)"
	exit 0
fi

scope="${TQ_STUB_SHEBANG_SCOPE:-}"
if [ -z "$scope" ]; then
	scope="$(find scripts/smoke -name '*.sh' -print | sort)"
fi

viol=0 checked=0
# shellcheck disable=SC2086 # word-splitting IS the scope list (find lines / glob words)
for f in $scope; do
	while IFS=$'\t' read -r kind first; do
		[ -n "$kind" ] || continue
		checked=$((checked + 1))
		[ "$kind" = "VIOLATION" ] || continue
		echo "FAIL: $f heredoc body does not start with an #!/bin/sh shebang (first body line: ${first:-<blank>}) — the exec-format-error class (2026-11-18 sixth window §e2: a blank first heredoc line fork/exec-failed every run while the script looked right)" >&2
		viol=$((viol + 1))
	done < <(
		python3 - "$f" <<'PYEOF'
import re, sys

def is_sh_shebang(line):
    # row 1737 (2026-11-20_01-14 closeout §g1): the kernel hands the
    # interpreter AT MOST ONE argument — everything after the first space
    # in the shebang line arrives as a single string. A MULTI-CLUSTER flag
    # line ("#!/bin/sh -e -x") therefore execs as sh "-e -x" → "invalid
    # option" (verified live on this host, dash) — the single-cluster form
    # ("-e", "-ex") is the only legal flags shape, so the classifier
    # accepts ONE flag cluster, never a space-separated series.
    return line == "#!/bin/sh" or re.match(r"^#!/bin/sh +-[A-Za-z]+$", line) is not None

path = sys.argv[1]
start_re = re.compile(r"<<-?[ \t]*([\"']?)([A-Za-z_][A-Za-z0-9_]*)\1")
lines = open(path, encoding="utf-8", errors="replace").read().split("\n")
i = 0
while i < len(lines):
    if lines[i].lstrip().startswith("#"):
        i += 1
        continue
    m = start_re.search(lines[i])
    if not m:
        i += 1
        continue
    delim = m.group(2)
    dash = "<<-" in lines[i][: m.start() + 3]
    i += 1
    body = []
    while i < len(lines):
        stripped = lines[i].lstrip("\t") if dash else lines[i]
        if stripped == delim:
            break
        body.append(stripped.rstrip("\r"))
        i += 1
    first = body[0] if body else ""
    if is_sh_shebang(first):
        print("OK\t" + first)
    elif first.startswith("#!/bin/sh"):
        # row 1737: a #!/bin/sh-prefixed first line that fails the
        # classifier (multi-cluster flags form) execs as "invalid option"
        # — it claims the sh-stub contract and breaks it, so VIOLATE.
        print("VIOLATION\t" + first)
    elif any(line.startswith("#!/bin/sh") for line in body[1:]):
        # misplaced rail: any #!/bin/sh shape (legal or broken-flags) in
        # body[1:] is the drift class — the first line owns the shebang.
        print("VIOLATION\t" + first)
    elif first == "" and any(line.startswith("#!") for line in body[1:]):
        print("VIOLATION\t" + first)
    i += 1
PYEOF
	)
done
echo "stub-shebang summary: $((checked - viol)) stub heredoc ok / $checked checked (${viol} violation(s))"
[ "$viol" -eq 0 ]
