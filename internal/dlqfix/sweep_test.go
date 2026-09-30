package dlqfix

import (
	"context"
	"encoding/json/v2"
	"os"
	"path/filepath"
	"strings"
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

func newTestSweeper(t *testing.T, s *sqlite.Store) *Sweeper {
	t.Helper()

	sw, err := NewSweeper(context.Background(), s, SweeperConfig{Model: "", Log: nil})
	if err != nil {
		t.Fatalf("NewSweeper: %v", err)
	}

	return sw
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

func TestSweeperGateArtifactWithoutShippedProofKeepsAutopsy(t *testing.T) {
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
			seedDeadGateArtifactTask(t, s, repo, tt.key(repo, key), gateArtifactEvidence(), gateArtifactLastError)

			stats, err := sw.Sweep(context.Background())
			if err != nil {
				t.Fatalf("sweep: %v", err)
			}

			if stats.AutoDismissed != 0 || stats.FixesEnqueued != 1 {
				t.Fatalf("stats = %+v, want 0 auto-dismissed, 1 autopsy", stats)
			}
		})
	}
}
