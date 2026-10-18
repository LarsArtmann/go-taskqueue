package dlqfix

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/harvest"
	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

func newTestStore(t *testing.T) *sqlite.Store {
	t.Helper()

	s, err := sqlite.Open(filepath.Join(t.TempDir(), "q.db"))
	if err != nil {
		t.Fatalf("sqlite.Open: %v", err)
	}

	t.Cleanup(func() { _ = s.Close() })

	return s
}

const (
	testOwner = "test-worker"
	testLease = 5 * time.Minute
)

// seedDeadAgentTask enqueues an agent task with a one-attempt budget and
// fails its single attempt (carrying failure evidence) so it dead-letters —
// the real path a task takes into the DLQ.
func seedDeadAgentTask(
	t *testing.T,
	s *sqlite.Store,
	payload executor.AgentPayload,
) task.Task {
	t.Helper()

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal agent payload: %v", err)
	}

	ctx := context.Background()

	enq, err := s.Enqueue(ctx, task.New{
		Type:        executor.TaskTypeAgent,
		Project:     "demo",
		Priority:    7,
		Payload:     raw,
		MaxAttempts: 1,
		DedupKey:    "seed:" + payload.Prompt,
	})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	_, claim, err := s.ClaimDue(ctx, testOwner, testLease)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}

	evidence, err := json.Marshal(executor.FailureEvidence{
		Stage:    "verify",
		ExitCode: 2,
		Tail:     "FAIL: TestShipTheThing",
	})
	if err != nil {
		t.Fatalf("marshal evidence: %v", err)
	}

	if err := s.Fail(ctx, enq.ID, claim, "verify failed", 0, evidence); err != nil {
		t.Fatalf("fail: %v", err)
	}

	got, err := s.Get(ctx, enq.ID)
	if err != nil || got.Status != task.Dead {
		t.Fatalf("seeded task not dead: %+v (%v)", got, err)
	}

	return got
}

// finishTask claims and completes one pending task with result detail.
func finishTask(t *testing.T, s *sqlite.Store, id task.ID, detail executor.DLQFixResult) {
	t.Helper()

	ctx := context.Background()

	_, claim, err := s.ClaimDue(ctx, testOwner, testLease)
	if err != nil {
		t.Fatalf("claim %s: %v", id, err)
	}

	raw, err := json.Marshal(detail)
	if err != nil {
		t.Fatalf("marshal verdict: %v", err)
	}

	if err := s.Complete(ctx, id, claim, raw); err != nil {
		t.Fatalf("complete %s: %v", id, err)
	}
}

func newTestSweeper(t *testing.T, s queue.Store) *Sweeper {
	t.Helper()

	sw, err := NewSweeper(context.Background(), s, SweeperConfig{Model: "", Log: nil})
	if err != nil {
		t.Fatalf("NewSweeper: %v", err)
	}

	return sw
}

// flakyEnqueueStore fails the next N enqueues with a store-shaped error —
// the transient failure class a mint refusal represents. Everything else
// rides the real store.
type flakyEnqueueStore struct {
	queue.Store

	failures atomic.Int32
}

func (f *flakyEnqueueStore) Enqueue(ctx context.Context, n task.New) (task.Task, error) {
	if f.failures.Load() > 0 {
		f.failures.Add(-1)

		return task.Task{}, errors.New("sqlite: database is locked (simulated)")
	}

	return f.Store.Enqueue(ctx, n)
}

// captureHandler records slog records so tests can pin the warn that
// accompanies a mint refusal.
type captureHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *captureHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.records = append(h.records, r)

	return nil
}

func (h *captureHandler) WithAttrs([]slog.Attr) slog.Handler { return h }

func (h *captureHandler) WithGroup(string) slog.Handler { return h }

// deadLetterSeq returns the journal seq of the task's dead-lettered fact.
func deadLetterSeq(t *testing.T, s *sqlite.Store, id task.ID) int64 {
	t.Helper()

	return factSeqFor(t, s, journal.DeadLettered, id)
}

// factSeqFor returns the journal seq of the task's fact of the given type.
func factSeqFor(t *testing.T, s *sqlite.Store, typ journal.FactType, id task.ID) int64 {
	t.Helper()

	facts, err := s.Facts(context.Background(), 0, 10_000)
	if err != nil {
		t.Fatalf("facts: %v", err)
	}

	for _, f := range facts {
		if f.Type == typ && f.TaskID == id.String() {
			return f.Seq
		}
	}

	t.Fatalf("no %s fact for %s", typ, id)

	return 0
}

// pendingDLQFixTasks lists the dlqfix tasks in the store.
func pendingDLQFixTasks(t *testing.T, s *sqlite.Store) []task.Task {
	t.Helper()

	dlqfix := executor.TaskTypeDLQFix

	got, err := s.List(context.Background(), queue.Filter{Type: &dlqfix})
	if err != nil {
		t.Fatalf("list dlqfix tasks: %v", err)
	}

	return got
}

