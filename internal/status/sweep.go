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
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

const (
	// defaultPageSize bounds one Facts page per sweep iteration.
	defaultPageSize = 500

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
	// Model, when set, is written into every status payload.
	Model string
	// Log receives one summary line per mint. nil logs nothing.
	Log *slog.Logger
	// PageSize bounds one fact-stream page; 0 selects the default.
	PageSize int
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
	// or a report already in flight).
	Skipped int
}

// Sweeper turns the journal's completion facts into periodic status reports.
// It is safe for concurrent use (a mutex serializes sweeps; ticks and the
// --once drain watcher may call it from different goroutines).
type Sweeper struct {
	store queue.Store
	cfg   SweeperConfig

	mu        sync.Mutex
	watermark int64
	persisted int64 // last checkpoint written to the watermarks table
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

	if cfg.PageSize <= 0 {
		cfg.PageSize = defaultPageSize
	}

	persisted, exists, err := store.Watermark(ctx, ConsumerKey)
	if err != nil {
		return nil, fmt.Errorf("status sweep: read watermark: %w", err)
	}

	// seq 0 with a row is a real cursor ("bootstrapped on an empty journal,
	// consumed nothing yet"): resume from it, do not jump to head.
	if exists {
		return &Sweeper{store: store, cfg: cfg, watermark: persisted, persisted: persisted}, nil
	}

	head, err := store.HeadSeq(ctx)
	if err != nil {
		return nil, fmt.Errorf("status sweep: read journal head: %w", err)
	}

	// First run: eagerly persist the head (even 0) so a crash before the
	// first sweep still resumes exactly here (and never replays history).
	if err := store.SaveWatermark(ctx, ConsumerKey, head); err != nil {
		return nil, fmt.Errorf("status sweep: persist bootstrap watermark: %w", err)
	}

	return &Sweeper{store: store, cfg: cfg, watermark: head, persisted: head}, nil
}

// Sweep consumes new facts since the last pass and mints status tasks for
// full windows. Idempotent by dedup: calling it twice never duplicates work,
// so a replayed page (crash between consumption and checkpoint) re-hits
// dedup keys instead of minting duplicates. The cursor checkpoints after
// each page; a failed checkpoint stops the sweep — facts are never consumed
// past an unpersisted cursor.
func (s *Sweeper) Sweep(ctx context.Context) (SweepStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var stats SweepStats

	// A pending checkpoint gates sweeping: retry it before consuming
	// anything new.
	if s.watermark > s.persisted {
		if err := s.store.SaveWatermark(ctx, ConsumerKey, s.watermark); err != nil {
			return stats, fmt.Errorf("status sweep: checkpoint %d: %w", s.watermark, err)
		}

		s.persisted = s.watermark
	}

	for {
		facts, err := s.store.Facts(ctx, s.watermark, s.cfg.PageSize)
		if err != nil {
			return stats, fmt.Errorf("status sweep: read facts after %d: %w", s.watermark, err)
		}

		for _, f := range facts {
			s.watermark = f.Seq
			stats.Facts++

			s.handleFact(ctx, f, &stats)
		}

		// Page end: checkpoint AFTER the last consumed fact — never before,
		// or a crash would silently skip the page's completions.
		if len(facts) > 0 {
			if err := s.store.SaveWatermark(ctx, ConsumerKey, s.watermark); err != nil {
				return stats, fmt.Errorf("status sweep: checkpoint %d: %w", s.watermark, err)
			}

			s.persisted = s.watermark
		}

		if len(facts) < s.cfg.PageSize {
			return stats, nil
		}
	}
}

// handleFact reacts to one completion fact: only agent completions can fill
// a window; every other type (status included — the loop guard) is consumed
// without effect.
func (s *Sweeper) handleFact(ctx context.Context, f journal.Fact, stats *SweepStats) {
	if f.Type != journal.Completed {
		return
	}

	t, err := s.store.Get(ctx, task.ID(f.TaskID))
	if err != nil {
		stats.Skipped++

		return
	}

	if t.Type != executor.TaskTypeAgent {
		return
	}

	s.maybeMint(ctx, t, f, stats)
}

