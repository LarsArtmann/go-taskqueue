package main

import (
	"context"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/composition"
)

// TestProjectionPumpRunNilHostParksUntilDone pins the read-only-mount
// demotion contract (TODO row: eleventh dispatch; fixed by 14cb413a): a
// runtime that lost the fold-lock election mounts with Host nil, and
// projectionPump.Run must hold the pump open until ctx is done and return
// ctx.Err() — never start a host, never call notify, never SIGSEGV on the
// nil host (the second-serve crash class, smoke auth phase 2026-10-25).
func TestProjectionPumpRunNilHostParksUntilDone(t *testing.T) {
	t.Parallel()

	pump := &projectionPump{rt: &composition.ProjectionRuntime{}}

	ctx, cancel := context.WithCancel(context.Background())
	returned := make(chan error, 1)

	go func() { returned <- pump.Run(ctx, func(int64) {}) }()

	select {
	case err := <-returned:
		t.Fatalf("Run returned before ctx cancellation: %v — the nil-Host read-only mount must park, not exit", err)
	case <-time.After(100 * time.Millisecond):
	}

	cancel()

	select {
	case err := <-returned:
		if err != context.Canceled {
			t.Fatalf("Run returned %v after cancellation, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return within 2s of ctx cancellation")
	}
}
