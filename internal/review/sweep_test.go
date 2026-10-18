package review

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

func newTestStore(t *testing.T) *sqlite.Store {
	t.Helper()

	s, err := sqlite.Open(filepath.Join(t.TempDir(), "q.db"))
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}

	t.Cleanup(func() { _ = s.Close() })

	return s
}

const (
	testOwner = "test-worker"
	testLease = 5 * time.Minute
)

// finishTask claims and completes one pending task with result detail.
func finishTask(t *testing.T, s *sqlite.Store, id task.ID, detail json.RawMessage) {
	t.Helper()

	ctx := context.Background()

	_, claim, err := s.ClaimDue(ctx, testOwner, testLease)
	if err != nil {
		t.Fatalf("claim %s: %v", id, err)
	}

	if err := s.Complete(ctx, id, claim, detail); err != nil {
		t.Fatalf("complete %s: %v", id, err)
	}
}

// runAgentTask enqueues and completes one agent task so the journal holds a
// real task.completed fact with result detail.
func runAgentTask(
	t *testing.T,
	s *sqlite.Store,
	payload executor.AgentPayload,
	result executor.AgentResult,
) task.Task {
	t.Helper()

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal agent payload: %v", err)
	}

	enq, err := s.Enqueue(context.Background(), task.New{
		Type:     executor.TaskTypeAgent,
		Project:  "demo",
		Payload:  raw,
		DedupKey: "seed:" + payload.Prompt,
	})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	detail, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}

	finishTask(t, s, enq.ID, detail)

	return enq
}

// runReviewTask enqueues and completes one review task.
func runReviewTask(
	t *testing.T,
	s *sqlite.Store,
	payload executor.ReviewPayload,
	result executor.ReviewResult,
) task.Task {
	t.Helper()

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal review payload: %v", err)
	}

	enq, err := s.Enqueue(context.Background(), task.New{
		Type:     executor.TaskTypeReview,
		Project:  "demo",
		Payload:  raw,
		DedupKey: "seed-review:" + payload.ReviewedTask,
	})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	detail, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}

	finishTask(t, s, enq.ID, detail)

	return enq
}

// listByType returns every task of one type (the store does not expose
// dedup-key lookups; type filtering is enough to find what a sweep minted).
func listByType(t *testing.T, s *sqlite.Store, taskType string) []task.Task {
	t.Helper()

	tasks, err := s.List(context.Background(), queue.Filter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	var out []task.Task

	for _, tk := range tasks {
		if tk.Type == taskType {
			out = append(out, tk)
		}
	}

	return out
}

func TestSweepEnqueuesOneReviewPerCompletedAgentTask(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	ctx := context.Background()

	// The sweeper is created FIRST (it rides alongside the pool): its
	// watermark starts at the journal head at first Sweep, then follows.
	sw, err := NewSweeper(ctx, s, SweeperConfig{})

	done := runAgentTask(t, s, executor.AgentPayload{
		Repo: "demo", Prompt: "add the frobnicator", Yolo: true,
	}, executor.AgentResult{CommitSHA: "abc1234", FilesChanged: []string{"frob.go"}})

	stats, err := sw.Sweep(ctx)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}

	if stats.ReviewsEnqueued != 1 {
		t.Fatalf("reviews enqueued = %d, want 1 (stats %+v)", stats.ReviewsEnqueued, stats)
	}

	reviews := listByType(t, s, executor.TaskTypeReview)
	if len(reviews) != 1 {
		t.Fatalf("review tasks in store = %d, want 1", len(reviews))
	}

	if reviews[0].Project != "demo" {
		t.Fatalf("review project = %q, want demo", reviews[0].Project)
	}

	var got executor.ReviewPayload
	if err := json.Unmarshal(reviews[0].Payload, &got); err != nil {
		t.Fatalf("review payload: %v", err)
	}

	if got.Repo != "demo" || got.ReviewedTask != done.ID.String() ||
		got.Item != "add the frobnicator" || got.CommitSHA != "abc1234" ||
		len(got.FilesChanged) != 1 || !got.Yolo {
		t.Fatalf("review payload mismatch: %+v", got)
	}

	again, err := sw.Sweep(ctx)
	if err != nil {
		t.Fatalf("second sweep: %v", err)
	}

	if again.ReviewsEnqueued != 0 || again.ReviewsKnown != 0 {
		t.Fatalf("second sweep must be a no-op on a quiet journal, got %+v", again)
	}
}

