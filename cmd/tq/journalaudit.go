package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// factsPageSize bounds each Facts read when draining the journal.
const factsPageSize = 1000

// replayState is one task's projection rebuilt from facts alone (ADR-0001:
// every state change is a fact; the queue view is a projection).
type replayState struct {
	status   task.Status
	attempts int
	priority *int
	dedupKey string
	// known flags guard legacy/thin facts: fields whose facts never
	// carried the value are not diffed (an absence of evidence is not
	// drift).
	priorityKnown bool
	dedupKnown    bool
}

// replayProjection rebuilds each task's state from the fact journal alone.
// Only facts that CARRY a state change move the projection; heartbeat,
// orphaned, cancel-requested and session.* facts are observations and are
// ignored.
func replayProjection(facts []journal.Fact) map[task.ID]*replayState {
	out := make(map[task.ID]*replayState)

	// maxAttempt raises the replayed attempt count: failed and
	// dead-lettered facts carry the post-increment attempt number.
	maxAttempt := func(id task.ID, attempt int) {
		state := out[id]
		if state == nil {
			state = &replayState{}
			out[id] = state
		}

		if attempt > state.attempts {
			state.attempts = attempt
		}
	}

	for _, fact := range facts {
		id := task.ID(fact.TaskID)

		switch fact.Type {
		case journal.Enqueued:
			state := out[id]
			if state == nil {
				state = &replayState{}
				out[id] = state
			}

			state.status = task.Pending
			// Plain enqueue starts the budget at zero; RescueDead's
			// re-emitted enqueue resets it (the store does attempts = 0).
			state.attempts = 0
			var detail queue.EnqueueDetail
			if err := json.Unmarshal(fact.Detail, &detail); err == nil {
				if detail.Priority != nil {
					p := *detail.Priority
					state.priority = &p
					state.priorityKnown = true
				}

				if detail.DedupKey != "" {
					state.dedupKey = detail.DedupKey
					state.dedupKnown = true
				}
			}
		case journal.Claimed:
			state := out[id]
			if state == nil {
				state = &replayState{}
				out[id] = state
			}

			state.status = task.Running
		case journal.Completed:
			out[id].status = task.Completed
		case journal.Failed:
			// task.failed alone means the attempt was recorded and the
			// task returned to Pending (attempts remained); exhaustion is
			// its own fact.
			maxAttempt(id, fact.Attempt)

			if state := out[id]; state != nil {
				state.status = task.Pending
			}
		case journal.DeadLettered:
			maxAttempt(id, fact.Attempt)

			if state := out[id]; state != nil {
				state.status = task.Dead
			}
		case journal.Cancelled:
			if state := out[id]; state != nil {
				state.status = task.Cancelled
			}
		case journal.Requeued:
			// preflight refusal: back to Pending, no attempt burned
			if state := out[id]; state != nil {
				state.status = task.Pending
			}
		case journal.Released:
			// lease expiry: back to Pending until reclaimed
			if state := out[id]; state != nil {
				state.status = task.Pending
			}
		case journal.Reprioritized:
			var evidence queue.ReprioritizeEvidence
			if err := json.Unmarshal(fact.Detail, &evidence); err == nil {
				state := out[id]
				if state == nil {
					state = &replayState{}
					out[id] = state
				}

				p := evidence.NewPriority
				state.priority = &p
				state.priorityKnown = true
			}
		case journal.Heartbeat, journal.CancelRequested, journal.Orphaned,
			journal.SessionOpened, journal.SessionClosed:
			// Observation facts: no state change.
		}
	}

	return out
}

// DriftRow is one field of one task whose stored value disagrees with the
// fact replay.
type DriftRow struct {
	TaskID   string `json:"taskId"`
	Field    string `json:"field"` // status | attempts | priority | dedup_key
	Stored   string `json:"stored"`
	Replayed string `json:"replayed"`
}

// FieldCoverage counts, per diffed field, how many compared tasks the
// journal could actually verify. Legacy journals (facts recorded before
// enqueue details carried priority/dedup_key) leave those counts below
// TasksCompared: sparseness there means "nothing to diff against", never
// hidden drift — the known-flags deliberately skip unverifiable fields.
type FieldCoverage struct {
	Status   int `json:"status"`
	Attempts int `json:"attempts"`
	Priority int `json:"priority"`
	DedupKey int `json:"dedupKey"`
}

