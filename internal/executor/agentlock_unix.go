//go:build unix

package executor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// agentSlotDir is where machine-wide agent slot files live. It is shared by
// every tq process on the host (pools, workers) so the cap holds across
// processes, not just inside one pool.
func agentSlotDir() string {
	if d := os.Getenv("TQ_AGENT_SLOT_DIR"); d != "" {
		return d
	}

	return filepath.Join(os.TempDir(), "tq-agent-slots")
}

// acquireAgentSlot blocks until one of max machine-wide agent slots is free
// and returns its release func. Slots are flock'd files: a crashed process
// loses its flocks automatically (the kernel closes the fds), so a SIGKILLed
// pool never leaks slots. ctx cancellation aborts the wait.
func acquireAgentSlot(ctx context.Context, max int) (func(), error) {
	if max <= 0 {
		return func() {}, nil
	}

	dir := agentSlotDir()

	if err := os.MkdirAll(dir, 0o1777); err != nil {
		return nil, fmt.Errorf("agent slot dir: %w", err)
	}

	for {
		for slot := 1; slot <= max; slot++ {
			path := filepath.Join(dir, fmt.Sprintf("slot-%d.lock", slot))

			f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o666)
			if err != nil {
				return nil, fmt.Errorf("agent slot file: %w", err)
			}

			// LOCK_EX|LOCK_NB: take the slot or move to the next one.
			if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
				_ = f.Close()
				continue
			}

			released := false

			return func() {
				if released {
					return
				}

				released = true

				_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
				_ = f.Close()
			}, nil
		}

		// All slots busy: wait for a release (or shutdown) and retry.
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("agent slots: %w", ctx.Err())
		case <-time.After(200 * time.Millisecond):
		}
	}
}