func TestSweeperMintsOneAutopsyPerDeadAgentTask(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	sw := newTestSweeper(t, s)

	dead := seedDeadAgentTask(t, s, executor.AgentPayload{
		Repo:   "demo",
		Prompt: "ship the frobnicator\n\nTask-Queue-ID: {{TASK_ID}}",
		Yolo:   true,
	})

	stats, err := sw.Sweep(context.Background())
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}

	if stats.FixesEnqueued != 1 {
		t.Fatalf("FixesEnqueued = %d, want 1 (stats %+v)", stats.FixesEnqueued, stats)
	}

	tasks := pendingDLQFixTasks(t, s)
	if len(tasks) != 1 {
		t.Fatalf("dlqfix tasks = %d, want 1", len(tasks))
	}

	autopsy := tasks[0]
	if autopsy.Project != dead.Project || autopsy.Priority != dead.Priority {
		t.Fatalf("autopsy did not inherit project/priority: %+v", autopsy)
	}

	var payload executor.DLQFixPayload
	if err := json.Unmarshal(autopsy.Payload, &payload); err != nil {
		t.Fatalf("autopsy payload: %v (%s)", err, autopsy.Payload)
	}

	if payload.Repo != "demo" || payload.DeadTask != dead.ID.String() || payload.DeadType != executor.TaskTypeAgent {
		t.Fatalf("payload identity wrong: %+v", payload)
	}

	if payload.Work != "ship the frobnicator\n\nTask-Queue-ID: {{TASK_ID}}" {
		t.Fatalf("payload work not verbatim: %q", payload.Work)
	}

	if !payload.Yolo {
		t.Fatal("payload must mirror the dead task's yolo")
	}

	wantEvidence := executor.FailureEvidence{Stage: "verify", ExitCode: 2, Tail: "FAIL: TestShipTheThing"}
	if payload.Failure != wantEvidence {
		t.Fatalf("payload evidence wrong: %+v", payload.Failure)
	}

	if DedupKey(dead.ID) != "dlqfix:"+dead.ID.String() {
		t.Fatalf("DedupKey = %q", DedupKey(dead.ID))
	}
}

// TestSweeperAutopsyCarriesReclaimDisposition pins that an autopsy minted for
// a task with a prior crash reclaim (task.released, effect unknown) carries
// the count, so the autopsy prompt warns that the repo state may be partial.
func TestSweeperAutopsyCarriesReclaimDisposition(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	sw := newTestSweeper(t, s)

	dead := seedDeadAgentTask(t, s, executor.AgentPayload{
		Repo:   "demo",
		Prompt: "ship the frobnicator",
		Yolo:   true,
	})

	if err := s.AppendFact(context.Background(), journal.Fact{
		TaskID: dead.ID.String(),
		Type:   journal.Released,
		Detail: jsontext.Value(`{"effect":"unknown","reason":"lease-expiry"}`),
	}); err != nil {
		t.Fatalf("append released fact: %v", err)
	}

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

	if payload.ReclaimsWithUnknownEffect != 1 {
		t.Fatalf("ReclaimsWithUnknownEffect = %d, want 1", payload.ReclaimsWithUnknownEffect)
	}
}

// TestSweeperReplayDoesNotDuplicateAutopsy pins the crash-recovery story: a
// REPLAYED page (crash between consumption and checkpoint — simulated with
// the SetWatermark ops hatch, the rewind `tq watermarks set` exposes) must
// not mint a second autopsy. The autopsy is CLAIMED first so the
// fresh-vs-known stats heuristic is deterministic: a claimed stored task can
// only count as known.
func TestSweeperReplayDoesNotDuplicateAutopsy(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	sw := newTestSweeper(t, s)

	headBeforeDeath, err := s.HeadSeq(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	seedDeadAgentTask(t, s, executor.AgentPayload{Repo: "demo", Prompt: "p"})

	if _, err := sw.Sweep(context.Background()); err != nil {
		t.Fatalf("mint sweep: %v", err)
	}

	if _, _, err := s.ClaimDue(context.Background(), testOwner, testLease); err != nil {
		t.Fatalf("claim autopsy: %v", err)
	}

	if err := s.SetWatermark(context.Background(), ConsumerKey, headBeforeDeath); err != nil {
		t.Fatal(err)
	}

	replay := newTestSweeper(t, s)

	stats, err := replay.Sweep(context.Background())
	if err != nil {
		t.Fatalf("replay sweep: %v", err)
	}

	if stats.FixesEnqueued != 0 || stats.FixesKnown != 1 {
		t.Fatalf("replay sweep stats = %+v, want known-only", stats)
	}

	if tasks := pendingDLQFixTasks(t, s); len(tasks) != 1 {
		t.Fatalf("dlqfix tasks after replay = %d, want 1 (no duplicate row)", len(tasks))
	}
}

// TestSweeperNeverAutopsiesNonAgentDeaths pins the loop guard: only agent
// tasks are autopsied — a dead autopsy can never mint another autopsy, and
// sh/review/status deaths stay human surfaces.
func TestSweeperNeverAutopsiesNonAgentDeaths(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	ctx := context.Background()

	sw := newTestSweeper(t, s)

	for _, taskType := range []string{"sh", executor.TaskTypeReview, executor.TaskTypeStatus, executor.TaskTypeDLQFix} {
		enq, err := s.Enqueue(ctx, task.New{
			Type: taskType, Project: "demo", MaxAttempts: 1,
			Payload: []byte(`{}`), DedupKey: "guard:" + taskType,
		})
		if err != nil {
			t.Fatalf("enqueue %s: %v", taskType, err)
		}

		_, claim, err := s.ClaimDue(ctx, testOwner, testLease)
		if err != nil {
			t.Fatalf("claim %s: %v", taskType, err)
		}

		if err := s.Fail(ctx, enq.ID, claim, "boom", 0, nil); err != nil {
			t.Fatalf("fail %s: %v", taskType, err)
		}
	}

	stats, err := sw.Sweep(ctx)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}

	if stats.FixesEnqueued != 0 || stats.FixesKnown != 0 {
		t.Fatalf("stats = %+v, want no mints", stats)
	}

	if stats.Skipped != 4 {
		t.Fatalf("Skipped = %d, want 4 (stats %+v)", stats.Skipped, stats)
	}
}

