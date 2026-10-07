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
// replayStateFor returns the task's projection entry, creating it on
// first sight — the get-or-create every fact arm of replayProjection
// shares.
func replayStateFor(out map[task.ID]*replayState, id task.ID) *replayState {
	state := out[id]
	if state == nil {
		state = &replayState{}
		out[id] = state
	}

	return state
}

// replaySetStatus moves an already-known task's status; ids the journal
// never introduced stay ignored instead of being conjured into existence.
func replaySetStatus(out map[task.ID]*replayState, id task.ID, status task.Status) {
	if state := out[id]; state != nil {
		state.status = status
	}
}

func replayProjection(facts []journal.Fact) map[task.ID]*replayState {
	out := make(map[task.ID]*replayState)

	// maxAttempt raises the replayed attempt count: failed and
	// dead-lettered facts carry the post-increment attempt number.
	maxAttempt := func(id task.ID, attempt int) {
		state := replayStateFor(out, id)

		if attempt > state.attempts {
			state.attempts = attempt
		}
	}

	for _, fact := range facts {
		id := task.ID(fact.TaskID)

		switch fact.Type {
		case journal.Enqueued:
			state := replayStateFor(out, id)

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
			state := replayStateFor(out, id)

			state.status = task.Running
		case journal.Completed:
			out[id].status = task.Completed
		case journal.Failed:
			// task.failed alone means the attempt was recorded and the
			// task returned to Pending (attempts remained); exhaustion is
			// its own fact.
			maxAttempt(id, fact.Attempt)

			replaySetStatus(out, id, task.Pending)
		case journal.DeadLettered:
			maxAttempt(id, fact.Attempt)

			replaySetStatus(out, id, task.Dead)
		case journal.Cancelled:
			replaySetStatus(out, id, task.Cancelled)
		case journal.Requeued:
			// preflight refusal: back to Pending, no attempt burned
			replaySetStatus(out, id, task.Pending)
		case journal.Released:
			// lease expiry: back to Pending until reclaimed
			replaySetStatus(out, id, task.Pending)
		case journal.Reprioritized:
			var evidence queue.ReprioritizeEvidence
			if err := json.Unmarshal(fact.Detail, &evidence); err == nil {
				state := replayStateFor(out, id)

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
	// Requeues summarizes the task.requeued facts in the replayed range
	// (01-37 ask): which not-the-task's-fault refusal family returned
	// tasks to Pending, and how many parked an owed close-out. Zero-value
	// when the range carries no requeues.
	Requeues RequeueSummary `json:"requeues"`
}

// RequeueSummary breaks a journal's task.requeued facts down by refusal
// class (queue.RequeueClass*; legacy facts without a class normalize to
// unknown) and counts the resume-closeout parks.
type RequeueSummary struct {
	Total          int            `json:"total"`
	ByClass        map[string]int `json:"byClass,omitempty"`
	ResumeCloseout int            `json:"resumeCloseout,omitempty"`
}

// requeueSummary scans the fact range for task.requeued facts and
// aggregates them by class. Pure: no I/O.
func requeueSummary(facts []journal.Fact) RequeueSummary {
	var summary RequeueSummary

	for _, fact := range facts {
		if fact.Type != journal.Requeued {
			continue
		}

		summary.Total++

		var evidence queue.RequeueEvidence
		if err := json.Unmarshal(fact.Detail, &evidence); err != nil {
			continue
		}

		class := evidence.Class
		if class == "" {
			class = queue.RequeueClassUnknown
		}

		if summary.ByClass == nil {
			summary.ByClass = make(map[string]int)
		}

		summary.ByClass[class]++

		if evidence.ResumeCloseout {
			summary.ResumeCloseout++
		}
	}

	return summary
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
	report.Requeues = requeueSummary(facts)

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
	fmt.Printf("coverage: status %d/%d, attempts %d/%d, priority %d/%d, dedup key %d/%d\n",
		report.Coverage.Status, report.TasksCompared,
		report.Coverage.Attempts, report.TasksCompared,
		report.Coverage.Priority, report.TasksCompared,
		report.Coverage.DedupKey, report.TasksCompared)

	switch {
	case report.TasksCompared > 0 && report.Coverage.Priority == 0 && report.Coverage.DedupKey == 0:
		fmt.Println(
			"priority/dedup key: NOT REPLAYABLE — engine enqueue details are thin {project,type}" +
				" (S1 divergence D2, M4-gated); 0/N does NOT mean the tasks lack priorities",
		)
	case report.Coverage.Priority < report.TasksCompared || report.Coverage.DedupKey < report.TasksCompared:
		fmt.Println("(below-total priority/dedup means legacy facts predate enrichment)")
	}

	if !report.HasDrift() {
		fmt.Println("no drift: stored projections equal the fact replay (ADR-0001 invariant holds)")
	} else {
		fmt.Printf("DRIFT: %d field(s) diverge between the tasks table and the fact journal:\n", len(report.Drift))

		for _, row := range report.Drift {
			fmt.Printf("  %s: %s stored=%s replayed=%s\n", row.TaskID, row.Field, row.Stored, row.Replayed)
		}
	}

	switch report.Requeues.Total {
	case 0:
		fmt.Println("requeues: none in the replayed fact range")
	default:
		fmt.Printf("requeues: %d not-the-task's-fault return(s) to Pending, by class:\n", report.Requeues.Total)

		classes := make([]string, 0, len(report.Requeues.ByClass))
		for class := range report.Requeues.ByClass {
			classes = append(classes, class)
		}

		sort.Strings(classes)

		for _, class := range classes {
			fmt.Printf("  %s: %d\n", class, report.Requeues.ByClass[class])
		}

		if report.Requeues.ResumeCloseout > 0 {
			fmt.Printf("  (%d parked an owed close-out — re-claim resumes at close-out, not a re-run)\n",
				report.Requeues.ResumeCloseout)
		}

		if n := report.Requeues.ByClass[queue.RequeueClassRateLimit]; n > 0 {
			fmt.Println(
				"  (rate-limit requeues never burn an attempt; 3 consecutive ENVIRONMENTAL ones escalate — see the env-streak breaker)",
			)
		}

		if n := report.Requeues.ByClass[queue.RequeueClassBudget]; n > 0 {
			fmt.Println(
				"  (budget requeues park paid turns until the daily cap or budget-cmd window resets — no attempt burn, outside the env-streak breaker; the pool LOOKS idle on purpose)",
			)
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
			"  (advisory: facts written before the redaction pass are the likely source; " +
				"inspect with `tq show <id>`, never re-print the secret)",
		)
	}

	fmt.Println("(advisory: investigate with `tq show <id>` and `tq facts -detail` before repairing)")

	return nil
}
