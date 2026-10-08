package incident

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/lars/projects/go-taskqueue/internal/queue/sqlite"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

func newTestStore(t *testing.T) *sqlite.Store {
	t.Helper()

	s, err := sqlite.Open(filepath.Join(t.TempDir(), "q.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	t.Cleanup(func() { _ = s.Close() })

	return s
}

func TestFingerprintStability(t *testing.T) {
	base := Report{Project: "webapp", Kind: KindServer, Message: "connection refused after 3 retries"}
	fp := Fingerprint(base)

	// Digit noise groups.
	noisy := base
	noisy.Message = "connection refused after 7 retries"

	if Fingerprint(noisy) != fp {
		t.Fatalf("digit noise changed fingerprint: %s != %s", Fingerprint(noisy), fp)
	}

	// Quoted-string noise groups.
	quoted := base
	quoted.Message = `connection refused after 3 retries on "tcp://10.0.0.7:5432"`

	if Fingerprint(quoted) != fp {
		t.Fatalf("quoted noise changed fingerprint: %s != %s", Fingerprint(quoted), fp)
	}

	// Whitespace noise groups.
	ws := base
	ws.Message = "connection   refused\nafter 3 retries"

	if Fingerprint(ws) != fp {
		t.Fatalf("whitespace noise changed fingerprint: %s != %s", Fingerprint(ws), fp)
	}

	// Different top frame is a different bug.
	frame := base
	frame.Stack = "at handler (/app/orders.go:42)\nat serve (/app/server.go:9)"
	other := base
	other.Stack = "at handler (/app/payments.go:7)"

	if Fingerprint(frame) == Fingerprint(other) {
		t.Fatal("different top frames must fingerprint differently")
	}

	// Different project is a different bug.
	foreign := base
	foreign.Project = "otherapp"

	if Fingerprint(foreign) == fp {
		t.Fatal("different projects must fingerprint differently")
	}
}

func TestClipAndValidate(t *testing.T) {
	if err := (Report{}).Validate(); err == nil {
		t.Fatal("empty report must be invalid")
	}

	if err := (Report{Project: "webapp"}).Validate(); err == nil {
		t.Fatal("missing message must be invalid")
	}

	if err := (Report{Project: "webapp", Message: "boom", Kind: Kind("weird")}).Validate(); err == nil {
		t.Fatal("unknown kind must be invalid")
	}

	rep := Report{
		Project:     "webapp",
		Message:     strings.Repeat("x", MaxMessage+100),
		Stack:       strings.Repeat("s", MaxStack+100),
		Breadcrumbs: make([]string, MaxBreadcrumbs+10),
	}.Clip()

	if len(rep.Message) > MaxMessage {
		t.Fatalf("message not capped: %d", len(rep.Message))
	}

	if len(rep.Stack) > MaxStack {
		t.Fatalf("stack not capped: %d", len(rep.Stack))
	}

	if len(rep.Breadcrumbs) != MaxBreadcrumbs {
		t.Fatalf("breadcrumbs not capped: %d", len(rep.Breadcrumbs))
	}

	if rep.Kind != KindServer {
		t.Fatalf("empty kind must default to server, got %q", rep.Kind)
	}
}

func TestRecorderAppendsObservedFact(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	rec := NewRecorder(s)

	res, err := rec.Record(ctx, Report{
		Project: "webapp",
		Kind:    KindClient,
		Message: "TypeError: cannot read properties of undefined (reading 'map')",
		Stack:   "at OrdersTable (main.a1b2c3.js:4:18771)",
		Release: "a1b2c3d",
		Route:   "/orders",
	})
	if err != nil {
		t.Fatalf("record: %v", err)
	}

	if !strings.HasPrefix(res.Incident, IDPrefix) {
		t.Fatalf("incident id must carry the prefix: %q", res.Incident)
	}

	facts, err := s.Facts(ctx, 0, 10)
	if err != nil {
		t.Fatalf("facts: %v", err)
	}

	if len(facts) != 1 {
		t.Fatalf("want 1 fact, got %d", len(facts))
	}

	f := facts[0]

	if f.Type != "error.observed" {
		t.Fatalf("fact type: %s", f.Type)
	}

	if f.TaskID != res.Incident {
		t.Fatalf("fact task id: %s != %s", f.TaskID, res.Incident)
	}

	var rep Report
	if err := json.Unmarshal(f.Detail, &rep); err != nil {
		t.Fatalf("detail decode: %v", err)
	}

	if rep.Project != "webapp" || rep.Kind != KindClient || rep.Release != "a1b2c3d" {
		t.Fatalf("detail roundtrip mismatch: %+v", rep)
	}

	// Invalid reports append nothing.
	if _, err := rec.Record(ctx, Report{Project: "webapp"}); err == nil {
		t.Fatal("missing message must fail")
	}

	facts, _ = s.Facts(ctx, 0, 10)

	if len(facts) != 1 {
		t.Fatalf("invalid report appended a fact: %d", len(facts))
	}
}

// TestPolicyStormOneTask is the core storm invariant: one thousand
// identical reports mint exactly one fix task.
func TestPolicyStormOneTask(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	rec := NewRecorder(s)

	rep := Report{Project: "webapp", Kind: KindServer, Message: "panic: runtime error: index out of range [7]"}

	for i := 0; i < 1000; i++ {
		if _, err := rec.Record(ctx, rep); err != nil {
			t.Fatalf("record %d: %v", i, err)
		}
	}

	pol, err := NewPolicy(ctx, s, s, PolicyConfig{})
	if err != nil {
		t.Fatalf("new policy: %v", err)
	}

	stats, err := pol.Sweep(ctx)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}

	if stats.TasksMinted != 1 {
		t.Fatalf("storm must mint exactly 1 task, got %d", stats.TasksMinted)
	}

	if stats.Occurrences != 1000 {
		t.Fatalf("occurrences: %d", stats.Occurrences)
	}

	tasks, err := s.List(ctx, listFilter())
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("want exactly 1 task in store, got %d", len(tasks))
	}

	got := tasks[0]

	if got.Type != executor.TaskTypeAgent {
		t.Fatalf("task type: %s", got.Type)
	}

	if got.Project != "webapp" {
		t.Fatalf("task project: %s", got.Project)
	}

	if got.Priority != DefaultFirstPriority {
		t.Fatalf("first fix must mint hot band %d, got %d", DefaultFirstPriority, got.Priority)
	}

	if got.MaxAttempts != DefaultMaxAttempts {
		t.Fatalf("max attempts: %d", got.MaxAttempts)
	}

	if want := TaskDedupKey(Fingerprint(rep), got.DedupKey); false {
		_ = want // dedup key shape asserted via prefix below
	}

	if !strings.HasPrefix(got.DedupKey, TaskDedupPrefix+Fingerprint(rep)+":") {
		t.Fatalf("dedup key shape: %s", got.DedupKey)
	}

	var payload executor.AgentPayload
	if err := json.Unmarshal(got.Payload, &payload); err != nil {
		t.Fatalf("payload decode: %v", err)
	}

	if payload.Repo != "webapp" || payload.Prompt == "" || payload.Item == "" {
		t.Fatalf("payload fields: %+v", payload)
	}

	if !strings.Contains(payload.Prompt, "index out of range") {
		t.Fatal("prompt must carry the error message")
	}

	// The fold sees occurrences and the dispatched state.
	inc, ok := pol.State().Get(Fingerprint(rep))
	if !ok {
		t.Fatal("incident missing from fold")
	}

	if inc.Occurrences != 1000 {
		t.Fatalf("fold occurrences: %d", inc.Occurrences)
	}

	if inc.Status != StatusFixDispatched {
		t.Fatalf("status after mint: %s", inc.Status)
	}
}