// DriftReport is the journal-vs-store divergence result.
type DriftReport struct {
	TasksCompared int           `json:"tasksCompared"`
	FactsReplayed int           `json:"factsReplayed"`
	Coverage      FieldCoverage `json:"coverage"`
	Drift         []DriftRow    `json:"drift,omitempty"`
	// SecretEvidence rows are the secrets-in-logs audit (17-21 #18 / 20-58
	// f33): stored facts whose error text or evidence detail carries a
	// provider-token-shaped string — almost always written before the
	// executor's redaction pass shipped. Advisory; never includes the
	// matched secret itself.
	SecretEvidence []SecretHit `json:"secret_evidence,omitempty"`
}

// SecretHit locates token-shaped strings in one stored fact WITHOUT
// printing them: count and location only, the remedy is operator action
// (tq show <id>, or editing the journal), never a re-print.
type SecretHit struct {
	Seq    int64  `json:"seq"`
	TaskID string `json:"taskId"`
	Type   string `json:"type"`
	Field  string `json:"field"` // error | detail
	Count  int    `json:"count"`
}

// HasDrift reports whether any task state diverged.
func (r DriftReport) HasDrift() bool { return len(r.Drift) > 0 }

// diffProjection compares stored rows against the replayed projection,
// collecting drift rows and per-field coverage. Pure: no I/O, directly
// testable without a store.
func diffProjection(tasks []task.Task, replay map[task.ID]*replayState) DriftReport {
	report := DriftReport{TasksCompared: len(tasks)}

	for _, candidate := range tasks {
		want, ok := replay[candidate.ID]
		if !ok {
			// A stored task with NO enqueue fact is drift by definition.
			report.Drift = append(report.Drift, DriftRow{
				TaskID: string(candidate.ID), Field: "status",
				Stored: string(candidate.Status), Replayed: "(no facts)",
			})

			continue
		}

		report.Coverage.Status++
		report.Coverage.Attempts++

		if want.priorityKnown {
			report.Coverage.Priority++
		}

		if want.dedupKnown {
			report.Coverage.DedupKey++
		}

		if want.status != candidate.Status {
			report.Drift = append(report.Drift, DriftRow{
				TaskID: string(candidate.ID), Field: "status",
				Stored: string(candidate.Status), Replayed: string(want.status),
			})
		}

		if want.attempts != candidate.Attempts {
			report.Drift = append(report.Drift, DriftRow{
				TaskID: string(candidate.ID), Field: "attempts",
				Stored: strconv.Itoa(candidate.Attempts), Replayed: strconv.Itoa(want.attempts),
			})
		}

		if want.priorityKnown && (want.priority == nil || *want.priority != candidate.Priority) {
			replayed := "(unknown)"
			if want.priority != nil {
				replayed = strconv.Itoa(*want.priority)
			}

			report.Drift = append(report.Drift, DriftRow{
				TaskID: string(candidate.ID), Field: "priority",
				Stored: strconv.Itoa(candidate.Priority), Replayed: replayed,
			})
		}

		if want.dedupKnown && want.dedupKey != candidate.DedupKey {
			report.Drift = append(report.Drift, DriftRow{
				TaskID: string(candidate.ID), Field: "dedup_key",
				Stored: candidate.DedupKey, Replayed: want.dedupKey,
			})
		}
	}

	sort.Slice(report.Drift, func(i, j int) bool {
		if report.Drift[i].TaskID != report.Drift[j].TaskID {
			return report.Drift[i].TaskID < report.Drift[j].TaskID
		}

		return report.Drift[i].Field < report.Drift[j].Field
	})

	return report
}

