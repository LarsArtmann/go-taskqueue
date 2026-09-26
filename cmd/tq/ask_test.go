package main

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// askFixture enqueues one agent task, claims it (the asker must be
// mid-run), and returns the task with a fresh store handle.
func askFixture(t *testing.T, dbPath string) (task.Task, *sqlite.Store) {
	t.Helper()

	ctx := context.Background()

	store, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	t.Cleanup(func() { _ = store.Close() })

	if _, err := store.Enqueue(
		ctx,
		task.New{Type: "agent", Project: "demo", Payload: jsontext.Value(`{"repo":"demo","prompt":"p"}`)},
	); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	claimed, _, err := store.ClaimDue(ctx, "w1", time.Minute)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}

	return claimed, store
}

func TestCmdAskRecordsFactAndMarker(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "q.db")

	claimed, store := askFixture(t, dbPath)

	marker := filepath.Join(dir, "marker.json")
	t.Setenv("TQ_QUESTION_FILE", marker)

	if err := cmdAsk([]string{
		"--task", claimed.ID.String(),
		"--type", "confirmation",
		"--options", "ship v3,stay on v2",
		"--expires", "1h",
		"--db", dbPath,
		"Ship as v3 now?",
	}); err != nil {
		t.Fatalf("ask: %v", err)
	}

	trail, err := store.FactsForTask(context.Background(), claimed.ID.String(), 0)
	if err != nil {
		t.Fatal(err)
	}

	var asked queue.QuestionAskedDetail

	found := false

	for _, f := range trail {
		if f.Type != journal.QuestionAsked {
			continue
		}

		found = true

		if err := json.Unmarshal(f.Detail, &asked); err != nil {
			t.Fatalf("asked detail: %v (%s)", err, f.Detail)
		}
	}

	if !found {
		t.Fatal("no task.question-asked fact recorded")
	}

	if asked.Ref == "" || asked.Question != "Ship as v3 now?" || asked.Type != "confirmation" {
		t.Errorf("asked detail = %+v", asked)
	}

	if len(asked.Options) != 2 || asked.Options[0] != "ship v3" {
		t.Errorf("options = %v", asked.Options)
	}

	if asked.ExpiresAt <= time.Now().Add(30*time.Minute).UnixMilli() {
		t.Errorf("expires_at = %d, want ~1h out", asked.ExpiresAt)
	}

	if asked.Repo != "demo" {
		t.Errorf("repo = %q, want the task's project", asked.Repo)
	}

	// The marker is the same detail: the executor parses it into the park.
	raw, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("marker: %v", err)
	}

	var parsed queue.QuestionAskedDetail
	if err := json.Unmarshal(jsontext.Value(raw), &parsed); err != nil {
		t.Fatalf("marker not question JSON: %v (%s)", err, raw)
	}

	if parsed.Ref != asked.Ref {
		t.Errorf("marker ref = %s, want %s", parsed.Ref, asked.Ref)
	}
}

func TestCmdAskRefusesNonRunningTask(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "q.db")

	ctx := context.Background()

	store, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}

	defer func() { _ = store.Close() }()

	enq, err := store.Enqueue(ctx, task.New{Type: "agent", Payload: jsontext.Value(`{}`)})
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("TQ_QUESTION_FILE", filepath.Join(dir, "marker.json"))

	err = cmdAsk([]string{"--task", enq.ID.String(), "--db", dbPath, "hello?"})
	if err == nil || !strings.Contains(err.Error(), "not running") {
		t.Fatalf("err = %v, want the running-only refusal", err)
	}
}

func TestCmdAskReAskPendingDoesNotDuplicateFact(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "q.db")

	claimed, store := askFixture(t, dbPath)

	marker := filepath.Join(dir, "marker.json")
	t.Setenv("TQ_QUESTION_FILE", marker)

	argv := []string{"--task", claimed.ID.String(), "--db", dbPath, "  Ship   as v3 NOW?  "}

	if err := cmdAsk(argv); err != nil {
		t.Fatalf("ask 1: %v", err)
	}

	// A re-ask after a crash converges on the same ref (normalization is
	// case/whitespace-insensitive) and must NOT append a second fact.
	if err := cmdAsk(append(argv[:len(argv)-1], "ship AS v3 now?")); err != nil {
		t.Fatalf("ask 2: %v", err)
	}

	trail, _ := store.FactsForTask(context.Background(), claimed.ID.String(), 0)

	n := 0

	for _, f := range trail {
		if f.Type == journal.QuestionAsked {
			n++
		}
	}

	if n != 1 {
		t.Fatalf("question-asked facts = %d, want 1 (re-ask is a no-op)", n)
	}

	// The park re-arms: the marker reflects the fresh run's question.
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("marker missing after re-ask: %v", err)
	}
}

func TestCmdAskAnsweredRefIsNoop(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "q.db")

	claimed, store := askFixture(t, dbPath)

	marker := filepath.Join(dir, "marker.json")
	t.Setenv("TQ_QUESTION_FILE", marker)

	argv := []string{"--task", claimed.ID.String(), "--db", dbPath, "Ship as v3 now?"}

	if err := cmdAsk(argv); err != nil {
		t.Fatalf("ask: %v", err)
	}

	// The owner rules; the store injects and unblocks.
	if err := store.RecordAnswer(context.Background(), claimed.ID, queue.AnswerRecord{
		Ref:    questionRef(claimed.ID.String(), "Ship as v3 now?"),
		Answer: "yes",
	}); err != nil {
		t.Fatalf("record answer: %v", err)
	}

	// The resumed run re-asks the same question: honor the ruling instead.
	if err := cmdAsk(argv); err != nil {
		t.Fatalf("re-ask after answer: %v", err)
	}

	trail, _ := store.FactsForTask(context.Background(), claimed.ID.String(), 0)

	nAsked, nAnswered := 0, 0

	for _, f := range trail {
		switch f.Type {
		case journal.QuestionAsked:
			nAsked++
		case journal.QuestionAnswered:
			nAnswered++
		}
	}

	if nAsked != 1 || nAnswered != 1 {
		t.Fatalf("facts after re-ask: asked=%d answered=%d, want 1/1", nAsked, nAnswered)
	}
}

func TestCmdAskValidation(t *testing.T) {
	t.Setenv("TQ_QUESTION_FILE", filepath.Join(t.TempDir(), "m.json"))

	for _, tc := range []struct {
		name string
		argv []string
		want string
	}{
		{name: "missing task", argv: []string{"hello?"}, want: "--task is required"},
		{name: "no question", argv: []string{"--task", "t1"}, want: "exactly one question"},
		{name: "blank question", argv: []string{"--task", "t1", "   "}, want: "empty question"},
		{name: "bad type", argv: []string{"--task", "t1", "--type", "poll", "q"}, want: "unknown question type"},
		{name: "bad expiry", argv: []string{"--task", "t1", "--expires", "soon", "q"}, want: "not a positive duration"},
		{name: "over-cap expiry", argv: []string{"--task", "t1", "--expires", "400h", "q"}, want: "cap"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// --db leads so it parses as a flag: parsing stops at the first
			// positional (the question text).
			err := cmdAsk(append([]string{"--db", filepath.Join(t.TempDir(), "unused.db")}, tc.argv...))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}
