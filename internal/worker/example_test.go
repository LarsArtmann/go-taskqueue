package worker_test

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"

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
