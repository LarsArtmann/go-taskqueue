//go:build unix

package e2e

import (
	"context"
	"path/filepath"
	"sync"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/budget"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// TestBudgetCountIsSerializedAcrossPools pins turnstone's "authorize the set"
// concern against tq's budget projection: SpentToday is a COUNT over enqueued
// facts (single-writer serialized), not a mutable per-pool counter, so two
// pools over one database can never disagree about the day's spend, and no
// read-then-write overdraw is possible by construction.
func TestBudgetCountIsSerializedAcrossPools(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	db := filepath.Join(dir, "q.db")

	a := openStore(t, db)
	defer func() { _ = a.Close() }()

	b := openStore(t, db)
	defer func() { _ = b.Close() }()

	ctx := context.Background()

	const enqueues = 20

	var wg sync.WaitGroup

	errs := make(chan error, enqueues)

	for i := range enqueues {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			store := a
			if i%2 == 0 {
				store = b
			}

			if _, err := store.Enqueue(ctx, task.New{Type: "sh", Payload: []byte(`"true"`)}); err != nil {
				errs <- err
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Fatalf("concurrent enqueue over two handles: %v", err)
	}

	guard := budget.Guard{DailyCap: 1000}

	spentA := guard.SpentToday(ctx, a)
	spentB := guard.SpentToday(ctx, b)

	if spentA != enqueues || spentB != enqueues {
		t.Fatalf("SpentToday a=%d b=%d, want %d/%d (shared serialized projection)",
			spentA, spentB, enqueues, enqueues)
	}
}