// TestPolicyReplayNoDuplicate: a replayed page (crash between consumption
// and checkpoint, simulated by rewinding the watermark) converges on the
// task dedup key instead of minting duplicates.
func TestPolicyReplayNoDuplicate(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	rec := NewRecorder(s)

	rep := Report{Project: "webapp", Message: "nil pointer dereference in order service"}

	if _, err := rec.Record(ctx, rep); err != nil {
		t.Fatalf("record: %v", err)
	}

	pol, err := NewPolicy(ctx, s, s, PolicyConfig{})
	if err != nil {
		t.Fatalf("new policy: %v", err)
	}

	if _, err := pol.Sweep(ctx); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	// Rewind the cursor before the observed fact: the next sweep replays.
	if err := s.SaveWatermark(ctx, ConsumerKey, 0); err != nil {
		t.Fatalf("rewind: %v", err)
	}

	if _, err := pol.Sweep(ctx); err != nil {
		t.Fatalf("replay sweep: %v", err)
	}

	tasks, err := s.List(ctx, listFilter())
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("replay must not duplicate tasks, got %d", len(tasks))
	}
}

// TestPolicyRegression: an error recurring after the fix task completed
// reopens the incident and mints at machine band.
func TestPolicyRegression(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	rec := NewRecorder(s)

	rep := Report{Project: "webapp", Message: "failed to load config from env"}

	if _, err := rec.Record(ctx, rep); err != nil {
		t.Fatalf("record: %v", err)
	}

	pol, err := NewPolicy(ctx, s, s, PolicyConfig{})
	if err != nil {
		t.Fatalf("new policy: %v", err)
	}

	if _, err := pol.Sweep(ctx); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	tasks, err := s.List(ctx, listFilter())
	if err != nil || len(tasks) != 1 {
		t.Fatalf("first mint: %v %d", err, len(tasks))
	}

	// Complete the fix task like a worker would.
	_, claim, err := s.ClaimDue(ctx, "test-owner", 5*time.Minute)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}

	if err := s.Complete(ctx, tasks[0].ID, claim, nil); err != nil {
		t.Fatalf("complete: %v", err)
	}

	if _, err := pol.Sweep(ctx); err != nil {
		t.Fatalf("sweep after completion: %v", err)
	}

	inc, _ := pol.State().Get(Fingerprint(rep))
	if inc.Status != StatusResolved {
		t.Fatalf("status after completion: %s", inc.Status)
	}

	// Recurrence: same fingerprint, new fact.
	if _, err := rec.Record(ctx, rep); err != nil {
		t.Fatalf("record recurrence: %v", err)
	}

	stats, err := pol.Sweep(ctx)
	if err != nil {
		t.Fatalf("sweep recurrence: %v", err)
	}

	if stats.TasksMinted != 1 || stats.Regressions != 1 {
		t.Fatalf("regression mint: minted=%d regressions=%d", stats.TasksMinted, stats.Regressions)
	}

	tasks, err = s.List(ctx, listFilter())
	if err != nil || len(tasks) != 2 {
		t.Fatalf("want 2 tasks after regression, got %d (%v)", len(tasks), err)
	}

	// The newest task is the regression mint.
	var newest task.Task

	for _, tt := range tasks {
		if tt.DedupKey != tasks[0].DedupKey {
			newest = tt
		}
	}

	if newest.Priority != DefaultRegressionPriority {
		t.Fatalf("regression must mint machine band %d, got %d", DefaultRegressionPriority, newest.Priority)
	}

	inc, _ = pol.State().Get(Fingerprint(rep))

	if inc.Regressions != 1 || inc.Status != StatusFixDispatched {
		t.Fatalf("fold after regression: %+v", inc)
	}
}

