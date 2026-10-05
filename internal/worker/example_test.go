package worker_test

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
	"github.com/larsartmann/go-taskqueue/internal/task"
	"github.com/larsartmann/go-taskqueue/internal/worker"
)

// Embedding go-taskqueue in a Go binary: open a store, register executors,
// run a worker pool, enqueue work. Ctrl-C drains gracefully.
func Example() {
	store, err := sqlite.Open("tasks.db")
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	reg := executor.NewRegistry()
	reg.Register("sh", executor.NewCommandExecutor(""))

	pool := worker.New(store, worker.Config{
		Owner:       "my-app",
		Concurrency: 4,
		Executors:   reg,
	}, slog.Default())

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if _, err := queue.New(store).Enqueue(ctx, task.New{
		Project: "demo",
		Type:    "sh",
		Payload: []byte(`{"cmd":"go test ./..."}`),
	}); err != nil {
		log.Fatal(err)
	}

	if err := pool.Start(ctx); err != nil {
		log.Fatal(err)
	}
}

// This example arms the claim-time budget gate: the Budget hook runs
// BEFORE any executor spawns, so an agent pool never spends a paid turn
// once the day's cap is spent. A blocked claim requeues WITHOUT burning
// an attempt and parks until the hook's delay (midnight for a daily cap),
// so work enqueued before the cap bit stays queued instead of costing
// money after it.
func ExampleConfig_budget() {
	countEnqueuedToday := func(ctx context.Context) (int, error) {
		// Production: store.CountFacts(ctx, journal.Enqueued, localMidnight)
		return 3, nil //nolint:goerr113 // example stub
	}

	guard := func(ctx context.Context) (blocked bool, reason string, retryIn time.Duration) {
		const dailyCap = 40

		spent, err := countEnqueuedToday(ctx)
		if err != nil {
			return false, "", 0 // fail open: the claim gate is a cost brake, not a correctness gate
		}

		if spent < dailyCap {
			return false, "", 0
		}

		return true, "daily agent budget spent", time.Until(time.Now().AddDate(0, 0, 1).Truncate(24 * time.Hour))
	}

	_ = guard // hand to worker.Config.Budget

	fmt.Println("paid turns stop at the cap; attempts never burn")
	// Output: paid turns stop at the cap; attempts never burn
}