func TestSweepSkipsNonAgentCompletions(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	ctx := context.Background()

	enq, err := s.Enqueue(ctx, task.New{Type: "sh", Payload: json.RawMessage(`{"cmd":"echo hi"}`)})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	finishTask(t, s, enq.ID, nil)

	sw, err := NewSweeper(ctx, s, SweeperConfig{})

	stats, err := sw.Sweep(ctx)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}

	if stats.ReviewsEnqueued != 0 {
		t.Fatalf("sh completions must not be reviewed, got %+v", stats)
	}
}

func TestSweepAutofixMintsFixTasksPerFinding(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	ctx := context.Background()

	sw, err := NewSweeper(ctx, s, SweeperConfig{Autofix: true})

	runAgentTask(
		t,
		s,
		executor.AgentPayload{Repo: "demo", Prompt: "original item"},
		executor.AgentResult{CommitSHA: "def5678"},
	)

	runReviewTask(t, s, executor.ReviewPayload{
		Repo:         "demo",
		ReviewedTask: "t-reviewed",
		Item:         "original item",
		CommitSHA:    "def5678",
	}, executor.ReviewResult{
		Verdict: executor.VerdictRequestChanges,
		Findings: []executor.ReviewFinding{
			{Title: "nil map write", Severity: "high", Detail: "write to nil map in frob.go"},
			{Title: "missing test", Severity: "medium"},
		},
	})

	stats, err := sw.Sweep(ctx)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}

	if stats.FixesEnqueued != 2 {
		t.Fatalf("fixes enqueued = %d, want 2 (stats %+v)", stats.FixesEnqueued, stats)
	}

	fixes := listByType(t, s, executor.TaskTypeAgent)
	// The seeded, completed agent task is also type "agent" — count only
	// the pending fix tasks the sweep minted.
	var pendingFixes []task.Task

	for _, fix := range fixes {
		if fix.Status == task.Pending {
			pendingFixes = append(pendingFixes, fix)
		}
	}

	if len(pendingFixes) != 2 {
		t.Fatalf("pending fix tasks in store = %d, want 2", len(pendingFixes))
	}

	var nilMapFix task.Task

	for _, fix := range pendingFixes {
		var p executor.AgentPayload

		if err := json.Unmarshal(fix.Payload, &p); err != nil {
			t.Fatalf("fix payload: %v", err)
		}

		if strings.Contains(p.Prompt, "nil map write") {
			nilMapFix = fix

			for _, want := range []string{"original item", "high", "def5678", "Fix this finding"} {
				if !strings.Contains(p.Prompt, want) {
					t.Errorf("fix prompt missing %q", want)
				}
			}
		}
	}

	if nilMapFix.ID == "" || nilMapFix.Project != "demo" {
		t.Fatal("fix task for 'nil map write' not found")
	}

	again, err := sw.Sweep(ctx)
	if err != nil {
		t.Fatalf("resweep: %v", err)
	}

	if again.FixesEnqueued != 0 {
		t.Fatalf("re-sweep must not duplicate fix tasks, got %+v", again)
	}
}