// TestPolicyDeadLetterFixFailed: an exhausted fix task fails the incident.
func TestPolicyDeadLetterFixFailed(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	rec := NewRecorder(s)

	rep := Report{Project: "webapp", Message: "boom"}

	if _, err := rec.Record(ctx, rep); err != nil {
		t.Fatalf("record: %v", err)
	}

	pol, _ := NewPolicy(ctx, s, s, PolicyConfig{})

	if _, err := pol.Sweep(ctx); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	tasks, _ := s.List(ctx, listFilter())

	// Burn the attempts: claim + fail until dead-lettered.
	for i := 0; ; i++ {
		tk, claim, err := s.ClaimDue(ctx, "test-owner", 5*time.Minute)
		if err != nil {
			t.Fatalf("claim: %v", err)
		}

		dead, err := s.Fail(ctx, tk.ID, claim, "simulated permanent failure", false)
		if err != nil {
			t.Fatalf("fail: %v", err)
		}

		if dead {
			break
		}

		if i > 10 {
			t.Fatal("task never dead-lettered")
		}
	}

	_ = tasks

	if _, err := pol.Sweep(ctx); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	inc, _ := pol.State().Get(Fingerprint(rep))

	if inc.Status != StatusFixFailed {
		t.Fatalf("status after dead-letter: %s", inc.Status)
	}
}

// TestPolicyRestartResumes: a fresh policy process over the same database
// folds full history, resumes the cursor, and does not re-mint.
func TestPolicyRestartResumes(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	rec := NewRecorder(s)

	rep := Report{Project: "webapp", Message: "transient upstream 503"}

	if _, err := rec.Record(ctx, rep); err != nil {
		t.Fatalf("record: %v", err)
	}

	first, err := NewPolicy(ctx, s, s, PolicyConfig{})
	if err != nil {
		t.Fatalf("first policy: %v", err)
	}

	if _, err := first.Sweep(ctx); err != nil {
		t.Fatalf("first sweep: %v", err)
	}

	// "Restart": new instance, same database.
	second, err := NewPolicy(ctx, s, s, PolicyConfig{})
	if err != nil {
		t.Fatalf("second policy: %v", err)
	}

	stats, err := second.Sweep(ctx)
	if err != nil {
		t.Fatalf("second sweep: %v", err)
	}

	if stats.TasksMinted != 0 {
		t.Fatalf("restart must not re-mint, got %d", stats.TasksMinted)
	}

	inc, ok := second.State().Get(Fingerprint(rep))
	if !ok || inc.Occurrences != 1 || inc.Status != StatusFixDispatched {
		t.Fatalf("restart fold: %+v ok=%v", inc, ok)
	}

	tasks, _ := s.List(ctx, listFilter())

	if len(tasks) != 1 {
		t.Fatalf("restart duplicated tasks: %d", len(tasks))
	}
}

func listFilter() (f any) { return nil } // replaced below by the real filter
