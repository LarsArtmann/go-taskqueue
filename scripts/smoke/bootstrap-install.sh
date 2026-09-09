#!/usr/bin/env bash
# Bootstrap --install live smoke: the REAL binary renders the systemd unit
# + pool.conf into a fake $HOME, stubbed systemctl/loginctl capture the
# enable calls, and the running system is never touched (21:40 §e-item).
#
# Assertions:
#   1. ~/.config/tq/pool.conf exists with the resolved settings
#   2. ~/.config/systemd/user/tq-agent-pool.service exists with the drain
#      invariants (KillSignal=SIGINT, KillMode=process, TimeoutStopSec,
#      ProtectSystem=full)
#   3. systemctl daemon-reload + enable --now + loginctl enable-linger were
#      invoked exactly once each — against the STUBS, never the host
set -uo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
WORK="$(mktemp -d)"
TQ="${TQ_BIN:-$WORK/tq}"
KEEP="${SMOKE_KEEP:-0}"
cleanup() { [ "$KEEP" = 1 ] && echo "workdir kept: $WORK" || rm -rf "$WORK"; }
trap cleanup EXIT

if [ -z "${TQ_BIN:-}" ]; then
	echo "== building tq =="
	(cd "$REPO_ROOT" && go build -o "$TQ" ./cmd/tq) || exit 1
fi

export HOME="$WORK/home"
mkdir -p "$HOME" "$WORK/stubbin"

# Stubs: log every invocation, always succeed. If the REAL systemctl ever
# ran, this smoke would fail its log assertions before touching the host.
for tool in systemctl loginctl; do
	cat >"$WORK/stubbin/$tool" <<EOF
#!/bin/sh
printf '%s %s\n' '$tool' "\$*" >> '$WORK/syscalls.log'
exit 0
EOF
	chmod +x "$WORK/stubbin/$tool"
done
export PATH="$WORK/stubbin:$PATH"

REPO="$WORK/demo"
mkdir -p "$REPO"
printf '# Work\n\n- [ ] someday item\n' >"$REPO/TODO_LIST.md"

# Minimal git identity + no signing: the fake HOME hides the real global
# gitconfig (and its SSH signing key), which would break every commit.
export GIT_CONFIG_GLOBAL="$WORK/gitconfig"
printf '[user]\n\tname = smoke\n\temail = smoke@test\n[commit]\n\tgpgsign = false\n' >"$GIT_CONFIG_GLOBAL"

git -C "$REPO" init -q
git -C "$REPO" add -A
git -C "$REPO" commit -qm init

echo "== tq bootstrap --install (fake \$HOME, stubbed systemctl) =="
DB="$WORK/q.db"
if ! "$TQ" bootstrap "$REPO" --install \
	--projects-dir "$WORK" \
	--db "$DB" \
	--agents 2 --daily-budget 7 \
	--verify "demo=go build ./... && go test ./... -count=1" \
	--reasoning medium; then
	echo "FAIL: bootstrap --install exited non-zero"
	exit 1
fi

CONF="$HOME/.config/tq/pool.conf"
UNIT="$HOME/.config/systemd/user/tq-agent-pool.service"

fail=0
for path in "$CONF" "$UNIT"; do
	[ -f "$path" ] || {
		echo "FAIL: $path missing"
		fail=1
	}
done

grep -q "^repos = $REPO$" "$CONF" || {
	echo "FAIL: pool.conf does not pin the repo"
	fail=1
}
grep -q "^daily-budget = 7$" "$CONF" || {
	echo "FAIL: pool.conf does not carry the budget"
	fail=1
}

# The pinned verify command must land in the repo EXACTLY as the flag
# spelled it — the payload gate is only as good as the rail it reads.
VERIFY_FILE="$REPO/.tq-verify"
if [ ! -f "$VERIFY_FILE" ]; then
	echo "FAIL: $VERIFY_FILE missing"
	fail=1
elif ! grep -qxF 'go build ./... && go test ./... -count=1' "$VERIFY_FILE"; then
	echo "FAIL: .tq-verify does not match the --verify flag exactly:"
	cat "$VERIFY_FILE"
	fail=1
fi

for invariant in "KillSignal=SIGINT" "KillMode=process" "TimeoutStopSec=" "ProtectSystem=full" "ExecStart="; do
	grep -q "$invariant" "$UNIT" || {
		echo "FAIL: unit missing drain invariant $invariant"
		fail=1
	}
done

[ -f "$WORK/syscalls.log" ] || {
	echo "FAIL: systemctl/loginctl never invoked (stub log missing)"
	fail=1
}
for call in "systemctl --user daemon-reload" "systemctl --user enable --now tq-agent-pool.service" "loginctl enable-linger"; do
	grep -qF "$call" "$WORK/syscalls.log" || {
		echo "FAIL: expected call not made: $call"
		fail=1
	}
done

if [ "$fail" = 0 ]; then
	echo "bootstrap-install smoke ok: unit + pool.conf rendered, enable calls stubbed, host untouched"
fi
exit "$fail"
