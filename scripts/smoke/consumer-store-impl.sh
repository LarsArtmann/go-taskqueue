#!/usr/bin/env bash
# T5 twin (2026-11-23 embed-guard replacement): out-of-tree Store-
# IMPLEMENTABILITY smoke. The 2026-11-23 driver-facade retirement moved
# the store drivers to internal/queue/{sqlite,postgres} — an external
# adopter can no longer construct a built-in store, so the external
# adoptability story is "implement queue.Store yourself against the
# facades" (examples/embed's own doc comment names it). Nothing proved
# that story compiles: consumer-install.sh blank-imports the facades
# (importability only), and an interface whose signatures reference
# non-facade types would break every external implementer silently.
#
# This smoke builds a scratch module OUTSIDE the repo whose storeStub
# implements EVERY queue.Store method with exact signatures and asserts
# `var _ queue.Store = storeStub{}` compiles — implementability, not
# just importability. A signature referencing an unaliased internal
# type fails HERE with the method named, instead of shipping.
#
# TWO legs:
#   default            proxy @latest — the exact shape an external
#                      adopter resolves (release truth; tags lag by
#                      design, same semantics as consumer-install.sh).
#                      Wired into the CI consumer job.
#   TQ_STORE_IMPL_REPLACE=1   replaces to the local tree (HEAD truth;
#                      the replace list is DERIVED live from examples/
#                      embed's go.mod block, extended with the facades
#                      this leg's own main.go imports (embed's graph
#                      misses some, e.g. journal), and that block's
#                      graph coverage is gate-pinned by
#                      scripts/check-embed-replace-sync.sh). Wired into
#                      ci-local so interface drift fires at the push, not
#                      at the tag wave.
#
#   scripts/smoke/consumer-store-impl.sh                 # @latest leg
#   TQ_STORE_IMPL_REPLACE=1 scripts/smoke/consumer-store-impl.sh  # HEAD leg
set -euo pipefail

MODULE="github.com/larsartmann/go-taskqueue"
repo_root="$(cd "$(dirname "$0")/../.." && pwd)"
verify_dir="$(mktemp -d)"
trap 'rm -rf "$verify_dir"' EXIT

