// Package status automates the "done prompt": after every N completed agent
// tasks in a project, the sweeper mints ONE status task — a headless agent
// that writes a full status report (docs/status/<timestamp>.md) and closes
// the loop by appending the next work items to the repo's TODO_LIST.md,
// where the next harvest tick picks them up.
//
// Loop safety is structural (same rule as internal/review): only completions
// of "agent" tasks are counted — status tasks never trigger status tasks.
// The N-completion window is derived from queue state (completed agent tasks
// updated since the last status task's creation), never from sweeper memory,
// so restarts cannot desynchronize it. Idempotency is the store's dedup:
// the window's triggering completion task IDs key the minted tasks.
package status

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"slices"
	"sort"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
	"github.com/larsartmann/go-taskqueue/internal/watermark"
)

const (
	// ConsumerKey is the sweeper's identity in the watermarks table. One
	// cursor is shared by every status sweeper over the same database:
	// completions recorded while no sweeper ran are counted on the next
	// start (the window query is queue-derived, not cursor-derived).
	ConsumerKey = "status-sweeper"

	// maxWindowItems bounds the Completed list pinned into one status
	// payload: prompts stay sane even when a poisoned/failed report let a
	// window grow far past Every.
	maxWindowItems = 50
)

// SweeperConfig controls one Sweeper.
type SweeperConfig struct {
	// Every is the number of completed agent tasks per project that mint
	// one status report. Required, >= 1.
	Every int
	// MinInterval is the per-project floor between two status reports:
	// a mint is skipped while the project's last status task is younger
	// (2026-10-11 ceremony cut — --status-every 5 produced 213 reports in
	// one repo; the window still accumulates, so the next report after the
	// floor covers everything). 0 = no floor.
	MinInterval time.Duration
	// Model, when set, is written into every status payload.
	Model string
	// Log receives one summary line per mint. nil logs nothing.
	Log *slog.Logger
	// PageSize bounds one fact-stream page; 0 selects the default.
	PageSize int
	// AllowDirty mirrors the pool's --allow-dirty: minted status payloads
	// then tolerate an uncommitted tree (RequireClean=false). A
	// multi-agent repo is effectively always dirty; without this the
	// status preflight would requeue forever on an --allow-dirty pool.
	AllowDirty bool
	// TaskTimeout is pinned into minted payloads as TimeoutMinutes (0
	// keeps the executor's default). The executor default (15m) is tuned
	// for stub-y repos; a real done-prompt run PLUS the repo verify gate
	// needs the pool's own task budget.
	TaskTimeout time.Duration
}

// SweepStats summarizes one sweep pass.
type SweepStats struct {
	// Facts is the number of new facts consumed.
	Facts int
	// ReportsEnqueued counts fresh status tasks this sweep created.
	ReportsEnqueued int
	// ReportsKnown counts mints that hit an existing dedup key (the report
	// task already existed — replayed page or a concurrent sweeper).
	ReportsKnown int
	// Skipped counts completions that could not yield a status task
	// (foreign payload shape, vanished task record, window not yet full,
	// or a report already in flight). A refused MINT is not counted here:
	// it aborts the sweep before the checkpoint so the completion
	// redelivers.
	Skipped int
}

// Sweeper turns the journal's completion facts into periodic status reports.
// It is safe for concurrent use (the cursor serializes sweeps; ticks and the
// --once drain watcher may call it from different goroutines).
//
// The struct shell deliberately stays per-sweeper (config fields and fact
// validation differ across the four sweepers); the shared pump is
// watermark.Cursor.Sweep.
type Sweeper struct {
	store queue.Store
	cfg   SweeperConfig

	cur *watermark.Cursor
}

// NewSweeper returns a sweeper over store. The cursor resumes from the
// persisted checkpoint; a first run bootstraps at the journal head
// (completions that predate the sweeper are not replayed as trigger facts —
// but the window QUERY counts them, so the first N-th completion after
// start reports on the full backlogged window). Rewind deliberately with
// `tq watermarks set status-sweeper SEQ`.
func NewSweeper(ctx context.Context, store queue.Store, cfg SweeperConfig) (*Sweeper, error) {
	if cfg.Every < 1 {
		return nil, fmt.Errorf("status sweep: Every must be >= 1, got %d", cfg.Every)
	}

	cur, err := watermark.New(ctx, watermark.Config{
		Store:  store,
		Key:    ConsumerKey,
		Domain: "status sweep",
	})
	if err != nil {
		return nil, err
	}

	return &Sweeper{store: store, cfg: cfg, cur: cur}, nil
}

