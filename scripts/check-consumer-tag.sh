#!/usr/bin/env bash
# Tagged-facade consumer-path proof: a scratch module that knows NOTHING
# about this checkout resolves the root module plus all seven ADR-0016
# facades at an EXPLICIT tag from the module proxy and compiles an import
# of every facade. This is the canonical release verification (02-02
# report §f11; 00-23 §b1/f3): the literal `go install <facade>@<tag>` is
# impossible-by-design — facade go.mods carry relative dev replaces, so
# the toolchain refuses them everywhere (sandbox-independent) — while a
# consumer ignores dependency replaces and resolves through the require
# graph, which is exactly the path this proof exercises.
#
#   scripts/check-consumer-tag.sh            # latest vX.Y.Z tag
#   scripts/check-consumer-tag.sh v0.3.4     # explicit tag
#   scripts/check-consumer-tag.sh --self-test
#
# release.sh --push runs this at the release version after the tag wave
# lands; standalone it re-verifies any released tag on demand.
set -euo pipefail
cd "$(dirname "$0")/.."

MODULE="github.com/larsartmann/go-taskqueue"

# ADR-0016 facade surface: the root module plus the seven re-export
# facades (mirrors check-pkg-proxy.sh's list; check-facade-parity pins
# the pairs — adding an 8th facade means updating all three).
FACADES=("$MODULE/task" "$MODULE/journal" "$MODULE/queue"
	"$MODULE/queue/sqlite" "$MODULE/queue/postgres"
	"$MODULE/executor" "$MODULE/worker")

# write_consumer_spec emits the scratch module the proof builds: a go.mod
# for an in-the-clear consumer, the go get list pinning the root + every
# facade at <version> (explicit tags — @latest would prove nothing about
# THIS release), and a main.go importing every facade. A function so
# --self-test pins the exact shipped generation offline.
write_consumer_spec() {
	local dir="$1" ver="$2"
	# pins: in-the-clear-consumer-branch — scratch go.mod pins the exact
	# tagged version of root + every facade (no replace, no workspace)
	cat >"$dir/go.mod" <<EOF
module tq-consumer-tag

go 1.27.1
EOF
	: >"$dir/get.list"
	printf '%s@%s\n' "$MODULE" "$ver" >>"$dir/get.list"
	local facade
	for facade in "${FACADES[@]}"; do
		printf '%s@%s\n' "$facade" "$ver" >>"$dir/get.list"
	done
	{
		echo "package main"
		echo
		for facade in "${FACADES[@]}"; do
			echo "import _ \"$facade\""
		done
		echo
		echo "func main() {}"
	} >"$dir/main.go"
}

self_test=false
version=""
for arg in "$@"; do
	case $arg in
	--self-test) self_test=true ;;
	-*)
		echo "FAIL: unknown flag: $arg" >&2
		exit 1
		;;
	*) version="$arg" ;;
	esac
done

if $self_test; then
	# Offline rails: flag rejection + scratch-spec generation. The
	# network half (go get/tidy/build) is the real gate — exercised by
	# release.sh --push and one-off runs, never faked here.
	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' EXIT

	# Canary (expect-branch contract): rc!=0 must come from the unknown-flag
	# rejection branch's own FAIL line.
	bogus_out="$(./scripts/check-consumer-tag.sh --bogus-flag 2>&1)" && bogus_rc=0 || bogus_rc=$?
	if [ "$bogus_rc" -eq 0 ] || [[ "$bogus_out" != *"FAIL: unknown flag: --bogus-flag"* ]]; then
		echo "FAIL: --bogus-flag accepted or refused on the wrong branch (rc=$bogus_rc): $bogus_out" >&2
		exit 1
	fi

	latest="$(git tag --list 'v[0-9]*' | sort -V | tail -1)"
	[ -n "$latest" ] || {
		echo "FAIL: no vX.Y.Z tag found for the default-tag rail" >&2
		exit 1
	}

	write_consumer_spec "$tmp" "$latest"
	grep -qx "module tq-consumer-tag" "$tmp/go.mod" || {
		echo "FAIL: scratch go.mod module line wrong" >&2
		exit 1
	}
	for facade in "${FACADES[@]}"; do
		grep -qx "$facade@$latest" "$tmp/get.list" || {
			echo "FAIL: $facade missing from the get list at $latest" >&2
			exit 1
		}
		grep -qx "import _ \"$facade\"" "$tmp/main.go" || {
			echo "FAIL: main.go does not import $facade" >&2
			exit 1
		}
	done
	grep -qx "$MODULE@$latest" "$tmp/get.list" || {
		echo "FAIL: root module missing from the get list at $latest" >&2
		exit 1
	}
	echo "SELF-TEST OK (flag rail + spec generation at $latest)"
	exit 0
fi

if [ -z "$version" ]; then
	version="$(git tag --list 'v[0-9]*' | sort -V | tail -1)"
fi
[ -n "$version" ] || {
	echo "consumer-tag SKIP: no vX.Y.Z tag found" >&2
	exit 0
}

verify_dir="$(mktemp -d)"
trap 'rm -rf "$verify_dir"' EXIT

(
	cd "$verify_dir"
	export GOWORK=off
	export GOTOOLCHAIN=auto
	export GOEXPERIMENT=jsonv2
	write_consumer_spec . "$version"
	mapfile -t get_specs <get.list
	go get "${get_specs[@]}" >/dev/null
	go mod tidy >/dev/null
	go build -o /dev/null .
)
echo "CONSUMER-TAG OK: root + 7 facades resolve from the proxy and compile at $version"