// TestFixPromptFooterContract pins the fix prompt's two id surfaces: the
// QUOTED original contract resolves {{TASK_ID}} to the reviewed task's id
// (what the work agent actually saw), while the fix task's own commit
// instruction keeps the placeholder so runAgent resolves it to the fix
// task's id at run time. Mixing them up is how a reviewer or fix agent
// ends up demanding the wrong run's id in commit footers.
func TestFixPromptFooterContract(t *testing.T) {
	t.Parallel()

	prompt := fixPrompt(executor.ReviewPayload{
		ReviewedTask: "000001a0reviewedtaskid00000000000",
		Item:         "do the work\n\nTask-Queue-ID: {{TASK_ID}}",
		CommitSHA:    "abc1234",
	}, executor.ReviewFinding{Title: "nil map write", Severity: "high"})

	if !strings.Contains(prompt, "Task-Queue-ID: 000001a0reviewedtaskid00000000000") {
		t.Fatalf("quoted original must resolve its footer to the reviewed task's id:\n%s", prompt)
	}

	if strings.Count(prompt, "{{TASK_ID}}") != 1 {
		t.Fatalf("exactly the fix task's own footer instruction keeps the placeholder, got %d occurrences",
			strings.Count(prompt, "{{TASK_ID}}"))
	}

	if !strings.Contains(prompt, "Task-Queue-ID: {{TASK_ID}}\n") {
		t.Fatalf("fix task's own footer instruction must carry the placeholder verbatim:\n%s", prompt)
	}
}

// TestFixPromptAnchorsAndDanglingDisposition pins the hardening contract:
// the fix prompt carries the finding's anchor text (falling back to the
// payload's commit for the sha), and a dangling-commit disposition rule —
// re-anchor via the quoted text, never invent a change when the anchor is
// gone. The single-footer ruling (one Task-Queue-ID per commit, fix
// ticket's id) is pinned verbatim.
func TestFixPromptAnchorsAndDanglingDisposition(t *testing.T) {
	t.Parallel()

	prompt := fixPrompt(executor.ReviewPayload{
		ReviewedTask: "000001a0reviewedtaskid00000000000",
		Item:         "do the work",
		CommitSHA:    "payloadsha1",
	}, executor.ReviewFinding{Title: "nil map write", Severity: "high", Anchor: "m[k] = v", CommitSHA: "findingsha2"})

	for _, want := range []string{
		"`m[k] = v`",
		"git cat-file -e findingsha2",
		"re-anchor via the verbatim anchor text",
		"ONE Task-Queue-ID footer per commit",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("fix prompt missing %q:\n%s", want, prompt)
		}
	}

	if !strings.Contains(prompt, "findingsha2") || strings.Contains(prompt, "payloadsha1") {
		t.Errorf("finding-level commit sha must win over the payload's:\n%s", prompt)
	}

	if strings.Count(prompt, "Task-Queue-ID:") != 1 {
		t.Errorf("fix prompt must carry exactly one footer instruction, got:\n%s", prompt)
	}

	// Without a finding-level sha the payload's commit is the anchor.
	fallback := fixPrompt(executor.ReviewPayload{
		ReviewedTask: "000001a0reviewedtaskid00000000000",
		Item:         "do the work",
		CommitSHA:    "payloadsha1",
	}, executor.ReviewFinding{Title: "t", Severity: "low", Anchor: "some text"})
	if !strings.Contains(fallback, "git cat-file -e payloadsha1") {
		t.Errorf("payload sha must anchor the finding when the finding omits its own:\n%s", fallback)
	}
}

func TestSweepAutofixIgnoresApproveAndOffSwitch(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	ctx := context.Background()

	reviewed := runAgentTask(t, s, executor.AgentPayload{Repo: "demo", Prompt: "item"}, executor.AgentResult{})
	runReviewTask(t, s, executor.ReviewPayload{
		Repo: "demo", ReviewedTask: reviewed.ID.String(), Item: "item",
	}, executor.ReviewResult{Verdict: executor.VerdictApprove})

	swOff, err := NewSweeper(ctx, s, SweeperConfig{})
	if err != nil {
		t.Fatalf("new sweeper off: %v", err)
	}

	statsOff, err := swOff.Sweep(ctx)
	if err != nil {
		t.Fatalf("sweep off: %v", err)
	}

	if statsOff.FixesEnqueued != 0 {
		t.Fatalf("autofix off must not mint fixes, got %+v", statsOff)
	}

	swOn, err := NewSweeper(ctx, s, SweeperConfig{Autofix: true})
	if err != nil {
		t.Fatalf("new sweeper on: %v", err)
	}

	statsOn, err := swOn.Sweep(ctx)
	if err != nil {
		t.Fatalf("sweep on: %v", err)
	}

	if statsOn.FixesEnqueued != 0 {
		t.Fatalf("approve verdicts must not mint fixes, got %+v", statsOn)
	}
}