// Sweep consumes new facts since the last pass and mints status tasks for
// full windows. Idempotent by dedup: calling it twice never duplicates work,
// so a replayed page (crash between consumption and checkpoint) re-hits
// dedup keys instead of minting duplicates.
//
// A mint refusal — a store failure while reading a completion, listing the
// window, or enqueueing the report — aborts the sweep BEFORE the checkpoint:
// the fact redelivers on the next pass instead of silently dropping the
// window's report forever (dlqfix 13-15 §e pattern). Deterministic skips
// (foreign shapes, vanished records, done-guard dedup hits) stay
// checkpointed past: retrying them can never succeed.
func (s *Sweeper) Sweep(ctx context.Context) (SweepStats, error) {
	var stats SweepStats

	err := s.cur.Sweep(ctx, func(ctx context.Context, f journal.Fact) error {
		stats.Facts++

		return s.handleFact(ctx, f, &stats)
	})

	return stats, err
}

// handleFact reacts to one completion fact: only agent completions can fill
// a window; every other type (status included — the loop guard) is consumed
// without effect. A mint refusal propagates (checkpoint held); deterministic
// skips never abort the sweep.
func (s *Sweeper) handleFact(ctx context.Context, f journal.Fact, stats *SweepStats) error {
	if f.Type != journal.Completed {
		return nil
	}

	t, err := s.store.Get(ctx, task.ID(f.TaskID))
	if err != nil {
		if watermark.PermanentOutcome(err) {
			stats.Skipped++

			return nil
		}

		return s.refuse(f.TaskID, "read completed record", err)
	}

	if t.Type != executor.TaskTypeAgent {
		return nil
	}

	return s.maybeMint(ctx, t, stats)
}

// refuse reports a mint refusal and returns the error that holds the
// sweep's checkpoint: a refused mint must not advance past the completion,
// or the window's report never happens without an operator watermark
// rewind (mirror of the dlqfix refuseMint pattern).
func (s *Sweeper) refuse(completed, why string, err error) error {
	s.warn("status: report mint refused — checkpoint held",
		"completed", completed, "why", why, "err", err)

	return fmt.Errorf("status: report mint refused for completion %s (%s): %w", completed, why, err)
}

func (s *Sweeper) warn(msg string, args ...any) {
	if s.cfg.Log != nil {
		s.cfg.Log.Warn(msg, args...)
	}
}

// maybeMint mints the status task when this completion fills the project's
// window: >= Every completed agent tasks since the last status task (or
// ever), and no status task pending/running for the project. A store
// failure under the mint refuses (checkpoint held); the done-guard (a dedup
// key whose report already completed) counts known.
func (s *Sweeper) maybeMint(ctx context.Context, t task.Task, stats *SweepStats) error {
	var agentPayload executor.AgentPayload

	if err := json.Unmarshal(
		t.Payload,
		&agentPayload,
	); err != nil || agentPayload.Repo == "" ||
		agentPayload.Prompt == "" {
		stats.Skipped++

		return nil
	}

	statusTasks, err := s.store.List(ctx, queue.Filter{Project: &t.Project, Type: ptr(executor.TaskTypeStatus)})
	if err != nil {
		return s.refuse(t.ID.String(), "list project status tasks", err)
	}

	var lastReport time.Time

	for _, statusTask := range statusTasks {
		if statusTask.Status == task.Pending || statusTask.Status == task.Running {
			// A report for this project is already in flight; let it land
			// before minting the next window's.
			stats.Skipped++

			return nil
		}

		if statusTask.CreatedAt.After(lastReport) {
			lastReport = statusTask.CreatedAt
		}
	}

	if s.cfg.MinInterval > 0 && !lastReport.IsZero() && time.Since(lastReport) < s.cfg.MinInterval {
		stats.Skipped++

		return nil
	}

	agentTasks, err := s.store.List(ctx, queue.Filter{Project: &t.Project, Type: ptr(executor.TaskTypeAgent)})
	if err != nil {
		return s.refuse(t.ID.String(), "list project agent tasks", err)
	}

	window := make([]executor.StatusCompletion, 0, s.cfg.Every)

	for _, agentTask := range agentTasks {
		if agentTask.Status != task.Completed {
			continue
		}

		// Completion time, not creation: a task created before the last
		// report but finished after it belongs to THIS window.
		if !lastReport.IsZero() && !agentTask.UpdatedAt.After(lastReport) {
			continue
		}

		completion := executor.StatusCompletion{
			TaskID: agentTask.ID.String(),
			// Nanosecond precision: entries completed within the same
			// second (parallel load) must still sort by true completion
			// order — second-granularity strings tie and fall through to
			// the random TaskID tiebreak, which once flipped a window so
			// its first entry was the report-less task (one-off flake in
			// TestSweepPinsCloseoutReportPaths, 2026-09-23 02-35 §c1).
			CompletedAt: agentTask.UpdatedAt.UTC().Format(time.RFC3339Nano),
		}

		var ap executor.AgentPayload
		if json.Unmarshal(agentTask.Payload, &ap) == nil {
			completion.Item = workItemLabel(ap)
		}

		// Every entry carries its own mechanical outcome (commit + files),
		// pulled from the task's completion fact — richer reports without
		// git-log guesswork.
		completion.Commit, completion.Files, _ = s.completionDetail(ctx, agentTask.ID)
		completion.Report = closeoutReportPath(agentPayload.Repo, agentTask.ID)

		window = append(window, completion)
	}

	if len(window) < s.cfg.Every {
		return nil // window not full yet; nothing to mint
	}

	sort.Slice(window, func(i, j int) bool {
		if window[i].CompletedAt != window[j].CompletedAt {
			return window[i].CompletedAt < window[j].CompletedAt
		}

		return window[i].TaskID < window[j].TaskID
	})

	if len(window) > maxWindowItems {
		window = window[len(window)-maxWindowItems:]
	}

	payload := executor.StatusPayload{
		Repo:      agentPayload.Repo,
		Project:   t.Project,
		Completed: window,
		Model:     s.cfg.Model,
		Yolo:      agentPayload.Yolo,
		// Mirror the operator's dirty-tree stance: an --allow-dirty pool
		// must not have its status runs deadlocked by other agents' WIP.
		RequireClean: new(!s.cfg.AllowDirty),
	}
	if s.cfg.TaskTimeout > 0 {
		payload.TimeoutMinutes = int(s.cfg.TaskTimeout / time.Minute)
	}

	blob, err := json.Marshal(payload)
	if err != nil {
		stats.Skipped++

		return nil
	}

	got, err := s.store.Enqueue(ctx, task.New{
		Type:     executor.TaskTypeStatus,
		Project:  t.Project,
		Priority: t.Priority,
		Payload:  blob,
		DedupKey: StatusDedupKey(t.Project, t.ID),
	})
	if errors.Is(err, queue.ErrTaskDone) {
		// The done-guard (Queue wrapper): the dedup key matched a
		// COMPLETED report — this window's report already ran. Known,
		// not refused: the checkpoint moves past.
		stats.ReportsKnown++

		return nil
	}

	if err != nil {
		return s.refuse(t.ID.String(), "enqueue status report", err)
	}

	if got.Attempts == 0 && got.Status == task.Pending {
		stats.ReportsEnqueued++

		s.log("status: enqueued report", "task", got.ID.String(), "repo", agentPayload.Repo,
			"project", t.Project, "window", len(window))
	} else {
		stats.ReportsKnown++
	}

	return nil
}

