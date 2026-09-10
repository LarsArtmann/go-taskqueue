// Command fullcore is the full-core embed story in one file: it wires the
// queue, an executor registry, and a worker pool together as a library —
// no tq binary involved — and proves the backend-choice import: pick
// --backend sqlite (default, single file) or --backend postgres (shared
// queue across machines). Both stores satisfy queue.Store, so the rest of
// the program is backend-agnostic.
//
// Usage:
//
//	go run ./examples/fullcore
//	go run ./examples/fullcore --backend postgres --dsn postgres://127.0.0.1:5432/taskqueue?sslmode=disable
//
// The demo enqueues a few shell tasks plus a custom in-process executor
// task, runs the pool until the queue drains, and prints the final status
// counts. Keep --backend postgres pointed at a local/test database; the
// example applies the store schema to it.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/queue/postgres"
	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
	"github.com/larsartmann/go-taskqueue/internal/task"
	"github.com/larsartmann/go-taskqueue/internal/worker"
)

func main() {
	backend := flag.String("backend", "sqlite", "queue backend: sqlite or postgres")
	db := flag.String("db", "fullcore.db", "sqlite database path (backend=sqlite)")
	dsn := flag.String("dsn", "postgres://127.0.0.1:5432/taskqueue?sslmode=disable", "postgres DSN (backend=postgres)")
	concurrency := flag.Int("concurrency", 2, "parallel task executions")
	timeout := flag.Duration("timeout", time.Minute, "overall drain deadline")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	// Backend choice: the import above IS the choice. Both types implement
	// queue.Store, so everything below compiles against either one.
	var store queue.Store
	switch *backend {
	case "sqlite":
		s, err := sqlite.Open(*db)
		if err != nil {
			log.Fatal(err)
		}

		defer s.Close()
		store = s
	case "postgres":
		s, err := postgres.Open(ctx, *dsn, 0)
		if err != nil {
			log.Fatalf("postgres backend: %v", err)
		}

		defer s.Close()
		store = s
	default:
		log.Fatalf("unknown --backend %q (want sqlite or postgres)", *backend)
	}

	q := queue.New(store)

	// Producers: plain enqueue calls, same shapes the tq CLI uses.
	demos := []task.New{
		{Project: "demo", Type: "sh", Payload: json.RawMessage(`{"cmd":"echo hello from fullcore"}`)},
		{Project: "demo", Type: "sh", Payload: json.RawMessage(`{"cmd":"echo second shell task"}`)},
		{Project: "demo", Type: "greet", Payload: json.RawMessage(`{"name":"embedder"}`)},
		{Project: "demo", Type: "flaky", Payload: json.RawMessage(`"attempt 1 fails, attempt 2 succeeds"`)},
	}

	for i, n := range demos {
		if _, err := q.Enqueue(ctx, n); err != nil {
			log.Fatalf("enqueue demo %d: %v", i, err)
		}
	}

	// Executors: register task types on a plain registry. "sh" delegates to
	// the built-in shell executor; "greet" and "flaky" are custom in-process
	// executors — any Go function becomes a runnable task type.
	executors := executor.NewRegistry()
	executors.Register("sh", executor.NewCommandExecutor(""))
	executors.RegisterFunc("greet", func(_ context.Context, t task.Task) error {
		var v struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(t.Payload, &v); err != nil {
			return err
		}

		fmt.Printf("greet: hello, %s (task %s)\n", v.Name, t.ID)
		return nil
	})
	flaky := 0
	executors.RegisterFunc("flaky", func(_ context.Context, _ task.Task) error {
		flaky++
		if flaky == 1 {
			return fmt.Errorf("deliberate first-attempt failure")
		}

		fmt.Println("flaky: succeeded on retry")
		return nil
	})

	pool := worker.New(store, worker.Config{
		Concurrency: *concurrency,
		Executors:   executors,
	}, slog.Default())

	done := make(chan struct{})
	go func() {
		_ = pool.Start(ctx)
		close(done)
	}()

	// Consumer-side observation: poll the store projections until the
	// queue drains (or the deadline hits).
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for drained := false; !drained; {
		select {
		case <-ctx.Done():
			log.Fatal("deadline exceeded before the queue drained")
		case <-done:
			return
		case <-ticker.C:
			tasks, err := store.List(ctx, queue.Filter{})
			if err != nil {
				log.Fatal(err)
			}

			counts := map[task.Status]int{}
			for _, t := range tasks {
				counts[t.Status]++
			}

			drained = counts[task.Pending] == 0 && counts[task.Running] == 0
		}
	}

	cancel()
	<-done

	report(store)
}

func report(store queue.Store) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tasks, err := store.List(ctx, queue.Filter{})
	if err != nil {
		log.Fatal(err)
	}

	counts := map[task.Status]int{}
	for _, t := range tasks {
		counts[t.Status]++

		if t.LastError != "" {
			fmt.Printf("%s (%s): last error: %s\n", t.ID, t.Type, t.LastError)
		}
	}

	fmt.Println("drained; final counts:")
	for _, s := range []task.Status{task.Completed, task.Pending, task.Running, task.Dead, task.Cancelled} {
		if counts[s] > 0 {
			fmt.Printf("  %-9s %d\n", s, counts[s])
		}
	}
}
