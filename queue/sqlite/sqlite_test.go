package sqlite_test

import (
	"context"
	"testing"

	internalqueue "github.com/larsartmann/go-taskqueue/internal/queue"
	internaltask "github.com/larsartmann/go-taskqueue/internal/task"
	tq "github.com/larsartmann/go-taskqueue/queue/sqlite"
)

func TestFacadeSurface(t *testing.T) {
	s, err := tq.Open(t.TempDir()+"/tq.db", tq.WithProjectExclusivity())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	ctx := context.Background()
	n, err := s.Enqueue(ctx, internaltask.New{Type: "sh", Project: "facade", Payload: []byte("true")})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	got, err := s.Get(ctx, n.ID)
	if err != nil || got.ID != n.ID {
		t.Fatalf("get = %v/%v", got.ID, err)
	}
}

func TestStoreSatisfiesContract(t *testing.T) {
	var _ internalqueue.Store = &tq.Store{}
}
