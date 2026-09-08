// Package review closes the loop around agent work: every completed agent
// task gets ONE review by a second agent, and a request_changes verdict can
// mint fix tasks. The reviewer only contributes findings — everything
// downstream (verdict bookkeeping, fix-task fan-out, dedup) is mechanical.
//
// Loop safety is structural: reviews are never reviewed (the sweeper only
// reviews the "agent" type), each task is reviewed at most once (the
// review:<task-id> dedup key), and fix tasks minted from findings are
// bounded by the pool's budget guard, which counts every enqueue — reviews
// included — against the daily cap.
package review

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

const (
	// defaultPageSize bounds one Facts page per sweep iteration.
	defaultPageSize = 500

	// ConsumerKey is the sweeper's identity in the watermarks table: the
	// persisted cursor shared by every sweeper over the same database, so
	// agent tasks completed while no sweeper was running still get their
	// review when the next pool starts.
	ConsumerKey = "review-sweeper"
)

// SweeperConfig controls one Sweeper.
type SweeperConfig struct {
	// Model, when set, is written into every review payload (and fix task).
	Model string
	// Autofix, when true, mints an agent fix task per finding of every
	// request_changes review. Off by default: verdict-only mode is the safe
	// first step; autofix closes the loop autonomously.
	Autofix bool
	// Log receives one summary line per sweep. nil logs nothing.
	Log *slog.Logger
	// PageSize bounds one fact-stream page; 0 selects the default.
	PageSize int
}

// SweepStats summarizes one sweep pass.
type SweepStats struct {
	// Facts is the number of new facts consumed.
	Facts int
	// ReviewsEnqueued counts fresh review tasks this sweep created.
	ReviewsEnqueued int
	// ReviewsKnown counts completed agent tasks whose review task already
	// existed (dedup hit — the review ran, or is pending, elsewhere).
	ReviewsKnown int
	// FixesEnqueued counts fresh fix tasks minted from findings.
	FixesEnqueued int
	// Skipped counts completions that could not yield a review (foreign
	// payload shape, vanished task record).
	Skipped int
}

// Sweeper turns the journal's completion facts into review work. It is safe
// for concurrent use (a mutex serializes sweeps; ticks and the --once
// drain watcher may call it from different goroutines).
type Sweeper struct {
	store queue.Store
	cfg   SweeperConfig

	mu        sync.Mutex
	watermark int64
	persisted int64 // last checkpoint written to the watermarks table
}

// NewSweeper returns a sweeper over store. The cursor resumes from the
// persisted checkpoint — completions recorded while no sweeper was running
// are reviewed on the next start. A first run bootstraps at the journal
// head: completions that predate the sweeper are not replayed (same
// semantics as the papdashboard bridge; review the gap via `tq show`, or
// rewind with `tq watermarks set review-sweeper SEQ`).
func NewSweeper(ctx context.Context, store queue.Store, cfg SweeperConfig) (*Sweeper, error) {
	if cfg.PageSize <= 0 {
		cfg.PageSize = defaultPageSize
	}

	persisted, exists, err := store.Watermark(ctx, ConsumerKey)
	if err != nil {
		return nil, fmt.Errorf("review sweep: read watermark: %w", err)
	}

	// seq 0 with a row is a real cursor ("bootstrapped on an empty journal,
	// consumed nothing yet"): resume from it, do not jump to head.
	if exists {
		return &Sweeper{store: store, cfg: cfg, watermark: persisted, persisted: persisted}, nil
	}

	head, err := store.HeadSeq(ctx)
	if err != nil {
		return nil, fmt.Errorf("review sweep: read journal head: %w", err)
	}

	// First run: eagerly persist the head (even 0) so a crash before the
	// first sweep still resumes exactly here (and never replays history).
	if err := store.SaveWatermark(ctx, ConsumerKey, head); err != nil {
		return nil, fmt.Errorf("review sweep: persist bootstrap watermark: %w", err)
	}

	return &Sweeper{store: store, cfg: cfg, watermark: head, persisted: head}, nil
}

// Sweep consumes new facts since the last pass and enqueues review (and,
// with Autofix, fix) tasks. Idempotent by dedup: calling it twice never
// duplicates work, so a replayed page (crash between consumption and
// checkpoint) re-hits dedup keys instead of minting duplicates. The cursor
// checkpoints after each page; a failed checkpoint stops the sweep — facts
// are never consumed past an unpersisted cursor.
func (s *Sweeper) Sweep(ctx context.Context) (SweepStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var stats SweepStats

	// A pending checkpoint gates sweeping (same rule as the bridge's
	// drain): retry it before consuming anything new.
	if s.watermark > s.persisted {
		if err := s.store.SaveWatermark(ctx, ConsumerKey, s.watermark); err != nil {
			return stats, fmt.Errorf("review sweep: checkpoint %d: %w", s.watermark, err)
		}

		s.persisted = s.watermark
	}

	for {
		facts, err := s.store.Facts(ctx, s.watermark, s.cfg.PageSize)
		if err != nil {
			return stats, fmt.Errorf("review sweep: read facts after %d: %w", s.watermark, err)
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
				return stats, fmt.Errorf("review sweep: checkpoint %d: %w", s.watermark, err)
			}

			s.persisted = s.watermark
		}

		if len(facts) < s.cfg.PageSize {
			return stats, nil
		}
	}
}