func TestSweeperRescuesOnFixedVerdict(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	sw := newTestSweeper(t, s)

	dead := seedDeadAgentTask(t, s, executor.AgentPayload{Repo: "demo", Prompt: "p"})

	if _, err := sw.Sweep(context.Background()); err != nil {
		t.Fatalf("mint sweep: %v", err)
	}

	autopsy := pendingDLQFixTasks(t, s)[0]
	finishTask(t, s, autopsy.ID, executor.DLQFixResult{
		Verdict:   executor.VerdictFixed,
		Summary:   "root cause was a bad flag; fixed and proven",
		CommitSHA: "deadbee",
	})

	stats, err := sw.Sweep(context.Background())
	if err != nil {
		t.Fatalf("dispose sweep: %v", err)
	}

	if stats.Rescued != 1 {
		t.Fatalf("Rescued = %d, want 1 (stats %+v)", stats.Rescued, stats)
	}

	got, err := s.Get(context.Background(), dead.ID)
	if err != nil {
		t.Fatal(err)
	}

	if got.Status != task.Pending || got.Attempts != 0 || got.MaxAttempts != dead.MaxAttempts {
		t.Fatalf("rescued task wrong: %+v (want pending, fresh attempts, original budget)", got)
	}
}

func TestSweeperDismissesOnWontfixVerdict(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	sw := newTestSweeper(t, s)

	dead := seedDeadAgentTask(t, s, executor.AgentPayload{Repo: "demo", Prompt: "p"})

	if _, err := sw.Sweep(context.Background()); err != nil {
		t.Fatalf("mint sweep: %v", err)
	}

	autopsy := pendingDLQFixTasks(t, s)[0]
	finishTask(t, s, autopsy.ID, executor.DLQFixResult{
		Verdict: executor.VerdictWontFix,
		Summary: "needs credentials only the operator holds",
	})

	stats, err := sw.Sweep(context.Background())
	if err != nil {
		t.Fatalf("dispose sweep: %v", err)
	}

	if stats.Dismissed != 1 {
		t.Fatalf("Dismissed = %d, want 1 (stats %+v)", stats.Dismissed, stats)
	}

	got, err := s.Get(context.Background(), dead.ID)
	if err != nil {
		t.Fatal(err)
	}

	if got.Status != task.Cancelled {
		t.Fatalf("dismissed task status = %s, want cancelled", got.Status)
	}

	trail, err := s.FactsForTask(context.Background(), dead.ID.String(), 0)
	if err != nil {
		t.Fatal(err)
	}

	var detail map[string]string

	sawFact := false

	for _, f := range trail {
		if f.Type != journal.Cancelled {
			continue
		}

		sawFact = true

		if json.Unmarshal(f.Detail, &detail) != nil {
			t.Fatalf("dismiss fact detail not JSON: %s", f.Detail)
		}
	}

	wantReason := "needs credentials only the operator holds"
	if !sawFact || detail["reason"] != wantReason || detail["dismissed_by"] != DismissedBySweeper {
		t.Fatalf("dismiss fact = %v (fact seen: %v)", detail, sawFact)
	}
}

// TestSweeperDispositionIsBenignWhenMovedElsewhere pins the idempotency
// story: if a human rescued (or dismissed) the dead task between the
// autopsy's completion and the sweep, the disposition degrades to a skipped
// counter instead of crashing or double-acting.
func TestSweeperDispositionIsBenignWhenMovedElsewhere(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	sw := newTestSweeper(t, s)

	dead := seedDeadAgentTask(t, s, executor.AgentPayload{Repo: "demo", Prompt: "p"})

	if _, err := sw.Sweep(context.Background()); err != nil {
		t.Fatalf("mint sweep: %v", err)
	}

	autopsy := pendingDLQFixTasks(t, s)[0]
	finishTask(t, s, autopsy.ID, executor.DLQFixResult{
		Verdict: executor.VerdictFixed,
		Summary: "s",
	})

	// The operator got there first.
	if err := s.RescueDead(context.Background(), dead.ID, 3); err != nil {
		t.Fatalf("manual rescue: %v", err)
	}

	stats, err := sw.Sweep(context.Background())
	if err != nil {
		t.Fatalf("dispose sweep: %v", err)
	}

	if stats.Rescued != 0 || stats.Skipped != 1 {
		t.Fatalf("stats = %+v, want skipped-not-crash", stats)
	}
}

