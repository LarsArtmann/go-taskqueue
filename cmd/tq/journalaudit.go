package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// replayStatuses rebuilds each task's status from the fact journal alone
// (ADR-0001: every state change is a fact; the queue view is a projection).
// Only facts that CARRY a state change move the projection; heartbeat,
// orphaned, cancel-requested, reprioritized and session.* facts are
// observations and are ignored.
func replayStatuses(facts []journal.Fact) map[task.ID]task.Status {
	out := make(map[task.ID]task.Status)

	for _, fact := range facts {
		switch fact.Type {
		case journal.Enqueued:
			out[task.ID(fact.TaskID)] = task.Pending
		case journal.Claimed:
			out[task.ID(fact.TaskID)] = task.Running
		case journal.Completed:
			out[task.ID(fact.TaskID)] = task.Completed
		case journal.Failed:
			// task.failed alone means the attempt was recorded and the
			// task returned to Pending (attempts remained); exhaustion is
			// its own fact.
			out[task.ID(fact.TaskID)] = task.Pending
		case journal.DeadLettered:
			out[task.ID(fact.TaskID)] = task.Dead
		case journal.Cancelled:
			out[task.ID(fact.TaskID)] = task.Cancelled
		case journal.Requeued:
			// preflight refusal: back to Pending, no attempt burned
			out[task.ID(fact.TaskID)] = task.Pending
		case journal.Released:
			// lease expiry: back to Pending until reclaimed
			out[task.ID(fact.TaskID)] = task.Pending
		case journal.Heartbeat, journal.CancelRequested, journal.Orphaned,
			journal.Reprioritized, journal.SessionOpened, journal.SessionClosed:
			// Observation facts: no state change.
		}
	}
	return out
}

// DriftRow is one task whose stored status disagrees with the fact replay.
type DriftRow struct {
	TaskID       string `json:"taskId"`
	StoredStatus string `json:"stored"`
	Replayed     string `json:"replayed"`
}

// DriftReport is the journal-vs-store divergence result.
type DriftReport struct {
	TasksCompared int        `json:"tasksCompared"`
	FactsReplayed int        `json:"factsReplayed"`
	Drift         []DriftRow `json:"drift,omitempty"`
}

// HasDrift reports whether any task state diverged.
func (r DriftReport) HasDrift() bool { return len(r.Drift) > 0 }

// journalDrift rebuilds task state from the journal and diffs it against
// the stored tasks table. Advisory by contract: the caller decides whether
// divergence fails anything.
func journalDrift(ctx context.Context, s queue.Store) (DriftReport, error) {
	var facts []journal.Fact
	after := int64(0)
	for {
		batch, err := s.Facts(ctx, after, 1000)
		if err != nil {
			return DriftReport{}, fmt.Errorf("read facts after %d: %w", after, err)
		}
		facts = append(facts, batch...)
		if len(batch) < 1000 {
			break
		}
		after = batch[len(batch)-1].Seq
	}

	tasks, err := s.List(ctx, queue.Filter{})
	if err != nil {
		return DriftReport{}, fmt.Errorf("list tasks: %w", err)
	}

	replay := replayStatuses(facts)
	report := DriftReport{TasksCompared: len(tasks), FactsReplayed: len(facts)}

	for _, tk := range tasks {
		want, ok := replay[tk.ID]
		if !ok {
			// A stored task with NO enqueue fact is drift by definition.
			report.Drift = append(report.Drift, DriftRow{
				TaskID: string(tk.ID), StoredStatus: string(tk.Status), Replayed: "(no facts)",
			})

			continue
		}

		if want != tk.Status {
			report.Drift = append(report.Drift, DriftRow{
				TaskID: string(tk.ID), StoredStatus: string(tk.Status), Replayed: string(want),
			})
		}
	}

	sort.Slice(report.Drift, func(i, j int) bool { return report.Drift[i].TaskID < report.Drift[j].TaskID })

	return report, nil
}

// cmdJournalAudit runs the drift audit and renders it (text or JSON).
// Advisory-first (TODO row, 08-01 §f5): divergence prints loudly but never
// fails the command.
func cmdJournalAudit(ctx context.Context, s queue.Store, asJSON bool) error {
	report, err := journalDrift(ctx, s)
	if err != nil {
		return err
	}
	if asJSON {
		return json.NewEncoder(os.Stdout).Encode(report)
	}
	fmt.Printf("journal drift audit: %d task(s) compared against %d fact(s)\n",
		report.TasksCompared, report.FactsReplayed)
	if !report.HasDrift() {
		fmt.Println("no drift: stored statuses equal the fact replay (ADR-0001 invariant holds)")
		return nil
	}

	fmt.Printf("DRIFT: %d task(s) diverge between the tasks table and the fact journal:\n", len(report.Drift))
	for _, row := range report.Drift {
		fmt.Printf("  %s: stored=%s replayed=%s\n", row.TaskID, row.StoredStatus, row.Replayed)
	}

	fmt.Println("(advisory: investigate with `tq show <id>` and `tq facts --task <id>` before repairing)")
	return nil
}
