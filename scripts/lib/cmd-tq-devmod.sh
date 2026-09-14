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
	{
		cat "$dir/go.mod"
		printf '\nreplace github.com/larsartmann/go-taskqueue => ../..\n'
		sed -n 's|^replace \(github.com/larsartmann/go-taskqueue/internal[^ ]*\) => ./\(.*\)$|replace \1 => ../../\2|p' "$root/go.mod"
	} >"$dir/dev.mod"
	cp "$dir/go.sum" "$dir/dev.sum"
	# The replaced internal modules can gain external dependencies between
	# releases (before the next tag bump lands in cmd/tq's committed
	# requires); the ROOT go.sum already sums that exact graph (root builds
	# with the same replace set), so union it in — dev.sum is derived, never
	# committed, and extra entries are harmless.
	cat "$root/go.sum" >>"$dir/dev.sum"
	CMD_TQ_DIR="$dir"
}

cmdtq_devmod_cleanup() {
	rm -f "${CMD_TQ_DIR:?CMD_TQ_DIR not set}/dev.mod" "${CMD_TQ_DIR:?}/dev.sum"
}
