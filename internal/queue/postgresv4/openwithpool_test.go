package postgresv4

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/larsartmann/go-taskqueue/internal/task"
)

// TestOpenWithPoolCallerOwned pins the pool-ownership contract preserved
// from internal/queue/postgres at the S1 flip: the store works, the
// schema is applied, and Close leaves the caller's pool usable.
func TestOpenWithPoolCallerOwned(t *testing.T) {
	dsn := testDSN(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	defer pool.Close()

	s, err := OpenWithPool(ctx, pool)
	if err != nil {
		t.Fatalf("OpenWithPool: %v", err)
	}

	if _, err := s.Enqueue(ctx, task.New{Type: "sh", Project: "pool-ownership", Payload: []byte("true")}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	if err := s.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	if err := pool.Ping(ctx); err != nil {
		t.Fatal("Close must not tear down a caller-owned pool")
	}
}
