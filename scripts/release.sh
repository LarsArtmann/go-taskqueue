#!/usr/bin/env bash
# Release runner — codifies the v0.1.0 release checklist
# (docs/release/archived/2026-09-06_v0.1.0_CHECKLIST.md) so v0.2.0+ is a
# command, not a memory exercise. Owner decisions baked in: no squash, v0.x
# GitHub Releases are pre-releases, tags are annotated and immutable.
#
# Usage:
#   scripts/release.sh v0.2.0            # pre-tag gates only (safe default)
#   scripts/release.sh v0.2.0 --tag      # gates, then cut the annotated tag
#   scripts/release.sh v0.2.0 --push     # tag + push + verify + GitHub Release
#
# The gates delegate to scripts/ci-local.sh (the CI replicant) so this
# script and CI can never drift apart. --push performs owner-gated actions
# (pushing, publishing); everything before it is read-only.
set -euo pipefail
cd "$(dirname "$0")/.."

step() { printf '\n== %s\n' "$*"; }
die() {
	echo "FAIL: $*" >&2
	exit 1
}

VERSION="${1:-}"
MODE="${2:-}"
[ -n "$VERSION" ] || die "usage: scripts/release.sh vX.Y.Z [--tag|--push]"
[[ "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || die "version '$VERSION' is not vX.Y.Z"
case "$MODE" in
"") ;;
--tag | --push) ;;
*) die "unknown mode '$MODE' (use --tag or --push)" ;;
esac

MODULE="$(head -1 go.mod | cut -d' ' -f2)"

step "preconditions"
[ -d .git ] || die "not a git repo"
command -v git >/dev/null || die "git missing"
LAST_TAG="$(git tag --sort=-v:refname | grep -E '^v[0-9]' | head -1 || true)"
echo "module: $MODULE  last tag: ${LAST_TAG:-none}  target: $VERSION"
if [ -n "$LAST_TAG" ] && [ "$VERSION" != "$LAST_TAG" ] && [ "$(printf '%s\n%s\n' "$LAST_TAG" "$VERSION" | sort -V | head -1)" = "$VERSION" ]; then
	die "$VERSION does not sort after $LAST_TAG — a release must move forward (tags are immutable; fixes ship as a NEW version)"
fi
TAG_EXISTS=false
if git rev-parse -q --verify "refs/tags/$VERSION" >/dev/null; then
	if [ "$MODE" = "--push" ] && [ "$(git rev-list -n1 "$VERSION")" = "$(git rev-list -n1 HEAD)" ]; then
		TAG_EXISTS=true
		echo "tag $VERSION already cut at HEAD — resuming the release for push"
	else
		die "tag $VERSION already exists — tags are immutable once the proxy indexes them"
	fi
fi
if [ -n "$(git status --porcelain)" ]; then
	git status --short
	die "working tree not clean — commit (or let the auto-daemon commit) before releasing; the tag must point at the exact verified tree"
fi

step "CHANGELOG section for $VERSION"
grep -q "^## \[$VERSION\] - [0-9]\{4\}-[0-9]\{2\}-[0-9]\{2\}$" CHANGELOG.md ||
	die "CHANGELOG.md has no '## [$VERSION] - YYYY-MM-DD' section — cut [Unreleased] into it first"
awk -v v="## [$VERSION]" '
	index($0, v) == 1 {in_section=1; next}
	in_section && /^## \[/ {exit}
	in_section {print}
' CHANGELOG.md >/tmp/tq-release-notes.md
[ -s /tmp/tq-release-notes.md ] || die "CHANGELOG section for $VERSION is empty"

step "flake.nix version sync (tq version reports it)"
flake_ver="$(sed -n 's/^[[:space:]]*version = "\(.*\)";$/\1/p' flake.nix | head -1)"
[ "$flake_ver" = "${VERSION#v}" ] || die "flake.nix version ($flake_ver) != release ${VERSION#v} — bump the version attr AND the -ldflags line"
grep -q "main.version=${VERSION#v}" flake.nix || die "flake.nix ldflags does not carry ${VERSION#v} — nix binaries would report the wrong tq version"

step "go.mod hygiene"
# Sibling-relative replaces for the internal sub-modules are the multi-module
# pattern (ADR-0011): consumers ignore them and resolve via the require
# versions, which the subdirectory tags below make real. Anything else is
# proxy poison.
bad_replaces="$(grep '^replace' go.mod | grep -vE '^replace github\.com/larsartmann/go-taskqueue/internal/[a-z0-9-]+ => \./internal/[a-z0-9-]+$' || true)"
if [ -n "$bad_replaces" ]; then
	echo "$bad_replaces"
	die "go.mod has non-sibling replace directives — poison in published tags"