// TestSweepWatermarkResumesAcrossSweeps: only NEW completions after the
// sweeper's first pass get reviews — the watermark moves forward.
func TestSweepWatermarkResumesAcrossSweeps(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	ctx := context.Background()

	sw, err := NewSweeper(ctx, s, SweeperConfig{})
	if err != nil {
		t.Fatalf("new sweeper: %v", err)
	}

	first := runAgentTask(t, s, executor.AgentPayload{Repo: "demo", Prompt: "first"}, executor.AgentResult{})

	if _, err := sw.Sweep(ctx); err != nil {
		t.Fatalf("first sweep: %v", err)
	}

	// The review of `first` is now pending; run it to completion (what the
	// pool would do) so it cannot shadow the next agent task in ClaimDue
	// ordering.
	for _, rev := range listByType(t, s, executor.TaskTypeReview) {
		approve, err := json.Marshal(executor.ReviewResult{Verdict: executor.VerdictApprove})
		if err != nil {
			t.Fatal(err)
		}

		finishTask(t, s, rev.ID, approve)
	}

	second := runAgentTask(t, s, executor.AgentPayload{Repo: "demo", Prompt: "second"}, executor.AgentResult{})

	stats, err := sw.Sweep(ctx)
	if err != nil {
		t.Fatalf("second sweep: %v", err)
	}

	if stats.ReviewsEnqueued != 1 {
		t.Fatalf("only the new completion gets a review, got %+v", stats)
	}

	var firstReviewed, secondReviewed bool

	for _, rev := range listByType(t, s, executor.TaskTypeReview) {
		var payload executor.ReviewPayload

		if err := json.Unmarshal(rev.Payload, &payload); err != nil {
			t.Fatalf("review payload: %v", err)
		}

		switch payload.ReviewedTask {
		case first.ID.String():
			firstReviewed = true
		case second.ID.String():
			secondReviewed = true
		}
	}

	if !firstReviewed || !secondReviewed {
		t.Fatalf("reviewed coverage wrong: first=%v second=%v", firstReviewed, secondReviewed)
	}
}

func TestSweepWatermarkStartsAtHead(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	ctx := context.Background()

	runAgentTask(t, s, executor.AgentPayload{Repo: "demo", Prompt: "pre-existing"}, executor.AgentResult{})

	// A sweeper created AFTER the completion starts at the journal head:
	// historical completions are not replayed (documented gap semantics,
	// same as the papdashboard bridge watermark).
	sw, err := NewSweeper(ctx, s, SweeperConfig{})

	stats, err := sw.Sweep(ctx)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}

	if stats.ReviewsEnqueued != 0 {
		t.Fatalf("sweeper must not replay pre-start completions, got %+v", stats)
	}
}