// maybeMint mints the status task when this completion fills the project's
// window: >= Every completed agent tasks since the last status task (or
// ever), and no status task pending/running for the project.
func (s *Sweeper) maybeMint(ctx context.Context, t task.Task, f journal.Fact, stats *SweepStats) {
	var agentPayload executor.AgentPayload

	if err := json.Unmarshal(t.Payload, &agentPayload); err != nil || agentPayload.Repo == "" || agentPayload.Prompt == "" {
		stats.Skipped++

		return
	}

	statusTasks, err := s.store.List(ctx, queue.Filter{Project: &t.Project, Type: ptr(executor.TaskTypeStatus)})
	if err != nil {
		stats.Skipped++

		return
	}

	var lastReport time.Time

	for _, st := range statusTasks {
		if st.Status == task.Pending || st.Status == task.Running {
			// A report for this project is already in flight; let it land
			// before minting the next window's.
			stats.Skipped++

			return
		}

		if st.CreatedAt.After(lastReport) {
			lastReport = st.CreatedAt
		}
	}

	agentTasks, err := s.store.List(ctx, queue.Filter{Project: &t.Project, Type: ptr(executor.TaskTypeAgent)})
	if err != nil {
		stats.Skipped++

		return
	}

	window := make([]executor.StatusCompletion, 0, s.cfg.Every)

	for _, at := range agentTasks {
		if at.Status != task.Completed {
			continue
		}

		// Completion time, not creation: a task created before the last
		// report but finished after it belongs to THIS window.
		if !lastReport.IsZero() && !at.UpdatedAt.After(lastReport) {
			continue
		}

		completion := executor.StatusCompletion{
			TaskID:      at.ID.String(),
			CompletedAt: at.UpdatedAt.UTC().Format(time.RFC3339),
		}

		var ap executor.AgentPayload
		if json.Unmarshal(at.Payload, &ap) == nil {
			completion.Item = itemExcerpt(ap.Prompt)
		}

		if at.ID == t.ID {
			var result executor.AgentResult
			if json.Unmarshal(f.Detail, &result) == nil {
				completion.Commit = result.CommitSHA
				completion.Files = result.FilesChanged
			}
		}

		window = append(window, completion)
	}

	if len(window) < s.cfg.Every {
		return // window not full yet; nothing to mint
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

	payload, err := json.Marshal(executor.StatusPayload{
		Repo:      agentPayload.Repo,
		Project:   t.Project,
		Completed: window,
		Model:     s.cfg.Model,
		Yolo:      agentPayload.Yolo,
	})
	if err != nil {
		stats.Skipped++

		return
	}

	got, err := s.store.Enqueue(ctx, task.New{
		Type:     executor.TaskTypeStatus,
		Project:  t.Project,
		Priority: t.Priority,
		Payload:  payload,
		DedupKey: StatusDedupKey(t.Project, t.ID),
	})
	if err != nil {
		stats.Skipped++

		return
	}

	if got.Attempts == 0 && got.Status == task.Pending {
		stats.ReportsEnqueued++

		s.log("status: enqueued report", "task", got.ID.String(), "repo", agentPayload.Repo,
			"project", t.Project, "window", len(window))
	} else {
		stats.ReportsKnown++
	}
}

func (s *Sweeper) log(msg string, args ...any) {
	if s.cfg.Log != nil {
		s.cfg.Log.Info(msg, args...)
	}
}

// StatusDedupKey is the dedup identity of the report for one window: the
// project plus the completion that filled the window. Two windows never
// share a trigger completion; the same window replayed never mints twice.
func StatusDedupKey(project string, trigger task.ID) string {
	return "status:" + project + ":" + trigger.String()
}

// itemExcerpt reduces an agent prompt to its first line, bounded — a window
// entry pointer, not a transcript (full prompts live in `tq show`).
func itemExcerpt(prompt string) string {
	line := strings.TrimSpace(prompt)
	if idx := strings.IndexByte(line, '\n'); idx >= 0 {
		line = line[:idx]
	}

	const maxLen = 200
	if len(line) > maxLen {
		line = line[:maxLen] + "…"
	}

	return line
}

func ptr[v any](val v) *v {
	return &val
}
