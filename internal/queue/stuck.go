package queue

import (
	"context"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/task"
)

// CountStuckRunning counts Running tasks whose lease expired without a
// reclaim — work that WOULD run if any worker were claiming. It is the one
// expired-lease probe shared by `tq doctor` and the webui health dashboard,
// so the two operator surfaces cannot drift. A list error counts zero:
// callers surface store failures through their own checks.
func CountStuckRunning(ctx context.Context, store Store, now time.Time) int {
	running := task.Running

	tasks, err := store.List(ctx, Filter{Status: &running})
	if err != nil {
		return 0
	}

	stuck := 0

	for _, t := range tasks {
		if t.LeaseExpires != nil && t.LeaseExpires.Before(now) {
			stuck++
		}
	}

	return stuck
}
