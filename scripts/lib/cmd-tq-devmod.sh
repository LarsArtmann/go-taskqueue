# shellcheck shell=bash
# Sourced by scripts/build-tq.sh and scripts/test-cmd-tq.sh (ADR-0017).
#
# The committed cmd/tq/go.mod is REPLACE-FREE so `go install …/cmd/tq@vX.Y.Z`
# resolves through the module proxy. In-repo builds must therefore run the
# module through a GENERATED dev.mod that carries the repo's local replace
# set (root module + every internal sub-module), mirroring how the root
# module itself builds. dev.mod/dev.sum are derived, never committed.
#
# cmdtq_devmod: generates cmd/tq/dev.mod + dev.sum from the committed files.
cmdtq_devmod() {
	local root dir
	root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
	dir="$root/cmd/tq"
	# Preflight (M17.2): the committed go.mod losing its module declaration
	# (a bad merge, a sed accident) otherwise surfaces as a confusing
	# downstream tidy/build failure. Fail HERE, naming the file and the fix.
	if ! grep -q '^module github.com/larsartmann/go-taskqueue/cmd/tq$' "$dir/go.mod"; then
		echo "build-tq: cmd/tq/go.mod lost its module declaration" >&2
		echo "  expected the line: module github.com/larsartmann/go-taskqueue/cmd/tq" >&2
		echo "  restore it (git restore cmd/tq/go.mod) and retry" >&2

		return 1
	fi
	# The tidy below must not inherit a pinned GOTOOLCHAIN=local on a binary
	# older than the go.mod floor: the failure is swallowed (|| true) and the
	# gate then dies at build with "updates to go.mod needed". Same env-lie
	# the gate itself guards against one step later.
	export GOTOOLCHAIN=auto
	{
		cat "$dir/go.mod"
		printf '\nreplace github.com/larsartmann/go-taskqueue => ../..\n'
		sed -n 's|^replace \(github.com/larsartmann/go-taskqueue/internal[^ ]*\) => ./\(.*\)$|replace \1 => ../../\2|p' "$root/go.mod"
	} >"$dir/dev.mod"
	cp "$dir/go.sum" "$dir/dev.sum"
	# Replaced internal modules can gain external dependencies between
	# releases (before the next tag bump lands in cmd/tq's committed
	# requires) — the committed go.mod cannot express them yet, so tidy the
	# DERIVED modfile against the replaced graph: dev.mod/dev.sum get exactly
	# the requirements and hashes the local build needs, and the committed
	# files stay proxy-clean. Needs every module in the local cache (the
	# root build fetches them).
	(
		cd "$dir" || exit 1
		GOWORK=off GOFLAGS='' go mod tidy -modfile=dev.mod >/dev/null 2>&1 || true
	)
	CMD_TQ_DIR="$dir"
}

cmdtq_devmod_cleanup() {
	rm -f "${CMD_TQ_DIR:?CMD_TQ_DIR not set}/dev.mod" "${CMD_TQ_DIR:?}/dev.sum"
	if [ -n "${CMD_TQ_WORK:-}" ]; then
		rm -rf "$(dirname "$CMD_TQ_WORK")"
	fi
}

# cmdtq_devwork: generates a throwaway go.work (in a /tmp scratch dir, never
# the repo tree — the auto-commit daemon sweeps everything committable) whose
# use-set mirrors the root go.mod's replace targets. golangci-lint cannot
# consume the dev.mod -modfile (its internal `go env -json` probes reject
# build flags: "build flag -modfile only valid when using modules"), but it
# consumes GOWORK natively — so module-aware TOOLING lint runs through this
# second shim. Sets CMD_TQ_WORK to the work file path.
cmdtq_devwork() {
	local root gover
	root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
	gover="$(awk '$1 == "go" { print $2; exit }' "$root/go.mod")"
	CMD_TQ_WORK="$(mktemp -d "${TMPDIR:-/tmp}/cmdtq-work.XXXXXX")/go.work"
	{
		echo "go ${gover:?root go.mod has no go directive}"
		echo "use ("
		echo "	$root"
		echo "	$root/cmd/tq"
		awk -v root="$root" '/^replace github\.com\/larsartmann\/go-taskqueue.* => \.\// {
			path = $NF
			sub(/^\.\//, "", path)
			print "\t" root "/" path
		}' "$root/go.mod" | sort -u
		echo ")"
	} >"$CMD_TQ_WORK"
}
