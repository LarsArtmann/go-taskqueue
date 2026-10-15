#!/usr/bin/env bash
# dep-sweep e2e smoke (06-44 §f4): a stub planner emits a two-module
# release-overview plan (stale lib release + consumer bump), the pool mints
# the depbump tasks, --once drains them through the deterministic executor,
# and the scratch repos show the go get -> commit -> tag happy path. The
# second pool run pins the convergence rule: completed dedup keys come back
# as known hits, never as sweep failures. Scratch DB everywhere (production
# journal trap guard); fully offline (the consumer pin rides a filesystem
# replace, GOPROXY=off makes any surprise fetch fatal, not flaky).
set -euo pipefail
cd "$(dirname "$0")/../.."
REPO_ROOT="$(pwd)"

TMP="$(mktemp -d)"
cleanup() {
	# TQ_SMOKE_KEEP=1 keeps the scratch tree for forensics on a failure.
	if [ "${TQ_SMOKE_KEEP:-0}" = "1" ]; then
		echo "== TQ_SMOKE_KEEP=1: scratch tree left at $TMP"
		return
	fi
	rm -rf "$TMP"
}
trap cleanup EXIT

if [ -n "${TQ_BIN:-}" ]; then
	echo "== using prebuilt tq: $TQ_BIN"
	cp "$TQ_BIN" "$TMP/tq"
	chmod +x "$TMP/tq"
else
	echo "== build tq"
	"$REPO_ROOT/scripts/build-tq.sh" "$TMP/tq"
fi

export TQ_DB="$TMP/tasks.db"

# Hermetic go: no workspace, no proxy, no toolchain fetch.
export GOFLAGS=
export GOWORK=off
export GOPROXY=off
export GOTOOLCHAIN=local
export GIT_AUTHOR_NAME="tq smoke"
export GIT_AUTHOR_EMAIL="smoke@tq.local"
export GIT_COMMITTER_NAME="tq smoke"
export GIT_COMMITTER_EMAIL="smoke@tq.local"

LIB="$TMP/lib"
CONSUMER="$TMP/consumer"

echo "== scratch lib: v0.1.0 tagged, one unreleased commit on top (stale build)"
mkdir -p "$LIB"
git -C "$LIB" init -q
printf 'module example.com/smoke/depsweep/lib\n\ngo 1.24\n' >"$LIB/go.mod"
printf 'package lib\n\nconst Answer = 41\n' >"$LIB/lib.go"
printf 'package lib\n\nimport "testing"\n\nfunc TestAnswer(t *testing.T) { if Answer != 41 { t.Fail() } }\n' >"$LIB/lib_test.go"
git -C "$LIB" add -A
git -C "$LIB" commit -qm "lib v0.1.0"
git -C "$LIB" tag -a v0.1.0 -m v0.1.0
# The unreleased commit the release task must tag as v0.1.1. Additive on
# purpose: the consumer's behavior must not change across the bump.
printf 'package lib\n\nconst Greeting = "hi"\n' >"$LIB/greet.go"
git -C "$LIB" add -A
git -C "$LIB" commit -qm "unreleased greeting"

echo "== scratch consumer: pins lib v0.1.0 through a filesystem replace (offline)"
mkdir -p "$CONSUMER"
git -C "$CONSUMER" init -q
printf 'module example.com/smoke/depsweep/consumer\n\ngo 1.24\n\nrequire example.com/smoke/depsweep/lib v0.1.0\n\nreplace example.com/smoke/depsweep/lib => %s\n' "$LIB" >"$CONSUMER/go.mod"
printf 'package consumer\n\nimport "example.com/smoke/depsweep/lib"\n\nfunc Answer() int { return lib.Answer }\n' >"$CONSUMER/consumer.go"
printf 'package consumer\n\nimport "testing"\n\nfunc TestAnswer(t *testing.T) { if Answer() != 41 { t.Fail() } }\n' >"$CONSUMER/consumer_test.go"
git -C "$CONSUMER" add -A
git -C "$CONSUMER" commit -qm "consumer v0.1.0"

echo "== stub planner: fixed release-overview plan (lib release + consumer bump)"
cat >"$TMP/plan.json" <<EOF
{"modules":[
  {"key":"lib","label":"lib","dir":"$LIB","hasUnreleasedWork":true,"currentVersion":"v0.1.0","nextVersion":"v0.1.1","suggestedBump":"patch","staleBuild":true,"dependencies":[],"outdatedConsumers":[{"key":"consumer","dir":"$CONSUMER","currentVersion":"v0.1.0","targetVersion":"v0.1.1","dependencyKey":"example.com/smoke/depsweep/lib","isMajorBump":false}]},
  {"key":"consumer","label":"consumer","dir":"$CONSUMER","staleBuild":false,"dependencies":[{"key":"lib"}]}
]}
EOF
STUB="$TMP/planner"
cat >"$STUB" <<EOF
#!/bin/sh
# release-overview --dir <dir> --format json: the plan is static.
cat "$TMP/plan.json"
EOF
chmod +x "$STUB"