fi
! grep '00010101' go.mod || die "go.mod has a pseudo-version (replace-directive leak)"
# go install of the published module resolves the internal sub-modules
# through the module proxy: every internal require must be a real version
# whose subdirectory tag exists BEFORE the release tag is cut.
while read -r mod ver; do
	sub_tag="${mod#github.com/larsartmann/go-taskqueue/}/$ver"
	git rev-parse -q --verify "refs/tags/$sub_tag" >/dev/null || {
		die "$mod requires $ver but tag $sub_tag does not exist — cut it (git tag -a $sub_tag) before releasing"
	}
done < <(grep -E '^[[:space:]]*github\.com/larsartmann/go-taskqueue/internal/[a-z0-9-]+ v[0-9]' go.mod | awk '{print $1, $2}')

step "full CI gate (scripts/ci-local.sh — test + nix jobs on this exact tree)"
./scripts/ci-local.sh

step "smoke the nix-built binary through the web UI"
TQ_BIN="$(nix build --print-out-paths)/bin/tq" ./scripts/smoke/webui.sh

if [ -z "$MODE" ]; then
	echo
	echo "GATES GREEN for $VERSION — re-run with --tag to cut the annotated tag,"
	echo "then with --push to publish (owner-gated)."
	exit 0
fi

step "cut annotated tag $VERSION on HEAD $(git rev-parse --short HEAD)"
# Tag immediately after the gates: the auto-commit daemon may commit at any
# moment; a tag on a later daemon commit is fine (it only adds bookkeeping),
# but a tag BEFORE the release commits land is the classic mistake.
if [ "$TAG_EXISTS" = "true" ]; then
	echo "tag $VERSION already cut at HEAD — skipping (resume for push)"
else
	git tag -a "$VERSION" -m "Release $VERSION

$(cat /tmp/tq-release-notes.md)"
	points_at="$(git tag --points-at HEAD)"
	echo "$points_at" | grep -qx "$VERSION" || die "tag does not point at HEAD"
	git show "$VERSION:go.mod" >/tmp/tq-tag-gomod.txt
	head -1 /tmp/tq-tag-gomod.txt | grep -q "$MODULE" || die "tagged tree has the wrong module path"
fi

step "cut internal sub-module tags (go install resolution for the split)"
internal_tags="$(grep -E '^[[:space:]]*github\.com/larsartmann/go-taskqueue/internal/[a-z0-9-]+ v[0-9]' go.mod | awk '{print substr($1, length("github.com/larsartmann/go-taskqueue/") + 1) "/" $2}')"
for sub_tag in $internal_tags; do
	if git rev-parse -q --verify "refs/tags/$sub_tag" >/dev/null; then
		echo "$sub_tag already exists"
	else
		git tag -a "$sub_tag" -m "$sub_tag (released with $VERSION)"
		echo "cut $sub_tag"
	fi
done

if [ "$MODE" = "--tag" ]; then
	echo
	echo "TAG $VERSION CUT (not pushed) — re-run with --push to publish."
	exit 0
fi

step "push master + tag (owner-gated)"
git push origin master
git push origin "$VERSION"
for sub_tag in $internal_tags; do
	git rev-parse -q --verify "refs/tags/$sub_tag" >/dev/null && git push origin "$sub_tag"
done

step "module proxy verification"
sleep 10
for attempt in 1 2 3 4 5; do
	if GOFLAGS= go list -m -versions "$MODULE" 2>/dev/null | tr ' ' '\n' | grep -qx "$VERSION"; then
		echo "proxy serves $VERSION"
		break
	fi
	echo "proxy does not list $VERSION yet (attempt $attempt/5) — propagation takes minutes"
	[ "$attempt" = 5 ] && die "proxy never listed $VERSION; verify https://proxy.golang.org/$MODULE/@v/$VERSION.info before retrying anything (never re-tag)"
	sleep 30
done

step "clean-room go get + go install (build the real consumer path)"
verify_dir="$(mktemp -d)"
trap 'rm -rf "$verify_dir"' EXIT
(
	cd "$verify_dir"
	go mod init release-verify
	go get "$MODULE@$VERSION" >/dev/null
	go mod verify
	# go get alone only resolves metadata; it cannot catch a broken
	# sub-module require (v0.0.0-style pins resolve nothing on the proxy).
	# Building the binary is the proof the published tree is installable.
	GOBIN="$verify_dir/bin" go install "$MODULE/cmd/tq@$VERSION"
	"$verify_dir/bin/tq" version >/dev/null
)

step "GitHub Release (pre-release: v0.x policy)"
command -v gh >/dev/null || die "gh CLI missing — create the release manually from /tmp/tq-release-notes.md"
gh release create "$VERSION" --title "$VERSION" --notes-file /tmp/tq-release-notes.md --prerelease

echo
echo "RELEASE $VERSION PUBLISHED — remaining manual step: verify the CI run on"
echo "refs/tags/$VERSION is green (gh run list --limit 3)."