func TestSweepCatchesUpAcrossRestarts(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	ctx := context.Background()

	// Pool A ran once and checkpointed; its sweeper then died with it.
	first, err := NewSweeper(ctx, s, SweeperConfig{})
	if err != nil {
		t.Fatalf("new sweeper A: %v", err)
	}

	if _, err := first.Sweep(ctx); err != nil {
		t.Fatalf("sweep A: %v", err)
	}

	// While no sweeper was running, an agent task completed.
	done := runAgentTask(
		t,
		s,
		executor.AgentPayload{Repo: "demo", Prompt: "completed-while-down"},
		executor.AgentResult{},
	)

	// Pool B starts: the persisted cursor resumes BEFORE that completion,
	// so it is reviewed despite the sweeper having been down.
	second, err := NewSweeper(ctx, s, SweeperConfig{})
	if err != nil {
		t.Fatalf("new sweeper B: %v", err)
	}

	stats, err := second.Sweep(ctx)
	if err != nil {
		t.Fatalf("sweep B: %v", err)
	}

	if stats.ReviewsEnqueued != 1 {
		t.Fatalf("completion while sweeper was down was not reviewed, got %+v", stats)
	}

	reviews := listByType(t, s, executor.TaskTypeReview)
	if len(reviews) != 1 {
		t.Fatalf("got %d review tasks, want 1", len(reviews))
	}

	var payload executor.ReviewPayload
	if err := json.Unmarshal(reviews[0].Payload, &payload); err != nil {
		t.Fatalf("review payload: %v", err)
	}

	if payload.ReviewedTask != done.ID.String() {
		t.Fatalf("reviewed task = %s, want %s", payload.ReviewedTask, done.ID)
	}
}

func TestFixDedupKeyIsStableAndDistinct(t *testing.T) {
	t.Parallel()

	a := FixDedupKey(task.ID("r1"), "nil map write")
	b := FixDedupKey(task.ID("r1"), "nil map write")
	c := FixDedupKey(task.ID("r1"), "other finding")
	d := FixDedupKey(task.ID("r2"), "nil map write")

	if a != b {
		t.Fatal("same review + title must produce the same key")
	}

	if a == c || a == d {
		t.Fatal("different title or review must produce a different key")
	}
}

// TestSweepSkipsPreflightDoneCompletions pins the no-review contract of
// done-preflight completions: a completion whose AgentResult carries
// PreflightDone never ran a session, so minting its review would re-create
// the exact no-op agent burn the gate exists to kill.
func TestSweepSkipsPreflightDoneCompletions(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	ctx := context.Background()

	sw, err := NewSweeper(ctx, s, SweeperConfig{})
	if err != nil {
		t.Fatalf("sweeper: %v", err)
	}

	runAgentTask(t, s, executor.AgentPayload{
		Repo: "demo", Prompt: "already landed elsewhere", Yolo: true,
	}, executor.AgentResult{PreflightDone: "done preflight: commit(s) already carry this task's Task-Queue-ID footer"})

	stats, err := sw.Sweep(ctx)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}

	if stats.ReviewsEnqueued != 0 {
		t.Fatalf("reviews enqueued = %d, want 0 (stats %+v)", stats.ReviewsEnqueued, stats)
	}

	if reviews := listByType(t, s, executor.TaskTypeReview); len(reviews) != 0 {
		t.Fatalf("review tasks in store = %d, want 0", len(reviews))
	}

	if stats.Skipped != 1 {
		t.Fatalf("skipped = %d, want 1 (the preflight-done completion)", stats.Skipped)
	}
}

