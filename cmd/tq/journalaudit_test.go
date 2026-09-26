package main

import (
	"context"
	"database/sql"
	"encoding/json/jsontext"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
	"github.com/larsartmann/go-taskqueue/internal/task"
	_ "modernc.org/sqlite"
)

// seedDrift corrupts the tasks table of a CLOSED store's database file and
// returns the store reopened for the audit (hermetic: facts stay truthful,
// the projection lies).
func seedDrift(t *testing.T, path string) *sqlite.Store {
	t.Helper()

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("raw open: %v", err)
	}

	if _, err := db.Exec(
		`UPDATE tasks SET status = 'completed', attempts = 99, priority = 42, dedup_key = 'seeded'`,
	); err != nil {
		t.Fatalf("seed drift: %v", err)
	}

	if err := db.Close(); err != nil {
		t.Fatalf("raw close: %v", err)
	}

	store, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}

	t.Cleanup(func() { _ = store.Close() })

	return store
}

// journalAuditStore builds a scratch sqlite store (the doctor-test pattern).
func journalAuditStore(t *testing.T) *sqlite.Store {
	t.Helper()

	store, err := sqlite.Open(t.TempDir() + "/q.db")
	if err != nil {
		t.Fatalf("sqlite.Open: %v", err)
	}

	t.Cleanup(func() { _ = store.Close() })

	return store
}

