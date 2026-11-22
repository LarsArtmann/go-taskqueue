package dlqfix

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/executor"
)

// TestSweeperAutopsyLinkifiesFindingCites pins that an autopsy minted for
// a dead task whose prompt cites `#finding-N` carries the citation as a
// /task/<dead>#finding-N anchor link, so the autopsy agent lands on the
// exact finding row of the dead task's detail page (design-pass §f27).
func TestSweeperAutopsyLinkifiesFindingCites(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	sw := newTestSweeper(t, s)

	seedDeadAgentTask(t, s, executor.AgentPayload{
		Repo:   "demo",
		Prompt: "fix the offset flagged in #finding-2\n\nTask-Queue-ID: {{TASK_ID}}",
		Yolo:   true,
	})

	if _, err := sw.Sweep(context.Background()); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	tasks := pendingDLQFixTasks(t, s)
	if len(tasks) != 1 {
		t.Fatalf("dlqfix tasks = %d, want 1", len(tasks))
	}

	var payload executor.DLQFixPayload
	if err := json.Unmarshal(tasks[0].Payload, &payload); err != nil {
		t.Fatalf("autopsy payload: %v (%s)", err, tasks[0].Payload)
	}

	want := "[#finding-2](/task/" + payload.DeadTask + "#finding-2)"
	if !strings.Contains(payload.Work, want) {
		t.Fatalf("autopsy work missing linkified cite %s:\n%s", want, payload.Work)
	}
}
