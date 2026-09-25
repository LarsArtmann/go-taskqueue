package status

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/queue"
)

func TestDbgFinishLoop(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	sw := newSweeperOrDie(t, s, 2)
	ctx := context.Background()

	runAgentTask(t, s, 0, executor.AgentResult{})
	runAgentTask(t, s, 1, executor.AgentResult{})
	if _, err := sw.Sweep(ctx); err != nil {
		t.Fatalf("sweep1: %v", err)
	}
	runAgentTask(t, s, 2, executor.AgentResult{})
	runAgentTask(t, s, 3, executor.AgentResult{})
	if _, err := sw.Sweep(ctx); err != nil {
		t.Fatalf("sweep2: %v", err)
	}

	allType := ""
	all, err := s.List(ctx, queue.Filter{Type: &allType})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, tk := range all {
		t.Logf("ROW id=%s type=%s status=%s dedup=%q attempts=%d", tk.ID, tk.Type, tk.Status, tk.DedupKey, tk.Attempts)
	}
	_ = json.Marshal
}
