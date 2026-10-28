// Package retryx owns the shared transient-retry file-operation helpers
// that multiple modules need for the Windows delayed-release class: a
// Rename over a path another process still holds open fails with a
// sharing violation for a short window (proven live by the 2026-10-25
// test-windows CI red on TestConvergeProjectionHome); a short bounded
// retry rides it out. Each helper graduates here instead of growing a
// copy per module (06-11 §e1/§f14).
package retryx

import (
	"context"
	"os"
	"time"

	"github.com/larsartmann/go-retry"
)

// RenameTransient renames src to dst with a short bounded transient
// retry (3 attempts, 50ms initial, 100ms cap, 2× multiplier) so a
// Windows sharing violation on a recently closed reader heals instead
// of failing the caller. A nonexistent source is tolerated as success
// (callers that must observe absence stat first).
func RenameTransient(src, dst string) error {
	return retry.Do(context.Background(), retry.Config{ //nolint:exhaustruct // optional hooks unset
		MaxAttempts:  3,
		InitialDelay: 50 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
	}, func(_ context.Context, _ int) error {
		err := os.Rename(src, dst)
		if os.IsNotExist(err) {
			return nil
		}

		return err
	})
}
