// Command embed is the adopter on-ramp: the same embed story as
// examples/fullcore, but importing ONLY the public facade modules
// (ADR-0016) — exactly the import lines an external consumer writes.
// Nothing here may import internal/…; if you find yourself adding such
// an import, the facade is missing a name (fix the facade, not this
// example — scripts/check-facade-parity.sh pins the contract).
//
// Usage:
//
//	go run ./examples/embed
//	go run ./examples/embed --backend postgres --dsn postgres://tq@127.0.0.1:55432/tqtest?sslmode=disable
//
// The demo enqueues three tasks (one deliberately failing first attempt),
// runs a two-slot worker pool until the queue drains, and prints final
// status counts. Postgres mode demonstrates postgres.OpenWithPool: the
// example owns the pool, and store.Close leaves it running.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/larsartmann/go-taskqueue/executor"
	"github.com/larsartmann/go-taskqueue/queue"
	"github.com/larsartmann/go-taskqueue/queue/postgres"
	"github.com/larsartmann/go-taskqueue/queue/sqlite"
	"github.com/larsartmann/go-taskqueue/task"
	"github.com/larsartmann/go-taskqueue/worker"
)

func main() {
	backend := flag.String("backend", "sqlite", "sqlite or postgres")
	dbPath := flag.String("db", "", "sqlite file (sqlite backend)")
	dsn := flag.String("dsn", "", "postgres DSN (postgres backend)")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var store queue.Store

	switch *backend {
	case "sqlite":
		if *dbPath == "" {
			f, err := os.CreateTemp("", "tq-embed-*.db")
			if err != nil {
				log.Fatal(err)
			}

			*dbPath = f.Name()
			_ = f.Close()

			defer os.Remove(*dbPath)
		}

		s, err := sqlite.Open(*dbPath)
		if err != nil {
			log.Fatal(err)
		}

		defer s.Close()

		store = s
	case "postgres":
		if *dsn == "" {
			log.Fatal("--dsn is required for --backend postgres")
		}

		pool, err := pgxpool.New(ctx, *dsn)
		if err != nil {
			log.Fatal(err)
		}

		defer pool.Close()

		s, err := postgres.OpenWithPool(ctx, pool)
		if err != nil {
			log.Fatal(err)
		}

		// The pool is OURS: s.Close() would not have closed it, so the
		// deferred pool.Close() above stays the single shutdown point.
		store = s
	default:
		log.Fatalf("unknown --backend %q (want sqlite or postgres)", *backend)
	}

	executors := executor.NewRegistry()
	executors.RegisterFunc("greet", func(_ context.Context, t task.Task) error {
		fmt.Printf("greet: payload %s (task %s)\n", string(t.Payload), t.ID)

		return nil
	})

	attempts := 0
	executors.RegisterFunc("flaky", func(_ context.Context, _ task.Task) error {
		attempts++
		if attempts == 1 {
			return errors.New("deliberate first-attempt failure")
		}

		fmt.Println("flaky: succeeded on retry")

		return nil
	})

	q := queue.New(store)

	for i := range 2 {
		if _, err := q.Enqueue(ctx, task.New{
			Type:    "greet",
			Project: "embed-demo",
			Payload: []byte(fmt.Sprintf("job %d", i)),
		}); err != nil {
			log.Fatal(err)
		}
	}

	if _, err := q.Enqueue(ctx, task.New{Type: "flaky", Project: "embed-demo"}); err != nil {
		log.Fatal(err)
	}

	pool := worker.New(store, worker.Config{
		Concurrency: 2,
		Executors:   executors,
	}, slog.Default())

	done := make(chan struct{})

	go func() {
		_ = pool.Start(ctx)

		close(done)
	}()

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for drained := false; !drained; {
		select {
		case <-ctx.Done():
			log.Fatal("deadline exceeded before the queue drained")
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

	reportCtx, reportCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer reportCancel()

	tasks, err := store.List(reportCtx, queue.Filter{})
	if err != nil {
		log.Fatal(err)
	}

	counts := map[task.Status]int{}
	for _, t := range tasks {
		counts[t.Status]++
	}

	fmt.Printf("drained: %d completed, %d dead\n", counts[task.Completed], counts[task.Dead])

	if counts[task.Completed] != 3 || counts[task.Dead] != 0 {
		log.Fatal("unexpected final counts")
	}
}
