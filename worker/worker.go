// Package worker is the public facade over the worker-pool module
// (ADR-0016): the claim→heartbeat→execute loop, its Config, and the Pool.
// The implementation stays internal; this package pins the names.
package worker

import internalworker "github.com/larsartmann/go-taskqueue/internal/worker"

type (
	Config = internalworker.Config
	Pool   = internalworker.Pool
)

var (
	New        = internalworker.New
	ExpBackoff = internalworker.ExpBackoff
)