run_pool() {
	timeout 300 "$TMP/tq" agent-pool \
		--repos "$LIB,$CONSUMER" \
		--dep-sweep --dep-sweep-bin "$STUB" --dep-sweep-dir "$TMP" --dep-sweep-interval 24h \
		--once --poll 50ms --task-timeout 120s >"$TMP/pool$1.log" 2>&1 ||
		{
			cat "$TMP/pool$1.log"
			exit 1
		}
}

echo "== pool run 1: sweep mints 2, drain runs go get -> commit -> tag"
run_pool 1
grep -q "dep sweep done" "$TMP/pool1.log" || {
	cat "$TMP/pool1.log"
	echo "FAIL: dep sweep never ran"
	exit 1
}
grep -q "minted=2" "$TMP/pool1.log" || {
	cat "$TMP/pool1.log"
	echo "FAIL: want minted=2 (lib release + consumer bump)"
	exit 1
}

"$TMP/tq" tasks --type depbump --json >"$TMP/tasks.json"
[ "$(grep -c '"status": "completed"' "$TMP/tasks.json")" = "2" ] || {
	cat "$TMP/tasks.json"
	echo "FAIL: want both depbump tasks completed"
	exit 1
}

echo "== consumer bump task waits on the lib release task (plan DAG -> task deps)"
CONSUMER_ID="$(grep -B8 '"project": "consumer"' "$TMP/tasks.json" | grep -o '"id": "[^"]*"' | head -1 | cut -d'"' -f4)"
LIB_ID="$(grep -B8 '"project": "lib"' "$TMP/tasks.json" | grep -o '"id": "[^"]*"' | head -1 | cut -d'"' -f4)"
[ -n "$CONSUMER_ID" ] && [ -n "$LIB_ID" ] || {
	echo "FAIL: could not resolve depbump task ids"
	exit 1
}
"$TMP/tq" show "$CONSUMER_ID" >"$TMP/consumer-task.json"
grep -q "\"$LIB_ID\"" "$TMP/consumer-task.json" || {
	cat "$TMP/consumer-task.json"
	echo "FAIL: consumer task must depend on the lib release task $LIB_ID"
	exit 1
}

echo "== lib released: v0.1.1 tag on the unreleased commit"
git -C "$LIB" tag --list 'v0.1.1' | grep -qx 'v0.1.1' || {
	git -C "$LIB" tag --list
	echo "FAIL: release tag v0.1.1 missing"
	exit 1
}
[ "$(git -C "$LIB" rev-parse v0.1.1^{commit})" = "$(git -C "$LIB" rev-parse HEAD)" ] || {
	echo "FAIL: v0.1.1 must tag the lib HEAD (the unreleased commit)"
	exit 1
}

echo "== consumer bumped: exact pin, conventional commit, clean tree"
grep -q 'example.com/smoke/depsweep/lib v0.1.1' "$CONSUMER/go.mod" || {
	cat "$CONSUMER/go.mod"
	echo "FAIL: consumer still pins the old lib version"
	exit 1
}
[ "$(git -C "$CONSUMER" log --format=%s -1)" = "chore(deps): bump example.com/smoke/depsweep/lib to v0.1.1" ] || {
	git -C "$CONSUMER" log --oneline -3
	echo "FAIL: bump commit message missing"
	exit 1
}
[ -z "$(git -C "$CONSUMER" status --porcelain)" ] || {
	git -C "$CONSUMER" status --porcelain
	echo "FAIL: consumer tree left dirty"
	exit 1
}

echo "== pool run 2: unchanged plan converges (known=2, minted=0, no failure)"
BEFORE_LIB_TAGS="$(git -C "$LIB" tag --list | wc -l)"
BEFORE_CONSUMER_COMMITS="$(git -C "$CONSUMER" rev-list --count HEAD)"
run_pool 2
grep -q "known=2" "$TMP/pool2.log" || {
	cat "$TMP/pool2.log"
	echo "FAIL: want known=2 on the second sweep"
	exit 1
}
grep -q "dep sweep failed" "$TMP/pool2.log" && {
	cat "$TMP/pool2.log"
	echo "FAIL: completed dedup keys must be known hits, not sweep failures"
	exit 1
}
[ "$("$TMP/tq" tasks --type depbump --count)" = "2 matching task(s)" ] || {
	echo "FAIL: second sweep re-minted tasks"
	exit 1
}
[ "$(git -C "$LIB" tag --list | wc -l)" = "$BEFORE_LIB_TAGS" ] || {
	echo "FAIL: second sweep re-tagged the lib"
	exit 1
}
[ "$(git -C "$CONSUMER" rev-list --count HEAD)" = "$BEFORE_CONSUMER_COMMITS" ] || {
	echo "FAIL: second sweep re-committed the consumer bump"
	exit 1
}

echo "PASS: dep-sweep e2e (mint, DAG, go get -> commit -> tag, convergence)"