// handleFact reacts to one completion fact: agent completions gain a review
// task, review completions may gain fix tasks (Autofix).
func (s *Sweeper) handleFact(ctx context.Context, f journal.Fact, stats *SweepStats) {
	if f.Type != journal.Completed {
		return
	}

	t, err := s.store.Get(ctx, task.ID(f.TaskID))
	if err != nil {
		stats.Skipped++

		return
	}

	switch t.Type {
	case executor.TaskTypeAgent:
		s.enqueueReview(ctx, t, f, stats)
	case executor.TaskTypeReview:
		if s.cfg.Autofix {
			s.mintFixes(ctx, t, f, stats)
		}
	}
}

// enqueueReview mints the one review task for a completed agent task.
func (s *Sweeper) enqueueReview(ctx context.Context, t task.Task, f journal.Fact, stats *SweepStats) {
	var agentPayload executor.AgentPayload

	if err := json.Unmarshal(t.Payload, &agentPayload); err != nil || agentPayload.Repo == "" || agentPayload.Prompt == "" {
		stats.Skipped++

		return
	}

	var agentResult executor.AgentResult

	_ = json.Unmarshal(f.Detail, &agentResult) // absent/legacy detail is fine

	payload, err := json.Marshal(executor.ReviewPayload{
		Repo:         agentPayload.Repo,
		ReviewedTask: t.ID.String(),
		Item:         agentPayload.Prompt,
		CommitSHA:    agentResult.CommitSHA,
		FilesChanged: agentResult.FilesChanged,
		Model:        s.cfg.Model,
		Yolo:         agentPayload.Yolo,
	})
	if err != nil {
		stats.Skipped++

		return
	}

	got, err := s.store.Enqueue(ctx, task.New{
		Type:     executor.TaskTypeReview,
		Project:  t.Project,
		Priority: t.Priority,
		Payload:  payload,
		DedupKey: ReviewDedupKey(t.ID),
	})
	if err != nil {
		stats.Skipped++

		return
	}

	if got.Attempts == 0 && got.Status == task.Pending {
		stats.ReviewsEnqueued++

		s.log("review: enqueued", "task", got.ID.String(), "reviews", t.ID.String(), "repo", agentPayload.Repo)
	} else {
		stats.ReviewsKnown++
	}
}

// mintFixes turns a request_changes review's findings into agent fix tasks.
func (s *Sweeper) mintFixes(ctx context.Context, t task.Task, f journal.Fact, stats *SweepStats) {
	var reviewPayload executor.ReviewPayload

	if err := json.Unmarshal(t.Payload, &reviewPayload); err != nil || reviewPayload.Repo == "" {
		stats.Skipped++

		return
	}

	var result executor.ReviewResult

	if err := json.Unmarshal(f.Detail, &result); err != nil || result.Verdict != executor.VerdictRequestChanges {
		return // approve, or detail lost — nothing to fix
	}

	for _, finding := range result.Findings {
		payload, err := json.Marshal(executor.AgentPayload{
			Repo:   reviewPayload.Repo,
			Model:  reviewPayload.Model,
			Yolo:   reviewPayload.Yolo,
			Prompt: fixPrompt(reviewPayload, finding),
		})
		if err != nil {
			stats.Skipped++

			continue
		}

		got, err := s.store.Enqueue(ctx, task.New{
			Type:     executor.TaskTypeAgent,
			Project:  t.Project,
			Priority: t.Priority,
			Payload:  payload,
			DedupKey: FixDedupKey(t.ID, finding.Title),
		})
		if err != nil {
			stats.Skipped++

			continue
		}

		if got.Attempts == 0 && got.Status == task.Pending {
			stats.FixesEnqueued++

			s.log("review: enqueued fix", "task", got.ID.String(), "review", t.ID.String(),
				"finding", finding.Title, "repo", reviewPayload.Repo)
		}
	}
}

func (s *Sweeper) log(msg string, args ...any) {
	if s.cfg.Log != nil {
		s.cfg.Log.Info(msg, args...)
	}
}

// ReviewDedupKey is the dedup identity of the review of one task: exactly
// one review per reviewed task, forever.
func ReviewDedupKey(reviewed task.ID) string {
	return "review:" + reviewed.String()
}

// FixDedupKey is the dedup identity of the fix task for one finding of one
// review: the same finding re-reported by the same review never spawns a
// second fix task.
func FixDedupKey(review task.ID, findingTitle string) string {
	sum := sha256.Sum256([]byte(findingTitle))

	return "reviewfix:" + review.String() + ":" + hex.EncodeToString(sum[:8])
}

// fixPrompt builds the instruction for a fix task minted from one finding.
func fixPrompt(p executor.ReviewPayload, finding executor.ReviewFinding) string {
	var b strings.Builder

	b.WriteString("A code reviewer rejected your earlier work on this task and filed one finding. Fix EXACTLY this finding — no unrelated changes.\n\n")
	b.WriteString("## Original task\n\n" + strings.TrimSpace(p.Item) + "\n\n")
	b.WriteString("## Reviewer finding (" + finding.Severity + ")\n\n" + strings.TrimSpace(finding.Title) + "\n\n")

	if detail := strings.TrimSpace(finding.Detail); detail != "" {
		b.WriteString(detail + "\n\n")
	}

	if p.CommitSHA != "" {
		b.WriteString("The rejected change is commit " + p.CommitSHA + ".\n\n")
	}

	b.WriteString("Address the finding minimally, keep the repository's contracts (AGENTS.md / docs), and make the repo's own gates (build, vet, tests, format) pass before finishing.")

	return b.String()
}
