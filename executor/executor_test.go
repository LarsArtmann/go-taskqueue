package executor_test

import (
	"context"
	"errors"
	"testing"

	tq "github.com/larsartmann/go-taskqueue/executor"
	internaltask "github.com/larsartmann/go-taskqueue/internal/task"
)

func TestFacadeSurface(t *testing.T) {
	r := tq.NewRegistry()
	called := false

	r.RegisterFunc("noop", func(ctx context.Context, t internaltask.Task) error {
		called = true

		return nil
	})

	ex, err := r.Lookup("noop")
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}

	if err := ex.Execute(context.Background(), internaltask.Task{}); err != nil || !called {
		t.Fatalf("execute called=%v err=%v", called, err)
	}

	if _, err := r.Lookup("missing"); !errors.Is(err, tq.ErrUnknownType) {
		t.Fatalf("missing lookup = %v, want ErrUnknownType", err)
	}

	if tq.TaskTypeAgent != "agent" || tq.EvidenceTailBytes != 4096 {
		t.Fatal("const re-exports drifted")
	}
}
