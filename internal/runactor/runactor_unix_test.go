//go:build unix

package runactor

import (
	"context"
	"os"
	"syscall"
	"testing"
	"time"
)

func TestInterruptCancelsGracefully(t *testing.T) {
	g := New(context.Background())
	g.InterruptOn(os.Interrupt, syscall.SIGTERM)

	cancelled := make(chan struct{})

	g.Go("server", func(ctx context.Context) error {
		<-ctx.Done()
		close(cancelled)

		return nil
	})

	if err := syscall.Kill(syscall.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatalf("send SIGTERM: %v", err)
	}

	select {
	case <-cancelled:
	case <-time.After(2 * time.Second):
		t.Fatal("SIGTERM did not cancel the actor context")
	}

	if err := g.Run(); err != nil {
		t.Fatalf("interrupted Run = %v, want nil (graceful)", err)
	}
}