// TestSweeperIgnoresForeignCompletions pins that completed non-dlqfix tasks
// pass through the cursor without dispositions (they are not even skips —
// they are simply not ours).
func TestSweeperIgnoresForeignCompletions(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	ctx := context.Background()

	sw := newTestSweeper(t, s)

	// D1 dies and STAYS dead. D2 is a plain agent task that completes
	// normally (never dead) — its completed fact must pass the sweeper
	// without a disposition.
	stillDead := seedDeadAgentTask(t, s, executor.AgentPayload{Repo: "demo", Prompt: "p-one"})

	foreignRaw, err := json.Marshal(executor.AgentPayload{Repo: "demo", Prompt: "p-two"})
	if err != nil {
		t.Fatal(err)
	}

	foreign, err := s.Enqueue(ctx, task.New{
		Type: executor.TaskTypeAgent, Project: "demo", Payload: foreignRaw, DedupKey: "foreign",
	})
	if err != nil {
		t.Fatal(err)
	}

	_, claim, err := s.ClaimDue(ctx, testOwner, testLease)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}

	if err := s.Complete(ctx, foreign.ID, claim, nil); err != nil {
		t.Fatalf("complete: %v", err)
	}

	stats, err := sw.Sweep(ctx)
	if err != nil {
		t.Fatalf("mint sweep: %v", err)
	}

	if stats.FixesEnqueued != 1 {
		t.Fatalf("FixesEnqueued = %d, want 1 (only the dead task)", stats.FixesEnqueued)
	}

	if stats.Rescued != 0 || stats.Dismissed != 0 {
		t.Fatalf("stats = %+v, want no dispositions from foreign completions", stats)
	}

	tasks := pendingDLQFixTasks(t, s)
	if len(tasks) != 1 {
		t.Fatalf("dlqfix tasks = %d, want 1", len(tasks))
	}

	var payload executor.DLQFixPayload
	if err := json.Unmarshal(tasks[0].Payload, &payload); err != nil {
		t.Fatal(err)
	}

	if payload.DeadTask != stillDead.ID.String() {
		t.Fatalf("autopsy minted for %s, want the still-dead %s", payload.DeadTask, stillDead.ID)
	}
}

// seedDeadGateArtifactTask enqueues an agent task for repo (a real temp dir
// with a TODO_LIST.md), gives it the caller's dedup key, and fails its only
// attempt with the caller's evidence + last error — the shape the
// gate-artifact auto-dismissal judges.
func seedDeadGateArtifactTask(
	t *testing.T,
	s *sqlite.Store,
	repo, dedupKey string,
	evidence executor.FailureEvidence,
	lastError string,
) task.Task {
	t.Helper()

	raw, err := json.Marshal(executor.AgentPayload{Repo: repo, Prompt: "ship it", Yolo: true})
	if err != nil {
		t.Fatalf("marshal agent payload: %v", err)
	}

	ctx := context.Background()

	enq, err := s.Enqueue(ctx, task.New{
		Type:        executor.TaskTypeAgent,
		Project:     "demo",
		Payload:     raw,
		MaxAttempts: 1,
		DedupKey:    dedupKey,
	})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	_, claim, err := s.ClaimDue(ctx, testOwner, testLease)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}

	ev, err := json.Marshal(evidence)
	if err != nil {
		t.Fatalf("marshal evidence: %v", err)
	}

	if err := s.Fail(ctx, enq.ID, claim, lastError, 0, ev); err != nil {
		t.Fatalf("fail: %v", err)
	}

	got, err := s.Get(ctx, enq.ID)
	if err != nil || got.Status != task.Dead {
		t.Fatalf("seeded task not dead: %+v (%v)", got, err)
	}

	return got
}

// writeTodoRepo makes a real repo dir with a TODO_LIST.md holding the given
// checkbox lines, and returns the keyed dedup key of the FIRST line's item.
func writeTodoRepo(t *testing.T, lines ...string) (string, string) {
	t.Helper()

	repo := t.TempDir()
	todo := filepath.Join(repo, harvest.DefaultTodoFile)

	if err := os.WriteFile(todo, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatalf("write TODO_LIST: %v", err)
	}

	text := strings.TrimPrefix(strings.TrimPrefix(lines[0], "- [x] "), "- [ ] ")

	return repo, harvest.ItemKey(filepath.Base(repo), text)
}

func gateArtifactEvidence() executor.FailureEvidence {
	return executor.FailureEvidence{
		Stage:       "verify",
		VerifyStage: executor.VerifyStageGofmt,
		Tail:        "ok  \tdemo\t0.01s\n",
	}
}

const gateArtifactLastError = `agent verify gate environmental signature [vendor-gofmt] ("..."): boom`

func TestSweeperAutoDismissesGateArtifactShippedDeath(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)

	sw, err := NewSweeper(context.Background(), s, SweeperConfig{
		Scanner: commitsScanner(),
	})
	if err != nil {
		t.Fatalf("NewSweeper: %v", err)
	}

	repo, key := writeTodoRepo(t, "- [x] ship the widget")
	dead := seedDeadGateArtifactTask(t, s, repo, key, gateArtifactEvidence(), gateArtifactLastError)

	stats, err := sw.Sweep(context.Background())
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}

	if stats.AutoDismissed != 1 || stats.FixesEnqueued != 0 {
		t.Fatalf("stats = %+v, want 1 auto-dismiss, 0 autopsies", stats)
	}

	if tasks := pendingDLQFixTasks(t, s); len(tasks) != 0 {
		t.Fatalf("autopsy minted for an auto-dismissed death: %d", len(tasks))
	}

	got, err := s.Get(context.Background(), dead.ID)
	if err != nil || got.Status != task.Cancelled {
		t.Fatalf("dead task not cancelled: %+v (%v)", got, err)
	}

	trail, err := s.FactsForTask(context.Background(), dead.ID.String(), 0)
	if err != nil {
		t.Fatal(err)
	}

	var detail map[string]string

	sawFact := false

	for _, f := range trail {
		if f.Type != journal.Cancelled {
			continue
		}

		sawFact = true

		if json.Unmarshal(f.Detail, &detail) != nil {
			t.Fatalf("dismiss fact detail not JSON: %s", f.Detail)
		}
	}

	if !sawFact || !strings.Contains(detail["reason"], "gate-artifact auto-dismiss") ||
		!strings.Contains(detail["reason"], executor.VerifyGateEnvCode) ||
		detail["dismissed_by"] != DismissedBySweeper {
		t.Fatalf("dismiss fact = %v (fact seen: %v)", detail, sawFact)
	}
}