func TestJournalDriftNoDriftOverFullLifecycle(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := journalAuditStore(t)

	if _, err := store.Enqueue(ctx, task.New{Project: "j", Type: "sh", Payload: []byte(`"true"`)}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	if _, err := store.Enqueue(ctx, task.New{Project: "j", Type: "sh", Payload: []byte(`"false"`)}); err != nil {
		t.Fatalf("Enqueue #2: %v", err)
	}

	// First claim: complete whichever task came up (happy path).
	first, claim1, err := store.ClaimDue(ctx, "w1", time.Minute)
	if err != nil {
		t.Fatalf("ClaimDue #1: %v", err)
	}

	if err := store.Complete(ctx, first.ID, claim1, nil); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	// t2: dead-letter immediately (permanent failure: failed + dead-lettered).
	claimed, claim2, err := store.ClaimDue(ctx, "w1", time.Minute)
	if err != nil {
		t.Fatalf("ClaimDue t2: %v", err)
	}

	if err := store.FailPermanent(ctx, claimed.ID, claim2, "boom", nil); err != nil {
		t.Fatalf("FailPermanent: %v", err)
	}

	// The remaining pending task: cancel before it is ever claimed.
	if _, err := store.Enqueue(ctx, task.New{Project: "j", Type: "sh", Payload: []byte(`"true"`)}); err != nil {
		t.Fatalf("Enqueue #3: %v", err)
	}

	pending, err := store.List(ctx, queue.Filter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	for i := range pending {
		if pending[i].Status == task.Pending {
			if err := store.Cancel(ctx, pending[i].ID, "no longer needed"); err != nil {
				t.Fatalf("Cancel: %v", err)
			}

			break
		}
	}

	report, err := journalDrift(ctx, store)
	if err != nil {
		t.Fatalf("journalDrift: %v", err)
	}

	if report.HasDrift() {
		t.Fatalf("unexpected drift: %+v", report.Drift)
	}

	if report.TasksCompared != 3 {
		t.Errorf("TasksCompared = %d, want 3", report.TasksCompared)
	}

	if report.FactsReplayed == 0 {
		t.Errorf("FactsReplayed = 0, want > 0")
	}
}

func TestJournalDriftSeededDriftAllFields(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	path := t.TempDir() + "/drift.db"

	store, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("sqlite.Open: %v", err)
	}

	if _, err := store.Enqueue(ctx, task.New{
		Project: "j", Type: "sh", Payload: []byte(`"true"`), Priority: 3, DedupKey: "todo:real",
	}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	store = seedDrift(t, path)

	report, err := journalDrift(ctx, store)
	if err != nil {
		t.Fatalf("journalDrift: %v", err)
	}

	if !report.HasDrift() {
		t.Fatalf("expected seeded drift, got %+v", report)
	}

	fields := map[string]DriftRow{}
	for _, row := range report.Drift {
		fields[row.Field] = row
	}

	for _, field := range []string{"status", "attempts", "priority", "dedup_key"} {
		row, ok := fields[field]
		if !ok {
			t.Errorf("no drift row for field %s (report: %+v)", field, report)

			continue
		}

		if row.Replayed == row.Stored {
			t.Errorf("field %s: stored == replayed == %q, expected divergence", field, row.Stored)
		}
	}

	if fields["status"].Stored != "completed" || fields["status"].Replayed != "pending" {
		t.Errorf("status row = %+v, want stored=completed replayed=pending", fields["status"])
	}

	if fields["priority"].Replayed != "3" {
		t.Errorf("priority row = %+v, want replayed=3", fields["priority"])
	}

	if fields["dedup_key"].Replayed != "todo:real" {
		t.Errorf("dedup row = %+v, want replayed=todo:real", fields["dedup_key"])
	}

	// The seeded task was enqueued AFTER enrichment, so every field was
	// diffable despite all four drifting.
	if report.Coverage != (FieldCoverage{Status: 1, Attempts: 1, Priority: 1, DedupKey: 1}) {
		t.Errorf("coverage = %+v, want all fields at 1/1", report.Coverage)
	}
}

func TestReplayProjectionTransitions(t *testing.T) {
	t.Parallel()

	steps := []struct {
		id   string
		typ  journal.FactType
		want task.Status
	}{
		{"a", journal.Enqueued, task.Pending},
		{"a", journal.Claimed, task.Running},
		{"a", journal.Completed, task.Completed},
		{"b", journal.Enqueued, task.Pending},
		{"b", journal.Failed, task.Pending},
		{"b", journal.DeadLettered, task.Dead},
		{"c", journal.Enqueued, task.Pending},
		{"c", journal.Cancelled, task.Cancelled},
		{"d", journal.Enqueued, task.Pending},
		{"d", journal.Requeued, task.Pending},
		{"e", journal.Enqueued, task.Pending},
		{"e", journal.Released, task.Pending},
	}

	facts := make([]journal.Fact, 0, len(steps))

	for i, step := range steps {
		attempt := 0
		if step.typ == journal.Failed || step.typ == journal.DeadLettered {
			attempt = i // any post-increment number; max() takes the last
		}

		facts = append(facts, journal.Fact{TaskID: step.id, Type: step.typ, Attempt: attempt})
	}

	got := replayProjection(facts)

	want := map[string]task.Status{
		"a": task.Completed, "b": task.Dead, "c": task.Cancelled, "d": task.Pending, "e": task.Pending,
	}

	for id, expected := range want {
		if got[task.ID(id)] == nil {
			t.Errorf("task %s: missing from replay", id)

			continue
		}

		if got[task.ID(id)].status != expected {
			t.Errorf("task %s: replayed %v, want %v", id, got[task.ID(id)].status, expected)
		}
	}

	if got[task.ID("b")].attempts != 5 {
		t.Errorf("task b: attempts = %d, want 5 (max failed/dead-lettered attempt)",
			got[task.ID("b")].attempts)
	}
}

func TestReplayProjectionPriorityAndDedup(t *testing.T) {
	t.Parallel()

	p := 7
	facts := []journal.Fact{
		{TaskID: "p", Type: journal.Enqueued, Detail: jsontext.Value(`{"priority":3,"dedup_key":"todo:x"}`)},
		{
			TaskID: "p",
			Type:   journal.Reprioritized,
			Detail: jsontext.Value(`{"old_priority":3,"new_priority":7,"source":"manual"}`),
		},
		{TaskID: "legacy", Type: journal.Enqueued},
	}

	got := replayProjection(facts)

	state := got[task.ID("p")]
	if state == nil || state.priority == nil || *state.priority != p {
		t.Fatalf("task p: replayed priority = %v, want %d", state, p)
	}

	if state.dedupKey != "todo:x" || !state.dedupKnown {
		t.Errorf("task p: dedup = %q known=%v, want todo:x known=true", state.dedupKey, state.dedupKnown)
	}

	legacy := got[task.ID("legacy")]
	if legacy.priorityKnown || legacy.dedupKnown {
		t.Errorf("legacy thin fact must not claim knowledge: %+v", legacy)
	}
}

func TestReplayProjectionRescueResetsAttempts(t *testing.T) {
	t.Parallel()

	p := 5
	facts := []journal.Fact{
		{TaskID: "r", Type: journal.Enqueued, Detail: jsontext.Value(`{"priority":5,"dedup_key":"todo:r"}`)},
		{TaskID: "r", Type: journal.Claimed},
		{TaskID: "r", Type: journal.Failed, Attempt: 1},
		{TaskID: "r", Type: journal.DeadLettered, Attempt: 1},
		{TaskID: "r", Type: journal.Enqueued, Detail: jsontext.Value(`{"rescue":"true"}`)},
	}

	got := replayProjection(facts)

	state := got[task.ID("r")]
	if state == nil {
		t.Fatal("task r: missing from replay")
	}

	if state.status != task.Pending {
		t.Errorf("rescued status = %v, want pending", state.status)
	}

	if state.attempts != 0 {
		t.Errorf("rescued attempts = %d, want 0 (the store resets the budget)", state.attempts)
	}

	if state.priority == nil || *state.priority != p || state.dedupKey != "todo:r" {
		t.Errorf("rescue must not touch identity: %+v", state)
	}
}

func TestJournalDriftNoDriftAfterRescue(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := journalAuditStore(t)

	enqueued, err := store.Enqueue(ctx, task.New{Project: "j", Type: "sh", Payload: []byte(`"false"`), MaxAttempts: 1})
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	claimed, err := store.ClaimDue(ctx, "w1", time.Minute)
	if err != nil {
		t.Fatalf("ClaimDue: %v", err)
	}

	if claimed.ID != enqueued.ID {
		t.Fatalf("claimed %s, want %s", claimed.ID, enqueued.ID)
	}

	if err := store.Fail(ctx, enqueued.ID, "w1", "boom", 0, nil); err != nil {
		t.Fatalf("Fail: %v", err)
	}

	if err := store.RescueDead(ctx, enqueued.ID, 2); err != nil {
		t.Fatalf("RescueDead: %v", err)
	}

	report, err := journalDrift(ctx, store)
	if err != nil {
		t.Fatalf("journalDrift: %v", err)
	}

	if report.HasDrift() {
		t.Fatalf("unexpected drift after rescue: %+v", report.Drift)
	}

	// Enrichment wrote priority (as an explicit 0) but the empty dedup key
	// is indistinguishable from a legacy fact — coverage reports that
	// honestly instead of pretending to have diffed it.
	if report.TasksCompared != 1 {
		t.Errorf("TasksCompared = %d, want 1", report.TasksCompared)
	}

	if report.Coverage != (FieldCoverage{Status: 1, Attempts: 1, Priority: 1, DedupKey: 0}) {
		t.Errorf("coverage = %+v, want status/attempts/priority 1, dedup 0", report.Coverage)
	}
}

func TestDiffProjectionCoverageSkipsLegacyThinFacts(t *testing.T) {
	t.Parallel()

	stored := []task.Task{
		{ID: "enriched", Status: task.Pending, Attempts: 1, Priority: 3, DedupKey: "todo:e"},
		// attempts 0: claims never burn attempts (only failed facts do), so
		// a thin-fact task with no failed facts must hold zero.
		{ID: "legacy", Status: task.Pending, Attempts: 0, Priority: 9, DedupKey: "todo:l"},
		{ID: "ghost", Status: task.Running},
	}

	facts := []journal.Fact{
		{TaskID: "enriched", Type: journal.Enqueued, Detail: jsontext.Value(`{"priority":3,"dedup_key":"todo:e"}`)},
		{TaskID: "enriched", Type: journal.Claimed},
		{TaskID: "enriched", Type: journal.Failed, Attempt: 1},
		// Thin fact: recorded before enrichment — priority/dedup unverifiable.
		{TaskID: "legacy", Type: journal.Enqueued},
	}

	report := diffProjection(stored, replayProjection(facts))

	// The ghost row is drift by definition (no facts at all); legacy must
	// NOT drift — its stored priority 9 / dedup todo:l were never diffed,
	// an absence of evidence is not drift.
	if len(report.Drift) != 1 || report.Drift[0].TaskID != "ghost" ||
		report.Drift[0].Field != "status" || report.Drift[0].Replayed != "(no facts)" {
		t.Fatalf("drift = %+v, want exactly the ghost status row", report.Drift)
	}

	if report.TasksCompared != 3 {
		t.Errorf("TasksCompared = %d, want 3", report.TasksCompared)
	}

	// The ghost row consumed no coverage at all; legacy contributed only
	// status/attempts (always recorded), never priority/dedup.
	if report.Coverage != (FieldCoverage{Status: 2, Attempts: 2, Priority: 1, DedupKey: 1}) {
		t.Errorf("coverage = %+v, want status/attempts 2, priority/dedup 1", report.Coverage)
	}
}

// fakeAuditToken is shape-valid but fake: an OpenAI-style key body long
// enough to trip the detector.
const fakeAuditToken = "sk-abcdefghijklmnopqrstuvwxyz012345"

func TestScanFactSecretsFindsTokenShapedEvidence(t *testing.T) {
	t.Parallel()

	facts := []journal.Fact{
		// Evidence carriers: failed detail + dead-letter error text.
		{
			Seq:    3,
			TaskID: "leak-detail",
			Type:   journal.Failed,
			Detail: jsontext.Value(`{"stage":"agent","tail":"boom ` + fakeAuditToken + `"}`),
		},
		{Seq: 4, TaskID: "leak-error", Type: journal.DeadLettered, Error: "agent run failed: " + fakeAuditToken},
		// Two hits in one field count as two.
		{Seq: 5, TaskID: "leak-twice", Type: journal.Failed, Error: fakeAuditToken + " / " + fakeAuditToken},
		// NOT scanned: enqueue payloads are provided, not leaked.
		{
			Seq:    6,
			TaskID: "payload-clean",
			Type:   journal.Enqueued,
			Detail: jsontext.Value(`{"payload":"` + fakeAuditToken + `"}`),
		},
		// Clean facts produce no rows.
		{
			Seq:    7,
			TaskID: "clean",
			Type:   journal.Failed,
			Error:  "exit status 1",
			Detail: jsontext.Value(`{"tail":"build failed"}`),
		},
	}

	hits := scanFactSecrets(facts)

	if len(hits) != 3 {
		t.Fatalf("hits = %+v, want 3 rows", hits)
	}

	if hits[0].Seq != 3 || hits[0].Field != "detail" || hits[0].Count != 1 {
		t.Errorf("row 0 = %+v, want seq 3 detail x1", hits[0])
	}

	if hits[1].Seq != 4 || hits[1].Field != "error" || hits[1].TaskID != "leak-error" {
		t.Errorf("row 1 = %+v, want seq 4 error on leak-error", hits[1])
	}

	if hits[2].Count != 2 {
		t.Errorf("row 2 count = %d, want 2", hits[2].Count)
	}

	for _, hit := range hits {
		if strings.Contains(hit.Type, fakeAuditToken) || strings.Contains(hit.TaskID, "sk-") {
			t.Errorf("hit row must never carry the secret itself: %+v", hit)
		}
	}
}

func TestScanFactSecretsCleanJournalIsEmpty(t *testing.T) {
	t.Parallel()

	facts := []journal.Fact{
		{Seq: 1, TaskID: "t", Type: journal.Enqueued},
		{Seq: 2, TaskID: "t", Type: journal.Failed, Error: "command failed: exit status 2: make: *** [all] Error 2"},
	}

	if hits := scanFactSecrets(facts); len(hits) != 0 {
		t.Errorf("hits = %+v, want none", hits)
	}
}
