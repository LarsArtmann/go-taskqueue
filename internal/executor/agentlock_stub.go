//go:build !unix

package executor

import "context"

// acquireAgentSlot is a no-op on non-unix platforms: the machine-wide agent
// cap uses flock, which has no portable Go API on Windows (the same honesty
// as the missing process-group kill — the Windows test job documents the
// gap). Pools on those platforms rely on per-pool --concurrency only.
func acquireAgentSlot(_ context.Context, _ int) (func(), error) {
	return func() {}, nil
}