// commitsScanner fakes a scanner that always attributes one footer commit.
func commitsScanner() executor.GitScanner {
	return executor.GitScannerFunc(func(context.Context, string, string, string) ([]executor.Commit, error) {
		return []executor.Commit{{SHA: "abc123", Subject: "feat: widget"}}, nil
	})
}

func TestSweeperAutoDismissLegacyAllOkFact(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)

	sw, err := NewSweeper(context.Background(), s, SweeperConfig{
		Scanner: commitsScanner(),
	})
	if err != nil {
		t.Fatalf("NewSweeper: %v", err)
	}

	repo, _ := writeTodoRepo(t, "- [ ] other row")
	key := harvest.ItemKey(filepath.Base(repo), "ship the old widget")
	dead := seedDeadGateArtifactTask(t, s, repo, key,
		executor.FailureEvidence{
			Stage:    "verify",
			ExitCode: 1,
			Tail:     "ok  \tdemo\t0.01s\nok  \tinternal/queue\t0.4s\n",
		},
		`agent verify failed ("go test ./... && test -z "$(gofmt -l .)""): exit 1: ok  demo`,
	)

	stats, err := sw.Sweep(context.Background())
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}

	if stats.AutoDismissed != 1 {
		t.Fatalf("stats = %+v, want the legacy all-ok death auto-dismissed", stats)
	}

	if got, _ := s.Get(context.Background(), dead.ID); got.Status != task.Cancelled {
		t.Fatalf("dead task not cancelled: %s", got.Status)
	}
}

func TestSweeperGateArtifactWithoutShippedProofNeverAutopsies(t *testing.T) {
	t.Parallel()

	noCommits := executor.GitScannerFunc(func(context.Context, string, string, string) ([]executor.Commit, error) {
		return nil, nil
	})

	tests := []struct {
		name    string
		todo    []string
		key     func(repo, key string) string
		scanner executor.GitScanner
	}{
		{
			name:    "item still open",
			todo:    []string{"- [ ] ship the widget"},
			key:     func(_, key string) string { return key },
			scanner: commitsScanner(),
		},
		{
			name:    "no footer commits",
			todo:    []string{"- [x] ship the widget"},
			key:     func(_, key string) string { return key },
			scanner: noCommits,
		},
		{
			name:    "non-harvest task",
			todo:    []string{"- [x] ship the widget"},
			key:     func(_, _ string) string { return "external:id" },
			scanner: commitsScanner(),
		},
		{
			name:    "no scanner wired",
			todo:    []string{"- [x] ship the widget"},
			key:     func(_, key string) string { return key },
			scanner: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := newTestStore(t)

			cfg := SweeperConfig{}
			if tt.scanner != nil {
				cfg.Scanner = tt.scanner
			}

			sw, err := NewSweeper(context.Background(), s, cfg)
			if err != nil {
				t.Fatalf("NewSweeper: %v", err)
			}

			repo, key := writeTodoRepo(t, tt.todo...)
			dead := seedDeadGateArtifactTask(
				t,
				s,
				repo,
				tt.key(repo, key),
				gateArtifactEvidence(),
				gateArtifactLastError,
			)

			stats, err := sw.Sweep(context.Background())
			if err != nil {
				t.Fatalf("sweep: %v", err)
			}

			// No shipped proof → no auto-dismiss; the gate-artifact error
			// marker is machine-proven environmental → autopsy refused. The
			// scanner-run auto-dismiss remains the rescue path once proof
			// lands (see TestSweeperAutoDismissesGateArtifactShippedDeath).
			if stats.EnvSkipped != 1 {
				t.Fatalf("stats = %+v, want 1 env-skipped refusal", stats)
			}

			if tasks := pendingDLQFixTasks(t, s); len(tasks) != 0 {
				t.Fatalf("autopsy minted for environmental gate-artifact death: %+v", tasks)
			}

			if got, err := s.Get(context.Background(), dead.ID); err != nil || got.Status != task.Dead {
				t.Fatalf("dead task left the DLQ: %s (%v)", got.Status, err)
			}
		})
	}
}