// journalDrift rebuilds task state from the journal and diffs it against
// the stored tasks table (status, attempts, priority, dedup key). Advisory
// by contract: the caller decides whether divergence fails anything.
func journalDrift(ctx context.Context, store queue.Store) (DriftReport, error) {
	var facts []journal.Fact

	after := int64(0)

	for {
		batch, err := store.Facts(ctx, after, factsPageSize)
		if err != nil {
			return DriftReport{}, fmt.Errorf("read facts after %d: %w", after, err)
		}

		facts = append(facts, batch...)
		if len(batch) < factsPageSize {
			break
		}

		after = batch[len(batch)-1].Seq
	}

	tasks, err := store.List(ctx, queue.Filter{})
	if err != nil {
		return DriftReport{}, fmt.Errorf("list tasks: %w", err)
	}

	report := diffProjection(tasks, replayProjection(facts))
	report.FactsReplayed = len(facts)
	report.SecretEvidence = scanFactSecrets(facts)

	return report, nil
}

// scanFactSecrets runs the secrets-in-logs detector over the evidence-bearing
// fields (error text + detail JSON) of the OUTPUT-derived fact types and
// returns one row per fact/field with hits. Payloads are NOT scanned:
// whatever an enqueuer stored there was provided intentionally, not leaked
// through an output tail.
func scanFactSecrets(facts []journal.Fact) []SecretHit {
	evidenceCarriers := map[journal.FactType]bool{
		journal.Failed:       true,
		journal.DeadLettered: true,
		journal.Completed:    true,
		journal.Requeued:     true,
	}

	var hits []SecretHit

	for _, fact := range facts {
		if !evidenceCarriers[fact.Type] {
			continue
		}

		for field, content := range map[string]string{
			"error":  fact.Error,
			"detail": string(fact.Detail),
		} {
			if n := executor.SecretHits(content); n > 0 {
				hits = append(hits, SecretHit{
					Seq:    fact.Seq,
					TaskID: fact.TaskID,
					Type:   string(fact.Type),
					Field:  field,
					Count:  n,
				})
			}
		}
	}

	sort.Slice(hits, func(i, j int) bool { return hits[i].Seq < hits[j].Seq })

	return hits
}

// cmdJournalAudit runs the drift audit and renders it (text or JSON).
// Advisory-first (TODO row, 08-01 §f5): divergence prints loudly but never
// fails the command.
func cmdJournalAudit(ctx context.Context, store queue.Store, asJSON bool) error {
	report, err := journalDrift(ctx, store)
	if err != nil {
		return err
	}

	if asJSON {
		return json.NewEncoder(os.Stdout).Encode(report)
	}

	fmt.Printf("journal drift audit: %d task(s) compared against %d fact(s) (status, attempts, priority, dedup key)\n",
		report.TasksCompared, report.FactsReplayed)
	fmt.Printf("coverage: status %d/%d, attempts %d/%d, priority %d/%d, dedup key %d/%d"+
		" (below-total priority/dedup means legacy facts predate enrichment)\n",
		report.Coverage.Status, report.TasksCompared,
		report.Coverage.Attempts, report.TasksCompared,
		report.Coverage.Priority, report.TasksCompared,
		report.Coverage.DedupKey, report.TasksCompared)

	if !report.HasDrift() {
		fmt.Println("no drift: stored projections equal the fact replay (ADR-0001 invariant holds)")
	} else {
		fmt.Printf("DRIFT: %d field(s) diverge between the tasks table and the fact journal:\n", len(report.Drift))

		for _, row := range report.Drift {
			fmt.Printf("  %s: %s stored=%s replayed=%s\n", row.TaskID, row.Field, row.Stored, row.Replayed)
		}
	}

	if len(report.SecretEvidence) == 0 {
		fmt.Println("secret scan: no provider-token-shaped strings in fact evidence")
	} else {
		fmt.Printf(
			"SECRET EVIDENCE: %d fact field(s) carry provider-token-shaped strings:\n",
			len(report.SecretEvidence),
		)

		for _, hit := range report.SecretEvidence {
			fmt.Printf("  seq=%d %s %s field=%s hits=%d\n", hit.Seq, hit.Type, hit.TaskID, hit.Field, hit.Count)
		}

		fmt.Println(
			"  (advisory: facts written before the redaction pass are the likely source; inspect with `tq show <id>`, never re-print the secret)",
		)
	}

	fmt.Println("(advisory: investigate with `tq show <id>` and `tq facts -detail` before repairing)")

	return nil
}
