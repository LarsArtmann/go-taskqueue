package incident

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
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

func allTasks(t *testing.T, s *sqlite.Store) []task.Task {
	t.Helper()

	tasks, err := s.List(context.Background(), queue.Filter{})
	if err != nil {
		t.Fatalf("list tasks: %v", err)
	}

	return tasks
}

func TestFingerprintStability(t *testing.T) {
	base := Report{Project: "webapp", Kind: KindServer, Message: "connection refused after 3 retries"}
	fp := Fingerprint(base)

	noisy := base
	noisy.Message = "connection refused after 7 retries"

	if Fingerprint(noisy) != fp {
		t.Fatalf("digit noise changed fingerprint: %s != %s", Fingerprint(noisy), fp)
	}

	quoted := base
	quoted.Message = `cannot open 'config.json': no such file`
	quotedPeer := base
	quotedPeer.Message = `cannot open 'secrets.yaml': no such file`

	if Fingerprint(quoted) != Fingerprint(quotedPeer) {
		t.Fatalf("quoted noise changed fingerprint: %s != %s", Fingerprint(quoted), Fingerprint(quotedPeer))
	}

	ws := base
	ws.Message = "connection   refused\nafter 3 retries"

	if Fingerprint(ws) != fp {
		t.Fatalf("whitespace noise changed fingerprint: %s != %s", Fingerprint(ws), fp)
	}

	frame := base
	frame.Stack = "at handler (/app/orders.go:42)\nat serve (/app/server.go:9)"
	other := base
	other.Stack = "at handler (/app/payments.go:7)"

	if Fingerprint(frame) == Fingerprint(other) {
		t.Fatal("different top frames must fingerprint differently")
	}

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

	crumbs := make([]string, MaxBreadcrumbs+10)
	for i := range crumbs {
		crumbs[i] = strings.Repeat("c", MaxCrumb+50)
	}

	rep := Report{
		Project:     "webapp",
		Message:     strings.Repeat("x", MaxMessage+100),
		Stack:       strings.Repeat("s", MaxStack+100),
		Breadcrumbs: crumbs,
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

	for _, c := range rep.Breadcrumbs {
		if len(c) > MaxCrumb {
			t.Fatalf("crumb not capped: %d", len(c))
		}
	}

	if rep.Kind != KindServer {
		t.Fatalf("empty kind must default to server, got %q", rep.Kind)
	}

	// A cut landing mid-rune must back off to the rune boundary instead
	// of halving a multibyte character into U+FFFD.
	msg := strings.Repeat("é", 100) // 200 bytes, 2-byte runes
	clipped := Report{Project: "webapp", Message: msg}.Clip().Message
	if !utf8.ValidString(clipped) {
		t.Fatalf("clipped message split a rune: %q", clipped)
	}
	if want := (MaxMessage / 2) * 2; len(clipped) != want {
		t.Fatalf("rune-boundary cap = %d bytes, want %d", len(clipped), want)
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

	if _, err := rec.Record(ctx, Report{Project: "webapp"}); err == nil {
		t.Fatal("missing message must fail")
	}

	if facts, _ = s.Facts(ctx, 0, 10); len(facts) != 1 {
		t.Fatalf("invalid report appended a fact: %d", len(facts))
	}
}

// TestPolicyStormOneTask is the core storm invariant: one thousand
// identical reports mint exactly one fix task.
func TestPolicyStormOneTask(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	rec := NewRecorder(s)

	// The policy starts FIRST (production flow: pool running, then errors
	// arrive). A first run bootstraps at the journal head — facts that
	// predate the policy are folded but not reacted to; `tq watermarks set
	// incident-policy SEQ` rewinds to replay them.
	pol, err := NewPolicy(ctx, s, s, PolicyConfig{})
	if err != nil {
		t.Fatalf("new policy: %v", err)
	}

	rep := Report{Project: "webapp", Kind: KindServer, Message: "panic: runtime error: index out of range [7]"}

	for i := 0; i < 1000; i++ {
		if _, err := rec.Record(ctx, rep); err != nil {
			t.Fatalf("record %d: %v", i, err)
		}
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

	tasks := allTasks(t, s)

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

	rep := Report{Project: "webapp", Kind: KindServer, Message: "nil pointer dereference in order service"}

	pol, err := NewPolicy(ctx, s, s, PolicyConfig{})
	if err != nil {
		t.Fatalf("new policy: %v", err)
	}

	if _, err := rec.Record(ctx, rep); err != nil {
		t.Fatalf("record: %v", err)
	}

	if _, err := pol.Sweep(ctx); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	if err := s.SaveWatermark(ctx, ConsumerKey, 0); err != nil {
		t.Fatalf("rewind: %v", err)
	}

	if _, err := pol.Sweep(ctx); err != nil {
		t.Fatalf("replay sweep: %v", err)
	}

	if tasks := allTasks(t, s); len(tasks) != 1 {
		t.Fatalf("replay must not duplicate tasks, got %d", len(tasks))
	}
}

// TestPolicyRegression: an error recurring after the fix task completed
// reopens the incident and mints at machine band.
func TestPolicyRegression(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	rec := NewRecorder(s)

	rep := Report{Project: "webapp", Kind: KindServer, Message: "failed to load config from env"}

	pol, err := NewPolicy(ctx, s, s, PolicyConfig{})
	if err != nil {
		t.Fatalf("new policy: %v", err)
	}

	if _, err := rec.Record(ctx, rep); err != nil {
		t.Fatalf("record: %v", err)
	}

	if _, err := pol.Sweep(ctx); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	tasks := allTasks(t, s)

	if len(tasks) != 1 {
		t.Fatalf("first mint: %d tasks", len(tasks))
	}

	firstDedup := tasks[0].DedupKey

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

	tasks = allTasks(t, s)

	if len(tasks) != 2 {
		t.Fatalf("want 2 tasks after regression, got %d", len(tasks))
	}

	var regression task.Task

	for _, tt := range tasks {
		if tt.DedupKey != firstDedup {
			regression = tt
		}
	}

	if regression.Priority != DefaultRegressionPriority {
		t.Fatalf("regression must mint machine band %d, got %d",
			DefaultRegressionPriority, regression.Priority)
	}

	// The mint fact folds back on the next tick (short-page delivery);
	// settle the fold before asserting on it.
	if _, err := pol.Sweep(ctx); err != nil {
		t.Fatalf("settling sweep: %v", err)
	}

	inc, _ = pol.State().Get(Fingerprint(rep))

	if inc.Regressions != 1 || inc.Status != StatusFixDispatched {
		t.Fatalf("fold after regression: %+v", inc)
	}

	if len(inc.Mints) != 2 {
		t.Fatalf("fold must record both mints: %+v", inc.Mints)
	}

	if !inc.Mints[1].Regression || inc.Mints[1].Priority != DefaultRegressionPriority {
		t.Fatalf("second mint must be the regression: %+v", inc.Mints[1])
	}
}

// TestPolicyDeadLetterFixFailed: a permanently failed fix task fails the
// incident in the fold.
func TestPolicyDeadLetterFixFailed(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	rec := NewRecorder(s)

	rep := Report{Project: "webapp", Kind: KindServer, Message: "boom"}

	pol, err := NewPolicy(ctx, s, s, PolicyConfig{})
	if err != nil {
		t.Fatalf("new policy: %v", err)
	}

	if _, err := rec.Record(ctx, rep); err != nil {
		t.Fatalf("record: %v", err)
	}

	if _, err := pol.Sweep(ctx); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	tasks := allTasks(t, s)

	if len(tasks) != 1 {
		t.Fatalf("mint: %d tasks", len(tasks))
	}

	tk, claim, err := s.ClaimDue(ctx, "test-owner", 5*time.Minute)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}

	if err := s.FailPermanent(ctx, tk.ID, claim, "simulated permanent failure", nil); err != nil {
		t.Fatalf("fail permanent: %v", err)
	}

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

	rep := Report{Project: "webapp", Kind: KindServer, Message: "transient upstream 503"}

	first, err := NewPolicy(ctx, s, s, PolicyConfig{})
	if err != nil {
		t.Fatalf("first policy: %v", err)
	}

	if _, err := rec.Record(ctx, rep); err != nil {
		t.Fatalf("record: %v", err)
	}

	if _, err := first.Sweep(ctx); err != nil {
		t.Fatalf("first sweep: %v", err)
	}

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

	if tasks := allTasks(t, s); len(tasks) != 1 {
		t.Fatalf("restart duplicated tasks: %d", len(tasks))
	}
}

// TestPolicyFirstRunReplaysHistory: errors recorded while no policy ever
// ran (the API records standalone) mint on the pool's first start — the
// incident family is new, so the whole journal is in scope.
func TestPolicyFirstRunReplaysHistory(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	rec := NewRecorder(s)

	rep := Report{Project: "webapp", Kind: KindServer, Message: "panic in nightly window"}

	if _, err := rec.Record(ctx, rep); err != nil {
		t.Fatalf("record: %v", err)
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
		t.Fatalf("first run must replay history, minted=%d", stats.TasksMinted)
	}

	if tasks := allTasks(t, s); len(tasks) != 1 {
		t.Fatalf("tasks: %d", len(tasks))
	}
}
