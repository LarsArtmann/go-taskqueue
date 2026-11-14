#!/usr/bin/env bash
# check-subtag.sh: GOPRIVATE-correct clean-room verifier for any
# `<module>@<tag>` (the v0.4.3 supersession-proof mktemp dance, pinned as
# a reusable gate because every sub-tag proof needs the same rails).
#
# What it proves: the tag resolves and BUILDS the way a fresh consumer
# sees it — a scratch module, a FRESH GOMODCACHE (no warm-cache skew, the
# v0.4.1 class), no workspace, no vendor, and `url.<checkout>.insteadOf`
# rewriting the module path to THIS checkout so the private fetch never
# touches the proxy. GOPRIVATE is set to the module path so the sumdb/proxy
# path is skipped for it — the first hand-rolled attempt tripped sumdb
# exactly because GOPRIVATE was missing while insteadOf served locally.
# (Discovery caveat: `go get` still serves an HTTPS ?go-get=1 meta query
# for the module HOST before git runs — the insteadOf rewrite covers only
# the actual clone, never the discovery step. github.com answers the query
# even for private repos (auth-gated meta), so private github modules
# verify clean here; a SELF-HOSTED host without a ?go-get=1 endpoint 404s
# the query and `go get` dies BEFORE the insteadOf fetch is ever reached.
# Escape rails (caller-exported env passes through the clean-room subshell
# — the gate pins only GOWORK/GOTOOLCHAIN/GOMODCACHE/GOPRIVATE):
#   GOVCS='<module-path>:git'   pin the VCS command (does NOT skip the
#                               discovery query — 'off' only forbids VCS
#                               after discovery, so there is no honest
#                               --discovery-off flag; the query is
#                               unskippable at the go level)
#   GONOSUMDB / GONOPROXY       widen the no-sumdb / no-proxy sets beyond
#                               the gate's GOPRIVATE
# For a host that cannot answer ?go-get=1, the only cure is serving the
# meta tag or vendoring differently — no flag can rescue it.)
#
# Steps per tag: go get <module>@<tag> → go list -m all (graph closes) →
# go build <module> (the module's root package compiles). Refuses a tag
# absent from the checkout (cut the tag first — a local-only fetch of an
# untagged tree proves nothing).
#
# Usage: scripts/check-subtag.sh <module>@<tag> [--repo <path>]
#        scripts/check-subtag.sh --self-test
#   <module>@<tag>  e.g. github.com/larsartmann/go-taskqueue/cmd/tq@v0.4.3
#   --pkg           import path to compile instead of the module root
#   --repo          checkout whose git objects serve the insteadOf fetch
#                   (default: this repo); the tag must exist there.
set -uo pipefail

ok_count=0
fail_count=0

ok() {
	echo "ok: $*"
	ok_count=$((ok_count + 1))
}

fail() {
	echo "fail: $*"
	fail_count=$((fail_count + 1))
}

die() {
	echo "check-subtag: $*" >&2
	exit 1
}

usage() {
	echo "usage: $0 <module>@<tag> [--repo <path>]" >&2
	echo "       $0 --self-test" >&2
	exit 2
}

cleanup() {
	if [ -n "${scratch:-}" ] && [ -n "${cache:-}" ]; then
		chmod -R u+w "$scratch" "$cache" 2>/dev/null
		rm -rf "$scratch" "$cache"
		scratch=
		cache=
	fi
}

verify_subtag() {
	local module=$1 tag=$2 repo=$3 pkg=${4:-} rc=0
	local scratch='' cache=
	cleanup
	scratch=$(mktemp -d)
	cache=$(mktemp -d)

	tag_sha=$(git -C "$repo" rev-parse -q --verify "refs/tags/$tag^{commit}")
	local tag_used=$tag
	if [ -z "$tag_sha" ]; then
		# sub-module convention: <repo-relative-dir>/<tag> (cmd/tq/v0.4.3
		# for .../go-taskqueue/cmd/tq@v0.4.3); the repo-relative dir is the
		# module path minus the root module's path prefix.
		local rootmod suffix
		rootmod=$(sed -n 's/^module //p' "$repo/go.mod" 2>/dev/null | head -1)
		suffix=${module#"$rootmod"/}
		tag_sha=$(git -C "$repo" rev-parse -q --verify "refs/tags/$suffix/$tag^{commit}") &&
			tag_used="$suffix/$tag"
	fi
	if [ -z "$tag_sha" ]; then
		fail "tag $tag not found in $repo (also tried <module-suffix>/$tag) — cut it before verifying (a local fetch of an untagged tree proves nothing)"
		return 1
	fi
	ok "tag $tag_used present in checkout ($tag_sha)"

	(
		cd "$scratch"
		export GOWORK=off
		export GOTOOLCHAIN=auto
		export GOMODCACHE="$cache"
		# GOPRIVATE: skip proxy+sumdb for the module path — the insteadOf
		# rewrite serves the clone from the local checkout's git objects.
		export GOPRIVATE="$module"
		git init -q .
		git config url."file://$repo".insteadOf "https://$module/"
		printf 'module verify\n\ngo 1.26\n' >go.mod
		go get "$module@$tag" || exit 11
		go list -m all >/dev/null || exit 12
		local build_target=$module
		[ -n "$pkg" ] && build_target=$pkg
		go build "$build_target" 2>"$scratch/build.err" || exit 13
	) || rc=$?
	if [ "$rc" -eq 13 ] && [ -z "$pkg" ] && grep -q 'no required module provides package' "$scratch/build.err" 2>/dev/null; then
		ok "$module ships no root package (app-layer module) — list-only proof; pass --pkg <import> to compile one"
		rc=0
	fi
	case $rc in
	0) ok "clean-room verify $module@$tag (get + list -m all + build; fresh GOMODCACHE, GOPRIVATE, insteadOf → $repo)" ;;
	11) fail "go get $module@$tag failed — tag present but the fetch graph does not resolve (sub-module require missing?)" ;;
	12) fail "go list -m all failed for $module@$tag — consumer graph does not close (broken require/replace pin)" ;;
	13) fail "go build failed for $module@$tag — the target package does not compile" ;;
	*) fail "clean-room verify $module@$tag crashed (rc=$rc)" ;;
	esac
	cleanup
	[ "$rc" -eq 0 ]
}