// TestSweepSkipsDocsOnlyChanges pins the 2026-10-11 ceremony cut: a
// completed agent run whose derived diff is documentation only (prose
// formats, docs trees, licenses) does not mint a paid review — one
// code-bearing file or no derivation at all still reviews.
func TestSweepSkipsDocsOnlyChanges(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	for _, tc := range []struct {
		name       string
		files      []string
		wantSkip   bool
		wantReview bool
	}{
		{"docs-only skipped", []string{"README.md", "docs/arch.md", "LICENSE"}, true, false},
		{"mixed reviews", []string{"README.md", "main.go"}, false, true},
		{"code reviews", []string{"internal/foo.go"}, false, true},
		{"no derivation reviews", nil, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := newTestStore(t)

			// Sweeper FIRST: NewSweeper bootstraps at the journal head, so
			// completions recorded before it exist are not trigger facts.
			sw, err := NewSweeper(ctx, s, SweeperConfig{})
			if err != nil {
				t.Fatalf("NewSweeper: %v", err)
			}

			runAgentTask(t, s,
				executor.AgentPayload{Repo: "demo", Prompt: "docs item " + tc.name},
				executor.AgentResult{CommitSHA: "abc123", FilesChanged: tc.files},
			)

			stats, err := sw.Sweep(ctx)
			if err != nil {
				t.Fatalf("sweep: %v", err)
			}

			reviews := listByType(t, s, executor.TaskTypeReview)
			if tc.wantSkip && stats.DocsSkipped != 1 {
				t.Fatalf("DocsSkipped = %d, want 1 (stats %+v)", stats.DocsSkipped, stats)
			}

			if tc.wantSkip && len(reviews) != 0 {
				t.Fatalf("docs-only change minted a review: %+v", reviews)
			}

			if tc.wantReview && len(reviews) != 1 {
				t.Fatalf("review tasks = %d, want 1", len(reviews))
			}
		})
	}
}

// TestDocsLikePath pins the file classifier the docs-only cut rides on.
func TestDocsLikePath(t *testing.T) {
	t.Parallel()

	for _, docs := range []string{"README.md", "docs/x/y.md", "CHANGELOG.md", "LICENSE", "NOTICE", "notes.txt", "spec.adoc"} {
		if !docsLikePath(docs) {
			t.Errorf("%q must classify as docs", docs)
		}
	}

	for _, code := range []string{"main.go", "app.css", "flake.nix", ".github/workflows/ci.yml", "go.mod", "run.sh"} {
		if docsLikePath(code) {
			t.Errorf("%q must classify as code", code)
		}
	}
}

func TestMergeOnApprove(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()

	payload, err := json.Marshal(executor.ReviewPayload{
		Repo:        repo,
		Branch:      "tq/0001",
		BaseBranch:  "master",
		WorktreeDir: "/wt/0001",
	})
	if err != nil {
		t.Fatal(err)
	}

	approve, err := json.Marshal(executor.ReviewResult{Verdict: executor.VerdictApprove})
	if err != nil {
		t.Fatal(err)
	}

	requestChanges, err := json.Marshal(executor.ReviewResult{Verdict: executor.VerdictRequestChanges})
	if err != nil {
		t.Fatal(err)
	}

	tk := task.Task{Type: executor.TaskTypeReview, Payload: payload}

	tests := []struct {
		name   string
		detail []byte
		want   int
	}{
		{"approve merges", approve, 1},
		{"request_changes keeps the branch", requestChanges, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var (
				merges []string
				s      = &Sweeper{}
				stats  SweepStats
			)

			s.cfg.Merge = func(repoDir, branch, base, message string) error {
				merges = append(merges, branch+"→"+base)

				return nil
			}

			s.mergeOnApprove(context.Background(), tk, journal.Fact{Detail: tt.detail}, &stats)

			if len(merges) != tt.want {
				t.Fatalf("merges = %v, want %d", merges, tt.want)
			}
		})
	}
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

// flakyReviewStore fails Gets for one task ID and review enqueues with a
// store-shaped error — the transient failure class a mint refusal
// represents. Everything else rides the real store.
type flakyReviewStore struct {
	queue.Store

	failGetID    task.ID
	failGets     atomic.Int32
	failEnqueues atomic.Int32
}

func (f *flakyReviewStore) Get(ctx context.Context, id task.ID) (task.Task, error) {
	if id == f.failGetID && f.failGets.Load() > 0 {
		f.failGets.Add(-1)

		return task.Task{}, errors.New("sqlite: database is locked (simulated)")
	}

	return f.Store.Get(ctx, id)
}

func (f *flakyReviewStore) Enqueue(ctx context.Context, n task.New) (task.Task, error) {
	if f.failEnqueues.Load() > 0 {
		f.failEnqueues.Add(-1)

		return task.Task{}, errors.New("sqlite: database is locked (simulated)")
	}

	return f.Store.Enqueue(ctx, n)
}