leg="proxy @latest"
replace_block=""
if [ "${TQ_STORE_IMPL_REPLACE:-0}" = "1" ]; then
	leg="local-tree replaces (HEAD truth)"
	# Derived live from examples/embed/go.mod's replace block (ruling
	# 2026-11-03: KEEP-FOREVER) -- ONE hand-maintained graph list, not
	# two: every tq module in the transitive graph needs a line there,
	# or tidy resolves internals from a stale tag against HEAD facades.
	# The block's left side is the full module path; the right side is
	# rewritten to the repo-root DIRECTORY (module path minus the
	# $MODULE prefix). Coverage of that block is pinned by
	# scripts/check-embed-replace-sync.sh; an EMPTY derivation is
	# refused below (it would run this leg proxy-mixed, not HEAD-truth).
	mapfile -t replace_mods < <(cd "$repo_root/examples/embed" && sed -n '/^replace (/,/^)/p' go.mod | sed -e '1d' -e '$d' -e 's/=> .*$//' -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//' | grep -v '^$')
	if [ "${#replace_mods[@]}" -eq 0 ]; then
		echo "STORE-IMPL FAIL: EMPTY REPLACE LIST derived from examples/embed/go.mod -- the replace-block extraction found nothing (block shape changed?). Fix the extraction or restore embed's KEEP-FOREVER block; an empty block would run this leg proxy-mixed instead of HEAD-truth." >&2
		exit 1
	fi
	# Coverage for the leg's OWN imports: embed's block mirrors embed's
	# graph, and embed does not import every facade the Store surface
	# names (the journal facade today) -- an uncovered import would
	# resolve from the proxy and re-arm the exact proxy-mixed class
	# this leg exists to prevent. Derived from this script's main.go
	# heredoc, never frozen.
	imported_facades="$(sed -n "/^[[:space:]]*cat >main.go <<'EOF'/,/^EOF\$/p" "$0" | grep -oE "${MODULE//./\\.}/[a-z]+" | sed "s~^$MODULE/~~" | sort -u || true)"
	if [ -z "$imported_facades" ]; then
		echo "STORE-IMPL FAIL: imported-facade extraction found nothing in the main.go heredoc (shape changed?) -- an uncovered import would resolve from the proxy, not HEAD; fix the extraction." >&2
		exit 1
	fi
	for facade in $imported_facades; do
		mod="$MODULE/$facade"
		printf '%s\n' "${replace_mods[@]}" | grep -qxF "$mod" || replace_mods+=("$mod")
	done
	replace_block=$'replace (\n'
	for mod in "${replace_mods[@]}"; do
		replace_block+="	$mod => $repo_root/${mod#"$MODULE/"}"$'\n'
	done
	replace_block+=')'
fi

(
	cd "$verify_dir"
	export GOWORK=off
	export GOTOOLCHAIN=auto
	go mod init consumer >/dev/null
	go mod edit -go=1.27.1

	# The stub implements the FULL Store surface with exact signatures —
	# bodies stay unwritten panics: the compile is the assertion. When
	# the interface grows, the build fails naming the missing method
	# (that failure IS the guard firing); update the stub in the same
	# change that grows the contract. The only types the signatures may
	# name are facade-reachable ones (queue/task/journal aliases) plus
	# stdlib — anything else cannot compile from out of tree, which is
	# precisely the rot class this smoke exists to catch.
	cat >main.go <<'EOF'
package main

import (
	"context"
	"encoding/json/jsontext"
	"time"

	"github.com/larsartmann/go-taskqueue/journal"
	"github.com/larsartmann/go-taskqueue/queue"
	"github.com/larsartmann/go-taskqueue/task"
)

type storeStub struct{}

func (storeStub) Enqueue(ctx context.Context, n task.New) (task.Task, error) { panic("stub") }
func (storeStub) ClaimDue(ctx context.Context, owner string, lease time.Duration) (task.Task, queue.Claim, error) {
	panic("stub")
}
func (storeStub) ClaimDueTypes(ctx context.Context, owner string, lease time.Duration, types []string) (task.Task, queue.Claim, error) {
	panic("stub")
}
func (storeStub) Complete(ctx context.Context, id task.ID, claim queue.Claim, result jsontext.Value) error {
	panic("stub")
}
func (storeStub) Fail(ctx context.Context, id task.ID, claim queue.Claim, errText string, backoff time.Duration, evidence jsontext.Value) error {
	panic("stub")
}
func (storeStub) FailPermanent(ctx context.Context, id task.ID, claim queue.Claim, errText string, evidence jsontext.Value) error {
	panic("stub")
}
func (storeStub) Requeue(ctx context.Context, id task.ID, claim queue.Claim, errText string, delay time.Duration, resumeCloseout bool, class string, provider string) error {
	panic("stub")
}
func (storeStub) UpdatePendingPriority(ctx context.Context, id task.ID, newPriority int, source, reason string) error {
	panic("stub")
}
func (storeStub) Park(ctx context.Context, id task.ID, reason string, until time.Time, by string) error {
	panic("stub")
}
func (storeStub) RecordAnswer(ctx context.Context, id task.ID, ans queue.AnswerRecord) error {
	panic("stub")
}
func (storeStub) Heartbeat(ctx context.Context, id task.ID, claim queue.Claim, extend time.Duration) error {
	panic("stub")
}
func (storeStub) Cancel(ctx context.Context, id task.ID, reason string) error          { panic("stub") }
func (storeStub) CancelRunning(ctx context.Context, id task.ID, reason string) error   { panic("stub") }
func (storeStub) CancelRequested(ctx context.Context, id task.ID) (bool, error)        { panic("stub") }
func (storeStub) CancelOwned(ctx context.Context, id task.ID, claim queue.Claim) error { panic("stub") }
func (storeStub) MarkOrphaned(ctx context.Context, cutoff time.Time) (int, error)      { panic("stub") }
func (storeStub) RescueDead(ctx context.Context, id task.ID, maxAttempts int) error    { panic("stub") }
func (storeStub) DismissDead(ctx context.Context, id task.ID, reason, by string) error { panic("stub") }
func (storeStub) Get(ctx context.Context, id task.ID) (task.Task, error)               { panic("stub") }
func (storeStub) List(ctx context.Context, f queue.Filter) ([]task.Task, error)        { panic("stub") }
func (storeStub) Facts(ctx context.Context, after int64, limit int) ([]journal.Fact, error) {
	panic("stub")
}
func (storeStub) LastFacts(ctx context.Context, limit int) ([]journal.Fact, error) { panic("stub") }
func (storeStub) HeadSeq(ctx context.Context) (int64, error)                       { panic("stub") }
func (storeStub) Watermark(ctx context.Context, consumer string) (seq int64, exists bool, err error) {
	panic("stub")
}
func (storeStub) SaveWatermark(ctx context.Context, consumer string, seq int64) error { panic("stub") }
func (storeStub) SavePriorityScore(ctx context.Context, score queue.PriorityScore) error {
	panic("stub")
}
func (storeStub) PriorityScore(ctx context.Context, itemKey string) (queue.PriorityScore, bool, error) {
	panic("stub")
}
func (storeStub) PriorityScores(ctx context.Context) ([]queue.PriorityScore, error) { panic("stub") }
func (storeStub) DeletePriorityScores(ctx context.Context, itemKeys []string) (int64, error) {
	panic("stub")
}
func (storeStub) FactsForTask(ctx context.Context, id string, limit int) ([]journal.Fact, error) {
	panic("stub")
}
func (storeStub) ClaimCounts(ctx context.Context) (map[string]int, error) { panic("stub") }
func (storeStub) CountFacts(ctx context.Context, ftype journal.FactType, since time.Time) (int64, error) {
	panic("stub")
}
func (storeStub) FactsSince(ctx context.Context, ftype journal.FactType, since time.Time, limit int) ([]journal.Fact, error) {
	panic("stub")
}
func (storeStub) StatusCounts(ctx context.Context) (map[task.Status]int, error) { panic("stub") }
func (storeStub) ProjectCounts(ctx context.Context) (map[string]map[task.Status]int, error) {
	panic("stub")
}
func (storeStub) CountTasks(ctx context.Context, f queue.Filter) (int, error) { panic("stub") }
func (storeStub) Close() error                                                { panic("stub") }

var _ queue.Store = storeStub{}

func main() { _ = storeStub{} }
EOF

	if [ -n "$replace_block" ]; then
		printf '%s\n' "$replace_block" >>go.mod
		GOFLAGS=-mod=mod go mod tidy
	else
		go get "$MODULE/queue@latest" "$MODULE/task@latest" "$MODULE/journal@latest" >/dev/null
		GOFLAGS=-mod=mod go mod tidy
	fi
	go build -o /dev/null .
	echo "STORE-IMPL-OK: queue.Store implementable out-of-tree against the facades ($leg)"
)