// TestSweeperRefusesEnvironmentalAutopsy pins the 2026-10-11 ceremony cut:
// a dead agent task whose LastError carries a machine-proven environmental
// marker (env-streak breaker, verify-gate env/dead/slow classes) gets NO
// autopsy — the paid turn could only rule wontfix. The task stays dead and
// operator-visible; a judged failure still autopsies.
func TestSweeperRefusesEnvironmentalAutopsy(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		lastError string
		refused   bool
	}{
		{"env-streak refused", "environmental requeue streak burned the attempt [env-streak]: streak 3: dirty tree", true},
		{"gate-env refused", "agent verify gate environmental signature [vendor-gofmt] (\".tq-verify\"): all package stages passed", true},
		{"gate-dead refused", "agent verify gate dead (\".tq-verify\"): failure is pre-existing/environmental", true},
		{"gate-slow refused", "agent verify gate slow (\".tq-verify\"): deadline before a verdict", true},
		{"judged failure autopsies", "verify failed: TestShipTheThing", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := newTestStore(t)
			sw := newTestSweeper(t, s)

			dead := seedDeadAgentTaskWithReason(t, s, executor.AgentPayload{
				Repo:   "demo",
				Prompt: "ship the frobnicator",
				Yolo:   true,
			}, tc.lastError)

			stats, err := sw.Sweep(context.Background())
			if err != nil {
				t.Fatalf("sweep: %v", err)
			}

			tasks := pendingDLQFixTasks(t, s)

			if tc.refused {
				if stats.EnvSkipped != 1 || len(tasks) != 0 {
					t.Fatalf("env death must refuse the autopsy: stats %+v, dlqfix tasks %d", stats, len(tasks))
				}

				got, err := s.Get(context.Background(), dead.ID)
				if err != nil || got.Status != task.Dead {
					t.Fatalf("refused autopsy must leave the task dead for the operator: %+v (%v)", got, err)
				}

				return
			}

			if stats.FixesEnqueued != 1 || len(tasks) != 1 {
				t.Fatalf("judged failure must still autopsy: stats %+v, dlqfix tasks %d", stats, len(tasks))
			}
		})
	}
}

// seedDeadAgentTaskWithReason is seedDeadAgentTask with a parameterized
// failure reason (the environmental markers live in the reason text).
func seedDeadAgentTaskWithReason(
	t *testing.T,
	s *sqlite.Store,
	payload executor.AgentPayload,
	reason string,
) task.Task {
	t.Helper()

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal agent payload: %v", err)
	}

	ctx := context.Background()

	enq, err := s.Enqueue(ctx, task.New{
		Type:        executor.TaskTypeAgent,
		Project:     "demo",
		Priority:    7,
		Payload:     raw,
		MaxAttempts: 1,
		DedupKey:    "seed:" + payload.Prompt,
	})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	_, claim, err := s.ClaimDue(ctx, testOwner, testLease)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}

	evidence, err := json.Marshal(
		executor.FailureEvidence{Stage: "verify", ExitCode: 2, Tail: "FAIL: TestShipTheThing"},
	)
	if err != nil {
		t.Fatalf("marshal evidence: %v", err)
	}

	if err := s.Fail(ctx, enq.ID, claim, reason, 0, evidence); err != nil {
		t.Fatalf("fail: %v", err)
	}

	got, err := s.Get(ctx, enq.ID)
	if err != nil || got.Status != task.Dead {
		t.Fatalf("seeded task not dead: %+v (%v)", got, err)
	}

	return got
}

// TestSweeperMintRefusalHoldsCheckpoint pins the hardening (13-15 §e): a
// refused autopsy mint (transient store failure) aborts the sweep BEFORE
// the checkpoint — warned, watermark held behind the death — so the retry
// sweep of the SAME cursor redelivers the death and mints the autopsy.
func TestSweeperMintRefusalHoldsCheckpoint(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	ctx := context.Background()

	flaky := &flakyEnqueueStore{Store: s}
	flaky.failures.Store(1)

	capture := &captureHandler{}

	sw, err := NewSweeper(ctx, flaky, SweeperConfig{Log: slog.New(capture)})
	if err != nil {
		t.Fatalf("NewSweeper: %v", err)
	}

	dead := seedDeadAgentTask(t, s, executor.AgentPayload{Repo: "demo", Prompt: "ship it"})

	stats, err := sw.Sweep(ctx)
	if err == nil || !strings.Contains(err.Error(), "mint refused") {
		t.Fatalf("sweep err = %v, want mint refusal", err)
	}

	if stats.FixesEnqueued != 0 {
		t.Fatalf("FixesEnqueued = %d, want 0 (stats %+v)", stats.FixesEnqueued, stats)
	}

	deadSeq := deadLetterSeq(t, s, dead.ID)

	wm, ok, err := s.Watermark(ctx, ConsumerKey)
	if err != nil || !ok {
		t.Fatalf("watermark: %v (ok %v)", err, ok)
	}

	if wm >= deadSeq {
		t.Fatalf("watermark = %d, want held behind the dead-letter fact at %d", wm, deadSeq)
	}

	warned := false

	for _, r := range capture.records {
		if r.Level == slog.LevelWarn && strings.Contains(r.Message, "mint refused") {
			warned = true
		}
	}

	if !warned {
		t.Fatalf("mint-refusal warn missing from %+v", capture.records)
	}

	flaky.failures.Store(0)

	retry, err := sw.Sweep(ctx)
	if err != nil {
		t.Fatalf("retry sweep: %v", err)
	}

	if retry.FixesEnqueued != 1 {
		t.Fatalf("retry FixesEnqueued = %d, want 1 (stats %+v)", retry.FixesEnqueued, retry)
	}

	if tasks := pendingDLQFixTasks(t, s); len(tasks) != 1 {
		t.Fatalf("dlqfix tasks = %d, want 1", len(tasks))
	}
}