func (s *Sweeper) log(msg string, args ...any) {
	if s.cfg.Log != nil {
		s.cfg.Log.Info(msg, args...)
	}
}

// completionDetail reads one task's completion-fact detail as AgentResult.
// Best-effort by design: a vanished fact, an empty detail (plain agent runs
// without a TQ_RESULT self-report) or a foreign shape returns ok=false and
// the window entry simply carries no commit info.
func (s *Sweeper) completionDetail(ctx context.Context, id task.ID) (commit string, files []string, ok bool) {
	facts, err := s.store.FactsForTask(ctx, id.String(), maxWindowItems)
	if err != nil {
		return "", nil, false
	}

	for _, fact := range slices.Backward(facts) {
		if fact.Type != journal.Completed {
			continue
		}

		var result executor.AgentResult
		if json.Unmarshal(fact.Detail, &result) != nil {
			return "", nil, false
		}

		return result.CommitSHA, result.FilesChanged, true
	}

	return "", nil, false
}

// closeoutReportPath resolves one completed task's closeout report under
// the repo's docs/status/ (the DefaultCloseoutPrompt names it
// docs/status/tasks/<YYYY-MM-DD_HH-MM>_task-<id>.md since the O7
// report-placement ruling; pre-tasks/ reports live at docs/status/ root
// and stay resolvable — only the task id is stable). Best-effort:
// no match, no repo on disk or a bad pattern returns "" and the window
// entry carries no report. Returns the repo-relative path of the NEWEST
// match (several matches only when a task somehow reported twice).
func closeoutReportPath(repo string, id task.ID) string {
	if repo == "" {
		return ""
	}

	var matches []string

	for _, dir := range []string{"tasks", "."} {
		globMatches, err := filepath.Glob(
			filepath.Join(repo, "docs", "status", dir, "*_task-"+id.String()+".md"),
		)
		if err != nil {
			return ""
		}

		matches = append(matches, globMatches...)
	}

	if len(matches) == 0 {
		return ""
	}

	sort.Strings(matches)

	rel, err := filepath.Rel(repo, matches[len(matches)-1])
	if err != nil {
		return ""
	}

	return filepath.ToSlash(rel)
}

// StatusDedupKey is the dedup identity of the report for one window: the
// project plus the completion that filled the window. Two windows never
// share a trigger completion; the same window replayed never mints twice.
func StatusDedupKey(project string, trigger task.ID) string {
	return "status:" + project + ":" + trigger.String()
}

// workItemLabel is the human-readable line for a window entry: the raw
// TODO_LIST item when the harvester pinned one, else the prompt's first line
// (payloads minted before Item existed).
func workItemLabel(ap executor.AgentPayload) string {
	if ap.Item != "" {
		return executor.Excerpt(ap.Item)
	}

	return executor.Excerpt(ap.Prompt)
}

//go:fix inline
func ptr[v any](val v) *v {
	return new(val)
}