// completedFactSeq returns the journal seq of the task's completed fact.
func completedFactSeq(t *testing.T, s *sqlite.Store, id task.ID) int64 {
	t.Helper()

	facts, err := s.Facts(context.Background(), 0, 10_000)
	if err != nil {
		t.Fatalf("facts: %v", err)
	}

	for _, f := range facts {
		if f.Type == journal.Completed && f.TaskID == id.String() {
			return f.Seq
		}
	}

	t.Fatalf("no completed fact for %s", id)

	return 0
}

func watermarkOf(t *testing.T, s queue.Store) int64 {
	t.Helper()

	wm, ok, err := s.Watermark(context.Background(), ConsumerKey)
	if err != nil || !ok {
		t.Fatalf("watermark: %v (ok %v)", err, ok)
	}

	return wm
}

// pinRefusalHoldsCheckpoint asserts the refusal shape: the sweep errored,
// the watermark is held behind the triggering fact, and the refusal was
// warned.
func pinRefusalHoldsCheckpoint(
	t *testing.T,
	s *sqlite.Store,
	capture *captureHandler,
	triggerSeq int64,
	stats SweepStats,
	err error,
) {
	t.Helper()

	if err == nil || !strings.Contains(err.Error(), "mint refused") {
		t.Fatalf("sweep err = %v, want mint refusal", err)
	}

	if stats.ReviewsEnqueued != 0 || stats.FixesEnqueued != 0 {
		t.Fatalf("stats = %+v, want no executed mint", stats)
	}

	if wm := watermarkOf(t, s); wm >= triggerSeq {
		t.Fatalf("watermark = %d, want held behind the triggering fact at %d", wm, triggerSeq)
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
}

// TestSweepGetRefusalHoldsCheckpoint pins the mint-refusal gate (dlqfix
// 13-15 §e pattern): a transient store failure reading the completed task
// aborts the sweep BEFORE the checkpoint — warned, watermark held behind
// the Completed fact — so the retry sweep of the SAME cursor redelivers the
// completion and mints the review instead of silently dropping it forever.
func TestSweepGetRefusalHoldsCheckpoint(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	ctx := context.Background()

	flaky := &flakyReviewStore{Store: s}
	capture := &captureHandler{}

	sw, err := NewSweeper(ctx, flaky, SweeperConfig{Log: slog.New(capture)})
	if err != nil {
		t.Fatalf("NewSweeper: %v", err)
	}

	completed := runAgentTask(t, s, executor.AgentPayload{Repo: "demo", Prompt: "ship it"}, executor.AgentResult{})

	flaky.failGetID = completed.ID
	flaky.failGets.Store(1)

	stats, err := sw.Sweep(ctx)
	pinRefusalHoldsCheckpoint(t, s, capture, completedFactSeq(t, s, completed.ID), stats, err)

	flaky.failGets.Store(0)

	retry, err := sw.Sweep(ctx)
	if err != nil {
		t.Fatalf("retry sweep: %v", err)
	}

	if retry.ReviewsEnqueued != 1 {
		t.Fatalf("retry ReviewsEnqueued = %d, want 1 (stats %+v)", retry.ReviewsEnqueued, retry)
	}
}

// TestSweepEnqueueRefusalHoldsCheckpoint pins the enqueue leg of the gate:
// a transient failure minting the review task refuses and holds the
// checkpoint; the retry mints.
func TestSweepEnqueueRefusalHoldsCheckpoint(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	ctx := context.Background()

	flaky := &flakyReviewStore{Store: s}
	capture := &captureHandler{}

	sw, err := NewSweeper(ctx, flaky, SweeperConfig{Log: slog.New(capture)})
	if err != nil {
		t.Fatalf("NewSweeper: %v", err)
	}

	completed := runAgentTask(t, s, executor.AgentPayload{Repo: "demo", Prompt: "ship it"}, executor.AgentResult{})

	flaky.failEnqueues.Store(1)

	stats, err := sw.Sweep(ctx)
	pinRefusalHoldsCheckpoint(t, s, capture, completedFactSeq(t, s, completed.ID), stats, err)

	flaky.failEnqueues.Store(0)

	retry, err := sw.Sweep(ctx)
	if err != nil {
		t.Fatalf("retry sweep: %v", err)
	}

	if retry.ReviewsEnqueued != 1 {
		t.Fatalf("retry ReviewsEnqueued = %d, want 1 (stats %+v)", retry.ReviewsEnqueued, retry)
	}
}

// TestSweepDoneGuardIsKnownNotRefusal pins that the Queue wrapper's
// done-guard (the dedup key matched a COMPLETED review after an operator
// watermark rewind) counts known and checkpoints past — it must not wedge
// the cursor the way a store failure does.
func TestSweepDoneGuardIsKnownNotRefusal(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	ctx := context.Background()

	sw, err := NewSweeper(ctx, s, SweeperConfig{})
	if err != nil {
		t.Fatalf("NewSweeper: %v", err)
	}

	runAgentTask(t, s, executor.AgentPayload{Repo: "demo", Prompt: "p"}, executor.AgentResult{})

	if _, err := sw.Sweep(ctx); err != nil {
		t.Fatalf("mint sweep: %v", err)
	}

	review := listByType(t, s, executor.TaskTypeReview)[0]

	finishTask(t, s, review.ID, []byte(`{"verdict":"approve"}`))

	if err := s.SetWatermark(ctx, ConsumerKey, 0); err != nil {
		t.Fatalf("rewind: %v", err)
	}

	replay, err := NewSweeper(ctx, queue.New(s), SweeperConfig{})
	if err != nil {
		t.Fatalf("replay sweeper: %v", err)
	}

	stats, err := replay.Sweep(ctx)
	if err != nil {
		t.Fatalf("replay sweep: %v", err)
	}

	if stats.ReviewsKnown != 1 {
		t.Fatalf("stats = %+v, want the done-guard counted known", stats)
	}

	if reviews := listByType(t, s, executor.TaskTypeReview); len(reviews) != 1 {
		t.Fatalf("reviews = %d, want 1 (no re-mint)", len(reviews))
	}
}

// vanishedRecordStore hides one task record — the permanent (deterministic)
// store outcome: a Completed fact whose record is gone can never yield a
// review, so it must skip past instead of wedging the cursor.
type vanishedRecordStore struct {
	queue.Store

	missing task.ID
}

func (m *vanishedRecordStore) Get(ctx context.Context, id task.ID) (task.Task, error) {
	if id == m.missing {
		return task.Task{}, task.ErrNotFound
	}

	return m.Store.Get(ctx, id)
}

// TestSweepVanishedRecordSkipsPast pins the flip side of the gate: a
// permanent outcome (record not found) checkpoints past — the watermark
// ADVANCES behind the Completed fact, no error, no wedge.
func TestSweepVanishedRecordSkipsPast(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	ctx := context.Background()

	masked := &vanishedRecordStore{Store: s}

	sw, err := NewSweeper(ctx, masked, SweeperConfig{})
	if err != nil {
		t.Fatalf("NewSweeper: %v", err)
	}

	completed := runAgentTask(t, s, executor.AgentPayload{Repo: "demo", Prompt: "p"}, executor.AgentResult{})

	masked.missing = completed.ID

	triggerSeq := completedFactSeq(t, s, completed.ID)

	stats, err := sw.Sweep(ctx)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}

	if stats.Skipped != 1 || stats.ReviewsEnqueued != 0 {
		t.Fatalf("stats = %+v, want 1 skipped, no review", stats)
	}

	if wm := watermarkOf(t, s); wm < triggerSeq {
		t.Fatalf("watermark = %d, want advanced past the Completed fact at %d", wm, triggerSeq)
	}
}