// TestSweeperDoneGuardIsKnownNotRefusal pins that the Queue wrapper's
// done-guard (the dedup key matched a COMPLETED autopsy after an operator
// watermark rewind) counts known and checkpoints past — it must not wedge
// the cursor the way a store failure does.
func TestSweeperDoneGuardIsKnownNotRefusal(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	ctx := context.Background()

	sw := newTestSweeper(t, queue.New(s))

	seedDeadAgentTask(t, s, executor.AgentPayload{Repo: "demo", Prompt: "p"})

	if _, err := sw.Sweep(ctx); err != nil {
		t.Fatalf("mint sweep: %v", err)
	}

	autopsy := pendingDLQFixTasks(t, s)[0]

	finishTask(t, s, autopsy.ID, executor.DLQFixResult{
		Verdict: executor.VerdictWontFix,
		Summary: "needs credentials only the operator holds",
	})

	if err := s.SetWatermark(ctx, ConsumerKey, 0); err != nil {
		t.Fatalf("rewind: %v", err)
	}

	replay := newTestSweeper(t, queue.New(s))

	stats, err := replay.Sweep(ctx)
	if err != nil {
		t.Fatalf("replay sweep: %v", err)
	}

	if stats.FixesKnown != 1 {
		t.Fatalf("stats = %+v, want the done-guard counted known", stats)
	}

	if tasks := pendingDLQFixTasks(t, s); len(tasks) != 1 {
		t.Fatalf("dlqfix tasks = %d, want 1 (no re-mint)", len(tasks))
	}
}

// TestSweeperDeterministicSkipsDoNotGate pins the flip side of the
// hardening: by-design skips (a dead non-agent task) checkpoint past — a
// later dead agent task in the same page still mints.
func TestSweeperDeterministicSkipsDoNotGate(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	ctx := context.Background()

	sw := newTestSweeper(t, s)

	sh, err := s.Enqueue(ctx, task.New{
		Type: "sh", Project: "demo", MaxAttempts: 1,
		Payload: []byte(`{}`), DedupKey: "guard:sh",
	})
	if err != nil {
		t.Fatalf("enqueue sh: %v", err)
	}

	_, claim, err := s.ClaimDue(ctx, testOwner, testLease)
	if err != nil {
		t.Fatalf("claim sh: %v", err)
	}

	if err := s.Fail(ctx, sh.ID, claim, "boom", 0, nil); err != nil {
		t.Fatalf("fail sh: %v", err)
	}

	seedDeadAgentTask(t, s, executor.AgentPayload{Repo: "demo", Prompt: "p"})

	stats, err := sw.Sweep(ctx)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}

	if stats.Skipped != 1 || stats.FixesEnqueued != 1 {
		t.Fatalf("stats = %+v, want 1 skipped + 1 minted", stats)
	}
}

// flakyDisposeStore fails Gets for one task ID and DismissDead calls with a
// store-shaped error — the transient failure class a disposition refusal
// represents. Everything else rides the real store.
type flakyDisposeStore struct {
	queue.Store

	failID      task.ID
	failGets    atomic.Int32
	failDismiss atomic.Int32
}

func (f *flakyDisposeStore) Get(ctx context.Context, id task.ID) (task.Task, error) {
	if id == f.failID && f.failGets.Load() > 0 {
		f.failGets.Add(-1)

		return task.Task{}, errors.New("sqlite: database is locked (simulated)")
	}

	return f.Store.Get(ctx, id)
}

func (f *flakyDisposeStore) DismissDead(ctx context.Context, id task.ID, reason, by string) error {
	if f.failDismiss.Load() > 0 {
		f.failDismiss.Add(-1)

		return errors.New("sqlite: database is locked (simulated)")
	}

	return f.Store.DismissDead(ctx, id, reason, by)
}

// disposeRefusalSweep seeds a dead agent task, mints and completes its
// autopsy with the given verdict, and returns the pieces the refusal tests
// pin against.
func disposeRefusalSweep(
	t *testing.T,
	s *sqlite.Store,
	sw *Sweeper,
	verdict executor.DLQFixVerdict,
) (dead, autopsy task.ID) {
	t.Helper()

	seeded := seedDeadAgentTask(t, s, executor.AgentPayload{Repo: "demo", Prompt: "ship it"})

	if _, err := sw.Sweep(context.Background()); err != nil {
		t.Fatalf("mint sweep: %v", err)
	}

	autopsyTask := pendingDLQFixTasks(t, s)[0]

	finishTask(t, s, autopsyTask.ID, executor.DLQFixResult{Verdict: verdict, Summary: "s"})

	return seeded.ID, autopsyTask.ID
}

// pinRefusalHoldsCheckpoint asserts the refusal shape shared by the dispose
// hardening tests: the sweep errored, the watermark is held behind the
// autopsy's Completed fact, and the refusal was warned.
func pinRefusalHoldsCheckpoint(t *testing.T, s *sqlite.Store, capture *captureHandler, autopsy task.ID, stats SweepStats, err error) {
	t.Helper()

	if err == nil || !strings.Contains(err.Error(), "disposition refused") {
		t.Fatalf("sweep err = %v, want disposition refusal", err)
	}

	if stats.Rescued != 0 || stats.Dismissed != 0 {
		t.Fatalf("stats = %+v, want no executed disposition (stats)", stats)
	}

	completedSeq := factSeqFor(t, s, journal.Completed, autopsy)

	wm, ok, err := s.Watermark(context.Background(), ConsumerKey)
	if err != nil || !ok {
		t.Fatalf("watermark: %v (ok %v)", err, ok)
	}

	if wm >= completedSeq {
		t.Fatalf("watermark = %d, want held behind the Completed fact at %d", wm, completedSeq)
	}

	warned := false

	for _, r := range capture.records {
		if r.Level == slog.LevelWarn && strings.Contains(r.Message, "disposition refused") {
			warned = true
		}
	}

	if !warned {
		t.Fatalf("disposition-refusal warn missing from %+v", capture.records)
	}
}