# self_test uses a STUB go binary (env-hook, the fleet convention): the
# fixture path is a fake domain whose ?go-get=1 discovery query no network
# answers, so the positive path pins the wiring (GOPRIVATE, fresh cache,
# insteadOf config, tag-existence gate) by capturing what the real go
# would receive; the negative path exercises the real missing-tag refusal.
self_test() {
	local fixture fixture_sub stub_bin
	local scratch='' cache=
	fixture=$(mktemp -d)
	trap 'chmod -R u+w "${fixture:-}" 2>/dev/null; rm -rf "${fixture:-}"' EXIT
	fixture_sub="$fixture/example"
	git init -q "$fixture_sub"
	(
		cd "$fixture_sub" || exit
		git config user.email t@example.test
		git config user.name t
		printf 'module example.com/sub\n\ngo 1.26\n' >go.mod
		printf 'package sub\n\nconst V = "v0.1.0"\n' >sub.go
		git add .
		git commit -qm 'fixture module'
		git tag -a v0.1.0 -m 'fixture tag'
	)

	stub_bin="$fixture/stubbin"
	mkdir -p "$stub_bin"
	cat >"$stub_bin/go" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$@" >>"$CHECK_SUBTAG_STUB_LOG.args"
env | grep -E '^(GOWORK|GOMODCACHE|GOPRIVATE)=' >"$CHECK_SUBTAG_STUB_LOG.env"
exit 0
EOF
	chmod +x "$stub_bin/go"

	# Positive path under the stub go: rails must all be present.
	local log="$fixture/stub.log"
	CHECK_SUBTAG_STUB_LOG="$log" PATH="$stub_bin:$PATH" verify_subtag example.com/sub v0.1.0 "$fixture_sub" >/dev/null
	local pos_fail=$fail_count
	if [ "$pos_fail" -eq 0 ]; then :; else fail "self-test: positive fixture path reported failure"; fi
	grep -q '^GOWORK=off$' "$log.env" || fail "self-test: GOWORK not forced off in verify env"
	grep -q "^GOPRIVATE=example.com/sub$" "$log.env" || fail "self-test: GOPRIVATE not set to the module path (the sumdb-trip class)"
	local fresh_cache host_cache
	fresh_cache=$(sed -n 's/^GOMODCACHE=//p' "$log.env")
	host_cache=$(go env GOMODCACHE)
	[ -n "$fresh_cache" ] && [ "$fresh_cache" != "$host_cache" ] || fail "self-test: GOMODCACHE not redirected to a fresh scratch cache"
	grep -q '^example.com/sub@v0.1.0$' "$log.args" || fail "self-test: go get did not receive module@tag"
	# insteadOf wiring: assert the scratch repo's git config shape directly
	scratch=$(mktemp -d)
	cache=$(mktemp -d)
	(
		cd "$scratch" || exit
		git init -q .
		git config url."file://$fixture_sub".insteadOf "https://example.com/sub/"
	)
	git -C "$scratch" config --get 'url.file://'"$fixture_sub"'.insteadOf' >/dev/null ||
		fail "self-test: insteadOf config not storable in scratch repo"
	# Negative path: missing tag refuses with a NAMED message (real git).
	local neg_out neg_rc=0
	neg_out=$(verify_subtag example.com/sub v0.9.9 "$fixture_sub" 2>&1) || neg_rc=$?
	[ "$neg_rc" -ne 0 ] || fail "self-test: missing-tag path did not refuse"
	printf '%s' "$neg_out" | grep -q 'tag v0.9.9 not found' || fail "self-test: refusal does not name the missing tag"

	if [ "$fail_count" -eq 0 ]; then
		ok "self-test: fixture rails (GOPRIVATE/fresh-cache/GOWORK/get-args/insteadOf) + missing-tag refusal pinned"
	fi
	cleanup
	echo "self-test: ok=$ok_count fail=$fail_count"
	[ "$fail_count" -eq 0 ]
}

main() {
	[ "${1:-}" = "--self-test" ] && {
		self_test
		exit $?
	}
	local target=${1:-} repo=$PWD pkg=
	shift || true
	while [ $# -gt 0 ]; do
		case $1 in
		--repo)
			repo=${2:-}
			shift 2
			;;
		--pkg)
			pkg=${2:-}
			shift 2
			;;
		*) usage ;;
		esac
	done
	[ -n "$target" ] || usage
	case $target in
	*@*) ;;
	*) die "target must be <module>@<tag>, got: $target" ;;
	esac
	local module=${target%@*} tag=${target##*@}
	[ -d "$repo/.git" ] || die "--repo is not a git checkout: $repo"
	verify_subtag "$module" "$tag" "$repo" "$pkg"
	echo "check-subtag: ok=$ok_count fail=$fail_count"
	[ "$fail_count" -eq 0 ]
}

main "$@"
exit $?
