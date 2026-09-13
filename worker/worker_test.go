package worker_test

import (
	"log/slog"
	"testing"
	"time"

	tq "github.com/larsartmann/go-taskqueue/worker"
)

func TestFacadeSurface(t *testing.T) {
	p := tq.New(nil, tq.Config{Concurrency: 1, Lease: time.Second}, slog.Default())
	if p == nil {
		t.Fatal("New returned nil pool")
	}
	if p.InFlight() != 0 {
		t.Fatalf("InFlight = %d, want 0", p.InFlight())
	}
	if tq.ExpBackoff(1) <= 0 {
		t.Fatal("ExpBackoff must be positive")
	}
}