// TestSweeperDisposeRefusalHoldsCheckpoint pins the dispose-path hardening
// (sibling of the 2026-10-18 mint-refusal gate): a transient store failure
// reading the completed autopsy aborts the sweep BEFORE the checkpoint —
// warned, watermark held behind the Completed fact — so the retry sweep of
// the SAME cursor redelivers the fact and executes the verdict instead of
// silently dropping a completed autopsy's disposition forever.
func TestSweeperDisposeRefusalHoldsCheckpoint(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	ctx := context.Background()

	flaky := &flakyDisposeStore{Store: s}
	capture := &captureHandler{}

	sw, err := NewSweeper(ctx, flaky, SweeperConfig{Log: slog.New(capture)})
	if err != nil {
		t.Fatalf("NewSweeper: %v", err)
	}

	_, autopsy := disposeRefusalSweep(t, s, sw, executor.VerdictFixed)

	flaky.failID = autopsy
	flaky.failGets.Store(1)

	stats, err := sw.Sweep(ctx)
	pinRefusalHoldsCheckpoint(t, s, capture, autopsy, stats, err)

	flaky.failGets.Store(0)

	retry, err := sw.Sweep(ctx)
	if err != nil {
		t.Fatalf("retry sweep: %v", err)
	}

	if retry.Rescued != 1 {
		t.Fatalf("retry Rescued = %d, want 1 (stats %+v)", retry.Rescued, retry)
	}
}

// TestSweeperRescueReadRefusalHoldsCheckpoint pins the rescue leg of the
// gate: a transient failure reading the DEAD record during a "fixed"
// disposition refuses and holds the checkpoint; the retry rescues.
func TestSweeperRescueReadRefusalHoldsCheckpoint(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	ctx := context.Background()

	flaky := &flakyDisposeStore{Store: s}
	capture := &captureHandler{}

	sw, err := NewSweeper(ctx, flaky, SweeperConfig{Log: slog.New(capture)})
	if err != nil {
		t.Fatalf("NewSweeper: %v", err)
	}

	dead, autopsy := disposeRefusalSweep(t, s, sw, executor.VerdictFixed)

	flaky.failID = dead
	flaky.failGets.Store(1)

	stats, err := sw.Sweep(ctx)
	pinRefusalHoldsCheckpoint(t, s, capture, autopsy, stats, err)

	flaky.failGets.Store(0)

	retry, err := sw.Sweep(ctx)
	if err != nil {
		t.Fatalf("retry sweep: %v", err)
	}

	if retry.Rescued != 1 {
		t.Fatalf("retry Rescued = %d, want 1 (stats %+v)", retry.Rescued, retry)
	}
}

// TestSweeperDismissRefusalHoldsCheckpoint pins the dismiss leg of the gate:
// a transient DismissDead failure on a "wontfix" verdict refuses and holds
// the checkpoint; the retry dismisses.
func TestSweeperDismissRefusalHoldsCheckpoint(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	ctx := context.Background()

	flaky := &flakyDisposeStore{Store: s}
	capture := &captureHandler{}

	sw, err := NewSweeper(ctx, flaky, SweeperConfig{Log: slog.New(capture)})
	if err != nil {
		t.Fatalf("NewSweeper: %v", err)
	}

	_, autopsy := disposeRefusalSweep(t, s, sw, executor.VerdictWontFix)

	flaky.failDismiss.Store(1)

	stats, err := sw.Sweep(ctx)
	pinRefusalHoldsCheckpoint(t, s, capture, autopsy, stats, err)

	flaky.failDismiss.Store(0)

	retry, err := sw.Sweep(ctx)
	if err != nil {
		t.Fatalf("retry sweep: %v", err)
	}

	if retry.Dismissed != 1 {
		t.Fatalf("retry Dismissed = %d, want 1 (stats %+v)", retry.Dismissed, retry)
	}
}

// missingAutopsyStore hides one task record — the permanent (deterministic)
// store outcome: a Completed fact whose record is gone can never yield a
// disposition, so it must skip past instead of wedging the cursor.
type missingAutopsyStore struct {
	queue.Store

	missing task.ID
}

func (m *missingAutopsyStore) Get(ctx context.Context, id task.ID) (task.Task, error) {
	if id == m.missing {
		return task.Task{}, task.ErrNotFound
	}

	return m.Store.Get(ctx, id)
}

// TestSweeperPermanentStoreOutcomeSkipsPast pins the flip side of the
// disposition gate: a permanent outcome (record not found) checkpoints past
// — the watermark ADVANCES behind the Completed fact, no error, no wedge.
func TestSweeperPermanentStoreOutcomeSkipsPast(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	ctx := context.Background()

	sw := newTestSweeper(t, s)

	_, autopsy := disposeRefusalSweep(t, s, sw, executor.VerdictFixed)

	masked := &missingAutopsyStore{Store: s, missing: autopsy}
	retry := newTestSweeper(t, masked)

	stats, err := retry.Sweep(ctx)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}

	if stats.Skipped != 1 || stats.Rescued != 0 {
		t.Fatalf("stats = %+v, want 1 skipped, no rescue", stats)
	}

	completedSeq := factSeqFor(t, s, journal.Completed, autopsy)

	wm, ok, err := s.Watermark(ctx, ConsumerKey)
	if err != nil || !ok {
		t.Fatalf("watermark: %v (ok %v)", err, ok)
	}

	if wm < completedSeq {
		t.Fatalf("watermark = %d, want advanced past the Completed fact at %d", wm, completedSeq)
	}
}
